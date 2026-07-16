# DESIGN.md — Savings Goals (Metas de Ahorro)

## Overview

Savings Goals is a per-user feature that lets users set monetary targets and track contributions over time. It follows the same ownership, request-validation, and atomic-transaction patterns already established in `backend/modules/finances/` and the existing frontend tab architecture.

---

## 1. Backend

### 1.1 Files to create/modify

| File | Action | Purpose |
|---|---|---|
| `backend/modules/finances/savings.go` | **Create** | All goal and contribution handlers + `goalOwned` helper |
| `backend/modules/finances/types.go` | **Modify** | Add `SavingsGoal`, `SavingsContribution`, `savingsGoalRequest`, `savingsContributionRequest` types |
| `backend/modules/finances/routes.go` | **Modify** | Register new goal and contribution routes |
| `backend/store/migrate.go` | **Modify** | Add `savings_goals` and `savings_contributions` DDL |

### 1.2 New types (`types.go` additions)

```go
// --- structs ---

type SavingsGoal struct {
    ID           int64  `json:"id"`
    Name         string `json:"name"`
    TargetCents  int64  `json:"targetCents"`
    CurrentCents int64  `json:"currentCents"`
    Deadline     string `json:"deadline"`      // "YYYY-MM-DD" or ""
    Icon         string `json:"icon"`
    Color        string `json:"color"`
    CreatedAt    string `json:"createdAt"`
}

type SavingsContribution struct {
    ID          int64  `json:"id"`
    GoalID      int64  `json:"goalId"`
    AmountCents int64  `json:"amountCents"`
    Date        string `json:"date"`          // "YYYY-MM-DD"
    Note        string `json:"note"`
    CreatedAt   string `json:"createdAt"`
}

// --- request/validation ---

type savingsGoalRequest struct {
    Name        string `json:"name"`
    TargetCents int64  `json:"targetCents"`
    Deadline    string `json:"deadline"`      // "" = no deadline
    Icon        string `json:"icon"`
    Color       string `json:"color"`
}

func (r savingsGoalRequest) validate() error {
    if strings.TrimSpace(r.Name) == "" {
        return errors.New("name is required")
    }
    if r.TargetCents <= 0 {
        return errors.New("targetCents must be positive")
    }
    if r.Deadline != "" {
        d, err := time.Parse("2006-01-02", r.Deadline)
        if err != nil {
            return errors.New("invalid deadline format, use YYYY-MM-DD")
        }
        if d.Before(time.Now().Truncate(24 * time.Hour)) {
            return errors.New("deadline must be today or in the future")
        }
    }
    return nil
}

type savingsContributionRequest struct {
    AmountCents int64  `json:"amountCents"`
    Date       string `json:"date"`
    Note       string `json:"note"`
}

func (r savingsContributionRequest) validate() (time.Time, error) {
    if r.AmountCents <= 0 {
        return time.Time{}, errors.New("amountCents must be positive")
    }
    d, err := time.Parse("2006-01-02", r.Date)
    if err != nil {
        return time.Time{}, errors.New("invalid date, use YYYY-MM-DD")
    }
    return d, nil
}
```

### 1.3 Handler signatures (`savings.go`)

