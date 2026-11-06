package launcher

import (
	"context"
	"os"
	"testing"
)

func TestZZP130Jar(t *testing.T) {
	p := os.Getenv("MCL_P130_JAR")
	if p == "" {
		t.Skip()
	}
	c := worldgenCollector{ctx: context.Background(), snapshot: &WorldgenSnapshot{}, documents: map[string]WorldgenDocument{}}
	err := c.readSource(WorldgenSource{p, "mod:x.jar"})
	t.Logf("err=%v issues=%v", err, c.snapshot.Issues)
}
