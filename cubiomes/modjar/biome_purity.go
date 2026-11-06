package modjar

import "strings"

// A conservative effect check, independent of sampled values: dead branches
// with mutable state or unknown calls are rejected too. This lets a captured
// registry-only source outlive the loader without pretending to initialize
// unrelated gameplay registries. Broader stateful sources require full loading.
func proveBiomeQuery(v *machine, ref reference, seen map[reference]bool) {
	if seen[ref] {
		return
	}
	if len(seen) >= 1024 {
		fail("biome query graph budget")
	}
	seen[ref] = true
	if pureBiomeNative(ref) {
		return
	}
	if !v.fetchClass(ref.owner) {
		fail("unresolved biome API %s.%s%s", ref.owner, ref.name, ref.desc)
	}
	c := v.load(ref.owner)
	var found *method
	for i := range c.methods {
		m := &c.methods[i]
		if m.name == ref.name && m.desc == ref.desc {
			found = m
			break
		}
	}
	if found == nil {
		if c.super != "" {
			ref.owner = c.super
			proveBiomeQuery(v, ref, seen)
			return
		}
		fail("missing biome query method")
	}
	if found.origin != nil {
		c = found.origin
	}
	if len(found.injects) > 0 {
		fail("injected biome query method %s.%s", c.name, found.name)
	}
	if found.access&0x120 != 0 {
		fail("native or synchronized biome query unsupported")
	}
	children := methodReferencesChecked(c, *found, func(op byte, offset int) {
		switch {
		case op == 179 || op == 181 || op >= 79 && op <= 86 || op == 194 || op == 195 || op == 186:
			fail("mutable or dynamic biome query instruction %d in %s.%s", op, c.name, found.name)
		case op == 178:
			fail("biome query depends on static initialization state in %s.%s", c.name, found.name)
		case op == 180:
			r := reader{b: found.code, p: offset}
			fref := c.ref(r.u2())
			owner := v.load(fref.owner)
			valid := false
			for _, f := range owner.fields {
				if f.name == fref.name && f.desc == fref.desc {
					primitive := len(f.desc) == 1 || f.desc == "Ljava/lang/String;" || f.desc == "Lnet/minecraft/core/Holder;"
					valid = f.access&0x10 != 0 && primitive
					break
				}
			}
			if !valid {
				fail("biome query reads non-immutable field %s.%s", fref.owner, fref.name)
			}
		case op == 187:
			r := reader{b: found.code, p: offset}
			if name := c.className(r.u2()); name != densityClass+"$SinglePointContext" {
				fail("unbound biome query allocation %s", name)
			}
		}
	})
	for _, child := range children {
		proveBiomeQuery(v, child, seen)
	}
}

func pureBiomeNative(r reference) bool {
	if r.owner == climateSamplerClass && r.desc == "()L"+densityClass+";" {
		for _, name := range []string{"temperature", "humidity", "continentalness", "erosion", "depth", "weirdness"} {
			if r.name == name {
				return true
			}
		}
	}
	if r.owner == densityClass && r.name == "compute" && r.desc == "(L"+densityClass+"$FunctionContext;)D" {
		return true
	}
	if r.owner == densityClass+"$SinglePointContext" && r.name == "<init>" && r.desc == "(III)V" {
		return true
	}
	if r.owner == simplexClass && r.name == "getValue" && r.desc == "(DD)D" {
		return true
	}
	if r.owner == "java/lang/Math" {
		return strings.Contains("|floorDiv(II)I|abs(I)I|abs(F)F|min(FF)F|max(FF)F|", "|"+r.name+r.desc+"|")
	}
	if r.owner == "net/minecraft/util/Mth" {
		return strings.Contains("|sqrt(F)F|abs(F)F|clamp(FFF)F|", "|"+r.name+r.desc+"|")
	}
	if r.owner == "net/minecraft/core/QuartPos" && r.desc == "(I)I" && (r.name == "fromBlock" || r.name == "toBlock") {
		return true
	}
	return r.owner == "net/minecraft/core/SectionPos" && r.name == "blockToSectionCoord" && r.desc == "(I)I"
}
