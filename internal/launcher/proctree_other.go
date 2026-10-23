//go:build !windows

package launcher

import (
	"errors"
	"os/exec"
	"syscall"
)

// procTree on Unix: the game leads its own process group, so kill reaches
// every child it spawned.
type procTree struct{ cmd *exec.Cmd }

func prepareTree(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.Setpgid = true
}

func attachTree(c *exec.Cmd) (*procTree, error) { return &procTree{cmd: c}, nil }

func (t *procTree) kill() error {
	if t.cmd.Process == nil {
		return errors.New("no process")
	}
	return syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
}

func (t *procTree) alive() bool {
	return t.cmd.Process != nil && syscall.Kill(-t.cmd.Process.Pid, 0) == nil
}

func (t *procTree) close() {}
