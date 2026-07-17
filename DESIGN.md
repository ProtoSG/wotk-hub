# DESIGN.md — Finances: Transfer Ledger + Mandatory Card Account Model

## Overview

Two layered changes share this blueprint:

1. **Unified transfer ledger** — replaces three independent, hand-rolled
   money-movement code paths (goal contribution, a new card-to-card
   transfer, and the card seed) with a single `type='transfer'`
   transaction shape, and replaces `cards.balance_cents`/
   `used_credit_cents` (stored counters mutated from those places) with
   values computed live from `transactions`. See `SPEC.md` for the full
   decision log and scenario walkthrough.
2. **Mandatory-card account model** (layered on top) — makes `card_id`
   mandatory on income/expense (CHECK constraint, not column NOT NULL),
   adds an `income` branch to the `cardBalance` CASE, makes
   `subscriptions.card_id` NOT NULL, removes the reload concept
   entirely, renames the summary tile to "Disponible", adds an onboarding
   gate, and adds a delete-last-card invariant.

**The headline simplification**: once balance is computed instead of
stored, there is nothing to keep in sync. `cardDelta`, `cardAdjustment`,
and `applyCardDeltas` in `transactions.go` existed *only* to compute and
persist a delta to a stored column — under this design that whole
mechanism is deleted, not adapted. `UpdateTransaction`/`DeleteTransaction`
no longer "reverse the old delta then apply the new one"; a soft-deleted or
edited transaction is simply excluded/reflected the next time anything
reads the aggregate. The only thing that survives is a lock-then-check
pattern for insufficient-balance validation. The **mandatory-card** layer
then makes `card_id` non-optional on the write path and lets income move
balance the same way expense already did.

---

## 1. Backend

### 1.1 Files

| File | Action |
|---|---|
| `backend/store/migrate.go` | Transfer columns/constraint; drop `card_reloads` and the three stored card columns; make `savings_goals.default_card_id` NOT NULL; link `savings_contributions` to `transactions`. **Mandatory-card layer:** add `transactions.card_id` CHECK constraint, `subscriptions.card_id` SET NOT NULL, idempotent `DROP TABLE IF EXISTS card_reloads`. |
| `backend/modules/finances/types.go` | `Transaction` gains `FromCardID`/`ToCardID`; `Card` loses `InitialBalanceCents`; `transactionRequest` rejects `transfer` and now requires `CardID int64` (not `*int64`); `subscriptionRequest.CardID` becomes `int64` (required); new `cardTransferRequest`; `savingsGoalRequest.DefaultCardID` required; new shared `rejectCreditCardForInflow` + `errCreditInflow`. |
| `backend/modules/finances/cards.go` | Rewrite `scanCard`/`ListCards`/`CreateCard` for computed balance + seed transfer; `CreateCardTransfer` uses the shared inflow helper on both sides; `DeleteCard` gains last-active-card invariant (409). **Reload handlers (`ListReloads`/`CreateReload`/`scanCardReload`) DELETED** — the reload concept is removed. `cardsBaseQuery` CASE gains the mirrored income branch. |
| `backend/modules/finances/transactions.go` | Delete `cardDelta`/`cardAdjustment`/`applyCardDeltas`; add `cardBalance` (lock + compute) helper; rewrite the balance-check parts of `CreateTransaction`/`UpdateTransaction`; reject `type=transfer` in both plus `DeleteTransaction`. **Mandatory-card layer:** `cardBalance` CASE gains the guarded income branch; `CreateTransaction`/`UpdateTransaction` always resolve cardType + run `rejectCreditCardForInflow` on income; `cardId` no longer optional. `RefundTransaction` UNCHANGED (the income branch makes refund repone saldo automatically). |
| `backend/modules/finances/subscriptions.go` | `CreateSubscription`/`UpdateSubscription` drop the `!=nil` guard, always call `subscriptionCardExists`; `processDue` emits an expense row always tagged with the subscription's `card_id` (`int64`, not `sql.NullInt64`). |
| `backend/modules/finances/savings.go` | `CreateContribution` rewritten around the balance-check helper + transfer insert + `transaction_id` link; `CreateGoal`/`UpdateGoal` validate `defaultCardId` is required and non-`credito`. |
| `backend/modules/finances/summary.go` | Add `AND type != 'transfer'` to the balance and trend queries. No mandatory-card change — "Disponible" is frontend-only. |
| `backend/modules/finances/routes.go` | Add `POST /cards/transfers`. **Mandatory-card layer:** remove the reload routes (`GET/POST /cards/{id}/reloads`). |

