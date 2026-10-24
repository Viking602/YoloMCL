package nativeui

import (
	"encoding/json"
	"hash/fnv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Category hues: soft tint behind, a darker shade of the same hue for text
// (all text shades are >= 4.5:1 on their tint over white).
type catHue struct{ fg ui.Color }

var catPalette = []catHue{
	{ui.Hex("#1f7a3a")}, // 0 green: performance
	{ui.Hex("#1d5fb8")}, // 1 blue: technology
	{ui.Hex("#6b3fb5")}, // 2 purple: magic
	{ui.Hex("#a14d00")}, // 3 orange: adventure
	{ui.Hex("#b3261e")}, // 4 red: combat
	{ui.Hex("#55596a")}, // 5 grey: library
	{ui.Hex("#0b6e6e")}, // 6 teal: world
	{ui.Hex("#a8326e")}, // 7 pink: decoration
}

var catHueRules = []struct {
	hue  int
	keys []string
}{
	{0, []string{"optimi", "performance", "性能", "优化", "lightweight", "bug fix", "utility", "qol", "实用"}},
	{1, []string{"techn", "redstone", "automation", "energy", "storage", "transport", "processing", "科技", "红石", "create", "applied", "industrial", "thermal"}},
	{2, []string{"magic", "魔法", "blood", "thaum"}},
	{3, []string{"adventure", "rpg", "exploration", "quest", "冒险", "探索", "dimension", "维度", "structure", "结构"}},
	{4, []string{"combat", "pvp", "zombie", "armor", "weapon", "hardcore", "mobs", "战斗", "生物", "装备"}},
	{5, []string{"library", "api", "前置", "config", "server", "misc", "杂项", "mcreator", "kubejs", "crafttweaker"}},
	{6, []string{"worldgen", "world gen", "world-gen", "biome", "ore", "世界", "群系", "矿", "farming", "food", "农业", "食物"}},
	{7, []string{"decor", "cosmetic", "building", "furniture", "装饰", "建筑", "map", "地图", "atmosphere", "realistic", "cartoon", "fantasy", "vanilla-like"}},
}

func catColor(key string) ui.Color {
	k := strings.ToLower(key)
	for _, r := range catHueRules {
		for _, w := range r.keys {
			if strings.Contains(k, w) {
				return catPalette[r.hue].fg
			}
		}
	}
	h := fnv.New32a()
	h.Write([]byte(k))
	return catPalette[int(h.Sum32()%uint32(len(catPalette)))].fg
}

// catTag is a category chip in its hue; filled when selected.
func catTag(c *ui.Context, key, label string, selected bool) {
	s := uiScale(c)
	fg := catColor(key)
	bg, tc := fg.Alpha(.11), fg
	if selected {
		bg, tc = fg, ui.RGBA(255, 255, 255, 1)
	}
	ui.Row(c).Shrink(0).Height(18 * s).PaddingX(6 * s).Radius(5 * s).Background(bg).Center().Children(func() {
		ui.Text(c, label).FontSize(typeCaption2 * s).FontWeight(600).TextColor(tc).NoWrap()
	})
}

func (a *App) catKey() string { return a.provider + "|" + normalizeResourceType(a.resourceType) }

func (a *App) selectedCats() []string { return a.catSel[a.catKey()] }

func (a *App) catSelected(id string) bool {
	for _, x := range a.selectedCats() {
		if x == id {
			return true
		}
	}
	return false
}

func (a *App) toggleCat(id string) {
	if a.catSel == nil {
		a.catSel = map[string][]string{}
	}
	k := a.catKey()
	cur := a.catSel[k]
	out := []string{}
	found := false
	for _, x := range cur {
		if x == id {
			found = true
			continue
		}
		out = append(out, x)
	}
	if !found {
		out = append(out, id)
	}
	a.catSel[k] = out
	a.search()
}

// catLabel is a category's display name in the UI language, or "" when the
// Chinese UI has no translation for it.
func (a *App) catLabel(name string) string {
	if a.isEN() {
		return categoryTitle(name)
	}
	if l := a.translateCategory(name); hasCJK(l) {
		return l
	}
	return ""
}

func (a *App) ensureCats() {
	k := a.catKey()
	if _, ok := a.catOpts[k]; ok || a.catAsked[k] || a.service == nil {
		return
	}
	if a.catAsked == nil {
		a.catAsked, a.catOpts, a.catFailed = map[string]bool{}, map[string][]launcher.Category{}, map[string]bool{}
	}
	a.catAsked[k] = true
	src, kind := a.provider, normalizeResourceType(a.resourceType)
	b, _ := json.Marshal(map[string]string{"source": src, "type": kind})
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		var cats []launcher.Category
		raw, e := a.service.Dispatch(a.ctx, "categories", string(b))
		if e == nil {
			_ = json.Unmarshal([]byte(raw), &cats)
		}
		a.post(func() {
			if e != nil {
				logUIError("categories: " + e.Error())
				a.catFailed[k] = true
				return
			}
			a.catOpts[k] = cats
		})
	}()
}

// catIDFor maps a card's category (slug on Modrinth, name on CurseForge)
// to the id Search takes.
func (a *App) catIDFor(cat string) string {
	for _, o := range a.catOpts[a.catKey()] {
		if strings.EqualFold(o.Name, cat) || strings.EqualFold(o.Slug, cat) || o.ID == cat {
			return o.ID
		}
	}
	return ""
}

func (a *App) discoverCatsRail(c *ui.Context) {
	s := uiScale(c)
	a.ensureCats()
	k := a.catKey()
	if m := &a.discoverMotion.categories; m.key != k {
		*m = discoverCategoryMotion{key: k, entered: c.Now()}
	}
	if a.catFailed[k] {
		a.discoverCategoryBody(c).Children(func() {
			railTitle(c, a.L("分类", "Tags"))
			if a.loadFailed(c, "cats-failed", failInline, false) {
				delete(a.catFailed, k)
				delete(a.catAsked, k)
			}
		})
		return
	}
	opts := a.catOpts[k]
	if len(opts) == 0 {
		return
	}
	a.discoverCategoryBody(c).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			railTitle(c, a.L("分类", "Tags"))
			ui.Spacer(c).Grow(1)
			if n := len(a.selectedCats()); n > 0 {
				b := ui.ButtonBase(c.Key("cats-clear")).Label(a.L("清除", "Clear")).Height(22 * s).PaddingX(8 * s).Radius(6 * s).AlignItems(ui.Center)
				fg := accent
				if b.Hovered() {
					b.Background(accentSoft)
				}
				if b.Children(func() {
					ui.Text(c, a.L("清除", "Clear")+" "+itoaN(n)).FontSize(typeCaption * s).FontWeight(600).TextColor(fg).NoWrap()
				}).Clicked() {
					a.catSel[a.catKey()] = nil
					a.search()
				}
			}
		})
		scrollArea(c, "cats-scroll").FillWidth().MaxHeight(220 * s).Children(func() {
			ui.Row(c).FillWidth().Wrap().Gap(6*s).Padding(0, 10*s).Children(func() {
				for _, o := range opts {
					op := o
					l := a.catLabel(op.Name)
					if l == "" || isLoaderCategory(op.Slug) {
						continue
					}
					sel := a.catSelected(op.ID)
					b := ui.ButtonBase(c.Key("cat-" + op.ID)).Label(l).Radius(5 * s)
					if b.Children(func() { catTag(c, op.Name, l, sel) }).Clicked() {
						a.toggleCat(op.ID)
					}
				}
			})
		})
	})
}
