# SPEC — Savings Goals (Metas de Ahorro) in Finances

## Decision log

- Savings goals are **per-user**: scoped by `created_by = current user ID` from the JWT
  cookie, same ownership pattern as `transactions`, `cards`, and `subscriptions`
  (see `scopeToOwner` in `backend/modules/finances/helpers.go`). Not shared household state.
- `current_cents` on `savings_goals` is denormalized — updated atomically inside the
  same transaction as each `savings_contributions` insert. No separate "recalc" step.
- Contributions are purely additive — users can only add to a goal, not withdraw.
  Withdrawal is out of scope for v1 and can be added later as a signed amount or a
  separate `withdrawals` table.
- Goals without a `deadline` are valid (open-ended saving).

---

## Data model

### `savings_goals`

| column        | type                       | notes                                              |
|---------------|----------------------------|----------------------------------------------------|
| id            | BIGSERIAL PK               |                                                    |
| name          | TEXT NOT NULL               | e.g. "Viaje a Cusco", "Fondo emergencial"         |
| target_cents  | BIGINT NOT NULL CHECK (target_cents > 0) |                                      |
| current_cents | BIGINT NOT NULL DEFAULT 0   | denormalized; updated on each contribution         |
| deadline      | DATE                       | nullable; optional target date                     |
| icon          | TEXT NOT NULL DEFAULT 'piggy-bank' | lucide icon key                           |
| color         | TEXT NOT NULL DEFAULT '#10b981' | hex color for UI progress bar                |
| created_by    | BIGINT REFERENCES users(id) | ownership — same pattern as cards/subscriptions    |
| created_at    | TIMESTAMPTZ NOT NULL DEFAULT now() |                                           |
| updated_at    | TIMESTAMPTZ NOT NULL DEFAULT now() |                                           |

### `savings_contributions`

| column           | type                       | notes                                             |
|------------------|----------------------------|----------------------------------------------------|
| id               | BIGSERIAL PK               |                                                    |
| goal_id          | BIGINT NOT NULL REFERENCES savings_goals(id) ON DELETE CASCADE |                   |
| amount_cents     | BIGINT NOT NULL CHECK (amount_cents > 0) |                                 |
| note             | TEXT NOT NULL DEFAULT ''   | optional description of this contribution          |
| occurred_on      | DATE NOT NULL             | date of the contribution                           |
| created_by       | BIGINT REFERENCES users(id) | ownership                                         |
| created_at       | TIMESTAMPTZ NOT NULL DEFAULT now() |                                           |

---

## API

Ownership: same `scopeToOwner` deny-by-default pattern as all other finances
entities — non-admin roles only see/mutate their own goals and contributions;
admins see everything unscoped.

### Goals

- `GET /api/finances/savings-goals` → `{ goals: SavingsGoal[] }`
- `POST /api/finances/savings-goals` → `SavingsGoal` (201)
- `PUT /api/finances/savings-goals/{id}` → `SavingsGoal` (404 if not owned)
- `DELETE /api/finances/savings-goals/{id}` → `{ success: true }` (404 if not owned)

### Contributions

- `GET /api/finances/savings-goals/{id}/contributions` → `{ contributions: SavingsContribution[] }` (404 if goal not owned)
- `POST /api/finances/savings-goals/{id}/contributions` → `SavingsContribution` (201, 404 if goal not owned)

### Validation

`savingsGoalRequest.validate()`:
- `name` required, non-empty
- `targetCents` > 0
- `deadline` optional; if present must be a valid `YYYY-MM-DD` and must be in the future

`savingsContributionRequest.validate()`:
- `amountCents` > 0
- `date` valid `YYYY-MM-DD`
- `note` optional (defaults to `''`)

### Atomic contribution flow

`CreateContribution` executes in a single DB transaction:
1. Insert into `savings_contributions`.
2. Increment `savings_goals.current_cents` by `amount_cents`.

If step 1 fails, no row is inserted. If step 2 fails, the whole transaction rolls back.

---

## Frontend

### Types (`finance.types.ts`)

