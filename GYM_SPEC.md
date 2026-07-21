# SPEC — Gym: Exercise Catalog, Routines, Workout Log & Progress

Status: draft, pre-implementation.
Scope: a new `gym` module (backend + frontend) to log strength training — sets, reps and weight per exercise per session — on top of a seeded exercise catalog, with reusable routine templates and per-exercise progress charts.

---

## 1. Goals

1. Keep a **catalog of exercises** (413 rows, seeded from `exercises_data.csv`) served by the API, filterable by muscle and equipment.
2. Build **reusable routine templates** ("Día de Pecho") that pre-load exercises into a session.
3. Log a **workout session**: per exercise, an ordered list of sets with reps + weight.
4. Show **progress charts per exercise over time** (max weight, estimated 1RM, volume).

### Non-goals (this iteration)

- Body metrics (body weight, measurements, photos). Explicitly deferred to a later change.
- Cardio-specific fields (duration, distance, pace). Cardio exercises exist in the catalog but are logged with the same set/rep/weight shape for now.
- Rest timers, supersets, RPE, plate calculator, social/sharing features.
- Multi-user sharing of routines. Routines are per-user, same as the rest of the app.

---

## 2. Decision log

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | Catalog lives in the **backend** as a real table, not a static frontend asset | Centralized, filterable and paginable by API; lets user-created custom exercises coexist with seeded ones; avoids shipping a 413-row CSV in the JS bundle. |
| D2 | Sessions are logged as **set-level rows**, not a JSON blob | Progress queries (max weight per exercise over time, volume) are aggregations. SQL does this; a JSON blob would force in-app scanning. |
| D3 | Routines are **templates**, decoupled from sessions | A session snapshots its exercises at start time. Editing a routine later must NOT rewrite past sessions. |
| D4 | Charts reuse **`recharts` ^3.8.1** (already a dependency) | No new dependency. Same visual language as `TrendChart.tsx` / `CategoryChart.tsx` in Finances. |
| D5 | Weight stored as **`weight_grams BIGINT`** | Same integer-money reasoning as `amount_cents` in finances: no float drift. 2.5 kg increments and lb conversions stay exact. |
| D6 | Seed is **idempotent on exercise `name`** | `store.Migrate` has no migration tool — schema and seed are re-run every boot. Seeding must be `ON CONFLICT DO NOTHING`. |
| D7 | Module mounts at `/api/gym`, auth-protected like `couple` | No public endpoints; the catalog is not needed pre-login (unlike finance categories). |

---

## 3. Source data — `exercises_data.csv`

Current location: `frontend/src/assets/exercises/exercises_data.csv` (untracked).
Target location: `backend/modules/gym/data/exercises.csv`, embedded with `go:embed`.

**Shape** — 413 data rows + header:

```csv
name,equipment,primary_muscle,secondary_muscle,source,sourceType
21s Bicep Curl,Barbell,Biceps,None,None,None
Bench Press (Barbell),Barbell,Chest,"Triceps, Shoulders",https://…/00251201-Barbell-Bench-Press_Chest.mp4,video
```

Facts verified against the file:

- **Properly quoted CSV.** `secondary_muscle` is a quoted comma-list (`"Quadriceps, Lower Back, Glutes"`). Parse with `encoding/csv`, never `strings.Split` on the raw line.
- **`name` is unique** across all 413 rows — safe as the natural conflict key for seeding.
- **The literal string `None`** is used as the null sentinel in `equipment`, `secondary_muscle`, `source` and `sourceType`. It must be normalized to empty/`NULL` at import — except in `primary_muscle`, where `None` never appears but `Other` does.
- `equipment` domain: `Barbell, Dumbbell, Kettlebell, Machine, Plate, Resistance Band, Suspension, Other, None`.
- `primary_muscle` domain: `Abdominals, Abductors, Adductors, Biceps, Calves, Cardio, Chest, Forearms, Full Body, Glutes, Hamstrings, Lats, Lower Back, Neck, Other, Quadriceps, Shoulders, Traps, Triceps, Upper Back`.
- `sourceType`: 222 `video`, 139 `image`, 52 `None`.
- `source` media is hosted on a **third-party S3 bucket** (`pump-app.s3.eu-west-2.amazonaws.com`). Treat as a best-effort thumbnail: the UI must render fine when the URL 404s or is empty. Do not proxy or hotlink it into a critical path.

