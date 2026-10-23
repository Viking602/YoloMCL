//go:build windows

package launcher

import (
	"os"
	"os/exec"
	"syscall"
)

// Junctions are common for game libraries and do not need the privilege that
// Windows requires for symlinks. Exercise both with ordinary user permissions.
func scanTestDirectoryLink(target, path string) error {
	if err := os.Symlink(target, path); err == nil {
		return nil
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference='Stop'; New-Item -ItemType Junction -Path $env:MCL_TEST_LINK -Target $env:MCL_TEST_TARGET | Out-Null")
	cmd.Env = append(os.Environ(), "MCL_TEST_LINK="+path, "MCL_TEST_TARGET="+target)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
