package launcher

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// SeedMapSpawn distinguishes saved world/bed positions from climate estimates.
type SeedMapSpawn struct {
	X, Y, Z         int
	Dimension, Kind string
}

// SeedMapSaves reads names/seeds only. Unlike the Worlds management page it does
// not walk region files to measure save sizes or count backup archives.
func (s *Service) SeedMapSaves(ctx context.Context, rootID, versionID string) ([]World, error) {
	saves, _, err := s.worldPaths(rootID, versionID)
	if err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(saves)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()
	f, err := r.Open(".")
	if err != nil {
		return nil, err
	}
	entries, readErr := f.ReadDir(129)
	f.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	if len(entries) > 128 {
		return nil, errors.New("too many seed map saves")
	}
	var result []World
	for _, en := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !en.IsDir() || en.Type()&os.ModeSymlink != 0 || strings.HasPrefix(en.Name(), ".") {
			continue
		}
		world := World{Dir: en.Name(), Name: en.Name()}
		worldRoot, err := r.OpenRoot(en.Name())
		if err == nil {
			level, e := readWorldLevelRoot(ctx, worldRoot)
			worldRoot.Close()
			if e == nil {
				world.RandomSeed, world.HasRandomSeed = level.RandomSeed, level.HasRandomSeed
				if level.Name != "" {
					world.Name = level.Name
				}
			}
		}
		result = append(result, world)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// EstimateSpawn mirrors Climate.SpawnFinder's horizontal climate search. The
// server's final surface/spawn-chunk checks are not available without terrain;
// callers must label this as an area estimate, never a saved spawn position.
func (s *LocalSampler) EstimateSpawn(ctx context.Context) (*SeedMapSpawn, error) {
	g := s.dimensions["minecraft:overworld"].engine
	if g == nil {
		return nil, nil
	}
	position, err := g.EstimateSpawn(ctx)
	if err != nil || position == nil {
		return nil, err
	}
	return &SeedMapSpawn{X: position.X, Z: position.Z, Dimension: "minecraft:overworld", Kind: "estimate"}, nil
}
