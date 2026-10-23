package nativeui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type serverProbe struct {
	status  *launcher.ServerStatus // nil after a completed probe means unreachable
	icon    ui.ImageSource
	checked time.Time
}

type recentServerPing struct {
	key     string
	gen     uint64
	loading bool
	cancel  context.CancelFunc
	cache   map[string]serverProbe
}

func (a *App) cancelServerPing() {
	if a.recentPing.cancel != nil {
		a.recentPing.cancel()
	}
	a.recentPing.gen++
	a.recentPing.key, a.recentPing.cancel, a.recentPing.loading = "", nil, false
}

func (a *App) serverProbe(address string) serverProbe {
	key := a.instStatusKey + "|" + address
	if a.recentPing.key != key {
		a.cancelServerPing()
		a.recentPing.key = key
	}
	cached := a.recentPing.cache[address]
	if a.recentPing.loading || time.Since(cached.checked) < 30*time.Second || a.ctx.Err() != nil {
		return cached
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.recentPing.cancel, a.recentPing.loading = cancel, true
	a.recentPing.gen++
	gen, scope := a.recentPing.gen, a.instStatusKey
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer cancel()
		st, err := launcher.QueryServerStatus(ctx, address)
		if ctx.Err() != nil {
			return
		}
		result := serverProbe{checked: time.Now()}
		if err == nil {
			result.status = &st
			if len(st.IconPNG) > 0 {
				result.icon, _ = decodeResourceImage(st.IconPNG)
			}
		}
		a.post(func() {
			r := a.root()
			if gen != a.recentPing.gen || a.recentPing.key != key || r == nil || r.ID+"|"+r.SelectedVersion != scope {
				return
			}
			a.recentPing.loading, a.recentPing.cancel = false, nil
			if a.recentPing.cache == nil {
				a.recentPing.cache = make(map[string]serverProbe)
			}
			if len(a.recentPing.cache) >= 16 {
				oldest, when := "", time.Now()
				for address, entry := range a.recentPing.cache {
					if entry.checked.Before(when) {
						oldest, when = address, entry.checked
					}
				}
				delete(a.recentPing.cache, oldest)
			}
			a.recentPing.cache[address] = result
		})
	}()
	return cached
}

func (a *App) ovServer(c *ui.Context, st launcher.InstanceStatus) {
	s := uiScale(c)
	if target := st.RecentServer; target != nil && !a.overviewHolding {
		a.serverProbe(target.Address)
	}
	st = swappedContent(a, c, "ov-server", []any{st.Dir != "", st.RecentServer}, st)
	a.resetContentChildren("ov-server", "server-details")
	server := st.RecentServer
	var probe serverProbe
	address := ""
	if server != nil {
		probe = a.recentPing.cache[server.Address]
		address = strings.TrimSuffix(server.Address, ":25565")
	}
	a.ovPanel(c, "ov-server", a.L("最近服务器", "Recent server"), st, func() {
		if server == nil {
			return
		}
		ui.Row(c.Key("server-status-slot")).Label("服务器状态").Height(24 * s).MinWidth(40 * s).Justify(ui.End).AlignItems(ui.Center).Children(func() {
			switch {
			case probe.status != nil:
				latency := max(1, probe.status.Latency.Milliseconds())
				quietTag(c, fmt.Sprintf("%d ms", latency), successInk)
			case !probe.checked.IsZero():
				ui.Text(c, "×").Label(a.L("服务器不可达", "Server unreachable")).FontSize(22 * s).FontWeight(650).TextColor(danger).Tooltip(a.L("不可达", "Unreachable"))
			default:
				ui.Text(c, "…").Label(a.L("正在查询服务器", "Checking server")).FontSize(typeTitle * s).TextColor(muted)
			}
		})
	}, func() {
		if st.Dir == "" {
			ovLoading(c)
			return
		}
		if server == nil {
			ui.Row(c.Key("server-identity")).Label("服务器标识").FillWidth().Height(62 * s).Shrink(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
				a.serverIcon(c, nil)
				title, hint := a.L("暂无连接记录", "No recent connection"), a.L("加入多人服务器后显示在这里", "Your latest multiplayer server appears here")
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(5 * s).Children(func() {
					ui.Text(c, title).FontSize(typeTitle * s).FontWeight(650).TextColor(ink).SingleLine().FillWidth()
					ui.Text(c, hint).FontSize(typeFootnote * s).TextColor(muted).SingleLine().FillWidth().Tooltip(hint)
				})
			})
			return
		}
		ui.Row(c.Key("server-identity")).Label("服务器标识").FillWidth().Height(62 * s).Shrink(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
			a.serverIcon(c, probe.icon)
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(5 * s).Children(func() {
				ui.Text(c, address).FontSize(typeTitle * s).FontWeight(700).TextColor(ink).SingleLine().FillWidth().Tooltip(address)
				text := a.L("上次连接 ", "Connected ")
				if !server.Confirmed {
					text = a.L("最近连接尝试 ", "Last attempted ")
				}
				ui.Text(c, text+agoText(a, server.Connected)).FontSize(typeFootnote * s).TextColor(muted).SingleLine().FillWidth().Tooltip(server.Connected.Local().Format("2006-01-02 15:04:05"))
			})
		})
		ui.Column(c).Label("服务器信息").FillWidth().Grow(1).MinHeight(0).Gap(8 * s).Clip().Children(func() {
			var signature any
			if info := probe.status; info != nil {
				signature = []any{address, info.Description, info.Version, info.PlayersKnown, info.Online, info.Max}
			}
			info := swappedContent(a, c, "server-details", signature, probe.status)
			a.contentBody(c, "server-details").Gap(8 * s).Children(func() {
				if info != nil {
					if info.Description != "" {
						ui.Text(c, info.Description).FontSize(typeCallout * s).TextColor(inkSecondary).FillWidth().MinWidth(0).MaxLines(2).Ellipsis("…").Tooltip(info.Description)
					}
					ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
						if info.PlayersKnown {
							ui.Text(c, fmt.Sprintf(a.L("%d / %d 在线", "%d / %d online"), info.Online, info.Max)).FontSize(typeFootnote * s).FontWeight(600).TextColor(successInk).SingleLine().Shrink(0)
						}
						if info.Version != "" {
							ui.Text(c, info.Version).FontSize(typeFootnote * s).TextColor(muted).SingleLine().Grow(1).MinWidth(0).Tooltip(info.Version)
						}
					})
				}
			})
		})
		ui.Row(c).FillWidth().Height(30 * s).Shrink(0).Justify(ui.End).Children(func() {
			if fixedButton(c, "ov-copy-server", a.L("复制地址", "Copy address"), "doc", 0, btnSm*s, false, false) && a.overviewActionReady("ov-server") {
				c.WriteClipboard(address)
				a.notice(a.L("已复制服务器地址", "Server address copied"))
			}
		})
	})
}

func (a *App) serverIcon(c *ui.Context, source ui.ImageSource) {
	s := uiScale(c)
	ui.Box(c.Key("server-icon-slot")).Label("服务器图标").Size(42*s, 42*s).Radius(12 * s).Background(accentSoft).Clip().Center().Shrink(0).Children(func() {
		if source != nil {
			img := ui.Image(c.Key("server-icon-image"), source).Size(42*s, 42*s).Fit(ui.Cover)
			if !c.Preferences().ReduceMotion {
				img.Transition(ui.ElementTransition{Animation: &motionEnter, Colors: true, Enter: &ui.Motion{Opacity: 0}})
			}
		} else {
			ui.Icon(c, icons["globe"]).Size(22*s, 22*s).TextColor(accent)
		}
	})
}
