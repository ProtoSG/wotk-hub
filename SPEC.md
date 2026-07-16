# SPEC — Tarjetas (Cards) in Finances

## Decision log
- Cards are **per-user**: scoped by `created_by = current user ID` from the JWT
  cookie, same ownership pattern as `transactions` (see `scopeToOwner` in
  `backend/modules/finances/helpers.go`). Not shared household state.

## Data model

### `cards`
| column       | type          | notes                                          |
|--------------|---------------|-------------------------------------------------|
| id           | BIGSERIAL PK  |                                                   |
| name         | TEXT NOT NULL | e.g. "BCP Sueldo"                                |
| type         | TEXT NOT NULL | CHECK IN ('debito','credito','prepago')          |
| bank         | TEXT NOT NULL DEFAULT ''  | issuer name                         |
| last4        | TEXT NOT NULL DEFAULT ''  | last 4 digits, display only         |
| color        | TEXT NOT NULL DEFAULT '#3B82F6' | hex color for UI card face    |
| icon         | TEXT NOT NULL DEFAULT 'credit-card' | lucide icon key            |
| balance_cents| BIGINT NOT NULL DEFAULT 0 |                                       |
| created_by   | BIGINT REFERENCES users(id) |                                 |
| created_at   | TIMESTAMPTZ NOT NULL DEFAULT now() |                          |

### `card_reloads`
| column       | type          | notes                                          |
|--------------|---------------|-------------------------------------------------|
| id           | BIGSERIAL PK  |                                                   |
| card_id      | BIGINT NOT NULL REFERENCES cards(id) |                          |
| amount_cents | BIGINT NOT NULL CHECK (amount_cents > 0) |                      |
| occurred_on  | DATE NOT NULL |                                                   |
| note         | TEXT NOT NULL DEFAULT '' |                                       |
| created_by   | BIGINT REFERENCES users(id) |                                 |
| created_at   | TIMESTAMPTZ NOT NULL DEFAULT now() |                          |

A `CreateReload` call inserts the reload row and atomically increments
`cards.balance_cents` by `amount_cents` (single transaction).

### `transactions.card_id`
`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS card_id BIGINT REFERENCES cards(id)`
Nullable link only — no automatic balance deduction logic (out of scope for
this change; can be added later if requested).

## API

Ownership: same `scopeToOwner` deny-by-default pattern as transactions —
non-admin roles only see/mutate their own cards; admin sees all.

- `GET /api/finances/cards` → `{ cards: Card[] }`
- `POST /api/finances/cards` → `Card` (201)
- `PUT /api/finances/cards/{id}` → `Card` (404 if not owned)
- `DELETE /api/finances/cards/{id}` → `{ success: true }` (404 if not owned)
- `GET /api/finances/cards/{id}/reloads` → `{ reloads: CardReload[] }` (404 if card not owned)
- `POST /api/finances/cards/{id}/reloads` → `CardReload` (201, 404 if card not owned)

Validation (`cardRequest.validate()`):
- `name` required
- `type` in `debito|credito|prepago`
- `balanceCents` >= 0 (create only; not editable directly via PUT — PUT
  updates name/type/bank/last4/color/icon only, balance changes only via reload)

Validation (`cardReloadRequest.validate()`):
- `amountCents` > 0
- `date` valid `YYYY-MM-DD`

## Frontend

- `useCards` hook (added to `useFinanceApi.ts` alongside existing finance
  calls, same axios/api pattern) — `listCards`, `createCard`, `updateCard`,
  `deleteCard`, `listReloads`, `createReload`.
- `types/finance.types.ts` gains `Card`, `CardInput`, `CardType`,
  `CardReload`, `CardReloadInput`.
- `TarjetasTab.tsx` — grid of card faces (colored, icon, name, bank, last4,
  balance via `formatPEN`), edit/delete menu per card (same
  `DropdownMenu` pattern as `MovimientosTab`), "Recargar" button opens
  `ReloadForm`, "Nueva tarjeta" opens `CardForm`.
- `CardForm.tsx` — create/edit dialog, same `react-hook-form` + `zod` +
  `Dialog` pattern as `TransactionForm.tsx`.
- `ReloadForm.tsx` — amount + date + note dialog, same pattern, calls
  `createReload` and refreshes the card list (updated balance).
- `FinancesPage.tsx` — add `Tarjetas` tab (icon: `CreditCard` from
  lucide-react) between Movimientos and Suscripciones.

All labels/placeholders in Spanish, matching existing tabs.
