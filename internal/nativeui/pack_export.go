package nativeui

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type packExportState struct {
	rootID, version, format   string
	name, packVersion, author string
	preview                   *launcher.PackExportPreview
	entries                   []launcher.PackExportEntry
	rows                      ui.ListState
	saving                    bool
}

func (a *App) packExportSection(c *ui.Context, r launcher.Instance) {
	version := a.form.ConfigVersion
	if version == "" {
		version = r.SelectedVersion
	}
	if version == "" {
		return
	}
	s := uiScale(c)
	ui.Row(c.Key("pack-export-section")).Label("导出整合包设置").FillWidth().Wrap().Gap(12*s).Padding(16*s).Radius(12*s).Background(raised).Border(1, line).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Basis(240 * s).MinWidth(0).Gap(4 * s).Children(func() {
			ui.Text(c, a.L("导出整合包", "Export modpack")).FontSize(typeSubhead * s).Bold().TextColor(ink)
			label(c, a.L("保存为 Modrinth 或 CurseForge 格式，分享当前实例。", "Share this instance as a Modrinth or CurseForge pack.")).FontSize(typeFootnote * s).FillWidth().MinWidth(0)
		})
		if button(c, "pack-export", a.L("导出…", "Export…"), "download", false, r.Running || a.instancePending("exportPack")) {
			a.openPackExport(r.ID, version)
		}
	})
}

func (a *App) openPackExport(rootID, version string) {
	// Export has its own form; keep unsaved instance settings underneath it.
	previousForm := a.form
	a.openDialog("export")
	a.form = previousForm
	g := &packExportState{rootID: rootID, version: version, format: "modrinth"}
	a.packExport = g
	a.execute("previewPackExport", map[string]string{"rootId": rootID, "version": version}, func(b json.RawMessage) {
		preview := decode[launcher.PackExportPreview](b)
		g.preview, g.entries = &preview, slices.Clone(preview.Entries)
		g.name, g.packVersion = preview.Name, preview.PackVersion
	})
}

func (a *App) packExportDialog(c *ui.Context) {
	g := a.packExport
	if g == nil {
		return
	}
	s := uiScale(c)
	if g.preview == nil {
		ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
			if a.dialogPending() {
				statusSpinner(c, 14*s)
				label(c, a.L("正在读取实例内容…", "Reading instance contents…"))
			} else if button(c, "export-retry-preview", a.L("重新读取", "Retry"), "refresh", false, false) {
				a.openPackExport(g.rootID, g.version)
			}
		})
		return
	}
	preview := g.preview
	ui.Row(c).FillWidth().Gap(8 * s).Wrap().Children(func() {
		badge(c, "Minecraft "+preview.Minecraft, accent)
		if preview.Loader != "vanilla" {
			quietTag(c, a.loaderDisplay(preview.Loader)+" "+preview.LoaderVersion, muted)
		}
	})
	ui.Row(c).FillWidth().Gap(12 * s).Children(func() {
		for _, format := range []string{"modrinth", "curseforge"} {
			name, ext, logo := "Modrinth", ".mrpack", modrinthLogo
			if format == "curseforge" {
				name, ext, logo = "CurseForge", ".zip", curseforgeLogo
			}
			on := g.format == format
			b := ui.ButtonBase(c.Key("export-format-"+format)).Label(name).Grow(1).Basis(0).MinWidth(0).Height(58*s).Radius(12*s).Padding(12*s).Row().Gap(10*s).AlignItems(ui.Center).Background(raised).Border(1, line).Disabled(g.saving)
			if on {
				b.Background(accentSoft).Border(1.5, accent)
			}
			if b.Children(func() {
				ui.Image(c, logo).Size(24*s, 24*s).Fit(ui.Contain).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2 * s).Children(func() {
					ui.Text(c, name).FontSize(typeBody * s).FontWeight(650).TextColor(ink).NoWrap()
					label(c, ext).FontSize(typeCaption * s)
				})
				if on {
					ui.Icon(c, icons["check"]).Size(13*s, 13*s).TextColor(accent).Shrink(0)
				}
			}).Clicked() {
				g.format = format
			}
		}
	})
	ui.Row(c).FillWidth().Gap(12 * s).Children(func() {
		ui.Column(c).Grow(2).Basis(0).MinWidth(0).Children(func() { field(c, a.L("名称", "Name"), &g.name, "My Modpack") })
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() { field(c, a.L("整合包版本", "Pack version"), &g.packVersion, "1.0.0") })
	})
	if g.format == "curseforge" {
		field(c, a.L("作者（可选）", "Author (optional)"), &g.author, "")
	}
	ui.Column(c).FillWidth().Gap(8 * s).Children(func() {
		ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			label(c, a.L("包含的内容", "Contents to include")).FontWeight(600)
			ui.Spacer(c).Grow(1)
			if fixedButton(c, "export-defaults", a.L("恢复默认", "Reset selection"), "refresh", 0, btnSm*s, false, g.saving) {
				for i := range g.entries {
					g.entries[i].Selected = preview.Entries[i].Selected
				}
			}
		})
		if len(g.entries) == 0 {
			label(c, a.L("此实例没有附加文件，将导出游戏与加载器信息。", "No extra files; the export will contain the game and loader manifest.")).FontSize(typeFootnote * s)
		} else {
			ui.List(c.Key("pack-export-contents"), &g.rows, len(g.entries)).ItemKey(func(i int) any { return g.entries[i].Name }).FillWidth().Height(min(160, float32(len(g.entries))*34) * s).Background(raised).Radius(10 * s).Rows(func(row ui.ListRow) {
				c, entry := row.Context, &g.entries[row.Index]
				on := entry.Selected
				b := ui.ButtonBase(c.Key(entry.Name)).Label("导出内容 " + entry.Name).FillWidth().Height(34 * s).Row().Gap(10 * s).PaddingX(10 * s).AlignItems(ui.Center).Disabled(g.saving)
				if b.Children(func() {
					box := ui.Box(c).Size(17*s, 17*s).Radius(5*s).Border(1, line).Background(panel).Shrink(0).Center()
					if on {
						box.Background(accent).Border(1, accent)
					}
					box.Children(func() {
						if on {
							ui.Icon(c, icons["check"]).Size(10*s, 10*s).TextColor(panel)
						}
					})
					icon := "doc"
					if entry.Folder {
						icon = "folder"
					}
					ui.Icon(c, icons[icon]).Size(14*s, 14*s).TextColor(muted).Shrink(0)
					tooltip(c, ui.Text(c, entry.Name).FontSize(typeFootnote*s).TextColor(ink).Grow(1).Basis(0).MinWidth(0).SingleLine().Ellipsis("…"), entry.Name)
				}).Clicked() {
					entry.Selected = !on
				}
			})
		}
	})
	label(c, a.L("使用平台文件引用；未识别的文件随包打包。默认不选存档和个人选项，不包含启动器账号、运行环境与缓存。", "Uses provider file references; unidentified files are bundled. Worlds and personal options start unselected. Launcher accounts, runtimes and caches are excluded.")).FontSize(typeCaption * s).FillWidth().MinWidth(0)
}

