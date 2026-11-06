package cubiomes

import (
	"encoding/json"
	"math"
	"sync"
)

// perlinSimplex is PerlinSimplexNoise: levels[0] is the highest octave.
type perlinSimplex struct {
	levels       []*[256]byte
	input, value float64
}

func newPerlinSimplex(seed int64, octaves ...int) *perlinSimplex {
	r := newNoiseRandom(seed, true)
	first, last := octaves[0], octaves[len(octaves)-1]
	has := map[int]bool{}
	for _, o := range octaves {
		has[o] = true
	}
	n := -first + last + 1
	p := &perlinSimplex{levels: make([]*[256]byte, n)}
	base := newImprovedNoise(r).p
	if last >= 0 && last < n && has[0] {
		p.levels[last] = &base
	}
	for m := last + 1; m < n; m++ {
		if has[last-m] {
			lv := newImprovedNoise(r).p
			p.levels[m] = &lv
		} else {
			r.skipLegacy(262)
		}
	}
	// Positive octaves (last > 0) reseed from the base noise; no vanilla
	// temperature noise uses them.
	p.input = math.Pow(2, float64(last))
	p.value = 1 / (math.Pow(2, float64(n)) - 1)
	return p
}

func (p *perlinSimplex) get(x, z float64) float64 {
	d, e, f := 0.0, p.input, p.value
	for _, lv := range p.levels {
		if lv != nil {
			d += simplex2D(lv, x*e, z*e) * f
		}
		e /= 2
		f *= 2
	}
	return d
}

var (
	temperatureNoise       = newPerlinSimplex(1234, 0)
	frozenTemperatureNoise = newPerlinSimplex(3456, -2, -1, 0)
	biomeInfoNoise         = newPerlinSimplex(2345, 0)
)

type biomeClimate struct {
	temp   float32
	frozen bool
}

type biomeClimateTable struct {
	docs map[string]Document
	mu   sync.Mutex
	m    map[string]biomeClimate
}

func biomeClimates(docs map[string]Document) *biomeClimateTable {
	return &biomeClimateTable{docs: docs, m: map[string]biomeClimate{}}
}

func (t *biomeClimateTable) get(id string) biomeClimate {
	t.mu.Lock()
	defer t.mu.Unlock()
	if c, ok := t.m[id]; ok {
		return c
	}
	var d struct {
		Temperature float32 `json:"temperature"`
		Modifier    string  `json:"temperature_modifier"`
	}
	d.Temperature = 0.5
	_ = json.Unmarshal(t.docs["biome/"+id].Data, &d)
	c := biomeClimate{d.Temperature, d.Modifier == "frozen"}
	t.m[id] = c
	return c
}

// coldEnoughToSnow is Biome.coldEnoughToSnow (1.21.1): the height-adjusted
// temperature, with the frozen modifier, below 0.15.
func (t *biomeClimateTable) coldEnoughToSnow(id string, x, y, z int) bool {
	c := t.get(id)
	temp := c.temp
	if c.frozen {
		d := frozenTemperatureNoise.get(float64(x)*.05, float64(z)*.05) * 7
		e := biomeInfoNoise.get(float64(x)*.2, float64(z)*.2)
		if d+e < .3 && biomeInfoNoise.get(float64(x)*.09, float64(z)*.09) < .8 {
			temp = .2
		}
	}
	if y > 80 {
		g := float32(temperatureNoise.get(float64(float32(x)/8), float64(float32(z)/8)) * 8)
		temp = temp - (g+float32(y)-80)*float32(.05)/40
	}
	return temp < .15
}
