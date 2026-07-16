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
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
