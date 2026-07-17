package finances

import (
	"errors"
	"testing"
)

// transactionRequest.validate must reject income/expense without a cardId
// with "cardId requerido" — the core of the mandatory-card model. A zero
// or negative cardId means the client omitted or sent garbage.
func TestTransactionRequestValidate_RequiresCardID(t *testing.T) {
	tests := []struct {
		name    string
		req     transactionRequest
		wantErr string
		wantOK  bool
	}{
		{
			name:    "income missing cardId (zero value) rejected",
			req:     transactionRequest{Type: "income", AmountCents: 100, Category: "sueldo", Date: "2026-07-17"},
			wantErr: "cardId requerido",
		},
		{
			name:    "expense negative cardId rejected",
			req:     transactionRequest{Type: "expense", AmountCents: 100, Category: "comida", Date: "2026-07-17", CardID: -5},
			wantErr: "cardId requerido",
		},
		{
			name:    "expense missing cardId rejected",
			req:     transactionRequest{Type: "expense", AmountCents: 100, Category: "comida", Date: "2026-07-17"},
			wantErr: "cardId requerido",
		},
		{
			name:   "income with valid cardId accepted",
			req:    transactionRequest{Type: "income", AmountCents: 100, Category: "sueldo", Date: "2026-07-17", CardID: 7},
			wantOK: true,
		},
		{
			name:   "expense with valid cardId accepted",
			req:    transactionRequest{Type: "expense", AmountCents: 100, Category: "comida", Date: "2026-07-17", CardID: 9},
			wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.req.validate()
			if tt.wantOK {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

// rejectCreditCardForInflow is the shared inflow guard: credito cards may
// never be the target of an income (or, in slice 1b, a card-to-card
// transfer). Anything else returns nil. Kept as a pure function so every
// caller (CreateTransaction in this slice; CreateCardTransfer in 1b)
// applies the identical rule without divergence.
func TestRejectCreditCardForInflow(t *testing.T) {
	tests := []struct {
		name    string
		cardTyp string
		wantErr error
	}{
		{name: "credito rejected", cardTyp: cardTypeCredit, wantErr: errCreditInflow},
		{name: "debito allowed", cardTyp: "debito", wantErr: nil},
		{name: "prepago allowed", cardTyp: "prepago", wantErr: nil},
		{name: "empty allowed (defensive)", cardTyp: "", wantErr: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rejectCreditCardForInflow(tt.cardTyp)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected errCreditInflow, got %v", err)
			}
		})
	}
}
