package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestWorldgenLargeInlineClimateList(t *testing.T) {
	// Generated datapacks commonly inline the full vanilla-sized climate list
	// with indentation, rather than referring to a small preset resource.
	const entry = `{"biome":"example:wide","parameters":{"temperature":[-1,1],"humidity":[-1,1],"continentalness":[-1,1],"erosion":[-1,1],"depth":[-1,1],"weirdness":[-1,1],"offset":0}}`
	var compact bytes.Buffer
	compact.WriteString(`{"type":"minecraft:overworld","generator":{"type":"minecraft:noise","settings":"minecraft:overworld","biome_source":{"type":"minecraft:multi_noise","biomes":[`)
	for i := range 7593 {
		if i != 0 {
			compact.WriteByte(',')
		}
		compact.WriteString(entry)
	}
	compact.WriteString(`]}}}`)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, compact.Bytes(), "", "  "); err != nil {
		t.Fatal(err)
	}
	if pretty.Len() <= 2<<20 || pretty.Len() >= 16<<20 {
		t.Fatalf("fixture no longer exercises the large resource case: %d", pretty.Len())
	}
	const resource = "data/minecraft/dimension/overworld.json"
	dir := t.TempDir()
	pack := filepath.Join(dir, "pack")
	writeTestFile(t, filepath.Join(pack, filepath.FromSlash(resource)), pretty.Bytes())
	jar := filepath.Join(dir, "pack.jar")
	writeTestFile(t, jar, zipBytes(t, map[string][]byte{resource: pretty.Bytes()}))
	for _, source := range []WorldgenSource{{jar, "mod:large.jar"}, {pack, "datapack:large"}} {
		t.Run(source.Name, func(t *testing.T) {
			ctx := context.Background()
			input := WorldgenInput{Minecraft: "1.21.1", CacheDir: t.TempDir(), Sources: []WorldgenSource{source}}
			firstHash := ""
			for range 2 { // Check both initial ingest and the serialized cache.
				snapshot, err := WorldgenIngest(ctx, input)
				if err != nil {
					t.Fatal(err)
				}
				if len(snapshot.Issues) != 0 {
					t.Fatalf("large valid resource rejected: %+v", snapshot.Issues)
				}
				var actual bytes.Buffer
				if err := json.Compact(&actual, worldgenDoc(t, snapshot, "dimension", "minecraft:overworld").Data); err != nil || !bytes.Equal(actual.Bytes(), compact.Bytes()) {
					t.Fatal("inline parameter list was replaced or truncated")
				}
				if firstHash != "" && firstHash != snapshot.Hash {
					t.Fatal("large resource changed across cache reload")
				}
				firstHash = snapshot.Hash
				sampler := testSeedMapSampler(t, snapshot, 0)
				if coverage := sampler.Coverage("minecraft:overworld"); coverage.Opaque || coverage.Terrain != "multi_noise" {
					t.Fatalf("large climate list did not compile: %+v", coverage)
				}
				if biome, ok := sampler.BiomeAt("minecraft:overworld", 0, 64, 0); !ok || biome != "example:wide" {
					t.Fatalf("mod's actual biome rules were not sampled: %s %v", biome, ok)
				}
			}
		})
	}
}

func TestWorldgenJSON16MiBBoundary(t *testing.T) {
	raw := bytes.Repeat([]byte(" "), 16<<20)
	copy(raw, "{}")
	if _, ok := worldgenJSONComments(raw); !ok || !boundedWorldgenJSON(raw) {
		t.Fatal("valid JSON at the 16 MiB limit was rejected")
	}
	raw = append(raw, ' ')
	if _, ok := worldgenJSONComments(raw); ok || boundedWorldgenJSON(raw) {
		t.Fatal("JSON beyond 16 MiB was accepted")
	}
}
