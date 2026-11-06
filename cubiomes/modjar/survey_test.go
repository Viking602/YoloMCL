package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiscoveryMetadataSurvey (opt-in MODJAR_SURVEY=<versions dir>) checks
// whether every jar with TerraBlender registrations mentions terrablender
// in loader metadata or mixin/entrypoint files.
func TestDiscoveryMetadataSurvey(t *testing.T) {
	root := os.Getenv("MODJAR_SURVEY")
	if root == "" {
		t.Skip()
	}
	jars, _ := filepath.Glob(filepath.Join(root, "*", "mods", "*.jar"))
	seen := map[string]bool{}
	regions, misses, scanned := 0, 0, 0
	for _, j := range jars {
		base := strings.ToLower(filepath.Base(j))
		if seen[base] {
			continue
		}
		seen[base] = true
		z, err := zip.OpenReader(j)
		if err != nil {
			continue
		}
		scanned++
		mention := false
		for _, f := range z.File {
			n := strings.ToLower(f.Name)
			if strings.HasPrefix(n, "terrablender/") {
				mention = true
				break
			}
			if strings.HasSuffix(n, ".class") || f.UncompressedSize64 > 1<<20 {
				continue
			}
			if strings.HasPrefix(n, "meta-inf/") || n == "fabric.mod.json" || n == "quilt.mod.json" || strings.HasSuffix(n, ".json") && !strings.Contains(n, "/") {
				r, e := f.Open()
				if e != nil {
					continue
				}
				b, _ := io.ReadAll(r)
				r.Close()
				if bytes.Contains(bytes.ToLower(b), []byte("terrablender")) {
					mention = true
					break
				}
			}
		}
		d, err := DiscoverTerraBlender(context.Background(), &z.Reader)
		z.Close()
		if err == nil && d.HasRegions() {
			regions++
			if !mention {
				misses++
				t.Logf("NO METADATA MENTION: %s", j)
			}
		}
	}
	t.Logf("scanned %d unique jars, %d with regions, %d without metadata mention", scanned, regions, misses)
}