```go
package finances

import (
    "database/sql"
    "errors"
    "log"
    "net/http"
    "time"

    "workhub/httpx"
    "workhub/middleware"

    chi "github.com/go-chi/chi/v5"
)

// --- scan helpers ---

func scanGoal(row interface{ Scan(...any) error }) (SavingsGoal, error) {
    var g SavingsGoal
    var deadline interface{} // nullable DATE
    var createdAt time.Time
    err := row.Scan(&g.ID, &g.Name, &g.TargetCents, &g.CurrentCents,
        &deadline, &g.Icon, &g.Color, &g.CreatedAt)
    if err != nil {
        return g, err
    }
    if deadline != nil {
        g.Deadline = deadline.(time.Time).Format("2006-01-02")
    }
    g.CreatedAt = createdAt.Format(time.RFC3339)
    return g, nil
}

func scanContribution(row interface{ Scan(...any) error }) (SavingsContribution, error) {
    var c SavingsContribution
    var occurredOn, createdAt time.Time
    err := row.Scan(&c.ID, &c.GoalID, &c.AmountCents, &occurredOn, &c.Note, &createdAt)
    if err != nil {
        return c, err
    }
    c.Date = occurredOn.Format("2006-01-02")
    c.CreatedAt = createdAt.Format(time.RFC3339)
    return c, nil
}

// --- goal ownership guard ---

func (h *handler) goalOwned(id int64, role string, userID int64) error {
    query := `SELECT id FROM savings_goals WHERE id = $1`
    args := []any{id}
    query, args = scopeToOwner(query, args, role, userID)
    var got int64
    return h.db.QueryRow(query, args...).Scan(&got)
}

// --- handlers ---

func (h *handler) ListGoals(w http.ResponseWriter, r *http.Request) {
    userID, role, ok := middleware.UserFromContext(r.Context())
    if !ok {
        httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
        return
    }
    query := `SELECT id, name, target_cents, current_cents, deadline, icon, color, created_at
              FROM savings_goals WHERE 1=1`
    args := []any{}
    query, args = scopeToOwner(query, args, role, userID)
    query += " ORDER BY id DESC"

    rows, err := h.db.Query(query, args...)
    if err != nil {
        log.Printf("finances: list goals failed: %v", err)
        httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
        return
    }
    defer rows.Close()

    goals := []SavingsGoal{}
    for rows.Next() {
        g, err := scanGoal(rows)
        if err != nil {
            log.Printf("finances: scan goal failed: %v", err)
            httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
            return
        }
        goals = append(goals, g)
    }
    httpx.WriteJSON(w, http.StatusOK, map[string]any{"goals": goals})
}

func (h *handler) CreateGoal(w http.ResponseWriter, r *http.Request) {
    userID, _, ok := middleware.UserFromContext(r.Context())
    if !ok {
        httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
        return
    }
    var req savingsGoalRequest
    if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
        return
    }
    if err := req.validate(); err != nil {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
        return
    }

    deadline := (*time.Time)(nil)
    if req.Deadline != "" {
        t, _ := time.Parse("2006-01-02", req.Deadline)
        deadline = &t
    }

    row := h.db.QueryRow(
        `INSERT INTO savings_goals (name, target_cents, current_cents, deadline, icon, color, created_by)
         VALUES ($1, $2, 0, $3, $4, $5, $6)
         RETURNING id, name, target_cents, current_cents, deadline, icon, color, created_at`,
        req.Name, req.TargetCents, deadline, req.Icon, req.Color, userID,
    )
    g, err := scanGoal(row)
    if err != nil {
        log.Printf("finances: create goal failed: %v", err)
        httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
        return
    }
    httpx.WriteJSON(w, http.StatusCreated, g)
}

func (h *handler) UpdateGoal(w http.ResponseWriter, r *http.Request) {
    userID, role, ok := middleware.UserFromContext(r.Context())
    if !ok {
        httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
        return
    }
    id, err := parseID(r)
    if err != nil {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
        return
    }
    var req savingsGoalRequest
    if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
        return
    }
    if err := req.validate(); err != nil {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
        return
    }

    deadline := (*time.Time)(nil)
    if req.Deadline != "" {
        t, _ := time.Parse("2006-01-02", req.Deadline)
        deadline = &t
    }

    query := `UPDATE savings_goals
              SET name = $1, target_cents = $2, deadline = $3, icon = $4, color = $5
              WHERE id = $6`
    args := []any{req.Name, req.TargetCents, deadline, req.Icon, req.Color, id}
    query, args = scopeToOwner(query, args, role, userID)
    query += ` RETURNING id, name, target_cents, current_cents, deadline, icon, color, created_at`

    row := h.db.QueryRow(query, args...)
    g, err := scanGoal(row)
    if errors.Is(err, sql.ErrNoRows) {
        httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "goal not found")
        return
    }
    if err != nil {
        log.Printf("finances: update goal failed: %v", err)
        httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
        return
    }
    httpx.WriteJSON(w, http.StatusOK, g)
}

func (h *handler) DeleteGoal(w http.ResponseWriter, r *http.Request) {
    userID, role, ok := middleware.UserFromContext(r.Context())
    if !ok {
        httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
        return
    }
    id, err := parseID(r)
    if err != nil {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
        return
    }
    query := `DELETE FROM savings_goals WHERE id = $1`
    args := []any{id}
    query, args = scopeToOwner(query, args, role, userID)

    res, err := h.db.Exec(query, args...)
    if err != nil {
        log.Printf("finances: delete goal failed: %v", err)
        httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
        return
    }
    if n, _ := res.RowsAffected(); n == 0 {
        httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "goal not found")
        return
    }
    httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *handler) ListContributions(w http.ResponseWriter, r *http.Request) {
    userID, role, ok := middleware.UserFromContext(r.Context())
    if !ok {
        httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
        return
    }
    id, err := parseID(r)
    if err != nil {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
        return
    }
    if err := h.goalOwned(id, role, userID); errors.Is(err, sql.ErrNoRows) {
        httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "goal not found")
        return
    } else if err != nil {
        log.Printf("finances: list contributions failed: %v", err)
        httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
        return
    }

    rows, err := h.db.Query(
        `SELECT id, goal_id, amount_cents, occurred_on, note, created_at
         FROM savings_contributions WHERE goal_id = $1 ORDER BY occurred_on DESC, id DESC`,
        id,
    )
    if err != nil {
        log.Printf("finances: list contributions failed: %v", err)
        httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
        return
    }
    defer rows.Close()

    contributions := []SavingsContribution{}
    for rows.Next() {
        c, err := scanContribution(rows)
        if err != nil {
            log.Printf("finances: scan contribution failed: %v", err)
            httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
            return
        }
        contributions = append(contributions, c)
    }
    httpx.WriteJSON(w, http.StatusOK, map[string]any{"contributions": contributions})
}

// CreateContribution — atomic two-step transaction:
//   1. INSERT savings_contributions
//   2. UPDATE savings_goals SET current_cents = current_cents + $2
// Both succeed or both roll back. This keeps current_cents denormalized
// but consistent with the sum of all contribution rows.
func (h *handler) CreateContribution(w http.ResponseWriter, r *http.Request) {
    userID, role, ok := middleware.UserFromContext(r.Context())
    if !ok {
        httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
        return
    }
    id, err := parseID(r)
    if err != nil {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
        return
    }
    if err := h.goalOwned(id, role, userID); errors.Is(err, sql.ErrNoRows) {
        httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "goal not found")
        return
    } else if err != nil {
        log.Printf("finances: create contribution ownership check failed: %v", err)
        httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
        return
    }

    var req savingsContributionRequest
    if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
        return
    }
    occurredOn, err := req.validate()
    if err != nil {
        httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
        return
    }

    tx, err := h.db.Begin()
    if err != nil {
        log.Printf("finances: create contribution failed: %v", err)
        httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
        return
    }

    // Step 1 — insert contribution row
    row := tx.QueryRow(
        `INSERT INTO savings_contributions (goal_id, amount_cents, occurred_on, note, created_by)
         VALUES ($1, $2, $3, $4, $5)
         RETURNING id, goal_id, amount_cents, occurred_on, note, created_at`,
        id, req.AmountCents, occurredOn, req.Note, userID,
    )
    c, err := scanContribution(row)
    if err != nil {
        tx.Rollback()
        log.Printf("finances: create contribution failed: %v", err)
        httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
        return
    }

    // Step 2 — increment goal's current_cents
    if _, err := tx.Exec(
        `UPDATE savings_goals SET current_cents = current_cents + $1 WHERE id = $2`,
        req.AmountCents, id,
    ); err != nil {
        tx.Rollback()
        log.Printf("finances: create contribution failed: %v", err)
        httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
        return
    }

    if err := tx.Commit(); err != nil {
        log.Printf("finances: create contribution failed: %v", err)
        httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
        return
    }

    httpx.WriteJSON(w, http.StatusCreated, c)
}
```

