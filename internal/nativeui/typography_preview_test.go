package nativeui

import (
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Capture the real install dialog and discovery page, with identical fixture
// data before and after a typography change. No alternate rendering path.
func TestTypographyProductPages(t *testing.T) {
	if os.Getenv("MCL_NATIVE_QA_OUTPUT") == "" {
		t.Skip("set MCL_NATIVE_QA_OUTPUT to capture typography previews")
	}
	a, tt, _ := projectDialogFixture(t, 4)
	a.project.Title = "All the Mods 11 — Explore, build and discover a new Minecraft adventure with friends"
	a.project.Description = "Explore Minecraft with friends. Install the pack and choose a Java runtime. The download requires 512 MB."
	a.dialog, a.dialogOpen = "project", true
	tt.Frame()
	tt.Advance(time.Second)
	snapshotUI(t, tt, "install-en")
	if r, ok := tt.Find(a.project.Title); ok {
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Advance(650 * time.Millisecond)
		snapshotUI(t, tt, "install-title-hover")
	}
	tt.Move(0, 0)
	a.project.Description = "为 Minecraft 1.21.1 管理模组与资源包，安装前检查 Java 运行环境。下载大小为 512 MB，完成后可在启动器中选择游戏版本并启动。"
	tt.Frame()
	snapshotUI(t, tt, "install-zh")

	a, tt = discoverMotionTester(t)
	ps := discoverMotionProjects(0, 3)
	ps[0].Title = "All the Mods 11 — Explore Minecraft with friends and discover a new adventure"
	ps[0].Description = "Explore new worlds with friends. Download the resource pack: 512 MB."
	ps[1].Title = "探索新的世界，与朋友一起构建属于自己的 Minecraft 冒险旅程"
	ps[1].Description = "为 Minecraft 1.21.1 管理模组与资源包，安装前检查 Java 运行环境，下载大小为 512 MB。"
	ps[2].Title = "Java Edition"
	ps[2].Description = "A Minecraft resource pack."
	ps[0].Downloads = 111111
	showDiscoverResults(a, tt, launcher.SearchQuery{Source: "modrinth", Type: "modpack", Limit: 18}, ps)
	tt.Advance(time.Second)
	snapshotUI(t, tt, "discover-111111")
	if r, ok := tt.Find(ps[0].Title); ok {
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Advance(650 * time.Millisecond)
		snapshotUI(t, tt, "discover-truncated-hover")
	}
	tt.Move(0, 0)
	a.searchResult.Projects[0].Downloads = 888888
	a.discoverMotion.cardProjects = nil
	tt.Frame()
	tt.Advance(time.Second)
	snapshotUI(t, tt, "discover-888888")
}

func TestTypographyNativeTextFallbacks(t *testing.T) {
	if os.Getenv("MCL_NATIVE_QA_OUTPUT") == "" {
		t.Skip("set MCL_NATIVE_QA_OUTPUT to capture typography previews")
	}
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	description := ""
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		s := uiScale(c)
		if description == "" {
			// Find a real paragraph end that puts only MB on its final line in
			// the existing UI font. The same fixture is used for both builds.
			font := ui.Font{Family: uiSansFamily(), Size: typeFootnote * s}
			for n := 1; n < 80; n++ {
				candidate := strings.Repeat("Minecraft ", n) + "resource pack requires 512 MB"
				lines := ui.ShapeText(candidate, font, 520*s).Lines()
				last := lines[len(lines)-1]
				units := utf16.Encode([]rune(candidate))
				if strings.TrimSpace(string(utf16.Decode(units[last.Start:last.End]))) == "MB" {
					description = candidate
					break
				}
			}
			if description == "" {
				t.Fatal("could not construct paragraph-end preview")
			}
		}
		ui.Column(c).Fill().Padding(30).Background(bg).Lang(LangZH).Children(func() {
			emptyState(c, "typography-empty", "search", "Choose a Minecraft version and a compatible Java runtime to start your next adventure with friends", description, nil)
		})
	}, 1280, 860)
	tt.Advance(time.Second)
	snapshotUI(t, tt, "wrapped-heading-paragraph")
}
