# Manual de despliegue: work-hub (Go + React) con GitHub Actions + Dokploy

Adaptado del flujo usado en `restaurant-management` (Java+Dokploy) a este stack:
- **Backend**: Go (chi), un solo binario, migraciones propias (`store.Migrate`, corren solas al arrancar).
- **Frontend**: React + Vite, build estático servido por nginx.
- **Monorepo**: un solo repo (`wotk-hub`) con `backend/` y `frontend/` — dos imágenes, dos apps Dokploy, dos workflows con `paths:` separados (ya están en `.github/workflows/backend.yml` y `frontend.yml`).

---

## 0. Arquitectura del flujo

```
push a main (backend/** o frontend/**) → GitHub Actions:
   test/quality (build+vet+gofmt | lint+typecheck+build)
   → build imagen (local) → Trivy scan → push a GHCR (:latest + :sha)
   → SSH al VPS → curl API local de Dokploy → Dokploy hace pull + run
Traefik (Dokploy) → HTTPS (Let's Encrypt) → contenedor
```

Principios (idénticos al flujo de Java):
- **Artefacto inmutable**: se construye una vez, se despliega esa imagen.
- **Pull-based**: Dokploy jala la imagen; el panel queda privado.
- **Secretos fuera de la imagen**: van como env vars en runtime.
- **Path-scoped**: tocar solo `frontend/` no dispara el pipeline del backend, y viceversa.

---

## 1. Prerrequisitos

- VPS (mín. 2 GB RAM) con Ubuntu/Debian.
- Dominio propio. Ej. `tudominio.com` → dos subdominios: `app.tudominio.com` (frontend), `api.tudominio.com` (backend).
- Repo `wotk-hub` en GitHub (ya existe: `github.com/ProtoSG/wotk-hub`).

**Importante — mismo dominio raíz**: las cookies de auth (`access_token`/`refresh_token`) usan `SameSite=Lax` (`backend/modules/auth/helpers.go`). Eso funciona cross-subdominio (`app.` ↔ `api.` del mismo `tudominio.com`) porque siguen siendo "same-site". **No** funcionaría si frontend y backend viven en dominios raíz distintos.

---

## 2. Endurecer el VPS (una vez)

Igual que la guía de Java — ver `DEPLOYMENT_GUIDE_JAVA_DOKPLOY.md` §2 si el VPS es nuevo:
- Usuario `deploy` con sudo, SSH por llave (sin password, sin root).
- `ufw` (22/80/443) + `fail2ban` + `unattended-upgrades`.
- Docker publica puertos saltándose ufw → Postgres **sin puerto externo**, solo red interna de Dokploy.

Si el VPS ya corre Dokploy para `restaurant-management`, sáltate este paso — se reutiliza el mismo VPS/panel para las dos apps de `wotk-hub`.

---

## 3. Instalar Dokploy (si no está ya)

```bash
curl -sSL https://dokploy.com/install.sh | sudo sh
```

Si ya tenés Dokploy corriendo (otro proyecto en el mismo VPS), no reinstales — solo creá un **nuevo proyecto** dentro del panel para `wotk-hub`.

---

## 4. Health endpoint (ya agregado)

`backend/main.go` expone `GET /health` → `{"status":"ok"}`, público (montado antes del grupo con `JWTAuth`). Úsalo para el healthcheck de Dokploy y para verificar el deploy:

```bash
curl https://api.tudominio.com/health
```

---

## 5. Dockerfiles (ya existen en el repo)

- `backend/Dockerfile` — multi-stage `golang:1.25-alpine` → `alpine:3.20`, binario estático (`CGO_ENABLED=0`), expone `3001`.
- `frontend/Dockerfile` — multi-stage `oven/bun:1-alpine` → `nginx:1.27-alpine`, expone `80`, recibe `VITE_API_URL` como build-arg (se hornea en el bundle en build time — Vite no lee env vars en runtime).
- `backend/.dockerignore` / `frontend/.dockerignore` — ya excluyen `.env`, `.git`, `*.md`.

No hay que tocar nada acá; ya están listos para CI.

---

## 6. Workflows de GitHub Actions (ya existen)

`.github/workflows/backend.yml` y `.github/workflows/frontend.yml` — cada uno dispara solo con cambios en su subcarpeta (`paths:`), y en PR solo corre el job de calidad (sin build/push/deploy, sin secretos expuestos).

