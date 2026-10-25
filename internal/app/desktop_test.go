package app

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

func TestRendererPreferenceAndEnvironment(t *testing.T) {
	for _, hardware := range []bool{false, true} {
		for _, override := range []string{"unset", "", "0", "1"} {
			t.Run(fmt.Sprintf("hardware=%v/env=%s", hardware, override), func(t *testing.T) {
				t.Setenv("MYGO_GPU", override)
				if override == "unset" {
					if err := os.Unsetenv("MYGO_GPU"); err != nil {
						t.Fatal(err)
					}
				}
				if err := configureRendering(hardware); err != nil {
					t.Fatal(err)
				}
				got, set := os.LookupEnv("MYGO_GPU")
				want, wantSet := override, true
				if override == "unset" {
					want, wantSet = "", false
					if runtime.GOOS == "windows" && !hardware {
						want, wantSet = "0", true
					}
				}
				if got != want || set != wantSet {
					t.Fatalf("MYGO_GPU=%q, set=%v; want %q, set=%v", got, set, want, wantSet)
				}
			})
		}
	}
}
