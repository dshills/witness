package models

import (
	"encoding/json"
	"time"
)

// Alert represents a health or anomaly signal.
type Alert struct {
	AlertID      string          `json:"alert_id"`
	RunID        string          `json:"run_id"`
	Timestamp    time.Time       `json:"timestamp"`
	Severity     Severity        `json:"severity"`
	Type         string          `json:"type"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	RelatedIDs   []string        `json:"related_ids,omitempty"`
	Acknowledged bool            `json:"acknowledged"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}
