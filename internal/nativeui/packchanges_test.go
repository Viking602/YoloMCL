package nativeui

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mcl/internal/launcher"
)

func changePlanFixture() *launcher.PackUpdatePlan {
	p := &launcher.PackUpdatePlan{ID: "p", RootID: "r1", Name: "Fabulously Optimized", OldVersion: "6.2.3", NewVersion: "6.5.0", OldGame: "Minecraft 1.21.1 · fabric 0.16.9", NewGame: "Minecraft 1.21.1 · fabric 0.19.3", GameChanged: true, Unchanged: 34, DownloadBytes: 38 << 20, Changelog: "6.5.0\n- Added Crash Assistant\n- Updated Sodium"}
	p.Overwritten = []launcher.PackChange{{Path: "config/extra.txt", Reason: "user"}, {Path: "config/yosbr/config/some/really/deep/nested/folder/structure/that/never/ends/pack-info.txt", Reason: "modified"}}
	p.Updated = []launcher.PackChange{{Path: "mods/sodium-fabric-0.8.13+mc1.21.1.jar", Reason: "renamed", From: "mods/sodium-fabric-0.8.13-beta.2+mc1.21.1.jar", Disabled: true}, {Path: "config/sodium-options.json"}}
	for i := 0; i < 14; i++ {
		p.Added = append(p.Added, launcher.PackChange{Path: fmt.Sprintf("mods/some-added-mod-%02d-1.21.1.jar", i)})
	}
	p.Removed = []launcher.PackChange{{Path: "mods/lithium.jar"}}
	p.Manual = []launcher.PackChange{{Path: "mods/entityculling-fabric-1.7.1-mc1.21.jar", Reason: "kept-old", From: "mods/entityculling-fabric-1.6.6-mc1.21.jar"}}
	return p
}

func TestPackChangeListGroups(t *testing.T) {
	for _, sz := range []struct {
		name  string
		w, h  int
		scale float32
	}{{"1x", 1536, 1024, 1}, {"2x", 1536, 1024, 2}, {"min", 980, 680, 1}} {
		a, tt := acctTester(t, "zh-CN", sz.w, sz.h)
		tt.SetScale(sz.scale)
		a.openPackPlan(changePlanFixture(), false)
		tt.Advance(500 * time.Millisecond)
		if os.Getenv("MCL_NATIVE_QA_OUTPUT") != "" {
			snapshotUI(t, tt, "upd2_changes_default_"+sz.name)
		}
		if _, ok := tt.Find("变更文件 overwritten"); !ok {
			t.Fatalf("%s: 已覆盖 must start expanded", sz.name)
		}
		for _, k := range []string{"updated", "added", "removed", "manual"} {
			if _, ok := tt.Find("变更文件 " + k); ok {
				t.Fatalf("%s: %s must start collapsed", sz.name, k)
			}
		}
		box, _ := tt.Find("活动弹窗")
		hdr, _ := tt.Find("变更分组 overwritten")
		if hdr.W <= 0 || hdr.X < box.X || hdr.X+hdr.W > box.X+box.W+1 {
			t.Fatalf("%s: header outside dialog %+v %+v", sz.name, hdr, box)
		}
		// whole header toggles; opening eases in
		tt.ClickAt(hdr.X+hdr.W-20, hdr.Y+hdr.H/2)
		tt.Advance(60 * time.Millisecond)
		mid, ok := tt.Find("变更文件 overwritten")
		tt.Advance(400 * time.Millisecond)
		if _, still := tt.Find("变更文件 overwritten"); still {
			t.Fatalf("%s: header click should collapse", sz.name)
		}
		if !ok || mid.H <= 0 {
			t.Fatalf("%s: collapse should animate, not pop", sz.name)
		}
		a.packExpand = map[string]bool{"overwritten": true, "updated": true, "added": true, "removed": true, "manual": true}
		tt.Advance(500 * time.Millisecond)
		added, _ := tt.Find("变更文件 added")
		if want := float32(changeRowCap*changeRowH+8) * uiScaleFor(sz.w, sz.h); added.H > want+1 {
			t.Fatalf("%s: long group must cap and scroll: %v > %v", sz.name, added.H, want)
		}
		if os.Getenv("MCL_NATIVE_QA_OUTPUT") != "" {
			snapshotUI(t, tt, "upd2_changes_expanded_"+sz.name)
			a.packExpand = map[string]bool{}
			tt.Advance(500 * time.Millisecond)
			snapshotUI(t, tt, "upd2_changes_collapsed_"+sz.name)
		}
	}
}

func uiScaleFor(w, h int) float32 {
	return max(.65, min(1, float32(w)/1536, (float32(h)-42)/982))
}

var _ = ui.Center
