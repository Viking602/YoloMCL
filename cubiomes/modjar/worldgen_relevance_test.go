package modjar

import (
	"testing"
)

func TestWorldgenRelevantDatapackAndFramework(t *testing.T) {
	if !WorldgenRelevant(fixtureArchive(t, map[string][]byte{
		"data/sample/worldgen/biome/plains.json": []byte(`{}`),
	})) {
		t.Fatal("worldgen datapack")
	}
	if !WorldgenRelevant(fixtureArchive(t, map[string][]byte{
		"terrablender/api/Region.class": []byte{0},
	})) {
		t.Fatal("terrablender api")
	}
	if !WorldgenRelevant(fixtureArchive(t, map[string][]byte{
		"fabric.mod.json": []byte(`{"schemaVersion":1,"id":"sample","version":"1","entrypoints":{"terrablender":["sample.Main"]}}`),
	})) {
		t.Fatal("terrablender entrypoint")
	}
	if WorldgenRelevant(fixtureArchive(t, map[string][]byte{
		"fabric.mod.json":             []byte(`{"schemaVersion":1,"id":"crash_assistant","version":"1"}`),
		"crash_assistant.mixins.json": []byte(`{}`),
	})) {
		t.Fatal("crash helper must stay off registration classpath")
	}
	if !ClientOnlyMod(fixtureArchive(t, map[string][]byte{
		"fabric.mod.json": []byte(`{"schemaVersion":1,"id":"ui","version":"1","environment":"client"}`),
	})) {
		t.Fatal("client only")
	}
	if ClientOnlyMod(fixtureArchive(t, map[string][]byte{
		"fabric.mod.json":               []byte(`{"schemaVersion":1,"id":"ui","version":"1","environment":"client"}`),
		"data/ui/worldgen/biome/x.json": []byte(`{}`),
	})) {
		t.Fatal("client with worldgen datapack remains relevant")
	}
}