### 1.4 Routes additions (`routes.go`)

Add after the existing card routes:

```go
r.Get("/savings-goals", h.ListGoals)
r.Post("/savings-goals", h.CreateGoal)
r.Put("/savings-goals/{id}", h.UpdateGoal)
r.Delete("/savings-goals/{id}", h.DeleteGoal)
r.Get("/savings-goals/{id}/contributions", h.ListContributions)
r.Post("/savings-goals/{id}/contributions", h.CreateContribution)
```

### 1.5 Database migration additions (`store/migrate.go`)

Append to the `stmts` slice:

```go
`CREATE TABLE IF NOT EXISTS savings_goals (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT    NOT NULL,
    target_cents  BIGINT  NOT NULL CHECK (target_cents > 0),
    current_cents BIGINT  NOT NULL DEFAULT 0,
    deadline      DATE,
    icon          TEXT    NOT NULL DEFAULT 'piggy-bank',
    color         TEXT    NOT NULL DEFAULT '#10b981',
    created_by    BIGINT  REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
)`,

`ALTER TABLE savings_goals ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`,

`CREATE TABLE IF NOT EXISTS savings_contributions (
    id           BIGSERIAL PRIMARY KEY,
    goal_id      BIGINT  NOT NULL REFERENCES savings_goals(id) ON DELETE CASCADE,
    amount_cents BIGINT  NOT NULL CHECK (amount_cents > 0),
    note         TEXT    NOT NULL DEFAULT '',
    occurred_on  DATE    NOT NULL,
    created_by   BIGINT  REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
)`,

`CREATE INDEX IF NOT EXISTS idx_savings_contributions_goal_id ON savings_contributions (goal_id)`,
`CREATE INDEX IF NOT EXISTS idx_savings_contributions_occurred_on ON savings_contributions (occurred_on)`,
```

