package nativeui

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	addonCardMin   float32 = 300 // narrowest card before the grid drops a column
	addonCardH     float32 = 186
	addonCardGap   float32 = 12
	addonCardColsN         = 4
)

// addonGridCols is how many cards fit the measured width.
func addonGridCols(w, s float32) int {
	if w <= 0 {
		return 2
	}
	n := int((w + addonCardGap*s) / (addonCardMin*s + addonCardGap*s))
	return max(1, min(addonCardColsN, n))
}

// addonGrid lays entries out as equal-width cards, row by row. Long lists
// (mods) only build the rows near the viewport; the rest are spacers.
func (a *App) addonGrid(c *ui.Context, key string, shown []addonEntry, card func(addonEntry)) {
	s := uiScale(c)
	grid := ui.Column(c.Key(key)).Label("资源卡片").FillWidth().Gap(addonCardGap * s)
	b := grid.LayoutBounds()
	if a.addonGridW == nil {
		a.addonGridW = map[string]float32{}
	}
	w := b.W
	if w <= 0 {
		w = a.addonGridW[key]
	}
	if w <= 0 {
		w = a.tabViewBounds.W - 2*pageGutter*s
	}
	if b.W > 0 {
		a.addonGridW[key] = b.W
	}
	cols := addonGridCols(w, s)
	rows := (len(shown) + cols - 1) / cols
	rowH := (addonCardH + addonCardGap) * s
	first, last := 0, rows
	if key == "mod-grid" {
		a.modListBounds = b
		_, height := c.Size()
		first, last = addonGridWindow(rows, rowH, b, a.tabViewBounds, height)
	}
	grid.Children(func() {
		if first > 0 {
			ui.Box(c.Key(key + "-pad-top")).FillWidth().Height(float32(first)*rowH - addonCardGap*s)
		}
		for r := first; r < last; r++ {
			ui.Row(c.Key(fmt.Sprintf("%s-row-%d", key, r))).FillWidth().Height(addonCardH * s).Gap(addonCardGap * s).Children(func() {
				for i := r * cols; i < (r+1)*cols; i++ {
					if i < len(shown) {
						card(shown[i])
					} else {
						ui.Box(c).Grow(1).Basis(0).MinWidth(0)
					}
				}
			})
		}
		if last < rows {
			ui.Box(c.Key(key + "-pad-end")).FillWidth().Height(float32(rows-last)*rowH - addonCardGap*s)
		}
	})
}

// Grid rows are much taller and more expensive than list rows. Virtualize
// from the first layout, with two spare rows rather than 40 cold rows or
// 12 spare rows (up to 160 / 48 cards) from the list's window calculation.
func addonGridWindow(rows int, rowH float32, grid, view ui.Rect, height float32) (int, int) {
	if rows == 0 || rowH <= 0 {
		return 0, rows
	}
	const overscan = 2
	if view.H <= 0 {
		view.H = height
	}
	if grid.H <= 0 {
		return 0, min(rows, max(1, int(math.Ceil(float64(view.H/rowH))))+overscan)
	}
	top := max(float32(0), view.Y-grid.Y)
	first := min(rows, max(0, int(top/rowH)-overscan))
	last := min(rows, int(math.Ceil(float64((top+view.H)/rowH)))+overscan)
	return first, max(first, last)
}

