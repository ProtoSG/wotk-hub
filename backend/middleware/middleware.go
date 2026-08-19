package middleware

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"workhub/httpx"

	"github.com/golang-jwt/jwt/v5"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

// ParseAllowedOrigins splits a comma-separated origins string into a
// trimmed slice. Shared by CORS below and the games module's WebSocket
// upgrade origin check (see shared/wshub.NewUpgrader) so the two never
// drift out of sync on what counts as an allowed origin.
func ParseAllowedOrigins(origins string) []string {
	allowed := strings.Split(origins, ",")
	for i := range allowed {
		allowed[i] = strings.TrimSpace(allowed[i])
	}
	return allowed
}

func CORS(origins string) func(http.Handler) http.Handler {
	allowed := ParseAllowedOrigins(origins)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqOrigin := r.Header.Get("Origin")
			for _, o := range allowed {
				if o == reqOrigin {
					w.Header().Set("Access-Control-Allow-Origin", reqOrigin)
					w.Header().Set("Vary", "Origin")
					break
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			// Content-Disposition isn't in the browser's default exposed-header
			// safelist, so cross-origin JS (ytdlp's blob download) can't read the
			// real filename without this — it silently falls back to a default.
			w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
			// Cookies are how auth travels now (JWTAuth reads access_token),
			// so cross-origin requests must be allowed to carry them.
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type contextKey string

const userContextKey contextKey = "workhub_auth_user"

// authUser is the value JWTAuth stores in the request context.
type authUser struct {
	userID int64
	role   string
}

// verifyJWTCookie reads the access_token cookie and validates it (HS256,
// signed with secret), returning the authenticated user on success. Shared
// by JWTAuth and RequireAuth so the JWT-parsing logic only lives in one
// place.
func verifyJWTCookie(r *http.Request, secret string) (authUser, error) {
	cookie, err := r.Cookie("access_token")
	if err != nil {
		return authUser{}, err
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return authUser{}, jwt.ErrTokenInvalidClaims
	}

	sub, err := claims.GetSubject()
	if err != nil {
		return authUser{}, err
	}
	userID, err := strconv.ParseInt(sub, 10, 64)
	if err != nil {
		return authUser{}, err
	}
	role, _ := claims["role"].(string)

	return authUser{userID: userID, role: role}, nil
}

// JWTAuth reads the access_token cookie, validates it (HS256, signed with
// secret) and stores the authenticated user's id and role in the request
// context for downstream handlers via UserFromContext. Rejects with 401 on
// any missing/invalid/expired token.
func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := verifyJWTCookie(r, secret)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth populates the request context from the access_token cookie
// when one is present and valid, same as JWTAuth — but never rejects: a
// missing or invalid cookie just means the handler sees UserFromContext's
// !ok, not a 401. For routes that are deliberately reachable without a
// session (e.g. finances' public GET /categories) but still want to
// recognize a logged-in caller when there is one, instead of always
// treating everyone as anonymous.
func OptionalAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user, err := verifyJWTCookie(r, secret); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), userContextKey, user))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth authenticates a request via the access_token cookie first
// (same as JWTAuth), and falls back to a long-lived API key passed as
// Authorization: Bearer <key> when no valid cookie is present. This lets
// external automation (e.g. a Shortcut) hit the same protected endpoints as
// the browser without an interactive login. On success either way, stores
// the authenticated user's id and role in the request context via
// UserFromContext. Rejects with 401 if neither the cookie nor the key is
// valid.
func RequireAuth(db *sql.DB, jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user, err := verifyJWTCookie(r, jwtSecret); err == nil {
				ctx := context.WithValue(r.Context(), userContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				provided := strings.TrimPrefix(auth, "Bearer ")
				sum := sha256.Sum256([]byte(provided))
				hash := hex.EncodeToString(sum[:])

				var user authUser
				err := db.QueryRow(
					`SELECT api_keys.user_id, users.role FROM api_keys
					 JOIN users ON users.id = api_keys.user_id
					 WHERE api_keys.key_hash = $1 AND api_keys.revoked_at IS NULL`,
					hash,
				).Scan(&user.userID, &user.role)
				if err != nil && err != sql.ErrNoRows {
					log.Printf("middleware: api key lookup failed: %v", err)
				}
				if err == nil {
					// Best-effort — a failed timestamp update shouldn't fail
					// the request the key is authenticating, so its error is
					// only logged, not surfaced to the caller.
					if _, uerr := db.Exec(`UPDATE api_keys SET last_used_at = now() WHERE key_hash = $1`, hash); uerr != nil {
						log.Printf("middleware: api key last_used_at update failed: %v", uerr)
					}

					ctx := context.WithValue(r.Context(), userContextKey, user)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		})
	}
}

// UserFromContext returns the authenticated user's id and role stored by a
// preceding JWTAuth, and whether an authenticated user was present at all.
func UserFromContext(ctx context.Context) (userID int64, role string, ok bool) {
	u, ok := ctx.Value(userContextKey).(authUser)
	if !ok {
		return 0, "", false
	}
	return u.userID, u.role, true
}

// RequireRole 403s unless the authenticated user's role (set by a preceding
// JWTAuth) is in the allowed set. Used to gate admin-only routes like
// /api/db, and as the explicit allowlist on shared routes like /api/couple.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, role, ok := UserFromContext(r.Context())
			if !ok || !slices.Contains(roles, role) {
				httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CLITokenAuth validates a static CLI token passed as
// Authorization: Bearer <token>. Uses constant-time comparison (like ytdlp's
// public token) so the token's length/content don't leak through timing.
// On success, stores a minimal authUser in context (userID=1, role=admin).
func CLITokenAuth(token string) func(http.Handler) http.Handler {
	expectedHash := sha256.Sum256([]byte(token))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "missing token")
				return
			}
			provided := strings.TrimPrefix(auth, "Bearer ")
			providedHash := sha256.Sum256([]byte(provided))
			if subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) != 1 {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid token")
				return
			}
			// CLI callers get admin role by default (they have the secret token)
			ctx := context.WithValue(r.Context(), userContextKey, authUser{userID: 1, role: "admin"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// Unwrap lets http.ResponseController (used e.g. by ytdlp.Download to extend
// the write deadline for long streaming responses) reach the underlying
// ResponseWriter instead of stopping at this wrapper.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Hijack lets gorilla/websocket upgrade the connection (games.ServeWS).
// Unlike ytdlp's deadline extension above, gorilla/websocket type-asserts
// http.Hijacker directly against the ResponseWriter it's given — it doesn't
// go through http.ResponseController/Unwrap — so this wrapper has to
// implement Hijacker itself and forward to the underlying ResponseWriter,
// or every websocket upgrade fails with "does not implement http.Hijacker"
// the moment this Logger middleware wraps it.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}
