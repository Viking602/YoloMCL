package structure3d

import "testing"

// TestTerrainCarvedByStructure: terrain columns whose top the structure
// sets to air drop to the carved floor; others keep their height.
func TestTerrainCarvedByStructure(t *testing.T) {
	m := &Mesh{carved: map[[2]int]map[int]bool{{0, 0}: {70: true, 69: true, 68: true}, {1, 0}: {60: true}}}
	if got := m.carveTop(0, 0, 70); got != 67 {
		t.Fatalf("carved column top %d, want 67", got)
	}
	if got := m.carveTop(1, 0, 70); got != 70 {
		t.Fatalf("buried air must not lower the column: %d", got)
	}
	if got := m.carveTop(5, 5, 70); got != 70 {
		t.Fatalf("untouched column %d", got)
	}
}
