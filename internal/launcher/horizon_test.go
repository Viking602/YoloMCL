package launcher

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestHorizonPlatformAssets(t *testing.T) {
	cases := []struct {
		goos, goarch      string
		primary, fallback string
		ok                bool
	}{
		{"windows", "amd64", "horizon-windows-x64.zip", "", true},
		{"windows", "arm64", "horizon-windows-arm64.zip", "horizon-windows-x64.zip", true},
		{"linux", "amd64", "horizon-linux-x86_64.tar.gz", "", true},
		{"darwin", "arm64", "horizon-macos-arm64.zip", "", true},
		{"darwin", "amd64", "horizon-macos-x64.zip", "", true},
		{"linux", "arm64", "", "", false},
	}
	for _, tc := range cases {
		p, fb, ok := horizonPlatformAssets(tc.goos, tc.goarch)
		if p != tc.primary || fb != tc.fallback || ok != tc.ok {
			t.Errorf("horizonPlatformAssets(%s, %s) = (%q, %q, %v), want (%q, %q, %v)",
				tc.goos, tc.goarch, p, fb, ok, tc.primary, tc.fallback, tc.ok)
		}
	}
}

func TestHorizonReleaseParsingAndOrdering(t *testing.T) {
	raw := []horizonGitHubRelease{
		{
			TagName:     "launcher@0.2.93",
			Name:        "Horizon launcher 0.2.93",
			PublishedAt: "2026-11-20T12:00:00Z",
			Assets: []struct {
				Name string `json:"name"`
				Size int64  `json:"size"`
			}{{Name: "horizon-windows-x64.zip", Size: 10}},
		},
		{
			TagName:     "sdk-0a875da91d",
			Name:        "Mod SDK 0a875da91d",
			PublishedAt: "2026-11-20T11:30:00Z",
			Assets: []struct {
				Name string `json:"name"`
				Size int64  `json:"size"`
			}{{Name: "horizon-windows-x64.zip", Size: 10}},
		},
		{
			TagName:     "nightly-2026-11-19",
			Name:        "Nightly 2026-11-19 · 5ef7734",
			Prerelease:  true,
			PublishedAt: "2026-11-20T02:32:23Z",
			Assets: []struct {
				Name string `json:"name"`
				Size int64  `json:"size"`
			}{{Name: "horizon-windows-x64.zip", Size: 100}},
		},
		{
			TagName:     "nightly-2026-11-20",
			Name:        "Nightly 2026-11-20 · 0a875da",
			Prerelease:  true,
			PublishedAt: "2026-11-20T11:00:00Z",
			Assets: []struct {
				Name string `json:"name"`
				Size int64  `json:"size"`
			}{{Name: "horizon-windows-x64.zip", Size: 200}},
		},
		{
			TagName:     "nightly",
			Name:        "Nightly 2026-11-20 · 0a875da",
			Prerelease:  true,
			PublishedAt: "2026-11-20T11:18:27Z",
			Assets: []struct {
				Name string `json:"name"`
				Size int64  `json:"size"`
			}{{Name: "horizon-windows-x64.zip", Size: 200}, {Name: "sdk.json", Size: 7000}},
		},
		{
			TagName:     "nightly-2026-11-18",
			Name:        "Snapshot 2026-11-18 · aaaaaaa", // wrong title prefix for nightly tag
			Prerelease:  true,
			PublishedAt: "2026-11-18T11:00:00Z",
			Assets: []struct {
				Name string `json:"name"`
				Size int64  `json:"size"`
			}{{Name: "horizon-windows-x64.zip", Size: 50}},
		},
		{
			TagName:     "v0.3.0",
			Name:        "Horizon 0.3.0",
			Draft:       true,
			PublishedAt: "2026-11-21T00:00:00Z",
			Assets: []struct {
				Name string `json:"name"`
				Size int64  `json:"size"`
			}{{Name: "horizon-windows-x64.zip", Size: 300}},
		},
	}

	got := parseHorizonReleases(raw, "horizon-windows-arm64.zip", "horizon-windows-x64.zip", horizonDownloadBaseURL)
	if len(got) != 2 {
		t.Fatalf("expected 2 releases, got %d: %+v", len(got), got)
	}
	if got[0].BuildID != "nightly-2026-11-20-0a875da" || got[0].ReleaseTag != "nightly" || !got[0].HasSDKJSON {
		t.Fatalf("unexpected first release: %+v", got[0])
	}
	if got[0].AssetName != "horizon-windows-x64.zip" {
		t.Fatalf("expected x64 fallback asset on windows/arm64, got %q", got[0].AssetName)
	}
	if got[1].BuildID != "nightly-2026-11-19-5ef7734" || got[1].ReleaseTag != "nightly-2026-11-19" {
		t.Fatalf("unexpected second release: %+v", got[1])
	}

	escaped := horizonAssetURL(horizonDownloadBaseURL, "v1/../../other?x#y", "a%b.zip")
	wantURL := "https://github.com/horizon-rust/horizon/releases/download/v1%2F..%2F..%2Fother%3Fx%23y/a%25b.zip"
	if escaped != wantURL {
		t.Fatalf("horizonAssetURL = %q, want %q", escaped, wantURL)
	}

	// Explicit buildID must not silently fall back to the moving "nightly" tag.
	if _, ok := selectHorizonRelease(got, "nightly-2026-11-01-deadbee", "nightly"); ok {
		t.Fatal("selectHorizonRelease unexpectedly substituted a different nightly commit")
	}
	if sel, ok := selectHorizonRelease(got, "", "nightly"); !ok || sel.BuildID != "nightly-2026-11-20-0a875da" {
		t.Fatalf("selectHorizonRelease by tag = (%+v, %v)", sel, ok)
	}
}

