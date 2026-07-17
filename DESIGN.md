# DESIGN.md — Unified Transfer Ledger for Finances

## Overview

Replaces three independent, hand-rolled money-movement code paths (card
reload, goal contribution, and a new card-to-card transfer) with a single
`type='transfer'` transaction shape, and replaces `cards.balance_cents`/
`used_credit_cents` (stored counters mutated from those three places) with
values computed live from `transactions`. See `SPEC.md` for the full
decision log and scenario walkthrough — this document is the file-by-file
blueprint.

**The headline simplification**: once balance is computed instead of
stored, there is nothing to keep in sync. `cardDelta`, `cardAdjustment`,
and `applyCardDeltas` in `transactions.go` exist *only* to compute and
persist a delta to a stored column — under this design that whole
mechanism is deleted, not adapted. `UpdateTransaction`/`DeleteTransaction`
no longer need to "reverse the old delta then apply the new one"; a
soft-deleted or edited transaction is simply excluded/reflected the next
time anything reads the aggregate. The only thing that survives is a
lock-then-check pattern for insufficient-balance validation.

---

## 1. Backend

### 1.1 Files to modify

| File | Action |
|---|---|
| `backend/store/migrate.go` | Add transfer columns/constraint, drop `card_reloads` and the three stored card columns, make `savings_goals.default_card_id` NOT NULL, link `savings_contributions` to `transactions` |
| `backend/modules/finances/types.go` | `Transaction` gains `FromCardID`/`ToCardID`; `Card` loses `InitialBalanceCents`; `transactionRequest` rejects `transfer`; new `cardTransferRequest`; `savingsGoalRequest.DefaultCardID` becomes required |
| `backend/modules/finances/cards.go` | Rewrite `scanCard`/`ListCards`/`CreateCard` for computed balance + seed transfer; delete `CreateReload`/`ListReloads`' old bodies, replace with transfer-backed versions; add `CreateCardTransfer` |
| `backend/modules/finances/transactions.go` | Delete `cardDelta`/`cardAdjustment`/`applyCardDeltas`; add `cardBalance` (lock + compute) helper; rewrite the balance-check parts of `CreateTransaction`/`UpdateTransaction`; reject `type=transfer` in both plus `DeleteTransaction` |
| `backend/modules/finances/savings.go` | `CreateContribution` rewritten around the new balance-check helper + transfer insert + `transaction_id` link; `CreateGoal`/`UpdateGoal` validate `defaultCardId` is required and non-`credito` |
| `backend/modules/finances/summary.go` | Add `AND type != 'transfer'` to the balance and trend queries |
| `backend/modules/finances/routes.go` | Remove reload routes' old wiring (same paths, new handler bodies), add `POST /cards/transfers` |

### 1.2 Migration (`store/migrate.go`)

Append to the `stmts` slice. **Must run against truncated finance data** —
see §4.

```go
// Unified transfer ledger: reload, goal contribution, and card-to-card
// transfer all become `type='transfer'` transactions instead of three
// separate hand-mutated code paths. See SPEC.md.
`ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_type_check`,
`ALTER TABLE transactions ADD CONSTRAINT transactions_type_check CHECK (type IN ('income','expense','transfer'))`,
`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS from_card_id BIGINT REFERENCES cards(id)`,
`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS to_card_id BIGINT REFERENCES cards(id)`,
`CREATE INDEX IF NOT EXISTS idx_transactions_from_card_id ON transactions (from_card_id)`,
`CREATE INDEX IF NOT EXISTS idx_transactions_to_card_id ON transactions (to_card_id)`,

// Card balance/used-credit stop being stored — computed live from
// transactions (see cardBalance in transactions.go). initial_balance_cents
// is replaced by a seed transfer at card-creation time.
`ALTER TABLE cards DROP COLUMN IF EXISTS balance_cents`,
`ALTER TABLE cards DROP COLUMN IF EXISTS initial_balance_cents`,
`ALTER TABLE cards DROP COLUMN IF EXISTS used_credit_cents`,

// card_reloads is replaced by transactions WHERE type='transfer' AND
// from_card_id IS NULL.
`DROP TABLE IF EXISTS card_reloads`,

// A goal without a card was pure bookkeeping with no ledger effect —
// every goal is now a real transfer target. Requires clean data (no
// existing NULL default_card_id rows) — see SPEC.md's no-backfill decision.
`ALTER TABLE savings_goals ALTER COLUMN default_card_id SET NOT NULL`,

// Links each contribution to the transfer transaction that backed it, so
// the card used is known permanently even if the goal's default card
// changes later.
`ALTER TABLE savings_contributions ADD COLUMN IF NOT EXISTS transaction_id BIGINT REFERENCES transactions(id)`,
```

