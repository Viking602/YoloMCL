package cubiomes

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSurfaceBiomeHolderSets(t *testing.T) {
	docs := map[string]Document{
		"biome_tag/test:outer": {Data: json.RawMessage(`{"values":["#test:inner",{"id":"#test:absent","required":false}]}`)},
		"biome_tag/test:inner": {Data: json.RawMessage(`{"values":["minecraft:crimson_forest","test:marsh"]}`)},
		"biome_tag/test:cycle": {Data: json.RawMessage(`{"values":["#test:cycle"]}`)},
		"biome_tag/test:bad":   {Data: json.RawMessage(`{"values":[{"required":false}]}`)},
	}
	for _, raw := range []string{`"minecraft:crimson_forest"`, `["minecraft:crimson_forest","test:marsh"]`, `"#test:outer"`, `[]`} {
		sc := &surfaceCompiler{c: newDensityCompiler(context.Background(), docs, 0, false)}
		cond, err := sc.cond(json.RawMessage(`{"type":"minecraft:biome","biome_is":`+raw+`}`), 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, biome := range []string{"minecraft:crimson_forest", "test:marsh", "minecraft:plains"} {
			ctx := &surfaceCtx{in: &SurfaceInput{Biome: func(int) string { return biome }}}
			want := raw != `[]` && (biome == "minecraft:crimson_forest" || biome == "test:marsh" && raw != `"minecraft:crimson_forest"`)
			if got := cond(ctx); got != want {
				t.Fatalf("%s for %s: got %t want %t", raw, biome, got, want)
			}
		}
	}
	for _, raw := range []string{`null`, `""`, `[null]`, `[1]`, `"#test:cycle"`, `"#test:absent"`, `"#test:bad"`} {
		sc := &surfaceCompiler{c: newDensityCompiler(context.Background(), docs, 0, false)}
		if _, err := sc.biomes(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted invalid surface biome set %s", raw)
		}
	}
}
