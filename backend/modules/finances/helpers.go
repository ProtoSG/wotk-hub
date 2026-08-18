package finances

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
	"workhub/shared/scope"

	chi "github.com/go-chi/chi/v5"
)

// itoa is a tiny alias used when building numbered SQL placeholders.
func itoa(n int) string {
	return strconv.Itoa(n)
}

func parseID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}

// monthRange parses "YYYY-MM" and returns [start, end) month bounds.
// Empty month defaults to the current month.
func monthRange(month string) (time.Time, time.Time, error) {
	if month == "" {
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0), nil
	}
	start, err := time.Parse("2006-01", month)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid month: %s", month)
	}
	return start, start.AddDate(0, 1, 0), nil
}

// scopeToOwner is finances' name for the shared scope.ToOwner helper (see
// backend/shared/scope) — kept as a package-local wrapper so the ~20
// existing call sites across this module don't need to change.
func scopeToOwner(query string, args []any, role string, userID int64) (string, []any) {
	return scope.ToOwner(query, args, role, userID)
}
