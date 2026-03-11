package models

import "time"

// Stage represents a major workflow phase within a run.
type Stage struct {
	StageID         string      `json:"stage_id"`
	RunID           string      `json:"run_id"`
	Name            string      `json:"name"`
	Order           int         `json:"order"`
	Status          StageStatus `json:"status"`
	StartedAt       *time.Time  `json:"started_at,omitempty"`
	EndedAt         *time.Time  `json:"ended_at,omitempty"`
	ProgressPercent *float64    `json:"progress_percent,omitempty"`
	Summary         string      `json:"summary,omitempty"`
}
