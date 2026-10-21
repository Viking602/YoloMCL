//go:build !windows

package launcher

func platformJavaHomes() []string { return nil }
func platformJavaDisks() []string {
	return []string{"/usr/lib/jvm", "/opt", "/Library/Java/JavaVirtualMachines", "/Applications"}
}
