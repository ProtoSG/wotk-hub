package pet

// ActionStatus is one of the 5 daily care actions' status, from the shared
// team's perspective. Locked/UnlocksAtHour are always computed server-side
// (see care()'s unlock guard and respondState) so the frontend never does
// its own hour math — that would risk a client clock/timezone mismatch,
// where the backend is the only source of truth for "what time is it in
// Lima right now".
type ActionStatus struct {
	Done bool `json:"done"`
	// By is the display name (users.name) of whoever logged this action
	// today, nil if nobody has yet. Pointer rather than "" so the frontend
	// can tell "not done" apart from a name that happened to be empty.
	By     *string `json:"by,omitempty"`
	Locked bool    `json:"locked"`
	// Missed is true once this action's window has closed (current Lima
	// hour past its deadlineHour — see careActions) and it's still undone
	// today. Doesn't lock the action out — it's still tappable for its
	// normal boost — but the frontend uses this to flag it instead of
	// showing it as a plain still-open turn: care_score already took the
	// missed-window penalty for it (see applyMissedWindowDecay).
	Missed        bool `json:"missed"`
	UnlocksAtHour int  `json:"unlocksAtHour"`
}

// State is the shared couple pet's current status. CareScore itself isn't
// exposed as a raw number in the UI by design (kept as mood only, not a
// score to optimize) — it's still serialized here since it's cheap and a
// future admin/debug view might want it; the frontend just doesn't surface
// it as a number.
type State struct {
	CareScore int    `json:"careScore"`
	Mood      string `json:"mood"` // happy | neutral | sad | hungry
	// Name is "" until the shop's rename item is bought at least once — the
	// frontend falls back to a generic title while empty.
	Name string `json:"name"`
	// Streak is consecutive days with at least one care action. Resets to 0
	// the moment a full day passes with none — unless StreakFreezes > 0 at
	// that moment, in which case one freeze is consumed and the streak
	// survives instead (see loadOrCreate).
	Streak int `json:"streak"`
	// Sparks is shop currency, earned on PerfectDay. Unlike CareScore this
	// IS meant to be shown as a number — see the sparks column comment in
	// migrate.go for why that's not a contradiction of the "no raw stats"
	// rule above.
	Sparks int `json:"sparks"`
	// StreakFreezes is how many streak-protection tokens the team currently
	// owns, bought in the shop.
	StreakFreezes int `json:"streakFreezes"`
	// FreezeJustConsumed is true only in the one response where a freeze was
	// spent to protect the streak from a neglect gap — see petRow's
	// freezeJustConsumed. The frontend uses this to toast it once instead of
	// the spend happening silently.
	FreezeJustConsumed bool `json:"freezeJustConsumed"`
	// PerfectDay is true once all 5 actions are done today.
	PerfectDay bool         `json:"perfectDay"`
	Bathe      ActionStatus `json:"bathe"`
	Breakfast  ActionStatus `json:"breakfast"`
	Lunch      ActionStatus `json:"lunch"`
	Play       ActionStatus `json:"play"`
	Dinner     ActionStatus `json:"dinner"`
}

type stateResponse struct {
	Pet State `json:"pet"`
}
