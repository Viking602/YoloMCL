package launcher

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBlake2b512Vectors(t *testing.T) {
	empty := blake2b512(nil)
	wantEmpty := "786a02f742015903c6c6fd852552d272912f4740e15847618a86e217f71f5419d25e1031afee585313896444934eb04b903a685b1448b755d56f701afe9be2ce"
	if got := hex.EncodeToString(empty[:]); got != wantEmpty {
		t.Fatalf("blake2b512(\"\") = %s, want %s", got, wantEmpty)
	}
	abc := blake2b512([]byte("abc"))
	wantABC := "ba80a53f981c4d0d6a2797b69f12f6e94c212f14685ac4b74b12bb6fdbffa2d17d87c5392aab792dc252d5de4533cc9518d38aa8dbf1925ab92386edd4009923"
	if got := hex.EncodeToString(abc[:]); got != wantABC {
		t.Fatalf("blake2b512(\"abc\") = %s, want %s", got, wantABC)
	}
}

func makeMinisignPair(t *testing.T) (pubB64 string, sign func(data []byte, prehash bool, trustedComment string) string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyID := [8]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}
	pubRaw := make([]byte, 42)
	pubRaw[0], pubRaw[1] = 'R', 'W'
	copy(pubRaw[2:10], keyID[:])
	copy(pubRaw[10:42], pub)
	pubB64 = base64.StdEncoding.EncodeToString(pubRaw)

	sign = func(data []byte, prehash bool, trustedComment string) string {
		sigRaw := make([]byte, 74)
		var msg []byte
		if prehash {
			sigRaw[0], sigRaw[1] = 'E', 'D'
			sum := blake2b512(data)
			msg = sum[:]
		} else {
			sigRaw[0], sigRaw[1] = 'E', 'd'
			msg = data
		}
		copy(sigRaw[2:10], keyID[:])
		sig := ed25519.Sign(priv, msg)
		copy(sigRaw[10:74], sig)
		globalMsg := append(append([]byte(nil), sig...), []byte(trustedComment)...)
		globalSig := ed25519.Sign(priv, globalMsg)
		return "untrusted comment: signature from test key\n" +
			base64.StdEncoding.EncodeToString(sigRaw) + "\n" +
			"trusted comment: " + trustedComment + "\n" +
			base64.StdEncoding.EncodeToString(globalSig) + "\n"
	}
	return pubB64, sign
}

func TestVerifyMinisign(t *testing.T) {
	pubB64, sign := makeMinisignPair(t)
	otherPub, _ := makeMinisignPair(t)
	payload := []byte("horizon-mod-binary-payload-12345")

	sigED := sign(payload, true, "timestamp:1700000000\tfile:test.hmod")
	if err := verifyMinisign(pubB64, sigED, payload); err != nil {
		t.Fatalf("valid prehashed ED signature failed: %v", err)
	}
	sigEd := sign(payload, false, "timestamp:1700000000\tfile:test.hmod")
	if err := verifyMinisign(pubB64, sigEd, payload); err != nil {
		t.Fatalf("valid legacy Ed signature failed: %v", err)
	}
	if err := verifyMinisign(pubB64, sigED, []byte("tampered")); err == nil {
		t.Fatal("expected tampered payload to fail minisign verification")
	}
	tamperedComment := strings.Replace(sigED, "file:test.hmod", "file:evil.hmod", 1)
	if err := verifyMinisign(pubB64, tamperedComment, payload); err == nil {
		t.Fatal("expected tampered trusted comment to fail minisign verification")
	}
	if err := verifyMinisign(otherPub, sigED, payload); err == nil {
		t.Fatal("expected wrong public key to fail minisign verification")
	}
}

