// Package team resolves the shared identity every couple-scoped feature
// (daily riddle, pet, ...) groups its state under, so each new feature
// doesn't need to reinvent or duplicate this lookup.
package team

import "database/sql"

// ResolveTeamID returns the admin account's user id. This app only ever
// serves the one couple who owns it (see users.role's admin/guest check
// constraint), so "team" isn't per-caller, it's everyone who uses this app,
// anchored to a stable reference point instead of whichever partner
// happened to call first. Extracted from the riddle game (games.resolveTeamID)
// once the pet feature needed the exact same lookup.
func ResolveTeamID(db *sql.DB) (int64, error) {
	var id int64
	err := db.QueryRow(`SELECT id FROM users WHERE role = 'admin' ORDER BY id LIMIT 1`).Scan(&id)
	return id, err
}
