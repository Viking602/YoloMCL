//go:build race

package nativeui

import "time"

func init() {
	// Race instrumentation makes full biome rasters substantially slower.
	// Normal UI tests retain their five-second responsiveness deadline.
	nativeUITestWait = 30 * time.Second
}
