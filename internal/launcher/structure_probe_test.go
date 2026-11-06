package launcher

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

var structJSON = regexp.MustCompile(`^data/([^/]+)/worldgen/structure/(.+)\.json$`)

func structureIDsInZip(files []*zip.File, out map[string]bool) {
	for _, f := range files {
		if m := structJSON.FindStringSubmatch(f.Name); m != nil {
			out[m[1]+":"+m[2]] = true
		}
		low := strings.ToLower(f.Name)
		if strings.HasSuffix(low, ".jar") && strings.HasPrefix(low, "meta-inf/") {
			r, err := f.Open()
			if err != nil {
				continue
			}
			b, _ := io.ReadAll(r)
			r.Close()
			if z, err := zip.NewReader(bytes.NewReader(b), int64(len(b))); err == nil {
				structureIDsInZip(z.File, out)
			}
		}
	}
}

// Probe: YOLOMCL_STRUCT_PROBE="<.minecraft>|<version>[;<version>...]" previews
// every structure the instance's mods declare and prints coverage.
func TestStructurePreviewCoverageProbe(t *testing.T) {
	spec := os.Getenv("YOLOMCL_STRUCT_PROBE")
	if spec == "" {
		t.Skip("set YOLOMCL_STRUCT_PROBE")
	}
	root, versions, _ := strings.Cut(spec, "|")
	s := testService(t)
	r, err := s.AddRoot(root, "probe")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range strings.Split(versions, ";") {
		store, err := s.StructureAssetStore(r.ID, v)
		if err != nil {
			t.Fatal(err)
		}
		ids := map[string]bool{}
		mods, _ := s.ListModsAt(r.ID, v)
		gameDir := Root{Path: r.Path, SelectedVersion: v}.VersionDir()
		for _, m := range mods {
			if !m.Enabled {
				continue
			}
			if z, err := zip.OpenReader(filepath.Join(gameDir, "mods", m.File)); err == nil {
				structureIDsInZip(z.File, ids)
				z.Close()
			}
		}
		list := make([]string, 0, len(ids))
		for id := range ids {
			list = append(list, id)
		}
		sort.Strings(list)
		counts := map[string]int{}
		var examples = map[string][]string{}
		for _, id := range list {
			target, _ := ResolvePreviewTarget(store, id, "")
			res, err := PreviewStructure(store, target, 1, 0, 0, nil, 96, 72, structure3d.DefaultCamera())
			kind := "ok"
			var pe *structure3d.PreviewError
			switch {
			case errors.As(err, &pe):
				kind = pe.Kind
			case err != nil:
				kind = "error"
			case res.Unsupported != "":
				kind = res.UnsupportedKind
			}
			counts[kind]++
			if len(examples[kind]) < 4 {
				examples[kind] = append(examples[kind], id)
			}
			if kind == "missing_template" || kind == "error" {
				t.Logf("  %s %s: %v", kind, id, err)
			}
		}
		t.Logf("%s: %d structures, mods=%d, layers=%d, counts=%v", v, len(list), len(mods), len(store.Layers()), counts)
		for k, ex := range examples {
			t.Logf("   %s e.g. %v", k, ex)
		}
		store.Close()
	}
}
