package events

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidationError contains all validation violations for an event.
type ValidationError struct {
	Violations []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("event validation failed: %s", strings.Join(e.Violations, "; "))
}

// Validate checks an event for required fields and returns all violations.
func Validate(evt Event) error {
	var violations []string

	if evt.EventID == "" {
		violations = append(violations, "event_id is required")
	}
	if evt.SchemaVersion == "" {
		violations = append(violations, "schema_version is required")
	} else if evt.SchemaVersion != SchemaVersion {
		violations = append(violations, fmt.Sprintf("unknown schema_version: %q", evt.SchemaVersion))
	}
	if evt.Timestamp.IsZero() {
		violations = append(violations, "timestamp is required")
	}
	if evt.RunID == "" {
		violations = append(violations, "run_id is required")
	}
	if evt.Type == "" {
		violations = append(violations, "type is required")
	} else if !IsKnownEventType(evt.Type) {
		violations = append(violations, fmt.Sprintf("unknown event type: %q", evt.Type))
	}
	if evt.Source == "" {
		violations = append(violations, "source is required")
	}
	if evt.Payload == nil {
		violations = append(violations, "payload is required")
	} else if !json.Valid(evt.Payload) {
		violations = append(violations, "payload is not valid JSON")
	}

	if len(violations) > 0 {
		return &ValidationError{Violations: violations}
	}
	return nil
}
