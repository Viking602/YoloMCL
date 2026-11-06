package modjar

import (
	"context"
	"testing"
)

func TestRegistrationDiscoveryUsesTypedReferences(t *testing.T) {
	ctx := context.Background()
	for _, enabled := range []bool{false, true} {
		d, err := DiscoverTerraBlender(ctx, regionFixture(t, "optional_provider", enabled, false))
		if err != nil || len(d.RegionClasses) != 1 || len(d.Callers) != 1 {
			t.Fatalf("discovery %+v: %v", d, err)
		}
	}
	c := &fixtureClass{}
	code := append(c.string("terrablender/api/Regions.register"), 87, 177)
	c.method("log", "()V", 9, code)
	z := fixtureArchive(t, map[string][]byte{"example/Unrelated.class": c.bytes("example/Unrelated", "java/lang/Object")})
	d, err := DiscoverTerraBlender(ctx, z)
	if err != nil || d.HasRegions() {
		t.Fatalf("mere string became a provider: %+v %v", d, err)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := DiscoverTerraBlender(ctx, z); err != context.Canceled {
		t.Fatal(err)
	}
}
