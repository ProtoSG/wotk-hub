package auth

import "fmt"

// User is the auth-safe view of a users row (never includes password_hash).
type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// adminUserView is the row shape ListUsers returns — includes created_at,
// which the auth-safe User (used everywhere else) intentionally omits.
type adminUserView struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

type deleteUserResponse struct {
	Deleted bool `json:"deleted"`
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r registerRequest) validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.Email == "" {
		return fmt.Errorf("email is required")
	}
	if len(r.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}

// createAPIKeyRequest is the body for POST /auth/keys. Name is optional —
// it's just a label to tell keys apart later, defaults to ”.
type createAPIKeyRequest struct {
	Name string `json:"name"`
}

// apiKeyCreated is the response for POST /auth/keys. Key is the raw,
// unhashed value — it is returned only this once and cannot be recovered
// later, since only its hash is persisted.
type apiKeyCreated struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	CreatedAt string `json:"created_at"`
}

// apiKeyView is the row shape GET /auth/keys returns — never the hash or
// the raw key, only enough to identify and manage a key.
type apiKeyView struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
	RevokedAt  *string `json:"revoked_at"`
}

type revokeAPIKeyResponse struct {
	Revoked bool `json:"revoked"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r loginRequest) validate() error {
	if r.Email == "" || r.Password == "" {
		return fmt.Errorf("email and password are required")
	}
	return nil
}
