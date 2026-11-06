package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZZDumpOverworldSource(t *testing.T) {
	for _, p := range strings.Split(os.Getenv("MCL_DUMP_SAVES"), ";") {
		if p == "" {
			continue
		}
		f, err := os.Open(filepath.Join(p, "level.dat"))
		if err != nil {
			t.Log(err)
			continue
		}
		lv, err := parseLevelDat(context.Background(), f)
		f.Close()
		if err != nil {
			t.Log(err)
			continue
		}
		raw := string(lv.Dimensions["minecraft:overworld"])
		if i := strings.LastIndex(raw, "]"); i > 0 {
			raw = raw[i:]
		}
		t.Logf("%s gen=%v\n%s", p, lv.Generators["minecraft:overworld"], raw)
	}
}
