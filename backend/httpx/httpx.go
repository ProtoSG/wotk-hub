// Package httpx provides small shared HTTP helpers (JSON response writing,
// request body decoding with size limits) used across backend modules so the
// logic isn't duplicated in every handlers.go.
package httpx

import (
	"encoding/json"
	"net/http"
)

// DefaultMaxBodyBytes caps request bodies to guard against oversized
// payloads exhausting memory. 1MB is generous for the JSON payloads this
// API accepts (connection params, transactions, queries, etc).
const DefaultMaxBodyBytes int64 = 1 << 20 // 1MB

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// SuccessResponse is the standard body for a write endpoint that has nothing
// else to report back beyond "it worked" (deletes, logout, etc).
type SuccessResponse struct {
	Success bool `json:"success"`
}

// WriteSuccess writes the standard {"success": true} body.
func WriteSuccess(w http.ResponseWriter, status int) {
	WriteJSON(w, status, SuccessResponse{Success: true})
}

// DecodeJSON decodes the JSON request body into dst, enforcing maxBytes as an
// upper bound on the body size via http.MaxBytesReader. Callers are
// responsible for writing an error response when a non-nil error is
// returned (e.g. a generic 400 "invalid request body").
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	return json.NewDecoder(r.Body).Decode(dst)
}
