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
	CardID      *int64 `json:"cardId,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

type transactionRequest struct {
	Type        string `json:"type"`
	AmountCents int64  `json:"amountCents"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Date        string `json:"date"`
	CardID      *int64 `json:"cardId"`
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
	if r.CardID != nil && *r.CardID <= 0 {
		return time.Time{}, fmt.Errorf("invalid cardId")
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

type Card struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	Type                string `json:"type"`
	Bank                string `json:"bank"`
	Last4               string `json:"last4"`
	Color               string `json:"color"`
	Icon                string `json:"icon"`
	BalanceCents        int64  `json:"balanceCents"`
	InitialBalanceCents int64  `json:"initialBalanceCents"`
	CreditLimitCents    int64  `json:"creditLimitCents"`
	UsedCreditCents     int64  `json:"usedCreditCents"`
	CreatedAt           string `json:"createdAt"`
}

const cardTypeCredit = "credito"

var cardTypes = []string{"debito", cardTypeCredit, "prepago"}

// cardRequest leaves the balance fields as pointers so an omitted field means
// "keep what's stored" rather than "set to zero" — the pre-existing card form
// doesn't send them at all.
type cardRequest struct {
	Name                string `json:"name"`
	Type                string `json:"type"`
	Bank                string `json:"bank"`
	Last4               string `json:"last4"`
	Color               string `json:"color"`
	Icon                string `json:"icon"`
	InitialBalanceCents *int64 `json:"initialBalanceCents"`
	CreditLimitCents    *int64 `json:"creditLimitCents"`
}

func (r cardRequest) validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !slices.Contains(cardTypes, r.Type) {
		return fmt.Errorf("invalid type: %s", r.Type)
	}
	if r.InitialBalanceCents != nil && *r.InitialBalanceCents < 0 {
		return fmt.Errorf("initialBalanceCents must not be negative")
	}
	if r.CreditLimitCents != nil && *r.CreditLimitCents < 0 {
		return fmt.Errorf("creditLimitCents must not be negative")
	}
	return nil
}

// cardDelta is a pending adjustment to one card's counters, accumulated before
// being written so that reversing and re-applying the same card collapses into
// a single UPDATE.
type cardDelta struct {
	balanceCents    int64
	usedCreditCents int64
}

// cardAdjustment reports how a transaction moves the counters of the card it
// is tagged to. Only expenses move a card: an income is not a reload, so
// tagging one to a card records where the money landed without topping it up.
// A credit card's expense draws down its limit instead of its balance.
func cardAdjustment(cardType, txType string, amountCents int64) cardDelta {
	if txType != "expense" {
		return cardDelta{}
	}
	if cardType == cardTypeCredit {
		return cardDelta{usedCreditCents: amountCents}
	}
	return cardDelta{balanceCents: -amountCents}
}

// reverse returns the adjustment that undoes d.
func (d cardDelta) reverse() cardDelta {
	return cardDelta{balanceCents: -d.balanceCents, usedCreditCents: -d.usedCreditCents}
}

func (d cardDelta) isZero() bool {
	return d.balanceCents == 0 && d.usedCreditCents == 0
}

type CardReload struct {
	ID          int64  `json:"id"`
	CardID      int64  `json:"cardId"`
	AmountCents int64  `json:"amountCents"`
	Date        string `json:"date"`
	Note        string `json:"note"`
	CreatedAt   string `json:"createdAt"`
}

type cardReloadRequest struct {
	AmountCents int64  `json:"amountCents"`
	Date        string `json:"date"`
	Note        string `json:"note"`
}

func (r cardReloadRequest) validate() (time.Time, error) {
	if r.AmountCents <= 0 {
		return time.Time{}, fmt.Errorf("amountCents must be positive")
	}
	d, err := time.Parse(dateLayout, r.Date)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date: %s", r.Date)
	}
	return d, nil
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

type SavingsGoal struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	TargetCents   int64  `json:"targetCents"`
	CurrentCents  int64  `json:"currentCents"`
	Deadline      string `json:"deadline,omitempty"`
	Icon          string `json:"icon"`
	Color         string `json:"color"`
	DefaultCardID *int64 `json:"defaultCardId,omitempty"`
	CreatedBy     int64  `json:"createdBy"`
	CreatedAt     string `json:"createdAt"`
}

type SavingsContribution struct {
	ID          int64  `json:"id"`
	GoalID      int64  `json:"goalId"`
	AmountCents int64  `json:"amountCents"`
	Date        string `json:"date"`
	Note        string `json:"note,omitempty"`
	CreatedBy   int64  `json:"createdBy"`
	CreatedAt   string `json:"createdAt"`
}

// Deadline is a pointer so an omitted or empty field means "no deadline"
// (SQL NULL) rather than the empty string, which Postgres rejects as a date
// — see cardRequest's balance fields for the same pattern.
type savingsGoalRequest struct {
	Name          string  `json:"name"`
	TargetCents   int64   `json:"targetCents"`
	Deadline      *string `json:"deadline"`
	Icon          string  `json:"icon"`
	Color         string  `json:"color"`
	DefaultCardID *int64  `json:"defaultCardId"`
}

func (r savingsGoalRequest) validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.TargetCents <= 0 {
		return fmt.Errorf("targetCents must be positive")
	}
	if r.Deadline != nil && *r.Deadline != "" {
		if _, err := time.Parse(dateLayout, *r.Deadline); err != nil {
			return fmt.Errorf("invalid deadline date")
		}
	}
	return nil
}

// normalizedDeadline collapses an empty (but non-nil) deadline to nil, so it
// binds as SQL NULL instead of the invalid empty string.
func (r savingsGoalRequest) normalizedDeadline() *string {
	if r.Deadline == nil || *r.Deadline == "" {
		return nil
	}
	return r.Deadline
}

type savingsContributionRequest struct {
	AmountCents int64  `json:"amountCents"`
	Date        string `json:"date"`
	Note        string `json:"note"`
}

func (r savingsContributionRequest) validate() (time.Time, error) {
	if r.AmountCents <= 0 {
		return time.Time{}, fmt.Errorf("amountCents must be positive")
	}
	d, err := time.Parse(dateLayout, r.Date)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date")
	}
	return d, nil
}
