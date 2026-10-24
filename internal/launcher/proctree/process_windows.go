//go:build windows

package proctree

import (
	"os/exec"
	"syscall"
)

func HideWindow(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
