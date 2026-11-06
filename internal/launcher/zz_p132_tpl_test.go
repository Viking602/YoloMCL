package launcher

import (
	"archive/zip"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// MCL_P132_TPL="jar|entry;entry" prints template sizes straight from a jar.
func TestZZP132TemplateSize(t *testing.T) {
	spec := os.Getenv("MCL_P132_TPL")
	if spec == "" {
		t.Skip()
	}
	jar, entries, _ := strings.Cut(spec, "|")
	z, err := zip.OpenReader(jar)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, name := range strings.Split(entries, ";") {
		for _, f := range z.File {
			if f.Name != name {
				continue
			}
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			root, err := structure3d.DecodeNBT(data)
			t.Logf("%s size=%v err=%v", name, root["size"], err)
		}
	}
}
