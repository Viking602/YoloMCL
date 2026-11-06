package structure3d

import (
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
)

var waterTint = color.RGBA{0x3f, 0x76, 0xe4, 255}

func fluidKind(id string) string {
	switch id {
	case "minecraft:water", "minecraft:flowing_water":
		return "water"
	case "minecraft:lava", "minecraft:flowing_lava":
		return "lava"
	}
	return ""
}

// sprite bakes the first scheduled animation frame, not the whole atlas.
// Preview meshes are static; no JAR reads or animation work occur per frame.
func (c *modelCache) sprite(store *Store, id string) image.Image {
	key := "sprite/" + id
	c.mu.Lock()
	img, ok := c.tex[key]
	c.mu.Unlock()
	if ok {
		return img
	}
	img = c.texture(store, id)
	if img != nil {
		ns, name := Namespaced(id)
		if raw, err := store.Read(AssetPath(ns, "textures", name+".png.mcmeta")); err == nil {
			img = firstSpriteFrame(img, raw)
		}
	}
	c.mu.Lock()
	c.tex[key] = img
	c.mu.Unlock()
	return img
}

func firstSpriteFrame(img image.Image, raw []byte) image.Image {
	var meta struct {
		Animation *struct {
			Width, Height int
			Frames        []json.RawMessage
		}
	}
	if json.Unmarshal(raw, &meta) != nil || meta.Animation == nil {
		return img
	}
	b := img.Bounds()
	w, h := meta.Animation.Width, meta.Animation.Height
	switch {
	case w == 0 && h == 0:
		w, h = min(b.Dx(), b.Dy()), min(b.Dx(), b.Dy())
	case w == 0:
		w = b.Dx()
	case h == 0:
		h = b.Dy()
	}
	if w <= 0 || h <= 0 || w > b.Dx() || h > b.Dy() || b.Dx()%w != 0 || b.Dy()%h != 0 {
		return nil
	}
	index := 0
	if len(meta.Animation.Frames) > 0 {
		frame := meta.Animation.Frames[0]
		var number *int
		if json.Unmarshal(frame, &number) == nil {
			if number == nil {
				return nil
			}
			index = *number
		} else {
			var entry struct{ Index *int }
			if json.Unmarshal(frame, &entry) != nil || entry.Index == nil {
				return nil
			}
			index = *entry.Index
		}
	}
	nx, ny := b.Dx()/w, b.Dy()/h
	if index < 0 || index >= nx*ny {
		return nil
	}
	frame := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(frame, frame.Bounds(), img, image.Pt(b.Min.X+(index%nx)*w, b.Min.Y+(index/nx)*h), draw.Src)
	return frame
}

func (c *modelCache) fluidSprite(store *Store, kind string, flow bool) image.Image {
	name := kind + "_still"
	if flow {
		name = kind + "_flow"
	}
	key := "fluid/" + name
	c.mu.Lock()
	img, ok := c.tex[key]
	c.mu.Unlock()
	if ok {
		return img
	}
	img = c.sprite(store, "minecraft:block/"+name)
	if img != nil && kind == "water" {
		b := img.Bounds()
		colored := image.NewNRGBA(b)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				p := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				p.R = uint8(int(p.R) * int(waterTint.R) / 255)
				p.G = uint8(int(p.G) * int(waterTint.G) / 255)
				p.B = uint8(int(p.B) * int(waterTint.B) / 255)
				colored.SetNRGBA(x, y, p)
			}
		}
		img = colored
	}
	c.mu.Lock()
	c.tex[key] = img
	c.mu.Unlock()
	return img
}

func (c *modelCache) fluidModel(store *Store, kind string) *ResolvedModel {
	still, flow := c.fluidSprite(store, kind, false), c.fluidSprite(store, kind, true)
	if still == nil {
		still = flow
	}
	if flow == nil {
		flow = still
	}
	if still == nil {
		fallback := waterTint
		if kind == "lava" {
			fallback = color.RGBA{0xe3, 0x6b, 0x1c, 255}
		}
		pixel := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		pixel.Set(0, 0, fallback)
		still, flow = pixel, pixel
	}
	el := cubeAll(still)
	for i := range el.Faces {
		f := &el.Faces[i]
		if f.Dir != "up" && f.Dir != "down" {
			f.Texture = flow
		}
		f.Blend, f.Emissive = kind == "water", kind == "lava"
	}
	return &ResolvedModel{Elements: []Element{el}, Fluid: kind}
}