func TestHorizonSignatureVerification(t *testing.T) {
	// Built-in release key must parse cleanly.
	builtinKeys, err := horizonTrustedKeys(nil)
	if err != nil || len(builtinKeys) == 0 {
		t.Fatalf("built-in keys invalid: err=%v len=%d", err, len(builtinKeys))
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	digest := sha256.Sum256([]byte("archive-bytes"))
	hexDigest := hex.EncodeToString(digest[:])
	sums := []byte(fmt.Sprintf("%s  horizon-windows-x64.zip\n%s *horizon-linux-x86_64.tar.gz\n", hexDigest, strings.ToUpper(hexDigest)))

	parsed, err := parseHorizonSHA256SUMS(string(sums))
	if err != nil {
		t.Fatal(err)
	}
	if parsed["horizon-windows-x64.zip"] != hexDigest || parsed["horizon-linux-x86_64.tar.gz"] != hexDigest {
		t.Fatalf("unexpected parsed sums: %+v", parsed)
	}

	sign := func(tag string, data []byte) string {
		msg := append([]byte("horizon release "+tag+"\n"), data...)
		return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, msg)) + "\n"
	}

	const version = "nightly-2026-11-20-0a875da"
	sig := sign(version, sums)
	if err := verifyHorizonReleaseSignature(version, sums, sig, []ed25519.PublicKey{pub}); err != nil {
		t.Fatalf("valid signature failed: %v", err)
	}
	// Wrong tag (replay under another version or channel tag) must fail.
	if err := verifyHorizonReleaseSignature("nightly", sums, sig, []ed25519.PublicKey{pub}); err == nil {
		t.Fatal("channel tag unexpectedly accepted version signature")
	}
	if err := verifyHorizonReleaseSignature("nightly-2026-11-21-1111111", sums, sig, []ed25519.PublicKey{pub}); err == nil {
		t.Fatal("different nightly version unexpectedly accepted signature")
	}
	// Tampered checksums must fail.
	tampered := bytes.Replace(sums, []byte(hexDigest[:8]), []byte("00000000"), 1)
	if err := verifyHorizonReleaseSignature(version, tampered, sig, []ed25519.PublicKey{pub}); err == nil {
		t.Fatal("tampered checksums unexpectedly verified")
	}
	// Wrong key must fail.
	if err := verifyHorizonReleaseSignature(version, sums, sig, []ed25519.PublicKey{otherPub}); err == nil {
		t.Fatal("untrusted key unexpectedly verified")
	}
	// Empty key set fails closed.
	if err := verifyHorizonReleaseSignature(version, sums, sig, nil); err == nil {
		t.Fatal("empty key list unexpectedly verified")
	}
}

