package privacy

import (
	"fmt"
	"regexp"
)

// Redactor applies regex-based redaction to strings.
type Redactor struct {
	patterns []*regexp.Regexp
}

// NewRedactor compiles the given regex patterns into a Redactor.
// Returns an error if any pattern fails to compile.
func NewRedactor(patterns []string) (*Redactor, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid redaction pattern %q: %w", p, err)
		}
		compiled = append(compiled, re)
	}
	return &Redactor{patterns: compiled}, nil
}

// Redact replaces all matches of configured patterns with "[REDACTED]".
func (r *Redactor) Redact(s string) string {
	for _, re := range r.patterns {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}