func TestHorizonAssetURLAllowlist(t *testing.T) {
	svc, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.verifyHorizonAssetURL("https://github.com/alice/my-mod", "https://github.com/horizon-rust/horizon-mods/releases/download/v1/a.hmod"); err != nil {
		t.Fatalf("official registry release URL rejected: %v", err)
	}
	if err := svc.verifyHorizonAssetURL("https://github.com/alice/my-mod", "https://github.com/alice/my-mod/releases/download/v1.0.0/a.hmod"); err != nil {
		t.Fatalf("project source GitHub release URL rejected: %v", err)
	}
	for _, bad := range []string{
		"http://github.com/horizon-rust/horizon-mods/releases/download/v1/a.hmod",
		"https://evil.example.com/a.hmod",
		"https://github.com/other/repo/releases/download/v1/a.hmod",
	} {
		if err := svc.verifyHorizonAssetURL("https://github.com/alice/my-mod", bad); err == nil {
			t.Fatalf("expected untrusted URL %q to be rejected", bad)
		}
	}
}

func TestHorizonRegistrySearchCategoriesAndVersions(t *testing.T) {
	idx := horizonModIndex{
		Schema:    1,
		Generated: "2026-11-21T00:00:00Z",
		Projects: []horizonRegistryProject{
			{
				ID:         "flesh_to_leather",
				Kind:       "mod",
				Name:       "Flesh to Leather",
				Summary:    "Smelt rotten flesh into leather",
				Authors:    []string{"Alice"},
				Downloads:  1200,
				Updated:    "2026-11-20T10:00:00Z",
				Categories: []string{"utility", "food"},
				Tags:       []string{"smelting", "leather"},
				Icon:       "https://example.com/flesh.png",
				Versions: []horizonRegistryVersion{
					{
						Version:   "1.1.0",
						Published: "2026-11-20T10:00:00Z",
						Game:      "26.3",
						File:      horizonRegistryFile{Name: "flesh_to_leather-1.1.0.hmod", URL: "https://github.com/horizon-rust/horizon-mods/releases/download/v1/flesh.hmod", Size: 1024, SHA256: strings.Repeat("a", 64)},
					},
					{
						Version:   "1.0.0",
						Published: "2026-11-18T10:00:00Z",
						Game:      "26.3",
						File:      horizonRegistryFile{Name: "flesh_to_leather-1.0.0.hmod", URL: "https://github.com/horizon-rust/horizon-mods/releases/download/v0/flesh.hmod", Size: 1000, SHA256: strings.Repeat("b", 64)},
					},
				},
			},
			{
				ID:         "zoom_helper",
				Kind:       "mod",
				Name:       "Zoom Helper",
				Summary:    "Smooth spyglass zoom",
				Authors:    []string{"Bob"},
				Downloads:  3500,
				Updated:    "2026-11-19T10:00:00Z",
				Categories: []string{"utility"},
				Versions: []horizonRegistryVersion{{
					Version:   "0.5.0",
					Published: "2026-11-19T10:00:00Z",
					Game:      "26.3",
					File:      horizonRegistryFile{Name: "zoom_helper.hmod", URL: "https://github.com/horizon-rust/horizon-mods/releases/download/v1/zoom.hmod", Size: 512, SHA256: strings.Repeat("c", 64)},
				}},
			},
			{
				ID:         "native_only_mod",
				Kind:       "native-mod",
				Name:       "Native Rust Mod",
				Summary:    "Source-only crate",
				Downloads:  99999,
				Categories: []string{"technology"},
				Versions:   []horizonRegistryVersion{{Version: "1.0.0", Game: "26.3", File: horizonRegistryFile{Name: "src.tar.gz", Size: 100, SHA256: strings.Repeat("d", 64)}}},
			},
			{
				ID:         "revoked_mod",
				Kind:       "mod",
				Name:       "Malicious Mod",
				Downloads:  50000,
				Categories: []string{"adventure"},
				Versions:   []horizonRegistryVersion{{Version: "1.0.0", Game: "26.3", File: horizonRegistryFile{Name: "bad.hmod", Size: 100, SHA256: strings.Repeat("e", 64)}}},
			},
			{
				ID:         "aurora_shader",
				Kind:       "shader",
				Name:       "Aurora Shader",
				Summary:    "Atmospheric wgpu shader",
				Authors:    []string{"ShaderDev"},
				Downloads:  800,
				Updated:    "2026-11-21T01:00:00Z",
				Categories: []string{"atmosphere", "shadows"},
				Versions: []horizonRegistryVersion{{
					Version:   "2.0.0",
					Published: "2026-11-21T01:00:00Z",
					Game:      "26.3",
					File:      horizonRegistryFile{Name: "aurora.hshader", URL: "https://github.com/horizon-rust/horizon-mods/releases/download/v2/aurora.hshader", Size: 4096, SHA256: strings.Repeat("f", 64)},
				}},
			},
			{
				ID:         "crisp_pack",
				Kind:       "resourcepack",
				Name:       "Crisp 16x",
				Summary:    "Clean vanilla textures",
				Authors:    []string{"Artist"},
				Downloads:  600,
				Updated:    "2026-11-17T00:00:00Z",
				Categories: []string{"16x", "vanilla-like"},
				Versions: []horizonRegistryVersion{{
					Version:   "1.0.0",
					Published: "2026-11-17T00:00:00Z",
					Game:      "26.3",
					File:      horizonRegistryFile{Name: "crisp.zip", URL: "https://github.com/horizon-rust/horizon-mods/releases/download/v1/crisp.zip", Size: 8192, SHA256: strings.Repeat("1", 64)},
				}},
			},
		},
		Revoked: []horizonRevokedEntry{
			{ID: "revoked_mod", Reason: "policy violation"},
			{ID: "flesh_to_leather", SHA256: strings.Repeat("b", 64), Reason: "broken build"},
		},
	}

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(idx)
	}))
	defer srv.Close()

	svc, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc.http = srv.Client()
	svc.horizonTest = &horizonTestConfig{modIndexURL: srv.URL + "/index.json"}
	ctx := context.Background()

	// Search mods: excludes native-mod and revoked_mod, default sorts by downloads when query is empty.
	res, err := svc.Search(ctx, SearchQuery{Source: "horizon", Type: "mod"})
	if err != nil {
		t.Fatalf("Search(horizon, mod): %v", err)
	}
	if res.Total != 2 || len(res.Projects) != 2 {
		t.Fatalf("expected 2 installable Horizon mods, got total=%d len=%d (%+v)", res.Total, len(res.Projects), res.Projects)
	}
	if res.Projects[0].ID != "zoom_helper" || res.Projects[1].ID != "flesh_to_leather" {
		t.Fatalf("unexpected default mod order: %+v", res.Projects)
	}
	if res.Projects[1].URL != "https://playhorizon.dev/mods/flesh_to_leather" {
		t.Fatalf("unexpected project URL: %q", res.Projects[1].URL)
	}

	// Sort by updated puts flesh_to_leather first.
	resUpd, err := svc.Search(ctx, SearchQuery{Source: "horizon", Type: "mod", Sort: "updated"})
	if err != nil || len(resUpd.Projects) != 2 || resUpd.Projects[0].ID != "flesh_to_leather" {
		t.Fatalf("sort updated failed: err=%v projects=%+v", err, resUpd.Projects)
	}

	// Category filter + query filter.
	resCat, err := svc.Search(ctx, SearchQuery{Source: "horizon", Type: "mod", Categories: []string{"food"}, Query: "leather"})
	if err != nil || resCat.Total != 1 || resCat.Projects[0].ID != "flesh_to_leather" {
		t.Fatalf("category/query search failed: err=%v res=%+v", err, resCat)
	}

	// Modpack returns empty cleanly.
	resPack, err := svc.Search(ctx, SearchQuery{Source: "horizon", Type: "modpack"})
	if err != nil || resPack.Total != 0 || len(resPack.Projects) != 0 {
		t.Fatalf("expected empty modpack search for horizon, got err=%v res=%+v", err, resPack)
	}

	// Categories exclude native-mod ("technology") and revoked ("adventure").
	cats, err := svc.Categories(ctx, "horizon", "mod")
	if err != nil {
		t.Fatal(err)
	}
	var catIDs []string
	for _, c := range cats {
		catIDs = append(catIDs, c.ID)
	}
	if !slices.Equal(catIDs, []string{"food", "utility"}) {
		t.Fatalf("Categories(horizon, mod) = %v, want [food utility]", catIDs)
	}

	// Versions exclude revoked SHA256 (1.0.0) and tag horizon loader for .hmod / .hshader but not .zip resourcepack.
	modVers, err := svc.Versions(ctx, "horizon", "flesh_to_leather", "26.3", "horizon")
	if err != nil || len(modVers) != 1 || modVers[0].ID != "1.1.0" || !slices.Equal(modVers[0].Loaders, []string{"horizon"}) {
		t.Fatalf("Versions(flesh_to_leather) = %+v, err=%v", modVers, err)
	}
	shaderVers, err := svc.Versions(ctx, "horizon", "aurora_shader", "26.3", "")
	if err != nil || len(shaderVers) != 1 || !slices.Equal(shaderVers[0].Loaders, []string{"horizon"}) {
		t.Fatalf("Versions(aurora_shader) = %+v, err=%v", shaderVers, err)
	}
	rpVers, err := svc.Versions(ctx, "horizon", "crisp_pack", "26.3", "")
	if err != nil || len(rpVers) != 1 || len(rpVers[0].Loaders) != 0 {
		t.Fatalf("Versions(crisp_pack) = %+v, err=%v", rpVers, err)
	}
}

