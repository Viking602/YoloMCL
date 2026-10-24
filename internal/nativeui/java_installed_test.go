package nativeui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func javaInstalledFixture(t *testing.T, lang string, width, height int, managed bool) (*App, *ui.Tester) {
	t.Helper()
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	delete(scrollTracks, "java-list")
	a := &App{javaScannedOnce: true}
	a.OverrideLanguage(lang)
	for i, major := range []int{26, 25, 21, 21} {
		home, vendor := fmt.Sprintf(`C:\Program Files\Zulu\zulu-%d`, major), "Zulu"
		if i == 2 {
			home, vendor = `C:\Program Files\Eclipse Adoptium\jdk-21.0.10.7-hotspot`, "Temurin"
		}
		if managed && i == 2 {
			home = `C:\Users\Player\AppData\Roaming\YoloMCL\` + strings.Repeat(`long-runtime-folder\`, 5) + `runtime\custom-jdk-21`
			vendor = "Very Long Community OpenJDK Vendor Name"
		}
		j := launcher.InstalledJava{
			JavaInfo: launcher.JavaInfo{Path: home + `\bin\java.exe`, Version: fmt.Sprintf("%d.0.10", major), Major: major, Vendor: vendor},
			Home:     home, Size: int64(388-i*18) * 1024 * 1024, Managed: managed && i == 2,
		}
		a.java = append(a.java, j.JavaInfo)
		a.javaGet.list = append(a.javaGet.list, j)
	}
	a.snapshot.Instances = []launcher.Instance{
		{Versions: []launcher.VersionSummary{{JavaPath: a.java[1].Path}, {JavaPath: a.java[2].Path}}},
		{Versions: []launcher.VersionSummary{{JavaPath: a.java[1].Path}}},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		s := uiScale(c)
		ui.Column(c).Fill().Padding(32*s, 24*s).Background(bg).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(settingsColumnWidth * s).AlignSelf(ui.Center).Children(func() { a.javaSection(c) })
		})
	}, width, height)
	tt.Advance(500 * time.Millisecond)
	return a, tt
}

func TestJavaInstalledResponsiveControls(t *testing.T) {
	for _, lang := range []string{LangZH, LangEN} {
		for _, size := range [][2]int{{1536, 1024}, {900, 600}} {
			for _, managed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s-%d-managed-%t", lang, size[0], managed), func(t *testing.T) {
					a, tt := javaInstalledFixture(t, lang, size[0], size[1], managed)
					list, _ := tt.Find("Java 环境列表")
					for _, j := range a.javaGet.list {
						row, ok := tt.Find("Java 安装项 " + j.Path)
						if !ok || row.X < 0 || row.Y < 0 || row.X+row.W > float32(size[0]) || row.Y+row.H > min(float32(size[1]), list.Y+list.H) {
							t.Fatalf("runtime row missing or outside window: %+v", row)
						}
						path, _ := tt.Find("Java 安装路径 " + j.Path)
						usage, _ := tt.Find("Java 使用情况 " + j.Path)
						actions, ok := tt.Find("Java 操作 " + j.Path)
						if !ok || actions.X+actions.W > row.X+row.W || path.W < 120 || path.X+path.W > usage.X || usage.X+usage.W > actions.X || actions.Y < row.Y || actions.Y+actions.H > row.Y+row.H {
							t.Fatalf("runtime metadata overlaps actions: path=%+v usage=%+v actions=%+v row=%+v", path, usage, actions, row)
						}
					}
					snapshotUI(t, tt, fmt.Sprintf("java-runtimes-%s-%d-managed-%t", lang, size[0], managed))
					if tt.HasText(a.L("卸载", "Uninstall")) != managed {
						t.Fatal("uninstall must only be offered for launcher-managed Java")
					}
					if managed {
						clickUI(t, tt, a.L("卸载", "Uninstall"))
						if !tt.HasText(a.L("确认卸载", "Confirm")) || len(a.javaGet.list) != 4 {
							t.Fatal("uninstall no longer requires confirmation")
						}
					}
				})
			}
		}
	}
}

func TestJavaInstalledSettingsScroll(t *testing.T) {
	a, tt := settingsTester(t)
	fixture, _ := javaInstalledFixture(t, LangZH, 1536, 1024, false)
	a.java, a.javaGet = fixture.java, fixture.javaGet
	a.javaScannedOnce = true
	a.javaGet.listRevision = a.javaScanRevision
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	tt.Scroll(800, 600, 0, 10000)
	tt.Frame()
	footer, ok := tt.Find("关于 YoloMCL 面板")
	if !ok || footer.Y < 42 || footer.Y+footer.H > 1024 {
		t.Fatalf("installed runtimes made the settings footer unreachable: %+v", footer)
	}
	snapshotUI(t, tt, "java-runtimes-settings")

	a.javaGet.list = []launcher.InstalledJava{}
	tt.Frame()
	tt.Scroll(800, 600, 0, 10000)
	tt.Advance(500 * time.Millisecond)
	snapshotUI(t, tt, "java-runtimes-empty")
	a.javaScanning = true
	tt.Frame()
	snapshotUI(t, tt, "java-runtimes-scanning")
}
