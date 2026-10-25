package nativeui

import (
	"os"
	"runtime"

	"github.com/egoist/mygo/ui"
)

// titlebarLayout is the one place the window chrome differs by OS; pages
// never branch on the platform. The framework reports the room the native
// window controls take (ui.Context.TitleBar): the traffic lights on the
// left on macOS, the caption buttons on the right on Windows, and the GTK
// button layout's sides on Linux. It reports zero in full screen.
type titlebarLayout struct {
	OS          string
	Left, Right float32 // room the native controls take, in px
	// CustomControls draws our own min/max/close: only when the platform
	// gives none (frameless Windows/Linux fallback). Never on macOS: no
	// fake traffic lights.
	CustomControls bool
}

// This screenshot-only override is fixed at startup. Font selection calls this
// path for every label and digit, so avoid a Windows environment lookup (and
// UTF-16 allocation) for each widget on each animation frame.
var titlebarPlatform = os.Getenv("MCL_TITLEBAR_OS")

// titlebarOS is runtime.GOOS, or the startup MCL_TITLEBAR_OS override.
func titlebarOS() string {
	if titlebarPlatform != "" {
		return titlebarPlatform
	}
	return runtime.GOOS
}

// macTrafficInset is the left room the traffic lights need (close..zoom
// plus margin), used when the framework does not report it (tests).
const macTrafficInset float32 = 76

func (a *App) titlebar(c *ui.Context) titlebarLayout {
	bar := c.TitleBar()
	l := titlebarLayout{OS: titlebarOS(), Left: max(bar.Left, 0), Right: max(bar.Right, 0)}
	forced := titlebarPlatform != ""
	switch l.OS {
	case "darwin":
		if forced && l.Left == 0 && !a.fullScreen() {
			l.Left = macTrafficInset // DIPs: the native buttons do not follow the UI scale
		}
		l.Right = 0
	default:
		l.CustomControls = bar.Right == 0 && bar.Left == 0
		if forced { // the fake backend has no native controls
			l.CustomControls = true
		}
	}
	return l
}

func (a *App) fullScreen() bool { return a.win != nil && a.win.IsFullScreen() }
