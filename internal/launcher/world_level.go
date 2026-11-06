package launcher

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

const worldgenDataPath = "data/minecraft/world_gen_settings.dat"

type worldgenMetadataError struct{ cause error }

func (e *worldgenMetadataError) Error() string {
	return fmt.Sprintf("%s: %v", worldgenDataPath, e.cause)
}
func (e *worldgenMetadataError) Unwrap() error { return e.cause }

// A failed companion must invalidate worldgen inputs while allowing save lists
// to retain the display metadata already parsed successfully from level.dat.
func worldLevelMetadataAvailable(err error) bool {
	var companion *worldgenMetadataError
	return err == nil || errors.As(err, &companion)
}

// Save metadata may span level.dat and 26.1+'s world_gen_settings.dat. Every
// caller uses the same bounded reader, confined to this save, without chunks
// or external player data. A corrupt companion never falls back to stale data.
func readWorldLevel(ctx context.Context, dir string) (LevelInfo, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return LevelInfo{}, err
	}
	defer r.Close()
	return readWorldLevelRoot(ctx, r)
}

func readWorldLevelRoot(ctx context.Context, root *os.Root) (LevelInfo, error) {
	f, err := root.Open("level.dat")
	if err != nil {
		return LevelInfo{}, err
	}
	level, err := parseLevelDat(ctx, f)
	f.Close()
	if err != nil {
		return level, err
	}
	f, err = root.Open(worldgenDataPath)
	if errors.Is(err, fs.ErrNotExist) {
		return level, nil
	}
	level.RandomSeed, level.HasRandomSeed = 0, false
	level.Dimensions, level.Generators = nil, nil
	if err != nil {
		return level, &worldgenMetadataError{err}
	}
	defer f.Close()
	settings, err := parseWorldgenDat(ctx, f)
	if err != nil {
		return level, &worldgenMetadataError{err}
	}
	level.RandomSeed, level.HasRandomSeed = settings.RandomSeed, settings.HasRandomSeed
	level.Dimensions, level.Generators = settings.Dimensions, settings.Generators
	return level, nil
}

func parseWorldgenDat(ctx context.Context, r io.Reader) (LevelInfo, error) {
	if err := ctx.Err(); err != nil {
		return LevelInfo{}, err
	}
	gz, err := gzip.NewReader(r)
	if err != nil {
		return LevelInfo{}, err
	}
	defer gz.Close()
	p := &nbtReader{r: &limitedReader{r: bufio.NewReader(modReader{ctx: ctx, r: gz}), n: maxLevelData}}
	var level LevelInfo
	if p.u8() != 10 {
		return level, errors.New("not an NBT compound")
	}
	p.str()
	p.compound(func(path, name string, tag byte) bool {
		if path != "/data" {
			return false
		}
		if name == "seed" && tag == 4 {
			level.RandomSeed, level.HasRandomSeed = int64(p.u64()), true
			return true
		}
		if name == "dimensions" && tag == 10 {
			p.readWorldgenDimensions(&level)
			return true
		}
		return false
	}, "")
	if p.err != nil {
		return level, p.err
	}
	if !level.HasRandomSeed || len(level.Dimensions) == 0 {
		return level, errors.New("missing saved seed or dimensions")
	}
	return level, nil
}
