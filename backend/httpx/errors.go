package httpx

import "net/http"

// Error codes for structured API errors. Each code maps to a specific
// HTTP status and a user-facing message.
const (

	// Auth errors (401)
	CodeUnauthorized       = "AUTH_UNAUTHORIZED"
	CodeInvalidCredentials = "AUTH_INVALID_CREDENTIALS"
	CodeTokenExpired       = "AUTH_TOKEN_EXPIRED"
	CodeTokenInvalid       = "AUTH_TOKEN_INVALID"

	// Auth errors (403)
	CodeForbidden = "AUTH_FORBIDDEN"

	// Validation (400)
	CodeValidation = "VALIDATION_ERROR"
	CodeBadRequest = "BAD_REQUEST"

	// Not found (404)
	CodeNotFound = "NOT_FOUND"

	// Conflict (409)
	CodeConflict = "CONFLICT"

	// Server errors (5xx)
	CodeInternal           = "INTERNAL_ERROR"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// APIError is the standard error response body.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError writes a JSON error response using the given code and message.
func WriteError(w http.ResponseWriter, status int, code string, msg string) {
	WriteJSON(w, status, APIError{Code: code, Message: msg})
}
