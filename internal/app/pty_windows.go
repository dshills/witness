//go:build windows

package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

func startPTY(_ *exec.Cmd) (*os.File, error) {
	return nil, fmt.Errorf("pty not supported on windows")
}

func handleWinch(_ *os.File) func() {
	return func() {}
}

func setupRawStdin() (func(), error) {
	return func() {}, nil
}

func relayPTY(_ *os.File, _ io.Writer) {}
