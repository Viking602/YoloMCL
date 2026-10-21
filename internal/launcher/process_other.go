//go:build !windows

package launcher

import "os/exec"

func hideWindow(c *exec.Cmd) {}
