//go:build linux
// +build linux

package runner

import (
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

func runPlain(line string) error {
	stdinFd := int(os.Stdin.Fd())
	if p, err := unix.IoctlGetTermios(stdinFd, unix.TCGETS); err == nil && p != nil {
		old := *p
		defer unix.IoctlSetTermios(stdinFd, unix.TCSETS, &old)
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}

	cmd := exec.Command(shell, "-c", line)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}
