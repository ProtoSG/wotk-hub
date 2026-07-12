package finances

import (
	"fmt"
	"slices"
	"time"
)

var expenseCategories = []string{
	"comida", "transporte", "vivienda", "servicios", "salud",
	"educacion", "entretenimiento", "ropa", "suscripciones", "otros",
}

var incomeCategories = []string{
	"sueldo", "freelance", "inversiones", "regalo", "otros",
}

const dateLayout = "2006-01-02"

type Transaction struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	AmountCents int64  `json:"amountCents"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Date        string `json:"date"`
	CreatedAt   string `json:"createdAt"`
}

type transactionRequest struct {
	Type        string `json:"type"`
	AmountCents int64  `json:"amountCents"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Date        string `json:"date"`
}

func (r transactionRequest) validate() (time.Time, error) {
	if r.Type != "income" && r.Type != "expense" {
		return time.Time{}, fmt.Errorf("invalid type: %s", r.Type)
	}
	if r.AmountCents <= 0 {
		return time.Time{}, fmt.Errorf("amountCents must be positive")
	}
	cats := expenseCategories
	if r.Type == "income" {
		cats = incomeCategories
	}
	if !slices.Contains(cats, r.Category) {
		return time.Time{}, fmt.Errorf("invalid category: %s", r.Category)
	}
	d, err := time.Parse(dateLayout, r.Date)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date: %s", r.Date)
	}
	return d, nil
}

type Subscription struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	AmountCents   int64  `json:"amountCents"`
	Frequency     string `json:"frequency"`
	Category      string `json:"category"`
	NextBillingOn string `json:"nextBillingOn"`
	Active        bool   `json:"active"`
	CreatedAt     string `json:"createdAt"`
}

type subscriptionRequest struct {
	Name          string `json:"name"`
	AmountCents   int64  `json:"amountCents"`
	Frequency     string `json:"frequency"`
	Category      string `json:"category"`
	NextBillingOn string `json:"nextBillingOn"`
	Active        *bool  `json:"active"`
}

func (r subscriptionRequest) validate() (time.Time, error) {
	if r.Name == "" {
		return time.Time{}, fmt.Errorf("name is required")
	}
	if r.AmountCents <= 0 {
		return time.Time{}, fmt.Errorf("amountCents must be positive")
	}
	if r.Frequency != "weekly" && r.Frequency != "monthly" && r.Frequency != "yearly" {
		return time.Time{}, fmt.Errorf("invalid frequency: %s", r.Frequency)
	}
	if !slices.Contains(expenseCategories, r.Category) {
		return time.Time{}, fmt.Errorf("invalid category: %s", r.Category)
	}
	d, err := time.Parse(dateLayout, r.NextBillingOn)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid nextBillingOn: %s", r.NextBillingOn)
	}
	return d, nil
}

func (r subscriptionRequest) isActive() bool {
	if r.Active == nil {
		return true
	}
	return *r.Active
}

type Budget struct {
	ID                int64  `json:"id"`
	Category          string `json:"category"`
	MonthlyLimitCents int64  `json:"monthlyLimitCents"`
	SpentCents        int64  `json:"spentCents"`
}

type budgetRequest struct {
	MonthlyLimitCents int64 `json:"monthlyLimitCents"`
}

func (r budgetRequest) validate(category string) error {
	if !slices.Contains(expenseCategories, category) {
		return fmt.Errorf("invalid category: %s", category)
	}
	if r.MonthlyLimitCents <= 0 {
		return fmt.Errorf("monthlyLimitCents must be positive")
	}
	return nil
}

type TrendPoint struct {
	Month        string `json:"month"`
	IncomeCents  int64  `json:"incomeCents"`
	ExpenseCents int64  `json:"expenseCents"`
}

type CategoryAmount struct {
	Category    string `json:"category"`
	AmountCents int64  `json:"amountCents"`
}

type Summary struct {
	BalanceCents      int64            `json:"balanceCents"`
	MonthIncomeCents  int64            `json:"monthIncomeCents"`
	MonthExpenseCents int64            `json:"monthExpenseCents"`
	MonthlyTrend      []TrendPoint     `json:"monthlyTrend"`
	CategoryBreakdown []CategoryAmount `json:"categoryBreakdown"`
}
