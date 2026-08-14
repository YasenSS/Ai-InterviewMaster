package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDashboardSummaryOmitsMissingAverageScore(t *testing.T) {
	payload, err := json.Marshal(DashboardSummaryResponse{
		ScoreTrend:        []ScoreTrendResponse{},
		ImprovementTopics: []ImprovementTopicResponse{},
		RecentResumes:     []ResumeSummaryResponse{},
		RecentInterviews:  []InterviewSummaryResponse{},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(payload), `"average_score"`) {
		t.Fatalf("missing average score should be omitted: %s", payload)
	}
}
