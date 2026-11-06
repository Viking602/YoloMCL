package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func zzShape(v any, depth int, b *strings.Builder) {
	pad := strings.Repeat("  ", depth)
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			switch cc := c.(type) {
			case string, float64, bool:
				fmt.Fprintf(b, "%s%s=%v\n", pad, k, cc)
			case []any:
				fmt.Fprintf(b, "%s%s[%d]\n", pad, k, len(cc))
				if len(cc) > 0 && depth < 6 {
					zzShape(cc[0], depth+1, b)
				}
			default:
				fmt.Fprintf(b, "%s%s{\n", pad, k)
				if depth < 6 {
					zzShape(c, depth+1, b)
				}
			}
		}
	}
}

func TestZZShapeOverworld(t *testing.T) {
	p := os.Getenv("MCL_DUMP_SAVE")
	if p == "" {
		t.Skip("set MCL_DUMP_SAVE to inspect a local save")
	}
	f, err := os.Open(filepath.Join(p, "level.dat"))
	if err != nil {
		t.Fatal(err)
	}
	lv, err := parseLevelDat(context.Background(), f)
	if closeErr := f.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(lv.Dimensions["minecraft:overworld"], &v); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	zzShape(v, 0, &b)
	t.Log("\n" + b.String())
}