**Domains are stored as free `TEXT`, not `CHECK` constraints.** Custom user exercises may introduce new equipment values, and a `CHECK` would need an `ALTER` on every addition — bad fit for the append-only migration style.

---

## 4. Data model

Postgres, appended to `backend/store/migrate.go` as new idempotent statements, following the existing file's conventions (`BIGSERIAL`, `TIMESTAMPTZ NOT NULL DEFAULT now()`, `created_by BIGINT REFERENCES users(id)`).

### `exercises` — catalog (seeded + user-created)

```sql
CREATE TABLE IF NOT EXISTS exercises (
  id               BIGSERIAL PRIMARY KEY,
  name             TEXT NOT NULL UNIQUE,
  equipment        TEXT NOT NULL DEFAULT '',
  primary_muscle   TEXT NOT NULL DEFAULT 'Other',
  secondary_muscle TEXT NOT NULL DEFAULT '',   -- comma-separated, as in the CSV
  media_url        TEXT NOT NULL DEFAULT '',
  media_type       TEXT NOT NULL DEFAULT '',   -- 'image' | 'video' | ''
  is_custom        BOOLEAN NOT NULL DEFAULT false,
  created_by       BIGINT REFERENCES users(id),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_exercises_primary_muscle ON exercises (primary_muscle);
CREATE INDEX IF NOT EXISTS idx_exercises_equipment ON exercises (equipment);
```

`is_custom = false` marks seeded rows: the seeder only touches those, and deletion is blocked on them.

### `routines` — templates

