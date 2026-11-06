package launcher

import (
	"fmt"
	"os"
	"time"
)

// seedMapStage returns a stage timer printing to stderr when MCL_SEEDMAP_TRACE
// is set; each call ends the previous stage and starts the named one.
func seedMapStage(name string) func(string) {
	if os.Getenv("MCL_SEEDMAP_TRACE") == "" {
		return func(string) {}
	}
	start, cur := time.Now(), name
	return func(next string) {
		fmt.Fprintf(os.Stderr, "seedmap-trace %-18s %v\n", cur, time.Since(start).Round(time.Millisecond))
		start, cur = time.Now(), next
	}
}
