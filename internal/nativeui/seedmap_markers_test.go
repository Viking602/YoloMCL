package nativeui

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestSeedMapMarkerGroupsDoNotOverlap(t *testing.T) {
	var hits []launcher.FeatureHit
	for i := range 240 {
		hits = append(hits, launcher.FeatureHit{ID: fmt.Sprintf("test:site_%d", i), X: 20 + i%20*9, Z: 20 + i/20*11})
	}
	for _, scale := range []float32{1, 1.5, 2} {
		point := func(x, z int) (float32, float32) { return float32(x) * scale, float32(z) * scale }
		selected := hits[151]
		markers := layoutSeedMapMarkers(hits, &selected, 300*scale, 200*scale, scale, point)
		seen := map[string]bool{}
		for i, marker := range markers {
			for _, hit := range marker.hits {
				if seen[hit.ID] {
					t.Fatal("a structure appeared in multiple groups")
				}
				seen[hit.ID] = true
			}
			for _, other := range markers[i+1:] {
				if math.Abs(float64(marker.x-other.x)) < float64(34*scale) && math.Abs(float64(marker.y-other.y)) < float64(34*scale) {
					t.Fatal("marker badges overlap")
				}
			}
		}
		if len(seen) != len(hits) || len(markers) >= len(hits) || !markers[0].selected || markers[0].hits[0] != selected {
			t.Fatalf("grouping lost structures or selected anchor: %d/%d", len(seen), len(hits))
		}
		reversed := slices.Clone(hits)
		slices.Reverse(reversed)
		if got := layoutSeedMapMarkers(reversed, &selected, 300*scale, 200*scale, scale, point); !reflect.DeepEqual(markers, got) {
			t.Fatal("tile delivery order changed marker grouping")
		}
	}
}

func TestSeedMapMarkerGroupMembersRemainAccessible(t *testing.T) {
	a, tt, _ := worldsTester(t)
	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("seedmap.open"))
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	tt.Advance(400 * time.Millisecond)
	v := a.seedMap
	hits := []launcher.FeatureHit{
		{ID: "minecraft:villages", X: 128, Z: 64, Y: 64, Confidence: "candidate"},
		{ID: "minecraft:desert_pyramids", X: 128, Z: 64, Y: 64, Confidence: "candidate"},
		{ID: "minecraft:shipwrecks", X: 136, Z: 72, Y: 64, Confidence: "candidate"},
	}
	v.frame.Hits, v.hits, v.markerLayout = hits, hits, seedMapMarkerLayout{}
	tt.Frame()
	if len(v.markerLayout.markers) != 1 {
		t.Fatalf("overlapping fixtures did not form one group: %d", len(v.markerLayout.markers))
	}
	marker := v.markerLayout.markers[0]
	label := fmt.Sprintf(a.L("seedmap.marker_group"), len(marker.hits), marker.hits[0].X, marker.hits[0].Z)
	clickUI(t, tt, label)
	if len(v.markerGroup) != len(hits) {
		t.Fatal("group click did not expose every member")
	}
	for _, hit := range hits {
		clickUI(t, tt, a.seedMapHitLabel(hit))
		if v.selected == nil || *v.selected != hit {
			t.Fatal("coincident structure could not be selected")
		}
	}
	snapshotUI(t, tt, "seedmap-marker-group")
	clickUI(t, tt, a.L("seedmap.hide_markers"))
	if _, exists := tt.Find(label); exists || len(v.markerGroup) != 0 {
		t.Fatal("hidden group still intercepts map clicks")
	}
	// Map dragging stays available when its structure overlay is hidden.
	stage, ok := tt.Find(a.L("seedmap.canvas"))
	if !ok {
		t.Fatal("missing canvas")
	}
	x, y := stage.X+20, stage.Y+20
	before := v.viewport
	tt.Press(x, y)
	tt.Move(x+40, y+20)
	tt.Release(x+40, y+20)
	if v.viewport.X == before.X || v.viewport.Z == before.Z {
		t.Fatal("hidden markers blocked panning")
	}
}
