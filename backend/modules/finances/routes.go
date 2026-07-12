package finances

import (
	"database/sql"
	"net/http"

	chi "github.com/go-chi/chi/v5"
)

type handler struct {
	db *sql.DB
}

func Routes(db *sql.DB) http.Handler {
	h := &handler{db: db}
	r := chi.NewRouter()

	r.Get("/transactions", h.ListTransactions)
	r.Post("/transactions", h.CreateTransaction)
	r.Put("/transactions/{id}", h.UpdateTransaction)
	r.Delete("/transactions/{id}", h.DeleteTransaction)

	r.Get("/subscriptions", h.ListSubscriptions)
	r.Post("/subscriptions", h.CreateSubscription)
	r.Put("/subscriptions/{id}", h.UpdateSubscription)
	r.Delete("/subscriptions/{id}", h.DeleteSubscription)

	r.Get("/budgets", h.ListBudgets)
	r.Put("/budgets/{category}", h.UpsertBudget)
	r.Delete("/budgets/{category}", h.DeleteBudget)

	r.Get("/summary", h.Summary)

	return r
}
