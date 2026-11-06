package launcher

import "context"

type seedMapProgressKey struct{}

// SeedMapProgress receives coarse open stages: "prepare", "sources",
// "regions", "register" (detail = mod ID), "biomes". Called from the worker.
type SeedMapProgress func(stage, detail string)

// WithSeedMapProgress attaches a progress callback to an open request.
func WithSeedMapProgress(ctx context.Context, f SeedMapProgress) context.Context {
	return context.WithValue(ctx, seedMapProgressKey{}, f)
}

func seedMapProgress(ctx context.Context, stage, detail string) {
	if f, ok := ctx.Value(seedMapProgressKey{}).(SeedMapProgress); ok && f != nil {
		f(stage, detail)
	}
}
