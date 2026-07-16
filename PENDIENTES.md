# Pendientes — wotk-hub

## 🔴 Alta prioridad

### Backend
- [ ] **Zero tests** — no existe un solo `*_test.go`. Agregar: tests de handlers con mock de DB (`sqlmock` o interface-based DI), tests de middleware (JWT parsing, RequireRole, CLITokenAuth)
- [ ] **Logging no estructurado** — `log.Printf` sin niveles ni request ID. Migrar a `log/slog`
- [ ] **Sin rate limiting** — `/api/auth/login` vulnerable a brute force. Agregar middleware por IP/token (token bucket o sliding window)
- [ ] **Migraciones sin versionado** — `CREATE TABLE IF NOT EXISTS` en startup. Sin rollback, sin CI pre-validate. Considerar `golang-migrate` o `goose`

### Deployment
- [ ] **Go version 1.25 en CI es inexistente** — debe ser `1.23` o `1.24` (a julio 2026). Fix: `.github/workflows/*.yml`
- [ ] **`/health` sin DB ping** — responde ok aunque la DB esté caída. Debería hacer `db.Ping()` para que Dokploy detecte instancias unhealthy
- [ ] **Migraciones en startup** — corre en `main()`, no en CI. En blue/green deployments ambas instancias podrían pelear por el schema. Mover a pipeline de CI

---

## 🟠 Media prioridad

### Auth
- [ ] **Refresh tokens no rotan** — el viejo queda válido hasta `refreshTokenTTL` (30 días). Si fue interceptado, sigue funcional. Implementar refresh token rotation (el viejo se invalida al emitir uno nuevo)
- [ ] **No existe "logout en todos los dispositivos"** — falta endpoint para revocar todos los refresh tokens de un usuario
- [ ] **`accessTokenTTL` de 15 min es corto** — considerar 1h o hacer refresh proactivo antes del expiry desde el interceptor de 401

### Frontend
- [ ] **Sin TanStack Query** — Zustand custom para server state no tiene cache invalidation declarativa ni background refetching. Migrar a `@tanstack/react-query`
- [ ] **Race condition en AuthGuard** — `useState(!hasHydrated)` puede quedar en estado incorrecto si `hasHydrated` cambia entre render y el primer `useEffect`. Fix trivial pero bug real
- [ ] **Error boundary sin reporting** — solo `console.error`. Integrar Sentry o solución propia

### CLI (workhubctl)
- [ ] **`server start` con `time.Sleep(2s)` fijo** — usar polling con timeout en vez de sleep. Si el server tarda >2s (cold start, migración lenta), reporta falsamente que no arrancó
- [ ] **`server logs` no muestra logs reales** — implementar lectura desde archivo o stdout/stderr redirigido
- [ ] **Flag `--api-key` inconsistente** — el CLI usa `--api-key` pero la auth es `Authorization: Bearer CLI_TOKEN`. Renombrar a `--token` o `--cli-token` para claridad
- [ ] **Sin `finances import/export`** — para una app de finanzas es esencial. Agregar comandos para CSV

---

## 🟡 Baja prioridad

### DB
- [ ] **Falta índice compuesto** — `WHERE occurred_on BETWEEN ? AND ? AND category = ?` es el filtro más común en transactions. Índice compuesto o partial sería más eficiente
- [ ] **Sin índice en `refresh_tokens.user_id`** — full table scan en cada logout. Crear índice sobre `user_id`
- [ ] **Tabla `couple_dates` sin índice en `status`** — si en el futuro se filtra por `status = 'planned'`, full scan
- [ ] **Sin soft deletes** — no hay `deleted_at` en ninguna tabla. No hay forma de auditar o hacer undo

### Seguridad
- [ ] **`CLITokenAuth` hardcodea `userID=1, role=admin`** — sin permisos granulares, sin forma de revocar el token sin rotar el env var. Considerar: múltiples tokens con permisos específicos, o revocar por hash del token en DB
- [ ] **DB Manager acepta credenciales en request body** — `POST /api/db/connect` recibe password en el body. No hay validación de legitimidad del request (no es SQL injection, pero sí exposición de credenciales a quien tenga el JWT admin)
- [ ] **No hay email verification ni password reset** — el campo `email` es único pero no está verificado. Necesario si se abre registro público

### Frontend
- [ ] **Loading skeletons** — `RouteFallback` es spinner genérico. Rutas pesadas (Finances, DbManager) se beneficiarían de skeletons reales
- [ ] **`ApiError` en axios.ts no se usa consistentemente** — los calls con `try/catch` muchas veces usan `err.message` sin inspectar `err.code`

### Backend
- [ ] **Módulos tight-coupled al `*sql.DB`** — pasar un interface `Querier` (con `Query`, `QueryRow`, `Exec`) para permitir tests sin DB real
- [ ] **Conexiones BD hardcodeadas** — `SetMaxOpenConns(10)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(30min)` en `store.go`. Hacer configurables via env vars
- [ ] **`shutdownTimeout` de 10s puede ser corto** — si hay requests en vuelo >10s (ytdlp download), se cortan. Revisar para downloads largos
- [ ] **Error 500 sin request ID** — errores 500 no incluyen ID, imposible correlacionar con logs. Ya solucionable con logging estructurado + request ID

---

## Notas de auditoría

- Las migraciones corren en `main()` via `store.Migrate()`. No hay forma de pre-validar en CI ni de hacer rollback.
- El flujo JWT/refresh es crítico y no tiene tests de regresión.
- No hay centralized error tracking (Sentry, Grafana) — errores de producción son invisibles hasta que el usuario se queja.
