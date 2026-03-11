package replay

import (
	"strings"
	"testing"
	"time"

	"github.com/dshills/witness/internal/aggregate"
	"github.com/dshills/witness/internal/models"
)

func TestGeneratePostmortem(t *testing.T) {
	t0 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(5 * time.Minute)

	stageStart := t0.Add(30 * time.Second)
	stageEnd := t0.Add(4 * time.Minute)

	state := aggregate.RunState{
		Run: models.Run{
			RunID:     "run_123",
			Name:      "my-workflow",
			RepoRoot:  "/home/user/repo",
			Branch:    "main",
			Command:   []string{"claude", "code"},
			Status:    models.RunStatusCompleted,
			StartedAt: t0,
			EndedAt:   &t1,
		},
		Stages: []models.Stage{
			{
				StageID:   "stage_1",
				Name:      "setup",
				Order:     0,
				Status:    models.StageStatusCompleted,
				StartedAt: &stageStart,
				EndedAt:   &stageEnd,
			},
		},
		TotalInputTokens:  50000,
		TotalOutputTokens: 10000,
		TotalCachedTokens: 5000,
		TotalCostUSD:      1.2345,
		CostByModel: map[string]float64{
			"claude-3-opus":  1.0,
			"claude-3-haiku": 0.2345,
		},
		Commits: []models.Commit{
			{
				CommitID: "c1",
				SHA:      "abc1234def5678",
				Message:  "initial commit",
			},
			{
				CommitID: "c2",
				SHA:      "def5678abc1234",
				Message:  "add feature",
			},
		},
		HotFiles: map[string]int{
			"main.go": 3,
			"util.go": 1,
			"test.go": 2,
		},
		Alerts: []models.Alert{
			{
				AlertID:     "a1",
				Severity:    models.SeverityWarning,
				Title:       "Budget Warning",
				Description: "Approaching budget limit",
			},
		},
		FailureCount: 1,
	}

	result := GeneratePostmortem(state)

	// Verify key sections are present.
	checks := []struct {
		label    string
		contains string
	}{
		{"title", "Postmortem Summary"},
		{"run name", "my-workflow"},
		{"run id", "run_123"},
		{"repo", "/home/user/repo"},
		{"branch", "main"},
		{"command", "claude code"},
		{"status", "completed"},
		{"duration", "5m0s"},
		{"failures", "Failures: 1"},
		{"stage name", "setup"},
		{"stage status", "completed"},
		{"stage icon", "[OK]"},
		{"input tokens", "50000"},
		{"output tokens", "10000"},
		{"cached tokens", "5000"},
		{"total cost", "$1.2345"},
		{"commits total", "Total: 2"},
		{"files touched", "Files touched: 3"},
		{"commit sha", "abc1234"},
		{"commit message", "initial commit"},
		{"alert severity", "warning"},
		{"alert title", "Budget Warning"},
	}

	for _, c := range checks {
		if !strings.Contains(result, c.contains) {
			t.Errorf("postmortem missing %s: expected to contain %q\nGot:\n%s", c.label, c.contains, result)
		}
	}
}

func TestGeneratePostmortemMinimal(t *testing.T) {
	state := aggregate.RunState{
		Run: models.Run{
			RunID:  "run_min",
			Name:   "minimal",
			Status: models.RunStatusFailed,
		},
	}

	result := GeneratePostmortem(state)

	if !strings.Contains(result, "Postmortem Summary") {
		t.Error("minimal postmortem should contain title")
	}
	if !strings.Contains(result, "minimal") {
		t.Error("minimal postmortem should contain run name")
	}
	if !strings.Contains(result, "failed") {
		t.Error("minimal postmortem should contain status")
	}

	// Should NOT contain sections for empty data.
	if strings.Contains(result, "--- Stages ---") {
		t.Error("minimal postmortem should not contain stages section")
	}
	if strings.Contains(result, "--- Cost ---") {
		t.Error("minimal postmortem should not contain cost section")
	}
	if strings.Contains(result, "--- Commits ---") {
		t.Error("minimal postmortem should not contain commits section")
	}
	if strings.Contains(result, "--- Alerts ---") {
		t.Error("minimal postmortem should not contain alerts section")
	}
}
