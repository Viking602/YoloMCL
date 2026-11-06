package structure3d

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenJarSubroot(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "m.jar")
	if err := os.WriteFile(jar, zipWith(
		"assets/m/x.png", []byte("ROOT"),
		"resourcepacks/extra/assets/m/x.png", []byte("SUB"),
		"resourcepacks/extra/pack.mcmeta", []byte("{}"),
	), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Store{}
	defer s.Close()
	if err := s.OpenJar("mod", jar); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenJarSubroot("pack:m:extra", jar, "resourcepacks/extra/"); err != nil {
		t.Fatal(err)
	}
	b, err := s.Read("assets/m/x.png")
	if err != nil || string(b) != "SUB" {
		t.Fatalf("got %q %v", b, err)
	}
	if err := s.OpenJarSubroot("bad", jar, "resourcepacks/missing/"); err == nil {
		t.Fatal("missing subroot should err")
	}
}
