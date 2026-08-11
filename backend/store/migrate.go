package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// Migrate runs the app schema at startup. There is no migration tool:
// schema changes are appended as new idempotent statements
// (CREATE TABLE IF NOT EXISTS / ALTER TABLE ... ADD COLUMN IF NOT EXISTS).
func Migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS transactions (
			id           BIGSERIAL PRIMARY KEY,
			type         TEXT   NOT NULL CHECK (type IN ('income','expense')),
			amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
			category     TEXT   NOT NULL,
			description  TEXT   NOT NULL DEFAULT '',
			occurred_on  DATE   NOT NULL,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_occurred_on ON transactions (occurred_on)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions (category)`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			id              BIGSERIAL PRIMARY KEY,
			name            TEXT    NOT NULL,
			amount_cents    BIGINT  NOT NULL CHECK (amount_cents > 0),
			frequency       TEXT    NOT NULL CHECK (frequency IN ('weekly','monthly','yearly')),
			category        TEXT    NOT NULL,
			next_billing_on DATE    NOT NULL,
			active          BOOLEAN NOT NULL DEFAULT true,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS budgets (
			id                  BIGSERIAL PRIMARY KEY,
			category            TEXT   NOT NULL UNIQUE,
			monthly_limit_cents BIGINT NOT NULL CHECK (monthly_limit_cents > 0),
			created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS couple_dates (
			id           BIGSERIAL PRIMARY KEY,
			occurred_on  DATE   NOT NULL,
			place        TEXT   NOT NULL DEFAULT '',
			category     TEXT   NOT NULL,
			notes        TEXT   NOT NULL DEFAULT '',
			cost_cents   BIGINT,
			rating       SMALLINT CHECK (rating BETWEEN 1 AND 5),
			tiktok_url   TEXT   NOT NULL DEFAULT '',
			created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
			CHECK (cost_cents IS NULL OR cost_cents >= 0)
		)`,
		`ALTER TABLE couple_dates ADD COLUMN IF NOT EXISTS tiktok_url TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_couple_dates_occurred_on ON couple_dates (occurred_on)`,
		`CREATE TABLE IF NOT EXISTS users (
			id            BIGSERIAL PRIMARY KEY,
			email         TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			name          TEXT NOT NULL,
			role          TEXT NOT NULL CHECK (role IN ('admin','guest')),
			created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			id         BIGSERIAL PRIMARY KEY,
			user_id    BIGINT NOT NULL REFERENCES users(id),
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			revoked_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		// Long-lived API keys: an alternative to the cookie-JWT session for
		// external automation (e.g. an iOS/macOS Shortcut) that can't do an
		// interactive login. Same shape as refresh_tokens (hash-only storage,
		// nullable revoked_at), minus expiry — these are meant to live
		// indefinitely until manually revoked.
		`CREATE TABLE IF NOT EXISTS api_keys (
			id           BIGSERIAL PRIMARY KEY,
			user_id      BIGINT NOT NULL REFERENCES users(id),
			name         TEXT NOT NULL DEFAULT '',
			key_hash     TEXT NOT NULL UNIQUE,
			last_used_at TIMESTAMPTZ,
			revoked_at   TIMESTAMPTZ,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS created_by BIGINT REFERENCES users(id)`,
		`ALTER TABLE couple_dates ADD COLUMN IF NOT EXISTS created_by BIGINT REFERENCES users(id)`,
		`ALTER TABLE couple_dates ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'done' CHECK (status IN ('planned','done'))`,
		`CREATE TABLE IF NOT EXISTS cards (
			id            BIGSERIAL PRIMARY KEY,
			name          TEXT   NOT NULL,
			type          TEXT   NOT NULL CHECK (type IN ('debito','credito','prepago')),
			bank          TEXT   NOT NULL DEFAULT '',
			last4         TEXT   NOT NULL DEFAULT '',
			color         TEXT   NOT NULL DEFAULT '#3B82F6',
			icon          TEXT   NOT NULL DEFAULT 'credit-card',
			balance_cents BIGINT NOT NULL DEFAULT 0,
			created_by    BIGINT REFERENCES users(id),
			created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS card_reloads (
			id            BIGSERIAL PRIMARY KEY,
			card_id       BIGINT NOT NULL REFERENCES cards(id),
			amount_cents  BIGINT NOT NULL CHECK (amount_cents > 0),
			occurred_on   DATE   NOT NULL,
			note          TEXT   NOT NULL DEFAULT '',
			created_by    BIGINT REFERENCES users(id),
			created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_card_reloads_card_id ON card_reloads (card_id)`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS card_id BIGINT REFERENCES cards(id)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_card_id ON transactions (card_id)`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS credit_limit_cents BIGINT NOT NULL DEFAULT 0`,
		// initial_balance_cents and used_credit_cents used to be added here
		// too, but both are dropped further down (see "Card balance/used-
		// credit stop being stored" below) — on every boot (migrate has no
		// version tracking, see the package comment) that made this an
		// unconditional add-then-drop, permanently burning a column slot on
		// `cards` each run since Postgres never reclaims a dropped column's
		// attnum. Enough boots (redeploys, restarts, crash loops) exhausted
		// the 1600-column-per-table hard limit. Removed rather than left as
		// a no-op ADD, since the later DROP ... IF EXISTS already handles a
		// database that still has them from before this fix.
		`CREATE TABLE IF NOT EXISTS savings_goals (
			id            BIGSERIAL PRIMARY KEY,
			name          TEXT   NOT NULL,
			target_cents  BIGINT NOT NULL CHECK (target_cents > 0),
			current_cents BIGINT NOT NULL DEFAULT 0,
			deadline      DATE,
			icon          TEXT   NOT NULL DEFAULT 'piggy-bank',
			color         TEXT   NOT NULL DEFAULT '#10b981',
			created_by    BIGINT REFERENCES users(id),
			created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS savings_contributions (
			id            BIGSERIAL PRIMARY KEY,
			goal_id       BIGINT NOT NULL REFERENCES savings_goals(id),
			amount_cents  BIGINT NOT NULL CHECK (amount_cents > 0),
			occurred_on   DATE   NOT NULL,
			note          TEXT   NOT NULL DEFAULT '',
			created_by    BIGINT REFERENCES users(id),
			created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_savings_contributions_goal_id ON savings_contributions (goal_id)`,
		`ALTER TABLE savings_goals ADD COLUMN IF NOT EXISTS default_card_id BIGINT REFERENCES cards(id)`,
		// Deleting a goal is meant to delete its contributions with it (see
		// DeleteGoal) — the original FK had no ON DELETE action, so it just
		// blocked the delete instead.
		`ALTER TABLE savings_contributions DROP CONSTRAINT IF EXISTS savings_contributions_goal_id_fkey`,
		`ALTER TABLE savings_contributions ADD CONSTRAINT savings_contributions_goal_id_fkey
			FOREIGN KEY (goal_id) REFERENCES savings_goals(id) ON DELETE CASCADE`,
		// Soft delete for cards, savings_goals, and transactions: each is
		// either referenced by other tables (cards, savings_goals) or is the
		// financial ledger itself (transactions), so a hard delete either
		// fights FK constraints or destroys audit history. Nullable
		// timestamp, same pattern as refresh_tokens.revoked_at — NULL means
		// active, non-null means deleted (and when).
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`,
		`ALTER TABLE savings_goals ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`,
		// Unified transfer ledger: card reload, goal contribution, and
		// card-to-card transfer all become type='transfer' transactions
		// instead of three separate hand-mutated code paths. See SPEC.md.
		`ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_type_check`,
		`ALTER TABLE transactions ADD CONSTRAINT transactions_type_check CHECK (type IN ('income','expense','transfer'))`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS from_card_id BIGINT REFERENCES cards(id)`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS to_card_id BIGINT REFERENCES cards(id)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_from_card_id ON transactions (from_card_id)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_to_card_id ON transactions (to_card_id)`,
		// Card balance/used-credit stop being stored — computed live from
		// transactions (see cardBalance in transactions.go).
		// initial_balance_cents is replaced by a seed transfer at
		// card-creation time.
		`ALTER TABLE cards DROP COLUMN IF EXISTS balance_cents`,
		`ALTER TABLE cards DROP COLUMN IF EXISTS initial_balance_cents`,
		`ALTER TABLE cards DROP COLUMN IF EXISTS used_credit_cents`,
		// card_reloads is replaced by transactions WHERE type='transfer'
		// AND from_card_id IS NULL.
		`DROP TABLE IF EXISTS card_reloads`,
		// A goal without a card was pure bookkeeping with no ledger effect
		// — every goal is now a real transfer target. Requires clean data
		// (no existing NULL default_card_id rows).
		`ALTER TABLE savings_goals ALTER COLUMN default_card_id SET NOT NULL`,
		// Links each contribution to the transfer transaction that backed
		// it, so the card used is known permanently even if the goal's
		// default card changes later.
		`ALTER TABLE savings_contributions ADD COLUMN IF NOT EXISTS transaction_id BIGINT REFERENCES transactions(id)`,
		// Optional — a subscription's auto-charge tags the generated
		// expense to this card (see processDue in subscriptions.go).
		// Nullable: subscriptions with no card just generate an untagged
		// expense, same as today.
		`ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS card_id BIGINT REFERENCES cards(id)`,
		// Mandatory card account model: income/expense transactions must
		// be tagged to a card (card_id NOT NULL), while transfer rows
		// (reload, goal contribution, card-to-card) legitimately leave
		// card_id NULL — from_card_id/to_card_id carry the pair instead.
		// A column-wide NOT NULL would break every transfer, so a CHECK
		// encodes the real invariant. Subscriptions have no transfer path
		// → straight NOT NULL. Truncate-first / no-backfill: finance
		// tables are truncated before deploy so both constraints apply on
		// clean data (any legacy NULL card_id rows are LOST, by
		// convention).
		`ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_card_id_required_for_income_expense`,
		`ALTER TABLE transactions ADD CONSTRAINT transactions_card_id_required_for_income_expense
			CHECK (type = 'transfer' OR card_id IS NOT NULL)`,
		`ALTER TABLE subscriptions ALTER COLUMN card_id SET NOT NULL`,
		// card_reloads was already dropped at the unified-transfer-ledger
		// migration above; DROP IF EXISTS is kept here as an idempotent
		// safety net so a partially-migrated DB still converges.
		`DROP TABLE IF EXISTS card_reloads`,
		// Cards are now type-agnostic: no more debito/credito/prepago
		// distinction. Credit tracking is inferred purely from
		// credit_limit_cents > 0 (see cardBalance in transactions.go).
		`ALTER TABLE cards DROP COLUMN IF EXISTS type`,
		// Categories move from hardcoded Go slices to a real table so
		// they're user-manageable (CRUD via UI, see
		// modules/finances/categories.go). name is only unique per kind,
		// not globally — "otros" legitimately exists as both an expense
		// and an income category, same as the hardcoded lists it
		// replaces. transactions/subscriptions/budgets.category stay
		// plain TEXT (no FK): validated against this table at write
		// time instead, same tradeoff already made for those columns.
		`CREATE TABLE IF NOT EXISTS categories (
			id         BIGSERIAL PRIMARY KEY,
			name       TEXT NOT NULL,
			kind       TEXT NOT NULL CHECK (kind IN ('income','expense')),
			label      TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE (name, kind)
		)`,
		`INSERT INTO categories (name, kind, label) VALUES
			('comida', 'expense', 'Comida'),
			('transporte', 'expense', 'Transporte'),
			('vivienda', 'expense', 'Vivienda'),
			('servicios', 'expense', 'Servicios'),
			('salud', 'expense', 'Salud'),
			('educacion', 'expense', 'Educación'),
			('entretenimiento', 'expense', 'Entretenimiento'),
			('ropa', 'expense', 'Ropa'),
			('suscripciones', 'expense', 'Suscripciones'),
			('otros', 'expense', 'Otros'),
			('sueldo', 'income', 'Sueldo'),
			('freelance', 'income', 'Freelance'),
			('inversiones', 'income', 'Inversiones'),
			('regalo', 'income', 'Regalo'),
			('otros', 'income', 'Otros')
			ON CONFLICT (name, kind) DO NOTHING`,
		// Gym module — see GYM_SPEC.md. The catalog is seeded from a bundled
		// CSV (modules/gym/data/exercises.csv) by gym.SeedExercises, which
		// matches on name, so name carries a UNIQUE constraint. Equipment and
		// muscle columns are free TEXT rather than CHECK enums: user-created
		// exercises may introduce new values, and a CHECK would need an ALTER
		// for each one — a bad fit for this append-only migration style.
		`CREATE TABLE IF NOT EXISTS exercises (
			id               BIGSERIAL PRIMARY KEY,
			name             TEXT NOT NULL UNIQUE,
			equipment        TEXT NOT NULL DEFAULT '',
			primary_muscle   TEXT NOT NULL DEFAULT 'Other',
			secondary_muscle TEXT NOT NULL DEFAULT '',
			media_url        TEXT NOT NULL DEFAULT '',
			media_type       TEXT NOT NULL DEFAULT '',
			is_custom        BOOLEAN NOT NULL DEFAULT false,
			created_by       BIGINT REFERENCES users(id),
			created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		// Spanish how-to text, seeded from a companion CSV. Kept out of the
		// source catalog file so re-importing that file never touches it.
		`ALTER TABLE exercises ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT ''`,
		// How an exercise is logged. Most are weight x reps, but cardio is
		// distance and time, and an isometric hold is time alone — forcing
		// those into reps records a number that means nothing. Free TEXT
		// rather than a CHECK, same reasoning as the muscle columns.
		`ALTER TABLE exercises ADD COLUMN IF NOT EXISTS tracking_type TEXT NOT NULL DEFAULT 'weight_reps'`,
		`CREATE INDEX IF NOT EXISTS idx_exercises_primary_muscle ON exercises (primary_muscle)`,
		`CREATE INDEX IF NOT EXISTS idx_exercises_equipment ON exercises (equipment)`,
		`CREATE TABLE IF NOT EXISTS routines (
			id         BIGSERIAL PRIMARY KEY,
			name       TEXT NOT NULL,
			notes      TEXT NOT NULL DEFAULT '',
			color      TEXT NOT NULL DEFAULT '#3B82F6',
			icon       TEXT NOT NULL DEFAULT 'dumbbell',
			archived   BOOLEAN NOT NULL DEFAULT false,
			created_by BIGINT REFERENCES users(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS routine_exercises (
			id          BIGSERIAL PRIMARY KEY,
			routine_id  BIGINT NOT NULL REFERENCES routines(id) ON DELETE CASCADE,
			exercise_id BIGINT NOT NULL REFERENCES exercises(id),
			position    INT NOT NULL,
			target_sets INT NOT NULL DEFAULT 3 CHECK (target_sets > 0),
			target_reps INT NOT NULL DEFAULT 10 CHECK (target_reps > 0),
			notes       TEXT NOT NULL DEFAULT '',
			UNIQUE (routine_id, position)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_routine_exercises_routine_id ON routine_exercises (routine_id)`,
		// routine_id is ON DELETE SET NULL and name snapshots the routine's
		// name at start time: editing or deleting a template must never
		// rewrite what a past session recorded.
		`CREATE TABLE IF NOT EXISTS workout_sessions (
			id          BIGSERIAL PRIMARY KEY,
			routine_id  BIGINT REFERENCES routines(id) ON DELETE SET NULL,
			name        TEXT NOT NULL DEFAULT '',
			occurred_on DATE NOT NULL,
			started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			finished_at TIMESTAMPTZ,
			notes       TEXT NOT NULL DEFAULT '',
			created_by  BIGINT REFERENCES users(id),
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workout_sessions_occurred_on ON workout_sessions (occurred_on)`,
		`CREATE TABLE IF NOT EXISTS session_exercises (
			id          BIGSERIAL PRIMARY KEY,
			session_id  BIGINT NOT NULL REFERENCES workout_sessions(id) ON DELETE CASCADE,
			exercise_id BIGINT NOT NULL REFERENCES exercises(id),
			position    INT NOT NULL,
			notes       TEXT NOT NULL DEFAULT '',
			UNIQUE (session_id, position)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_session_exercises_session_id ON session_exercises (session_id)`,
		// Indexed on exercise_id because the progress chart query filters by
		// it across every session.
		`CREATE INDEX IF NOT EXISTS idx_session_exercises_exercise_id ON session_exercises (exercise_id)`,
		// weight_grams (not kg floats) for the same reason finances stores
		// amount_cents: 2.5 kg increments and lb conversions stay exact. 0 is
		// meaningful — bodyweight exercises. reps >= 0 allows a failed set.
		`CREATE TABLE IF NOT EXISTS exercise_sets (
			id                  BIGSERIAL PRIMARY KEY,
			session_exercise_id BIGINT NOT NULL REFERENCES session_exercises(id) ON DELETE CASCADE,
			set_number          INT    NOT NULL CHECK (set_number > 0),
			reps                INT    NOT NULL CHECK (reps >= 0),
			weight_grams        BIGINT NOT NULL DEFAULT 0 CHECK (weight_grams >= 0),
			is_warmup           BOOLEAN NOT NULL DEFAULT false,
			completed           BOOLEAN NOT NULL DEFAULT true,
			UNIQUE (session_exercise_id, set_number)
		)`,
		// Nullable in spirit, zero in practice: a weight_reps set leaves both
		// at 0, and they live on exercise_sets rather than a separate cardio
		// table so progress stays one query and hybrids (Farmers Walk carries
		// weight AND distance, Sled Push too) have a home.
		`ALTER TABLE exercise_sets ADD COLUMN IF NOT EXISTS duration_seconds INT NOT NULL DEFAULT 0`,
		`ALTER TABLE exercise_sets ADD COLUMN IF NOT EXISTS distance_meters INT NOT NULL DEFAULT 0`,
		`ALTER TABLE exercise_sets DROP CONSTRAINT IF EXISTS exercise_sets_duration_seconds_check`,
		`ALTER TABLE exercise_sets ADD CONSTRAINT exercise_sets_duration_seconds_check CHECK (duration_seconds >= 0)`,
		`ALTER TABLE exercise_sets DROP CONSTRAINT IF EXISTS exercise_sets_distance_meters_check`,
		`ALTER TABLE exercise_sets ADD CONSTRAINT exercise_sets_distance_meters_check CHECK (distance_meters >= 0)`,
		`CREATE INDEX IF NOT EXISTS idx_exercise_sets_session_exercise_id ON exercise_sets (session_exercise_id)`,
		// Emoji Movie Game — see modules/games/.
		`CREATE TABLE IF NOT EXISTS emoji_movies (
			id         BIGSERIAL PRIMARY KEY,
			emoji_str  TEXT NOT NULL,
			answer     TEXT NOT NULL,
			difficulty TEXT NOT NULL CHECK (difficulty IN ('easy','medium','hard')),
			created_by BIGINT REFERENCES users(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS emoji_game_sessions (
			id             BIGSERIAL PRIMARY KEY,
			player1_id     BIGINT NOT NULL REFERENCES users(id),
			player2_id     BIGINT REFERENCES users(id),
			p1_score       INT    NOT NULL DEFAULT 0,
			p2_score       INT    NOT NULL DEFAULT 0,
			current_emoji  TEXT   NOT NULL DEFAULT '',
			current_answer TEXT   NOT NULL DEFAULT '',
			status         TEXT   NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting','active','finished')),
			created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_emoji_game_sessions_player1_id ON emoji_game_sessions (player1_id)`,
		`CREATE INDEX IF NOT EXISTS idx_emoji_game_sessions_player2_id ON emoji_game_sessions (player2_id)`,
		// object_key is the durable pointer into MinIO; ON DELETE CASCADE
		// only reaches this row when its couple_dates parent is deleted, not
		// the actual MinIO object — SQL can't reach into object storage, so
		// the couple DeleteDate handler deletes each photo's MinIO object
		// itself before the SQL delete cascades these rows away.
		`CREATE TABLE IF NOT EXISTS couple_date_photos (
			id          BIGSERIAL PRIMARY KEY,
			date_id     BIGINT NOT NULL REFERENCES couple_dates(id) ON DELETE CASCADE,
			object_key  TEXT NOT NULL,
			created_by  BIGINT REFERENCES users(id),
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_couple_date_photos_date_id ON couple_date_photos (date_id)`,
		// Server-generated thumbnail variant, added alongside the existing
		// full-size object_key (see couple/photos.go UploadPhoto). Nullable,
		// no default: rows inserted before this change have NULL here, and
		// ListPhotos falls back to presigning object_key itself as the
		// thumbnail for those. This is a clean, permanent addition — no
		// paired DROP anywhere in this file. See the comment above the
		// `cards.credit_limit_cents` ALTER for why that matters: an
		// ADD-then-DROP pair for the same column, both in this statement
		// list, silently burns a permanent attnum slot on every boot
		// (migrate has no version tracking and re-runs every statement on
		// every boot) until the table hits Postgres's 1600-column hard
		// limit — that took prod down once already.
		`ALTER TABLE couple_date_photos ADD COLUMN IF NOT EXISTS thumbnail_object_key TEXT`,
		// La Última Pregunta — daily riddle game for couples.
		`CREATE TABLE IF NOT EXISTS daily_riddles (
			id            BIGSERIAL PRIMARY KEY,
			question      TEXT    NOT NULL,
			answer        TEXT    NOT NULL,
			hint          TEXT    NOT NULL DEFAULT '',
			difficulty    TEXT    NOT NULL CHECK (difficulty IN ('easy','medium','hard')),
			published_on  DATE    NOT NULL UNIQUE,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS riddle_game_sessions (
			id                BIGSERIAL PRIMARY KEY,
			team_id           BIGINT  NOT NULL REFERENCES users(id),
			partner_id        BIGINT  NOT NULL REFERENCES users(id),
			lives_remaining   INT     NOT NULL DEFAULT 3 CHECK (lives_remaining >= 0),
			p1_score          INT     NOT NULL DEFAULT 0,
			p2_score          INT     NOT NULL DEFAULT 0,
			current_riddle_id BIGINT REFERENCES daily_riddles(id),
			status            TEXT    NOT NULL DEFAULT 'active',
			created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS riddle_attempts (
			id            BIGSERIAL PRIMARY KEY,
			session_id    BIGINT  NOT NULL REFERENCES riddle_game_sessions(id),
			riddle_id     BIGINT  NOT NULL REFERENCES daily_riddles(id),
			solver_id     BIGINT  NOT NULL REFERENCES users(id),
			solved_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
			points_earned INT     NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_riddle_game_sessions_team_id ON riddle_game_sessions (team_id)`,
		`CREATE INDEX IF NOT EXISTS idx_riddle_attempts_session_id ON riddle_attempts (session_id)`,
		// One session per team per riddle — GetRiddleSession relies on this
		// for its ON CONFLICT upsert. Without it, every poll tick inserted a
		// fresh 'active' row instead of reusing the day's session, which both
		// hid solved/scored state behind the next freshly-created row and
		// ran the table up by thousands of rows per minute.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_riddle_game_sessions_team_riddle_uniq
			ON riddle_game_sessions (team_id, current_riddle_id)`,
		// Consecutive days solved without a miss — carries forward day to day
		// like lives/scores, resets to 0 the moment a day expires unsolved.
		`ALTER TABLE riddle_game_sessions ADD COLUMN IF NOT EXISTS streak INT NOT NULL DEFAULT 0`,
		// One row per browser/device a user has enabled push notifications
		// on (Web Push subscriptions aren't reusable across devices).
		// UNIQUE(user_id, endpoint) makes re-subscribing idempotent — the
		// same device re-registering (e.g. after clearing permissions and
		// re-granting) just updates its keys instead of creating a
		// duplicate row the reminder job would double-send to.
		`CREATE TABLE IF NOT EXISTS push_subscriptions (
			id         BIGSERIAL PRIMARY KEY,
			user_id    BIGINT NOT NULL REFERENCES users(id),
			endpoint   TEXT   NOT NULL,
			p256dh     TEXT   NOT NULL,
			auth       TEXT   NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE (user_id, endpoint)
		)`,
		// Shared couple pet — one row per team (see shared/team.ResolveTeamID),
		// not per user. care_score decays lazily on read (same pattern as the
		// riddle's expireStaleSessions — no separate cron needed for decay
		// itself). last_fed_on/last_played_on/last_cleaned_on are superseded
		// by pet_care_log below (5 actions with per-action attribution now,
		// not 3 anonymous booleans) — left in place as harmless unused legacy
		// columns per this file's additive-only migration style, no longer
		// read or written by Go code.
		`CREATE TABLE IF NOT EXISTS pet_state (
			id              BIGSERIAL PRIMARY KEY,
			team_id         BIGINT NOT NULL REFERENCES users(id),
			care_score      INT    NOT NULL DEFAULT 70 CHECK (care_score BETWEEN 0 AND 100),
			last_fed_on     DATE,
			last_played_on  DATE,
			last_cleaned_on DATE,
			last_updated_on DATE   NOT NULL DEFAULT CURRENT_DATE,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE (team_id)
		)`,
		// Consecutive days with at least one care action — same shape as the
		// riddle game's streak: +1 on the day's first action, resets to 0
		// the moment a full day passes with none (i.e. exactly when decay
		// fires).
		`ALTER TABLE pet_state ADD COLUMN IF NOT EXISTS streak INT NOT NULL DEFAULT 0`,
		// Per-action, per-day care log — replaces pet_state's 3 anonymous
		// last_*_on columns now that there are 5 actions and each needs to
		// remember WHO did it, not just that it happened. The UNIQUE
		// constraint is the idempotency mechanism (see pet's care() function):
		// INSERT ... ON CONFLICT DO NOTHING RETURNING id tells the caller in
		// one round trip whether this was a genuinely new action today.
		`CREATE TABLE IF NOT EXISTS pet_care_log (
			id         BIGSERIAL PRIMARY KEY,
			team_id    BIGINT NOT NULL REFERENCES users(id),
			action     TEXT NOT NULL CHECK (action IN ('bathe','breakfast','lunch','play','dinner')),
			care_date  DATE NOT NULL,
			user_id    BIGINT NOT NULL REFERENCES users(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE (team_id, action, care_date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pet_care_log_team_date ON pet_care_log (team_id, care_date)`,
		// Shop currency — "chispas" (sparks), earned by completing a perfect
		// day (all 5 actions in one calendar day), spent on shop items.
		// Unlike care_score, sparks IS shown as a raw number in the UI — a
		// shop needs a visible balance to be legible at all, that's a
		// different job than care_score's "ambient mood, not a stat to
		// chase" role.
		`ALTER TABLE pet_state ADD COLUMN IF NOT EXISTS sparks INT NOT NULL DEFAULT 0`,
		// Owned streak-freeze count — first shop item. Consumed in
		// loadOrCreate's streak-reset branch instead of zeroing the streak
		// when a full day passes with no care action, same "Duolingo streak
		// freeze" idea.
		`ALTER TABLE pet_state ADD COLUMN IF NOT EXISTS streak_freezes INT NOT NULL DEFAULT 0`,
		// Empty string means unnamed — NOT NULL DEFAULT '' rather than a
		// nullable column, same as every other pet_state column, so there's
		// never a NULL to special-case when reading it back.
		`ALTER TABLE pet_state ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT ''`,
		// Pet chat rate-limit log — one row per chat message sent, purely for
		// counting ("how many messages has this team sent in the last hour"),
		// no message content stored here at all (the message text itself is
		// never persisted, only forwarded to OpenCode Zen and discarded).
		// Inserted BEFORE the external API call in Chat() rather than after,
		// so a slow/failed call still counts against quota — same
		// "idempotency/accounting first, side effect second" ordering as
		// pet_care_log's INSERT ... ON CONFLICT, just without the conflict
		// (every message is a new row, there's nothing to dedupe here).
		`CREATE TABLE IF NOT EXISTS pet_chat_log (
			id         BIGSERIAL PRIMARY KEY,
			team_id    BIGINT NOT NULL REFERENCES users(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		// Supports the rate-limit query's WHERE team_id = $1 AND created_at >
		// now() - interval '1 hour' directly, same shape as
		// idx_pet_care_log_team_date above.
		`CREATE INDEX IF NOT EXISTS idx_pet_chat_log_team_created ON pet_chat_log (team_id, created_at)`,
		// Couples poems — private love notes written for each other.
		`CREATE TABLE IF NOT EXISTS couple_poems (
			id         BIGSERIAL PRIMARY KEY,
			author_id  BIGINT NOT NULL REFERENCES users(id),
			team_id    BIGINT NOT NULL REFERENCES users(id),
			content    TEXT   NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			seen_at    TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_couple_poems_team_id ON couple_poems (team_id)`,
		`CREATE INDEX IF NOT EXISTS idx_couple_poems_author_id ON couple_poems (author_id)`,
		// Couple date videos — TikTok-style vertical video journal per date.
		// Same object-storage-pointer shape as couple_date_photos: the two
		// object_key_* columns are the durable MinIO pointers (a 720p H.264
		// transcode and a JPEG thumbnail, both server-generated — see
		// couple/videos.go UploadVideo). ON DELETE CASCADE only reaches this
		// row when its couple_dates parent is deleted, not the actual MinIO
		// objects — the couple DeleteDate handler deletes each video's MinIO
		// objects itself first, same as it already does for photos.
		`CREATE TABLE IF NOT EXISTS couple_date_videos (
			id                BIGSERIAL PRIMARY KEY,
			date_id           BIGINT NOT NULL REFERENCES couple_dates(id) ON DELETE CASCADE,
			object_key_720p   TEXT   NOT NULL,
			object_key_thumb  TEXT   NOT NULL,
			original_filename TEXT   NOT NULL DEFAULT '',
			duration_seconds  INT    NOT NULL DEFAULT 0,
			created_by        BIGINT REFERENCES users(id),
			created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_couple_date_videos_date_id ON couple_date_videos (date_id)`,
		// Pet photos — snapshots taken via the camera in MascotaTab. Object
		// key is the durable MinIO pointer; the URL is returned freshly
		// presigned per request (never stored). Same object-storage-pointer
		// shape as couple_date_photos, no thumbnail needed here.
		`CREATE TABLE IF NOT EXISTS pet_photos (
			id         BIGSERIAL PRIMARY KEY,
			team_id    BIGINT NOT NULL REFERENCES users(id),
			object_key TEXT   NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pet_photos_team_id ON pet_photos (team_id)`,
	}
	for i, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("statement %d (%s...): %w", i, firstLine(s), err)
		}
	}
	return nil
}

// firstLine returns the first line of a statement, for error context — enough
// to name the table/operation without dumping the whole (often multi-line)
// statement into the log.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i != -1 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
