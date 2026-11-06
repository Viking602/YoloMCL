package structure3d

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"strings"
	"sync"
)

// Face is one textured quad of a block model element.
type Face struct {
	Dir            string // down up north south west east
	Texture        image.Image
	TintIndex      int // -1 none, 0 grass, 1 foliage, ...
	U0, V0, U1, V1 float64
}

// Element is a cuboid from a block model, in 0..16 model space.
type Element struct {
	From, To [3]float64
	Faces    []Face
}

// ResolvedModel is the expanded model for one block state.
type ResolvedModel struct {
	Elements []Element
	Ambient  bool
}

type modelCache struct {
	mu    sync.Mutex
	tex   map[string]image.Image
	model map[string]*ResolvedModel
}

func newModelCache() *modelCache {
	return &modelCache{tex: map[string]image.Image{}, model: map[string]*ResolvedModel{}}
}

func (c *modelCache) texture(store *Store, id string) image.Image {
	c.mu.Lock()
	if img, ok := c.tex[id]; ok {
		c.mu.Unlock()
		return img
	}
	c.mu.Unlock()
	ns, name := Namespaced(id)
	data, err := store.Read(AssetPath(ns, "textures", name+".png"))
	if err != nil {
		return nil
	}
	img, err := decodePNG(data)
	if err != nil {
		return nil
	}
	c.mu.Lock()
	c.tex[id] = img
	c.mu.Unlock()
	return img
}