---

## 2. Frontend

### 2.1 Types (`src/types/finance.types.ts`)

Add to the existing exports:

```ts
export interface SavingsGoal {
  id: number
  name: string
  targetCents: number
  currentCents: number
  deadline: string | null   // "YYYY-MM-DD"
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
  date: string   // "YYYY-MM-DD"
  note: string
  createdAt: string
}

export interface SavingsContributionInput {
  amountCents: number
  date: string
  note: string
}
```

### 2.2 Hook additions (`src/hooks/useFinanceApi.ts`)

Append to the imports and return block:

```ts
import type {
  // ... existing imports
  SavingsGoal,
  SavingsGoalInput,
  SavingsContribution,
  SavingsContributionInput,
} from '@/types/finance.types'

// Inside useFinanceApi return:

async function listSavingsGoals(): Promise<SavingsGoal[]> {
  const res = await api.get<{ goals: SavingsGoal[] }>('/api/finances/savings-goals')
  return res.data.goals
}

async function createSavingsGoal(input: SavingsGoalInput): Promise<SavingsGoal> {
  const res = await api.post<SavingsGoal>('/api/finances/savings-goals', input)
  return res.data
}

async function updateSavingsGoal(id: number, input: SavingsGoalInput): Promise<SavingsGoal> {
  const res = await api.put<SavingsGoal>(`/api/finances/savings-goals/${id}`, input)
  return res.data
}

async function deleteSavingsGoal(id: number): Promise<void> {
  await api.delete(`/api/finances/savings-goals/${id}`)
}

async function listSavingsGoalContributions(goalId: number): Promise<SavingsContribution[]> {
  const res = await api.get<{ contributions: SavingsContribution[] }>(
    `/api/finances/savings-goals/${goalId}/contributions`
  )
  return res.data.contributions
}

async function createSavingsGoalContribution(
  goalId: number,
  input: SavingsContributionInput
): Promise<SavingsContribution> {
  const res = await api.post<SavingsContribution>(
    `/api/finances/savings-goals/${goalId}/contributions`,
    input
  )
  return res.data
}

// Add to return object:
listSavingsGoals,
createSavingsGoal,
updateSavingsGoal,
deleteSavingsGoal,
listSavingsGoalContributions,
createSavingsGoalContribution,
```

### 2.3 Components to create

#### `src/pages/Finances/MetasTab.tsx`

New file. Contains three exported/gated components:

**`GoalForm`** — dialog for create/edit:
- Fields: name (`Input`), target amount (`Input type=number step=0.01`, converts PEN → cents), deadline (`Input type=date`, optional), icon selector (static list of lucide keys), color swatches (use `CARD_COLORS` from `TarjetasTab.tsx`).
- On submit: `createSavingsGoal` or `updateSavingsGoal`; calls `onSuccess` with the returned `SavingsGoal`.
- Reuses the `Dialog` + `DialogContent` + `DialogHeader` + `DialogFooter` shadcn pattern from `CardForm`.

**`ContributionForm`** — dialog to add a contribution:
- Fields: amount (`Input type=number step=0.01`), date (`Input type=date`), note (`Input`).
- On submit: calls `createSavingsGoalContribution`; calls `onSuccess` with the returned `SavingsContribution`.

**`MetasTab`** — main grid component:
- Receives `goals: SavingsGoal[]` and `onRefresh: () => void` as props (same pattern as `TarjetasTabProps`).
- Summary header: total `currentCents` across all goals vs. combined `targetCents`, displayed in a `CozyCard`-style header using PEN formatting.
- Grid of goal cards (`grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3`):
  - Top border via `style={{ borderTop: \`4px solid ${goal.color}\` }}` (same as `TarjetasTab`).
  - Icon rendered via lucide (key = `goal.icon`).
  - Linear progress bar: `<div class="w-full bg-muted rounded-full h-2"><div class="h-2 rounded-full" style={{ width: \`${Math.min(100, Math.round((goal.currentCents / goal.targetCents) * 100))}%\`, backgroundColor: goal.color }} /></div>`
  - Percentage label.
  - Formatted PEN amounts (`new Intl.NumberFormat('es-PE', { style: 'currency', currency: 'PEN' }).format(...)`).
  - Deadline badge: days remaining or formatted date (Spanish).
  - "Añadir" `Button` (size=sm, variant=ghost) → opens `ContributionForm`.
  - Edit / Delete icon buttons (`Pencil`, `Trash2`) → same pattern as `TarjetasTab`.
