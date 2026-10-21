package nativeui

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mcl/internal/launcher"
)

// Exercise the exact account-dialog/wardrobe combination in the reported
// screenshot. Rectangles are DIPs even when the renderer uses a different DPI.
func TestSingleLineControlsAcrossResizeAndDPI(t *testing.T) {
	for _, size := range [][2]int{{900, 540}, {1060, 720}, {1476, 905}, {1920, 1080}, {2560, 1440}} {
		for _, dpi := range []float32{1, 1.25, 1.5, 2} {
			if size[0] > 1920 && dpi != 1 {
				continue
			}
			t.Run(fmt.Sprintf("%dx%d@%g", size[0], size[1], dpi), func(t *testing.T) {
				a, tt := settingsTester(t)
				a.page = "wardrobe"
				a.openDialog("account")
				tt.SetSize(size[0], size[1])
				tt.SetScale(dpi)
				tt.Advance(400 * time.Millisecond) // let the dialog finish easing in
				s := max(.65, min(float32(1), float32(size[0])/1536, float32(size[1]-42)/982))
				for _, name := range []string{"按钮文字 refresh-skin", "按钮文字 close-dialog"} {
					r, ok := tt.Find(name)
					if !ok || r.W < 30*s || r.H > 29*s || r.X < 0 || r.Y < 42 || r.X+r.W > float32(size[0]) || r.Y+r.H > float32(size[1]) {
						t.Errorf("text wrapped or escaped window: %s = %+v", name, r)
					}
				}
				for _, name := range []string{"刷新", "关闭"} {
					r, ok := tt.Find(name)
					if !ok || math.Abs(float64(r.H-40*s)) > 1 {
						t.Errorf("button height changed with wrapped text: %s = %+v", name, r)
					}
				}
				modal, ok := tt.Find("活动弹窗")
				if !ok || modal.X < 0 || modal.Y < 42 || modal.X+modal.W > float32(size[0]) || modal.Y+modal.H > float32(size[1]) {
					t.Errorf("dialog escaped the available window: %+v", modal)
				}
				if size[0] == 1476 && dpi == 1 {
					snapshotUI(t, tt, "text-layout-fixed")
				}
			})
		}
	}
}

func TestOfficialVersionPickerIncludesHistoricalAndFutureCategories(t *testing.T) {
	a, tt := settingsTester(t)
	a.dialog, a.dialogOpen = "install", true
	a.form = formState{Loader: "原版", Target: "新建独立目录"}
	a.versionKind = "全部版本"
	a.minecraftVersions = []launcher.MinecraftVersion{
		{ID: "1.21.1", Type: "release", ReleaseTime: "2024-08-08T00:00:00Z"},
		{ID: "24w14potato", Type: "snapshot"},
		{ID: "b1.7.3", Type: "old_beta"},
		{ID: "rd-132211", Type: "old_alpha"},
		{ID: "new-official-category", Type: "future_category"},
	}
	if got := filterMinecraftVersions(a.minecraftVersions, "全部版本", ""); len(got) != 5 {
		t.Fatal("all versions dropped a catalogue category")
	}
	a.applyMinecraftFilter()
	tt.Frame()
	clickUI(t, tt, "Minecraft 版本分类")
	clickUI(t, tt, "历史 Alpha")
	if a.form.Minecraft != "rd-132211" {
		t.Fatalf("historical Alpha unavailable: %q", a.form.Minecraft)
	}
	clickUI(t, tt, "Minecraft 版本分类")
	clickUI(t, tt, "全部版本")
	typeUI(t, tt, "搜索 Minecraft 版本", "b1.7")
	if a.form.Minecraft != "b1.7.3" {
		t.Fatalf("historical Beta not searchable: %q", a.form.Minecraft)
	}
	snapshotUI(t, tt, "official-version-picker")
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Type("no-matching-version")
	if a.form.Minecraft != "" || !tt.HasText("没有匹配的版本") {
		t.Fatal("empty search retained an unrelated Minecraft version")
	}
}

func TestSmallWindowPagesAndSettingsScroll(t *testing.T) {
	a, tt := settingsTester(t)
	tt.SetSize(900, 540)
	for _, page := range []string{"home", "discover", "wardrobe", "settings"} {
		a.page = page
		tt.Frame()
		if page == "discover" {
			for _, name := range []string{"搜索资源", "Minecraft 版本筛选", "加载器筛选"} {
				r, ok := tt.Find(name)
				nav, _ := tt.Find("主导航")
				overlapsNav := r.X < nav.X+nav.W && r.X+r.W > nav.X && r.Y < nav.Y+nav.H && r.Y+r.H > nav.Y
				if !ok || r.W <= 0 || overlapsNav || r.X+r.W > 900 || r.Y+r.H > 540 {
					t.Errorf("resource control escaped the small window: %s = %+v", name, r)
				}
			}
		}
		if page == "settings" {
			tt.Scroll(700, 400, 0, 600)
			r, ok := tt.Find("恢复默认")
			if !ok || r.Y < 42 || r.Y+r.H > 540 {
				t.Errorf("settings cannot scroll to the action bar: %+v", r)
			}
		}
		snapshotUI(t, tt, "small-"+page)
	}
}
