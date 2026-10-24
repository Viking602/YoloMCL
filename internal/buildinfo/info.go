// Package buildinfo exposes the identity of this launcher build without I/O.
package buildinfo

import (
	_ "embed"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
)

const (
	Repository    = "Viking602/YoloMCL"
	RepositoryURL = "https://github.com/" + Repository
	ReleasesURL   = RepositoryURL + "/releases"
)

// These values are supplied by scripts/build.sh, never by runtime settings.
var version, commit, dirty string

//go:embed update-public-key.txt
var publicKey string

var tagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+\.)*[0-9A-Za-z-]+)?$`)

var current = sync.OnceValue(readCurrent)

// Info identifies the source used to compile the running launcher.
type Info struct {
	Version string
	Commit  string
	Dirty   bool
}

// ValidTag accepts v-prefixed semantic versions, including prerelease tags.
func ValidTag(tag string) bool {
	if !tagPattern.MatchString(tag) {
		return false
	}
	if _, suffix, ok := strings.Cut(tag, "-"); ok {
		for _, part := range strings.Split(suffix, ".") {
			if len(part) > 1 && part[0] == '0' && strings.Trim(part, "0123456789") == "" {
				return false
			}
		}
	}
	return true
}

// Current returns cached build metadata without file or network access.
func Current() Info { return current() }

func readCurrent() Info {
	info := Info{Version: version, Commit: commit, Dirty: dirty == "true"}
	if info.Version == "" {
		info.Version = "dev"
	}
	if info.Commit == "" {
		if data, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range data.Settings {
				switch setting.Key {
				case "vcs.revision":
					info.Commit = setting.Value
				case "vcs.modified":
					info.Dirty = setting.Value == "true"
				}
			}
		}
	}
	return info
}

// ShortCommit is the eight-character revision displayed beside the version.
func (i Info) ShortCommit() string {
	if i.Commit == "" {
		return "unknown"
	}
	return i.Commit[:min(8, len(i.Commit))]
}

// PublicKey is the Ed25519 verification key shared by all official platforms.
func PublicKey() string { return strings.TrimSpace(publicKey) }

// Feed is the manifest URL in the latest stable GitHub release for a target.
func Feed(goos, goarch string) string {
	return ReleasesURL + "/latest/download/update-" + goos + "-" + goarch + ".json"
}