- Empty state: `UICard` with `PiggyBank` icon and "Sin metas de ahorro" text.
- FloatingActionButton: same position as other tabs.

#### `src/pages/Finances/MetasTabWrapper.tsx` (or inline in `FinancesPage.tsx`)

Same pattern as `TarjetasTabWrapper`:

```tsx
function MetasTabWrapper() {
  const { listSavingsGoals } = useFinanceApi()
  const [goals, setGoals] = useState<SavingsGoal[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [hasError, setHasError] = useState(false)
  const [formOpen, setFormOpen] = useState(false)
  const [editGoal, setEditGoal] = useState<SavingsGoal | undefined>()

  useEffect(() => {
    let ignore = false
    listSavingsGoals()
      .then(data => { if (!ignore) { setGoals(data); setIsLoading(false) } })
      .catch(() => { if (!ignore) { setHasError(true); setIsLoading(false) } })
    return () => { ignore = true }
  }, [listSavingsGoals])

  const handleRefresh = useCallback(() => {
    setIsLoading(true)
    setHasError(false)
    listSavingsGoals()
      .then(data => { setGoals(data); setIsLoading(false) })
      .catch(() => { setHasError(true); setIsLoading(false) })
  }, [listSavingsGoals])

  // Loading / error / empty states mirror TarjetasTabWrapper exactly
  // ...
}
```

### 2.4 Component to modify

**`src/pages/Finances/FinancesPage.tsx`**:
1. Import `MetasTabWrapper` and `Target` icon.
2. Add to `TABS` array:
   ```ts
   { value: 'metas', label: 'Metas', icon: Target }
   ```
3. Add `<TabsContent value="metas">` after the tarjetas content:
   ```tsx
   <TabsContent value="metas" className="mt-4">
     <MetasTabWrapper />
   </TabsContent>
   ```

### 2.5 Color palette and icon options

- Reuse `CARD_COLORS` array from `TarjetasTab.tsx` as the goal color palette.
- Default icon: `piggy-bank`. Available icons: static curated list of lucide keys (e.g. `piggy-bank`, `plane`, `home`, `car`, `graduation-cap`, `heart`, `gift`, `briefcase`).

---

## 3. Contribution Atomic Transaction Pattern

The `CreateContribution` handler implements a two-step atomic transaction that mirrors `CreateReload` in `cards.go`.

### Why a transaction is required

`current_cents` on `savings_goals` is **denormalized**: it is not computed on read as `SUM(amount_cents)` from `savings_contributions`. Instead it is maintained as a pre-computed column, updated on every contribution. Without a transaction:

- If step 1 (insert row) succeeds but step 2 (`UPDATE current_cents`) fails → the contribution row exists but the goal shows the wrong balance.
- If step 1 fails → no inconsistency, but we lose the contribution.
- If step 2 succeeds but the `INSERT` then fails → `current_cents` is inflated with no corresponding row.

Using a transaction guarantees both steps succeed together or both roll back, keeping the denormalized counter consistent with the source-of-truth table.

### Transaction flow (pseudo-code)

```
BEGIN
  INSERT INTO savings_contributions (goal_id, amount_cents, occurred_on, note, created_by)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id, goal_id, amount_cents, occurred_on, note, created_at
  // → c

  UPDATE savings_goals
    SET current_cents = current_cents + $2   -- increment by amount_cents
    WHERE id = $1

  -- check rowCount = 1 or tx.Commit fails on PostgreSQL error
COMMIT
```

### Error handling

- `INSERT` failure → `tx.Rollback()`, return 500.
- `UPDATE` failure → `tx.Rollback()`, return 500.
- `tx.Commit()` failure → return 500.
- `SELECT ... FOR UPDATE` (row lock) is **not** required because the `WHERE id = $1` in the UPDATE will lock the goal row for the duration of the transaction in PostgreSQL's READ COMMITTED mode, which is sufficient for this increment-only operation.

---

## 4. Ownership and Security

All goal and contribution endpoints use the same `scopeToOwner` deny-by-default pattern as cards and subscriptions:

- `ListGoals` → `scopeToOwner` on the SELECT, so guests only see their own goals.
- `CreateGoal` → stamps `created_by = userID` from JWT context.
- `UpdateGoal` / `DeleteGoal` → `scopeToOwner` on the WHERE clause; returns 404 (not 403) to avoid revealing existence.
- `ListContributions` → first checks `goalOwned` (which calls `scopeToOwner` on the goal); returns 404 if the goal isn't owned.
- `CreateContribution` → first checks `goalOwned`, same 404-if-not-owned pattern.
- Admins bypass `scopeToOwner` and see all goals/contributions.