### 1.2 Migration (`store/migrate.go`)

Appended to the `stmts` slice. **Must run against truncated finance data**
— see §4. Truncate-first, no backfill: legacy NULL `card_id` rows on
`transactions` (income/expense) and on `subscriptions` are lost, same
convention as `savings_goals.default_card_id SET NOT NULL`.

```go
// --- Unified transfer ledger ---
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

// card_reloads is no longer a thing — the reload concept was removed by
// the mandatory-card model. Idempotent.
`DROP TABLE IF EXISTS card_reloads`,

`ALTER TABLE savings_goals ALTER COLUMN default_card_id SET NOT NULL`,
`ALTER TABLE savings_contributions ADD COLUMN IF NOT EXISTS transaction_id BIGINT REFERENCES transactions(id)`,

// --- Mandatory-card account model ---
// card_id is mandatory for income/expense. CHECK (not column NOT NULL)
// because transfer rows legitimately carry NULL card_id; a column-wide
// NOT NULL would break every transfer writer.
`ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_card_id_required_for_income_expense`,
`ALTER TABLE transactions ADD CONSTRAINT transactions_card_id_required_for_income_expense
   CHECK (type = 'transfer' OR card_id IS NOT NULL)`,
// Subscriptions have no transfer variant — straight NOT NULL.
`ALTER TABLE subscriptions ALTER COLUMN card_id SET NOT NULL`,
`DROP TABLE IF EXISTS card_reloads`, // idempotent safety; already dropped above
```

### 1.3 `types.go`

```go
const transactionTypeTransfer = "transfer"

// cardId is mandatory on the write path. Transfer rows never go through
// this shape — the three transfer writers (CreateCard's seed,
// CreateContribution, CreateCardTransfer) insert directly with
// from_/to_card_id and a NULL card_id, which the CHECK constraint allows.
type transactionRequest struct {
    Type        string `json:"type"`
    AmountCents int64  `json:"amountCents"`
    Category    string `json:"category"`
    Description string `json:"description"`
    Date        string `json:"date"`
    CardID      int64  `json:"cardId"` // required (was *int64)
}

func (r transactionRequest) validate() (time.Time, error) {
    if r.Type != "income" && r.Type != "expense" {
        return time.Time{}, fmt.Errorf("invalid type: %s", r.Type)
    }
    // ... amount/category/date checks ...
    if r.CardID <= 0 {
        return time.Time{}, fmt.Errorf("cardId requerido")
    }
    return d, nil
}

// subscriptionRequest.CardID is required too — processDue always emits a
// tagged expense, so the "Sin tarjeta" state is structurally impossible.
type subscriptionRequest struct {
    // ...
    CardID int64 `json:"cardId"` // required (was *int64)
}

// Shared credito-inflow guard. Applied at CreateTransaction (income) and
// CreateCardTransfer (both sides). NOT applied to RefundTransaction (an
// internal compensating entry, not a user income-tag) nor to subscriptions
// (processDue writes expense, allowed on credito).
var errCreditInflow = errors.New("no se puede taggear ingresos a una tarjeta de crédito")
func rejectCreditCardForInflow(cardType string) error {
    if cardType == cardTypeCredit { return errCreditInflow }
    return nil
}

// Transaction read model: CardID stays *int64 — transfer rows legitimately
// have it NULL. scanTransaction unchanged.
type Transaction struct {
    ID          int64  `json:"id"`
    Type        string `json:"type"`
    // ...
    CardID      *int64 `json:"cardId,omitempty"`
    FromCardID  *int64 `json:"fromCardId,omitempty"` // transfer only
    ToCardID    *int64 `json:"toCardId,omitempty"`   // transfer only
}

// New — card-to-card transfer.
type cardTransferRequest struct {
    FromCardID  int64  `json:"fromCardId"`
    ToCardID    int64  `json:"toCardId"`
    AmountCents int64  `json:"amountCents"`
    Date        string `json:"date"`
    Note        string `json:"note"`
}
```

