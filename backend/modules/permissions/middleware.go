package permissions

import (
	"database/sql"
	"log"
	"net/http"
	"workhub/httpx"
	"workhub/middleware"
)

// RequireModule 404s a guest whose module_permissions row for `module`
// isn't enabled — same "don't reveal it exists" convention as
// finances.cardOwned. Admin always passes through unchecked (this table
// only ever gates the guest account, see AllModules' comment).
//
// Applied at the route-mount level in main.go, after RequireRole/JWTAuth
// have already put a caller identity in context — this makes the
// Configuración toggle a real backend gate for gym/ytdlp, not just a hidden
// nav item a guest could bypass by calling the API directly.
func RequireModule(db *sql.DB, module string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, role, ok := middleware.UserFromContext(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
				return
			}
			if role == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			var enabled bool
			err := db.QueryRow(
				`SELECT enabled FROM module_permissions WHERE user_id = $1 AND module = $2`,
				userID, module,
			).Scan(&enabled)
			if err == sql.ErrNoRows || (err == nil && !enabled) {
				httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "not found")
				return
			}
			if err != nil {
				log.Printf("permissions: module check failed: %v", err)
				httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
