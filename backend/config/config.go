package config

import "os"

type Config struct {
	Port        string
	CORSOrigin  string
	DatabaseURL string
	// JWTSecret signs and validates access/refresh JWTs (see
	// middleware.JWTAuth). Required — main() fails fast if it's empty,
	// unlike the old optional API key.
	JWTSecret string
	// CookieSecure controls the Secure flag on auth cookies. false for local
	// http dev (the default), set COOKIE_SECURE=true in production (https).
	CookieSecure bool
	// YtdlpPublicToken, when set, mounts an unauthenticated /api/ytdlp/public
	// route gated by this shared token instead of JWT login. Empty (default)
	// means the public route isn't mounted at all.
	YtdlpPublicToken string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}
	origin := os.Getenv("CORS_ORIGIN")
	if origin == "" {
		origin = "http://localhost:5173"
	}
	// DATABASE_URL has no fallback default — main() fails fast if it's
	// empty, same as JWTSecret below. Local dev sets it explicitly via
	// backend/.env (see docker-compose), so this doesn't change local dev;
	// it just removes an insecure implicit default with embedded
	// credentials for any other environment.
	dbURL := os.Getenv("DATABASE_URL")
	jwtSecret := os.Getenv("JWT_SECRET")
	cookieSecure := os.Getenv("COOKIE_SECURE") == "true"
	ytdlpPublicToken := os.Getenv("YTDLP_PUBLIC_TOKEN")
	return Config{
		Port:             port,
		CORSOrigin:       origin,
		DatabaseURL:      dbURL,
		JWTSecret:        jwtSecret,
		CookieSecure:     cookieSecure,
		YtdlpPublicToken: ytdlpPublicToken,
	}
}
