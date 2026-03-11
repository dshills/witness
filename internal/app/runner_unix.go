//go:build !windows

package app

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcGroup configures the command to run in its own process group.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// forwardSignal sends a signal to the process group of the given process.
func forwardSignal(p *os.Process, sig os.Signal) error {
	s, ok := sig.(syscall.Signal)
	if !ok {
		return p.Signal(sig)
	}
	return syscall.Kill(-p.Pid, s)
}

// killProcessGroup sends SIGKILL to the process group.
func killProcessGroup(p *os.Process) error {
	return syscall.Kill(-p.Pid, syscall.SIGKILL)
}
