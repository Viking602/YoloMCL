package buildconfig

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
)

// Metadata reads Git identity only at build time. An untagged or modified build
// is deliberately a development build, even when the inspector is disabled.
func Metadata(root string) (buildinfo.Info, error) {
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		data, err := cmd.Output()
		return strings.TrimSpace(string(data)), err
	}
	info := buildinfo.Info{Version: "dev"}
	var err error
	info.Commit, err = git("rev-parse", "HEAD")
	if err != nil {
		return info, errors.New("cannot read Git commit")
	}
	status, err := git("status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return info, errors.New("cannot read Git working tree")
	}
	info.Dirty = status != ""
	if tags, err := git("tag", "--points-at", "HEAD"); err == nil && !info.Dirty {
		for _, tag := range strings.Fields(tags) {
			if buildinfo.ValidTag(tag) {
				info.Version = tag
			}
		}
	}
	return info, nil
}

// MetadataArguments adds public build identity and the update verification key.
// Arguments already contains publisher secrets and must never be logged.
func MetadataArguments(args []string, info buildinfo.Info, goos, goarch string, updates bool) ([]string, error) {
	if info.Version != "dev" && !buildinfo.ValidTag(info.Version) {
		return nil, errors.New("version must be dev or a v-prefixed SemVer tag")
	}
	if !regexp.MustCompile(`^[a-f0-9]{40,64}$`).MatchString(info.Commit) {
		return nil, errors.New("invalid Git commit")
	}
	const pkg = "github.com/Viking602/YoloMCL/internal/buildinfo"
	flags := " -X " + pkg + ".version=" + info.Version + " -X " + pkg + ".commit=" + info.Commit +
		" -X " + pkg + ".dirty=" + strconv.FormatBool(info.Dirty)
	if updates {
		key, err := base64.StdEncoding.DecodeString(buildinfo.PublicKey())
		if err != nil || len(key) != 32 || !buildinfo.ValidTag(info.Version) || info.Dirty {
			return nil, errors.New("online updates require a clean release tag and an Ed25519 public key")
		}
		if goos != "windows" && goos != "linux" && goos != "darwin" || goarch != "amd64" && goarch != "arm64" {
			return nil, errors.New("unsupported update target")
		}
		flags += " -X github.com/egoist/mygo.packageUpdateFeed=" + buildinfo.Feed(goos, goarch) +
			" -X github.com/egoist/mygo.packageUpdateKey=" + buildinfo.PublicKey()
	}
	result := append([]string(nil), args...)
	for i, arg := range result {
		if arg == "-ldflags" && i+1 < len(result) {
			result[i+1] += flags
			return result, nil
		}
	}
	return nil, fmt.Errorf("missing linker arguments")
}
