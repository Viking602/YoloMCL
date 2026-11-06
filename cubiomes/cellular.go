package cubiomes

import (
	"encoding/json"
	"errors"
	"math"
)

// FastNoiseLite's 3D cellular CellValue path, as used by Lithostitched regions.
// Coordinates are doubles; vectors, distances and return values are Java floats.
type cellularNoise struct {
	seed, salt        int32
	frequency, jitter float32
}

func (n *cellularNoise) sample(x, y, z float64) float64 {
	x, y, z = x*float64(n.frequency), y*float64(n.frequency), z*float64(n.frequency)
	round := func(v float64) int32 {
		if v >= 0 {
			return int32(v + .5)
		}
		return int32(v - .5)
	}
	xr, yr, zr := round(x), round(y), round(z)
	distance := float32(math.MaxFloat32)
	var closest int32
	jitter := float32(.39614353) * n.jitter
	for dx := int32(-1); dx <= 1; dx++ {
		for dy := int32(-1); dy <= 1; dy++ {
			for dz := int32(-1); dz <= 1; dz++ {
				xi, yi, zi := xr+dx, yr+dy, zr+dz
				hash := (n.seed ^ (xi * 501125321) ^ (yi * 1136930381) ^ (zi * 1720413743)) * 0x27d4eb2d
				i := uint32(hash) & 1020
				vx := float32(float64(xi)-x) + float32(cellularVectors[i]*jitter)
				vy := float32(float64(yi)-y) + float32(cellularVectors[i|1]*jitter)
				vz := float32(float64(zi)-z) + float32(cellularVectors[i|2]*jitter)
				d := float32(float32(vx*vx)+float32(vy*vy)) + float32(vz*vz)
				if d < distance {
					distance, closest = d, hash
				}
			}
		}
	}
	return float64(float32(closest) * float32(1.0/2147483648))
}

func (c *densityCompiler) compileCellular(fields map[string]json.RawMessage, n *densityNode, depth int) error {
	raw := fields["config"]
	var id string
	if json.Unmarshal(raw, &id) == nil {
		raw = c.docs["lithostitched_fast_noise_config/"+seedMapID(id)].Data
	}
	var config struct {
		Type      string
		Frequency *float32
		Salt      *int32
		Jitter    *float32
		Distance  string `json:"distance_function"`
		Return    string `json:"return_type"`
	}
	if json.Unmarshal(raw, &config) != nil || config.Type != "lithostitched:cellular" || config.Frequency == nil || config.Jitter == nil || config.Distance != "euclidean" && config.Distance != "euclidean_squared" || config.Return != "cell_value" || *config.Frequency < -1000 || *config.Frequency > 1000 || *config.Jitter < -1 || *config.Jitter > 1 {
		return &CompileError{"density_unsupported", "Lithostitched cellular config: " + id}
	}
	var salt int32
	if config.Salt != nil {
		salt = *config.Salt
	}
	n.cellular = &cellularNoise{seed: int32(c.seed) + salt, salt: salt, frequency: *config.Frequency, jitter: *config.Jitter}
	n.p, n.q = 1, 1
	for key, dst := range map[string]*float64{"xz_scale": &n.p, "y_scale": &n.q} {
		if v, ok := fields[key]; ok {
			value, err := densityNumber(v)
			if err != nil || math.Abs(value) > 1000 {
				return errors.New("invalid fast-noise scale")
			}
			*dst = value
		}
	}
	for key, dst := range map[string]**densityNode{"shift_x": &n.a, "shift_y": &n.b, "shift_z": &n.c} {
		raw := fields[key]
		if len(raw) == 0 {
			raw = json.RawMessage(`0`)
		}
		var err error
		*dst, err = c.compile(raw, depth+1)
		if err != nil {
			return err
		}
	}
	return nil
}
