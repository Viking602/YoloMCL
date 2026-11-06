package nativeui

import (
	"os"
	"time"
)

// MCL_UI_WAIT (e.g. "120s") lengthens waitUI for slow live-jar harnesses.
func init() {
	if d, err := time.ParseDuration(os.Getenv("MCL_UI_WAIT")); err == nil && d > nativeUITestWait {
		nativeUITestWait = d
	}
}
