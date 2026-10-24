//go:build !windows

package proctree

import "os/exec"

func HideWindow(c *exec.Cmd) {}