```sql
CREATE TABLE IF NOT EXISTS routines (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL,
  notes      TEXT NOT NULL DEFAULT '',
  color      TEXT NOT NULL DEFAULT '#3B82F6',
  icon       TEXT NOT NULL DEFAULT 'dumbbell',
  archived   BOOLEAN NOT NULL DEFAULT false,
  created_by BIGINT REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### `routine_exercises` — template contents

```sql
CREATE TABLE IF NOT EXISTS routine_exercises (
  id           BIGSERIAL PRIMARY KEY,
  routine_id   BIGINT NOT NULL REFERENCES routines(id) ON DELETE CASCADE,
  exercise_id  BIGINT NOT NULL REFERENCES exercises(id),
  position     INT NOT NULL,
  target_sets  INT NOT NULL DEFAULT 3 CHECK (target_sets > 0),
  target_reps  INT NOT NULL DEFAULT 10 CHECK (target_reps > 0),
  notes        TEXT NOT NULL DEFAULT '',
  UNIQUE (routine_id, position)
);
CREATE INDEX IF NOT EXISTS idx_routine_exercises_routine_id ON routine_exercises (routine_id);
```

### `workout_sessions` — a training day

```sql
CREATE TABLE IF NOT EXISTS workout_sessions (
  id            BIGSERIAL PRIMARY KEY,
  routine_id    BIGINT REFERENCES routines(id) ON DELETE SET NULL,  -- NULL = freestyle
  name          TEXT NOT NULL DEFAULT '',        -- snapshot of routine name at start
  occurred_on   DATE NOT NULL,
  started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at   TIMESTAMPTZ,                     -- NULL = in progress
  notes         TEXT NOT NULL DEFAULT '',
  created_by    BIGINT REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_workout_sessions_occurred_on ON workout_sessions (occurred_on);
```

`ON DELETE SET NULL` + the `name` snapshot are what keep D3 true: deleting a routine never destroys history.

### `session_exercises` — exercises performed in a session

```sql
CREATE TABLE IF NOT EXISTS session_exercises (
  id          BIGSERIAL PRIMARY KEY,
  session_id  BIGINT NOT NULL REFERENCES workout_sessions(id) ON DELETE CASCADE,
  exercise_id BIGINT NOT NULL REFERENCES exercises(id),
  position    INT NOT NULL,
  notes       TEXT NOT NULL DEFAULT '',
  UNIQUE (session_id, position)
);
CREATE INDEX IF NOT EXISTS idx_session_exercises_session_id ON session_exercises (session_id);
CREATE INDEX IF NOT EXISTS idx_session_exercises_exercise_id ON session_exercises (exercise_id);
```

`idx_session_exercises_exercise_id` is what makes the progress chart query cheap.

### `exercise_sets` — the actual log

```sql
CREATE TABLE IF NOT EXISTS exercise_sets (
  id                  BIGSERIAL PRIMARY KEY,
  session_exercise_id BIGINT NOT NULL REFERENCES session_exercises(id) ON DELETE CASCADE,
  set_number          INT    NOT NULL CHECK (set_number > 0),
  reps                INT    NOT NULL CHECK (reps >= 0),
  weight_grams        BIGINT NOT NULL DEFAULT 0 CHECK (weight_grams >= 0),
  is_warmup           BOOLEAN NOT NULL DEFAULT false,
  completed           BOOLEAN NOT NULL DEFAULT true,
  UNIQUE (session_exercise_id, set_number)
);
CREATE INDEX IF NOT EXISTS idx_exercise_sets_session_exercise_id ON exercise_sets (session_exercise_id);
```

`weight_grams = 0` is legal and meaningful: bodyweight exercises. `reps >= 0` allows a logged-but-failed set.

---

## 5. Seeding the catalog

Package `backend/modules/gym`, called once from startup right after `store.Migrate`.

```go
//go:embed data/exercises.csv
var exercisesCSV []byte

// SeedExercises imports the bundled catalog. Idempotent: rows are matched by
// name, so re-running on every boot is a no-op after the first import, and
// user-created exercises (is_custom = true) are never touched.
func SeedExercises(db *sql.DB) error
```

Rules:

1. Parse with `encoding/csv` (`FieldsPerRecord = 6`), skip the header.
2. Normalize the literal `"None"` to `""` for `equipment`, `secondary_muscle`, `media_url`, `media_type`.
3. Insert with `ON CONFLICT (name) DO NOTHING` — never `DO UPDATE`. A user renaming or re-pointing a row must not be clobbered on the next deploy.
4. Wrap in a single transaction. A partial catalog is worse than none.
5. Log the inserted count at info level; a failed seed is fatal at startup (an empty catalog makes the module unusable).

Also expose it as `workhubctl gym seed` for manual re-runs, matching `cmd/workhubctl/finances.go`.

---

## 6. API — `/api/gym`

Mounted in `backend/main.go` next to the others, inside the authenticated group:

```go
pr.With(middleware.RequireRole("admin", "guest")).Mount("/api/gym", gym.Routes(appDB))
```

`backend/modules/gym/routes.go` mirrors `finances.Routes` (a `handler{db}` struct, chi router, no public group).

### Catalog

| Method | Path | Notes |
|--------|------|-------|
| `GET` | `/exercises` | Filters: `?q=` (name ILIKE), `?muscle=`, `?equipment=`, `?limit=`&`?offset=`. Returns `{ exercises, total }`. |
| `GET` | `/exercises/filters` | Distinct `muscles` and `equipment` values, for picker dropdowns. |
| `POST` | `/exercises` | Custom exercise. Forces `is_custom = true`. |
| `PUT` | `/exercises/{id}` | Custom only. |
| `DELETE` | `/exercises/{id}` | Custom only, and 409 if referenced by any `session_exercises` row. |

### Routines

| Method | Path | Notes |
|--------|------|-------|
| `GET` | `/routines` | Includes exercise count per routine. |
| `GET` | `/routines/{id}` | Full template with ordered exercises + joined exercise metadata. |
| `POST` | `/routines` | Body carries the ordered `exercises[]`; written in one transaction. |
| `PUT` | `/routines/{id}` | Full replace of the exercise list (delete + reinsert inside a transaction). Simpler than diffing, and `position` stays consistent. |
| `DELETE` | `/routines/{id}` | Cascades the template; past sessions survive (`ON DELETE SET NULL`). |

### Sessions

| Method | Path | Notes |
|--------|------|-------|
| `GET` | `/sessions` | Filters `?from=`&`?to=`&`?routine_id=`. Summary rows: date, name, exercise count, total volume, duration. |
| `GET` | `/sessions/{id}` | Full nested tree: session → exercises → sets. |
| `POST` | `/sessions` | `{ routine_id?, occurred_on, name? }`. If `routine_id` is set, **materializes** `session_exercises` from the template and pre-creates `target_sets` empty sets. |
| `PUT` | `/sessions/{id}` | Edit date, name, notes. |
| `POST` | `/sessions/{id}/finish` | Stamps `finished_at`. Idempotent. |
| `DELETE` | `/sessions/{id}` | Cascades exercises and sets. |
| `POST` | `/sessions/{id}/exercises` | Add an exercise mid-session (appends at `MAX(position)+1`). |
| `DELETE` | `/sessions/{id}/exercises/{seId}` | Removes it and its sets; remaining positions are re-packed. |
| `PUT` | `/sessions/{id}/exercises/{seId}/sets` | **Bulk replace** the set list for that exercise. |
| `GET` | `/exercises/{id}/last-sets` | Sets from the most recent session that logged this exercise, plus its date. Backs the prefill described below; returns an empty list on a first-ever exercise. |

The bulk-replace on sets is deliberate: the mobile logging UI edits a small grid of rows locally and saves the whole block. Per-set `POST`/`PATCH` would multiply round-trips during a workout, exactly when the connection is worst.

### Progress

| Method | Path | Notes |
|--------|------|-------|
| `GET` | `/progress/exercises/{id}` | `?from=`&`?to=`. One point per session. |
| `GET` | `/progress/summary` | Sessions this month, total volume, current streak, most-trained muscle. |

Per-point payload:

```json
{
  "occurred_on": "2026-07-18",
  "session_id": 42,
  "max_weight_grams": 80000,
  "total_volume_grams": 1920000,
  "total_reps": 24,
  "top_set": { "reps": 8, "weight_grams": 80000 },
  "estimated_1rm_grams": 101333
}
```

- `total_volume_grams = Σ(reps × weight_grams)` over non-warmup, completed sets.
- `estimated_1rm_grams` uses **Epley**: `w × (1 + reps/30)`, computed on the top set. Warmup sets are excluded from every metric.
- Computed in SQL (one grouped query over `exercise_sets ⋈ session_exercises ⋈ workout_sessions`), not in Go loops.

---

## 7. Frontend

### Files

```
frontend/src/
  types/gym.types.ts
  hooks/useGymApi.ts                 # mirrors useFinanceApi: plain async fns over @/lib/axios
  pages/Gym/
    gymKeys.ts                       # query-key factory, mirrors financeKeys.ts
    GymPage.tsx                      # tab shell
    EntrenarTab.tsx                  # active session logger
    RutinasTab.tsx                   # routine list
    RoutineForm.tsx                  # template builder
    HistorialTab.tsx                 # session history
    ProgresoTab.tsx                  # per-exercise charts
    ExercisePicker.tsx               # searchable catalog dialog (shared by builder + session)
    SetGrid.tsx                      # the reps/weight input grid
    SessionCard.tsx
    ExerciseProgressChart.tsx        # recharts
    MobileTabNav.tsx                 # only if the Finances one can't be reused as-is
```

Wiring: `router/lazyPages.tsx` gets `GymPage`, `router/index.tsx` gets the `/gym` route, `layouts/Sidebar.tsx` gets `{ to: '/gym', label: 'Gimnasio', icon: Dumbbell }`.

UI copy stays **Spanish**, consistent with the rest of the app ("Finanzas", "Citas"). Code, identifiers and types stay English.

### Tabs

1. **Entrenar** — the core loop. If a session has `finished_at = NULL`, resume it; otherwise offer "Empezar rutina" (routine picker) or "Entrenamiento libre". Inside: one card per exercise, each with a `SetGrid` of `[set# | reps | kg | ✓]` rows, "+ Serie", and a **prefill from the last session for that exercise** so the common case is tapping ✓ four times.
2. **Rutinas** — list + `RoutineForm` builder (add from `ExercisePicker`, drag to reorder, set target sets/reps).
3. **Historial** — sessions grouped by month, expandable into the full set log. Mobile gets a card list, desktop a table — same split as `TransactionsTable` / `TransactionsMobileList`.
4. **Progreso** — pick an exercise, pick a range (1M / 3M / 6M / 1A), see a line chart. Metric toggle: peso máximo · 1RM estimado · volumen. Plus the `/progress/summary` tiles on top.

### Charts

`recharts` `LineChart`, one series at a time. Follow the existing `TrendChart.tsx` conventions for theming (CSS variables, no hardcoded palette), tooltips and empty states. Weights render in **kg with one decimal** (`weight_grams / 1000`); a `lib/weight.ts` helper mirrors `lib/currency.ts`.

---

## 8. Phasing

| Phase | Deliverable | Ships value on its own? |
|-------|-------------|-------------------------|
| P1 ✅ | Migration + CSV seeder + `GET /exercises` (+ filters) + `ExercisePicker` | Browsable catalog |
| P2 ✅ | Sessions + sets CRUD + `EntrenarTab` freestyle logging | **Yes — the log works** |
| P3 | Routines CRUD + builder + materialization on session start | Faster start |
| P4 | `/progress/*` + `ProgresoTab` charts | The payoff |
| P5 | `/progress/summary` tiles, history polish, streaks | Polish |

P2 is the first shippable slice. Do not let P3/P4 block it.

---

## 9. Acceptance criteria

1. Fresh boot on an empty DB creates all six tables and imports **413 exercises**; a second boot imports 0 and errors on nothing.
2. A user-renamed or user-created exercise survives a redeploy untouched.
3. A session started from a routine materializes that routine's exercises in order, with the configured `target_sets` empty set rows.
4. Editing or deleting a routine leaves every past session's exercises, sets and displayed name intact.
5. Set-level writes never partially apply: a failed bulk replace leaves the previous set list unchanged.
6. Warmup sets are excluded from max weight, volume and estimated 1RM.
7. The progress endpoint for an exercise with no logged sets returns an empty array, and the chart renders an empty state — not a crash and not a zero-line.
8. All weights round-trip exactly: `80000 g` in, `80.0 kg` displayed, `80000 g` back out.
9. The logging grid is usable one-handed on a phone: inputs are `inputMode="decimal"`, tap targets ≥ 44 px.
10. Deleting an exercise referenced by any session returns 409 with a clear message, and never orphans a set.

---

## 10. Open questions (not blocking P1–P2)

1. **Units** — kg is assumed throughout. If lb display is ever wanted, it's a frontend-only preference on top of `weight_grams`; no schema change.
2. **Cardio** — the catalog has a `Cardio` muscle group with ~duration-shaped exercises. For now they log as sets/reps/weight. If that grates in practice, add nullable `duration_seconds` / `distance_meters` to `exercise_sets` later.
3. **Ownership** — the app is single-household. Routines and sessions carry `created_by` but are not filtered by it, matching how `couple` and `finances` already behave. Revisit only if per-user isolation is ever wanted.
