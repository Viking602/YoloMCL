package modjar

import (
	"encoding/json"
	"strconv"
	"strings"
)

// selectFabricMods applies Fabric Loader's single-version rule: every mod ID
// (root or jar-in-jar) resolves to one candidate, the highest semantic version.
// Losing candidates and the libraries they nest are removed from the class
// space. Unparseable versions leave all candidates, so conflicts still fail.
func (c *Classpath) selectFabricMods() {
	type candidate struct {
		name    string
		version []string
	}
	byID := map[string][]candidate{}
	idOf := map[string]string{}
	var order []string
	for _, a := range c.archives {
		for _, f := range a.Reader.File {
			if f.Name != "fabric.mod.json" {
				continue
			}
			b, err := archiveBytes(c.ctx, f, 2<<20)
			if err != nil {
				return
			}
			var meta struct {
				ID      string `json:"id"`
				Version string `json:"version"`
			}
			if json.Unmarshal(loaderJSON(b), &meta) != nil || meta.ID == "" {
				continue
			}
			if byID[meta.ID] == nil {
				order = append(order, meta.ID)
			}
			idOf[a.Name] = meta.ID
			byID[meta.ID] = append(byID[meta.ID], candidate{a.Name, fabricVersion(meta.Version)})
		}
	}
	excluded := map[string]bool{}
	dropped := func(name string) bool {
		for n := range excluded {
			if name == n || strings.HasPrefix(name, n+"!/") {
				return true
			}
		}
		return false
	}
	// A candidate nested in a losing jar is never offered. Decide an ID only
	// after every enclosing jar's own ID is decided.
	decided := map[string]bool{}
	ready := func(name string) bool {
		parts := strings.Split(name, "!/")
		for i := 1; i < len(parts); i++ {
			if id := idOf[strings.Join(parts[:i], "!/")]; id != "" && !decided[id] {
				return false
			}
		}
		return true
	}
	for progress := true; progress; {
		progress = false
		for _, id := range order {
			if decided[id] {
				continue
			}
			var cs []candidate
			wait := false
			for _, cand := range byID[id] {
				if dropped(cand.name) {
					continue
				}
				if !ready(cand.name) {
					wait = true
					break
				}
				cs = append(cs, cand)
			}
			if wait {
				continue
			}
			decided[id] = true
			progress = true
			if len(cs) < 2 {
				continue
			}
			best := -1
			for i, cand := range cs {
				if cand.version == nil {
					best = -1
					break
				}
				if c := compareFabricVersion(cand.version, cs[best0(best)].version); best < 0 || c > 0 || c == 0 && depth(cand.name) < depth(cs[best].name) {
					best = i
				}
			}
			if best < 0 {
				continue
			}
			for i, cand := range cs {
				if i != best {
					excluded[cand.name] = true
				}
			}
		}
	}
	if len(excluded) == 0 {
		return
	}
	kept := c.archives[:0]
	for _, a := range c.archives {
		if !dropped(a.Name) {
			kept = append(kept, a)
		}
	}
	c.archives = kept
	for name, entries := range c.entries {
		out := entries[:0]
		for _, e := range entries {
			if !dropped(e.source) {
				out = append(out, e)
			}
		}
		if len(out) == 0 {
			delete(c.entries, name)
		} else {
			c.entries[name] = out
		}
	}
}

// fabricVersion splits a SemVer core (build metadata ignored) into numeric
// components followed by optional pre-release identifiers; nil if invalid.
func best0(i int) int { return max(i, 0) }

func depth(name string) int { return strings.Count(name, "!/") }

func fabricVersion(v string) []string {
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	core, pre, hasPre := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if core == "" {
		return nil
	}
	for _, p := range parts {
		if _, err := strconv.ParseUint(p, 10, 63); err != nil {
			return nil
		}
	}
	out := append([]string{}, parts...)
	out = append(out, "")
	if hasPre {
		out[len(out)-1] = "-" + pre
	}
	return out
}

func compareFabricVersion(a, b []string) int {
	na, nb := len(a)-1, len(b)-1
	for i := 0; i < max(na, nb); i++ {
		var x, y uint64
		if i < na {
			x, _ = strconv.ParseUint(a[i], 10, 63)
		}
		if i < nb {
			y, _ = strconv.ParseUint(b[i], 10, 63)
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	pa, pb := a[na], b[nb]
	switch {
	case pa == pb:
		return 0
	case pa == "":
		return 1
	case pb == "":
		return -1
	}
	return comparePrerelease(pa[1:], pb[1:])
}

func comparePrerelease(a, b string) int {
	xs, ys := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(xs) && i < len(ys); i++ {
		x, ex := strconv.ParseUint(xs[i], 10, 63)
		y, ey := strconv.ParseUint(ys[i], 10, 63)
		switch {
		case ex == nil && ey == nil:
			if x != y {
				if x < y {
					return -1
				}
				return 1
			}
		case ex == nil:
			return -1
		case ey == nil:
			return 1
		default:
			if c := strings.Compare(xs[i], ys[i]); c != 0 {
				return c
			}
		}
	}
	return len(xs) - len(ys)
}
