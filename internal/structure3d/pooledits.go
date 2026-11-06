package structure3d

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// PoolEdit is a change mod code makes to a loaded template pool when the
// server starts, after data packs have loaded: StructureTemplatePool's
// templates list gets Weight copies of Element appended (the common
// "addBuildingToPool" pattern), or, with MergeFrom set, every expanded
// template of another pool appended (templates.addAll). Edits apply in the
// order they were added, which is the order the game's listeners ran.
type PoolEdit struct {
	Pool      string
	Element   json.RawMessage
	Weight    int
	MergeFrom string
	Source    string // the mod whose code makes the edit, for logs
	// SelfAppend is TofuCraft's JigsawHelper.registerJigsaw (ServerStarted):
	// Weight copies go into templates, then templates.addAll(templates) and
	// rawTemplates.addAll(rawTemplates) double both lists; the element never
	// reaches rawTemplates. Lithostitched compiles rawTemplates into its own
	// list just before loadLevel, after ServerAboutToStart edits but before
	// ServerStarted, so its generator sees none of this: raw is left as is.
	SelfAppend bool `json:",omitempty"`
}

// AddPoolEdits records edits for PlaceJigsaw to apply on top of the data.
func (s *Store) AddPoolEdits(e ...PoolEdit) {
	s.mu.Lock()
	s.poolEdits = append(s.poolEdits, e...)
	s.mu.Unlock()
}

// SetPoolEdits replaces the recorded edits (tests trying other mod sets
// and registration orders).
func (s *Store) SetPoolEdits(e []PoolEdit) {
	s.mu.Lock()
	s.poolEdits = append([]PoolEdit(nil), e...)
	s.mu.Unlock()
}

// PoolEdits returns the recorded edits in application order.
func (s *Store) PoolEdits() []PoolEdit {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]PoolEdit(nil), s.poolEdits...)
}

func (pl *jigsawPlacer) applyPoolEdits(p *jPool) error {
	for _, e := range pl.store.PoolEdits() {
		if normID(e.Pool) != normID(p.id) {
			continue
		}
		if e.MergeFrom != "" {
			src, err := pl.pool(normID(e.MergeFrom))
			if err != nil {
				return err
			}
			if src != nil {
				p.raw = append(p.raw, src.raw...)
				p.templates = append(p.templates, src.templates...)
			}
			continue
		}
		el, err := parsePoolElement(e.Element, "")
		if err != nil {
			return fmt.Errorf("structure3d: pool %s edit from %s: %w", p.id, e.Source, err)
		}
		if e.SelfAppend {
			for n := 0; n < e.Weight; n++ {
				p.templates = append(p.templates, el)
			}
			p.templates = append(p.templates, p.templates...)
			continue
		}
		p.raw = append(p.raw, weighted{el, e.Weight})
		for n := 0; n < e.Weight; n++ {
			p.templates = append(p.templates, el)
		}
	}
	return nil
}

// SetLithostitchedJigsaw switches PlaceJigsaw to Lithostitched's
// AlternateJigsawGenerator, which replaces vanilla jigsaw placement whenever
// Lithostitched is loaded with breaks_seed_parity=true (its default).
func (s *Store) SetLithostitchedJigsaw(on bool) { s.lithoShuffle = on }

// LithostitchedJigsaw reports whether SetLithostitchedJigsaw(true) was called.
func (s *Store) LithostitchedJigsaw() bool { return s.lithoShuffle }

// lithoChildren is StructurePoolGenerator.findAndTestChildCandidates: try the
// pool, and only when nothing fits, recurse into its fallback.
func (pl *jigsawPlacer) lithoChildren(id string, chain []string, depth int, first bool, try func([]*jElement) bool) {
	if id == "minecraft:empty" || slices.Contains(chain, id) {
		return
	}
	chain = append(chain, id)
	p, err := pl.pool(id)
	if err != nil {
		pl.err = err
		return
	}
	if p == nil {
		return
	}
	src := p
	if depth == pl.maxDepth && first {
		if src, err = pl.pool(normID(p.fallback)); err != nil || src == nil {
			pl.err = err
			return
		}
	}
	cands := lithoShuffle(src.raw, pl.rnd)
	if len(cands) == 0 || try(cands) || pl.err != nil {
		return
	}
	pl.lithoChildren(normID(p.fallback), chain, depth, false, try)
}

// lithoShuffle is LithostitchedTemplates.shuffle: one nextFloat per raw entry,
// key -(u^(1/w)) with 1/w in float, stable ascending sort.
func lithoShuffle(raw []weighted, r *worldgen.JavaRandom) []*jElement {
	type keyed struct {
		el  *jElement
		key float64
	}
	ks := make([]keyed, len(raw))
	for i, w := range raw {
		u := r.NextFloat()
		ks[i] = keyed{w.el, -math.Pow(float64(u), float64(float32(1)/float32(w.w)))}
	}
	sort.SliceStable(ks, func(i, j int) bool { return ks[i].key < ks[j].key })
	out := make([]*jElement, len(ks))
	for i, k := range ks {
		out[i] = k.el
	}
	return out
}
