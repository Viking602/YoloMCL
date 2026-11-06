package nativeui

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/Viking602/YoloMCL/internal/i18n"
	"github.com/Viking602/YoloMCL/internal/launcher"
)

type seedMapLabels map[string]map[string]string

func readSeedMapLabels(snapshot *launcher.WorldgenSnapshot) seedMapLabels {
	labels := seedMapLabels{"en_us": {}, "zh_cn": {}}
	for _, doc := range snapshot.Documents {
		if doc.Kind != "language" {
			continue
		}
		_, language, ok := strings.Cut(doc.ID, ":")
		if !ok || labels[language] == nil {
			continue
		}
		var values map[string]string
		if json.Unmarshal(doc.Data, &values) != nil {
			continue
		}
		for k, v := range values {
			if v != "" {
				labels[language][k] = v
			}
		}
	}
	return labels
}

func (a *App) seedMapName(kind, id string, english bool) string {
	language := "zh_cn"
	if english {
		language = "en_us"
	}
	key := kind + "." + strings.NewReplacer(":", ".", "/", ".").Replace(id)
	if a.seedMap != nil {
		if name := a.seedMap.labels[language][key]; name != "" {
			return name
		}
	}
	if strings.HasPrefix(id, "minecraft:") {
		if name, ok := i18n.Lookup(english, "seedmap."+kind+"."+strings.TrimPrefix(id, "minecraft:")); ok {
			return name
		}
	}
	_, name, ok := strings.Cut(id, ":")
	if !ok {
		name = id
	}
	words := strings.Fields(strings.NewReplacer("_", " ", "/", " ").Replace(name))
	for i, word := range words {
		r := []rune(word)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

func (a *App) seedMapNames(kind, id string) (string, string) {
	primary := a.seedMapName(kind, id, a.isEN())
	secondary := a.seedMapName(kind, id, !a.isEN())
	if secondary == primary {
		secondary = ""
	}
	return primary, secondary
}

func seedMapDimensions(s *launcher.LocalSampler) []string {
	var options []string
	for _, id := range []string{"minecraft:overworld", "minecraft:the_nether", "minecraft:the_end"} {
		for _, known := range s.Dimensions() {
			if id == known {
				options = append(options, id)
				break
			}
		}
	}
	for _, id := range s.Dimensions() {
		if id != "minecraft:overworld" && id != "minecraft:the_nether" && id != "minecraft:the_end" {
			options = append(options, id)
		}
	}
	return options
}

func (a *App) seedMapStructureMatches(structure launcher.SeedMapStructure, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	values := []string{structure.ID, a.seedMapName("structure", structure.ID, true), a.seedMapName("structure", structure.ID, false)}
	values = append(values, structure.Structures...)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func seedMapStructureIcon(id string) string {
	switch {
	case strings.Contains(id, "village"), strings.Contains(id, "hut"), strings.Contains(id, "igloo"):
		return "home"
	case strings.Contains(id, "temple"), strings.Contains(id, "pyramid"), strings.Contains(id, "monument"):
		return "seedmap-temple"
	case strings.Contains(id, "ship"), strings.Contains(id, "ocean"):
		return "seedmap-ship"
	case strings.Contains(id, "mansion"), strings.Contains(id, "fortress"), strings.Contains(id, "complex"), strings.Contains(id, "outpost"):
		return "seedmap-fortress"
	case strings.Contains(id, "city"), strings.Contains(id, "cities"):
		return "seedmap-city"
	default:
		return "cube"
	}
}

var seedMapCoordPattern = regexp.MustCompile(`^(?i)x?\s*[=:]?\s*(-?\d+)\s*[\s,，]\s*(?:y?\s*[=:]?\s*-?\d+\s*[\s,，]\s*)?z?\s*[=:]?\s*(-?\d+)$`)

// parseSeedMapCoords reads "x z", "x, z", "x y z" or "X 100 Z -20" from the
// unified search box; a middle Y is accepted and ignored.
func parseSeedMapCoords(query string) (int, int, bool) {
	m := seedMapCoordPattern.FindStringSubmatch(strings.TrimSpace(query))
	if m == nil {
		return 0, 0, false
	}
	x, e1 := strconv.Atoi(m[1])
	z, e2 := strconv.Atoi(m[2])
	if e1 != nil || e2 != nil || x < -29900000 || x > 29900000 || z < -29900000 || z > 29900000 {
		return 0, 0, false
	}
	return x, z, true
}

func (a *App) seedMapBiomeMatches(id, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return false
	}
	for _, value := range []string{id, a.seedMapName("biome", id, true), a.seedMapName("biome", id, false)} {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}
