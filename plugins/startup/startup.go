// Package startup keeps the launcher's first frame independent of driver
// initialization. It is an MCL plugin, not part of or a replacement for MyGo.
package startup

import (
	"context"
	"os"
	"runtime/debug"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const mygoVersion = "v0.5.5"

// Accelerator is registered before constructing any windows. Content wraps
// only this application's main window; dialogs keep the normal MyGo path.
type Accelerator struct {
	once, presentOnce sync.Once
	done, presented   chan struct{}
	enabled           bool
}

func New() *Accelerator {
	return &Accelerator{done: make(chan struct{}), presented: make(chan struct{}), enabled: supportedMyGo() && os.Getenv("MYGO_GPU") != "0" && os.Getenv("MCL_STARTUP_GPU") != "0"}
}

func (a *Accelerator) Plugin() mygo.Plugin {
	return mygo.Plugin{Name: "mcl-startup", Setup: func() error {
		a.once.Do(func() {
			if !a.enabled {
				close(a.done)
				return
			}
			ctx, cancel := context.WithCancel(context.Background())
			mygo.App.OnQuit(cancel)
			go a.prepare(ctx, prepareGraphics)
		})
		return nil
	}}
}

func (a *Accelerator) prepare(ctx context.Context, warm func(context.Context)) {
	defer close(a.done)
	// Driver startup can compete with window creation and the CPU renderer
	// even on a worker. Give the first real frame exclusive priority, without
	// an arbitrary timer, then warm the driver while the UI is interactive.
	select {
	case <-ctx.Done():
		return
	case <-a.presented:
	}
	if ctx.Err() == nil {
		warm(ctx)
	}
}

// Content uses the same UI engine for the software frame and the GPU frames.
// Switching the renderer does not recreate widgets, focus, scroll or animations.
func (a *Accelerator) Content(view func(*ui.Context)) mygo.Content {
	v := ui.View(view)
	if !a.enabled {
		return v
	}
	return adapt(a.done, func() { a.presentOnce.Do(func() { close(a.presented) }) }, v.AttachContent)
}

// ContentAfter joins application preparation on the UI thread, after native
// window creation and before attaching the UI engine. No blank frame or timer
// is needed. Native APIs remain on MyGo's main thread on every platform.
func (a *Accelerator) ContentAfter(prepare func(), view func(*ui.Context)) mygo.Content {
	v := a.Content(view)
	return prepareContent(prepare, v.AttachContent)
}

func supportedMyGo() bool {
	b, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, d := range b.Deps {
		if d.Path == "github.com/egoist/mygo" {
			return d.Version == mygoVersion && d.Replace == nil
		}
	}
	return false
}
