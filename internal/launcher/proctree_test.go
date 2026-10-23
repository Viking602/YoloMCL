package launcher

import (
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// A game that spawns a long-lived child (like Crash Assistant's watcher
// JVM): killing the tree must end the child too, and the session wait must
// return only once the whole tree is gone.
func TestStopKillsWholeProcessTree(t *testing.T) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "start", "/b", "ping", "-n", "120", "127.0.0.1", ">nul", "&", "ping", "-n", "120", "127.0.0.1", ">nul")
	} else {
		cmd = exec.Command("sh", "-c", "sleep 120 & sleep 120")
	}
	hideWindow(cmd)
	prepareTree(cmd)
	if e := cmd.Start(); e != nil {
		t.Skip("cannot start shell:", e)
	}
	tree, e := attachTree(cmd)
	if e != nil {
		t.Fatal(e)
	}
	g := &runningGame{cmd: cmd, tree: tree}
	time.Sleep(700 * time.Millisecond) // let the child spawn
	if !tree.alive() {
		t.Fatal("tree should be alive")
	}
	if e := g.killTree(); e != nil {
		t.Fatal(e)
	}
	_ = cmd.Wait()
	done := make(chan struct{})
	go func() { g.waitTreeGone(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("child process survived the stop")
	}
}

// Killing only the leader (the old behaviour) leaves the child running, so
// waitTreeGone keeps the session open until the tree is really gone.
func TestTreeAliveWhileChildRuns(t *testing.T) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "start", "/b", "ping", "-n", "120", "127.0.0.1", ">nul", "&", "ping", "-n", "120", "127.0.0.1", ">nul")
	} else {
		cmd = exec.Command("sh", "-c", "sleep 120 & sleep 120")
	}
	hideWindow(cmd)
	prepareTree(cmd)
	if e := cmd.Start(); e != nil {
		t.Skip("cannot start shell:", e)
	}
	tree, _ := attachTree(cmd)
	time.Sleep(700 * time.Millisecond)
	_ = cmd.Process.Kill()
	time.Sleep(500 * time.Millisecond)
	if !tree.alive() {
		t.Fatal("child should still run after killing only the leader")
	}
	_ = tree.kill()
	go func() { _ = cmd.Wait() }()
	deadline := time.Now().Add(10 * time.Second)
	for tree.alive() && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if tree.alive() {
		t.Fatal("tree kill left processes")
	}
	tree.close()
}
