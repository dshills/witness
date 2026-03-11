package events

import (
	"encoding/json"
	"testing"
	"time"
)

func validEvent() Event {
	return Event{
		EventID:       "evt_test123",
		SchemaVersion: SchemaVersion,
		Timestamp:     time.Now().UTC(),
		RunID:         "run_test123",
		Type:          EventRunStarted,
		Source:        "test",
		Payload:       json.RawMessage(`{}`),
	}
}

func TestValidate_ValidEvent(t *testing.T) {
	evt := validEvent()
	if err := Validate(evt); err != nil {
		t.Fatalf("expected valid event, got error: %v", err)
	}
}

func TestValidate_MissingEventID(t *testing.T) {
	evt := validEvent()
	evt.EventID = ""
	err := Validate(evt)
	if err == nil {
		t.Fatal("expected error for missing event_id")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(ve.Violations), ve.Violations)
	}
}

func TestValidate_MissingSchemaVersion(t *testing.T) {
	evt := validEvent()
	evt.SchemaVersion = ""
	err := Validate(evt)
	if err == nil {
		t.Fatal("expected error for missing schema_version")
	}
}

func TestValidate_UnknownSchemaVersion(t *testing.T) {
	evt := validEvent()
	evt.SchemaVersion = "99.0"
	err := Validate(evt)
	if err == nil {
		t.Fatal("expected error for unknown schema_version")
	}
}

func TestValidate_MissingTimestamp(t *testing.T) {
	evt := validEvent()
	evt.Timestamp = time.Time{}
	err := Validate(evt)
	if err == nil {
		t.Fatal("expected error for missing timestamp")
	}
}

func TestValidate_MissingRunID(t *testing.T) {
	evt := validEvent()
	evt.RunID = ""
	err := Validate(evt)
	if err == nil {
		t.Fatal("expected error for missing run_id")
	}
}

func TestValidate_MissingType(t *testing.T) {
	evt := validEvent()
	evt.Type = ""
	err := Validate(evt)
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestValidate_UnknownType(t *testing.T) {
	evt := validEvent()
	evt.Type = "bogus.unknown"
	err := Validate(evt)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestValidate_MissingSource(t *testing.T) {
	evt := validEvent()
	evt.Source = ""
	err := Validate(evt)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestValidate_NilPayload(t *testing.T) {
	evt := validEvent()
	evt.Payload = nil
	err := Validate(evt)
	if err == nil {
		t.Fatal("expected error for nil payload")
	}
}

func TestValidate_InvalidJSONPayload(t *testing.T) {
	evt := validEvent()
	evt.Payload = json.RawMessage(`{not json}`)
	err := Validate(evt)
	if err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
}

func TestValidate_MultipleViolations(t *testing.T) {
	evt := Event{} // everything missing
	err := Validate(evt)
	if err == nil {
		t.Fatal("expected error for empty event")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	// Should have violations for: event_id, schema_version, timestamp, run_id, type, source, payload
	if len(ve.Violations) < 7 {
		t.Fatalf("expected at least 7 violations, got %d: %v", len(ve.Violations), ve.Violations)
	}
}

func TestValidate_PayloadPreservesUnknownFields(t *testing.T) {
	payload := json.RawMessage(`{"custom_field": "value", "nested": {"deep": true}}`)
	evt := validEvent()
	evt.Payload = payload

	if err := Validate(evt); err != nil {
		t.Fatalf("expected valid event, got error: %v", err)
	}

	// Round-trip through JSON to verify payload preservation
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var original, roundtripped map[string]interface{}
	if err := json.Unmarshal(payload, &original); err != nil {
		t.Fatalf("unmarshal original payload: %v", err)
	}
	if err := json.Unmarshal(decoded.Payload, &roundtripped); err != nil {
		t.Fatalf("unmarshal roundtripped payload: %v", err)
	}

	if len(original) != len(roundtripped) {
		t.Fatalf("payload field count mismatch: %d vs %d", len(original), len(roundtripped))
	}
}
