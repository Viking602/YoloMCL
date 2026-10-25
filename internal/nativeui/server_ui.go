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
	signature := struct {
		Ready, Present bool
		Server         launcher.InstanceServer
	}{Ready: st.Dir != ""}
	if st.RecentServer != nil {
		signature.Present, signature.Server = true, *st.RecentServer
	}
	st = swappedContent(a, c, "ov-server", signature, st)
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
		a.serverStatusBadge(c, probe)
	}, func() {
		if st.Dir == "" {
			ovLoading(c)
			return
		}
		if server == nil {
			emptyPanel(c, "server-empty", "server", a.L("还没有连接记录", "No connections yet"), a.L("从游戏中加入服务器，最近连接会显示在这里", "Join a multiplayer server to see your latest connection here"))
			return
		}
		ui.Column(c).Label("服务器信息").Font(uiSansFamily()).FillWidth().Grow(1).MinHeight(0).Clip().Children(func() {
			var signature struct {
				Present, PlayersKnown         bool
				Address, Description, Version string
				Online, Max                   int
			}
			if info := probe.status; info != nil {
				signature.Present, signature.PlayersKnown = true, info.PlayersKnown
				signature.Address, signature.Description, signature.Version = address, info.Description, info.Version
				signature.Online, signature.Max = info.Online, info.Max
			}
			info := swappedContent(a, c, "server-details", signature, probe.status)
			a.contentBody(c, "server-details").Grow(1).MinHeight(0).Gap(12 * s).Justify(ui.Center).Children(func() {
				ui.Row(c.Key("server-identity")).Label("服务器标识").FillWidth().Height(66 * s).Shrink(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
					a.serverIcon(c, probe.icon)
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(5 * s).Children(func() {
						tooltip(c, ui.Text(c, address).FontSize(typeTitle2*s).FontWeight(650).TextColor(ink).SingleLine().FillWidth(), address)
						description := a.L("多人游戏服务器", "Multiplayer server")
						if info != nil && info.Description != "" {
							description = info.Description
						}
						tooltip(c, ui.Text(c, description).FontSize(typeFootnote*s).TextColor(muted).FillWidth().MaxLines(2).Ellipsis("…"), description)
					})
				})
				a.serverMetrics(c, info)
			})
		})
		ui.Row(c).Font(uiSansFamily()).FillWidth().Height(30 * s).Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
			text := a.L("上次连接 ", "Connected ")
			if !server.Confirmed {
				text = a.L("最近尝试 ", "Attempted ")
			}
			tooltip(c, ui.Text(c, text+agoText(a, server.Connected)).FontSize(typeCaption*s).TextColor(muted).SingleLine().Grow(1).Basis(0).MinWidth(0), server.Connected.Local().Format("2006-01-02 15:04:05"))
			copy := ui.ButtonBase(c.Key("ov-copy-server")).Label(a.L("复制地址", "Copy address")).Height(28 * s).PaddingX(8 * s).Radius(8 * s).Gap(5 * s).Shrink(0).AlignItems(ui.Center)
			color := ui.Transparent
			if copy.Hovered() {
				color = accentSoft
			}
			copy.Background(copy.AnimateColor("bg", color, fastAnim))
			if copy.Children(func() {
				ui.Icon(c, icons["doc"]).Size(12*s, 12*s).TextColor(accent)
				ui.Text(c, a.L("复制地址", "Copy address")).Label("按钮文字 ov-copy-server").FontSize(typeCaption * s).FontWeight(600).TextColor(accent).NoWrap()
			}).Clicked() && a.overviewActionReady("ov-server") {
				c.WriteClipboard(address)
				a.notice(a.L("已复制服务器地址", "Server address copied"))
			}
		})
	})
}

func (a *App) serverStatusBadge(c *ui.Context, probe serverProbe) {
	s := uiScale(c)
	text, label, color := a.L("查询中", "Checking"), a.L("正在查询服务器", "Checking server"), muted
	if probe.status != nil {
		text = fmt.Sprintf("%d ms", max(1, probe.status.Latency.Milliseconds()))
		label, color = text, successInk
		if probe.status.Latency >= 150*time.Millisecond {
			color = warningInk
		}
	} else if !probe.checked.IsZero() {
		text, label = a.L("不可达", "Unreachable"), a.L("服务器不可达", "Server unreachable")
	}
	ui.Row(c.Key("server-status-slot")).Label("服务器状态").Height(23 * s).PaddingX(8 * s).Gap(5 * s).Radius(999).Background(color.Alpha(.08)).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(5*s, 5*s).Radius(3 * s).Background(color).Shrink(0)
		numText(c, text, typeCaption*s, color).Label(label).FontWeight(600)
	})
}

func (a *App) serverMetrics(c *ui.Context, info *launcher.ServerStatus) {
	s := uiScale(c)
	players, version := "—", "—"
	if info != nil {
		if info.PlayersKnown {
			players = fmt.Sprintf("%d / %d", info.Online, info.Max)
		}
		if info.Version != "" {
			version = info.Version
		}
	}
	ui.Row(c.Key("server-metrics")).Label("服务器数据").FillWidth().Height(68*s).Shrink(0).Padding(10*s, 12*s).Gap(12 * s).Radius(10 * s).Background(ui.RGBA(100, 120, 160, .055)).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(.8).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
			ui.Text(c, a.L("在线玩家", "Players online")).FontSize(typeCaption * s).TextColor(muted).SingleLine().FillWidth()
			numText(c, players, typeTitle*s, ink).FontWeight(650).SingleLine().FillWidth()
		})
		ui.Box(c).Width(1).Height(30 * s).Shrink(0).Background(line)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
			ui.Text(c, a.L("版本", "Version")).FontSize(typeCaption * s).TextColor(muted).SingleLine().FillWidth()
			tooltip(c, ui.Text(c, version).FontSize(typeFootnote*s).FontWeight(600).TextColor(inkSecondary).SingleLine().FillWidth(), version)
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
