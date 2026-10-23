//go:build !windows

package startup

import "context"

// Metal/Cocoa and OpenGL's window contexts remain on MyGo's UI thread. The
// common adapter still presents the real software frame first, then requests
// GPU initialization on that same thread after the first frame has returned.
func prepareGraphics(context.Context) {}
