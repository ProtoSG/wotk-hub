package gym

import (
	"fmt"
	"net/http"
	"strconv"

	chi "github.com/go-chi/chi/v5"
)

// parseID reads a positive int64 URL param. Unlike finances.parseID it takes
// the param name, since gym has nested routes with two of them.
func parseID(r *http.Request, param string) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, param), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid %s", param)
	}
	return id, nil
}

// parseSessionExerciseIDs reads the {id}/{exerciseId} pair shared by the
// nested session-exercise routes.
func parseSessionExerciseIDs(r *http.Request) (sessionID, sessionExerciseID int64, err error) {
	sessionID, err = parseID(r, "id")
	if err != nil {
		return 0, 0, err
	}
	sessionExerciseID, err = parseID(r, "exerciseId")
	if err != nil {
		return 0, 0, err
	}
	return sessionID, sessionExerciseID, nil
}

// parsePaging validates ?limit= and ?offset=. Empty values fall back to the
// default page size and the first page; limit is capped so a client can't ask
// for the whole catalog in one response by accident.
func parsePaging(limitParam, offsetParam string) (int, int, error) {
	limit := defaultLimit
	if limitParam != "" {
		n, err := strconv.Atoi(limitParam)
		if err != nil || n <= 0 {
			return 0, 0, fmt.Errorf("invalid limit")
		}
		limit = n
		if limit > maxLimit {
			limit = maxLimit
		}
	}

	offset := 0
	if offsetParam != "" {
		n, err := strconv.Atoi(offsetParam)
		if err != nil || n < 0 {
			return 0, 0, fmt.Errorf("invalid offset")
		}
		offset = n
	}
	return limit, offset, nil
}
