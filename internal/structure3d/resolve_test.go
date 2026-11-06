package structure3d

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func zipOf(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write(data)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func iglooNBT(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "jar", "data", "minecraft", "structure", "igloo", "top.nbt"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A mod that ships its data in a JarJar jar under the legacy structures/ folder
// must resolve; the outer jar file itself holds no data.
func TestTemplateResolvesFromNestedJarAndLegacyFolder(t *testing.T) {
	inner := zipOf(t, map[string][]byte{
		"data/demo/structures/cabin_0.nbt":            iglooNBT(t),
		"data/demo/worldgen/template_pool/cabin.json": []byte(`{"fallback":"minecraft:empty","elements":[{"weight":1,"element":{"element_type":"minecraft:single_pool_element","location":"demo:cabin_0","processors":"minecraft:empty","projection":"rigid"}}]}`),
		"data/demo/worldgen/structure/cabin.json":     []byte(`{"type":"minecraft:jigsaw","start_pool":"demo:cabin","size":1}`),
	})
	outer := zipOf(t, map[string][]byte{"META-INF/jarjar/demo-core.jar": inner, "fabric.mod.json": []byte(`{}`)})
	file := filepath.Join(t.TempDir(), "demo.jar")
	if err := os.WriteFile(file, outer, 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Store{}
	defer s.Close()
	if err := s.OpenDir("vanilla", filepath.Join("testdata", "jar")); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenJar("mod:demo.jar", file); err != nil {
		t.Fatal(err)
	}
	if !s.HasTemplate("demo:cabin_0") {
		t.Fatalf("nested legacy template not found; layers=%v", s.Layers())
	}
	res, err := Preview(s, PreviewRequest{StructureID: "demo:cabin", Width: 160, Height: 120})
	if err != nil || res.Unsupported != "" || res.BlockCount < 20 {
		t.Fatalf("preview = %+v, %v", res, err)
	}
}

func TestCodeGeneratedAndMissingAreTyped(t *testing.T) {
	s := &Store{}
	defer s.Close()
	if err := s.OpenBytes("mod", zipOf(t, map[string][]byte{
		"data/demo/worldgen/structure/vault.json": []byte(`{"type":"demo:vault","biomes":"#minecraft:is_overworld"}`),
	})); err != nil {
		t.Fatal(err)
	}
	res, err := Preview(s, PreviewRequest{StructureID: "demo:vault", Width: 64, Height: 64})
	if err != nil || res.UnsupportedKind != KindCodeGenerated {
		t.Fatalf("code structure = %+v, %v", res, err)
	}
	_, err = Preview(s, PreviewRequest{TemplateID: "demo:nope", Width: 64, Height: 64})
	var pe *PreviewError
	if !errors.As(err, &pe) || pe.Kind != KindMissingTemplate {
		t.Fatalf("missing template err = %v", err)
	}
}
func TestLenientStructureJSON(t *testing.T) {
	src := []byte("{\n  \"type\": \"minecraft:jigsaw\", // c\n  /* b */ \"start_pool\": \"a://b\",\n  \"max_distance_from_center\": {\"horizontal\": 90, \"vertical\": 9},\n}")
	var def StructureDef
	if err := json.Unmarshal(lenientJSON(src), &def); err != nil {
		t.Fatalf("%v: %s", err, lenientJSON(src))
	}
	if def.StartPool != "a://b" || horizontalDistance(def.MaxDistanceFromCenter) != 90 {
		t.Fatalf("def = %+v", def)
	}
}
