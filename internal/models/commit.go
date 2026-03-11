package models

import "time"

// Commit represents a Git commit created during a run.
type Commit struct {
	CommitID     string    `json:"commit_id"`
	RunID        string    `json:"run_id"`
	SHA          string    `json:"sha"`
	Timestamp    time.Time `json:"timestamp"`
	Message      string    `json:"message"`
	AuthorName   string    `json:"author_name,omitempty"`
	AuthorEmail  string    `json:"author_email,omitempty"`
	FilesChanged *int      `json:"files_changed,omitempty"`
	Insertions   *int      `json:"insertions,omitempty"`
	Deletions    *int      `json:"deletions,omitempty"`
}
