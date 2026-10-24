package memory

import (
	"runtime"
	"testing"
)

func TestSystemMemory(t *testing.T) {
	info, e := SystemMemory()
	if e != nil {
		if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
			t.Skip(e)
		}
		t.Fatal(e)
	}
	if info.TotalMB < 512 || info.AvailableMB < 0 || info.AvailableMB > info.TotalMB || info.MaxMB > info.TotalMB || info.MaxMB > 131072 || info.RecommendedMB < 512 || info.RecommendedMB > info.MaxMB {
		t.Fatalf("invalid physical memory measurement: %+v", info)
	}
}
