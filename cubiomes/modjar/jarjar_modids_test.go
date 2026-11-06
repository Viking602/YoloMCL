package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"slices"
	"testing"
)

func TestArchiveModIDsIgnoresDependencyModIDs(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("META-INF/neoforge.mods.toml")
	f.Write([]byte("modLoader=\"javafml\"\n[[mods]]\nmodId = \"sauce\"\n[[dependencies.sauce]]\nmodId = \"minecraft\"\n[[dependencies.sauce]]\nmodId = \"ars_nouveau\"\n"))
	w.Close()
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	c := &Classpath{ctx: context.Background()}
	if got := c.archiveModIDs(Archive{Name: "x", Reader: z}); !slices.Equal(got, []string{"sauce"}) {
		t.Fatalf("mod IDs = %v, want [sauce]", got)
	}
}
