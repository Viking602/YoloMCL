package nativeui

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type instanceListRow struct {
	key, heading string
	count        int
	version      launcher.VersionSummary
}

// Keep only the active directory's derived list. Compare values rather than
// snapshot pointers: polling replaces snapshots, while renames may edit them
// in place. Selection and scrolling do not change the list's contents.
type instanceListData struct {
	rootID, language, filter, loader                   string
	packName, packVersion, packGame, packLoaderVersion string
	versions                                           []launcher.VersionSummary
	names                                              map[string]versionName
	rows                                               []instanceListRow
}

func (a *App) instanceListItems(r *launcher.Instance) ([]instanceListRow, map[string]versionName) {
	if r == nil {
		a.instListData = instanceListData{}
		return nil, nil
	}
	next := instanceListData{rootID: r.ID, language: a.lang(), filter: a.instFilter, loader: a.instLoader}
	if p := r.Pack; p != nil {
		next.packName, next.packVersion, next.packGame, next.packLoaderVersion = p.Name, p.Version, p.GameVersion, p.LoaderVersion
	}
	prev := &a.instListData
	if prev.names != nil && prev.rootID == next.rootID && prev.language == next.language && prev.filter == next.filter && prev.loader == next.loader &&
		prev.packName == next.packName && prev.packVersion == next.packVersion && prev.packGame == next.packGame && prev.packLoaderVersion == next.packLoaderVersion &&
		slices.Equal(prev.versions, r.Versions) {
		return prev.rows, prev.names
	}
	next.versions = slices.Clone(r.Versions)
	next.names = a.instanceNames(*r)
	next.rows = make([]instanceListRow, 0, len(r.Versions)+2)
	for _, group := range a.instGroups(*r, next.names) {
		if len(group.items) == 0 {
			continue
		}
		next.rows = append(next.rows, instanceListRow{key: r.ID + "/group/" + group.key, heading: group.title, count: len(group.items)})
		for _, version := range group.items {
			next.rows = append(next.rows, instanceListRow{key: r.ID + "/version/" + version.ID, version: version})
		}
	}
	a.instListData = next
	return next.rows, next.names
}

// MyGo's public List builds only the visible rows and two neighbours. Besides
// avoiding text layout for every installed pack, this prevents offscreen rows
// from starting icon reads before the first frame. Its measured anchor handles
// mixed header/instance heights, filtering, DPI changes and scroll-to-selection.
func (a *App) instanceVirtualList(c *ui.Context, r *launcher.Instance) {
	s := uiScale(c)
	rows, names := a.instanceListItems(r)
	if a.instScrollPending && r != nil {
		for i, item := range rows {
			if item.heading == "" && item.version.ID == r.SelectedVersion {
				a.instRows.ScrollTo(i, ui.Center)
				a.instScrollPending = false
				break
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
// every other version for each row and each search keystroke. Rebuild names
// whenever the list inputs change, including renames and language changes.
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
