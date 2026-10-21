package distribution

import (
	"bufio"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// cfKeyObf is injected at build time by scripts/build.ps1 / scripts/build.sh:
//
//	-ldflags "-X mcl/internal/distribution.cfKeyObf=<hex>"
//
// The value is the CurseForge API key XORed with cfKeyMask and hex-encoded, so
// the raw key never appears as a plain string in the binary.
var cfKeyObf string

// cfKeyMask must match the mask in scripts/build.ps1 and scripts/build.sh.
const cfKeyMask = "mcl-cf-9f3a7c1e-launcher"

func xorMask(b []byte) []byte {
	out := make([]byte, len(b))
	for i := range b {
		out[i] = b[i] ^ cfKeyMask[i%len(cfKeyMask)]
	}
	return out
}

// embeddedCurseForgeKey decodes the build-injected key ("" if none/invalid).
func embeddedCurseForgeKey() string {
	if cfKeyObf == "" {
		return ""
	}
	b, e := hex.DecodeString(strings.TrimSpace(cfKeyObf))
	if e != nil {
		return ""
	}
	return strings.TrimSpace(string(xorMask(b)))
}

// overrideCurseForgeKey returns a key from the CURSEFORGE_API_KEY environment
// variable, or from a .env file in the working directory or next to the exe.
func overrideCurseForgeKey() string {
	if v := strings.TrimSpace(os.Getenv("CURSEFORGE_API_KEY")); v != "" {
		return v
	}
	dirs := []string{"."}
	if exe, e := os.Executable(); e == nil {
		dirs = append(dirs, filepath.Dir(exe), filepath.Dir(filepath.Dir(exe)))
	}
	for _, d := range dirs {
		if v := dotenvValue(filepath.Join(d, ".env"), "CURSEFORGE_API_KEY"); v != "" {
			return v
		}
	}
	return ""
}

func dotenvValue(path, key string) string {
	f, e := os.Open(path)
	if e != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) != key {
			continue
		}
		return strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return ""
}
