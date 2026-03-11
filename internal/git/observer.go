package git

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dshills/witness/internal/events"
)

const (
	gitTimeout = 10 * time.Second
	source     = "witness/git"
)

// Observer polls a git repository at a configured interval and emits
// events for branch changes, status changes, and new commits.
type Observer struct {
	repoRoot string
	interval time.Duration
	sink     events.EventSink
	runID    string

	lastBranch string
	lastSHA    string
	lastDirty  int
	firstPoll  bool
}

// NewObserver creates a git Observer.
func NewObserver(repoRoot string, interval time.Duration, sink events.EventSink, runID string) *Observer {
	return &Observer{
		repoRoot:  repoRoot,
		interval:  interval,
		sink:      sink,
		runID:     runID,
		lastDirty: -1,
		firstPoll: true,
	}
}

// Start begins polling until the context is cancelled.
func (o *Observer) Start(ctx context.Context) error {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	// Immediate first poll.
	o.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			o.poll(ctx)
		}
	}
}

func (o *Observer) poll(ctx context.Context) {
	o.pollBranch(ctx)
	o.pollStatus(ctx)
	o.pollCommits(ctx)
	o.firstPoll = false
}

func (o *Observer) pollBranch(ctx context.Context) {
	branch, err := CurrentBranch(ctx, o.repoRoot)
	if err != nil {
		log.Printf("git observer: branch check failed: %v", err)
		return
	}
	if branch != o.lastBranch {
		if !o.firstPoll && o.lastBranch != "" {
			payload, _ := json.Marshal(map[string]string{"branch": branch})
			evt := events.NewEvent(o.runID, events.EventGitBranchChanged, source, payload)
			if err := o.sink.Append(ctx, evt); err != nil {
				log.Printf("git observer: emit branch event: %v", err)
			}
		}
		o.lastBranch = branch
	}
}

func (o *Observer) pollStatus(ctx context.Context) {
	dirty, err := dirtyFileCount(ctx, o.repoRoot)
	if err != nil {
		log.Printf("git observer: status check failed: %v", err)
		return
	}
	if dirty != o.lastDirty {
		if !o.firstPoll && o.lastDirty >= 0 {
			payload, _ := json.Marshal(map[string]int{"dirty_files": dirty})
			evt := events.NewEvent(o.runID, events.EventRepoStatusChanged, source, payload)
			if err := o.sink.Append(ctx, evt); err != nil {
				log.Printf("git observer: emit status event: %v", err)
			}
		}
		o.lastDirty = dirty
	}
}

func (o *Observer) pollCommits(ctx context.Context) {
	headSHA, err := headSHA(ctx, o.repoRoot)
	if err != nil {
		log.Printf("git observer: HEAD SHA failed: %v", err)
		return
	}

	if o.firstPoll {
		o.lastSHA = headSHA
		return
	}

	if headSHA == o.lastSHA {
		return
	}

	// Get new commits since last known SHA.
	commits, err := newCommitsSince(ctx, o.repoRoot, o.lastSHA)
	if err != nil {
		log.Printf("git observer: log failed: %v", err)
		o.lastSHA = headSHA
		return
	}

	for _, c := range commits {
		stats := commitStats(ctx, o.repoRoot, c.sha)
		payload, _ := json.Marshal(map[string]any{
			"commit_id":     events.NewID("commit"),
			"sha":           c.sha,
			"message":       c.message,
			"files_changed": stats.filesChanged,
			"insertions":    stats.insertions,
			"deletions":     stats.deletions,
		})
		evt := events.NewEvent(o.runID, events.EventGitCommitCreated, source, payload)
		if err := o.sink.Append(ctx, evt); err != nil {
			log.Printf("git observer: emit commit event: %v", err)
		}
	}

	o.lastSHA = headSHA
}

// DetectRepoRoot finds the git repository root for the given path.
func DetectRepoRoot(ctx context.Context, path string) (string, error) {
	out, err := gitCmd(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// CurrentBranch returns the current branch name.
func CurrentBranch(ctx context.Context, repoRoot string) (string, error) {
	out, err := gitCmd(ctx, repoRoot, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		// Detached HEAD; fall back to rev-parse.
		out, err = gitCmd(ctx, repoRoot, "rev-parse", "--short", "HEAD")
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(out), nil
}

func headSHA(ctx context.Context, repoRoot string) (string, error) {
	out, err := gitCmd(ctx, repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func dirtyFileCount(ctx context.Context, repoRoot string) (int, error) {
	out, err := gitCmd(ctx, repoRoot, "status", "--porcelain")
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(out) == "" {
		return 0, nil
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	return len(lines), nil
}

type commitInfo struct {
	sha     string
	message string
}

func newCommitsSince(ctx context.Context, repoRoot, sinceSHA string) ([]commitInfo, error) {
	out, err := gitCmd(ctx, repoRoot, "log", "--oneline", "--reverse", sinceSHA+"..HEAD")
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return nil, nil
	}

	var commits []commitInfo
	scanner := bufio.NewScanner(strings.NewReader(trimmed))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		msg := ""
		if len(parts) == 2 {
			msg = parts[1]
		}
		commits = append(commits, commitInfo{sha: parts[0], message: msg})
	}
	return commits, nil
}

type changeStats struct {
	filesChanged int
	insertions   int
	deletions    int
}

func commitStats(ctx context.Context, repoRoot, sha string) changeStats {
	out, err := gitCmd(ctx, repoRoot, "show", "--stat", "--format=", sha)
	if err != nil {
		return changeStats{}
	}
	return parseStatSummary(out)
}

// parseStatSummary parses the summary line of `git show --stat`.
// Example: " 3 files changed, 10 insertions(+), 2 deletions(-)"
func parseStatSummary(out string) changeStats {
	var stats changeStats
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		return stats
	}
	summary := lines[len(lines)-1]
	for _, part := range strings.Split(summary, ",") {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		switch {
		case strings.Contains(fields[1], "file"):
			stats.filesChanged = n
		case strings.Contains(fields[1], "insertion"):
			stats.insertions = n
		case strings.Contains(fields[1], "deletion"):
			stats.deletions = n
		}
	}
	return stats
}

func gitCmd(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}
