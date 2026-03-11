//go:build windows

package app

import (
	"os"
	"os/exec"
)

// setProcGroup is a no-op on Windows.
func setProcGroup(_ *exec.Cmd) {}

// forwardSignal sends a signal to the process on Windows (best-effort).
func forwardSignal(p *os.Process, sig os.Signal) error {
	return p.Signal(sig)
}

// killProcessGroup kills the process on Windows (best-effort).
func killProcessGroup(p *os.Process) error {
	return p.Kill()
}
