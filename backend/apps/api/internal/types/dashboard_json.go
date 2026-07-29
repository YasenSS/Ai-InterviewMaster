package types

import "encoding/json"

// MarshalJSON keeps the public dashboard contract precise: average_score is
// absent until the user has at least one persisted report.
func (r DashboardSummaryResponse) MarshalJSON() ([]byte, error) {
	type alias DashboardSummaryResponse
	return json.Marshal(struct {
		alias
		AverageScore *float64 `json:"average_score,omitempty"`
	}{
		alias:        alias(r),
		AverageScore: r.AverageScore,
	})
}
