package nativeui

import (
	"testing"
	"time"
)

// Switching versions must not flash: no busy state (which repaints every
// button as disabled), no rescan, and the new selection shows in the same
// frame while the old card text fades out.
func TestSelectVersionSwitchesInPlace(t *testing.T) {
	a, tt := homeTester(t, "zh-CN")
	before := a.versionSwapKey()
	a.selectVersion("r1", "1.21.8")
	if a.pendingOp("select") {
		t.Fatal("selectVersion set busy; every button would flash disabled")
	}
	if r := a.root(); r == nil || r.SelectedVersion != "1.21.8" {
		t.Fatalf("selection not applied locally: %+v", r)
	}
	if a.versionSwapKey() == before {
		t.Fatal("swap key did not change; card text would not crossfade")
	}
	// The fixture root is not in the service config, so the background save
	// fails: the selection must roll back and say why, still without busy.
	deadline := time.Now().Add(5 * time.Second)
	for (a.message == "" || a.root().SelectedVersion != "quilt-loader-0.20.0-beta.9-1.21.8") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if a.message == "" || !a.errorMessage {
		t.Fatal("failed save was not reported")
	}
	if r := a.root(); r == nil || r.SelectedVersion != "quilt-loader-0.20.0-beta.9-1.21.8" || a.pendingOp("select") {
		t.Fatalf("failed save did not roll back cleanly: %+v busy=%v", r, a.pendingOp("select"))
	}
}
