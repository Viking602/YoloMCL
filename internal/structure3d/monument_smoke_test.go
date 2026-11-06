package structure3d

import "testing"

func TestMonumentBlocksSmoke(t *testing.T) {
	m := MonumentBlocks(1, 0, 0, 63, nil, 39, 320)
	if len(m) < 1000 {
		t.Fatalf("too few blocks: %d", len(m))
	}
	prism, water, gold := 0, 0, 0
	for _, st := range m {
		switch st.Name {
		case "minecraft:prismarine", "minecraft:prismarine_bricks", "minecraft:dark_prismarine":
			prism++
		case "minecraft:water":
			water++
		case "minecraft:gold_block":
			gold++
		}
	}
	t.Logf("blocks=%d prism=%d water=%d gold=%d", len(m), prism, water, gold)
	if prism < 500 {
		t.Fatalf("expected prismarine massing, got %d", prism)
	}
}
