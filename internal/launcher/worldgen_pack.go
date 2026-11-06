package launcher

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
)

// WorldgenPackFormat is Minecraft's major/minor SERVER_DATA format. An integer
// max_format includes all minors; [major, minor] is an exact inclusive bound.
type WorldgenPackFormat struct{ Major, Minor int }

func (v WorldgenPackFormat) compare(other WorldgenPackFormat) int {
	if v.Major != other.Major {
		return v.Major - other.Major
	}
	return v.Minor - other.Minor
}

// Only release formats belong here. Snapshot names must come from their actual
// version.json instead of being guessed from the next release's name.
func worldgenReleasePackFormat(version string) *WorldgenPackFormat {
	formats := map[string]WorldgenPackFormat{
		"1.20": {15, 0}, "1.20.1": {15, 0}, "1.20.2": {18, 0},
		"1.20.3": {26, 0}, "1.20.4": {26, 0}, "1.20.5": {41, 0}, "1.20.6": {41, 0},
		"1.21": {48, 0}, "1.21.1": {48, 0}, "1.21.2": {57, 0}, "1.21.3": {57, 0},
		"1.21.4": {61, 0}, "1.21.5": {71, 0}, "1.21.6": {80, 0}, "1.21.7": {81, 0}, "1.21.8": {81, 0},
		"26.1": {101, 1}, "26.1.1": {101, 1}, "26.1.2": {101, 1}, "26.2": {107, 1}, "26.3": {121, 0},
	}
	format, ok := formats[version]
	if !ok {
		return nil
	}
	return &format
}

var worldgenOverlayDirectory = regexp.MustCompile(`^[-_a-zA-Z0-9.]+$`)

func worldgenFormatBound(raw json.RawMessage, top bool) (WorldgenPackFormat, error) {
	var major int
	if json.Unmarshal(raw, &major) == nil && major >= 0 {
		minor := 0
		if top {
			minor = math.MaxInt32
		}
		return WorldgenPackFormat{major, minor}, nil
	}
	var parts []int
	if json.Unmarshal(raw, &parts) != nil || len(parts) < 1 || len(parts) > 2 || parts[0] < 0 {
		return WorldgenPackFormat{}, errors.New("invalid pack format")
	}
	if len(parts) == 1 {
		minor := 0
		if top {
			minor = math.MaxInt32
		}
		return WorldgenPackFormat{parts[0], minor}, nil
	}
	if parts[1] < 0 {
		return WorldgenPackFormat{}, errors.New("invalid pack minor format")
	}
	return WorldgenPackFormat{parts[0], parts[1]}, nil
}

func (c *worldgenCollector) readVersionFormat(data []byte) {
	var version struct {
		PackVersion struct {
			Data      int `json:"data"`
			DataMajor int `json:"data_major"`
			DataMinor int `json:"data_minor"`
		} `json:"pack_version"`
	}
	if json.Unmarshal(data, &version) != nil {
		return
	}
	major := version.PackVersion.DataMajor
	if major == 0 {
		major = version.PackVersion.Data
	}
	if major > 0 && version.PackVersion.DataMinor >= 0 {
		c.format = &WorldgenPackFormat{major, version.PackVersion.DataMinor}
		c.snapshot.PackFormat = c.format
	}
}

func worldgenLegacyFormatRange(raw json.RawMessage) (WorldgenPackFormat, WorldgenPackFormat, error) {
	var number int
	if json.Unmarshal(raw, &number) == nil && number >= 0 {
		return WorldgenPackFormat{number, 0}, WorldgenPackFormat{number, math.MaxInt32}, nil
	}
	var pair []int
	if json.Unmarshal(raw, &pair) == nil && len(pair) == 2 && pair[0] >= 0 && pair[1] >= pair[0] {
		return WorldgenPackFormat{pair[0], 0}, WorldgenPackFormat{pair[1], math.MaxInt32}, nil
	}
	var bounds struct {
		Min *int `json:"min_inclusive"`
		Max *int `json:"max_inclusive"`
	}
	if json.Unmarshal(raw, &bounds) == nil && bounds.Min != nil && bounds.Max != nil && *bounds.Min >= 0 && *bounds.Max >= *bounds.Min {
		return WorldgenPackFormat{*bounds.Min, 0}, WorldgenPackFormat{*bounds.Max, math.MaxInt32}, nil
	}
	return WorldgenPackFormat{}, WorldgenPackFormat{}, errors.New("invalid legacy pack format range")
}

// Overlay order is semantic: later matching entries override earlier entries.
// ZIP entry order or lexical directory order must never substitute for it.
func worldgenPackOverlays(data []byte, format *WorldgenPackFormat) ([]string, error) {
	var metadata struct {
		Overlays struct {
			Entries []struct {
				Directory string          `json:"directory"`
				Formats   json.RawMessage `json:"formats"`
				Min       json.RawMessage `json:"min_format"`
				Max       json.RawMessage `json:"max_format"`
			} `json:"entries"`
		} `json:"overlays"`
	}
	if !boundedWorldgenJSON(data) || json.Unmarshal(data, &metadata) != nil || len(metadata.Overlays.Entries) > 64 {
		return nil, errors.New("invalid pack metadata")
	}
	if len(metadata.Overlays.Entries) != 0 && format == nil {
		return nil, errors.New("unknown data pack version for overlays")
	}
	var selected []string
	for _, entry := range metadata.Overlays.Entries {
		if !worldgenOverlayDirectory.MatchString(entry.Directory) || entry.Directory == "." || entry.Directory == ".." {
			return nil, errors.New("invalid overlay directory")
		}
		var low, high WorldgenPackFormat
		var err error
		if len(entry.Min) != 0 || len(entry.Max) != 0 {
			low, err = worldgenFormatBound(entry.Min, false)
			if err == nil {
				high, err = worldgenFormatBound(entry.Max, true)
			}
		} else {
			low, high, err = worldgenLegacyFormatRange(entry.Formats)
		}
		if err != nil || low.compare(high) > 0 {
			return nil, errors.New("invalid overlay format bounds")
		}
		if format.compare(low) >= 0 && format.compare(high) <= 0 {
			selected = append(selected, entry.Directory)
		}
	}
	return selected, nil
}
