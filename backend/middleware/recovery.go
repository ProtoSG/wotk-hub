package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"workhub/httpx"
)

// Recovery recovers from panics and returns a structured JSON error response
// so the client never sees a blank connection.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v\n%s", err, debug.Stack())
				httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