func TestHorizonArchiveExtractionAndSafety(t *testing.T) {
	ctx := context.Background()

	// Valid .zip with single top folder.
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "ok.zip")
	writeTestZip(t, zipPath, map[string][]byte{
		"horizon-windows-x64/horizon-client.exe": []byte("bin"),
		"horizon-windows-x64/research-lock.json": []byte(`{"minecraft":{"version":"26.3"}}`),
	})
	destZip := filepath.Join(tmp, "out-zip")
	if err := extractHorizonArchive(ctx, zipPath, "horizon-windows-x64.zip", destZip); err != nil {
		t.Fatal(err)
	}
	topZip, err := unwrapSingleTopDir(destZip)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(topZip, "horizon-client.exe")); err != nil {
		t.Fatalf("expected extracted client binary: %v", err)
	}

	// Valid .tar.gz with single top folder.
	tgzPath := filepath.Join(tmp, "ok.tar.gz")
	writeTestTarGz(t, tgzPath, map[string][]byte{
		"horizon-linux-x86_64/horizon-client":     []byte("elf"),
		"horizon-linux-x86_64/research-lock.json": []byte(`{"minecraft":{"version":"26.3"}}`),
	}, false)
	destTgz := filepath.Join(tmp, "out-tgz")
	if err := extractHorizonArchive(ctx, tgzPath, "horizon-linux-x86_64.tar.gz", destTgz); err != nil {
		t.Fatal(err)
	}
	topTgz, err := unwrapSingleTopDir(destTgz)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(topTgz, "horizon-client")); err != nil {
		t.Fatalf("expected extracted tar client binary: %v", err)
	}

	// ZipSlip must be rejected.
	badZip := filepath.Join(tmp, "evil.zip")
	writeTestZip(t, badZip, map[string][]byte{"../escape.txt": []byte("evil")})
	if err := extractHorizonArchive(ctx, badZip, "evil.zip", filepath.Join(tmp, "evil-zip-out")); err == nil {
		t.Fatal("expected ZipSlip archive to fail")
	}

	// Tar symlink must be rejected.
	badTar := filepath.Join(tmp, "symlink.tar.gz")
	writeTestTarGz(t, badTar, map[string][]byte{"horizon/a": []byte("x")}, true)
	if err := extractHorizonArchive(ctx, badTar, "symlink.tar.gz", filepath.Join(tmp, "evil-tar-out")); err == nil {
		t.Fatal("expected symlink tar archive to fail")
	}
}

