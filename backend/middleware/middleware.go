package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"log"
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

func CORS(origins string) func(http.Handler) http.Handler {
	allowed := strings.Split(origins, ",")
	for i := range allowed {
		allowed[i] = strings.TrimSpace(allowed[i])
	}
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

// JWTAuth reads the access_token cookie, validates it (HS256, signed with
// secret) and stores the authenticated user's id and role in the request
// context for downstream handlers via UserFromContext. Rejects with 401 on
// any missing/invalid/expired token.
func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("access_token")
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			claims := jwt.MapClaims{}
			token, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			sub, err := claims.GetSubject()
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			userID, err := strconv.ParseInt(sub, 10, 64)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			role, _ := claims["role"].(string)

			ctx := context.WithValue(r.Context(), userContextKey, authUser{userID: userID, role: role})
			next.ServeHTTP(w, r.WithContext(ctx))
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
				httpx.WriteError(w, http.StatusForbidden, "forbidden")
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
				httpx.WriteError(w, http.StatusUnauthorized, "missing token")
				return
			}
			provided := strings.TrimPrefix(auth, "Bearer ")
			providedHash := sha256.Sum256([]byte(provided))
			if subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) != 1 {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid token")
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
