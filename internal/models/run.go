package models

import "time"

// Run is the top-level unit of workflow execution.
type Run struct {
	RunID        string            `json:"run_id"`
	Name         string            `json:"name,omitempty"`
	RepoRoot     string            `json:"repo_root,omitempty"`
	Branch       string            `json:"branch,omitempty"`
	StartedAt    time.Time         `json:"started_at"`
	EndedAt      *time.Time        `json:"ended_at,omitempty"`
	Status       RunStatus         `json:"status"`
	Entrypoint   string            `json:"entrypoint,omitempty"`
	Command      []string          `json:"command,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	Host         string            `json:"host,omitempty"`
	User         string            `json:"user,omitempty"`
	WorkflowType string            `json:"workflow_type,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}
