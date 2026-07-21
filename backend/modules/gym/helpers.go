package gym

import (
	"fmt"
	"strconv"
)

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
