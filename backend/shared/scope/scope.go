// Package scope holds the per-user ownership-scoping helper shared by every
// module that has its own personal (not household-shared) rows — finances
// and gym so far. One copy instead of one per module, so the deny-by-default
// semantics stay identical everywhere it's used.
package scope

import "strconv"

// ToOwner appends "AND created_by = $N" (and its arg) to query when role
// isn't "admin" — deny-by-default, so any non-admin role only ever sees its
// own rows. Admins see everything unscoped, including legacy rows with a
// NULL created_by.
//
// query must already end in a boolean-context clause (WHERE ... or a JOIN's
// ON ...) for the appended AND to be valid SQL — most callers already have
// one (a soft-delete check, an id filter); a query with no natural WHERE can
// start with "WHERE true" as an anchor.
func ToOwner(query string, args []any, role string, userID int64) (string, []any) {
	if role == "admin" {
		return query, args
	}
	args = append(args, userID)
	return query + " AND created_by = $" + strconv.Itoa(len(args)), args
}
