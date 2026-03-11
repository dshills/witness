package models

import "time"

// FileChange represents a meaningful file system change relevant to a run.
type FileChange struct {
	ChangeID        string     `json:"change_id"`
	RunID           string     `json:"run_id"`
	Path            string     `json:"path"`
	ChangeType      ChangeType `json:"change_type"`
	Timestamp       time.Time  `json:"timestamp"`
	SizeBefore      *int64     `json:"size_before,omitempty"`
	SizeAfter       *int64     `json:"size_after,omitempty"`
	LineDeltaAdd    *int       `json:"line_delta_add,omitempty"`
	LineDeltaRemove *int       `json:"line_delta_remove,omitempty"`
	ContentHash     string     `json:"content_hash,omitempty"`
}
