package claudehooks

import "encoding/json"

// HookPayload is the common structure received from Claude Code hooks.
type HookPayload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	PermissionMode string `json:"permission_mode"`
	HookEventName  string `json:"hook_event_name"`
	AgentID        string `json:"agent_id,omitempty"`
	AgentType      string `json:"agent_type,omitempty"`

	// Tool-related fields (PreToolUse, PostToolUse).
	ToolName  string          `json:"tool_name,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`

	// PostToolUse only.
	ToolResult      *ToolResultPayload `json:"tool_result,omitempty"`
	ExecutionTimeMS int                `json:"execution_time_ms,omitempty"`

	// UserPromptSubmit.
	Prompt string `json:"prompt,omitempty"`

	// SessionStart / SubagentStart / SubagentStop.
	Source string `json:"source,omitempty"`

	// Notification.
	NotificationType string `json:"notification_type,omitempty"`
	Message          string `json:"message,omitempty"`
}

// ToolResultPayload is the tool_result field in PostToolUse hooks.
type ToolResultPayload struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ToolInputBash is the input for Bash tool calls.
type ToolInputBash struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
}

// ToolInputEdit is the input for Edit/Write tool calls.
type ToolInputEdit struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content,omitempty"`
}

// ToolInputRead is the input for Read tool calls.
type ToolInputRead struct {
	FilePath string `json:"file_path"`
}

// ToolInputSearch is the input for Glob/Grep tool calls.
type ToolInputSearch struct {
	Pattern string `json:"pattern,omitempty"`
	Path    string `json:"path,omitempty"`
	Query   string `json:"query,omitempty"`
}

// ToolInputAgent is the input for Agent tool calls.
type ToolInputAgent struct {
	Prompt    string `json:"prompt,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
}
