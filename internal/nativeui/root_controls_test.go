package nativeui

import (
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestDirectoryJavaDropdownAndMemorySlider(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	a.settings.Language = LangZH
	a.snapshot.ActiveRoot = "r"
	a.snapshot.Instances = []launcher.Instance{{Root: launcher.Root{ID: "r", Path: `D:\Minecraft`, Name: "Minecraft"}}}
	a.form.Memory = "4096"
	a.memoryInfo.MaxMB = 16384
	a.java = []launcher.JavaInfo{
		{Path: `D:\Java21\bin\java.exe`, Version: "21.0.10", Vendor: "Adoptium", Major: 21},
		{Path: `D:\Java17\bin\java.exe`, Version: "17.0.15", Vendor: "Adoptium", Major: 17},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Padding(24).Background(panel).Children(func() { a.rootDialog(c) })
	}, 800, 460)
	if tt.HasText(a.tr().RootCfgMemLabel + "输入") {
		t.Fatal("directory memory still uses the old text field")
	}
	slider, ok := tt.Find("目录内存滑块")
	if !ok || slider.W < 200 {
		t.Fatal("directory memory slider is missing")
	}
	tt.ClickAt(slider.X+slider.W*.5, slider.Y+slider.H/2)
	n, err := strconv.Atoi(a.form.Memory)
	if err != nil || n <= 4096 || n > 16384 || n%256 != 0 {
		t.Fatalf("slider did not update a bounded 256 MB increment: %q", a.form.Memory)
	}
	clickUI(t, tt, a.tr().RootCfgJavaLabel)
	clickUI(t, tt, "Java 21.0.10  Adoptium")
	if a.form.Java != a.java[0].Path {
		t.Fatal("Java dropdown saved its label instead of its executable path")
	}
	snapshotUI(t, tt, "directory-controls")
	clickUI(t, tt, a.tr().RootCfgJavaLabel)
	clickUI(t, tt, "跟随全局 Java 设置")
	if a.form.Java != "" {
		t.Fatal("following global Java did not clear the override")
	}
	a.form.MemAuto = true
	tt.Frame()
	if tt.HasText("目录内存滑块") {
		t.Fatal("automatic memory still accepts a manual slider value")
	}
}
