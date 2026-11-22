package launcher

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHorizonInPlaceUpgradeAndSwitchPreservesInstanceData(t *testing.T) {
	primaryAsset, _, ok := horizonPlatformAssets(runtime.GOOS, runtime.GOARCH)
	if !ok {
		t.Skipf("no Horizon release asset defined for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	binDir := t.TempDir()
	mockHorizonBin := buildHorizonMockBinary(t, binDir, "mock-horizon-upg")
	mockJavaBin := buildMockJava25Binary(t, binDir)
	mockBinBytes, err := os.ReadFile(mockHorizonBin)
	if err != nil {
		t.Fatal(err)
	}

	const (
		buildOld  = "nightly-2026-11-20-0a875da"
		nameOld   = "Nightly 2026-11-20 · 0a875da"
		buildNew  = "nightly-2026-11-21-9b4c2de"
		nameNew   = "Nightly 2026-11-21 · 9b4c2de"
		mcVersion = "26.3"
	)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	makeSignedArchive := func(buildID string) (archiveBytes, sumsBytes []byte, sigV2 string) {
		archivePath := filepath.Join(binDir, buildID+"-"+primaryAsset)
		lockJSON := []byte(`{"minecraft":{"version":"26.3","java_major":25}}`)
		topName := strings.TrimSuffix(strings.TrimSuffix(primaryAsset, ".zip"), ".tar.gz")
		files := map[string][]byte{
			topName + "/" + horizonBinaryName("horizon-client"):  mockBinBytes,
			topName + "/" + horizonBinaryName("horizon-prepare"): mockBinBytes,
			topName + "/research-lock.json":                      lockJSON,
			topName + "/build-id.txt":                            []byte(buildID),
		}
		if strings.HasSuffix(primaryAsset, ".zip") {
			writeTestZip(t, archivePath, files)
		} else {
			writeTestTarGz(t, archivePath, files, false)
		}
		b, err := os.ReadFile(archivePath)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		sums := []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), primaryAsset))
		sigMsg := append([]byte("horizon release "+buildID+"\n"), sums...)
		sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, sigMsg)) + "\n"
		return b, sums, sig
	}

	oldArchive, oldSums, oldSig := makeSignedArchive(buildOld)
	newArchive, newSums, newSig := makeSignedArchive(buildNew)

	clientBytes := []byte("fake-minecraft-client-jar")
	serverBytes := []byte("fake-minecraft-server-jar")
	clientSum := sha1.Sum(clientBytes)
	serverSum := sha1.Sum(serverBytes)
	clientSHA1 := hex.EncodeToString(clientSum[:])
	serverSHA1 := hex.EncodeToString(serverSum[:])

	var mu sync.Mutex
	publishNew := false
	offline := false

	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hasNew := publishNew
		isOffline := offline
		mu.Unlock()
		if isOffline {
			http.Error(w, "offline", http.StatusServiceUnavailable)
			return
		}
		switch r.URL.Path {
		case "/api/releases":
			rels := []horizonGitHubRelease{}
			if hasNew {
				rels = append(rels, horizonGitHubRelease{
					TagName:     "nightly",
					Name:        nameNew,
					Prerelease:  true,
					PublishedAt: "2026-11-21T11:18:27Z",
					Assets: []struct {
						Name string `json:"name"`
						Size int64  `json:"size"`
					}{
						{Name: primaryAsset, Size: int64(len(newArchive))},
						{Name: "sdk.json", Size: 64},
					},
				}, horizonGitHubRelease{
					TagName:     "nightly-2026-11-20",
					Name:        nameOld,
					Prerelease:  true,
					PublishedAt: "2026-11-20T11:18:27Z",
					Assets: []struct {
						Name string `json:"name"`
						Size int64  `json:"size"`
					}{
						{Name: primaryAsset, Size: int64(len(oldArchive))},
					},
				})
			} else {
				rels = append(rels, horizonGitHubRelease{
					TagName:     "nightly",
					Name:        nameOld,
					Prerelease:  true,
					PublishedAt: "2026-11-20T11:18:27Z",
					Assets: []struct {
						Name string `json:"name"`
						Size int64  `json:"size"`
					}{
						{Name: primaryAsset, Size: int64(len(oldArchive))},
						{Name: "sdk.json", Size: 64},
					},
				})
			}
			_ = json.NewEncoder(w).Encode(rels)
		case "/download/nightly/sdk.json":
			_, _ = w.Write([]byte(`{"schema":1,"build":{"game":"26.3","date":"2026-11-21"}}`))
		case "/download/nightly/" + horizonSumsFile:
			if hasNew {
				_, _ = w.Write(newSums)
			} else {
				_, _ = w.Write(oldSums)
			}
		case "/download/nightly/" + horizonSigV2File:
			if hasNew {
				_, _ = w.Write([]byte(newSig))
			} else {
				_, _ = w.Write([]byte(oldSig))
			}
		case "/download/nightly/" + primaryAsset:
			if hasNew {
				_, _ = w.Write(newArchive)
			} else {
				_, _ = w.Write(oldArchive)
			}
		case "/download/nightly-2026-11-20/" + horizonSumsFile:
			_, _ = w.Write(oldSums)
		case "/download/nightly-2026-11-20/" + horizonSigV2File:
			_, _ = w.Write([]byte(oldSig))
		case "/download/nightly-2026-11-20/" + primaryAsset:
			_, _ = w.Write(oldArchive)
		case "/mojang/manifest.json":
			verJSON := mustVersionJSON(t, srv.URL, mcVersion, clientSHA1, int64(len(clientBytes)), serverSHA1, int64(len(serverBytes)))
			verSum := sha1.Sum(verJSON)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"latest": map[string]string{"release": mcVersion, "snapshot": mcVersion},
				"versions": []map[string]string{{
					"id":   mcVersion,
					"type": "release",
					"url":  srv.URL + "/mojang/26.3.json",
					"sha1": hex.EncodeToString(verSum[:]),
				}},
			})
		case "/mojang/26.3.json":
			_, _ = w.Write(mustVersionJSON(t, srv.URL, mcVersion, clientSHA1, int64(len(clientBytes)), serverSHA1, int64(len(serverBytes))))
		case "/mojang/client.jar":
			_, _ = w.Write(clientBytes)
		case "/mojang/server.jar":
			_, _ = w.Write(serverBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc.http = srv.Client()
	svc.horizonTest = &horizonTestConfig{
		apiURL:      srv.URL + "/api/releases",
		mirrorURL:   srv.URL + "/api/releases",
		downloadURL: srv.URL + "/download",
		manifestURL: srv.URL + "/mojang/manifest.json",
		keys:        []ed25519.PublicKey{pub},
	}
	settings := svc.settings()
	settings.JavaPath = mockJavaBin
	if err := svc.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}

	root := newTestRoot(t, svc, "HorizonUpgrade")
	// Add a non-Horizon version to verify rejection.
	writeTestFile(t, filepath.Join(root.Path, "versions", "1.21.4", "1.21.4.json"), jsonBytes(t, map[string]any{
		"id":        "1.21.4",
		"mainClass": "net.minecraft.client.main.Main",
	}))
	if _, err := svc.CheckHorizonUpdate(context.Background(), root.ID, "1.21.4", false); err == nil {
		t.Fatal("CheckHorizonUpdate accepted non-Horizon version")
	}
	if _, err := svc.UpgradeHorizon(root.ID, "1.21.4", buildOld); err == nil {
		t.Fatal("UpgradeHorizon accepted non-Horizon version")
	}

	// 1. Install the older Horizon build first.
	instJob, err := svc.InstallGame(root.ID, "", "", mcVersion, "horizon", buildOld)
	if err != nil {
		t.Fatalf("InstallGame(old): %v", err)
	}
	if j := waitForJob(t, svc, instJob, 20*time.Second); j.State != "completed" {
		t.Fatalf("install failed: state=%s msg=%q", j.State, j.Message)
	}
	verID := "horizon-" + buildOld
	vDir := filepath.Join(root.Path, "versions", verID)

	// Populate user instance data that MUST survive in-place upgrades untouched.
	writeTestFile(t, filepath.Join(vDir, "saves", "MyWorld", "level.dat"), []byte("world-bytes"))
	writeTestFile(t, filepath.Join(vDir, "horizon-mods", "crate.hmod"), []byte("hmod-bytes"))
	writeTestFile(t, filepath.Join(vDir, "horizon-mods.txt"), []byte("mod.crate:true\n"))
	writeTestFile(t, filepath.Join(vDir, "resourcepacks", "Faithful.zip"), []byte("rp-bytes"))
	writeTestFile(t, filepath.Join(vDir, "options.txt"), []byte("lang:zh_cn\n"))

	// Initially up to date.
	info, err := svc.CheckHorizonUpdate(context.Background(), root.ID, verID, true)
	if err != nil {
		t.Fatalf("CheckHorizonUpdate(initial): %v", err)
	}
	if info.CurrentBuild != buildOld || info.HasUpdate || info.Latest == nil || info.Latest.BuildID != buildOld {
		t.Fatalf("unexpected initial update info: %+v", info)
	}

	// 2. Publish the newer nightly build and verify CheckHorizonUpdate detects it.
	mu.Lock()
	publishNew = true
	mu.Unlock()

	info, err = svc.CheckHorizonUpdate(context.Background(), root.ID, verID, true)
	if err != nil {
		t.Fatalf("CheckHorizonUpdate(newer): %v", err)
	}
	if !info.HasUpdate || info.CurrentBuild != buildOld || info.Latest == nil || info.Latest.BuildID != buildNew {
		t.Fatalf("expected update to %s, got %+v", buildNew, info)
	}

	// 3. Upgrade in place to latest (targetBuild == "") even while the game is marked running.
	svc.mu.Lock()
	svc.running[runKey(root.ID, verID)] = &runningGame{}
	svc.mu.Unlock()
	defer func() {
		svc.mu.Lock()
		delete(svc.running, runKey(root.ID, verID))
		svc.mu.Unlock()
	}()

	upgJobID, err := svc.UpgradeHorizon(root.ID, verID, "")
	if err != nil {
		t.Fatalf("UpgradeHorizon(latest) while running: %v", err)
	}
	upgJob := waitForJob(t, svc, upgJobID, 20*time.Second)
	if upgJob.State != "completed" {
		t.Fatalf("UpgradeHorizon failed: state=%s msg=%q", upgJob.State, upgJob.Message)
	}
	if upgJob.TitleKey != "horizon.upgrade" || upgJob.TitleArg != buildNew {
		t.Fatalf("unexpected upgrade job title key/arg: key=%q arg=%q", upgJob.TitleKey, upgJob.TitleArg)
	}
	if upgJob.TargetRootID != root.ID || upgJob.TargetVersion != verID {
		t.Fatalf("unexpected job targets: root=%q ver=%q", upgJob.TargetRootID, upgJob.TargetVersion)
	}

	// Verify the SAME version folder now reports buildNew, and all user data stayed intact.
	var found *VersionSummary
	for _, vs := range scanVersions(root.Path) {
		if vs.ID == verID {
			cp := vs
			found = &cp
		}
	}
	if found == nil || found.Loader != "horizon" || found.LoaderVersion != buildNew || found.Minecraft != mcVersion {
		t.Fatalf("unexpected VersionSummary after upgrade: %+v", found)
	}
	for rel, want := range map[string]string{
		"saves/MyWorld/level.dat":    "world-bytes",
		"horizon-mods/crate.hmod":    "hmod-bytes",
		"horizon-mods.txt":           "mod.crate:true\n",
		"resourcepacks/Faithful.zip": "rp-bytes",
		"options.txt":                "lang:zh_cn\n",
	} {
		got, err := os.ReadFile(filepath.Join(vDir, filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Fatalf("instance data %s altered after upgrade: got=%q err=%v", rel, string(got), err)
		}
	}

	// CheckHorizonUpdate now reports up-to-date and both builds cached locally.
	info, err = svc.CheckHorizonUpdate(context.Background(), root.ID, verID, false)
	if err != nil {
		t.Fatalf("CheckHorizonUpdate(after upgrade): %v", err)
	}
	if info.HasUpdate || info.CurrentBuild != buildNew {
		t.Fatalf("expected up-to-date on %s, got %+v", buildNew, info)
	}
	if !slices.Contains(info.LocalBuilds, buildOld) || !slices.Contains(info.LocalBuilds, buildNew) {
		t.Fatalf("expected both builds cached locally, got %v", info.LocalBuilds)
	}

	// 4. Re-installing the old build after in-place upgrade must NOT clobber
	// the upgraded horizon-<buildOld> instance; it must create a distinct folder.
	reinstJobID, err := svc.InstallGame(root.ID, "", "", mcVersion, "horizon", buildOld)
	if err != nil {
		t.Fatalf("InstallGame(old after upgrade): %v", err)
	}
	if j := waitForJob(t, svc, reinstJobID, 20*time.Second); j.State != "completed" {
		t.Fatalf("re-install of old build failed: state=%s msg=%q", j.State, j.Message)
	}
	_, upgradedMeta, err := findVersionJSON(root.Path, verID)
	if err != nil || upgradedMeta.Horizon == nil || upgradedMeta.Horizon.BuildID != buildNew {
		t.Fatalf("re-installing old build clobbered upgraded instance %s: meta=%+v err=%v", verID, upgradedMeta.Horizon, err)
	}
	altVerID := verID + "-2"
	_, altMeta, err := findVersionJSON(root.Path, altVerID)
	if err != nil || altMeta.Horizon == nil || altMeta.Horizon.BuildID != buildOld {
		t.Fatalf("expected separate instance %s for build %s, got %+v err=%v", altVerID, buildOld, altMeta.Horizon, err)
	}

	// 5. After clearing the in-memory release cache and going completely offline:
	// - quiet CheckHorizonUpdate (force=false) still lists locally cached builds;
	// - switching back to the cached older build succeeds and preserves release metadata.
	mu.Lock()
	offline = true
	mu.Unlock()
	svc.horizonMu.Lock()
	svc.horizonCache = nil
	svc.horizonMu.Unlock()

	offlineInfo, err := svc.CheckHorizonUpdate(context.Background(), root.ID, verID, false)
	if err != nil {
		t.Fatalf("CheckHorizonUpdate(offline quiet): %v", err)
	}
	if offlineInfo.CurrentBuild != buildNew || !slices.Contains(offlineInfo.LocalBuilds, buildOld) || !slices.Contains(offlineInfo.LocalBuilds, buildNew) {
		t.Fatalf("offline check hid cached Horizon builds: %+v", offlineInfo)
	}
	if _, err := svc.CheckHorizonUpdate(context.Background(), root.ID, verID, true); err == nil {
		t.Fatal("manual offline CheckHorizonUpdate(force=true) should still report network failure")
	}

	rollJobID, err := svc.UpgradeHorizon(root.ID, verID, buildOld)
	if err != nil {
		t.Fatalf("UpgradeHorizon(cached rollback): %v", err)
	}
	if j := waitForJob(t, svc, rollJobID, 20*time.Second); j.State != "completed" {
		t.Fatalf("cached rollback failed: state=%s msg=%q", j.State, j.Message)
	}
	_, revertedMeta, err := findVersionJSON(root.Path, verID)
	if err != nil || revertedMeta.Horizon == nil || revertedMeta.Horizon.BuildID != buildOld {
		t.Fatalf("expected build reverted to %s, got %+v err=%v", buildOld, revertedMeta.Horizon, err)
	}
	if revertedMeta.Horizon.ReleaseTag == "" || revertedMeta.Horizon.ReleaseName != nameOld || revertedMeta.Horizon.Asset != primaryAsset {
		t.Fatalf("cached upgrade wiped release metadata: %+v", revertedMeta.Horizon)
	}
}
