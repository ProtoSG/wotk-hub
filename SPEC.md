# SPEC — Finances: Transfer Ledger + Mandatory Card Account Model

> **Current state.** This document describes the live Finances model after
> the mandatory-card account model was layered on top of the original unified
> transfer ledger. The Decision log records the evolution; superseded entries
> are marked inline and explained by a later entry. The body prose reflects
> the *current* reality, not the historical one.

## Decision log

(Entries preserved in chronological order. Superseded entries are marked
inline and explained by a later entry.)

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
  reconciliation gap: `summary.go`'s balance already only sums
  `transactions`, so once contributions/seed/transfer are real rows there,
  the gap disappears at the source instead of needing a patched-on
  reconciliation line.
- `transactions` gains two nullable FKs, `from_card_id` and `to_card_id`,
  used only for `type='transfer'`. Whichever is NULL means "outside the
  card system" for that side:
  - Goal contribution: `from_card_id=<card>`, `to_card_id=NULL` (destination
    is a goal, not a card — linked separately, see below).
  - Card-to-card transfer: both set.
  - (The reload `from_card_id=NULL`, `to_card_id=<card>` variant is
    **[SUPERSEDED]** — the reload concept was removed entirely by the
    mandatory-card entry below.)
- `type='transfer'` is **never user-selectable**. The generic transaction
  endpoints (`CreateTransaction`, `UpdateTransaction`, `DeleteTransaction`)
  reject it outright — a transfer is only ever created as a side effect of
  a specific flow (Aportar a meta, Transferir entre tarjetas, or the seed
  inserted at card creation) and can only be undone from that flow's own
  screen, not from Movimientos. None of these flows have an individual
  undo/delete today, so this isn't a regression.
