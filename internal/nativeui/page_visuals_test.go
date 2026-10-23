package nativeui

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestPageVisuals renders every page and the main dialogs for the
// round-2 audit (MCL_AUDIT2=1): zh/en, 1x, 2x and minimum size.
func TestPageVisuals(t *testing.T) {
	if os.Getenv("MCL_AUDIT2") == "" || !shotsOn() {
		t.Skip("screenshots only")
	}
	views := []string{"home", "mods", "discover", "wardrobe", "tasks", "settings", "dlg_version", "dlg_root", "dlg_account", "dlg_add", "dlg_import", "dlg_install", "toast_error", "names"}
	for _, lang := range []string{"zh-CN", "en-US"} {
		for _, sz := range []struct {
			n     string
			w, h  int
			scale float32
		}{{"1x", 1280, 800, 1}, {"2x", 1280, 800, 2}, {"min", 900, 540, 1}} {
			for _, v := range views {
				if sz.n != "1x" && v != "home" && v != "settings" && v != "dlg_version" && v != "names" {
					continue
				}
				a, tt := acctTester(t, lang, sz.w, sz.h)
				tt.SetScale(sz.scale)
				a.closeDialog()
				switch v {
				case "mods", "discover", "wardrobe", "tasks", "settings":
					a.goPage(v)
				case "dlg_version":
					a.openVersionConfig(a.snapshot.ActiveRoot, a.snapshot.Instances[0].SelectedVersion)
				case "dlg_root":
					a.openDialog("root")
				case "dlg_account":
					a.openDialog("account")
				case "dlg_add":
					a.openDialog("add")
				case "dlg_import":
					a.openDialog("import")
				case "dlg_install":
					a.openDialog("install")
				case "toast_error":
					a.message, a.errorMessage = "下载失败：连接超时 (resources.download.minecraft.net)", true
					a.noticeFor, a.noticeRetry = a.message, func() {}
				case "names":
					namesFixture(a)
				}
				tt.Frame()
				tt.Advance(700 * time.Millisecond)
				l := map[string]string{"zh-CN": "zh", "en-US": "en"}[lang]
				snapshotUI(t, tt, fmt.Sprintf("audit2_%s_%s_%s", v, l, sz.n))
			}
		}
	}
}