func decodePNG(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

type rawModel struct {
	Parent   string            `json:"parent"`
	Textures map[string]string `json:"textures"`
	Elements []rawElement      `json:"elements"`
	Ambient  *bool             `json:"ambientocclusion"`
}

type rawElement struct {
	From  [3]float64         `json:"from"`
	To    [3]float64         `json:"to"`
	Faces map[string]rawFace `json:"faces"`
}

type rawFace struct {
	Texture string    `json:"texture"`
	UV      []float64 `json:"uv"`
	Tint    *int      `json:"tintindex"`
}

func (c *modelCache) resolve(store *Store, state BlockState) *ResolvedModel {
	key := state.Name
	for k, v := range state.Properties {
		key += "," + k + "=" + v
	}
	c.mu.Lock()
	if m, ok := c.model[key]; ok {
		c.mu.Unlock()
		return m
	}
	c.mu.Unlock()
	paths := c.blockstateModels(store, state)
	if len(paths) == 0 {
		paths = []string{"block/" + stripNS(state.Name)}
	}
	out := &ResolvedModel{Ambient: true}
	var lastTex map[string]string
	for _, modelPath := range paths {
		tex, elems, ambient := c.expandModel(store, modelPath, nil)
		out.Ambient = ambient
		lastTex = tex
		for _, e := range elems {
			el := Element{From: e.From, To: e.To}
			for dir, f := range e.Faces {
				tid := resolveTex(tex, f.Texture)
				img := c.texture(store, tid)
				if img == nil {
					img = missingTexture()
				}
				u0, v0, u1, v1 := 0.0, 0.0, 16.0, 16.0
				if len(f.UV) == 4 {
					u0, v0, u1, v1 = f.UV[0], f.UV[1], f.UV[2], f.UV[3]
				}
				tint := -1
				if f.Tint != nil {
					tint = *f.Tint
				}
				el.Faces = append(el.Faces, Face{Dir: dir, Texture: img, TintIndex: tint, U0: u0, V0: v0, U1: u1, V1: v1})
			}
			if len(el.Faces) > 0 {
				out.Elements = append(out.Elements, el)
			}
		}
	}
	// Fallback: full cube with particle/all texture when model empty.
	if len(out.Elements) == 0 {
		img := c.texture(store, firstTex(lastTex, "all", "particle", "side", "end", "texture"))
		if img == nil {
			img = missingTexture()
		}
		out.Elements = append(out.Elements, cubeAll(img))
	}
	c.mu.Lock()
	c.model[key] = out
	c.mu.Unlock()
	return out
}

func (c *modelCache) blockstateModels(store *Store, state BlockState) []string {
	ns, name := Namespaced(state.Name)
	data, err := store.Read(AssetPath(ns, "blockstates", name+".json"))
	if err != nil {
		return nil
	}
	var bs struct {
		Variants  map[string]json.RawMessage `json:"variants"`
		Multipart []struct {
			When  map[string]any  `json:"when"`
			Apply json.RawMessage `json:"apply"`
		} `json:"multipart"`
	}
	if json.Unmarshal(data, &bs) != nil {
		return nil
	}
	pickAll := func(raw json.RawMessage) []string {
		var one struct {
			Model string `json:"model"`
		}
		if json.Unmarshal(raw, &one) == nil && one.Model != "" {
			return []string{one.Model}
		}
		var many []struct {
			Model string `json:"model"`
		}
		if json.Unmarshal(raw, &many) == nil {
			out := make([]string, 0, len(many))
			for _, m := range many {
				if m.Model != "" {
					out = append(out, m.Model)
				}
			}
			return out
		}
		return nil
	}
	if len(bs.Variants) > 0 {
		best, bestScore := "", -1
		for k, raw := range bs.Variants {
			score := matchProps(k, state.Properties)
			if score > bestScore {
				bestScore, best = score, ""
				if ms := pickAll(raw); len(ms) > 0 {
					best = ms[0]
				}
			}
		}
		if best != "" {
			return []string{best}
		}
		return nil
	}
	if len(bs.Multipart) > 0 {
		var out []string
		for _, part := range bs.Multipart {
			if len(part.When) > 0 && !multipartWhenMatches(part.When, state.Properties) {
				continue
			}
			out = append(out, pickAll(part.Apply)...)
		}
		return out
	}
	return nil
}

func multipartWhenMatches(when map[string]any, props map[string]string) bool {
	for k, raw := range when {
		want := fmt.Sprint(raw)
		// OR lists encoded as "true|false"
		ok := false
		for _, opt := range strings.Split(want, "|") {
			if props[k] == opt {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func matchProps(variant string, props map[string]string) int {
	if variant == "" || variant == "normal" {
		return 0
	}
	score := 0
	for _, part := range strings.Split(variant, ",") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		if props[k] == v {
			score++
		} else {
			return -1
		}
	}
	return score
}

func (c *modelCache) expandModel(store *Store, id string, child map[string]string) (map[string]string, []rawElement, bool) {
	ns, name := Namespaced(id)
	if !strings.Contains(name, "/") {
		name = "block/" + name
	}
	data, err := store.Read(AssetPath(ns, "models", name+".json"))
	tex := map[string]string{}
	for k, v := range child {
		tex[k] = v
	}
	ambient := true
	if err != nil {
		return tex, nil, ambient
	}
	var m rawModel
	if json.Unmarshal(data, &m) != nil {
		return tex, nil, ambient
	}
	for k, v := range m.Textures {
		if _, ok := tex[k]; !ok {
			tex[k] = v
		}
	}
	if m.Ambient != nil {
		ambient = *m.Ambient
	}
	elems := m.Elements
	if m.Parent != "" {
		pt, pe, pa := c.expandModel(store, m.Parent, tex)
		tex = pt
		if len(elems) == 0 {
			elems = pe
		}
		ambient = pa
	}
	return tex, elems, ambient
}

func resolveTex(tex map[string]string, ref string) string {
	for i := 0; i < 8; i++ {
		if strings.HasPrefix(ref, "#") {
			next, ok := tex[ref[1:]]
			if !ok {
				return ref
			}
			ref = next
			continue
		}
		return ref
	}
	return ref
}

func firstTex(tex map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := tex[k]; ok {
			return resolveTex(tex, v)
		}
	}
	for _, v := range tex {
		return resolveTex(tex, v)
	}
	return ""
}

func stripNS(id string) string {
	_, p := Namespaced(id)
	return p
}

func cubeAll(img image.Image) Element {
	faces := []Face{}
	for _, d := range []string{"down", "up", "north", "south", "west", "east"} {
		faces = append(faces, Face{Dir: d, Texture: img, TintIndex: -1, U0: 0, V0: 0, U1: 16, V1: 16})
	}
	return Element{From: [3]float64{0, 0, 0}, To: [3]float64{16, 16, 16}, Faces: faces}
}

// BiomeTint returns grass/foliage tint. Default temperate plains greens.
func BiomeTint(index int) color.RGBA {
	switch index {
	case 0: // grass
		return color.RGBA{0x91, 0xbd, 0x59, 255}
	case 1: // foliage
		return color.RGBA{0x77, 0xab, 0x2f, 255}
	default:
		return color.RGBA{255, 255, 255, 255}
	}
}

func tinted(img image.Image, tint color.RGBA) image.Image {
	if tint.R == 255 && tint.G == 255 && tint.B == 255 {
		return img
	}
	b := img.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, img, b.Min, draw.Src)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := dst.RGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			dst.SetRGBA(x, y, color.RGBA{
				uint8(int(c.R) * int(tint.R) / 255),
				uint8(int(c.G) * int(tint.G) / 255),
				uint8(int(c.B) * int(tint.B) / 255),
				c.A,
			})
		}
	}
	return dst
}

// missingTexture is the magenta/black checker vanilla uses when a PNG is absent.
// Cached once; faces that fail store.Read paint this instead of vanishing.
func missingTexture() image.Image {
	missingTexOnce.Do(func() {
		img := image.NewRGBA(image.Rect(0, 0, 16, 16))
		mag := color.RGBA{R: 0xF8, G: 0x00, B: 0xF8, A: 0xFF}
		blk := color.RGBA{A: 0xFF}
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				if (x/8+y/8)%2 == 0 {
					img.SetRGBA(x, y, mag)
				} else {
					img.SetRGBA(x, y, blk)
				}
			}
		}
		missingTexImg = img
	})
	return missingTexImg
}

var (
	missingTexOnce sync.Once
	missingTexImg  image.Image
)
