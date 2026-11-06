package structure3d

import "testing"

func TestReadTagValuesMerges(t *testing.T) {
	s := &Store{}
	_ = s.OpenBytes("vanilla", zipWith("data/minecraft/tags/worldgen/biome/x.json", []byte(`{"values":["a","b"]}`)))
	_ = s.OpenBytes("mod", zipWith("data/minecraft/tags/worldgen/biome/x.json", []byte(`{"values":["c",{"id":"d","required":false}]}`)))
	got := s.ReadTagValues("data/minecraft/tags/worldgen/biome/x.json")
	if len(got) != 4 || got[0] != "a" || got[3] != "d" {
		t.Fatalf("merged %v", got)
	}
	_ = s.OpenBytes("pack", zipWith("data/minecraft/tags/worldgen/biome/x.json", []byte(`{"replace":true,"values":["e"]}`)))
	if got := s.ReadTagValues("data/minecraft/tags/worldgen/biome/x.json"); len(got) != 1 || got[0] != "e" {
		t.Fatalf("replace %v", got)
	}
	// A plain resource is replaced, not merged: the last layer wins.
	b, _ := s.Read("data/minecraft/tags/worldgen/biome/x.json")
	if string(b) != `{"replace":true,"values":["e"]}` {
		t.Fatalf("read %s", b)
	}
}
