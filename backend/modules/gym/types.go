package gym

// Exercise is a catalog entry. Seeded rows (is_custom = false) come from the
// bundled CSV; user-created ones are flagged is_custom = true so the seeder
// never touches them.
type Exercise struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Equipment       string `json:"equipment"`
	PrimaryMuscle   string `json:"primaryMuscle"`
	SecondaryMuscle string `json:"secondaryMuscle"`
	MediaURL        string `json:"mediaUrl"`
	MediaType       string `json:"mediaType"`
	IsCustom        bool   `json:"isCustom"`
}

type listExercisesResponse struct {
	Exercises []Exercise `json:"exercises"`
	Total     int64      `json:"total"`
}

// exerciseFiltersResponse feeds the picker's dropdowns with the values that
// actually exist in the catalog, instead of a hardcoded frontend list that
// would drift as custom exercises are added.
type exerciseFiltersResponse struct {
	Muscles   []string `json:"muscles"`
	Equipment []string `json:"equipment"`
}
