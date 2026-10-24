package nativeui

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestDialogEasesInAndOut(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1536, 1024)
	a.closeDialog()
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	if _, ok := tt.Find("活动弹窗"); ok {
		t.Fatal("dialog should be gone once the exit finishes")
	}
	a.openDialog("account")
	tt.Frame()
	tt.Advance(60 * time.Millisecond)
	mid, ok := tt.Find("活动弹窗")
	if !ok {
		t.Fatal("dialog drawn while easing in")
	}
	tt.Advance(400 * time.Millisecond)
	end, _ := tt.Find("活动弹窗")
	if math.Abs(float64(mid.W-end.W)) > .01 || math.Abs(float64(mid.H-end.H)) > .01 || mid.Y <= end.Y || mid.Y-end.Y > 12 {
		t.Errorf("dialog should slide into place at its native size: mid %+v end %+v", mid, end)
	}
	// closing midway keeps drawing it (no pop), interrupting reopens smoothly
	a.closeDialog()
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	if _, ok := tt.Find("活动弹窗"); !ok {
		t.Error("dialog should ease out, not vanish")
	}
	a.openDialog("account")
	tt.Frame()
	tt.Advance(16 * time.Millisecond)
	back, ok := tt.Find("活动弹窗")
	if !ok || back.W != end.W {
		t.Errorf("reopening should continue from the current value: %+v", back)
	}
	a.closeDialog()
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	if _, ok := tt.Find("活动弹窗"); ok {
		t.Error("dialog still shown after exit")
	}
}

func TestPageSlideDirection(t *testing.T) {
	a, tt := homeTester(t, "zh-CN")
	a.goPage("discover")
	if a.pageDir != 1 {
		t.Errorf("forward nav slides from the right: %v", a.pageDir)
	}
	tt.Advance(400 * time.Millisecond)
	a.goPage("mods")
	if a.pageDir != -1 {
		t.Errorf("backward nav slides from the left: %v", a.pageDir)
	}
}

// Across window sizes and DPI scales: logos and avatars stay square, the
// content column is capped and centred, and the bottom bar keeps 设置 right
// before 启动游戏 inside the window.
func TestScalingKeepsProportions(t *testing.T) {
	for _, size := range [][2]int{{900, 540}, {1280, 800}, {1920, 1080}, {2560, 1440}} {
		for _, dpi := range []float32{1, 1.25, 1.5, 2} {
			t.Run(fmt.Sprintf("%dx%d@%g", size[0], size[1], dpi), func(t *testing.T) {
				a, tt := acctTester(t, "zh-CN", size[0], size[1])
				a.closeDialog()
				tt.SetScale(dpi)
				tt.Advance(400 * time.Millisecond)
				set, ok1 := tt.Find("打开文件夹")
				launch, ok2 := tt.Find("按钮文字 launch")
				if !ok1 || !ok2 || set.X+set.W > launch.X || launch.X+launch.W > float32(size[0]) || math.Abs(float64(set.Y+set.H/2-(launch.Y+launch.H/2))) > 2 {
					t.Errorf("folder must sit just before launch: %+v %+v", set, launch)
				}
				for _, name := range []string{"quilt logo", "头像 Steve"} {
					if r, ok := tt.Find(name); ok && math.Abs(float64(r.W-r.H)) > .5 {
						t.Errorf("%s distorted: %+v", name, r)
					}
				}
				if r, ok := tt.Find("主版本卡"); !ok || r.X+r.W > float32(size[0])+.5 {
					t.Errorf("hero should fit the window: %+v", r)
				}
			})
		}
	}
}
