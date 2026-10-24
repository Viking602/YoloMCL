//go:build !windows

package proctree

import (
	"errors"
	"os/exec"
	"syscall"
)

// Tree on Unix: the game leads its own process group, so Kill reaches
// every child it spawned.
type Tree struct{ cmd *exec.Cmd }

func Prepare(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.Setpgid = true
}

func Attach(c *exec.Cmd) (*Tree, error) { return &Tree{cmd: c}, nil }

func (t *Tree) Kill() error {
	if t.cmd.Process == nil {
		return errors.New("no process")
	}
	return syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
}

func (t *Tree) Alive() bool {
	return t.cmd.Process != nil && syscall.Kill(-t.cmd.Process.Pid, 0) == nil
}

func (t *Tree) Close() {}