- `category` on a transfer row is a single fixed constant, `'transferencia'`
  — not user-chosen, not a 3-value taxonomy per sub-flow. The `description`
  field carries the human-readable distinction ("Aporte a meta: Viaje a
  Cusco", "Transferencia: BCP Débito → BCP Ahorro", "Saldo inicial"). This
  keeps `expenseCategories`/`incomeCategories` (and the budget/category
  breakdown UI built on them) untouched.
- `card_reloads` is **dropped**. The `DROP TABLE IF EXISTS` is kept idempotent
  in the migration. (The reload-*as-transfer* shim that used to live at
  `POST/GET /cards/{id}/reloads` is **[SUPERSEDED]** — both the endpoints and
  the Recargar UI are now gone, see the mandatory-card entry below.)
- **No historical backfill.** `card_reloads` and `savings_contributions`
  rows that predate a change are not migrated into the new model — the app is
  pre-production, finance data gets truncated for testing repeatedly, and a
  contribution's historical card can't be reconstructed with certainty if
  the goal's `default_card_id` changed after the fact. Every breaking
  migration ships against clean data.
- `savings_goals.default_card_id` becomes **required** (`NOT NULL`),
  restricted to `debito`/`prepago` card types (validated in Go — Postgres
  can't express "FK to a row where column X = Y" without a trigger, and a
  trigger is more machinery than this needs). A goal without a card was a
  pure bookkeeping entry with no ledger effect; making the card mandatory
  makes every goal a real transfer target.
- Card-to-card transfers exclude `credito` as either source or
  destination. A credit card doesn't hold a spendable balance in this model
  (it holds `used_credit_cents`, a debt figure) — "transferring out" of one
  doesn't make sense, and "paying down" one has different math
  (`used_credit_cents -= amount`, not `balance_cents += amount`). Explicitly
  deferred, not implemented.
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
  filter — `transfer` rows are not surfaced there. They're visible through
  their own context: a goal's contribution history. Showing them in the
  general ledger is a deliberate v2 candidate, not required now.
- The existing `card_id` column (used for `income`/`expense` tagging) is
  **not touched or reinterpreted**. Tagging an income to a card still does
  not move its balance — that behavior predates this change, nobody asked
  to change it, and collapsing it into the transfer concept would be a
  separate product decision, not a consequence of this one. Deliberately
  out of scope. — **[SUPERSEDED]** by the mandatory-card entry below.
- **[MANDATORY-CARD ACCOUNT MODEL — supersedes the three `[SUPERSEDED]`
  bullets above]** `card_id` is now **mandatory** on every income/expense
  transaction, enforced at the DB by a CHECK constraint
  (`type='transfer' OR card_id IS NOT NULL`) rather than a column-wide
  NOT NULL (transfer rows legitimately carry NULL `card_id`). Income
  tagged to a non-credito card now **moves its computed balance** via a
  new `WHEN card_id=$1 AND type='income' AND $2!='credito' THEN amount`
  branch in the `cardBalance` CASE; income/transfer tagged to a credito
  card is rejected by a shared `rejectCreditCardForInflow` helper
  ("no se puede taggear ingresos a una tarjeta de crédito");
  expense-to-credito is unchanged (adds to `used_credit_cents`). The
  **reload concept is removed entirely** — recharging a card is now done
  by tagging an income to it (or a card-to-card transfer); the
  `POST/GET /cards/{id}/reloads` endpoints and the Recargar UI are gone,
  so is the reload-as-transfer shim. `subscriptions.card_id` becomes
  **mandatory** (`NOT NULL`) so `processDue` always emits a tagged
  expense (the "Sin tarjeta" subscription state is structurally
  impossible now). `RefundTransaction` needs **no code change** — its
  compensating income row is already tagged to `old.CardID`, so the new
  income branch **repone saldo** automatically (refund of a credito
  expense doesn't reduce `used_credit_cents`; deferred to a future
  credit_lines split). With every income/expense tagged, "Sin asignar"
  is **structurally impossible**, so the ResumenTab reconciliation line
  is removed; the "Balance total" tile is renamed **"Disponible"** and
  computed frontend-side as `netWorth − Σ savings_goals.currentCents`
  (transfer-agnostic). A **page-level onboarding gate** blocks every
  Finances tab until the owner has ≥1 non-credito active card (initial
  balance MAY be 0; credito-alone does not clear the gate).
  `DeleteCard` rejects archiving the owner's last active card with **409**
  ("no podés archivar tu última tarjeta activa") — a guard added from
  scratch, no prior first-card rule existed. Migration ships against
  truncated finance tables (no backfill): legacy NULL `card_id` rows on
  `transactions` (income/expense) and on `subscriptions` are lost, same
  convention as `savings_goals.default_card_id SET NOT NULL`.

---

## Data model

### `transactions` (current state)

| column | type | notes |
|---|---|---|
| `type` | TEXT CHECK | `('income','expense','transfer')` |
| `from_card_id` | BIGINT REFERENCES cards(id), nullable | `type='transfer'` only. NULL = source is outside the card system (goal contribution, seed). |
| `to_card_id` | BIGINT REFERENCES cards(id), nullable | `type='transfer'` only. NULL = destination is outside the card system (goal contribution). |
| `category` | TEXT NOT NULL | unchanged column; transfers always write `'transferencia'` |
| `card_id` | BIGINT REFERENCES cards(id), nullable | **mandatory for income/expense**, enforced by `CHECK (type='transfer' OR card_id IS NOT NULL)`. Column stays nullable because transfer rows legitimately have it NULL. Income→non-credito moves this card's computed balance; income→credito is rejected at the handler. |

### `subscriptions` (current state)

| column | change |
|---|---|
| `card_id` | **NOT NULL** — `processDue` always emits an expense row tagged with the subscription's card. The "Sin tarjeta" subscription state is structurally impossible. |

### `cards` (current state)

| column | state |
|---|---|
| `balance_cents` | removed — computed live (see `cardBalance`) |
| `initial_balance_cents` | removed — replaced by a seed transfer at card creation |
| `used_credit_cents` | removed — computed live: `SUM` of expenses tagged to this card where `type='credito'` |
| `credit_limit_cents` | unchanged, stored (configured value, not derived) |

Computed balance definition (conceptually — the actual CASE is in
`transactions.go:cardBalance` and `cards.go:cardsBaseQuery`, mirror copies):

```
balance_cents(card) =
    SUM(amount_cents WHERE to_card_id   = card.id)                         -- transfers in (incl. seed)
  + SUM(amount_cents WHERE card_id = card.id AND type='income'  AND card.type != 'credito')  -- mandatory-card income branch
  - SUM(amount_cents WHERE from_card_id = card.id)                         -- transfers out (incl. contribution)
  - SUM(amount_cents WHERE card_id = card.id AND type='expense' AND card.type != 'credito')

used_credit_cents(card) =
    SUM(amount_cents WHERE card_id = card.id AND type='expense' AND card.type = 'credito')
```

All four sums exclude soft-deleted (`deleted_at IS NOT NULL`) transactions.
The income branch fires **only** for non-credito cards (`$2 != 'credito'`),
so credito `balance_cents` stays 0 even if an income row were somehow
tagged to one — defense in depth on top of the handler-level
`rejectCreditCardForInflow` guard.

### `card_reloads`

**Dropped.** `DROP TABLE IF EXISTS card_reloads` is kept idempotent in the
migration. There is no reload flow anymore — a card is "recharged" by
tagging an income to it (or a card-to-card transfer).

### `savings_goals` (current state)

| column | state |
|---|---|
| `default_card_id` | NOT NULL; validated in Go to reference a `debito`/`prepago` card |

### `savings_contributions` (current state)

| column | state |
|---|---|
| `transaction_id` | `BIGINT REFERENCES transactions(id)`, nullable only for pre-migration rows — every new contribution gets one |

---

## Scenarios — goal contribution (`CreateContribution`)

Request lifecycle, in order, branching at each decision point:

0. No valid session → 401.
1. Invalid body (`amountCents <= 0`, bad `date`) → 400, no transaction opened.
2. `SELECT ... FROM savings_goals WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, scoped by owner.
   - Not found / not owned / soft-deleted → 404.
   - Found → row locked for the rest of this transaction, closing the race where a concurrent `UpdateGoal` changes `default_card_id` mid-flight.
3. `SELECT type, balance FROM cards WHERE id=<default_card_id> AND deleted_at IS NULL FOR UPDATE` (default card is now guaranteed to exist on the goal, but may have been archived since).
   - Not found (archived) → rollback, 409 "la tarjeta predeterminada de esta meta fue eliminada — asigná una nueva antes de aportar".
   - Found, insufficient balance → rollback, 400 "saldo insuficiente en tarjeta". Nothing is written.
   - Found, sufficient balance → continue.
4. Insert `savings_contributions` (goal_id, amount, date, note, created_by).
5. `UPDATE savings_goals SET current_cents = current_cents + amount`.
6. Insert `transactions`: `type='transfer'`, `from_card_id=<default card>`, `to_card_id=NULL`, `category='transferencia'`, `description='Aporte a meta: <name>'`, `occurred_on=date`, `created_by`.
7. `UPDATE savings_contributions SET transaction_id=<step 6 id>`.
8. Commit. 201.

Goal deletion afterward: soft-deletes the goal row only. The transfer
transaction from step 6 stays untouched, permanently reflecting that the
money left the card.

## Scenarios — card-to-card transfer (`CreateCardTransfer`)

1. `fromCardId == toCardId` → 400 "no podés transferir a la misma tarjeta".
2. Either card not found / not owned / archived → 404.
3. Either card is `credito` → 400 "no se puede taggear ingresos a una tarjeta de crédito" (the shared `rejectCreditCardForInflow` helper runs on both sides).
4. Lock `fromCard` FOR UPDATE → insufficient balance → 400 "saldo insuficiente".
5. Insert one `transactions` row (`type='transfer'`, `from_card_id`, `to_card_id`, both set, `category='transferencia'`, `description='Transferencia: <A> → <B>'`).
6. Commit. 201.

## Scenarios — reload (`POST/GET /cards/{id}/reloads`) — REMOVED

The reload concept is removed by the mandatory-card model. The endpoints
return 404, the Recargar UI is gone, and the `card_reloads` table is dropped.
To add money to a card, tag an income to it (or do a card-to-card transfer).
See the Decision log (mandatory-card entry).

## Scenarios — mandatory `card_id` on transactions

- **Missing card_id rejected** — GIVEN a valid income/expense request, WHEN
  `cardId` omitted or `<= 0`, THEN 400 "cardId requerido", nothing written
  (`transactionRequest.validate` + the CHECK constraint both enforce it).
- **Income tagged to debito moves balance** — GIVEN card X (debito),
  computed balance B, WHEN `CreateTransaction type='income' card_id=X
  amount=100`, THEN `cardBalance(X) = B + 100`.
- **Credito rejects income tagging** — GIVEN card C `credito`, WHEN
  `CreateTransaction type='income' card_id=C`, THEN 400 "no se puede
  taggear ingresos a una tarjeta de crédito".
- **Credito rejects transfer tagging** — GIVEN `CreateCardTransfer` with C
  as source or destination, THEN 400 (same helper).
- **Expense to credito unchanged** — GIVEN card C `credito`,
  `used_credit_cents` U, WHEN expense `card_id=C` amount N, THEN
  `used_credit_cents(C) = U + N`; computed `balance_cents` unchanged.
- **Refund repone saldo** — GIVEN an expense row tagged to non-credito card
  X, refunded, WHEN `RefundTransaction`, THEN an income row `card_id=X` is
  inserted AND `cardBalance(X)` increases by amount (automatic via the
  income branch — `RefundTransaction` itself unchanged). MovimientosTab
  refund dialog copy reads "El reembolso sí repondrá el saldo de la
  tarjeta".

## Scenarios — mandatory `card_id` on subscriptions

- **Missing card_id rejected** — GIVEN `CreateSubscription` without
  `cardId`, WHEN POST `/subscriptions`, THEN 400 "cardId requerido".
- **processDue always tagged** — GIVEN a subscription with `card_id=X`,
  WHEN `processDue` fires, THEN the generated expense row has `card_id=X`
  (never NULL).

## Scenarios — finance onboarding gate (`FinancesPage.tsx`)

- **First visit, no cards** — GIVEN owner with zero non-deleted cards, WHEN
  opening `/finances`, THEN every tab renders an onboarding screen (page-level
  gate), not normal content, and the only offered action is "Agregar tarjeta"
  (inline card form with no crédito option, since crédito-alone doesn't clear
  the gate).
- **Credito-alone does not satisfy gate** — GIVEN owner has 1 credito card, 0
  non-credito cards, WHEN opening `/finances`, THEN the onboarding screen
  still blocks (credito excluded by the `type !== 'credito'` filter).
- **Gate clears on first non-credito card** — GIVEN owner creates a debito
  card with balance 0, WHEN returning to `/finances`, THEN all tabs render
  their normal content.

## Scenarios — delete-last-active-card invariant (`DeleteCard`)

- **Only active card** — GIVEN owner has exactly 1 active card (any type),
  WHEN `DELETE /cards/{id}`, THEN 409 "no podés archivar tu última tarjeta
  activa" (the COUNT fires before any mutation). Frontend disables the
  delete affordance when `cards.length === 1` and toasts on 409.
- **Two cards, delete one** — GIVEN owner has 2 active cards, WHEN
  `DELETE /cards/{id}`, THEN 200, card soft-deleted, other remains.

## Scenarios — summary "Disponible" tile (`ResumenTab.tsx`)

- **Goal commitment reduces Disponible** — GIVEN netWorth=1000, Σ
  `goals.currentCents`=300, THEN "Disponible" tile = 700 (computed
  frontend-side as `summary.balanceCents − Σ goals.currentCents`; no new
  backend field).
- **Card-to-card transfer neutral** — GIVEN a transfer 200 from card A to
  card B, THEN "Disponible" unchanged (transfer-agnostic — `summary.go`'s
  balanceQuery filters `type != 'transfer'`).
- **Sin asignar line gone** — WHEN loading ResumenTab, THEN no "Sin
  asignar" reconciliation row renders (structurally impossible now; the
  line and its diff computation were deleted).

---

## API changes

### New (card-to-card)

- `POST /api/finances/cards/transfers` → `{fromCardId, toCardId, amountCents, date, note}` → `Transaction` (201)

### Removed (mandatory-card model)

- `POST /api/finances/cards/{id}/reloads` — gone (404). Recharge a card by
  tagging an income to it instead.
- `GET /api/finances/cards/{id}/reloads` — gone (404).

### Changed

- `POST/PUT /api/finances/transactions` — `cardId` is now **required** in
  the body (`number`, not optional); 400 "cardId requerido" if missing/`<=0`.
  Income tagged to a `credito` card → 400 "no se puede taggear ingresos a
  una tarjeta de crédito". `type='transfer'` is still rejected outright.
- `POST/PUT /api/finances/subscriptions` — `cardId` is now **required**;
  400 "cardId requerido" if missing. A bogus card surfaces as 404.
- `DELETE /api/finances/cards/{id}` — now MAY return 409 "no podés archivar
  tu última tarjeta activa" if the owner has only one active card.
- `POST /api/finances/savings-goals` — `defaultCardId` required; 400 if
  missing or if it references a `credito` card.
- `POST /api/finances/savings-goals/{id}/contributions` — may now fail with
  409 if the goal's default card was archived.
- `GET /api/finances/cards` — `balanceCents`/`usedCreditCents` computed
  (not stored); `initialBalanceCents` removed from the response.
- `POST /api/finances/cards` — `initialBalanceCents` in the request, if > 0,
  creates a seed transfer instead of setting a stored column.
- `PUT/DELETE /api/finances/transactions/{id}` — reject `type='transfer'`
  (404, same as "not found" — a transfer isn't part of the editable ledger
  from this endpoint's perspective).

### Unchanged

- `GET/POST/PUT/DELETE` for `budgets` and plain `transactions` (income/expense)
  in shape; `transactions` gains the mandatory-`cardId` rule above.
- `GET /api/finances/summary` — response shape unchanged; the underlying
  balance and trend queries filter `AND type != 'transfer'`. The "Balance
  total" → "Disponible" rename and the `− Σ goals.currentCents` subtraction
  are **frontend-only** (`ResumenTab.tsx`), no new backend field.

---

## summary.go — correctness requirement, not optional

Two of the four aggregate queries in `Summary` are **unsafe** against the
`transfer` type if left as-is, and were fixed as part of the transfer-ledger
change (not after):

- Balance total: `SUM(CASE WHEN type='income' THEN amount ELSE -amount END)`
  — the `ELSE` branch would catch `transfer` rows and wrongly subtract them.
  Needs `AND type != 'transfer'` in the `WHERE`. (This is the value
  `ResumenTab` renames "Disponible" and subtracts `Σ goals.currentCents`
  from — the backend field is unchanged.)
- 6-month trend: groups by `type`, the Go consumer buckets anything that
  isn't `"income"` into `ExpenseCents`. Same fix — filter out `transfer` in
  the SQL `WHERE`.

Already safe, no change needed: month income/expense (`FILTER (WHERE type =
'income'/'expense')` explicitly), category breakdown (`WHERE type =
'expense'`), budgets `spent` join (`t.type = 'expense'`).

---

## Acceptance criteria

1. Card balance and used-credit are never stored — always derived from
   `transactions` at read time, for every card list/detail endpoint.
2. Contributing to a goal and transferring between cards each produce
   exactly one `transactions` row each (one row, two FKs for card-to-card).
3. None of the transfer-producing flows are reachable through
   `POST/PUT /transactions` — `type='transfer'` is rejected there.
4. A goal cannot be created or updated without a
   `debito`/`prepago` `defaultCardId`.
5. Contributing to a goal whose default card was archived since assignment
   fails with a clear 409, not a 500 or a silent no-op.
6. **"Disponible"** (was "Balance total") and the 6-month trend chart are
   unaffected by contributions or card-to-card transfers — verified by the
   same reconciliation scenario that originally surfaced this whole redesign
   (contribute part of a card's balance to a goal, confirm "Disponible"
   only reflects real income/expense minus goal commitments). The "Sin
   asignar" reconciliation line is gone, not patched on.
7. Editing/deleting a card, goal, or transaction still follows existing
   soft-delete rules — neither change touches that.
8. Movimientos (tab) shows no `transfer` rows; card and goal detail views
   show their own transfer history.
9. All Spanish user-facing strings match the existing tone
   (`"Aporte a meta: ..."`, `"Transferencia: ... → ..."`,
   `"Saldo inicial"`, `"saldo insuficiente en tarjeta"`,
   `"no se puede taggear ingresos a una tarjeta de crédito"`,
   `"no podés archivar tu última tarjeta activa"`,
   `"cardId requerido"`).
10. No historical `card_reloads`/`savings_contributions`/`transactions`/
    `subscriptions` data is preserved across the migration — verified by
    truncating finance tables before applying it in dev/test.
11. **Every income/expense transaction has a `card_id`** — enforced by
    `CHECK (type='transfer' OR card_id IS NOT NULL)`; "Sin asignar" is
    structurally impossible.
12. **Every subscription has a `card_id`** — `NOT NULL`; `processDue`
    always emits a tagged expense.
13. **Refund repone saldo** — `RefundTransaction` inserts an income row
    tagged to the original card; the card's computed balance increases by
    the refunded amount (non-credito).
14. **Onboarding gate** — `/finances` blocks all tabs until the owner has
    ≥1 non-credito active card; credito-alone does not clear the gate.
15. **Delete-last-card invariant** — `DELETE /cards/{id}` returns 409 when
    the owner has only one active card.