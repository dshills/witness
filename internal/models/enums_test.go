package models

import (
	"encoding/json"
	"testing"
)

func TestRunStatus_RoundTrip(t *testing.T) {
	statuses := []RunStatus{
		RunStatusPending, RunStatusRunning, RunStatusCompleted,
		RunStatusFailed, RunStatusCancelled, RunStatusStalled, RunStatusUnknown,
	}
	for _, s := range statuses {
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("Marshal(%v): %v", s, err)
		}
		var got RunStatus
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", data, err)
		}
		if got != s {
			t.Errorf("round-trip: got %v, want %v", got, s)
		}
	}
}

func TestRunStatus_UnknownValue(t *testing.T) {
	var s RunStatus
	err := json.Unmarshal([]byte(`"bogus"`), &s)
	if err == nil {
		t.Fatal("expected error for unknown RunStatus")
	}
}

func TestStageStatus_RoundTrip(t *testing.T) {
	statuses := []StageStatus{
		StageStatusPending, StageStatusRunning, StageStatusCompleted,
		StageStatusFailed, StageStatusSkipped,
	}
	for _, s := range statuses {
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("Marshal(%v): %v", s, err)
		}
		var got StageStatus
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", data, err)
		}
		if got != s {
			t.Errorf("round-trip: got %v, want %v", got, s)
		}
	}
}

func TestStageStatus_UnknownValue(t *testing.T) {
	var s StageStatus
	err := json.Unmarshal([]byte(`"bogus"`), &s)
	if err == nil {
		t.Fatal("expected error for unknown StageStatus")
	}
}

func TestChangeType_RoundTrip(t *testing.T) {
	types := []ChangeType{
		ChangeTypeCreated, ChangeTypeModified, ChangeTypeDeleted, ChangeTypeRenamed,
	}
	for _, ct := range types {
		data, err := json.Marshal(ct)
		if err != nil {
			t.Fatalf("Marshal(%v): %v", ct, err)
		}
		var got ChangeType
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", data, err)
		}
		if got != ct {
			t.Errorf("round-trip: got %v, want %v", got, ct)
		}
	}
}

func TestSeverity_RoundTrip(t *testing.T) {
	sevs := []Severity{SeverityInfo, SeverityWarning, SeverityError, SeverityCritical}
	for _, s := range sevs {
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("Marshal(%v): %v", s, err)
		}
		var got Severity
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", data, err)
		}
		if got != s {
			t.Errorf("round-trip: got %v, want %v", got, s)
		}
	}
}

func TestInvocationStatus_RoundTrip(t *testing.T) {
	statuses := []InvocationStatus{
		InvocationStatusPending, InvocationStatusRunning,
		InvocationStatusCompleted, InvocationStatusFailed,
	}
	for _, s := range statuses {
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("Marshal(%v): %v", s, err)
		}
		var got InvocationStatus
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", data, err)
		}
		if got != s {
			t.Errorf("round-trip: got %v, want %v", got, s)
		}
	}
}
