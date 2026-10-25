package nativeui

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// verTag is a small superscript badge hung at the top right of a version
// name: Java, 有更新, 已自定义, 运行中.
type verTag struct {
	text, label string
	color       ui.Color
}

func (a *App) versionTags(r launcher.Instance, v launcher.VersionSummary, featured bool) []verTag {
	t := a.tr()
	var tags []verTag
	if a.nameOf(r, v).MCTag {
		tags = append(tags, verTag{v.Minecraft, "Minecraft " + v.Minecraft, tagNeutral})
	}
	if featured && slices.Contains(r.RunningVersions, v.ID) {
		tags = append(tags, verTag{t.HomeHeroBadgeRunning, "", successInk})
	}
	if j := javaFor(r, v); j > 0 {
		tags = append(tags, verTag{fmt.Sprintf("Java %d", j), fmt.Sprintf("Java 标签 %d", j), accent})
	}
	if v.JavaPath != "" || v.MemoryMB > 0 || v.JVMArgs != "" {
		tags = append(tags, verTag{t.HomeBadgeCustomized, "", tagNeutral})
	}
	if packHasUpdate(r, v.ID) && (featured || r.Pack.GameVersion != "") {
		tags = append(tags, verTag{t.PackUpdateBadge, "", warningInk})
	}
	return tags
}

func microTag(c *ui.Context, tg verTag, tip string) {
	s := uiScale(c)
	lbl := tg.label
	if lbl == "" {
		lbl = "版本标签 " + tg.text
	}
	e := ui.Row(c).Label(lbl).Shrink(0).Padding(1.5*s, 5.5*s).Radius(5*s).Background(tg.color.Alpha(.10)).Border(1, tg.color.Alpha(.20)).Children(func() {
		ui.Text(c, midEllipsis(c, tg.text, tagMaxWidth*s, typeCaption2*s, 600)).FontSize(typeCaption2 * s).FontWeight(600).TextColor(tg.color).NoWrap()
	})
	if tip != "" {
		tooltip(c, e, tip)
	}
}

// titleWithTags lays the version name out with its tags right after it,
// top-aligned like superscripts. The name ellipsizes first; past max tags
// the rest collapse into "+N".
func titleWithTags(c *ui.Context, title, tip string, size float32, weight int, tags []verTag, maxTags int) {
	s := uiScale(c)
	ui.Row(c).FillWidth().MinWidth(0).Gap(5 * s).AlignItems(ui.Start).Children(func() {
		tooltip(c, ui.Text(c, title).Shrink(1).MinWidth(0).FontSize(size).FontWeight(weight).TextColor(ink).SingleLine(), tip)
		if len(tags) == 0 {
			return
		}
		ui.Row(c).Shrink(0).Gap(3*s).Margin(size*.12, 0, 0, 0).Children(func() {
			shown := tags
			if len(tags) > maxTags {
				shown = tags[:maxTags-1]
			}
			for _, tg := range shown {
				microTag(c, tg, "")
			}
			if rest := tags[len(shown):]; len(rest) > 0 {
				names := make([]string, len(rest))
				for i, r := range rest {
					names[i] = r.text
				}
				microTag(c, verTag{fmt.Sprintf("+%d", len(rest)), "版本标签更多", tagNeutral}, strings.Join(names, "   "))
			}
		})
	})
}

// isPackVersion reports whether v is the version the root's modpack
// installed (pack-lock present).
func isPackVersion(r launcher.Instance, v launcher.VersionSummary) bool {
	if r.Pack == nil || r.Pack.Name == "" {
		return false
	}
	if r.Pack.GameVersion != "" {
		return r.Pack.GameVersion == v.ID
	}
	return len(r.Versions) == 1
}

// versionName is the naming helper's result for one version.
type versionName struct {
	Title, Sub string
	MCTag      bool // show the Minecraft version as a tag (title lacks it)
	Default    bool // Title is the default form, not a user/pack/custom name
}

