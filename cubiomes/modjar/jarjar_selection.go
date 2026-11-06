package modjar

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// archiveModIDs reads the mod IDs a Forge/NeoForge mod file declares.
func (c *Classpath) archiveModIDs(a Archive) []string {
	var ids []string
	for _, f := range a.Reader.File {
		if f.Name != "META-INF/mods.toml" && f.Name != "META-INF/neoforge.mods.toml" {
			continue
		}
		b, err := archiveBytes(c.ctx, f, 1<<20)
		if err != nil {
			return nil
		}
		// Only [[mods]] entries declare mods; [[dependencies.*]] modId keys
		// name requirements (minecraft, neoforge, ...) and must not shadow.
		var meta struct {
			Mods []struct {
				ModID string `toml:"modId"`
			} `toml:"mods"`
		}
		if toml.Unmarshal(b, &meta) != nil {
			return nil
		}
		for _, m := range meta.Mods {
			if m.ModID != "" {
				ids = append(ids, m.ModID)
			}
		}
	}
	return ids
}

type containedJar struct {
	Identifier struct{ Group, Artifact string }
	Version    struct{ Range, ArtifactVersion string }
	Path       string
	source     string
}

// JarJar intersects every declaration, including declarations found recursively.
// If that leaves one artifact version, its selection is independent of the
// loader release's tie-breaking policy. Multiple eligible versions remain a
// scoped unresolved dependency; choosing the highest is not JarJar 0.4.1's rule.
func (c *Classpath) selectContained() {
	// FML JarJar: a nested mod jar whose mod ID is already supplied by a
	// top-level mod file is not loaded; the top-level file provides that mod.
	topLevel := map[string]bool{}
	byName := map[string]Archive{}
	for _, a := range c.archives {
		byName[a.Name] = a
		if !strings.Contains(a.Name, "!/") {
			for _, id := range c.archiveModIDs(a) {
				topLevel[id] = true
			}
		}
	}
	excluded := map[string]bool{}
	var remaining []containedJar
	for _, j := range c.contained {
		shadowed := false
		if a, ok := byName[j.source]; ok {
			for _, id := range c.archiveModIDs(a) {
				shadowed = shadowed || topLevel[id]
			}
		}
		if shadowed {
			excluded[j.source] = true
		} else {
			remaining = append(remaining, j)
		}
	}
	groups := map[string][]containedJar{}
	for _, j := range remaining {
		if j.Identifier.Group != "" && j.Identifier.Artifact != "" {
			id := j.Identifier.Group + ":" + j.Identifier.Artifact
			groups[id] = append(groups[id], j)
		}
	}
	c.selectionErrors = map[string]string{}
	for id, jars := range groups {
		versions := map[string]bool{}
		for _, j := range jars {
			versions[j.Version.ArtifactVersion] = true
		}
		var ranges []mavenRange
		var problem error
		var recommended *mavenItem
		hasRestrictions := false
		for _, j := range jars {
			if len(j.Version.ArtifactVersion) > 1024 {
				problem = fmt.Errorf("artifact version size limit")
				break
			}
			for _, ch := range j.Version.ArtifactVersion {
				if ch > 127 {
					problem = fmt.Errorf("unicode artifact version ordering is not bound")
					break
				}
			}
			if problem != nil {
				break
			}
			var r mavenRange
			r, problem = parseMavenRange(j.Version.Range)
			if problem != nil {
				break
			}
			// A recommended version combined with restrictions requires the
			// loader's restrictRanges semantics, rather than guessing precedence.
			if r.recommended != nil {
				if recommended != nil && compareMaven(recommended, r.recommended) != 0 {
					problem = fmt.Errorf("conflicting recommended versions")
					break
				}
				recommended = r.recommended
			} else {
				hasRestrictions = true
			}
			ranges = append(ranges, r)
		}
		if recommended != nil && hasRestrictions {
			problem = fmt.Errorf("mixed recommended/restricted ranges require loader selection")
		}
		var eligible []string
		if problem == nil {
			for version := range versions {
				v := mavenVersion(version)
				match := true
				for _, r := range ranges {
					match = match && r.contains(v)
				}
				if recommended != nil {
					match = match && compareMaven(v, recommended) == 0
				}
				if match {
					eligible = append(eligible, version)
				}
			}
			// Like the loader, pick the highest version every range accepts.
			slices.SortFunc(eligible, func(a, b string) int { return compareMaven(mavenVersion(b), mavenVersion(a)) })
			if len(eligible) == 0 {
				problem = fmt.Errorf("%d eligible versions require loader selection", len(eligible))
			}
		}
		if problem != nil {
			for _, j := range jars {
				c.selectionErrors[j.source] = fmt.Sprintf("JarJar %s: %v", id, problem)
			}
			continue
		}
		for _, j := range jars {
			if j.Version.ArtifactVersion != eligible[0] {
				excluded[j.source] = true
			}
		}
	}
	if len(excluded) == 0 {
		return
	}
	c.archives = slices.DeleteFunc(c.archives, func(a Archive) bool { return excluded[a.Name] })
	for name, entries := range c.entries {
		entries = slices.DeleteFunc(entries, func(e classEntry) bool { return excluded[e.source] })
		if len(entries) == 0 {
			delete(c.entries, name)
		} else {
			c.entries[name] = entries
		}
	}
}