### 1.3 `types.go` changes

```go
const transactionTypeTransfer = "transfer"

type Transaction struct {
    ID          int64  `json:"id"`
    Type        string `json:"type"`
    AmountCents int64  `json:"amountCents"`
    Category    string `json:"category"`
    Description string `json:"description"`
    Date        string `json:"date"`
    CardID      *int64 `json:"cardId,omitempty"`
    FromCardID  *int64 `json:"fromCardId,omitempty"` // new — transfer only
    ToCardID    *int64 `json:"toCardId,omitempty"`   // new — transfer only
    CreatedAt   string `json:"createdAt"`
}

func (r transactionRequest) validate() (time.Time, error) {
    if r.Type != "income" && r.Type != "expense" {
        return time.Time{}, fmt.Errorf("invalid type: %s", r.Type)
    }
    // transfer is never user-selectable here — see cardTransferRequest,
    // CreateReload, and CreateContribution for the only three ways a
    // transfer row gets created.
    ...
}

// Card response: BalanceCents/UsedCreditCents keep the same JSON shape,
// now populated by the computed-balance query instead of a stored column.
// InitialBalanceCents is removed — see CreateCard's seed transfer.
type Card struct {
    ID               int64  `json:"id"`
    Name             string `json:"name"`
    Type             string `json:"type"`
    Bank             string `json:"bank"`
    Last4            string `json:"last4"`
    Color            string `json:"color"`
    Icon             string `json:"icon"`
    BalanceCents     int64  `json:"balanceCents"`
    CreditLimitCents int64  `json:"creditLimitCents"`
    UsedCreditCents  int64  `json:"usedCreditCents"`
    CreatedAt        string `json:"createdAt"`
}

// cardRequest keeps InitialBalanceCents in the *request* shape — the
// frontend still sends a starting balance when creating a card, it just
// becomes a seed transfer instead of a stored column.
type cardRequest struct {
    Name                string `json:"name"`
    Type                string `json:"type"`
    Bank                string `json:"bank"`
    Last4               string `json:"last4"`
    Color               string `json:"color"`
    Icon                string `json:"icon"`
    InitialBalanceCents *int64 `json:"initialBalanceCents"`
    CreditLimitCents    *int64 `json:"creditLimitCents"`
}

// New — card-to-card transfer.
type cardTransferRequest struct {
    FromCardID  int64  `json:"fromCardId"`
    ToCardID    int64  `json:"toCardId"`
    AmountCents int64  `json:"amountCents"`
    Date        string `json:"date"`
    Note        string `json:"note"`
}

func (r cardTransferRequest) validate() (time.Time, error) {
    if r.FromCardID <= 0 || r.ToCardID <= 0 {
        return time.Time{}, fmt.Errorf("fromCardId and toCardId are required")
    }
    if r.FromCardID == r.ToCardID {
        return time.Time{}, fmt.Errorf("no podés transferir a la misma tarjeta")
    }
    if r.AmountCents <= 0 {
        return time.Time{}, fmt.Errorf("amountCents must be positive")
    }
    d, err := time.Parse(dateLayout, r.Date)
    if err != nil {
        return time.Time{}, fmt.Errorf("invalid date: %s", r.Date)
    }
    return d, nil
}

// savingsGoalRequest.DefaultCardID is now required — structural check only
// (non-nil, > 0). The debito/prepago restriction needs a DB lookup, so it's
// validated in the handler (see savings.go), same pattern as
// cardTypeOwned in transactions.go.
type savingsGoalRequest struct {
    Name          string  `json:"name"`
    TargetCents   int64   `json:"targetCents"`
    Deadline      *string `json:"deadline"`
    Icon          string  `json:"icon"`
    Color         string  `json:"color"`
    DefaultCardID *int64  `json:"defaultCardId"`
}

func (r savingsGoalRequest) validate() error {
    if r.Name == "" {
        return fmt.Errorf("name is required")
    }
    if r.TargetCents <= 0 {
        return fmt.Errorf("targetCents must be positive")
    }
    if r.DefaultCardID == nil || *r.DefaultCardID <= 0 {
        return fmt.Errorf("defaultCardId is required")
    }
    if r.Deadline != nil && *r.Deadline != "" {
        if _, err := time.Parse(dateLayout, *r.Deadline); err != nil {
            return fmt.Errorf("invalid deadline date")
        }
    }
    return nil
}
```