var (
	autoFabricRe = regexp.MustCompile(`^(fabric|quilt)-loader-[^-]+-(.+)$`)
	autoForgeRe  = regexp.MustCompile(`^(.+)-forge-?(.+)$`)
	autoOtherRe  = regexp.MustCompile(`(?i)^(.+)-(fabric|quilt|forge|neoforge)[ -]([0-9][^ ]*)$`)
)

// isAutoID reports whether a version folder id is one a launcher generates
// (vanilla, Fabric, Quilt, Forge, NeoForge naming), as opposed to a name
// the user typed at install time.
func isAutoID(v launcher.VersionSummary) bool {
	id, mc := v.ID, v.Minecraft
	switch {
	case id == mc:
		return true
	case strings.HasPrefix(id, "neoforge-") || strings.HasPrefix(id, "forge-"+mc):
		return true
	}
	if m := autoFabricRe.FindStringSubmatch(id); m != nil && (mc == "" || strings.HasSuffix(id, "-"+mc)) {
		return true // fabric-loader-<v>-<mc>, quilt-loader-0.20.0-beta.9-<mc>
	}
	if m := autoForgeRe.FindStringSubmatch(id); m != nil && (mc == "" || m[1] == mc) {
		return true
	}
	if m := autoOtherRe.FindStringSubmatch(id); m != nil && (mc == "" || m[1] == mc) {
		return true // other launchers' "1.21.1-Fabric 0.16.14"
	}
	return false
}

func (a *App) loaderLine(v launcher.VersionSummary) string {
	if loaderKey(v.Loader) == "vanilla" || v.Loader == "" {
		return a.tr().LoaderVanilla
	}
	lv := v.LoaderVersion
	if lv == "" {
		lv = loaderVersionOf(v)
	}
	return strings.TrimSpace(a.loaderDisplay(v.Loader) + " " + lv)
}

// nameOf names a version. Precedence: (1) the user-set display name, (2)
// the modpack's own name, (3) a custom folder id typed at install, (4) the
// default form 原版 <mc> / <Loader> <loaderVersion>. Outside the default
// form the Minecraft version is a tag and the loader stays in the
// subtitle, so no information is lost and nothing is said twice.
func (a *App) nameOf(r launcher.Instance, v launcher.VersionSummary) versionName {
	t := a.tr()
	loader := a.loaderLine(v)
	switch {
	case strings.TrimSpace(v.DisplayName) != "":
		return versionName{Title: v.DisplayName, Sub: loader, MCTag: v.Minecraft != ""}
	case isPackVersion(r, v):
		pv := r.Pack.Version
		if pv != "" && !strings.HasPrefix(strings.ToLower(pv), "v") && pv[0] >= '0' && pv[0] <= '9' {
			pv = "v" + pv
		}
		lv := a.loaderDisplay(v.Loader)
		if r.Pack.LoaderVersion != "" && loaderKey(v.Loader) != "vanilla" {
			lv += " " + r.Pack.LoaderVersion
		}
		sub := lv
		if pv != "" {
			sub = pv + "   " + lv
		}
		return versionName{Title: r.Pack.Name, Sub: sub, MCTag: v.Minecraft != ""}
	case !isAutoID(v):
		return versionName{Title: v.ID, Sub: loader, MCTag: v.Minecraft != ""}
	case loaderKey(v.Loader) == "vanilla":
		word := t.LoaderVanilla
		if v.Snapshot {
			word = t.VersionSnapshotWord
		}
		return versionName{Title: word + " " + v.Minecraft, Default: true}
	}
	lv := v.LoaderVersion
	if lv == "" {
		lv = loaderVersionOf(v)
	}
	if lv == "" { // loader build unknown: name by Minecraft version instead
		return versionName{Title: a.loaderDisplay(v.Loader) + " " + v.Minecraft, Default: true}
	}
	return versionName{Title: a.loaderDisplay(v.Loader) + " " + lv, MCTag: v.Minecraft != "", Default: true}
}

