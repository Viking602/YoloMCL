//go:build !windows

package launcher

import "context"

// macOS/Linux register installations in their Java-specific directories and
// expose the selected runtime through PATH, handled by java_system.go.
func platformJavaHomes(context.Context) []string { return nil }
