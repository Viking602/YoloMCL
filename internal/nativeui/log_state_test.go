package nativeui

import "testing"

func TestLogDeriveFilterSearchCollapse(t *testing.T) {
	lv := &logViewState{lines: parseLogLines([]string{
		"[1:00:00] [main/INFO] [a/]: hello",
		"[1:00:01] [main/ERROR] [b/]: crash",
		"java.lang.Exception: x",
		"\tat x.y(Z.java:1)",
		"[1:00:02] [main/WARN] [c/]: warn",
	}, "")}
	lv.derive()
	if len(lv.vis) != 3 || lv.counts["error"] != 1 || lv.counts["info"] != 1 {
		t.Fatalf("collapsed: %v %v", lv.vis, lv.counts)
	}
	lv.open = map[string]bool{lv.lines[1].Raw: true}
	lv.derive()
	if len(lv.vis) != 5 {
		t.Fatalf("expanded: %v", lv.vis)
	}
	lv.levels = map[string]bool{"warn": true}
	lv.derive()
	if len(lv.vis) != 1 {
		t.Fatalf("warn only: %v", lv.vis)
	}
	lv.levels, lv.open, lv.query = nil, nil, "z.java"
	lv.derive()
	if len(lv.hits) != 1 || len(lv.vis) != 5 {
		t.Fatalf("search opens group: %v %v", lv.vis, lv.hits)
	}
	lv.query, lv.clearAt = "", 4
	lv.derive()
	if len(lv.vis) != 1 {
		t.Fatalf("clear: %v", lv.vis)
	}
}
