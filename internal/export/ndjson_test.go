package export_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/dshills/witness/internal/export"
)

func TestNDJSONExporter(t *testing.T) {
	state := testState()
	evts := testEvents()

	var buf bytes.Buffer
	exp := &export.NDJSONExporter{}
	if err := exp.Export(context.Background(), state, evts, &buf); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	scanner := bufio.NewScanner(&buf)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nline: %s", lineCount+1, err, line)
		}
		// Each line should have event_id and type
		if _, ok := obj["event_id"]; !ok {
			t.Errorf("line %d missing event_id", lineCount+1)
		}
		if _, ok := obj["type"]; !ok {
			t.Errorf("line %d missing type", lineCount+1)
		}
		lineCount++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	if lineCount != len(evts) {
		t.Errorf("got %d lines, want %d", lineCount, len(evts))
	}
}
