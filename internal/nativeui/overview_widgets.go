package nativeui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// ovRegion is a dashboard panel with a title and optional trailing actions.
func ovRegion(c *ui.Context, key, title string, actions, fn func()) {
	s := uiScale(c)
	ui.Column(c.Key(key)).Label(title).Grow(1).Basis(0).MinWidth(0).MinHeight(0).Clip().Padding(16*s, 20*s).Gap(4*s).Radius(radiusCard*s).Background(ui.RGBA(255, 255, 255, .76)).Border(1, glassEdge).Shadow(0, 1*s, 3*s, 0, ui.RGBA(40, 52, 110, .05)).Children(func() {
		ui.Row(c).FillWidth().Height(30 * s).Shrink(0).Gap(6 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(typeHeadline * s).FontWeight(700).TextColor(ink).SingleLine().Grow(1).MinWidth(0).Tooltip(title)
			if actions != nil {
				actions()
			}
		})
		fn()
	})
}

func durText(a *App, sec int64) string {
	h, m := sec/3600, sec%3600/60
	switch {
	case h > 0:
		return fmt.Sprintf(a.L("%d 小时 %d 分", "%dh %dm"), h, m)
	case m > 0:
		return fmt.Sprintf(a.L("%d 分钟", "%d min"), m)
	}
	return fmt.Sprintf(a.L("%d 秒", "%ds"), sec)
}

// kvRow is a kv row whose value is custom content.
func kvRow(c *ui.Context, k string, value func()) {
	s := uiScale(c)
	ui.Row(c).FillWidth().Height(28 * s).Shrink(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(kvLabelW * s).Shrink(0).Clip().Children(func() { label(c, k).FontSize(typeCallout * s).NoWrap() })
		ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(8 * s).AlignItems(ui.Center).Children(value)
	})
}

func valueText(c *ui.Context, v string) {
	ui.Text(c, v).FontSize(typeCallout * uiScale(c)).FontWeight(600).TextColor(ink).NoWrap()
}

// instanceScopeNote says the tab only affects this instance and points to
// the global page for account and defaults.
func (a *App) instanceScopeNote(c *ui.Context) {
	s := uiScale(c)
	ui.Row(c.Key("inst-scope-note")).Label("实例设置范围").FillWidth().Height(40 * s).PaddingX(14 * s).Gap(10 * s).Radius(10 * s).Background(ui.RGBA(120, 130, 160, .07)).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icons["info"]).Size(14*s, 14*s).TextColor(muted).Shrink(0)
		label(c, a.L("只对当前实例生效，账号与默认 Java、内存等在全局设置", "Only this instance. Account and defaults are global")).FontSize(typeFootnote * s).SingleLine().Grow(1).Basis(0).MinWidth(0)
		b := ui.ButtonBase(c.Key("inst-open-global")).Label(a.L("全局设置", "Global settings")).Height(26 * s).PaddingX(8 * s).Radius(7 * s).Gap(4 * s).AlignItems(ui.Center).Shrink(0)
		bg := ui.Transparent
		if b.Hovered() {
			bg = accentSoft
		}
		b.Background(b.AnimateColor("bg", bg, fastAnim))
		if b.Children(func() {
			ui.Text(c, a.L("全局设置", "Global settings")).FontSize(typeFootnote * s).FontWeight(600).TextColor(accent).NoWrap()
			ui.Icon(c, icons["chevron"]).Size(10*s, 10*s).TextColor(accent).Shrink(0)
		}).Clicked() {
			a.goPage("settings")
		}
	})
}

func kv(c *ui.Context, k, v string) {
	s := uiScale(c)
	ui.Row(c).FillWidth().Height(28 * s).Shrink(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(kvLabelW * s).Shrink(0).Clip().Children(func() { label(c, k).FontSize(typeCallout * s).NoWrap() })
		ui.Text(c, v).FontSize(typeCallout * s).FontWeight(600).TextColor(ink).SingleLine().Grow(1).Basis(0).MinWidth(0).Tooltip(v)
	})
}
