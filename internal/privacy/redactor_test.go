package privacy

import "testing"

func TestRedactor_APIKeys(t *testing.T) {
	r, err := NewRedactor([]string{`(?i)(sk-[a-zA-Z0-9]{20,}|AKIA[A-Z0-9]{16})`})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"key: sk-abcdefghijklmnopqrstuvwxyz", "key: [REDACTED]"},
		{"aws: AKIAIOSFODNN7EXAMPLE", "aws: [REDACTED]"},
		{"no secrets here", "no secrets here"},
	}
	for _, tt := range tests {
		got := r.Redact(tt.input)
		if got != tt.want {
			t.Errorf("Redact(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRedactor_BearerTokens(t *testing.T) {
	r, err := NewRedactor([]string{`(?i)bearer\s+[A-Za-z0-9\-._~+/=]{20,}`})
	if err != nil {
		t.Fatal(err)
	}

	input := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test"
	got := r.Redact(input)
	if got == input {
		t.Error("expected bearer token to be redacted")
	}
}

func TestRedactor_NormalTextUnchanged(t *testing.T) {
	r, err := NewRedactor([]string{
		`(?i)(sk-[a-zA-Z0-9]{20,}|AKIA[A-Z0-9]{16})`,
		`(?i)bearer\s+[A-Za-z0-9\-._~+/=]{20,}`,
		`(?i)(password|secret|apikey)\s*[=:]\s*\S{8,}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	normal := "input_tokens: 1234, output_tokens: 5678"
	got := r.Redact(normal)
	if got != normal {
		t.Errorf("normal text should be unchanged, got %q", got)
	}
}

func TestRedactor_EmptyPatterns(t *testing.T) {
	r, err := NewRedactor(nil)
	if err != nil {
		t.Fatal(err)
	}
	input := "sk-abcdefghijklmnopqrstuvwxyz"
	got := r.Redact(input)
	if got != input {
		t.Errorf("empty patterns should be no-op, got %q", got)
	}
}

func TestRedactor_InvalidPattern(t *testing.T) {
	_, err := NewRedactor([]string{`(?P<bad`})
	if err == nil {
		t.Fatal("expected error for invalid regex pattern")
	}
}