`SavingsGoal.DefaultCardID` becomes a plain `int64` (no longer `*int64`) in
the response struct — it's never absent anymore.

### 1.4 `transactions.go` — what gets deleted, what replaces it

**Deleted entirely**: `cardDelta` struct, `cardAdjustment()`,
`(cardDelta) reverse()`, `(cardDelta) isZero()`, `applyCardDeltas()`,
`addDelta()`. Nothing computes or persists a delta anymore.

**New helper** — lock + compute, used by every flow that needs to check
"does this card have enough":

```go
// cardBalance locks the card row (pure mutex — balance itself isn't
// stored on it) and returns its live-computed balance and used-credit,
// excluding excludeTxID if given (0 = exclude nothing). Excluding the
// transaction being edited is what makes UpdateTransaction's balance
// check correct: without it, a transaction's own prior amount would be
// double-counted against itself.
func cardBalance(tx *sql.Tx, cardID int64, excludeTxID int64) (balanceCents, usedCreditCents int64, cardType string, err error) {
    err = tx.QueryRow(
        `SELECT type FROM cards WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
        cardID,
    ).Scan(&cardType)
    if err != nil {
        return 0, 0, "", err
    }

    err = tx.QueryRow(
        `SELECT
           COALESCE(SUM(CASE
             WHEN to_card_id = $1 THEN amount_cents
             WHEN from_card_id = $1 THEN -amount_cents
             WHEN card_id = $1 AND type = 'expense' AND $2 != 'credito' THEN -amount_cents
             ELSE 0
           END), 0),
           COALESCE(SUM(CASE
             WHEN card_id = $1 AND type = 'expense' AND $2 = 'credito' THEN amount_cents
             ELSE 0
           END), 0)
         FROM transactions
         WHERE deleted_at IS NULL AND id != $3
           AND (to_card_id = $1 OR from_card_id = $1 OR card_id = $1)`,
        cardID, cardType, excludeTxID,
    ).Scan(&balanceCents, &usedCreditCents)
    return balanceCents, usedCreditCents, cardType, err
}
```

**`CreateTransaction`** — the balance check for an expense tagged to a
debito/prepago card:

```go
if req.Type == "expense" && req.CardID != nil {
    balanceCents, _, cardType, err := cardBalance(tx, *req.CardID, 0)
    if err != nil { ... }
    if cardType != cardTypeCredit && balanceCents < req.AmountCents {
        tx.Rollback()
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "saldo insuficiente en tarjeta")
        return
    }
}
```

No `applyCardDeltas` call afterward — nothing to write. `used_credit_cents`
for a credit-card expense needs no check (credit cards draw down a limit,
not a balance; whether that should ever be capped by `credit_limit_cents`
is a pre-existing behavior this change doesn't touch).

**`UpdateTransaction`** — same shape, but excludes the row being edited
from the balance computation (`cardBalance(tx, cardID, editingTxID)`) so
the transaction's own prior amount isn't counted against itself.

**`DeleteTransaction`** — no balance handling at all anymore (soft-delete
already excludes the row from every future `cardBalance` call). Also
gains: reject if the row being deleted is `type='transfer'` (see below).

**Rejecting `type='transfer'` in the generic endpoints**:

```go
if req.Type == transactionTypeTransfer {
    httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid type: transfer")
    return
}
```
in `CreateTransaction`/`UpdateTransaction` (right after body decode); and
in `DeleteTransaction`, after `lockTransaction` returns the row:
```go
if old.Type == transactionTypeTransfer {
    tx.Rollback()
    httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "transaction not found")
    return
}
```
(404, not 400 — from this endpoint's perspective a transfer isn't part of
the editable ledger, same as something that doesn't exist.)

### 1.5 `cards.go` changes

**`scanCard`/`ListCards`** — the flat `cardColumns` constant goes away;
each card's balance/used-credit come from the `cardBalance`-shaped
subqueries (read-only version, no lock needed for a list read):

```go
const listCardsQuery = `
    SELECT
      c.id, c.name, c.type, c.bank, c.last4, c.color, c.icon,
      c.credit_limit_cents, c.created_at,
      COALESCE((SELECT SUM(CASE
        WHEN t.to_card_id = c.id THEN t.amount_cents
        WHEN t.from_card_id = c.id THEN -t.amount_cents
        WHEN t.card_id = c.id AND t.type = 'expense' AND c.type != 'credito' THEN -t.amount_cents
        ELSE 0 END)
       FROM transactions t
       WHERE t.deleted_at IS NULL
         AND (t.to_card_id = c.id OR t.from_card_id = c.id OR t.card_id = c.id)), 0) AS balance_cents,
      COALESCE((SELECT SUM(t.amount_cents) FROM transactions t
       WHERE t.deleted_at IS NULL AND t.card_id = c.id AND t.type = 'expense' AND c.type = 'credito'), 0) AS used_credit_cents
    FROM cards c
    WHERE c.deleted_at IS NULL`
