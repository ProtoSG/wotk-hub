package wshub

import (
	"net/http"
	"slices"

	"github.com/gorilla/websocket"
)

// NewUpgrader builds a websocket.Upgrader whose origin check accepts only
// the same allow-listed origins the app's CORS middleware reflects (see
// middleware.CORS / middleware.ParseAllowedOrigins). A WS handshake isn't
// subject to CORS at all (browsers don't preflight or block cross-origin
// Upgrade requests the way they do fetch/XHR), so this CheckOrigin is the
// one place that actually has to reject an unexpected Origin instead of
// just choosing not to echo it back in a header.
func NewUpgrader(allowedOrigins []string) *websocket.Upgrader {
	return &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			// No Origin header at all (non-browser client) — nothing to
			// check against, allow it same as a same-origin request.
			if origin == "" {
				return true
			}
			return slices.Contains(allowedOrigins, origin)
		},
	}
}
