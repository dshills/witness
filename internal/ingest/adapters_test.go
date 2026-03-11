package ingest

import (
	"testing"
)

func TestGenericJSONAdapter_CanParse_Valid(t *testing.T) {
	adapter := &GenericJSONAdapter{}

	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "valid tool result",
			line: `{"tool":"lint","status":"pass","summary":"ok"}`,
			want: true,
		},
		{
			name: "minimal tool result",
			line: `{"tool":"check","status":"fail"}`,
			want: true,
		},
		{
			name: "missing tool field",
			line: `{"status":"pass","summary":"ok"}`,
			want: false,
		},
		{
			name: "missing status field",
			line: `{"tool":"lint","summary":"ok"}`,
			want: false,
		},
		{
			name: "not JSON",
			line: `Building project...`,
			want: false,
		},
		{
			name: "empty object",
			line: `{}`,
			want: false,
		},
		{
			name: "empty string",
			line: ``,
			want: false,
		},
		{
			name: "witness event not tool result",
			line: `{"event_id":"evt_123","type":"run.started","run_id":"run_1"}`,
			want: false,
		},
		{
			name: "array not object",
			line: `[1,2,3]`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.CanParse([]byte(tt.line))
			if got != tt.want {
				t.Errorf("CanParse(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestGenericJSONAdapter_Parse(t *testing.T) {
	adapter := &GenericJSONAdapter{}
	line := []byte(`{"tool":"prism","status":"pass","summary":"clean","findings":{"error":0,"warning":1},"duration_ms":800}`)

	result, err := adapter.Parse(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Tool != "prism" {
		t.Errorf("tool = %q, want %q", result.Tool, "prism")
	}
	if result.DurationMS != 800 {
		t.Errorf("duration_ms = %d, want 800", result.DurationMS)
	}
}

func TestAdapterRegistry_TryParse_Match(t *testing.T) {
	registry := NewAdapterRegistry()
	line := []byte(`{"tool":"golangci-lint","status":"pass","summary":"no issues"}`)

	result, err := registry.TryParse(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Tool != "golangci-lint" {
		t.Errorf("tool = %q, want %q", result.Tool, "golangci-lint")
	}
}

func TestAdapterRegistry_TryParse_NoMatch(t *testing.T) {
	registry := NewAdapterRegistry()

	tests := []struct {
		name string
		line string
	}{
		{"plain text", "Building project..."},
		{"empty", ""},
		{"partial JSON", `{"foo":"bar"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := registry.TryParse([]byte(tt.line))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != nil {
				t.Errorf("expected nil result for %q, got %+v", tt.line, result)
			}
		})
	}
}

// mockAdapter is a test adapter that matches lines starting with "CUSTOM:".
type mockAdapter struct{}

func (m *mockAdapter) CanParse(line []byte) bool {
	return len(line) > 7 && string(line[:7]) == "CUSTOM:"
}

func (m *mockAdapter) Parse(_ []byte) (*ToolResult, error) {
	return &ToolResult{
		Tool:   "custom-tool",
		Status: "pass",
	}, nil
}

func TestAdapterRegistry_CustomAdapter_FirstMatchWins(t *testing.T) {
	registry := &AdapterRegistry{
		adapters: []ToolAdapter{
			&mockAdapter{},        // Checked first
			&GenericJSONAdapter{}, // Checked second
		},
	}

	// mockAdapter matches this line
	result, err := registry.TryParse([]byte("CUSTOM: some output"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result from custom adapter")
	}
	if result.Tool != "custom-tool" {
		t.Errorf("tool = %q, want %q", result.Tool, "custom-tool")
	}

	// GenericJSONAdapter matches this line (mockAdapter does not)
	result2, err := registry.TryParse([]byte(`{"tool":"lint","status":"ok"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2 == nil {
		t.Fatal("expected result from generic adapter")
	}
	if result2.Tool != "lint" {
		t.Errorf("tool = %q, want %q", result2.Tool, "lint")
	}
}
