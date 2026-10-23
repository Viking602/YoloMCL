//go:build !windows

package launcher

import "os"

func scanTestDirectoryLink(target, path string) error { return os.Symlink(target, path) }