func (a *App) packExportFooter(c *ui.Context) {
	g := a.packExport
	if g == nil || g.preview == nil {
		return
	}
	s := uiScale(c)
	ui.Box(c).FillWidth().Height(1).Background(line)
	ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
		n := 0
		for _, entry := range g.entries {
			if entry.Selected {
				n++
			}
		}
		label(c, fmt.Sprintf(a.L("已选 %d 项", "%d selected"), n)).FontSize(typeFootnote * s).Grow(1)
		if button(c, "export-save", a.L("选择位置并导出", "Save and export"), "download", true, g.saving || a.dialogPending() || strings.TrimSpace(g.name) == "" || strings.TrimSpace(g.packVersion) == "") {
			a.savePackExport(g)
		}
	})
}

var savePackDialog = mygo.Dialog.Save

func (a *App) savePackExport(g *packExportState) {
	entries := make([]string, 0, len(g.entries))
	for _, entry := range g.entries {
		if entry.Selected {
			entries = append(entries, entry.Name)
		}
	}
	// Capture all values before invoking the native dialog. Navigation, late
	// catalog responses or a second export cannot change this request.
	payload := map[string]any{"rootId": g.rootID, "version": g.version, "format": g.format, "name": g.name, "packVersion": g.packVersion, "author": g.author, "entries": entries}
	ext, name := ".mrpack", "Modrinth"
	if g.format == "curseforge" {
		ext, name = ".zip", "CurseForge"
	}
	base := strings.Trim(badFileChars.ReplaceAllString(g.name+"-"+g.packVersion, "_"), " ._")
	if base == "" {
		base = "modpack"
	}
	opts := mygo.SaveDialogOptions{Parent: a.win, Title: a.L("导出整合包", "Export modpack"), DefaultPath: base + ext, Filters: []mygo.FileFilter{{Name: name, Extensions: []string{strings.TrimPrefix(ext, ".")}}}}
	gen := a.dialogGen
	g.saving = true
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		path, err := savePackDialog(opts)
		a.post(func() {
			g.saving = false
			if !a.dialogOpen || a.dialog != "export" || a.dialogGen != gen || a.packExport != g {
				return
			}
			if err != nil {
				a.message, a.errorMessage = a.friendlyErr(err), true
				return
			}
			if path == "" {
				return
			}
			// Only the exact filename returned by Save received its native
			// overwrite confirmation. Appending an extension must not inherit it.
			overwrite := strings.EqualFold(filepath.Ext(path), ext)
			if filepath.Ext(path) == "" {
				path += ext
			}
			payload["path"], payload["overwrite"] = path, overwrite
			a.execute("exportPack", payload, a.taskAdded)
		})
	}()
}
