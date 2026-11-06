package nativeui

import (
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestNearestHitPicksClosestToOrigin(t *testing.T) {
	hits := []launcher.FeatureHit{{ID: "minecraft:village_plains", X: 2000, Z: 0}, {ID: "minecraft:village_plains", X: -300, Z: 400}, {ID: "minecraft:village_plains", X: 900, Z: 900}}
	got, ok := nearestHit(hits, 100, 100)
	if !ok || got.X != -300 || got.Z != 400 {
		t.Fatalf("nearestHit = %+v %v, want X -300 Z 400", got, ok)
	}
	if _, ok := nearestHit(nil, 0, 0); ok {
		t.Fatal("nearestHit(nil) reported a hit")
	}
}