**Backend** (`test` → `build-push` → `deploy`):
- `test`: `go vet ./...`, `go build ./...`, `gofmt -l .` (no hay `_test.go` todavía — agregar tests reales cuando existan y cambiar esto a `go test ./...`).
- `build-push`: build en `./backend`, Trivy (bloquea CRITICAL/HIGH), push a `ghcr.io/protosg/wotk-hub-backend`.
- `deploy`: SSH al VPS → `curl` a la API local de Dokploy con `DOKPLOY_APP_ID_BACKEND`.

**Frontend** (`quality` → `build-push` → `deploy`):
- `quality`: `bun install --frozen-lockfile`, `bun run lint`, `bun run build` (el build corre `tsc -b` primero, así que ya cubre el typecheck).
- `build-push`: build en `./frontend` con `build-args: VITE_API_URL=${{ vars.VITE_API_URL }}`, Trivy, push a `ghcr.io/protosg/wotk-hub-frontend`.
- `deploy`: igual que el backend pero con `DOKPLOY_APP_ID_FRONTEND`.

Ambos comparten `DOKPLOY_TOKEN` (un solo token sirve para todas las apps del panel).

---

## 7. DNS

| Type | Host | Answer |
|------|------|--------|
| A | `app` | `VPS_IP` |
| A | `api` | `VPS_IP` |

(O wildcard `*` si preferís no crear un registro por subdominio nuevo.)

---

## 8. Dokploy: Postgres + las dos apps

Un solo **proyecto** Dokploy para `wotk-hub` (comparte red interna entre servicios).

### Postgres
Create Service → **Database → PostgreSQL**, versión 16 (coincide con `backend/docker-compose.yml` local).
- Sin puerto externo.
- Anota el host interno (ej. `wotk-hub-db-xxxx`) — no hace falta correr `store.Migrate` a mano, el backend migra solo al arrancar (`main.go:47`).

### App: backend
Create Service → **Application** → **Provider: Docker** → `ghcr.io/protosg/wotk-hub-backend:latest`.
- Registry privado: `Registry URL: ghcr.io`, `Username: <tu-usuario>`, `Password: <PAT con read:packages>`.
- **Environment**:
  ```
  PORT=3001
  CORS_ORIGIN=https://app.tudominio.com
  DATABASE_URL=postgres://workhub:PASSWORD@HOST_INTERNO:5432/workhub?sslmode=disable
  JWT_SECRET=<openssl rand -base64 64>
  COOKIE_SECURE=true
  ```
- **Domains** → Host `api.tudominio.com`, Container Port `3001`, HTTPS ON, Let's Encrypt.
- **Health check path**: `/health`.
- Activa **force pull**.

### App: frontend
Create Service → **Application** → **Provider: Docker** → `ghcr.io/protosg/wotk-hub-frontend:latest`.
- Mismo registry privado que el backend.
- Sin env vars en runtime — `VITE_API_URL` ya quedó horneado en el bundle en build time (viene del repo variable `VITE_API_URL` en GitHub, ver §9).
- **Domains** → Host `app.tudominio.com`, Container Port `80`, HTTPS ON, Let's Encrypt.
- Activa **force pull**.

---

## 9. Secretos y variables para el auto-deploy

### Llave SSH dedicada de CI (reutilizable si ya la creaste para restaurant-management en el mismo VPS)
```bash
ssh-keygen -t ed25519 -f ci_deploy_key -N "" -C "ci-deploy@github-actions"
# pública al VPS:
echo 'CONTENIDO_DE_ci_deploy_key.pub' >> ~/.ssh/authorized_keys   # como usuario deploy
```

### Token de Dokploy + applicationIds
- **Token**: panel → Settings → API → Generate. Uno solo sirve para ambas apps.
- **applicationId**: en la URL de cada app → `.../services/application/<APP_ID>` (uno para backend, otro para frontend).

### Secrets en GitHub (repo → Settings → Secrets and variables → Actions → Secrets)
```bash
gh secret set DOCKERHUB_USERNAME --body "tu-usuario-dockerhub"
gh secret set DOCKERHUB_TOKEN
gh secret set SSH_HOST --body "VPS_IP"
gh secret set SSH_USER --body "deploy"
gh secret set SSH_KEY  < ci_deploy_key
gh secret set DOKPLOY_TOKEN
gh secret set DOKPLOY_APP_ID_BACKEND  --body "<APP_ID_BACKEND>"
gh secret set DOKPLOY_APP_ID_FRONTEND --body "<APP_ID_FRONTEND>"
```

