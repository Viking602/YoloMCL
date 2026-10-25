package nativeui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
)

// Only official archive layouts support the updater's relaunch path. Renaming
// a tagged executable disables in-place installation, not the security policy.
func officialExecutable(path, goos string) bool {
	if goos == "windows" {
		return strings.EqualFold(filepath.Base(path), "YoloMCL.exe")
	}
	if filepath.Base(path) != "YoloMCL" {
		return false
	}
	if goos == "darwin" {
		return filepath.Base(filepath.Dir(path)) == "MacOS" &&
			filepath.Base(filepath.Dir(filepath.Dir(path))) == "Contents" &&
			strings.HasSuffix(filepath.Dir(filepath.Dir(filepath.Dir(path))), ".app")
	}
	return goos == "linux"
}

func (a *App) restoreLauncherUpdatePolicy() {
	a.launcherUpdate.currentVersion = buildinfo.Current().Version
	if !buildinfo.ValidTag(a.launcherUpdate.currentVersion) {
		return
	}
	policy, err := buildinfo.ReadUpdatePolicy(filepath.Join(a.snapshot.DataDir, "update-policy.json"), runtime.GOOS+"-"+runtime.GOARCH, buildinfo.PublicKey())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("read launcher update policy: %v", err)
		}
		return
	}
	a.launcherUpdate.policy = policy
}

func newerLauncherPolicy(cached, incoming *buildinfo.UpdatePolicy) *buildinfo.UpdatePolicy {
	if incoming != nil && (cached == nil || buildinfo.CompareVersions(incoming.Version, cached.Version) > 0) {
		return incoming
	}
	return cached
}

func checkOfficialRelease(ctx context.Context, current, dir string, cached *buildinfo.UpdatePolicy) (launcherUpdateResult, error) {
	result := launcherUpdateResult{policy: cached}
	if !buildinfo.ValidTag(current) {
		return result, mygo.ErrUpdatesDisabled
	}
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || len(via) >= 10 {
			return errors.New("invalid update feed redirect")
		}
		return nil
	}}
	version, incoming, err := fetchLauncherUpdatePolicy(ctx, client, buildinfo.Feed(runtime.GOOS, runtime.GOARCH), runtime.GOOS+"-"+runtime.GOARCH, buildinfo.PublicKey())
	if err != nil {
		return result, err
	}
	result.policy = newerLauncherPolicy(cached, incoming)
	if result.policy != nil && buildinfo.CompareVersions(version, result.policy.Version) < 0 {
		return result, errors.New("update feed is older than the verified cached policy")
	}
	if result.policy != cached {
		if err := buildinfo.SaveUpdatePolicy(filepath.Join(dir, "update-policy.json"), result.policy); err != nil {
			// The verified policy still applies in this process if storage fails.
			log.Printf("cache launcher update policy: %v", err)
		}
	}
	if buildinfo.CompareVersions(version, current) <= 0 {
		return result, nil
	}
	result.release = &launcherRelease{version: version}
	path, err := os.Executable()
	if err != nil {
		return result, err
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	if !officialExecutable(path, runtime.GOOS) || !mygo.Updater.Enabled() {
		return result, nil // the dialog offers the official manual download
	}
	update, err := mygo.Updater.Check(ctx)
	if err != nil {
		return result, err
	}
	if update == nil || strings.TrimPrefix(update.Version, "v") != version {
		return result, errors.New("release changed during update check; retry to refresh it")
	}
	result.release.install = update.Install
	return result, nil
}

func fetchLauncherUpdatePolicy(ctx context.Context, client *http.Client, feed, target, key string) (string, *buildinfo.UpdatePolicy, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("update feed returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, buildinfo.MaxUpdatePolicySize+1))
	if err != nil {
		return "", nil, err
	}
	if len(data) > buildinfo.MaxUpdatePolicySize {
		return "", nil, errors.New("update manifest exceeds size limit")
	}
	var manifest struct {
		Version        string          `json:"version"`
		MinimumVersion json.RawMessage `json:"minimumVersion"`
		Signature      json.RawMessage `json:"policySignature"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", nil, err
	}
	if len(manifest.Version) > 128 || !buildinfo.ValidTag("v"+manifest.Version) {
		return "", nil, errors.New("invalid release version")
	}
	if manifest.MinimumVersion == nil && manifest.Signature == nil {
		return manifest.Version, nil, nil // legacy feeds cannot revoke a build
	}
	policy, err := buildinfo.DecodeUpdatePolicy(data, target, key)
	return manifest.Version, policy, err
}
