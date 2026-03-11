package export_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/dshills/witness/internal/export"
)

func TestMarkdownExporter(t *testing.T) {
	state := testState()
	evts := testEvents()

	var buf bytes.Buffer
	exp := &export.MarkdownExporter{}
	if err := exp.Export(context.Background(), state, evts, &buf); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	output := buf.String()

	// Check expected headers
	expectedHeaders := []string{
		"# Run Report: test-run",
		"## Summary",
		"## Stages",
		"## Token Usage",
		"## Commits",
		"## Alerts",
	}
	for _, header := range expectedHeaders {
		if !strings.Contains(output, header) {
			t.Errorf("output missing expected header %q", header)
		}
	}

	// Check data presence
	expectedData := []string{
		"run_001",
		"completed",
		"$0.05",
		"planning",
		"abc1234",
		"initial commit",
		"High cost",
		"main",
	}
	for _, data := range expectedData {
		if !strings.Contains(output, data) {
			t.Errorf("output missing expected data %q", data)
		}
	}

	// Check table structure for stages
	if !strings.Contains(output, "| Stage | Status | Duration |") {
		t.Error("output missing stages table header")
	}

	// Check by-model table
	if !strings.Contains(output, "### By Model") {
		t.Error("output missing By Model subsection")
	}
}
