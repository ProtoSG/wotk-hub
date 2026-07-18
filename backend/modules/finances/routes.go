package finances

import (
	"database/sql"
	"net/http"
	"workhub/middleware"

	chi "github.com/go-chi/chi/v5"
)

type handler struct {
	db *sql.DB
}

// Routes mounts the finances endpoints. GET /categories is public (used to
// populate category pickers before login); everything else requires a valid
// access_token cookie, so — like auth.Routes — the public/protected split
// happens inside this router instead of by the caller.
func Routes(db *sql.DB, jwtSecret string) http.Handler {
	h := &handler{db: db}
	r := chi.NewRouter()

	r.Get("/categories", h.ListCategories)

	r.Group(func(pr chi.Router) {
		pr.Use(middleware.RequireAuth(db, jwtSecret))

		pr.Get("/transactions", h.ListTransactions)
		pr.Post("/transactions", h.CreateTransaction)
		pr.Put("/transactions/{id}", h.UpdateTransaction)
		pr.Delete("/transactions/{id}", h.DeleteTransaction)
		pr.Post("/transactions/{id}/refund", h.RefundTransaction)

		pr.Get("/subscriptions", h.ListSubscriptions)
		pr.Post("/subscriptions", h.CreateSubscription)
		pr.Put("/subscriptions/{id}", h.UpdateSubscription)
		pr.Delete("/subscriptions/{id}", h.DeleteSubscription)

		pr.Get("/cards", h.ListCards)
		pr.Post("/cards", h.CreateCard)
		pr.Put("/cards/{id}", h.UpdateCard)
		pr.Delete("/cards/{id}", h.DeleteCard)
		pr.Post("/cards/transfers", h.CreateCardTransfer)

		pr.Get("/savings-goals", h.ListGoals)
		pr.Post("/savings-goals", h.CreateGoal)
		pr.Put("/savings-goals/{id}", h.UpdateGoal)
		pr.Delete("/savings-goals/{id}", h.DeleteGoal)
		pr.Get("/savings-goals/{id}/contributions", h.ListContributions)
		pr.Post("/savings-goals/{id}/contributions", h.CreateContribution)

		pr.Get("/budgets", h.ListBudgets)
		pr.Put("/budgets/{category}", h.UpsertBudget)
		pr.Delete("/budgets/{category}", h.DeleteBudget)

		pr.Post("/categories", h.CreateCategory)
		pr.Put("/categories/{id}", h.UpdateCategory)
		pr.Delete("/categories/{id}", h.DeleteCategory)

		pr.Get("/summary", h.Summary)
	})

	return r
}
