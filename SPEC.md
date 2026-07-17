# SPEC — Unified Transfer Ledger for Finances

## Decision log

- Card balance and credit usage stop being stored counters
  (`cards.balance_cents`, `cards.used_credit_cents`) mutated from three
  independent code paths (transactions, reloads, contributions). They are
  now **computed live** from `SUM()` over `transactions` — a card's balance
  is a query, not a column. This removes the class of bug where the three
  writers can drift out of sync (already happened once: the insufficient-
  balance check existed for reloads before it existed for contributions).
- Three money-movement flows that used to bypass the transactions ledger
  entirely (card reload, goal contribution, and the new card-to-card
  transfer) are unified as a third transaction **type: `transfer`**, kept
  distinct from `income`/`expense` — a transfer doesn't change net worth,
  it only moves where money sits. This is what fixes the "Sin asignar"
  reconciliation gap: `summary.go`'s "Balance total" already only sums
  `transactions`, so once reloads/contributions are real rows there, the
  gap disappears at the source instead of needing a patched-on
  reconciliation line.
- `transactions` gains two nullable FKs, `from_card_id` and `to_card_id`,
  used only for `type='transfer'`. Whichever is NULL means "outside the
  card system" for that side:
  - Reload: `from_card_id=NULL`, `to_card_id=<card>`.
  - Goal contribution: `from_card_id=<card>`, `to_card_id=NULL` (destination
    is a goal, not a card — linked separately, see below).
  - Card-to-card transfer: both set.
- The existing `card_id` column (used for `income`/`expense` tagging) is
  **not touched or reinterpreted**. Tagging an income to a card still does
  not move its balance — that behavior predates this change, nobody asked
  to change it, and collapsing it into the transfer concept would be a
  separate product decision, not a consequence of this one. Deliberately
  out of scope.
- `type='transfer'` is **never user-selectable**. The generic transaction
  endpoints (`CreateTransaction`, `UpdateTransaction`, `DeleteTransaction`)
  reject it outright — a transfer is only ever created as a side effect of
  a specific flow (Recargar, Aportar a meta, Transferir entre tarjetas) and
  can only be undone from that flow's own screen, not from Movimientos.
  None of the three flows have an individual undo/delete today, so this
  isn't a regression.
- `category` on a transfer row is a single fixed constant, `'transferencia'`
  — not user-chosen, not a 3-value taxonomy per sub-flow. The `description`
  field carries the human-readable distinction ("Recarga: BCP Débito",
  "Aporte a meta: Viaje a Cusco", "Transferencia: BCP Débito → BCP Ahorro").
  This keeps `expenseCategories`/`incomeCategories` (and the budget/category
  breakdown UI built on them) untouched.
- `card_reloads` is **dropped**. Its data becomes `transactions` rows with
  `type='transfer'`; `ListReloads` becomes a filtered query instead of a
  separate table.
- **No historical backfill.** `card_reloads` and `savings_contributions`
  rows that predate this change are not migrated into the new model — the
  app is pre-production, this session has already truncated finance data
  repeatedly for testing, and a contribution's historical card can't be
  reconstructed with certainty if the goal's `default_card_id` changed
  after the fact. The migration ships against clean data.
- `savings_goals.default_card_id` becomes **required** (`NOT NULL`),
  restricted to `debito`/`prepago` card types (validated in Go — Postgres
  can't express "FK to a row where column X = Y" without a trigger, and a
  trigger is more machinery than this needs). A goal without a card was a
  pure bookkeeping entry with no ledger effect; making the card mandatory
  makes every goal a real transfer target, which is the ideal-scenario
  target state agreed on above the decision log.
- Card-to-card transfers (v1) exclude `credito` as either source or
  destination. A credit card doesn't hold a spendable balance in this model
  (it holds `used_credit_cents`, a debt figure) — "transferring out" of one
  doesn't make sense, and "paying down" one has different math
  (`used_credit_cents -= amount`, not `balance_cents += amount`). Explicitly
  deferred, not implemented here.
- `savings_contributions` keeps its own table (goal-specific `note` and the
  denormalized `current_cents` progress counter aren't a balance-derivation
  concern) but gains a nullable `transaction_id` FK linking each
  contribution to the transfer that backed it. This is a free upgrade: today
  a contribution doesn't record which card it actually drew from at the
  time, only the goal's *current* `default_card_id` is known. With the
  link, that's permanent per-contribution history.
