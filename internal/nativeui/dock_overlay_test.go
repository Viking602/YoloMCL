package nativeui

import (
	"fmt"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// The split view scrolls its version list on its own: the hero (with
// 启动游戏) and the directory picker stay put while the list moves.
func TestSplitListScrollsHeroStays(t *testing.T) {
	for _, size := range [][2]int{{900, 540}, {1280, 800}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a, tt := p2Fixture(t, size[0], size[1], 1)
			namesFixture(a)
			root := &a.snapshot.Instances[len(a.snapshot.Instances)-1]
			for i := 0; i < 30; i++ {
				root.Versions = append(root.Versions, launcher.VersionSummary{ID: fmt.Sprintf("overlay-fixture-%02d", i), Minecraft: "26.1.2", Loader: "neoforge", LoaderVersion: "26.1.2.114"})
			}
			settle(tt)
			if _, ok := tt.Find("启动栏"); ok {
				t.Fatal("floating dock should be gone")
			}
			hero, ok := tt.Find("主版本卡")
			picker, ok2 := tt.Find("目录选择栏")
			if !ok || !ok2 {
				t.Fatal("hero / directory picker missing")
			}
			track := scrollTracks["inst-list-scroll"]
			if track == nil || track.st.MaxY < 160 {
				t.Fatal("fixture needs a scrollable version list")
			}
			track.st.Y = 0
			tt.Frame()
			before := track.st.Y
			tt.Scroll(60, float32(size[1])/2, 0, 120)
			tt.Advance(300 * time.Millisecond)
			if track.st.Y <= before {
				t.Error("version list did not scroll")
			}
			hero2, _ := tt.Find("主版本卡")
			picker2, _ := tt.Find("目录选择栏")
			if hero2.Y != hero.Y || picker2.Y != picker.Y {
				t.Errorf("hero/picker moved with the list: %+v→%+v %+v→%+v", hero, hero2, picker, picker2)
			}
			snapshotUI(t, tt, fmt.Sprintf("split-scroll-%dx%d", size[0], size[1]))
		})
	}
}