func TestHorizonModInstallManageCheckAndOverview(t *testing.T) {
	pubB64, sign := makeMinisignPair(t)
	tmp := t.TempDir()

	// Build a valid .hmod archive (zip with horizon-mod.toml + icon.png) for v1.0.0 and v2.0.0.
	iconPNG := []byte("\x89PNG\r\n\x1a\nfake-icon")
	hmodV1Path := filepath.Join(tmp, "flesh-1.0.0.hmod")
	writeTestZip(t, hmodV1Path, map[string][]byte{
		"horizon-mod.toml": []byte("id = \"flesh_to_leather\"\nname = \"Flesh to Leather\"\nversion = \"1.0.0\"\ndescription = \"Smelt rotten flesh\"\n"),
		"icon.png":         iconPNG,
	})
	hmodV1Bytes, err := os.ReadFile(hmodV1Path)
	if err != nil {
		t.Fatal(err)
	}
	hmodV1Sum := sha256.Sum256(hmodV1Bytes)
	hmodV1SHA := hex.EncodeToString(hmodV1Sum[:])
	hmodV1Sig := sign(hmodV1Bytes, true, "file:flesh-1.0.0.hmod")

	// Build a valid .hshader archive (zip with horizon-shaders.toml).
	hshaderPath := filepath.Join(tmp, "aurora.hshader")
	writeTestZip(t, hshaderPath, map[string][]byte{
		"horizon-shaders.toml": []byte("description = \"Atmospheric Horizon wgpu shader\"\n"),
	})
	hshaderBytes, err := os.ReadFile(hshaderPath)
	if err != nil {
		t.Fatal(err)
	}
	hshaderSum := sha256.Sum256(hshaderBytes)
	hshaderSHA := hex.EncodeToString(hshaderSum[:])
	hshaderSig := sign(hshaderBytes, true, "file:aurora.hshader")

	// Build a tampered-signature mod to verify rejection.
	badSig := sign([]byte("different-bytes"), true, "file:tampered.hmod")

	var srv *httptest.Server
	serveV2 := false
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			fleshVersions := []horizonRegistryVersion{{
				Version:   "1.0.0",
				Published: "2026-11-20T10:00:00Z",
				Game:      "26.3",
				File: horizonRegistryFile{
					Name:      "flesh-1.0.0.hmod",
					URL:       srv.URL + "/files/flesh-1.0.0.hmod",
					Size:      int64(len(hmodV1Bytes)),
					SHA256:    hmodV1SHA,
					Signature: hmodV1Sig,
				},
			}}
			if serveV2 {
				fleshVersions = append([]horizonRegistryVersion{{
					Version:   "2.0.0",
					Published: "2026-11-21T10:00:00Z",
					Game:      "26.3",
					File: horizonRegistryFile{
						Name:      "flesh-2.0.0.hmod",
						URL:       srv.URL + "/files/flesh-1.0.0.hmod",
						Size:      int64(len(hmodV1Bytes)),
						SHA256:    hmodV1SHA,
						Signature: hmodV1Sig,
					},
				}}, fleshVersions...)
			}
			_ = json.NewEncoder(w).Encode(horizonModIndex{
				Schema: 1,
				Projects: []horizonRegistryProject{
					{
						ID:         "flesh_to_leather",
						Kind:       "mod",
						Name:       "Flesh to Leather",
						Summary:    "Smelt rotten flesh into leather",
						Icon:       "https://example.com/flesh-icon.png",
						Key:        pubB64,
						Categories: []string{"utility"},
						Versions:   fleshVersions,
					},
					{
						ID:   "tampered_mod",
						Kind: "mod",
						Name: "Tampered Mod",
						Key:  pubB64,
						Versions: []horizonRegistryVersion{{
							Version: "1.0.0",
							Game:    "26.3",
							File: horizonRegistryFile{
								Name:      "tampered.hmod",
								URL:       srv.URL + "/files/flesh-1.0.0.hmod",
								Size:      int64(len(hmodV1Bytes)),
								SHA256:    hmodV1SHA,
								Signature: badSig,
							},
						}},
					},
					{
						ID:         "aurora_shader",
						Kind:       "shader",
						Name:       "Aurora Shader",
						Summary:    "Atmospheric shader",
						Key:        pubB64,
						Categories: []string{"atmosphere"},
						Versions: []horizonRegistryVersion{{
							Version: "1.0.0",
							Game:    "26.3",
							File: horizonRegistryFile{
								Name:      "aurora.hshader",
								URL:       srv.URL + "/files/aurora.hshader",
								Size:      int64(len(hshaderBytes)),
								SHA256:    hshaderSHA,
								Signature: hshaderSig,
							},
						}},
					},
				},
			})
		case "/files/flesh-1.0.0.hmod":
			_, _ = w.Write(hmodV1Bytes)
		case "/files/aurora.hshader":
			_, _ = w.Write(hshaderBytes)
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
	svc.horizonTest = &horizonTestConfig{modIndexURL: srv.URL + "/index.json"}

	// Create Horizon root.
	hRootDir := filepath.Join(t.TempDir(), ".minecraft")
	hVerID := "horizon-nightly-2026-11-20-0a875da"
	if err := atomicJSON(filepath.Join(hRootDir, "versions", hVerID, hVerID+".json"), gameVersion{
		ID:            hVerID,
		ClientVersion: "26.3",
		Horizon:       &horizonVersionMeta{BuildID: "nightly-2026-11-20-0a875da", Minecraft: "26.3", JavaMajor: 25},
	}); err != nil {
		t.Fatal(err)
	}
	hRoot, err := svc.AddRoot(hRootDir, "Horizon")
	if err != nil {
		t.Fatal(err)
	}

	// Create Fabric root to verify cross-runtime rejections.
	fRootDir := filepath.Join(t.TempDir(), ".minecraft")
	fVerID := "fabric-loader-0.16.0-1.21.1"
	if err := atomicJSON(filepath.Join(fRootDir, "versions", fVerID, fVerID+".json"), gameVersion{
		ID:            fVerID,
		InheritsFrom:  "1.21.1",
		ClientVersion: "1.21.1",
		MainClass:     "net.fabricmc.loader.impl.launch.knot.KnotClient",
	}); err != nil {
		t.Fatal(err)
	}
	fRoot, err := svc.AddRoot(fRootDir, "Fabric")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// ResolveInstall plans install on Horizon, and marks incompatible ("none") on Fabric.
	planH, err := svc.ResolveInstall(ctx, hRoot.ID, "horizon", "flesh_to_leather", "1.0.0", func([]PlanItem) {})
	if err != nil || len(planH) != 1 || planH[0].Status != "install" {
		t.Fatalf("ResolveInstall on Horizon = (%+v, %v), want target install", planH, err)
	}
	planF, err := svc.ResolveInstall(ctx, fRoot.ID, "horizon", "flesh_to_leather", "1.0.0", func([]PlanItem) {})
	if err != nil || len(planF) != 1 || planF[0].Status != "none" {
		t.Fatalf("ResolveInstall on Fabric = (%+v, %v), want target none", planF, err)
	}

	// Installing a Horizon mod onto Fabric must fail.
	if err := svc.installModIn(ctx, func(string, int, int) {}, fRoot, "horizon", "flesh_to_leather", "1.0.0", "", nil); err == nil {
		t.Fatal("expected installing Horizon mod onto Fabric instance to fail")
	}

	// Tampered minisign signature must fail closed.
	if err := svc.installModIn(ctx, func(string, int, int) {}, hRoot, "horizon", "tampered_mod", "1.0.0", "", nil); err == nil {
		t.Fatal("expected tampered minisign mod install to fail")
	}

	// Install valid signed Horizon mod onto Horizon instance.
	jobID, err := svc.InstallMod(hRoot.ID, "horizon", "flesh_to_leather", "1.0.0", "")
	if err != nil {
		t.Fatalf("InstallMod(horizon): %v", err)
	}
	if job := waitForJob(t, svc, jobID, 10*time.Second); job.State != "completed" {
		t.Fatalf("InstallMod job state=%s msg=%q", job.State, job.Message)
	}

	// Verify ListModsAt, AddonIcon, AddonRemoteInfo, InstanceFolder.
	mods, err := svc.ListModsAt(hRoot.ID, hVerID)
	if err != nil || len(mods) != 1 {
		t.Fatalf("ListModsAt = (%+v, %v), want 1 mod", mods, err)
	}
	m := mods[0]
	if m.ModID != "flesh_to_leather" || m.Name != "Flesh to Leather" || !m.Enabled || m.Source != "horizon" || m.VersionID != "1.0.0" || m.SHA1 == "" {
		t.Fatalf("unexpected installed Horizon mod: %+v", m)
	}
	gotIcon, err := svc.AddonIcon(hRoot.ID, hVerID, "mod", m.File)
	if err != nil || !bytes.Equal(gotIcon, iconPNG) {
		t.Fatalf("AddonIcon = (%q, %v), want embedded icon.png", gotIcon, err)
	}
	rem, err := svc.AddonRemoteInfo(ctx, hRoot.ID, hVerID, "mod", []AddonRef{{SHA1: m.SHA1, File: m.File}})
	if err != nil || rem[m.SHA1].Icon != "https://example.com/flesh-icon.png" {
		t.Fatalf("AddonRemoteInfo = (%+v, %v)", rem, err)
	}
	hFolder, err := svc.InstanceFolder(hRoot.ID, hVerID, "horizon-mods")
	if err != nil || !strings.HasSuffix(filepath.ToSlash(hFolder), "/horizon-mods") {
		t.Fatalf("InstanceFolder(horizon-mods) = (%q, %v)", hFolder, err)
	}

	// ToggleMod disables via horizon-mods.txt WITHOUT renaming the .hmod file.
	if err := svc.ToggleMod(hRoot.ID, m.File); err != nil {
		t.Fatalf("ToggleMod disable: %v", err)
	}
	mods, err = svc.ListModsAt(hRoot.ID, hVerID)
	if err != nil || len(mods) != 1 || mods[0].Enabled || mods[0].File != m.File {
		t.Fatalf("after ToggleMod disable: %+v, err=%v", mods, err)
	}
	stOff, err := svc.InstanceStatusContext(ctx, hRoot.ID, hVerID)
	if err != nil || stOff.Mods != 1 || stOff.EnabledMods != 0 || stOff.Sizes["mods"] <= 0 {
		t.Fatalf("InstanceStatus while mod disabled: %+v, err=%v", stOff, err)
	}
	if err := svc.ToggleMod(hRoot.ID, m.File); err != nil {
		t.Fatalf("ToggleMod re-enable: %v", err)
	}
	stOn, err := svc.InstanceOverviewContext(ctx, hRoot.ID, hVerID)
	if err != nil || stOn.Mods != 1 || stOn.EnabledMods != 1 {
		t.Fatalf("InstanceOverview after re-enable: %+v, err=%v", stOn, err)
	}

	// CheckMods detects v2.0.0 update from Horizon registry.
	serveV2 = true
	svc.horizonMu.Lock()
	svc.horizonModCache = nil
	svc.horizonMu.Unlock()
	chkJob, err := svc.CheckMods(hRoot.ID)
	if err != nil {
		t.Fatalf("CheckMods: %v", err)
	}
	if job := waitForJob(t, svc, chkJob, 10*time.Second); job.State != "completed" {
		t.Fatalf("CheckMods job state=%s msg=%q", job.State, job.Message)
	}
	mods, err = svc.ListModsAt(hRoot.ID, hVerID)
	if err != nil || len(mods) != 1 || mods[0].Status != "update" || mods[0].Latest == nil || mods[0].Latest.ID != "2.0.0" {
		t.Fatalf("expected update to 2.0.0 after CheckMods, got %+v, err=%v", mods, err)
	}

	// Install .hshader onto Horizon succeeds; onto Fabric fails.
	shaderJob, err := svc.InstallAssetPack(hRoot.ID, "shader", "horizon", "aurora_shader", "1.0.0")
	if err != nil {
		t.Fatalf("InstallAssetPack(horizon shader): %v", err)
	}
	if job := waitForJob(t, svc, shaderJob, 10*time.Second); job.State != "completed" {
		t.Fatalf("InstallAssetPack job state=%s msg=%q", job.State, job.Message)
	}
	badShaderJob, err := svc.InstallAssetPack(fRoot.ID, "shader", "horizon", "aurora_shader", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if job := waitForJob(t, svc, badShaderJob, 10*time.Second); job.State != "failed" {
		t.Fatalf("expected .hshader install on Fabric to fail, got state=%s", job.State)
	}
	shaders, err := svc.ListAssetPacksAt(hRoot.ID, hVerID, "shader")
	if err != nil || len(shaders) != 1 || shaders[0].Name != "aurora" || shaders[0].Description != "Atmospheric Horizon wgpu shader" {
		t.Fatalf("ListAssetPacksAt(shader) = (%+v, %v)", shaders, err)
	}
	stShader, err := svc.InstanceOverviewContext(ctx, hRoot.ID, hVerID)
	if err != nil || stShader.Shaders != 1 {
		t.Fatalf("expected overview Shaders=1, got %+v, err=%v", stShader, err)
	}

	// AddLocalAssetPacks accepts .hshader on Horizon and rejects it on non-Horizon instances.
	localHshader := filepath.Join(tmp, "local_sky.hshader")
	writeTestZip(t, localHshader, map[string][]byte{
		"horizon-shaders.toml": []byte("description = \"Local Horizon shader\"\n"),
	})
	if err := svc.AddLocalAssetPacks(fRoot.ID, "shader", []string{localHshader}); err == nil {
		t.Fatal("expected AddLocalAssetPacks(.hshader) on Fabric instance to fail")
	}
	if err := svc.AddLocalAssetPacks(hRoot.ID, "shader", []string{localHshader}); err != nil {
		t.Fatalf("AddLocalAssetPacks(.hshader) on Horizon: %v", err)
	}

	// AddLocalMods rejects .jar and accepts .hmod on Horizon.
	fakeJar := filepath.Join(tmp, "bad.jar")
	_ = os.WriteFile(fakeJar, []byte("jar"), 0o644)
	if err := svc.AddLocalMods(hRoot.ID, []string{fakeJar}); err == nil {
		t.Fatal("expected AddLocalMods(.jar) on Horizon to fail")
	}
	localHmod := filepath.Join(tmp, "local_extra.hmod")
	writeTestZip(t, localHmod, map[string][]byte{
		"horizon-mod.toml": []byte("id = \"local_extra\"\nname = \"Local Extra\"\nversion = \"0.1.0\"\n"),
	})
	if err := svc.AddLocalMods(hRoot.ID, []string{localHmod}); err != nil {
		t.Fatalf("AddLocalMods(.hmod): %v", err)
	}

	// RemoveMod moves .hmod to backups and removes state line from horizon-mods.txt.
	if err := svc.RemoveMod(hRoot.ID, "local_extra.hmod"); err != nil {
		t.Fatalf("RemoveMod(local_extra.hmod): %v", err)
	}
	mods, err = svc.ListModsAt(hRoot.ID, hVerID)
	if err != nil || len(mods) != 1 || mods[0].File != m.File {
		t.Fatalf("expected only original mod after RemoveMod, got %+v, err=%v", mods, err)
	}
}
