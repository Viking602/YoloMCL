package nativeui

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

const instListWidth float32 = 340

func (a *App) instGroups(r launcher.Instance, names map[string]versionName) []struct {
	key, title string
	items      []launcher.VersionSummary
} {
	t := a.tr()
	f := strings.ToLower(strings.TrimSpace(a.instFilter))
	var packs, vanilla []launcher.VersionSummary
	for _, v := range r.Versions {
		if a.instLoader != "" && loaderKey(v.Loader) != a.instLoader {
			continue
		}
		if f != "" {
			name := names[v.ID]
			hay := strings.ToLower(name.Title + " " + name.Sub + " " + v.ID + " " + v.Minecraft)
			if !strings.Contains(hay, f) {
				continue
			}
		}
		if loaderKey(v.Loader) == "vanilla" {
			vanilla = append(vanilla, v)
		} else {
			packs = append(packs, v)
		}
	}
	return []struct {
		key, title string
		items      []launcher.VersionSummary
	}{{"packs", a.L("整合包", "Modpacks"), packs}, {"vanilla", t.LoaderVanilla, vanilla}}
}

func (a *App) instanceList(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	r := a.root()
	ui.Column(c.Key("instance-list")).Label("版本列表").Width(instListWidth * s).Shrink(0).FillHeight().Children(func() {
		ui.Row(c.Key("inst-filter-row")).Label("筛选栏").FillWidth().Height(btnMd*s+20*s).Padding(20*s, 14*s, 0, 16*s).Gap(8 * s).AlignItems(ui.Center).Shrink(0).Children(func() {
			textField(c.Key("inst-filter"), &a.instFilter).Placeholder(a.L("筛选实例", "Filter instances")).Label(a.L("筛选实例", "Filter instances")).Grow(1).MinWidth(0)
			if iconOnlyButton(c, "inst-import", "layers", t.HomeImportPackBtn, btnMd*s, false) {
				a.openDialog("import")
			}
			if iconOnlyButton(c, "inst-add", "plus", t.HomeInstallBtn, btnMd*s, false) {
				a.openDialog("install")
			}
		})
		if r != nil {
			a.loaderChips(c, *r)
		}
		a.instanceVirtualList(c, r)
		a.rootPicker(c)
	})
}

func (a *App) instRow(c *ui.Context, r launcher.Instance, v launcher.VersionSummary, n versionName) {
	s := uiScale(c)
	sel := v.ID == r.SelectedVersion
	b := ui.ButtonBase(c.Key("ver-row-"+v.ID)).Label("版本卡片 "+v.ID).FillWidth().Height(54*s).Padding(0, 12*s).Radius(radiusRow * s).Gap(12 * s).AlignItems(ui.Center).Clip().
		Transition(ui.ElementTransition{Animation: &motionMove, Position: true, Enter: &ui.Motion{Opacity: 0, Scale: .97}, Exit: &ui.Motion{Opacity: 0, Scale: .97}})
	bg, fg, sub := ui.Transparent, ink, muted
	if sel {
		bg, fg, sub = accent, panel, ui.RGBA(255, 255, 255, .78)
	} else if b.Hovered() {
		bg = ui.RGBA(255, 255, 255, .8)
	}
	b.Background(b.AnimateColor("bg", bg, fastAnim))
	b.Children(func() {
		vv := v
		a.instIcon(c, r, &vv, 34*s)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Clip().Gap(2 * s).Children(func() {
			title, subText := n.Title, n.Sub
			ui.Row(c).FillWidth().MinWidth(0).Gap(6 * s).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).Shrink(1).MinWidth(0).FontSize(typeBody * s).FontWeight(600).TextColor(fg).SingleLine().Tooltip(v.ID)
				if n.MCTag && v.Minecraft != "" {
					ui.Spacer(c).Grow(1)
					a.mcTag(c, v.Minecraft, sel)
				}
			})
			if subText == "" {
				subText = a.loaderLine(v)
			}
			ui.Row(c).Height(18 * s).MinWidth(0).Gap(5 * s).AlignItems(ui.Center).Tooltip(subText).Children(func() {
				ui.Box(c).Size(20*s, 18*s).Shrink(0).Radius(4 * s).Background(panel).Center().Children(func() {
					if !loaderLogo(c, v.Loader, 14*s) {
						ui.Icon(c, icons[loaderIconName(v.Loader)]).Size(12*s, 12*s).TextColor(inkSecondary)
					}
				})
				if version := loaderBuild(v); version != "" {
					ui.Text(c, version).Label("列表加载器版本 " + v.ID).FontSize(typeFootnote * s).TextColor(sub).NoWrap().Shrink(0)
				}
			})
		})
		if slices.Contains(r.RunningVersions, v.ID) {
			ui.Box(c).Size(8*s, 8*s).Radius(999).Background(ui.Hex("#34c759")).Shrink(0)
		} else if packHasUpdate(r, v.ID) {
			// a pack update is waiting: a small arrow, the word is on the page
			ui.Box(c).Size(16*s, 16*s).Radius(999).Background(ui.Hex("#ff9500").Alpha(.16)).Center().Shrink(0).Tooltip(a.tr().PackUpdateBadge).Children(func() {
				ui.Icon(c, icons["download"]).Size(9*s, 9*s).TextColor(warningInk)
			})
		}
	})
	if b.RightClicked() {
		if !sel {
			a.selectVersion(r.ID, v.ID)
		}
		a.setInstTab("settings")
	} else if b.Clicked() && !sel {
		a.selectVersion(r.ID, v.ID)
		if a.instTab == "settings" {
			a.loadVersionConfigForm(v.ID)
		}
	}
}