### 1.4 `transactions.go` — `cardBalance` and the income branch

**Deleted entirely**: `cardDelta` struct, `cardAdjustment()`,
`(cardDelta) reverse()`, `(cardDelta) isZero()`, `applyCardDeltas()`,
`addDelta()`. Nothing computes or persists a delta anymore.

**`cardBalance`** — lock + compute, used by every flow that needs to check
"does this card have enough". Now carries the **income branch** (the
mandatory-card layer):

```go
func cardBalance(tx *sql.Tx, cardID int64, excludeTxID int64) (balanceCents, usedCreditCents int64, cardType string, err error) {
    err = tx.QueryRow(
        `SELECT type FROM cards WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
        cardID,
    ).Scan(&cardType)
    if err != nil { return 0, 0, "", err }

    err = tx.QueryRow(
        `SELECT
           COALESCE(SUM(CASE
             WHEN to_card_id   = $1 THEN amount_cents
             WHEN from_card_id = $1 THEN -amount_cents
             WHEN card_id = $1 AND type = 'income'  AND $2 != 'credito' THEN  amount_cents
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

The `WHEN card_id = $1 AND type = 'income' AND $2 != 'credito' THEN +amount`
branch is what makes income move a non-credito card's balance, and what
makes `RefundTransaction` repone saldo with **no code change** (its
compensating INSERT is already `type='income', ..., old.CardID`). The
`$2 != 'credito'` predicate is defense in depth on top of the
handler-level `rejectCreditCardForInflow` guard — even if an income row
were somehow tagged to a credito card, the CASE would ignore it.

**`CreateTransaction`** — `cardId` is now always present (validated
upstream). Always resolve `cardType` via `cardTypeOwned` (so a wrong-owner
card surfaces as 404 before the write opens), and on `income` run
`rejectCreditCardForInflow(cardType)`. The balance check for an expense
tagged to a debito/prepago card is the same lock-then-refuse shape as
before:

```go
cardType, err := h.cardTypeOwned(req.CardID, role, userID)
if err == sql.ErrNoRows { /* 404 */ }
if req.Type == "income" {
    if e := rejectCreditCardForInflow(cardType); e != nil {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, e.Error())
        return
    }
}
if req.Type == "expense" {
    balanceCents, _, _, err := cardBalance(tx, req.CardID, 0)
    if cardType != cardTypeCredit && balanceCents < req.AmountCents {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "saldo insuficiente en tarjeta")
        return
    }
}
// INSERT ... no applyCardDeltas afterward.
```

**`UpdateTransaction`** — same shape, but `cardBalance(tx, cardID,
editingTxID)` excludes the row being edited so the transaction's own prior
amount isn't counted against itself.

**`DeleteTransaction`** — no balance handling at all (soft-delete already
excludes the row from every future `cardBalance`). Rejects
`type='transfer'` (404 — a transfer isn't part of the editable ledger from
this endpoint's perspective).

**`RefundTransaction`** — **unchanged**. Inserts `type='income', ...,
old.CardID`; the income branch in `cardBalance` repone saldo
automatically. Refund of a credito expense does not reduce
`used_credit_cents` — a known edge deferred to a future credit_lines split
(see SPEC.md open question).

### 1.5 `cards.go`

**`scanCard`/`ListCards`** — `cardsBaseQuery` carries the computed balance
subqueries, with the mirrored income branch:

```sql
SELECT
  c.id, c.name, c.type, c.bank, c.last4, c.color, c.icon,
  c.credit_limit_cents, c.created_at,
  COALESCE((SELECT SUM(CASE
    WHEN t.to_card_id   = c.id THEN t.amount_cents
    WHEN t.from_card_id = c.id THEN -t.amount_cents
    WHEN t.card_id = c.id AND t.type = 'income'  AND c.type != 'credito' THEN  t.amount_cents
    WHEN t.card_id = c.id AND t.type = 'expense' AND c.type != 'credito' THEN -t.amount_cents
    ELSE 0 END)
   FROM transactions t
   WHERE t.deleted_at IS NULL
     AND (t.to_card_id = c.id OR t.from_card_id = c.id OR t.card_id = c.id)), 0) AS balance_cents,
  COALESCE((SELECT SUM(t.amount_cents) FROM transactions t
   WHERE t.deleted_at IS NULL AND t.card_id = c.id AND t.type = 'expense' AND c.type = 'credito'), 0) AS used_credit_cents
