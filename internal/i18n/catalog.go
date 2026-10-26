package i18n

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"slices"
	"strings"
)

//go:embed locales/*.json
var localeFiles embed.FS

type numberUnit struct {
	Size int64  `json:"size"`
	Name string `json:"name"`
}

type catalog struct {
	Code        string            `json:"code"`
	Copy        Copy              `json:"copy"`
	Messages    map[string]string `json:"messages"`
	Categories  map[string]string `json:"categories"`
	JobMessages map[string]string `json:"jobMessages"`
	Phases      map[string]string `json:"phases"`
	NumberUnits []numberUnit      `json:"numberUnits"`
	jobPrefixes []string
	labels      map[string]map[string]string
}

var (
	english = mustLoadCatalog(LangEN, nil)
	chinese = mustLoadCatalog(LangZH, &english)
)

// Catalogs are parsed once, before the first frame. Embedding keeps packaged
// builds independent of their working directory; JSON is the source of truth.
func mustLoadCatalog(code string, fallback *catalog) catalog {
	c, err := loadCatalog(localeFiles, "locales/"+code+".json", code, fallback)
	if err != nil {
		panic(err) // Invalid bundled translations are a build/configuration error.
	}
	return c
}

func loadCatalog(files fs.FS, path, code string, fallback *catalog) (catalog, error) {
	var c catalog
	if fallback != nil {
		c.Copy = fallback.Copy
		c.Messages = maps.Clone(fallback.Messages)
		c.Categories = maps.Clone(fallback.Categories)
		c.JobMessages = maps.Clone(fallback.JobMessages)
		c.Phases = maps.Clone(fallback.Phases)
		c.NumberUnits = slices.Clone(fallback.NumberUnits)
	}
	b, err := fs.ReadFile(files, path)
	if err != nil {
		return catalog{}, fmt.Errorf("i18n %s: %w", path, err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return catalog{}, fmt.Errorf("i18n %s: %w", path, err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return catalog{}, fmt.Errorf("i18n %s: expected one JSON object", path)
	}
	if c.Code != code {
		return catalog{}, fmt.Errorf("i18n %s: language code %q, want %q", path, c.Code, code)
	}
	if c.Messages == nil || c.Categories == nil || c.JobMessages == nil || c.Phases == nil || len(c.NumberUnits) == 0 {
		return catalog{}, fmt.Errorf("i18n %s: missing or null catalog section", path)
	}
	for i, unit := range c.NumberUnits {
		if unit.Size < 10 || unit.Size%10 != 0 || unit.Name == "" || (i > 0 && unit.Size >= c.NumberUnits[i-1].Size) {
			return catalog{}, fmt.Errorf("i18n %s: invalid number unit %d", path, i)
		}
	}
	c.Copy.Code = c.Code
	c.labels = labelKeys(c.Copy)
	for prefix := range c.JobMessages {
		if prefix == "" {
			return catalog{}, fmt.Errorf("i18n %s: empty job message prefix", path)
		}
		c.jobPrefixes = append(c.jobPrefixes, prefix)
	}
	// Specific messages must win over generic prefixes such as "Download".
	slices.SortFunc(c.jobPrefixes, func(a, b string) int {
		if len(a) != len(b) {
			return len(b) - len(a)
		}
		return strings.Compare(a, b)
	})
	return c, nil
}

func forLanguage(en bool) *catalog {
	if en {
		return &english
	}
	return &chinese
}

// Lookup returns a message and whether its key exists. Missing translations
// inherit English at load time; explicit empty strings remain empty.
func Lookup(en bool, key string) (string, bool) {
	text, ok := forLanguage(en).Messages[key]
	return text, ok
}

// Text returns a localized message, or the key itself for an unknown key.
func Text(en bool, key string) string {
	if text, ok := Lookup(en, key); ok {
		return text
	}
	return key
}

// Plural returns the one/other form of a counted message.
func Plural(en bool, key string, count int) string {
	if count == 1 {
		return Text(en, key+".one")
	}
	return Text(en, key+".other")
}

// JobMessage translates known launcher progress prefixes while preserving
// filenames and counts. Unknown backend messages are displayed unchanged.
func JobMessage(en bool, message string) string {
	if !en {
		return message
	}
	c := forLanguage(en)
	for _, prefix := range c.jobPrefixes {
		if rest, ok := strings.CutPrefix(message, prefix); ok {
			return c.JobMessages[prefix] + strings.ReplaceAll(rest, Text(false, "job.itemPrefix"), Text(en, "job.itemPrefix"))
		}
	}
	return message
}

// Phase translates a known Java scan phase, preserving unknown status text.
func Phase(en bool, phase string) string {
	if text, ok := forLanguage(en).Phases[phase]; ok {
		return text
	}
	return phase
}

// CompactNumber uses the locale's configured units. The caller formats exact
// counts when the result is empty. Truncation avoids rounding across a unit.
func CompactNumber(en bool, n int64) string {
	for _, unit := range forLanguage(en).NumberUnits {
		if n >= unit.Size {
			tenths := n / (unit.Size / 10)
			if tenths%10 == 0 {
				return fmt.Sprintf("%d%s", tenths/10, unit.Name)
			}
			return fmt.Sprintf("%d.%d%s", tenths/10, tenths%10, unit.Name)
		}
	}
	return ""
}

// Canonical recognizes selector labels from either catalog. UI state can
// therefore survive a language switch even after a translator edits a label.
func Canonical(group, value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if key := chinese.labels[group][value]; key != "" {
		return key
	}
	return english.labels[group][value]
}

func labelKeys(c Copy) map[string]map[string]string {
	groups := map[string]map[string]string{
		"resourceType":  {"mod": c.DiscoverTypeMod, "resourcepack": c.DiscoverTypeRP, "shader": c.DiscoverTypeShader, "modpack": c.DiscoverTypeModpack},
		"sort":          {"downloads": c.DiscoverSortDownloads, "updated": c.DiscoverSortUpdated, "relevance": c.DiscoverSortRelevance},
		"accountTab":    {"microsoft": c.AccTabMicrosoft, "third": c.AccTabThirdParty, "offline": c.AccTabOffline},
		"skinModel":     {"slim": c.WardrobeModelAlex, "default": c.WardrobeModelSteve},
		"installTarget": {"current": c.InstallTargetCurrent, "new": c.InstallTargetNew},
		"versionKind":   {"release": c.InstallKindRelease, "snapshot": c.InstallKindSnapshot, "old_beta": c.InstallKindBeta, "old_alpha": c.InstallKindAlpha, "other": c.InstallKindOther, "all": c.InstallKindAll},
		"loader":        {"vanilla": c.LoaderVanilla, "fabric": c.LoaderFabric, "quilt": c.LoaderQuilt, "forge": c.LoaderForge, "neoforge": c.LoaderNeoForge},
		"cape":          {"none": c.WardrobeCapeNone},
	}
	labels := make(map[string]map[string]string, len(groups))
	for group, entries := range groups {
		labels[group] = make(map[string]string, len(entries))
		for key, label := range entries {
			if label != "" {
				labels[group][strings.ToLower(strings.TrimSpace(label))] = key
			}
		}
	}
	labels["loader"][strings.ToLower(strings.TrimSpace(c.LoaderAll))] = "vanilla"
	return labels
}
