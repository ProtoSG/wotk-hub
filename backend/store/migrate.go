package store

import "database/sql"

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
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS initial_balance_cents BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS credit_limit_cents BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE cards ADD COLUMN IF NOT EXISTS used_credit_cents BIGINT NOT NULL DEFAULT 0`,
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
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