### Variable (no-secreta) en GitHub (mismo lugar, pestaña **Variables**)
```bash
gh variable set VITE_API_URL --body "https://api.tudominio.com"
```
(No es secret: termina embebida y visible en el JS del bundle igual — no hay nada que ocultar.)

### Verificá el endpoint de deploy (por túnel SSH al panel)
```bash
ssh -i ~/.ssh/vps -L 3000:localhost:3000 deploy@VPS_IP
curl -i -X POST http://localhost:3000/api/application.deploy \
  -H "x-api-key: TOKEN" -H "Content-Type: application/json" \
  -d '{"applicationId":"APP_ID_BACKEND"}'      # 200 = OK
```

---

## 10. Primer despliegue (orden importa)

1. Mergea los workflows a `main` (ya están en `.github/workflows/`). El primer push que toque `backend/**` o `frontend/**` construye y sube la imagen a GHCR — la app de Dokploy no se puede crear apuntando a una imagen que todavía no existe.
2. Confirma ambas imágenes en GitHub → repo → **Packages**.
3. Crea las dos apps en Dokploy (§8) + Postgres + env vars + dominios.
4. Pon los 7 secrets + 1 variable (§9) **antes** de mergear de nuevo, o el job `deploy` queda rojo (build-push igual funciona y sube la imagen).
5. Siguiente push a `main` → deploy automático. Verifica:
   ```bash
   curl https://api.tudominio.com/health
   curl -I https://app.tudominio.com
   ```

---

## 11. Rollback

Cada build deja `:<sha>` en GHCR. Para volver atrás: en Dokploy, cambia el tag de la app afectada (backend o frontend, son independientes) a un `:<sha>` previo y redeploy.

---

## 12. Troubleshooting

| Síntoma | Causa / fix |
|---|---|
| Backend arranca y muere: `JWT_SECRET is required` / `DATABASE_URL is required` | Faltan esas env vars en la app de Dokploy — `main.go` falla rápido a propósito, sin defaults inseguros. |
| Login funciona pero las llamadas siguientes dan 401 en loop | Frontend y backend en dominios raíz distintos → cookie `SameSite=Lax` no viaja. Deben compartir dominio raíz (`app.` / `api.` de `tudominio.com`). |
| CORS bloqueado en el navegador | `CORS_ORIGIN` en el backend no matchea exacto el origin del frontend (esquema+host, sin slash final). Revisa `backend/middleware/middleware.go`. |
| Cambié `VITE_API_URL` y no pasa nada | Es build-time, no runtime — necesita rebuild de la imagen del frontend (push nuevo o re-run del workflow), no alcanza con cambiar la env var en Dokploy. |
| `deploy` job falla por SSH | Falta la pública de CI en `authorized_keys` del VPS, o `SSH_KEY` mal copiada. |
| Trivy rojo | Igual que en la guía de Java: fuerza versiones de deps con CVE, o `apk upgrade --no-cache` ya está en ambos Dockerfiles para el SO base. |
| 500 en cualquier endpoint tras deploy | Revisa logs del contenedor backend en Dokploy — casi siempre `DATABASE_URL` mal armada o Postgres sin levantar todavía. |

---

## 13. Checklist rápido

- [ ] VPS endurecido, Dokploy instalado (o reutilizado), panel cerrado al público.
- [ ] DNS `app.` y `api.` apuntando al VPS.
- [ ] Postgres en Dokploy (sin puerto público), mismo proyecto que las dos apps.
- [ ] App backend: imagen, env vars (`PORT`, `CORS_ORIGIN`, `DATABASE_URL`, `JWT_SECRET`, `COOKIE_SECURE`), dominio `api.`, healthcheck `/health`.
- [ ] App frontend: imagen, dominio `app.`, sin env vars runtime.
- [ ] Llave CI en el VPS + 7 secrets + 1 variable en GitHub.
- [ ] Endpoint de deploy verificado (200) para ambos `applicationId`.
- [ ] `https://api.tudominio.com/health` y `https://app.tudominio.com` responden tras el primer deploy.