```

`scanCard` scans in this new column order (no more `InitialBalanceCents`).

**`CreateCard`** — inserts the card, then (in the same DB transaction) a
seed transfer if `InitialBalanceCents != nil && *InitialBalanceCents > 0`:

```go
tx, _ := h.db.Begin()
defer tx.Rollback()
row := tx.QueryRow(`INSERT INTO cards (...) VALUES (...) RETURNING id, ...`, ...)
c, err := scanCard(row) // balance will read as 0 here — no transactions yet
...
if req.InitialBalanceCents != nil && *req.InitialBalanceCents > 0 {
    tx.Exec(
        `INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, to_card_id)
         VALUES ('transfer', $1, 'transferencia', 'Saldo inicial', CURRENT_DATE, $2, $3)`,
        *req.InitialBalanceCents, userID, c.ID,
    )
}
tx.Commit()
// re-fetch the card (or just set c.BalanceCents = *req.InitialBalanceCents directly, cheaper)
```

**`CreateReload`** (same route, `POST /cards/{id}/reloads`) — body/response
shape unchanged, internals become:

```go
tx, _ := h.db.Begin()
row := tx.QueryRow(
    `INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, to_card_id)
     VALUES ('transfer', $1, 'transferencia', $2, $3, $4, $5)
     RETURNING id, amount_cents, occurred_on, created_at`,
    req.AmountCents, "Recarga: "+cardName, date, userID, id,
)
```
Response mapped to the existing `CardReload` shape for backward
compatibility with the frontend (no frontend type change needed).

**`ListReloads`** (same route) — reads from `transactions` instead of
`card_reloads`:
```sql
SELECT id, amount_cents, occurred_on, description, created_at
FROM transactions
WHERE deleted_at IS NULL AND type = 'transfer' AND from_card_id IS NULL AND to_card_id = $1
ORDER BY occurred_on DESC, id DESC
```

**New — `CreateCardTransfer`** (`POST /cards/transfers`):

```go
func (h *handler) CreateCardTransfer(w http.ResponseWriter, r *http.Request) {
    userID, role, ok := middleware.UserFromContext(r.Context())
    ...
    var req cardTransferRequest
    ...
    date, err := req.validate()
    ...

    tx, _ := h.db.Begin()
    defer tx.Rollback()

    // Lock both cards in id order — same deadlock-avoidance rule the old
    // applyCardDeltas used, just applied manually here since there are
    // exactly two.
    ids := []int64{req.FromCardID, req.ToCardID}
    slices.Sort(ids)
    types := map[int64]string{}
    for _, id := range ids {
        var t string
        var deleted sql.NullTime
        err := tx.QueryRow(`SELECT type, deleted_at FROM cards WHERE id = $1 FOR UPDATE`, id).Scan(&t, &deleted)
        if err == sql.ErrNoRows || deleted.Valid {
            httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
            return
        }
        types[id] = t
    }
    if types[req.FromCardID] == cardTypeCredit || types[req.ToCardID] == cardTypeCredit {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "solo se puede transferir entre débito/prepago")
        return
    }

    balanceCents, _, _, err := cardBalance(tx, req.FromCardID, 0)
    ...
    if balanceCents < req.AmountCents {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "saldo insuficiente")
        return
    }

    row := tx.QueryRow(
        `INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, from_card_id, to_card_id)
         VALUES ('transfer', $1, 'transferencia', $2, $3, $4, $5, $6)
         RETURNING `+transactionColumns,
        req.AmountCents, description, date, userID, req.FromCardID, req.ToCardID,
    )
    t, err := scanTransaction(row)
    ...
    tx.Commit()
    httpx.WriteJSON(w, http.StatusCreated, t)
}
```

Note this calls `cardBalance` a second time internally for the lock — since
both cards are already locked above via the manual sorted loop, the
`FOR UPDATE` inside `cardBalance` on `req.FromCardID` is redundant but
harmless (Postgres row locks are idempotent for the same transaction).

### 1.6 `savings.go` changes

**`CreateGoal`/`UpdateGoal`** — after `req.validate()` passes (structural
check), look up the card and confirm ownership + type:

```go
cardType, err := h.cardTypeOwned(*req.DefaultCardID, role, userID)
if err == sql.ErrNoRows {
    httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
    return
}
if cardType == cardTypeCredit {
    httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "la tarjeta predeterminada no puede ser de crédito")
    return
}
```
(`cardTypeOwned` already lives in `transactions.go`, reused as-is.)

**`CreateContribution`** — rewritten around `cardBalance` and a transfer
insert instead of the old hand-rolled `SELECT balance_cents FOR UPDATE` +
direct `UPDATE cards`:

```go
tx, _ := h.db.Begin()
defer tx.Rollback()

