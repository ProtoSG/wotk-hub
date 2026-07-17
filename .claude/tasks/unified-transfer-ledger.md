# Task: Unified Transfer Ledger for Finances

Read and follow: SPEC.md and DESIGN.md in the project root.

## Files to create/modify

### Backend
1. `backend/store/migrate.go` — transfer columns/constraint on `transactions`, drop `card_reloads`, drop `cards.balance_cents`/`initial_balance_cents`/`used_credit_cents`, `savings_goals.default_card_id` NOT NULL, `savings_contributions.transaction_id`
2. `backend/modules/finances/types.go` — `Transaction.FromCardID`/`ToCardID`, `Card` loses `InitialBalanceCents`, new `cardTransferRequest`, `savingsGoalRequest.DefaultCardID` required
3. `backend/modules/finances/cards.go` — computed-balance `scanCard`/`ListCards`, seed transfer in `CreateCard`, `CreateReload`/`ListReloads` rewritten over `transactions`, new `CreateCardTransfer`
4. `backend/modules/finances/transactions.go` — delete `cardDelta`/`cardAdjustment`/`applyCardDeltas`/`addDelta`, add `cardBalance` helper, reject `type=transfer` in Create/Update/Delete
5. `backend/modules/finances/savings.go` — `CreateContribution` uses `cardBalance` + transfer insert + `transaction_id` link; `CreateGoal`/`UpdateGoal` validate default card is owned + non-`credito`
6. `backend/modules/finances/summary.go` — `AND type != 'transfer'` on balance + trend queries
7. `backend/modules/finances/routes.go` — add `POST /cards/transfers`

### Frontend
8. `frontend/src/types/finance.types.ts` — `Card` loses `initialBalanceCents`, `Transaction` gains `fromCardId`/`toCardId`, `SavingsGoal.defaultCardId` required, new `CardTransferInput`
9. `frontend/src/hooks/useFinanceApi.ts` — add `createCardTransfer`
10. `frontend/src/pages/Finances/TarjetasTab.tsx` — "Transferir" action (destination card select, excludes credito)
11. `frontend/src/pages/Finances/MetasTab.tsx` — `GoalForm` card select becomes required, filtered to exclude credito, remove "sin tarjeta" state entirely

## Rules
- Follow SPEC.md's decision log exactly — especially: no historical backfill (truncate before migrating), transfer type never user-selectable, category always `'transferencia'` fixed.
- Balance/used-credit are computed, never stored — see `cardBalance` in DESIGN.md §1.4.
- Lock order for anything touching 2 cards: sort by id first (existing deadlock-avoidance rule, same as the old `applyCardDeltas`).
- Money in cents (int64) everywhere, Spanish user-facing strings.

## After all code
1. Truncate finance tables in dev DB before first run against the new migration.
2. Backend: `cd backend && go build ./... && go vet ./... && echo BACKEND_OK`
3. Frontend: `cd frontend && bun run lint && bunx tsc -b --noEmit && echo FRONTEND_OK`
4. Manually re-verify every scenario in SPEC.md's "Scenarios" sections against a live backend (contribution happy path, insufficient balance, archived default card, card-to-card transfer, reload, summary reconciliation) before considering this done.
