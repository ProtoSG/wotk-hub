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
	r.Post("/transactions/{id}/refund", h.RefundTransaction)

	r.Get("/subscriptions", h.ListSubscriptions)
	r.Post("/subscriptions", h.CreateSubscription)
	r.Put("/subscriptions/{id}", h.UpdateSubscription)
	r.Delete("/subscriptions/{id}", h.DeleteSubscription)

	r.Get("/cards", h.ListCards)
	r.Post("/cards", h.CreateCard)
	r.Put("/cards/{id}", h.UpdateCard)
	r.Delete("/cards/{id}", h.DeleteCard)
	r.Post("/cards/transfers", h.CreateCardTransfer)

	r.Get("/savings-goals", h.ListGoals)
	r.Post("/savings-goals", h.CreateGoal)
	r.Put("/savings-goals/{id}", h.UpdateGoal)
	r.Delete("/savings-goals/{id}", h.DeleteGoal)
	r.Get("/savings-goals/{id}/contributions", h.ListContributions)
	r.Post("/savings-goals/{id}/contributions", h.CreateContribution)

	r.Get("/budgets", h.ListBudgets)
	r.Put("/budgets/{category}", h.UpsertBudget)
	r.Delete("/budgets/{category}", h.DeleteBudget)

	r.Get("/categories", h.ListCategories)
	r.Post("/categories", h.CreateCategory)
	r.Put("/categories/{id}", h.UpdateCategory)
	r.Delete("/categories/{id}", h.DeleteCategory)

	r.Get("/summary", h.Summary)

	return r
}
