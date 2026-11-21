package nativeui

import (
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Switching top-level tabs slides the previous page out across multiple paint
// passes. Custom Draw closures on the exiting page (such as storageBar) must
// never read LayoutBounds on a ui.Element from the earlier pass.
func TestTabSwitchFromOverviewWithStorageBar(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1440, 900)
	a.closeDialog()
	a.snapshot.Instances = []launcher.Instance{{
		Root: launcher.Root{ID: "r1", Name: "MC", Path: `D:\MC\.minecraft`, SelectedVersion: "neoforge-21.1.250"},
		Versions: []launcher.VersionSummary{
			{ID: "neoforge-21.1.250", Minecraft: "1.21.1", Loader: "neoforge", LoaderVersion: "21.1.250"},
		},
	}}
	a.snapshot.ActiveRoot = "r1"
	a.instTab = "overview"
	a.instStatusKey = "r1|neoforge-21.1.250"
	a.instStatus = launcher.InstanceStatus{
		Dir: `D:\MC\.minecraft\versions\neoforge-21.1.250`,
		Sizes: map[string]int64{
			"mods":          512 << 20,
			"saves":         256 << 20,
			"resourcepacks": 128 << 20,
			"shaderpacks":   64 << 20,
		},
		Total: 1024 << 20,
	}

	tt.Frame()
	tt.Advance(300 * time.Millisecond)

	for _, tab := range []string{"发现", "衣橱", "任务", "实例", "发现", "实例"} {
		if err := tt.Click(tab); err != nil {
			t.Fatalf("click %s: %v", tab, err)
		}
		for range 15 {
			tt.Advance(20 * time.Millisecond)
		}
	}
}