// cardText is a version's title and subtitle, disambiguated: when another
// version in the directory would read the same (same title, same
// Minecraft), the folder id is added to the subtitle.
func (a *App) cardText(r launcher.Instance, v launcher.VersionSummary) (title, sub string) {
	n := a.nameOf(r, v)
	for _, o := range r.Versions {
		if o.ID != v.ID && o.Minecraft == v.Minecraft && a.nameOf(r, o).Title == n.Title {
			n.Sub = joinDistinct(n.Sub, v.ID)
			break
		}
	}
	return n.Title, n.Sub
}

// versionLine is a version's name on one line, for pickers and headers:
// the card title plus its Minecraft version when the title lacks it.
func (a *App) versionLine(r launcher.Instance, id string) string {
	for _, v := range r.Versions {
		if v.ID == id {
			n := a.nameOf(r, v)
			if n.MCTag {
				return n.Title + " " + v.Minecraft
			}
			return n.Title
		}
	}
	return id
}

// joinDistinct joins non-empty parts with wide spacing, dropping repeats.
func joinDistinct(parts ...string) string {
	var out []string
	seen := map[string]bool{}
	for _, p := range parts {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return strings.Join(out, "   ")
}

// tagNeutral is the grey of neutral tags: about 7:1 on its tint at 10.5px.
var tagNeutral = ui.Hex("#475569")

// tagMaxWidth caps a tag; longer text ellipsizes (tooltip on the title).
const tagMaxWidth float32 = 120

func loaderIconName(loader string) string {
	v := strings.ToLower(strings.TrimSpace(loader))
	switch v {
	case "fabric":
		return "loader-fabric"
	case "forge":
		return "loader-forge"
	case "neoforge":
		return "loader-neoforge"
	case "quilt":
		return "loader-quilt"
	case "all", "all loaders", "全部加载器":
		return "layers"
	default:
		return "loader-vanilla"
	}
}

func loaderTone(loader string) ui.Color {
	switch strings.ToLower(strings.TrimSpace(loader)) {
	case "fabric":
		return ui.Hex("#0284c7")
	case "forge":
		return ui.Hex("#d97706")
	case "neoforge":
		return ui.Hex("#ea580c")
	case "quilt":
		return ui.Hex("#7c3aed")
	default:
		return successInk
	}
}

// loaderLogoKey maps a loader name or icon name to its official logo key,
// or "" when there is none (e.g. "all loaders").
func loaderLogoKey(loader string) string {
	k := strings.TrimPrefix(loaderIconName(loader), "loader-")
	if loaderLogos[k] != nil {
		return k
	}
	return ""
}

// loaderLogo draws the loader's official logo unaltered inside a neutral
// rounded container (no tinted tile), or nothing when it has none.
func loaderLogo(c *ui.Context, loader string, size float32) bool {
	k := loaderLogoKey(loader)
	if k == "" {
		return false
	}
	ui.Box(c).Size(size, size).Radius(size*.26).Shrink(0).Background(panel).Border(1, line).Clip().Center().Children(func() {
		inner := size * .8
		if k == "forge" || k == "vanilla" {
			inner = size // full-bleed app-icon style logos
		}
		ui.Image(c, loaderLogos[k]).Size(inner, inner).Fit(ui.Contain).Label(k + " logo")
	})
	return true
}

// javaTag is the small tinted pill for a recommended/required Java major.
func javaTag(c *ui.Context, major int) {
	if major <= 0 {
		return
	}
	scale := uiScale(c)
	ui.Row(c).Label(fmt.Sprintf("Java 标签 %d", major)).Shrink(0).Padding(2*scale, 8*scale).Radius(999).Background(accent.Alpha(.10)).Border(1, accent.Alpha(.18)).Children(func() {
		ui.Text(c, fmt.Sprintf("Java %d", major)).FontSize(typeCaption2 * scale).FontWeight(600).TextColor(accent).NoWrap().Shrink(0)
	})
}

func (a *App) releaseLabel(t string) (string, ui.Color) {
	switch strings.ToLower(t) {
	case "beta":
		return "Beta", warningInk
	case "alpha":
		return "Alpha", danger
	}
	return a.L("正式", "Release"), successInk
}
