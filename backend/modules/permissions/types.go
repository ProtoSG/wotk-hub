package permissions

// AllModules is the fixed, exhaustive set of modules an admin can grant to
// the guest — DB Manager and Configuración deliberately never appear here:
// DB Manager is raw SQL access to the whole database, and Configuración is
// where these permissions themselves are managed, so letting a guest reach
// either would defeat the point. Citas and Juegos aren't here either — they
// have no gate at all, every account can already use them.
//
// Keys match the frontend route paths (Sidebar.tsx/router/index.tsx) 1:1 so
// the frontend can index straight from a module key to a nav entry.
var AllModules = []string{"dashboard", "finances", "gym", "ytdlp"}

func isValidModule(m string) bool {
	for _, v := range AllModules {
		if v == m {
			return true
		}
	}
	return false
}

// ModulePermission is one module's grant state for the guest account.
type ModulePermission struct {
	Module  string `json:"module"`
	Enabled bool   `json:"enabled"`
}

type listGuestPermissionsResponse struct {
	Modules []ModulePermission `json:"modules"`
}

type minePermissionsResponse struct {
	// Modules the caller currently has access to — admin always gets
	// AllModules; guest gets whatever's enabled in module_permissions.
	Modules []string `json:"modules"`
}

// updateGuestRequest is a partial map — only the keys present get written,
// so the client can toggle one switch without resending the whole set.
type updateGuestRequest struct {
	Modules map[string]bool `json:"modules"`
}
