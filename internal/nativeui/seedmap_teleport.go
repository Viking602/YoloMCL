package nativeui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

type seedMapTeleport struct {
	key, y string
	cancel context.CancelFunc
}

func (v *seedMapView) discardTeleport() {
	if v.teleport.cancel != nil {
		v.teleport.cancel()
	}
	v.teleport = seedMapTeleport{}
}

// Prepare a height once per selection, off the UI thread. Manual edits win over
// a late sampler result, and a result from another point/seed cannot replace it.
func (a *App) seedMapTeleportHeight(c *ui.Context, v *seedMapView, x, z int, slice bool, sliceY int) (string, bool) {
	key := fmt.Sprintf("%d:%s:%d:%d:%d:%t", v.generation, v.viewport.Dimension, x, z, v.viewport.Y, slice)
	if v.teleport.key != key {
		v.discardTeleport()
		v.teleport.key = key
		if slice {
			v.teleport.y = strconv.Itoa(sliceY)
		} else if v.sampler != nil {
			ctx, cancel := context.WithCancel(a.ctx)
			v.teleport.cancel = cancel
			sampler, dim, nearY := v.sampler, v.viewport.Dimension, v.viewport.Y
			a.bgWG.Add(1)
			go func() {
				defer a.bgWG.Done()
				y, ok := sampler.TeleportYAt(dim, x, z, nearY)
				a.post(func() {
					defer cancel()
					if ctx.Err() == nil && a.seedMap == v && v.teleport.key == key && v.teleport.y == "" && ok {
						v.teleport.y = strconv.Itoa(y)
					}
				})
			}()
		}
	}
	s := uiScale(c)
	ui.Row(c).Gap(8 * s).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.L("seedmap.teleport_y")).FontSize(typeCaption * s).TextColor(muted).NoWrap()
		textField(c.Key("seedmap-teleport-y"), &v.teleport.y).Label(a.L("seedmap.teleport_y_input")).Placeholder("Y").Width(76 * s).Height(btnSm * s)
	})
	y, err := strconv.Atoi(strings.TrimSpace(v.teleport.y))
	return fmt.Sprintf("/tp %d %d %d", x, y, z), err == nil && y >= -2048 && y <= 2047
}
