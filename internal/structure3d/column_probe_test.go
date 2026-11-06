package structure3d_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

func TestColumnProbe(t *testing.T) {
	dir := os.Getenv("COLUMN_PROBE")
	if dir == "" {
		t.Skip()
	}
	cc := &regionCache{dir: dir, regions: map[[2]int]*mcworld.Region{}, chunks: map[[2]int]*mcworld.Chunk{}}
	for _, xz := range [][2]int{{-10928, -3184}, {-10900, -3150}, {-10960, -3200}, {-10880, -3120}} {
		var sb []string
		prev := ""
		for y := 0; y < 120; y++ {
			b, ok := cc.block(xz[0], y, xz[1])
			if !ok {
				t.Logf("%v not generated", xz)
				break
			}
			b = strings.TrimPrefix(b, "minecraft:")
			if b != prev {
				sb = append(sb, b+"@"+itoa(y))
				prev = b
			}
		}
		t.Logf("%v %v", xz, sb)
	}
}

func itoa(i int) string {
	return strings.TrimSpace(strings.Repeat(" ", 0) + func() string {
		s := ""
		neg := i < 0
		if neg {
			i = -i
		}
		for {
			s = string(rune('0'+i%10)) + s
			i /= 10
			if i == 0 {
				break
			}
		}
		if neg {
			s = "-" + s
		}
		return s
	}())
}