- Deleting a goal (soft-delete, existing behavior) does **not** touch its
  linked transfer transactions — they stay in the ledger permanently. This
  matches the existing `DeleteGoalDialog` warning ("si la eliminas, NO se
  reintegrará a la tarjeta"): the money movement already happened and is
  not undone by archiving the goal.
- `cards.initial_balance_cents` is removed. Creating a card with a starting
  balance > 0 inserts a seed transfer (`from_card_id=NULL`,
  `to_card_id=<new card>`, description "Saldo inicial") in the same DB
  transaction as the card insert, instead of storing the starting amount
  as a separate field. One less derived column to keep in sync.
- `cards.credit_limit_cents` is **not** derived — it's a configured limit,
  not a movement, and stays a stored column. `used_credit_cents` follows
  the same computed-live treatment as `balance_cents`.
- Movimientos (the transaction list tab) keeps its current Ingresos/Gastos
  filter — `transfer` rows are not surfaced there in v1. They're visible
  through their own context: a card's reload history, a goal's contribution
  history. Showing them in the general ledger is a deliberate v2 candidate,
  not required now.

---

## Data model

### `transactions` (changes only)

| column | type | notes |
|---|---|---|
| `type` | TEXT CHECK | now `('income','expense','transfer')` |
| `from_card_id` | BIGINT REFERENCES cards(id), nullable | new. Only meaningful for `type='transfer'`. NULL = source is outside the card system (reload). |
| `to_card_id` | BIGINT REFERENCES cards(id), nullable | new. Only meaningful for `type='transfer'`. NULL = destination is outside the card system (goal contribution). |
| `category` | TEXT NOT NULL | unchanged column; transfers always write `'transferencia'` |
| `card_id` | BIGINT REFERENCES cards(id), nullable | **unchanged** — still the income/expense tagging column, untouched by this change |

### `cards` (changes only)

| column | change |
|---|---|
| `balance_cents` | **removed** — computed live: `SUM` of transfers/expenses that move this card's balance |
| `initial_balance_cents` | **removed** — replaced by a seed transfer row at card creation |
| `used_credit_cents` | **removed** — computed live: `SUM` of expenses tagged to this card where `type='credito'` |
| `credit_limit_cents` | unchanged, still stored (configured value, not derived) |

Computed balance definition (conceptually):

```
balance_cents(card) =
    SUM(amount_cents WHERE to_card_id = card.id)          -- transfers in
  - SUM(amount_cents WHERE from_card_id = card.id)        -- transfers out
  - SUM(amount_cents WHERE card_id = card.id AND type='expense' AND card.type != 'credito')

used_credit_cents(card) =
    SUM(amount_cents WHERE card_id = card.id AND type='expense' AND card.type = 'credito')
```

All three sums exclude soft-deleted (`deleted_at IS NOT NULL`) transactions, same as every other aggregate in `summary.go`/`budgets.go` today.

### `card_reloads`

**Dropped.** Reload history becomes `transactions WHERE type='transfer' AND from_card_id IS NULL AND to_card_id = <card>`.

### `savings_goals` (changes only)

| column | change |
|---|---|
| `default_card_id` | now `NOT NULL`; validated in Go to reference a `debito`/`prepago` card |

### `savings_contributions` (changes only)

| column | change |
|---|---|
| `transaction_id` | new, `BIGINT REFERENCES transactions(id)`, nullable only in the sense that old (pre-migration) rows won't have it — every new contribution always has a default card now, so every new row gets one |

---

## Scenarios — goal contribution (`CreateContribution`)

Request lifecycle, in order, branching at each decision point:

0. No valid session → 401.
1. Invalid body (`amountCents <= 0`, bad `date`) → 400, no transaction opened.
2. `SELECT ... FROM savings_goals WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, scoped by owner.
   - Not found / not owned / soft-deleted → 404.
   - Found → row locked for the rest of this transaction, closing the race where a concurrent `UpdateGoal` changes `default_card_id` mid-flight.
3. `SELECT type, balance FROM cards WHERE id=<default_card_id> AND deleted_at IS NULL FOR UPDATE` (default card is now guaranteed to exist on the goal, but may have been archived since).
   - Not found (archived) → rollback, 409 "la tarjeta predeterminada de esta meta fue eliminada — asigná una nueva antes de aportar". (Considered auto-clearing `default_card_id` and falling back silently — rejected: changes the goal's config without being asked, worse than an explicit error.)
   - Found, insufficient balance → rollback, 400 "saldo insuficiente en tarjeta". Nothing is written — no contribution row, no `current_cents` change, no transfer.
   - Found, sufficient balance → continue.
4. Insert `savings_contributions` (goal_id, amount, date, note, created_by).
5. `UPDATE savings_goals SET current_cents = current_cents + amount`.
6. Insert `transactions`: `type='transfer'`, `from_card_id=<default card>`, `to_card_id=NULL`, `category='transferencia'`, `description='Aporte a meta: <name>'`, `occurred_on=date`, `created_by`.
7. `UPDATE savings_contributions SET transaction_id=<step 6 id>`.
8. Apply the card delta via the same `applyCardDeltas` transactions already use for expenses — not a hand-rolled UPDATE.
9. Commit. 201.

Concurrency: two contributions to the same goal serialize on the goal-row lock from step 2 — the second waits for the first's commit/rollback before reading `default_card_id`/balance. Lock order is always goal → card, consistent across every call site, so no deadlock risk.

Goal deletion afterward: soft-deletes the goal row only. The transfer transaction from step 6 stays untouched, permanently reflecting that the money left the card.

## Scenarios — reload (`from_card_id=NULL`)

1. Card not found / not owned / archived → 404 (unchanged from today's `cardOwned` check).
2. Card found → insert `transactions` (`type='transfer'`, `from_card_id=NULL`, `to_card_id=<card>`, `category='transferencia'`, `description='Recarga: <card name>'`), apply delta (+amount) via `applyCardDeltas`, commit, 201.

No balance check needed (adding money never fails for insufficiency).

## Scenarios — card-to-card transfer (new)

1. `fromCardId == toCardId` → 400 "no podés transferir a la misma tarjeta".
2. Either card not found / not owned / archived → 404.
3. Either card is `credito` → 400 "solo se puede transferir entre débito/prepago" (v1 restriction).
4. Lock `fromCard` FOR UPDATE → insufficient balance → 400 "saldo insuficiente".
5. Insert one `transactions` row (`type='transfer'`, `from_card_id`, `to_card_id`, both set, `category='transferencia'`, `description='Transferencia: <A> → <B>'`).
6. `applyCardDeltas` with both cards in one call — reuses the existing multi-card, deadlock-safe (sorted by id) path already built for `UpdateTransaction`.
7. Commit. 201.

---

## API changes

### New

- `POST /api/finances/cards/transfers` → `{fromCardId, toCardId, amountCents, date, note}` → `Transaction` (201)

### Changed

- `POST /api/finances/cards/{id}/reloads` — same request/response shape, now internally inserts a `transfer` transaction instead of a `card_reloads` row + direct `UPDATE`.
- `GET /api/finances/cards/{id}/reloads` — same response shape, now reads from `transactions` instead of `card_reloads`.
- `POST/PUT /api/finances/savings-goals` — `defaultCardId` is now required; 400 if missing or if it references a `credito` card.
- `POST /api/finances/savings-goals/{id}/contributions` — same request/response shape; can now fail with 409 if the goal's default card was archived (new case, see scenarios above).
- `GET /api/finances/cards` — `balanceCents`/`usedCreditCents` in the response are unchanged in shape, now computed instead of stored. `initialBalanceCents` field is removed from the response.
- `POST /api/finances/cards` — `initialBalanceCents` in the request, if > 0, now creates a seed transfer instead of setting a stored column.
- `PUT/DELETE /api/finances/transactions/{id}` — now explicitly reject `type='transfer'` rows (404, same as "not found" — a transfer isn't part of the editable ledger from this endpoint's perspective).

### Unchanged

- `GET/POST/PUT/DELETE` for `subscriptions`, `budgets`, plain `transactions` (income/expense) — no shape or behavior change.
- `GET /api/finances/summary` — response shape unchanged; the underlying balance and trend queries gain `AND type != 'transfer'` (see below), fixing a real bug this change would otherwise introduce.

---

## summary.go — correctness requirement, not optional

Two of the four aggregate queries in `Summary` are **unsafe** against the new `transfer` type if left as-is, and must be fixed as part of this change, not after:

- Balance total: `SUM(CASE WHEN type='income' THEN amount ELSE -amount END)` — the `ELSE` branch currently catches anything that isn't income, which would include `transfer` rows and wrongly subtract them from balance total. Needs `AND type != 'transfer'` in the `WHERE`.
- 6-month trend: groups by `type`, and the Go consumer buckets anything that isn't `"income"` into `ExpenseCents`. Same fix — filter out `transfer` in the SQL `WHERE`.

Already safe, no change needed: month income/expense (`FILTER (WHERE type = 'income'/'expense')` explicitly), category breakdown (`WHERE type = 'expense'`), budgets `spent` join (`t.type = 'expense'`).

---

## Acceptance criteria

1. Card balance and used-credit are never stored — always derived from `transactions` at read time, for every card list/detail endpoint.
2. Recharging a card, contributing to a goal, and transferring between cards all produce exactly one `transactions` row each (two for card-to-card is wrong — one row, two FKs).
3. None of the three flows are reachable through `POST/PUT /transactions` — `type='transfer'` is rejected there.
4. A goal cannot be created or updated without a `debito`/`prepago` `defaultCardId`.
5. Contributing to a goal whose default card was archived since assignment fails with a clear 409, not a 500 or a silent no-op.
6. "Balance total" and the 6-month trend chart are unaffected by reloads, contributions, or card-to-card transfers — verified by the same reconciliation scenario that originally surfaced this whole redesign (recharge a card, contribute part of it to a goal, confirm "Balance total" only reflects real income/expense).
7. Deleting a card, goal, or transaction still follows existing soft-delete rules — this change doesn't touch that.
8. Movimientos (tab) shows no `transfer` rows; card and goal detail views show their own transfer history.
9. All Spanish user-facing strings match the existing tone (`"Aporte a meta: ..."`, `"Recarga: ..."`, `"Transferencia: ... → ..."`, `"saldo insuficiente en tarjeta"`).
10. No historical `card_reloads`/`savings_contributions` data is preserved across the migration — verified by truncating finance tables before applying it in dev/test.
