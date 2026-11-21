//go:build windows

package proctree

import (
	"os/exec"
	"syscall"
)

func HideWindow(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.HideWindow = true
}

// HideConsole suppresses a console window on Windows without setting SW_HIDE on
// STARTUPINFO, so native GUI games (such as Horizon) still show their main window.
func HideConsole(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	const createNoWindow = 0x08000000
	c.SysProcAttr.CreationFlags |= createNoWindow
}