var defaultCardID int64
err := tx.QueryRow(
    `SELECT default_card_id FROM savings_goals WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
    id,
).Scan(&defaultCardID)
if err == sql.ErrNoRows {
    httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "goal not found")
    return
}

balanceCents, _, cardType, err := cardBalance(tx, defaultCardID, 0)
if err == sql.ErrNoRows {
    httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict,
        "la tarjeta predeterminada de esta meta fue eliminada — asigná una nueva antes de aportar")
    return
}
if balanceCents < req.AmountCents {
    httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "saldo insuficiente en tarjeta")
    return
}

row := tx.QueryRow(
    `INSERT INTO savings_contributions (goal_id, amount_cents, occurred_on, note, created_by)
     VALUES ($1, $2, $3, $4, $5)
     RETURNING id, goal_id, amount_cents, occurred_on, note, created_by, created_at`,
    id, req.AmountCents, date, req.Note, userID,
)
c, err := scanContribution(row)
...

if _, err := tx.Exec(`UPDATE savings_goals SET current_cents = current_cents + $1 WHERE id = $2`, req.AmountCents, id); err != nil { ... }

var goalName string
tx.QueryRow(`SELECT name FROM savings_goals WHERE id = $1`, id).Scan(&goalName)
var transferID int64
err = tx.QueryRow(
    `INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, from_card_id)
     VALUES ('transfer', $1, 'transferencia', $2, $3, $4, $5)
     RETURNING id`,
    req.AmountCents, "Aporte a meta: "+goalName, date, userID, defaultCardID,
).Scan(&transferID)
...

if _, err := tx.Exec(`UPDATE savings_contributions SET transaction_id = $1 WHERE id = $2`, transferID, c.ID); err != nil { ... }

tx.Commit()
httpx.WriteJSON(w, http.StatusCreated, c)
```