```ts
export interface SavingsGoal {
  id: number
  name: string
  targetCents: number
  currentCents: number
  deadline: string | null   // YYYY-MM-DD
  icon: string
  color: string
  createdAt: string
}

export interface SavingsGoalInput {
  name: string
  targetCents: number
  deadline: string | null
  icon: string
  color: string
}

export interface SavingsContribution {
  id: number
  goalId: number
  amountCents: number
  date: string   // YYYY-MM-DD
  note: string
  createdAt: string
}

export interface SavingsContributionInput {
  amountCents: number
  date: string
  note: string
}
```

### Hook (`useFinanceApi.ts`)

Same axios/api pattern as all other finance calls:

```ts
listSavingsGoals(): Promise<SavingsGoal[]>
createSavingsGoal(input: SavingsGoalInput): Promise<SavingsGoal>
updateSavingsGoal(id: number, input: SavingsGoalInput): Promise<SavingsGoal>
deleteSavingsGoal(id: number): Promise<void>
listSavingsGoalContributions(goalId: number): Promise<SavingsContribution[]>
createSavingsGoalContribution(goalId: number, input: SavingsContributionInput): Promise<SavingsContribution>
```

### Components

**`MetasTab.tsx`** — new tab added to `FinancesPage.tsx`

- Displays a `CozyCard` summary header: total saved across all goals vs. combined target.
- Grid of goal cards, each showing:
  - Icon + name + color accent bar (top border)
  - Linear progress bar (current / target)
  - Percentage complete
  - Formatted PEN amounts
  - Optional deadline badge ("X días restantes" or "Fecha: DD/MM/YYYY")
  - "Añadir" button to open `ContributionForm`
  - Edit / Delete actions via icon buttons
- Empty state with piggy-bank icon and "Sin metas de ahorro" message
- Loading spinner / error+retry pattern matching `TarjetasTabWrapper`

**`GoalForm`** (inline in `MetasTab.tsx` or extracted) — create/edit dialog

Fields: name, target amount (PEN, decimal input → cents), deadline (date input, optional), icon selector, color swatches (same palette as `TarjetasTab`).

**`ContributionForm`** — add-contribution dialog

Fields: amount (PEN), date, note (optional).
On success: refreshes the goal card (updated `currentCents`) and the contribution list.

### Page integration

- New tab entry in `FinancesPage.tsx` `TABS` array:
  ```ts
  { value: 'metas', label: 'Metas', icon: Target }
  ```
- `<TabsContent value="metas">` renders `<MetasTabWrapper />` (loading/error/data-fetching wrapper with the same pattern as `TarjetasTabWrapper`).

### Color palette

Same `CARD_COLORS` array as `TarjetasTab` is reused for goal colors.

### Icon options

Same lucide icon keys used elsewhere in the app. Defaults to `piggy-bank`.

### Formatting

- All currency amounts via `formatPEN` from `@/lib/currency`.
- Dates in Spanish locale (e.g., "15/07/2026").
- Percentage: `Math.round((currentCents / targetCents) * 100)`.

---

## Acceptance criteria

1. **CRUD goals**: user can create a goal with name, target amount, optional deadline, icon, and color; edit any of those fields; delete a goal (cascades to contributions).
2. **Contributions**: user can add a contribution to any goal; `currentCents` updates atomically; contribution history is shown in goal detail.
3. **Progress display**: each goal card shows a visual progress bar, percentage, and formatted PEN amounts.
4. **Summary header**: "Total ahorrado" vs "Meta total" shown at the top of the tab.
5. **Ownership**: users only see and manage their own goals and contributions.
6. **FloatingActionButton**: same position and behavior as other tabs.
7. **Responsive**: mobile card list (single column) + desktop grid (2–3 columns).
8. **Loading/error states**: skeleton or spinner on fetch; error message + retry button on failure.
9. **No duplicate charges**: atomic transaction prevents `current_cents` from going out of sync with `savings_contributions` rows.
10. **All labels in Spanish**, matching the existing tab language.
