package ingest

import (
	"encoding/json"
	"sync"
)

// ToolAdapter can detect and parse a specific tool's JSON output format.
type ToolAdapter interface {
	// CanParse returns true if the line matches this adapter's format.
	CanParse(line []byte) bool
	// Parse parses the line into a ToolResult.
	Parse(line []byte) (*ToolResult, error)
}

// AdapterRegistry holds an ordered list of adapters and tries them
// in sequence. The first adapter whose CanParse returns true wins.
type AdapterRegistry struct {
	mu       sync.RWMutex
	adapters []ToolAdapter
}

// NewAdapterRegistry creates a registry with the default adapters.
func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{
		adapters: []ToolAdapter{
			&GenericJSONAdapter{},
		},
	}
}

// Register adds an adapter to the end of the registry.
func (r *AdapterRegistry) Register(a ToolAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters = append(r.adapters, a)
}

// TryParse attempts to parse the line using the first matching adapter.
// Returns nil, nil if no adapter matches.
func (r *AdapterRegistry) TryParse(line []byte) (*ToolResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, a := range r.adapters {
		if a.CanParse(line) {
			return a.Parse(line)
		}
	}
	return nil, nil
}

// GenericJSONAdapter parses the standard ToolResult JSON schema.
// It matches any JSON object that has both "tool" and "status" fields.
type GenericJSONAdapter struct{}

// CanParse returns true if the line is JSON containing "tool" and "status" keys.
func (a *GenericJSONAdapter) CanParse(line []byte) bool {
	if len(line) == 0 || line[0] != '{' {
		return false
	}
	// Quick probe: unmarshal just the fields we need.
	var probe struct {
		Tool   string `json:"tool"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(line, &probe); err != nil {
		return false
	}
	return probe.Tool != "" && probe.Status != ""
}

// Parse parses the line as a standard ToolResult.
func (a *GenericJSONAdapter) Parse(line []byte) (*ToolResult, error) {
	return ParseToolResult(line)
}
