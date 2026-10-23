package nativeui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type instanceListRow struct {
	key, heading string
	count        int
	version      launcher.VersionSummary
}

// MyGo's public List builds only the visible rows and two neighbours. Besides
// avoiding text layout for every installed pack, this prevents offscreen rows
// from starting icon reads before the first frame. Its measured anchor handles
// mixed header/instance heights, filtering, DPI changes and scroll-to-selection.
func (a *App) instanceVirtualList(c *ui.Context, r *launcher.Instance) {
	s := uiScale(c)
	var rows []instanceListRow
	var names map[string]versionName
	if r != nil {
		names = a.instanceNames(*r)
		for _, group := range a.instGroups(*r, names) {
			if len(group.items) == 0 {
				continue
			}
			rows = append(rows, instanceListRow{key: r.ID + "/group/" + group.key, heading: group.title, count: len(group.items)})
			for _, version := range group.items {
				if a.instScrollPending && version.ID == r.SelectedVersion {
					a.instRows.ScrollTo(len(rows), ui.Center)
					a.instScrollPending = false
				}
				rows = append(rows, instanceListRow{key: r.ID + "/version/" + version.ID, version: version})
			}
		}
	}
	track := scrollTracks["inst-list-scroll"]
	if track == nil {
		track = &scrollTrack{}
		scrollTracks["inst-list-scroll"] = track
	}
	list := ui.List(c.Key("inst-list-scroll"), &a.instRows, len(rows)).
		ItemKey(func(i int) any { return rows[i].key }).TrackScroll(&track.st).
		Grow(1).MinHeight(0).FillWidth().Padding(4*s, 12*s, 16*s).Gap(4 * s)
	a.instListBounds = list.LayoutBounds()
	overlayScrollbar(list, track).Rows(func(row ui.ListRow) {
		c, item := row.Context, rows[row.Index]
		if item.heading != "" {
			ui.Row(c).FillWidth().Padding(14*s, 10*s, 6*s, 10*s).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				label(c, item.heading).FontSize(typeFootnote * s).FontWeight(650).TextColor(muted).SingleLine().Shrink(0)
				label(c, fmt.Sprint(item.count)).FontSize(typeFootnote * s).TextColor(muted).SingleLine().Shrink(0)
			})
			return
		}
		a.instRow(c, *r, item.version, names[item.version.ID])
	})
}

// Disambiguate once per list build, not by comparing every version against
// every other version for each row and each search keystroke. Keep this local
// to the build so renames, pack updates and language changes never go stale.
func (a *App) instanceNames(r launcher.Instance) map[string]versionName {
	names := make(map[string]versionName, len(r.Versions))
	counts := make(map[[2]string]int, len(r.Versions))
	for _, v := range r.Versions {
		n := a.nameOf(r, v)
		names[v.ID] = n
		counts[[2]string{n.Title, v.Minecraft}]++
	}
	for _, v := range r.Versions {
		n := names[v.ID]
		if counts[[2]string{n.Title, v.Minecraft}] > 1 {
			n.Sub = joinDistinct(n.Sub, v.ID)
			names[v.ID] = n
		}
	}
	return names
}