FROM cards c
WHERE c.deleted_at IS NULL
```

`scanCard` scans this column order (no more `InitialBalanceCents`).

**`CreateCard`** — inserts the card, then (same DB transaction) a seed
transfer if `InitialBalanceCents > 0`:

```go
INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, to_card_id)
VALUES ('transfer', $1, 'transferencia', 'Saldo inicial', CURRENT_DATE, $2, $3)
```

**`DeleteCard`** — last-active-card invariant (409):

```go
var n int
tx.QueryRow(`SELECT COUNT(*) FROM cards WHERE created_by = $1 AND deleted_at IS NULL`, userID).Scan(&n)
if n <= 1 {
    tx.Rollback()
    httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict,
        "no podés archivar tu última tarjeta activa")
    return
}
// ... proceed to soft-delete
```

The COUNT is scoped by owner and fires **before** any mutation. Frontend
disables the delete affordance when `cards.length === 1` (progressive UX;
the 409 is the authoritative guard).

**`CreateCardTransfer`** — locks both cards in id-sorted order, runs
`rejectCreditCardForInflow` on both sides (the shared helper replaces the
old inline credito check), then `cardBalance` for the source's sufficiency
check, then a single `transfer` INSERT with both `from_card_id`/`to_card_id`
set. 201.

**Reload handlers (`CreateReload`/`ListReloads`/`scanCardReload`)** —
**DELETED**. The reload concept is removed by the mandatory-card model; the
`POST/GET /cards/{id}/reloads` routes are removed in §1.8. Recharge a card
by tagging an income to it instead.

### 1.6 `subscriptions.go`

`CreateSubscription`/`UpdateSubscription` drop the `!=nil` guard, always
call `subscriptionCardExists` (bogus card → 404). `processDue` emits an
expense row always tagged with the subscription's `card_id` (`int64`,
not `sql.NullInt64`). No credito inflow guard here — subscriptions charge
as expense, allowed on credito cards (design decision).

### 1.7 `savings.go`

`CreateGoal`/`UpdateGoal` validate `defaultCardId` is required and
non-`credito` (reuse `cardTypeOwned`). `CreateContribution` rewritten
around `cardBalance` + a transfer insert + `transaction_id` link — same
shape as the v1 design; the mandatory-card layer doesn't touch savings
(`default_card_id` was already mandatory in the transfer-ledger change).

### 1.8 `routes.go`

```go
r.Post("/cards/transfers", h.CreateCardTransfer)
// reload routes REMOVED — the reload concept is gone:
//   r.Get ("/cards/{id}/reloads", h.ListReloads)    — deleted
//   r.Post("/cards/{id}/reloads", h.CreateReload)   — deleted
```

---

## 2. Frontend

No response shape changes for `Card`, `Transaction`, or subscription list
endpoints (`initialBalanceCents` removed from `Card`; `fromCardId`/
`toCardId` added to `Transaction` as optional fields).

### 2.1 `types/finance.types.ts` + `hooks/useFinanceApi.ts`

- `Card`: remove `initialBalanceCents`.
- `CardInput`: keep `initialBalanceCents` (still sent on create).
- `Transaction`: add `fromCardId?: number`, `toCardId?: number`.
- `TransactionInput`: `cardId` becomes **required** (`number`).
- `SavingsGoal`/`SavingsGoalInput`: `defaultCardId` required (`number`).
- New: `CardTransferInput`.
- `CardReload`/`CardReloadInput`/`createReload`/`listReloads`: **removed**
  (their endpoints are gone).

### 2.2 `TarjetasTab.tsx`

- **ReloadForm/Recargar button removed** — no reload flow anymore.
- New action alongside (not replacing) "Transferir" on each non-credito card.
- **delete-last-card** — Trash disabled when `cards.length === 1`,
  `aria-label`/`title` "No podés archivar tu última tarjeta activa"; toast
  on the 409 (backend is authoritative).

### 2.3 `TransactionForm.tsx`

`cardId` schema `z.string().min(1, 'Elegí una tarjeta')` (required). Drop
the "(opcional)" label and the "Ninguna"/"Sin tarjeta" placeholder option.

### 2.4 `FinancesPage.tsx` — page-level onboarding gate

On mount, call `listCards`; count non-credito cards. If 0, render the gate
(header "Finanzas" + a centered "Para iniciar con tus finanzas / Agregá una
tarjeta de débito o prepago" + inline `CardFormFields blockCredit`), hide
all tabs + the mobile nav. When the first non-credito card is created,
`onSaved` re-runs `listCards`, the count goes to 1, and the gate lifts.
`blockCredit` prevents the user from locking themselves out by picking
crédito as their first card (crédito-alone doesn't clear the gate).

### 2.5 `ResumenTab.tsx` — "Disponible" tile

Tile label "Balance total" → **"Disponible"**; value =
`summary.balanceCents − Σ listGoals().currentCents` (computed frontend-side
— no new backend endpoint). Removed the "Sin asignar" reconciliation line
and its diff computation (structurally impossible now; every income/expense
has a card). The "En metas de ahorro" breakdown line stays.

### 2.6 `MovimientosTab.tsx` — refund copy

Refund dialog copy: "El reembolso agregará al balance total, pero no
repondrá el saldo de la tarjeta." → **"El reembolso sí repondrá el saldo de
la tarjeta."** (matches the new `cardBalance` income-branch behavior; the
backend `RefundTransaction` needs no change).

### 2.7 `MetasTab.tsx` / `GoalForm`

- Card select required (`z.string().min(1)`), filtered to exclude `credito`.
- Remove the "Sin tarjeta predeterminada" option.
- Remove the amber "Sin tarjeta — se descontará saldo general" warning
  block (every goal has a card now).
- `ContributionForm` surfaces 409 (default card archived) via the existing
  generic `toast.error(err.message)` — no special-case UI.

---

## 3. What does NOT change

- `PresupuestosTab.tsx`, `SuscripcionesTab.tsx` (shape) — untouched.
- `MovimientosTab.tsx` — no new filter, no transfer rows surfacing there.
- Card, transaction, and goal soft-delete (`deleted_at`) — unrelated
  mechanism, already correct; both changes only add `AND deleted_at IS
  NULL` to new aggregates the same way every existing query does.
- `summary.go` — the "Balance total" → "Disponible" rename and the
  `− Σ goals.currentCents` subtraction are **frontend-only**; the backend
  `balanceQuery` already filters `type != 'transfer'` and stays unchanged
  by the mandatory-card layer.

---

## 4. Migration operational note

`ALTER TABLE savings_goals ALTER COLUMN default_card_id SET NOT NULL`,
`ALTER TABLE subscriptions ALTER COLUMN card_id SET NOT NULL`, and the
`transactions.card_id` CHECK constraint all fail at startup if existing
rows violate them (NULL `card_id` on income/expense transactions, NULL
`card_id` on subscriptions, NULL `default_card_id` on goals). Per SPEC.md's
no-backfill decision, these ship against clean data — truncate
`transactions, cards, card_reloads, savings_goals, savings_contributions`
(cascade) before starting the backend on the new migration for the first
time in any environment carrying pre-change data. The `card_reloads` DROP
is idempotent (already dropped at a prior step; kept for safety).