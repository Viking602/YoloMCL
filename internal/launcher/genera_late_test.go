package launcher

import (
	"reflect"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// Jars installed after world creation contribute their pool edit sources;
// updates of an older mod family and older jars do not.
func TestEditSourcesSince(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC) }
	jars := []eraJar{
		{"[a] touhou_little_maid_spell-1.21.1-1.8.4.jar", day(12)},
		{"[b] kaleidoscope_grilling-1.1.1.jar", day(12)},
		{"Tsuki-1.7.0.jar", day(24)},
		{"[c] kaleidoscopecookery-1.6.0.jar", day(30)},
	}
	edits := []structure3d.PoolEdit{{Source: "touhou_little_maid_spell"}, {Source: "tsuki"}, {Source: "kaleidoscope_cookery"}}
	if got := editSourcesSince(jars, edits, day(14)); !reflect.DeepEqual(got, []string{"tsuki"}) {
		t.Fatalf("after 09-14: %v", got)
	}
	if got := editSourcesSince(jars, edits, day(25)); len(got) != 0 {
		t.Fatalf("after 09-25: %v", got)
	}
}
