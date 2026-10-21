package launcher

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOfficialInstallerHandlesEveryCatalogueCategory(t *testing.T) {
	for _, category := range []string{"release", "snapshot", "old_beta", "old_alpha", "future_category"} {
		t.Run(category, func(t *testing.T) {
			s := testService(t)
			r := newTestRoot(t, s, category)
			id := "official-" + category
			client := zipBytes(t, map[string][]byte{"client-fixture.txt": []byte(category)})
			clientHash, _ := hashes(client)
			asset := []byte("historical sound resource")
			assetHash, _ := hashes(asset)
			index := jsonBytes(t, map[string]any{"virtual": category == "old_beta", "map_to_resources": category == "old_alpha", "objects": map[string]any{"sound/example.ogg": map[string]any{"hash": assetHash, "size": len(asset)}}})
			indexHash, _ := hashes(index)
			v := gameVersion{ID: id, Type: category, MainClass: "fixture.Client", Downloads: map[string]artifact{"client": {URL: "https://fixture.test/client", SHA1: clientHash, Size: int64(len(client))}}}
			v.MinecraftArguments = "${auth_player_name} ${auth_session} --assetsDir ${game_assets}"
			v.AssetIndex.ID = "pre-1.6"
			v.AssetIndex.URL, v.AssetIndex.SHA1, v.AssetIndex.Size = "https://fixture.test/asset-index", indexHash, int64(len(index))
			metadata := jsonBytes(t, v)
			metadataHash, _ := hashes(metadata)
			manifest := jsonBytes(t, map[string]any{"versions": []MinecraftVersion{{ID: id, Type: category, URL: "https://fixture.test/version", SHA1: metadataHash}}})
			fakeHTTP(t, s, map[string][]byte{manifestURL: manifest, "https://fixture.test/version": metadata, "https://fixture.test/client": client, "https://fixture.test/asset-index": index, "https://resources.download.minecraft.net/" + assetHash[:2] + "/" + assetHash: asset})
			jobID, e := s.InstallGame(r.ID, "", "", id, "vanilla", "")
			if e != nil {
				t.Fatal(e)
			}
			if job := waitJob(t, s, jobID); job.State != "completed" {
				t.Fatal(job.Message)
			}
			if !checkHashes(filepath.Join(r.Path, "versions", id, id+".jar"), clientHash, "") {
				t.Fatal("official client was not installed with checksum validation")
			}
			root, _ := s.getRoot(r.ID)
			if root.SelectedVersion != id {
				t.Fatal("installed official client was not selected")
			}
			if category == "old_beta" && !checkHashes(filepath.Join(r.Path, "assets", "virtual", "pre-1.6", "sound", "example.ogg"), assetHash, "") {
				t.Fatal("historical virtual assets were not materialized")
			}
			if category == "old_alpha" && !checkHashes(filepath.Join(r.Path, "resources", "sound", "example.ogg"), assetHash, "") {
				t.Fatal("historical resources directory was not materialized")
			}
		})
	}
}

// This opt-in check downloads only Mojang metadata and one tiny historical
// client JAR. It never starts Minecraft or changes the player's game folders.
func TestLiveMojangOfficialCatalogue(t *testing.T) {
	if os.Getenv("MCL_MOJANG_LIVE") != "1" {
		t.Skip("set MCL_MOJANG_LIVE=1 for official-network verification")
	}
	s := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	manifest, e := s.Manifest(ctx)
	if e != nil {
		t.Fatal(e)
	}
	counts := map[string]int{}
	for _, v := range manifest.Versions {
		counts[v.Type]++
	}
	t.Logf("official catalogue: %d versions, categories %v", len(manifest.Versions), counts)
	ids := []string{manifest.Latest.Release, manifest.Latest.Snapshot, "1.6.4", "b1.7.3", "a1.2.6", "rd-132211"}
	for _, id := range ids {
		found := false
		for _, entry := range manifest.Versions {
			if entry.ID != id {
				continue
			}
			found = true
			path := filepath.Join(s.dir, "metadata", id+".json")
			if e = s.download(ctx, DownloadFile{Name: id, URL: entry.URL, SHA1: entry.SHA1}, path); e != nil {
				t.Fatal(e)
			}
			b, _ := os.ReadFile(path)
			var v gameVersion
			if e = json.Unmarshal(b, &v); e != nil {
				t.Fatal(e)
			}
			client := v.Downloads["client"]
			if client.URL == "" || client.SHA1 == "" || client.Size <= 0 || v.ID != id {
				t.Fatalf("official client metadata missing: %s", id)
			}
			t.Logf("%s / %s: client %d bytes, %d runtime libraries, Java %d", id, entry.Type, client.Size, len(v.Libraries), v.JavaVersion.MajorVersion)
			if id == "rd-132211" {
				jar := filepath.Join(s.dir, "client-smoke.jar")
				if e = s.download(ctx, DownloadFile{Name: id, URL: client.URL, SHA1: client.SHA1, Size: client.Size}, jar); e != nil {
					t.Fatal(e)
				}
				if !checkHashes(jar, client.SHA1, "") {
					t.Fatal("real historical client checksum mismatch")
				}
				t.Log("real historical client downloaded and SHA-1 verified")
			}
			break
		}
		if !found {
			t.Fatalf("official catalogue is missing verification sample %s", id)
		}
	}
}