// addonCard is one mod / pack card. The whole card is the toggle: hover
// tints all of it, a click requests a toggle (mods confirm before disabling),
// and an off card dims with a 已禁用 chip. Version, size and other facts
// are separate chips; the remove action floats at the bottom-right.
func (a *App) addonCard(c *ui.Context, it addonItem, onToggle func(), actions func()) {
	s := uiScale(c)
	primary, secondary := a.itemNames(it)
	ui.Box(c.Key("addon-card-" + it.key)).Label("资源卡片 " + primary).Grow(1).Basis(0).MinWidth(0).FillHeight().Children(func() {
		b := ui.ButtonBase(c.Key("toggle-"+it.key)).Label(a.L("切换启用 ", "Toggle ")+it.name).Description(it.file).Absolute().Left(0).Top(0).Right(0).Bottom(0).Radius(radiusCard*s).Clip().Padding(14*s, 14*s, 12*s).Gap(10 * s).Column()
		bg, edge := panel, ui.RGBA(60, 70, 110, .08)
		if b.Hovered() {
			bg, edge = ui.Hex("#f2f5fb"), ui.RGBA(60, 70, 110, .16)
		}
		if !it.enabled {
			bg = mixColor(bg, ui.Hex("#eef0f4"), .7)
		}
		b.Background(b.AnimateColor("bg", bg, fastAnim)).Border(1, b.AnimateColor("edge", edge, fastAnim))
		if b.Hovered() {
			// The file name shows under the card, kept inside the window.
			ui.TooltipBase(c, b, func(tip ui.Element) {
				tip.MaxWidth(460*s).Padding(5*s, 9*s).Radius(6*s).Background(ui.RGBA(28, 30, 38, .92)).Shadow(0, 2*s, 8*s, 0, ui.RGBA(0, 0, 0, .2))
				tip.Column().Gap(4 * s).Children(func() {
					if it.summary != "" {
						ui.Text(c, it.summary).FontSize(typeCaption * s).TextColor(ui.Hex("#ffffff")).FillWidth()
					}
					ui.Text(c, it.file).FontSize(typeCaption2 * s).TextColor(ui.RGBA(255, 255, 255, .62)).SingleLine()
				})
				tip.AttachTo(b, ui.AnchorBottom, ui.AnchorTop).Margin(6*s, 0, 0, 0)
			})
		}
		op := b.AnimateTo("op", map[bool]float32{true: 1, false: .45}[it.enabled], motionFast)
		if b.Children(func() {
			header := ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Opacity(op)
			if it.latest != "" {
				header.Padding(0, 16*s, 0, 0)
			}
			header.Children(func() {
				size := 40 * s
				icon := a.addonIcon(a.root().ID, a.root().SelectedVersion, it.kind, it.file, it.iconURL)
				ui.Box(c).Size(size, size).Shrink(0).Children(func() {
					if icon != nil {
						ui.Box(c).Size(size, size).Radius(size * .22).Clip().Background(panel).Center().Children(func() {
							ui.Image(c, icon).Size(size, size).Fit(ui.Contain)
						})
					} else {
						letterTile(c, primary+secondary, size, !it.enabled)
					}
				})
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
					ui.Text(c, primary).FillWidth().MinWidth(0).FontSize(typeBody * s).FontWeight(600).TextColor(ink).SingleLine()
					if sub := secondary; sub != "" {
						ui.Text(c, sub).FillWidth().MinWidth(0).FontSize(typeFootnote * s).TextColor(muted).SingleLine()
					}
				})
			})
			// Two-line summary in a fixed slot so every card has the same height.
			ui.Box(c).FillWidth().Height(36 * s).Shrink(0).Opacity(op).Children(func() {
				if it.summary != "" {
					ui.Text(c, it.summary).FillWidth().MinWidth(0).FontSize(typeFootnote * s).TextColor(inkSecondary).MaxLines(2).Ellipsis("…")
				}
			})
			// Category chips, then the facts.
			ui.Row(c).FillWidth().Height(18 * s).Shrink(0).Clip().Gap(6 * s).AlignItems(ui.Center).Opacity(op).Children(func() {
				shown := 0
				for _, cat := range it.cats {
					if shown >= 3 || isLoaderCategory(cat) {
						continue
					}
					lbl := a.translateCategory(cat)
					if !a.isEN() && strings.EqualFold(lbl, cat) {
						continue // no Chinese label: skip rather than show a raw slug
					}
					if a.isEN() {
						lbl = categoryTitle(cat)
					}
					if lbl != "" {
						ui.Box(c).Shrink(0).Children(func() { catTag(c, cat, lbl, false) })
						shown++
					}
				}
			})
			ui.Spacer(c).Grow(1)
			// Facts as chips; room is left on the right for the actions.
			ui.Row(c).FillWidth().Padding(0, 64*s, 0, 0).Clip().Gap(6 * s).AlignItems(ui.Center).Children(func() {
				if !it.enabled {
					quietTag(c, a.L("已禁用", "Disabled"), muted)
				}
				if logo := sourceLogo(it.source); logo != nil {
					ui.Row(c).Shrink(0).Height(18 * s).PaddingX(5 * s).Radius(5 * s).Background(ui.RGBA(120, 130, 160, .10)).Center().Children(func() {
						ui.Icon(c, logo).Size(11*s, 11*s).Label(sourceName(it.source))
					})
				}
				if it.version != "" {
					quietTag(c, it.version, inkSecondary)
				}
				if it.loader != "" {
					quietTag(c, it.loader, inkSecondary)
				}
				if it.size != "" {
					quietTag(c, it.size, inkSecondary)
				}
				if it.rank > 0 {
					quietTag(c, fmt.Sprintf("#%d", it.rank), inkSecondary)
				}
				if it.badge != "" {
					quietTag(c, it.badge, ui.Hex("#248a3d"))
				}
				if len(it.missing) > 0 {
					ui.Box(c).Shrink(0).Tooltip(a.L("缺少前置：", "Missing dependencies: ") + strings.Join(it.missing, ", ")).Children(func() { quietTag(c, a.L("缺少前置", "Missing deps"), danger) })
				}
			})
			if it.latest != "" {
				update := a.L("可更新 ", "Update ") + it.latest
				ui.Box(c.Key("addon-update-dot")).Absolute().Right(14*s).Top(14*s).Size(8*s, 8*s).Radius(999).Background(accent).Label(update).Tooltip(update)
			}
		}).Clicked() {
			onToggle()
		}
		if actions != nil {
			ui.Row(c).Absolute().Right(10 * s).Bottom(9 * s).Gap(2 * s).AlignItems(ui.Center).Children(actions)
		}
	})
}

// isLoaderCategory drops loader slugs from category chips: the instance
// already says which loader it runs.
func isLoaderCategory(cat string) bool {
	switch strings.ToLower(cat) {
	case "forge", "fabric", "neoforge", "quilt", "iris", "optifine", "canvas", "vanilla", "minecraft", "liteloader", "modloader", "rift", "bukkit", "paper", "spigot", "purpur", "folia", "velocity", "bungeecord", "waterfall", "sponge", "datapack":
		return true
	}
	return false
}

// categoryTitle turns a platform slug into a readable English label.
func categoryTitle(slug string) string {
	s := strings.TrimPrefix(strings.ToLower(slug), "mc-")
	words := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		if w == "and" {
			words[i] = "&"
			continue
		}
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
