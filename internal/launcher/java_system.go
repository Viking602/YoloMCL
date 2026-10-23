package launcher

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// System Java discovery is separate from Minecraft content discovery. Only
// exact environment/registry paths and immediate children of known Java
// installation directories are considered. Never pass these roots to WalkDir.
func systemJavaPaths(ctx context.Context) []string {
	out := environmentJavaPaths(os.Getenv, runtime.GOOS)
	for _, home := range platformJavaHomes(ctx) {
		if ctx.Err() != nil {
			return out
		}
		out = append(out, javaHomePaths(home, runtime.GOOS)...)
	}
	return append(out, installedJavaPaths(ctx, javaInstallLocations(runtime.GOOS, homeDir(), os.Getenv), runtime.GOOS)...)
}

func javaHomePaths(home, goos string) []string {
	home = strings.Trim(strings.TrimSpace(home), `"`)
	if home == "" || !filepath.IsAbs(home) {
		return nil
	}
	name := "java"
	if goos == "windows" {
		name = "java.exe"
	}
	if strings.EqualFold(filepath.Base(home), name) || goos == "windows" && strings.EqualFold(filepath.Base(home), "javaw.exe") {
		return []string{home}
	}
	out := []string{filepath.Join(home, "bin", name), filepath.Join(home, "jre", "bin", name)}
	if strings.EqualFold(filepath.Base(home), "bin") {
		out = append(out, filepath.Join(home, name))
	}
	if goos == "darwin" {
		out = append(out, filepath.Join(home, "Contents", "Home", "bin", name))
	}
	return out
}

func environmentJavaPaths(getenv func(string) string, goos string) []string {
	var out []string
	for _, key := range []string{"JAVA_HOME", "JDK_HOME", "JRE_HOME"} {
		out = append(out, javaHomePaths(getenv(key), goos)...)
	}
	name := "java"
	if goos == "windows" {
		name = "java.exe"
	}
	for _, dir := range filepath.SplitList(getenv("PATH")) {
		dir = strings.Trim(strings.TrimSpace(dir), `"`)
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		path := filepath.Join(dir, name)
		// Apple's /usr/bin/java is an installer stub, not a JDK. Enumerate
		// JavaVirtualMachines below instead of launching that stub.
		if goos == "darwin" && path == "/usr/bin/java" {
			continue
		}
		out = append(out, path)
	}
	return out
}

type javaInstallLocation struct{ path, prefix string }

func javaInstallLocations(goos, home string, getenv func(string) string) []javaInstallLocation {
	var out []javaInstallLocation
	add := func(path, prefix string) {
		if path != "" && filepath.IsAbs(path) {
			out = append(out, javaInstallLocation{path, prefix})
		}
	}
	switch goos {
	case "windows":
		addVendors := func(base string) {
			if base == "" || !filepath.IsAbs(base) {
				return
			}
			for _, vendor := range []string{"Java", "Eclipse Adoptium", "AdoptOpenJDK", "Zulu", "BellSoft", "Amazon Corretto", "Semeru", "GraalVM", "SAP", "RedHat"} {
				add(filepath.Join(base, vendor), "")
			}
			add(filepath.Join(base, "Microsoft"), "jdk")
		}
		for _, key := range []string{"ProgramW6432", "ProgramFiles", "ProgramFiles(x86)"} {
			addVendors(getenv(key))
		}
		if local := getenv("LOCALAPPDATA"); local != "" && filepath.IsAbs(local) {
			addVendors(filepath.Join(local, "Programs"))
		}
	case "darwin":
		add("/Library/Java/JavaVirtualMachines", "")
		if home != "" {
			add(filepath.Join(home, "Library", "Java", "JavaVirtualMachines"), "")
		}
		add("/opt/homebrew/opt", "openjdk")
		add("/usr/local/opt", "openjdk")
	default:
		for _, path := range []string{"/usr/lib/jvm", "/usr/lib64/jvm", "/usr/java", "/opt/java", "/opt/jdk"} {
			add(path, "")
		}
	}
	if home != "" {
		add(filepath.Join(home, ".jdks"), "")
		add(filepath.Join(home, ".gradle", "jdks"), "")
	}
	if goos != "windows" && home != "" {
		add(filepath.Join(home, ".sdkman", "candidates", "java"), "")
		add(filepath.Join(home, ".jenv", "versions"), "")
	}
	return out
}

func installedJavaPaths(ctx context.Context, locations []javaInstallLocation, goos string) []string {
	var out []string
	seen := map[string]bool{}
	for _, location := range locations {
		if ctx.Err() != nil {
			break
		}
		key := javaPathKey(filepath.Clean(location.path)) + "|" + location.prefix
		if seen[key] {
			continue
		}
		seen[key] = true
		if location.prefix == "" {
			out = append(out, javaHomePaths(location.path, goos)...)
		}
		f, err := os.Open(location.path)
		if err != nil {
			continue
		}
		// At most one level and 512 installations per known location. No
		// recursion into arbitrary files, sibling applications or disk roots.
		entries, _ := f.ReadDir(512)
		_ = f.Close()
		for _, entry := range entries {
			if ctx.Err() != nil {
				return out
			}
			if location.prefix != "" && !strings.HasPrefix(strings.ToLower(entry.Name()), location.prefix) {
				continue
			}
			if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
				continue
			}
			out = append(out, javaHomePaths(filepath.Join(location.path, entry.Name()), goos)...)
		}
	}
	return out
}
