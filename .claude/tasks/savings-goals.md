# Task: Implement Savings Goals

Read and follow: SPEC.md and DESIGN.md in the project root.

## Files to create/modify

### Backend
1. `backend/modules/finances/types.go` — add SavingsGoal, SavingsContribution structs + savingsGoalRequest + savingsContributionRequest with validate() methods
2. `backend/modules/finances/savings.go` — CREATE THIS FILE with: ListGoals, CreateGoal, UpdateGoal, DeleteGoal, goalOwned, ListContributions, CreateContribution (atomic tx with BEGIN/COMMIT)
3. `backend/modules/finances/routes.go` — add routes for savings-goals CRUD and contributions
4. `backend/store/migrate.go` — add savings_goals and savings_contributions tables

### Frontend
5. `frontend/src/types/finance.types.ts` — add SavingsGoal, SavingsContribution, SavingsGoalInput interfaces
6. `frontend/src/hooks/useFinanceApi.ts` — add: listGoals, createGoal, updateGoal, deleteGoal, listContributions, createContribution
7. `frontend/src/pages/Finances/MetasTab.tsx` — CREATE THIS FILE with GoalForm dialog, ContributionForm dialog, MetasTab grid with progress bars, FAB
8. `frontend/src/pages/Finances/FinancesPage.tsx` — add "Metas" tab to TABS array and TabsContent

## Rules
- Follow exact patterns from cards.go and TarjetasTab.tsx (they are the reference)
- Money in cents (int64), display with formatPEN
- Per-user via scopeToOwner helper (same as cards/subscriptions)
- Atomic transaction for CreateContribution: tx.Begin() -> INSERT savings_contributions -> UPDATE savings_goals current_cents -> tx.Commit()
- Spanish UI labels: Metas, Nueva meta, Agregar ahorro, icono, color, fecha, progreso
- Colors array: #10b981 (green), #3b82f6 (blue), #f59e0b (amber), #ef4444 (red), #8b5cf6 (purple), #06b6d4 (cyan), #ec4899 (pink), #84cc16 (lime)
- Icons: piggy-bank, target, plane, home, car, graduation-cap

## After all code
1. Backend: `cd ~/work/wotk-hub/backend && go fmt ./... && go vet ./... && go build ./... && echo BACKEND_OK`
2. Frontend: `cd ~/work/wotk-hub/frontend && npx eslint src/pages/Finances/MetasTab.tsx src/pages/Finances/FinancesPage.tsx src/hooks/useFinanceApi.ts src/types/finance.types.ts && npx tsc --noEmit && echo FRONTEND_OK`
3. Commit: `cd ~/work/wotk-hub && git checkout -b feat/savings-goals && git add -A && git commit -m "feat: add savings goals (metas de ahorro) to finances"`
