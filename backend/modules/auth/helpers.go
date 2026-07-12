package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenTTL    = 15 * time.Minute
	refreshTokenTTL   = 30 * 24 * time.Hour
	refreshCookiePath = "/api/auth/refresh"
)

// newAccessToken signs a short-lived HS256 JWT carrying the user's id (as the
// standard "sub" claim) and role. middleware.JWTAuth validates and reads it
// back on every protected request.
func newAccessToken(secret string, userID int64, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  strconv.FormatInt(userID, 10),
		"role": role,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(accessTokenTTL).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// newRefreshToken returns a random opaque token (the raw value sent to the
// client as a cookie) and its SHA-256 hash (the only form ever persisted).
func newRefreshToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func setAccessCookie(w http.ResponseWriter, secure bool, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(accessTokenTTL.Seconds()),
	})
}

func setRefreshCookie(w http.ResponseWriter, secure bool, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    value,
		Path:     refreshCookiePath,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(refreshTokenTTL.Seconds()),
	})
}

// clearAuthCookies expires both cookies. The refresh cookie must be cleared
// with the same Path it was set with, or the browser won't match it.
func clearAuthCookies(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: "access_token", Value: "", Path: "/", HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name: "refresh_token", Value: "", Path: refreshCookiePath, HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// issueTokens signs a fresh access token and stores+cookies a new refresh
// token for userID/role. Used by Register, Login and Refresh.
func (h *handler) issueTokens(w http.ResponseWriter, userID int64, role string) error {
	access, err := newAccessToken(h.secret, userID, role)
	if err != nil {
		return err
	}
	raw, hash, err := newRefreshToken()
	if err != nil {
		return err
	}
	_, err = h.db.Exec(
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, hash, time.Now().Add(refreshTokenTTL),
	)
	if err != nil {
		return err
	}
	setAccessCookie(w, h.secure, access)
	setRefreshCookie(w, h.secure, raw)
	return nil
}
