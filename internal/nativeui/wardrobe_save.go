package nativeui

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

var badFileChars = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]+`)

// safePNGName turns a player or skin name into a plain file name.
func safePNGName(name, suffix string) string {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".png")
	name = strings.Trim(badFileChars.ReplaceAllString(name, "_"), " ._")
	if name == "" {
		name = "skin"
	}
	return name + suffix + ".png"
}

// dataURLBytes decodes the original texture bytes the appearance carries.
func dataURLBytes(data string) []byte {
	if data == "" {
		return nil
	}
	if i := strings.Index(data, ","); i >= 0 && strings.HasPrefix(data, "data:") {
		data = data[i+1:]
	}
	b, e := base64.StdEncoding.DecodeString(data)
	if e != nil {
		return nil
	}
	return b
}

// validTexture is true for a real PNG of a size Minecraft accepts.
func validTexture(b []byte, cape bool) bool {
	cfg, e := png.DecodeConfig(bytes.NewReader(b))
	if e != nil {
		return false
	}
	if cape {
		return cfg.Width >= 64 && cfg.Height >= 32
	}
	return cfg.Width == 64 && (cfg.Height == 64 || cfg.Height == 32)
}

// currentSkinBytes is the skin on the model now: a picked local file first,
// else the account's own texture.
func (a *App) currentSkinBytes() []byte {
	if w := a.wd(); len(w.pendingSkin) > 0 {
		return w.pendingSkin
	}
	if data := dataURLBytes(a.appearance.Skin); len(data) > 0 {
		return data
	}
	if a.preview != nil && a.preview.skin == builtinSkin(a.preview.slim) {
		model := "default"
		if a.preview.slim {
			model = "slim"
		}
		return launcher.VanillaSkinPNG(model)
	}
	return nil
}

// saveTexture asks where to save and writes the original bytes unchanged.
func (a *App) saveTexture(defName string, b []byte, cape bool) {
	w := a.wd()
	if len(b) == 0 || !validTexture(b, cape) {
		w.saveMsg, w.savePath, w.saveErr = a.L("wardrobe_save.nothing_to_save"), "", true
		if cape {
			w.saveMsg = a.L("wardrobe_save.nothing_to_save_2")
		}
		return
	}
	title := a.L("wardrobe.save_skin")
	if cape {
		title = a.L("wardrobe_save.save_cape")
	}
	data := append([]byte(nil), b...)
	if a.win == nil {
		return
	}
	go func() {
		p, e := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, Title: title, DefaultPath: defName, Filters: []mygo.FileFilter{{Name: "PNG", Extensions: []string{"png"}}}})
		if e != nil || p == "" {
			return // cancelled
		}
		if !strings.EqualFold(filepath.Ext(p), ".png") {
			p += ".png"
		}
		e = writeFileAtomic(p, data)
		a.post(func() {
			if e != nil {
				u := a.uiErr(e)
				w.saveMsg, w.savePath, w.saveErr = a.L("wardrobe_save.could_not_save")+u.Title, "", true
				return
			}
			w.saveMsg, w.savePath, w.saveErr = a.L("wardrobe_save.saved")+filepath.Base(p), p, false
		})
	}()
}

func writeFileAtomic(p string, b []byte) error {
	if len(b) == 0 {
		return errors.New("empty")
	}
	tmp := p + ".tmp"
	if e := os.WriteFile(tmp, b, 0o644); e != nil {
		return e
	}
	return os.Rename(tmp, p)
}

// saveNotice is the one-line result, with 打开文件夹 on success.
func (a *App) saveNotice(c *ui.Context) {
	w := a.wd()
	if w.saveMsg == "" {
		return
	}
	s := uiScale(c)
	ui.Row(c.Key("save-notice")).FillWidth().Height(30 * s).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		col := successInk
		icon := "check"
		if w.saveErr {
			col, icon = danger, "info"
		}
		ui.Icon(c, icons[icon]).Size(12*s, 12*s).TextColor(col).Shrink(0)
		tooltip(c, ui.Text(c, w.saveMsg).FontSize(typeFootnote*s).TextColor(inkSecondary).SingleLine().Ellipsis("…").Grow(1).MinWidth(0), w.savePath)
		if w.savePath != "" && fixedButton(c, "save-open-folder", a.L("addons_tab.open_folder"), "folder", 0, btnSm*s, false, false) {
			c.OpenURL("file:///" + filepath.ToSlash(filepath.Dir(w.savePath)))
		}
		if b := ui.ButtonBase(c.Key("save-notice-x")).Label(a.L("logsessions.close")).Size(22*s, 22*s).Radius(6 * s).Center(); b.Children(func() {
			ui.Icon(c, icons["cross"]).Size(9*s, 9*s).TextColor(muted)
		}).Clicked() {
			w.saveMsg, w.savePath = "", ""
		}
	})
}

// capeBytes is the original PNG of an owned cape, if already fetched.
func (a *App) capeBytes(name string) []byte {
	w := a.wd()
	for _, cp := range a.appearance.Capes {
		if cp.Name != name {
			continue
		}
		if b := w.capeRaw[cp.URL]; len(b) > 0 {
			return b
		}
		if cp.Active {
			return dataURLBytes(a.appearance.Cape)
		}
	}
	return nil
}