`cardType` returned by `cardBalance` isn't used here — the card's type was
already validated as non-`credito` when it was assigned as the goal's
default (§1.6 above), so it can't have changed to `credito` since (cards
don't change type after creation in this app — `UpdateCard` doesn't allow
editing `type`, confirmed in the current handler).

### 1.7 `summary.go` changes

```go
balanceQuery, balanceArgs := scopeToOwner(
    `SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount_cents ELSE -amount_cents END), 0)
     FROM transactions WHERE deleted_at IS NULL AND type != 'transfer'`, []any{}, role, userID)
```
and the trend query gets the same `AND type != 'transfer'` added to its
`WHERE`. Month income/expense, category breakdown, and budgets' spent join
are already safe (see SPEC.md) — no change.

### 1.8 `routes.go`

```go
r.Post("/cards/transfers", h.CreateCardTransfer) // new
// reload routes unchanged in path, handler bodies changed per §1.5
r.Get("/cards/{id}/reloads", h.ListReloads)
r.Post("/cards/{id}/reloads", h.CreateReload)
```

---

## 2. Frontend

No response shape changes for `Card`, `CardReload`, or `Transaction` list
endpoints (`initialBalanceCents` removed from `Card`, `fromCardId`/
`toCardId` added to `Transaction` as optional fields) — most of the surface
is untouched.

### 2.1 `types/finance.types.ts`

- `Card`: remove `initialBalanceCents`.
- `CardInput`: keep `initialBalanceCents` (still sent on create).
- `Transaction`: add `fromCardId?: number`, `toCardId?: number`.
- `SavingsGoal`/`SavingsGoalInput`: `defaultCardId` becomes required
  (`number`, not `number | undefined`).
- New: `CardTransferInput { fromCardId: number; toCardId: number; amountCents: number; date: string; note: string }`.

### 2.2 `hooks/useFinanceApi.ts`

Add `createCardTransfer(input: CardTransferInput): Promise<Transaction>` →
`POST /api/finances/cards/transfers`. `listReloads`/`createReload` keep
their current signatures — only the backend internals changed.

### 2.3 `TarjetasTab.tsx`

New action alongside "Recargar" on each card: "Transferir" — opens a
dialog with amount, date, and a destination-card select (excludes the
current card and any `credito` cards, matching the backend restriction so
the error path is rare rather than the primary path).

### 2.4 `MetasTab.tsx` / `GoalForm`

- Card select in `GoalForm` becomes required (zod: `z.string().min(1)`
  instead of `.optional()`), filtered to exclude `credito` cards.
- Remove the "Sin tarjeta predeterminada" option from the select entirely.
- Remove the amber "Sin tarjeta — se descontará saldo general" warning
  block in the goal card display (§ MetasTab render) — every goal has a
  card now, that state is unreachable.
- `ContributionForm` gains a new error case to surface: 409 (default card
  archived) — same generic `toast.error(err.message)` handling already in
  place covers it, no special-case UI needed since the backend message is
  already the right user-facing text.

---

## 3. What does NOT change

- `MovimientosTab.tsx` — no new filter, no transfer rows surfacing there.
- `PresupuestosTab.tsx`, `SuscripcionesTab.tsx` — untouched, confirmed safe
  in SPEC.md.
- `ResumenTab.tsx`'s "Sin asignar"/"En metas de ahorro" reconciliation code
  (added in the previous session) — now structurally redundant for the
  card-balance side (transfers no longer create the gap it was patching),
  but left in place since it still correctly handles the residual "cash
  not tagged to any card" case. Not required to remove it as part of this
  change.
- Card, transaction, and goal soft-delete (`deleted_at`) — unrelated
  mechanism, already correct, this change only adds `AND deleted_at IS
  NULL` to the new aggregate queries the same way every other query
  already does.

---

## 4. Migration operational note

`ALTER TABLE savings_goals ALTER COLUMN default_card_id SET NOT NULL` fails
at startup if any existing row has a NULL `default_card_id`. Per SPEC.md's
no-backfill decision, this ships against clean data — truncate
`transactions, cards, card_reloads, savings_goals, savings_contributions`
(cascade) before starting the backend on the new migration for the first
time in any environment carrying pre-change data.
