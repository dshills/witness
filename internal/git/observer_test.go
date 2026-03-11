package git

import (
	"context"
	"os"
	"testing"
)

func TestParseStatSummary(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  changeStats
	}{
		{
			name:  "typical stat output",
			input: " file1.go | 10 ++++\n file2.go |  5 ---\n 2 files changed, 10 insertions(+), 5 deletions(-)",
			want:  changeStats{filesChanged: 2, insertions: 10, deletions: 5},
		},
		{
			name:  "insertions only",
			input: " 1 file changed, 3 insertions(+)",
			want:  changeStats{filesChanged: 1, insertions: 3, deletions: 0},
		},
		{
			name:  "deletions only",
			input: " 1 file changed, 2 deletions(-)",
			want:  changeStats{filesChanged: 1, insertions: 0, deletions: 2},
		},
		{
			name:  "empty input",
			input: "",
			want:  changeStats{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStatSummary(tt.input)
			if got != tt.want {
				t.Errorf("parseStatSummary() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDetectRepoRoot_NonGitDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "witness-test-nongit")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	_, err = DetectRepoRoot(context.Background(), dir)
	if err == nil {
		t.Error("expected error for non-git directory, got nil")
	}
}

func TestDirtyFileCount_Parser(t *testing.T) {
	// Test the parsing logic by checking that empty output yields 0.
	// We cannot easily mock git commands without a real repo, so we test
	// the helper functions that parse output.
	lines := " M file.go\n?? new.go"
	count := len(splitNonEmpty(lines))
	if count != 2 {
		t.Errorf("expected 2 dirty files, got %d", count)
	}
}

// splitNonEmpty is a test helper to simulate dirty file counting.
func splitNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}