func (a *App) mcTag(c *ui.Context, mc string, onAccent bool) {
	s := uiScale(c)
	bg, fg := fieldColor, inkSecondary
	if onAccent {
		bg, fg = ui.RGBA(255, 255, 255, .22), panel
	}
	ui.Row(c).Shrink(0).Height(19 * s).PaddingX(7 * s).Radius(6 * s).Background(bg).Center().Children(func() {
		ui.Text(c, mc).FontSize(typeCaption * s).FontWeight(600).TextColor(fg).NoWrap()
	})
}

// rootPicker is the .minecraft directory dropdown at the list's foot.
func (a *App) rootPicker(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ui.Row(c.Key("root-picker-row")).Label("目录选择栏").FillWidth().Padding(12*s, 18*s, 16*s, 20*s).Gap(8 * s).AlignItems(ui.Center).Draw(func(p *ui.Painter, b ui.Rect) {
		p.Fill(ui.Rect{X: b.X, Y: b.Y, W: b.W, H: 1}, line, 0)
	}).Children(func() {
		if len(a.snapshot.Instances) > 0 {
			ids := make([]string, len(a.snapshot.Instances))
			names := map[string]string{}
			for i, r := range a.snapshot.Instances {
				ids[i] = r.ID
				names[r.ID] = dirLabel(r)
			}
			cur := a.snapshot.ActiveRoot
			if selectBoxFull(c, "root-picker", a.L("游戏目录", "Game directory"), &cur, ids, func(string) (string, ui.Color) { return "folder", accent }, func(id string) string { return names[id] }).Grow(1).MinWidth(0).Changed() && cur != a.snapshot.ActiveRoot {
				a.selectRoot(cur)
			}
			if iconOnlyButton(c, "root-settings", "settings", t.HomeDirSettingsBtn, btnMd*s, a.root() == nil) {
				a.openDialog("root")
			}
		}
		if len(a.snapshot.Instances) > 0 && iconOnlyButton(c, "add-root-btn", "folder-add", t.HomeAddFolderBtn, btnMd*s, false) {
			if a.form.Path == "" && !a.hasRootPath(launcher.DefaultGameDir()) {
				a.form.Path = launcher.DefaultGameDir()
			}
			a.openDialog("add")
		}
	})
}

func (a *App) hasRootPath(p string) bool {
	for _, r := range a.snapshot.Instances {
		if p == "" || strings.EqualFold(filepath.Clean(r.Path), filepath.Clean(p)) {
			return true
		}
	}
	return p == ""
}

// dirLabel is a directory's display name: its label, or the folder name
// when the label is empty or just "MC".
func dirLabel(r launcher.Instance) string {
	n := strings.TrimSpace(r.Name)
	if n == "" || strings.EqualFold(n, "mc") || strings.EqualFold(n, "minecraft") {
		n = filepath.Base(filepath.Clean(r.Path))
		if strings.EqualFold(n, ".minecraft") {
			if p := filepath.Base(filepath.Dir(filepath.Clean(r.Path))); p != "" && p != "." && p != string(filepath.Separator) && !strings.EqualFold(p, "mc") {
				n = p + string(filepath.Separator) + ".minecraft"
			}
		}
	}
	return n
}

// loaderChips filters the list by loader; only loaders present show, and
// the row hides when the directory has a single kind.
func (a *App) loaderChips(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	order := []string{"fabric", "forge", "neoforge", "quilt", "vanilla"}
	have := map[string]bool{}
	for _, v := range r.Versions {
		have[loaderKey(v.Loader)] = true
	}
	var keys []string
	for _, k := range order {
		if have[k] {
			keys = append(keys, k)
		}
	}
	if a.instLoader != "" && !have[a.instLoader] {
		a.instLoader = ""
	}
	if len(keys) < 2 {
		return
	}
	// A compact dropdown keeps the filter on one line whatever loaders the
	// directory holds (a five-way segmented control truncates at this width).
	ui.Row(c.Key("loader-filter")).Label("加载器筛选").FillWidth().Padding(10*s, 16*s, 2*s, 16*s).Shrink(0).Children(func() {
		cur := a.instLoader
		opts := append([]string{""}, keys...)
		icon := func(k string) (string, ui.Color) {
			if k == "" {
				return "layers", accent
			}
			return loaderIconName(k), loaderTone(k)
		}
		show := func(k string) string {
			switch k {
			case "":
				return a.L("全部加载器", "All loaders")
			case "vanilla":
				return a.tr().LoaderVanilla
			}
			return a.loaderDisplay(k)
		}
		if selectBoxFull(c, "loader-select", a.L("按加载器筛选", "Filter by loader"), &cur, opts, icon, show).Grow(1).MinWidth(0).Changed() && cur != a.instLoader {
			a.instLoader = cur
			a.settings.InstanceLoaderFilter = cur
			a.instScrollPending = true
			draft := a.settings
			go func() { _ = a.service.SaveSettings(draft) }()
		}
	})
}
