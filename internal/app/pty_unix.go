//go:build !windows

package app

import (
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// startPTY starts cmd with a PTY and returns the master side.
// The caller must close ptmx when done.
func startPTY(cmd *exec.Cmd) (*os.File, error) {
	sz, err := pty.GetsizeFull(os.Stdin)
	if err != nil {
		sz = &pty.Winsize{Rows: 24, Cols: 80}
	}
	return pty.StartWithSize(cmd, sz)
}

// handleWinch relays terminal resize signals to the PTY.
// Returns a stop function.
func handleWinch(ptmx *os.File) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)

	go func() {
		for range ch {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()

	// Trigger initial resize.
	ch <- syscall.SIGWINCH

	return func() {
		signal.Stop(ch)
		close(ch)
	}
}

// setupRawStdin puts stdin into raw mode and returns a restore function.
func setupRawStdin() (func(), error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return func() {}, err
	}
	return func() { _ = term.Restore(fd, oldState) }, nil
}

// relayPTY copies stdin to the PTY and tees PTY output to stdout + w.
// Returns after the output relay finishes (child closed PTY).
func relayPTY(ptmx *os.File, w io.Writer) {
	// stdin -> child
	go func() {
		_, _ = io.Copy(ptmx, os.Stdin)
	}()

	// child -> stdout + ingest pipe
	tee := io.TeeReader(ptmx, w)
	_, _ = io.Copy(os.Stdout, tee)
}
