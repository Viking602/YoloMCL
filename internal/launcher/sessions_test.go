package launcher

import (
	"errors"
	"testing"
)

type exitErr struct{ code int }

func (e exitErr) Error() string { return "exit status" }
func (e exitErr) ExitCode() int { return e.code }

func TestSessionsPerLaunch(t *testing.T) {
	s := &Service{}
	a := Root{ID: "a", Name: "MC", SelectedVersion: "Pack A"}
	b := Root{ID: "b", Name: "MC", SelectedVersion: "Pack B"}
	la, lb := &gameLog{ring: newLineRing(10)}, &gameLog{ring: newLineRing(10)}
	ga := s.newSessionLocked(a, la, nil)
	gb := s.newSessionLocked(b, lb, nil)
	la.Write([]byte("only a\n"))
	lb.Write([]byte("only b\n"))
	ta, _ := s.SessionTail(ga.ID, 0)
	tb, _ := s.SessionTail(gb.ID, 0)
	if len(ta.Lines) != 1 || ta.Lines[0] != "only a" || tb.Lines[0] != "only b" {
		t.Fatalf("logs mixed: %v %v", ta.Lines, tb.Lines)
	}
	s.endSession(ga, exitErr{1})
	if ga.State != "crashed" || ga.ExitCode != 1 {
		t.Fatalf("want crashed, got %s %d", ga.State, ga.ExitCode)
	}
	gb.stopped = true
	s.endSession(gb, exitErr{1})
	if gb.State != "exited" {
		t.Fatalf("user stop must read exited, got %s", gb.State)
	}
	g2 := s.newSessionLocked(a, &gameLog{ring: newLineRing(10)}, nil)
	if g2.Seq != 2 {
		t.Fatalf("relaunch seq %d", g2.Seq)
	}
	if len(s.Sessions("a")) != 2 || len(s.Sessions("")) != 3 {
		t.Fatal("scope filter")
	}
	if e := s.CloseSession(g2.ID); e == nil {
		t.Fatal("running session must not close")
	}
	for i := 0; i < 10; i++ {
		g := s.newSessionLocked(b, &gameLog{ring: newLineRing(10)}, nil)
		s.endSession(g, errors.New("x"))
	}
	exited := 0
	for _, x := range s.Sessions("") {
		if x.State != "running" {
			exited++
		}
	}
	if exited != MaxExitedSessions {
		t.Fatalf("exited kept %d", exited)
	}
}

func TestSessionNameIsInstance(t *testing.T) {
	s := &Service{}
	g := s.newSessionLocked(Root{ID: "r", Name: "MC", SelectedVersion: "Pandemonium"}, &gameLog{ring: newLineRing(4)}, nil)
	if g.Name != "Pandemonium" {
		t.Fatalf("name %q", g.Name)
	}
}
