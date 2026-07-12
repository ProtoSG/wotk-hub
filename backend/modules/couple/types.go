package couple

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

var categories = []string{
	"cena", "almuerzo", "cine", "viaje", "aire_libre", "casa", "evento", "otro",
}

var statuses = []string{"planned", "done"}

const dateLayout = "2006-01-02"

type Date struct {
	ID         int64  `json:"id"`
	OccurredOn string `json:"occurredOn"`
	Place      string `json:"place"`
	Category   string `json:"category"`
	Notes      string `json:"notes"`
	CostCents  *int64 `json:"costCents,omitempty"`
	Rating     *int   `json:"rating,omitempty"`
	TiktokURL  string `json:"tiktokUrl"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
}

type dateRequest struct {
	OccurredOn string `json:"occurredOn"`
	Place      string `json:"place"`
	Category   string `json:"category"`
	Notes      string `json:"notes"`
	CostCents  *int64 `json:"costCents"`
	Rating     *int   `json:"rating"`
	TiktokURL  string `json:"tiktokUrl"`
	Status     string `json:"status"`
}

func (r dateRequest) validate() (time.Time, error) {
	d, err := time.Parse(dateLayout, r.OccurredOn)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid occurredOn: %s", r.OccurredOn)
	}
	if !slices.Contains(categories, r.Category) {
		return time.Time{}, fmt.Errorf("invalid category: %s", r.Category)
	}
	if r.CostCents != nil && *r.CostCents < 0 {
		return time.Time{}, fmt.Errorf("costCents cannot be negative")
	}
	if r.Rating != nil && (*r.Rating < 1 || *r.Rating > 5) {
		return time.Time{}, fmt.Errorf("rating must be between 1 and 5")
	}
	if !slices.Contains(statuses, r.Status) {
		return time.Time{}, fmt.Errorf("invalid status: %s", r.Status)
	}
	if r.TiktokURL != "" {
		u, err := url.Parse(r.TiktokURL)
		if err != nil || u.Scheme != "https" || !strings.HasSuffix(u.Hostname(), "tiktok.com") {
			return time.Time{}, fmt.Errorf("invalid tiktokUrl: must be an https tiktok.com link")
		}
	}
	return d, nil
}