func TestHorizonInstallPrepareLaunchEndToEnd(t *testing.T) {
	primaryAsset, _, ok := horizonPlatformAssets(runtime.GOOS, runtime.GOARCH)
	if !ok {
		t.Skipf("no Horizon release asset defined for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Compile small real test helper binaries for fake java, horizon-prepare, and horizon-client.
	binDir := t.TempDir()
	mockHorizonBin := buildHorizonMockBinary(t, binDir, "mock-horizon")
	mockJavaBin := buildMockJava25Binary(t, binDir)
	mockBinBytes, err := os.ReadFile(mockHorizonBin)
	if err != nil {
		t.Fatal(err)
	}

	// Package the mock Horizon release archive for the current OS/arch.
	const (
		buildID   = "nightly-2026-11-20-0a875da"
		relName   = "Nightly 2026-11-20 · 0a875da"
		mcVersion = "26.3"
	)
	archivePath := filepath.Join(binDir, primaryAsset)
	lockJSON := []byte(`{"minecraft":{"version":"26.3","java_major":25}}`)
	topName := strings.TrimSuffix(strings.TrimSuffix(primaryAsset, ".zip"), ".tar.gz")
	files := map[string][]byte{
		topName + "/" + horizonBinaryName("horizon-client"):  mockBinBytes,
		topName + "/" + horizonBinaryName("horizon-prepare"): mockBinBytes,
		topName + "/research-lock.json":                      lockJSON,
	}
	if strings.HasSuffix(primaryAsset, ".zip") {
		writeTestZip(t, archivePath, files)
	} else {
		writeTestTarGz(t, archivePath, files, false)
	}
	archiveBytes, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	archiveSum := sha256.Sum256(archiveBytes)
	archiveSHA256 := hex.EncodeToString(archiveSum[:])

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sumsBytes := []byte(fmt.Sprintf("%s  %s\n", archiveSHA256, primaryAsset))
	sigMsg := append([]byte("horizon release "+buildID+"\n"), sumsBytes...)
	sigV2 := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, sigMsg)) + "\n"

	// Mock vanilla 26.3 client/server/version JSON.
	clientBytes := []byte("fake-minecraft-client-jar")
	serverBytes := []byte("fake-minecraft-server-jar")
	clientSum := sha1.Sum(clientBytes)
	serverSum := sha1.Sum(serverBytes)
	clientSHA1 := hex.EncodeToString(clientSum[:])
	serverSHA1 := hex.EncodeToString(serverSum[:])

	var srv *httptest.Server
	mirrorHits := 0
	archiveDownloads := 0
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/releases":
			// Simulate GitHub rate-limit so mirror fallback is also verified.
			http.Error(w, "rate limit exceeded", http.StatusForbidden)
		case "/mirror/releases":
			mirrorHits++
			_ = json.NewEncoder(w).Encode([]horizonGitHubRelease{{
				TagName:     "nightly",
				Name:        relName,
				Prerelease:  true,
				PublishedAt: "2026-11-20T11:18:27Z",
				Assets: []struct {
					Name string `json:"name"`
					Size int64  `json:"size"`
				}{
					{Name: primaryAsset, Size: int64(len(archiveBytes))},
					{Name: "sdk.json", Size: 64},
				},
			}})
		case "/download/nightly/sdk.json":
			_, _ = w.Write([]byte(`{"schema":1,"build":{"game":"26.3","date":"2026-11-20"}}`))
		case "/download/nightly/" + horizonSumsFile:
			_, _ = w.Write(sumsBytes)
		case "/download/nightly/" + horizonSigV2File:
			_, _ = w.Write([]byte(sigV2))
		case "/download/nightly/" + primaryAsset:
			archiveDownloads++
			_, _ = w.Write(archiveBytes)
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

	dataDir := t.TempDir()
	svc, err := New(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	svc.http = srv.Client()
	svc.horizonTest = &horizonTestConfig{
		apiURL:      srv.URL + "/api/releases",
		mirrorURL:   srv.URL + "/mirror/releases",
		downloadURL: srv.URL + "/download",
		manifestURL: srv.URL + "/mojang/manifest.json",
		keys:        []ed25519.PublicKey{pub},
	}

	// Configure fake Java 25 so ensureJava resolves it deterministically.
	settings := svc.settings()
	settings.JavaPath = mockJavaBin
	if err := svc.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}

	// Verify LoaderVersions filtering & mirror fallback.
	ctx := context.Background()
	vers263, err := svc.LoaderVersions(ctx, "horizon", "26.3")
	if err != nil {
		t.Fatalf("LoaderVersions(horizon, 26.3): %v", err)
	}
	if !slices.Equal(vers263, []string{buildID}) {
		t.Fatalf("LoaderVersions(horizon, 26.3) = %v, want [%s]", vers263, buildID)
	}
	if mirrorHits == 0 {
		t.Fatal("expected mirror fallback when GitHub API returned 403")
	}
	versOther, err := svc.LoaderVersions(ctx, "horizon", "1.21.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(versOther) != 0 {
		t.Fatalf("expected no Horizon versions for 1.21.1, got %v", versOther)
	}
	if gotMC := svc.HorizonMinecraftVersion(ctx); gotMC != "26.3" {
		t.Fatalf("HorizonMinecraftVersion() = %q, want 26.3", gotMC)
	}

	// Prepare root with an unrelated Java mod in shared mods/ to prove Horizon stays isolated.
	gameRoot := filepath.Join(t.TempDir(), ".minecraft")
	if err := os.MkdirAll(filepath.Join(gameRoot, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameRoot, "mods", "java-mod.jar"), []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := svc.AddRoot(gameRoot, "Horizon Test")
	if err != nil {
		t.Fatal(err)
	}

	// Install Horizon into the root.
	jobID, err := svc.InstallGame(root.ID, "", "", "26.3", "horizon", buildID)
	if err != nil {
		t.Fatalf("InstallGame(horizon): %v", err)
	}
	job := waitForJob(t, svc, jobID, 20*time.Second)
	if job.State != "completed" {
		t.Fatalf("Horizon install job state=%s msg=%q", job.State, job.Message)
	}
	if job.TitleKey != "horizon.install" {
		t.Fatalf("expected TitleKey horizon.install, got %q", job.TitleKey)
	}

	// Verify version scan, loader metadata, and isolated VersionDir.
	updatedRoot, err := svc.getRoot(root.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantVerID := "horizon-" + buildID
	if updatedRoot.SelectedVersion != wantVerID {
		t.Fatalf("SelectedVersion = %q, want %q", updatedRoot.SelectedVersion, wantVerID)
	}
	wantGameDir := filepath.Join(updatedRoot.Path, "versions", wantVerID)
	if filepath.Clean(updatedRoot.VersionDir()) != filepath.Clean(wantGameDir) {
		t.Fatalf("VersionDir() = %q, want isolated %q", updatedRoot.VersionDir(), wantGameDir)
	}
	scanned := scanVersions(updatedRoot.Path)
	var found *VersionSummary
	for i := range scanned {
		if scanned[i].ID == wantVerID {
			found = &scanned[i]
		}
	}
	if found == nil {
		t.Fatalf("installed Horizon version %q not found in scanVersions: %+v", wantVerID, scanned)
	}
	if found.Loader != "horizon" || found.LoaderVersion != buildID || found.Minecraft != "26.3" || found.JavaVersion != 25 {
		t.Fatalf("unexpected VersionSummary: %+v", *found)
	}

	// Launch with an offline account must be refused by the Horizon owner gate.
	offlineAcct, err := svc.AddOffline("OfflinePlayer")
	if err != nil {
		t.Fatal(err)
	}
	offJobID, err := svc.Launch(root.ID, offlineAcct.ID)
	if err != nil {
		t.Fatal(err)
	}
	offJob := waitForJob(t, svc, offJobID, 10*time.Second)
	if offJob.State != "failed" || !strings.Contains(offJob.Message, "Microsoft") {
		t.Fatalf("expected offline Horizon launch to fail with Microsoft requirement, got state=%s msg=%q", offJob.State, offJob.Message)
	}

	// Save a mock Microsoft account with non-expiring token in vault and launch Horizon.
	const secretToken = "msa-super-secret-access-token-12345"
	msAcct := Account{ID: "ms-069a79f444e94726a5befca90e38aaf5", Name: "Alex", UUID: "069a79f444e94726a5befca90e38aaf5", Kind: "microsoft"}
	if err := svc.saveAccount(msAcct, tokenSecret{
		Access:  secretToken,
		Refresh: "refresh-token",
		Expires: time.Now().Add(2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	launchJobID, err := svc.Launch(root.ID, msAcct.ID)
	if err != nil {
		t.Fatalf("Launch(horizon): %v", err)
	}
	launchJob := waitForJob(t, svc, launchJobID, 15*time.Second)
	if launchJob.State != "completed" {
		t.Fatalf("Horizon launch job failed: state=%s msg=%q", launchJob.State, launchJob.Message)
	}

	// Wait for the mock horizon-client to write its launch report and exit cleanly.
	reportPath := filepath.Join(wantGameDir, "mock-launch-report.json")
	deadline := time.Now().Add(10 * time.Second)
	var reportBytes []byte
	for time.Now().Before(deadline) {
		reportBytes, err = os.ReadFile(reportPath)
		if err == nil && len(reportBytes) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("mock horizon-client did not write launch report: %v", err)
	}
	var report struct {
		Args        []string `json:"args"`
		Stdin       string   `json:"stdin"`
		HorizonHome string   `json:"horizonHome"`
	}
	if err := json.Unmarshal(reportBytes, &report); err != nil {
		t.Fatal(err)
	}
	// Verify token never appeared in CLI args.
	for _, arg := range report.Args {
		if strings.Contains(arg, secretToken) {
			t.Fatalf("access token leaked in command line argument: %q", arg)
		}
	}
	for _, flag := range []string{"--resources", "--game-dir", "--home", "--session-stdin"} {
		if !slices.Contains(report.Args, flag) {
			t.Fatalf("missing expected launch flag %s in args %v", flag, report.Args)
		}
	}
	if filepath.Clean(report.HorizonHome) != filepath.Clean(wantGameDir) {
		t.Fatalf("HORIZON_HOME = %q, want %q", report.HorizonHome, wantGameDir)
	}
	var sess struct {
		Name        string `json:"name"`
		UUID        string `json:"uuid"`
		AccessToken string `json:"access_token"`
		UserType    string `json:"user_type"`
	}
	if err := json.Unmarshal([]byte(report.Stdin), &sess); err != nil {
		t.Fatalf("invalid session JSON on stdin %q: %v", report.Stdin, err)
	}
	if sess.Name != "Alex" || sess.UUID != msAcct.UUID || sess.AccessToken != secretToken || sess.UserType != "msa" {
		t.Fatalf("unexpected session JSON on stdin: %+v", sess)
	}

	// Wait for process exit and check secret redaction in session logs.
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if len(svc.Sessions(root.ID)) > 0 && svc.Sessions(root.ID)[0].State != "running" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	sessions := svc.Sessions(root.ID)
	if len(sessions) == 0 || sessions[0].State != "exited" {
		t.Fatalf("unexpected session state: %+v", sessions)
	}
	if sessions[0].Java != "Horizon (Rust)" {
		t.Fatalf("expected session Java label Horizon (Rust), got %q", sessions[0].Java)
	}
	logs, err := svc.Logs(root.ID)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(logs, "\n")
	if strings.Contains(joined, secretToken) {
		t.Fatalf("secret token was not redacted in game logs: %s", joined)
	}
	if !strings.Contains(joined, "[REDACTED]") {
		t.Fatalf("expected redacted token marker in game logs: %s", joined)
	}

	// Installing into a second empty root must reuse the cached Horizon build
	// and prepared resources without re-downloading the archive or vanilla 26.3.
	secondParent := t.TempDir()
	job2ID, err := svc.InstallGame("", secondParent, "HorizonSecond", "26.3", "horizon", buildID)
	if err != nil {
		t.Fatalf("second InstallGame(horizon): %v", err)
	}
	job2 := waitForJob(t, svc, job2ID, 15*time.Second)
	if job2.State != "completed" || job2.ResultRoot == "" {
		t.Fatalf("second Horizon install failed: state=%s msg=%q resultRoot=%q", job2.State, job2.Message, job2.ResultRoot)
	}
	if archiveDownloads != 1 {
		t.Fatalf("expected cached Horizon build to be reused (archiveDownloads=%d, want 1)", archiveDownloads)
	}
	secondRoot, err := svc.getRoot(job2.ResultRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(secondRoot.Path, "versions", "26.3", "26.3.json")); !os.IsNotExist(err) {
		t.Fatalf("expected second Horizon root to reuse prepared resources without downloading vanilla 26.3 into it")
	}
	secondScan := scanVersions(secondRoot.Path)
	if len(secondScan) != 1 || secondScan[0].Loader != "horizon" || secondScan[0].Minecraft != "26.3" {
		t.Fatalf("unexpected versions in second Horizon root: %+v", secondScan)
	}
}

func TestHorizonModInstallRejected(t *testing.T) {
	svc, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rootDir := filepath.Join(t.TempDir(), ".minecraft")
	verID := "horizon-nightly-2026-11-20-0a875da"
	verPath := filepath.Join(rootDir, "versions", verID, verID+".json")
	if err := atomicJSON(verPath, gameVersion{
		ID:            verID,
		ClientVersion: "26.3",
		Horizon:       &horizonVersionMeta{BuildID: "nightly-2026-11-20-0a875da", Minecraft: "26.3", JavaMajor: 25},
	}); err != nil {
		t.Fatal(err)
	}
	root, err := svc.AddRoot(rootDir, "Horizon")
	if err != nil {
		t.Fatal(err)
	}
	err = svc.installModIn(context.Background(), func(string, int, int) {}, root, "modrinth", "p", "v", "", nil)
	if err == nil {
		t.Fatal("expected installing Java mod into Horizon instance to fail")
	}
	var loc interface{ LocalizedMessage(bool) string }
	if !errors.As(err, &loc) || !strings.Contains(loc.LocalizedMessage(true), "Horizon") {
		t.Fatalf("expected localized Horizon error, got %v", err)
	}
}

func TestDetectLoaderDoesNotConfuseHorizonModpackNames(t *testing.T) {
	if got := detectLoader(gameVersion{ID: "Create New Horizon"}); got != "vanilla" {
		t.Fatalf("detectLoader(Create New Horizon) = %q, want vanilla", got)
	}
	if got := detectLoader(gameVersion{ID: "Sky New Horizons"}); got != "vanilla" {
		t.Fatalf("detectLoader(Sky New Horizons) = %q, want vanilla", got)
	}
	if got := detectLoader(gameVersion{ID: "horizon-nightly-2026-11-20-0a875da"}); got != "horizon" {
		t.Fatalf("detectLoader(horizon-nightly-...) = %q, want horizon", got)
	}
}

func TestNeoForgeVersionPrefix(t *testing.T) {
	cases := []struct {
		mc     string
		prefix string
		ok     bool
	}{
		{"1.19.4", "", false},
		{"1.20.1", "", false},
		{"1.20.4", "20.4.", true},
		{"1.21", "21.0.", true},
		{"1.21.1", "21.1.", true},
		{"26.1", "26.1.0.", true},
		{"26.1.2", "26.1.2.", true},
		{"26.2", "26.2.0.", true},
		{"26.3", "26.3.0.", true},
		{"24w14a", "", false},
		{"b1.7.3", "", false},
	}
	for _, tc := range cases {
		got, ok := neoforgeVersionPrefix(tc.mc)
		if got != tc.prefix || ok != tc.ok {
			t.Errorf("neoforgeVersionPrefix(%q) = (%q, %v), want (%q, %v)", tc.mc, got, ok, tc.prefix, tc.ok)
		}
	}
}

func waitForJob(t *testing.T, svc *Service, id string, timeout time.Duration) Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snap := svc.Snapshot()
		for _, j := range snap.Jobs {
			if j.ID == id && j.State != "running" {
				return j
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish within %v", id, timeout)
	return Job{}
}

func mustVersionJSON(t *testing.T, baseURL, mc, clientSHA1 string, clientSize int64, serverSHA1 string, serverSize int64) []byte {
	t.Helper()
	v := map[string]any{
		"id":          mc,
		"type":        "release",
		"mainClass":   "net.minecraft.client.main.Main",
		"javaVersion": map[string]int{"majorVersion": 25},
		"libraries":   []any{},
		"downloads": map[string]any{
			"client": map[string]any{"url": baseURL + "/mojang/client.jar", "sha1": clientSHA1, "size": clientSize},
			"server": map[string]any{"url": baseURL + "/mojang/server.jar", "sha1": serverSHA1, "size": serverSize},
		},
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeTestZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, data := range files {
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
		hdr.SetMode(0o755)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTestTarGz(t *testing.T, path string, files map[string][]byte, addSymlink bool) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, data := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o755,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if addSymlink {
		if err := tw.WriteHeader(&tar.Header{
			Name:     "horizon/link",
			Linkname: "/etc/passwd",
			Typeflag: tar.TypeSymlink,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func buildHorizonMockBinary(t *testing.T, dir, name string) string {
	t.Helper()
	srcDir := filepath.Join(dir, name+"-src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const src = `package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func argVal(flag string) string {
	for i := 1; i < len(os.Args)-1; i++ {
		if os.Args[i] == flag {
			return os.Args[i+1]
		}
	}
	return ""
}

func hasFlag(flag string) bool {
	for _, a := range os.Args[1:] {
		if a == flag {
			return true
		}
	}
	return false
}

func main() {
	base := strings.ToLower(filepath.Base(os.Args[0]))
	if strings.Contains(base, "horizon-prepare") {
		if hasFlag("--launch-api") {
			fmt.Println(` + "`" + `{"launch_api":"0.1.1","horizon":"nightly-2026-11-20-0a875da","minecraft":"26.3","java_major":25,"schema":12,"output_version":3,"options":[{"name":"--prepare","value":null},{"name":"--server-jar","value":"PATH"},{"name":"--minecraft-dir","value":"PATH"},{"name":"--version-json","value":"PATH"},{"name":"--client-jar","value":"PATH"},{"name":"--libraries-dir","value":"PATH"},{"name":"--assets-dir","value":"PATH"},{"name":"--java-home","value":"PATH"},{"name":"--root","value":"PATH"},{"name":"--output","value":"PATH"},{"name":"--check","value":null},{"name":"--complete","value":null},{"name":"--progress","value":"json"},{"name":"--launch-api","value":null}],"exit_codes":[0,1,2]}` + "`" + `)
			return
		}
		outDir := argVal("--output")
		marker := filepath.Join(outDir, "horizon-resources.json")
		if hasFlag("--check") {
			if _, err := os.Stat(marker); err == nil {
				return
			}
			fmt.Println("not prepared: horizon-prepare --prepare")
			os.Exit(1)
		}
		if hasFlag("--prepare") {
			if _, err := os.Stat(argVal("--server-jar")); err != nil {
				fmt.Println(` + "`" + `{"error":"missing server jar"}` + "`" + `)
				os.Exit(1)
			}
			if _, err := os.Stat(argVal("--client-jar")); err != nil {
				fmt.Println(` + "`" + `{"error":"missing client jar"}` + "`" + `)
				os.Exit(1)
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				os.Exit(1)
			}
			fmt.Println(` + "`" + `{"message":"Importing 26.3","fraction":0.5}` + "`" + `)
			if err := os.WriteFile(marker, []byte(` + "`" + `{"schema":12,"minecraft":"26.3"}` + "`" + `), 0o644); err != nil {
				os.Exit(1)
			}
			fmt.Println(` + "`" + `{"message":"Done","fraction":1.0}` + "`" + `)
			return
		}
		os.Exit(2)
	}
	if hasFlag("--launch-api") {
		fmt.Println(` + "`" + `{"launch_api":"0.1.1","horizon":"nightly-2026-11-20-0a875da","minecraft":"26.3","crash_report_line":"Horizon: crash report saved to ","exit_codes":[0,1,2,101],"options":[{"name":"--resources","value":"DIR"},{"name":"--game-dir","value":"DIR"},{"name":"--home","value":"DIR"},{"name":"--width","value":"N"},{"name":"--height","value":"N"},{"name":"--fullscreen","value":null},{"name":"--session-stdin","value":null},{"name":"--launch-api","value":null}]}` + "`" + `)
		return
	}
	var stdinText string
	if hasFlag("--session-stdin") {
		b, _ := io.ReadAll(os.Stdin)
		stdinText = string(b)
	}
	gameDir := argVal("--game-dir")
	var sess struct {
		AccessToken string ` + "`json:\"access_token\"`" + `
	}
	_ = json.Unmarshal([]byte(stdinText), &sess)
	fmt.Println("Horizon: first frame")
	if sess.AccessToken != "" {
		fmt.Println("debug echo " + sess.AccessToken)
	}
	rep, _ := json.Marshal(map[string]any{
		"args":        os.Args[1:],
		"stdin":       stdinText,
		"horizonHome": os.Getenv("HORIZON_HOME"),
	})
	_ = os.WriteFile(filepath.Join(gameDir, "mock-launch-report.json"), rep, 0o644)
}
`
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, horizonBinaryName(name))
	cmd := exec.Command("go", "build", "-o", outPath, filepath.Join(srcDir, "main.go"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build mock horizon binary: %v\n%s", err, string(out))
	}
	return outPath
}

func buildMockJava25Binary(t *testing.T, dir string) string {
	t.Helper()
	javaHomeDir := filepath.Join(dir, "jdk-25")
	binDir := filepath.Join(javaHomeDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Join(dir, "mock-java-src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const src = `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, ` + "`" + `openjdk version "25.0.1" 2026-10-20` + "`" + `)
}
`
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(binDir, horizonBinaryName("java"))
	cmd := exec.Command("go", "build", "-o", outPath, filepath.Join(srcDir, "main.go"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build mock java binary: %v\n%s", err, string(out))
	}
	return outPath
}
