package launcher

import (
	_ "embed"
	"encoding/json"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

//go:embed game_preferences_catalog.json
var gamePreferencesJSON []byte

// GamePreference describes an options.txt value, including its display units.
// Instance editors only expose fields already written by that game version.
type GamePreference struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Group    string   `json:"group"`
	Kind     string   `json:"kind"`
	Default  string   `json:"default"`
	Min      float64  `json:"min"`
	Max      float64  `json:"max"`
	Step     float64  `json:"step"`
	Scale    float64  `json:"scale"`
	Offset   float64  `json:"offset"`
	Unit     string   `json:"unit,omitempty"`
	MaxLabel string   `json:"maxLabel,omitempty"`
	Choices  []string `json:"choices,omitempty"`
}

type GamePreferenceRow struct {
	GamePreference
	Current    string `json:"current"`
	Inherited  string `json:"inherited"`
	Override   string `json:"override"`
	Source     string `json:"source"`
	Configured bool   `json:"configured"`
}

var gamePreferenceCatalog = func() []GamePreference {
	var out []GamePreference
	if err := json.Unmarshal(gamePreferencesJSON, &out); err != nil {
		panic(err)
	}
	return out
}()

func gamePreference(id string) (GamePreference, bool) {
	for _, spec := range gamePreferenceCatalog {
		if spec.ID == id {
			return spec, true
		}
	}
	return GamePreference{ID: id, Group: "other", Kind: "text"}, false
}

func validPreferenceID(id string) bool {
	if id == "" || len(id) > 256 || validBindingID(id) || strings.ContainsAny(id, ":\r\n\x00") || strings.IndexFunc(id, unicode.IsControl) >= 0 {
		return false
	}
	// These are game metadata or are owned by the resource-pack manager.
	switch id {
	case "version", "resourcePacks", "incompatibleResourcePacks", "lastServer":
		return false
	}
	return true
}

func parseManagedOptions(data []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimPrefix(string(data), "\ufeff"), "\n") {
		key, value, ok := strings.Cut(strings.TrimSuffix(line, "\r"), ":")
		if ok && (validBindingID(key) || validPreferenceID(key)) {
			out[key] = value
		}
	}
	return out
}

func managedValues(bindings, options map[string]string) map[string]string {
	out := map[string]string{}
	maps.Copy(out, options)
	maps.Copy(out, bindings)
	return out
}

func (s *Service) globalManagedOptions() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return managedValues(s.cfg.KeyBindings, s.cfg.GamePreferences)
}

func globalPreferenceRows(global map[string]string) []GamePreferenceRow {
	rows := []GamePreferenceRow{}
	for _, spec := range gamePreferenceCatalog {
		value, owned := global[spec.ID]
		rows = append(rows, GamePreferenceRow{GamePreference: spec, Inherited: spec.Default, Override: value, Configured: owned, Source: "game"})
	}
	return rows
}

func preferenceRows(data []byte, global, local map[string]string, state keyBindingState) []GamePreferenceRow {
	rows := []GamePreferenceRow{}
	for id, current := range parseManagedOptions(data) {
		if validBindingID(id) {
			continue
		}
		spec, _ := gamePreference(id)
		row := GamePreferenceRow{GamePreference: spec, Current: current, Inherited: current, Source: "game"}
		if last, ok := state.Keys[id]; ok && last.Applied == current {
			row.Inherited = last.Original
		}
		if value, ok := global[id]; ok {
			row.Inherited, row.Source = value, "global"
		}
		row.Override, row.Configured = local[id]
		rows = append(rows, row)
	}
	order := map[string]int{}
	for i, spec := range gamePreferenceCatalog {
		order[spec.ID] = i
	}
	slices.SortFunc(rows, func(a, b GamePreferenceRow) int {
		x, xKnown := order[a.ID]
		y, yKnown := order[b.ID]
		if xKnown && yKnown {
			return x - y
		}
		if xKnown {
			return -1
		}
		if yKnown {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return rows
}

func validateGamePreferences(values map[string]string, allowed map[string]bool, global bool) (map[string]string, error) {
	if len(values) > 4096 {
		return nil, keyBindingError("too_many")
	}
	out := map[string]string{}
	for id, value := range values {
		spec, known := gamePreference(id)
		if !validPreferenceID(id) || global && !known || allowed != nil && !allowed[id] {
			return nil, keyBindingError("unknown_option", id)
		}
		if len(value) > 4096 || strings.ContainsAny(value, "\r\n\x00") {
			return nil, keyBindingError("invalid_option", id)
		}
		valid := true
		switch spec.Kind {
		case "bool":
			valid = value == "true" || value == "false"
		case "number":
			n, err := strconv.ParseFloat(value, 64)
			valid = err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= spec.Min && n <= spec.Max
			if spec.Step == 1 {
				valid = valid && n == math.Trunc(n)
			}
			// Minecraft reads integer options with Integer.parseInt, which
			// rejects values such as "16.000000" even though they are integral.
			if valid {
				value = strconv.FormatFloat(n, 'f', -1, 64)
			}
		case "select":
			valid = slices.Contains(spec.Choices, value)
		}
		if !valid {
			return nil, keyBindingError("invalid_option", id)
		}
		out[id] = value
	}
	return out, nil
}

func validateManagedValues(values map[string]string, global bool) (map[string]string, error) {
	bindings, options := map[string]string{}, map[string]string{}
	for id, value := range values {
		if validBindingID(id) {
			bindings[id] = value
		} else {
			options[id] = value
		}
	}
	bindings, err := validateKeyBindings(bindings, nil, global)
	if err != nil {
		return nil, err
	}
	options, err = validateGamePreferences(options, nil, global)
	if err != nil {
		return nil, err
	}
	return managedValues(bindings, options), nil
}
