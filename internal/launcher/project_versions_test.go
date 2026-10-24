package launcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	publisherconfig "github.com/Viking602/YoloMCL/internal/config"
)

func TestCurseForgeVersionsFollowPaginationAndKeepAllLoaders(t *testing.T) {
	s := testService(t)
	s.publisher = publisherconfig.Config{CurseForgeAPIKey: "fixture"}
	var offsets []int
	s.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		index, _ := strconv.Atoi(r.URL.Query().Get("index"))
		offsets = append(offsets, index)
		if r.URL.Query().Get("gameVersion") != "" || r.URL.Query().Get("modLoaderType") != "" {
			t.Error("unfiltered catalog was narrowed")
		}
		start, count := index, 50
		if index == 50 {
			start = 49
		}
		if index == 100 {
			start, count = 99, 21
		}
		var files []map[string]any
		for i := range count {
			n := start + i
			loader := "NeoForge"
			if n >= 50 {
				loader = "Fabric"
			}
			files = append(files, map[string]any{"id": n + 1, "modId": 123, "displayName": fmt.Sprint(n), "fileDate": fmt.Sprintf("2026-10-%02dT12:00:00Z", 28-n%28), "gameVersions": []string{"1.21.1", loader, "Client", "Java 21"}, "releaseType": n%3 + 1})
		}
		body := jsonBytes(t, map[string]any{"data": files, "pagination": map[string]int{"index": index, "pageSize": 50, "resultCount": count, "totalCount": 121}})
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}
	vs, err := s.Versions(context.Background(), "curseforge", "123", "", "")
	if err != nil || len(vs) != 120 || !slices.Equal(offsets, []int{0, 50, 100}) {
		t.Fatalf("incomplete or duplicated catalog: count=%d offsets=%v err=%v", len(vs), offsets, err)
	}
	loaders, types := map[string]bool{}, map[string]bool{}
	for i, v := range vs {
		if !slices.Equal(v.GameVersions, []string{"1.21.1"}) {
			t.Fatal("non-Minecraft metadata became a game version", v.GameVersions)
		}
		loaders[v.Loaders[0]], types[v.Type] = true, true
		if i > 0 && v.Published > vs[i-1].Published {
			t.Fatal("catalog not newest first")
		}
	}
	if len(loaders) != 2 || len(types) != 3 {
		t.Fatal("catalog lost loader/release channels")
	}
}

func TestCurseForgeVersionPageFailureDoesNotExposePartialCatalog(t *testing.T) {
	s := testService(t)
	s.publisher = publisherconfig.Config{CurseForgeAPIKey: "fixture"}
	s.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("index") != "0" {
			return nil, errors.New("second page unavailable")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":1}],"pagination":{"totalCount":2}}`))}, nil
	})}
	vs, err := s.Versions(context.Background(), "curseforge", "123", "", "")
	if err == nil || len(vs) != 0 {
		t.Fatal("partial response presented as full catalog", vs, err)
	}
}

func TestCurseForgeVersionTagsKeepShaderLoadersAndSnapshots(t *testing.T) {
	v := (cfFile{GameVersions: []string{"26.4-snapshot-5", "24w14a", "b1.7.3", "Iris", "OptiFine", "Java 8", "Client", "Server"}}).version()
	if !slices.Equal(v.Loaders, []string{"iris", "optifine"}) || !slices.Equal(v.GameVersions, []string{"26.4-snapshot-5", "24w14a", "b1.7.3"}) {
		t.Fatal("invalid provider tags", v)
	}
}

func TestResolveIncompatibleVersionPreservesInstalledVersion(t *testing.T) {
	s := testService(t)
	s.publisher = publisherconfig.Config{}
	r := newTestRoot(t, s, "Plan")
	installFabricFixture(t, s, r)
	r, _ = s.getRoot(r.ID)
	jar := fabricJar(t, "example", "0.9.0")
	sha, _ := hashes(jar)
	writeTestFile(t, filepath.Join(r.VersionDir(), "mods", "old.jar"), jar)
	if err := atomicJSON(filepath.Join(r.VersionDir(), ".mcl", "mods.json"), map[string]modRecord{"old.jar": {SHA1: sha, Source: "modrinth", ProjectID: "example", VersionID: "old"}}); err != nil {
		t.Fatal(err)
	}
	s.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		body := ""
		switch {
		case strings.HasSuffix(r.URL.Path, ".csv"):
			body = "id,curseforge,modrinth,name,english,abbr,modid\n"
		case r.URL.Path == "/v2/version_files":
			body = `{}`
		case r.URL.Path == "/v2/projects":
			body = `[{"id":"example","slug":"example","title":"Example"}]`
		case r.URL.Path == "/v2/version/new":
			body = `{"id":"new","project_id":"example","version_number":"9.0","game_versions":["26.3"],"loaders":["neoforge"],"dependencies":[{"project_id":"dep","dependency_type":"required"}]}`
		default:
			return nil, fmt.Errorf("unexpected dependency request for incompatible target: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	items, err := s.ResolveInstall(context.Background(), r.ID, "modrinth", "example", "new", func([]PlanItem) {})
	if err != nil || len(items) != 1 || items[0].Status != "none" || items[0].Note != "0.9.0" || items[0].File != "old.jar" {
		t.Fatalf("installed version corrupted by compatibility text: %+v %v", items, err)
	}
}
