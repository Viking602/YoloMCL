package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	publisherconfig "github.com/Viking602/YoloMCL/internal/config"
)

func TestCurseForgeSearchUsesPublisherKeyAndConsistentPageSize(t *testing.T) {
	s := testService(t)
	s.publisher = publisherconfig.Config{CurseForgeAPIKey: "test-publisher-key"}
	s.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		q := r.URL.Query()
		if r.URL.Host != "api.curseforge.com" || r.URL.Path != "/v1/mods/search" || r.Header.Get("x-api-key") != "test-publisher-key" {
			t.Fatal("incorrect CurseForge publisher access")
		}
		for k, v := range map[string]string{"gameId": "432", "classId": "4471", "pageSize": "50", "index": "18", "sortField": "6", "modLoaderType": "4", "gameVersion": "1.21.1", "searchFilter": "pack"} {
			if q.Get(k) != v {
				t.Errorf("query %s: got %q, want %q", k, q.Get(k), v)
			}
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":123,"name":"Test pack","logo":{"url":"https://cdn.example.test/icon.webp"}}],"pagination":{"totalCount":200}}`))}, nil
	})}
	r, e := s.Search(context.Background(), SearchQuery{Source: "curseforge", Type: "modpack", Query: "pack", Minecraft: "1.21.1", Loader: "fabric", Offset: 18, Limit: 100, Sort: "downloads"})
	if e != nil {
		t.Fatal(e)
	}
	if r.Limit != 50 || r.Offset != 18 || r.Total != 200 || len(r.Projects) != 1 || r.Projects[0].Icon == "" {
		t.Fatal("CurseForge pagination or logo lost")
	}
}

func TestPlayerSettingsCannotOverridePublisherCredentials(t *testing.T) {
	t.Setenv("MCL_DEVELOPMENT", "")
	s := testService(t)
	s.publisher = publisherconfig.Config{MicrosoftClientID: "12345678-1234-1234-1234-123456789abc", CurseForgeAPIKey: "private-build-key"}
	_, e := s.Dispatch(context.Background(), "settings", `{"settings":{"memoryMB":4096,"microsoftClientId":"player-input","curseForgeApiKey":"player-input"},"curseForgeApiKey":"player-input"}`)
	if e != nil {
		t.Fatal(e)
	}
	if s.publisher.CurseForgeAPIKey != "private-build-key" || s.publisher.MicrosoftClientID == "player-input" {
		t.Fatal("player settings changed publisher access")
	}
	raw, e := s.Dispatch(context.Background(), "snapshot", `{}`)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(raw, "private-build-key") || strings.Contains(raw, "curseForgeApiKey") || strings.Contains(raw, "microsoftClientId") {
		t.Fatal("publisher credentials exposed through UI snapshot")
	}
}

func TestVersionDownloadsSurviveProviderAndUIJSON(t *testing.T) {
	for _, source := range []string{"modrinth", "curseforge"} {
		t.Run(source, func(t *testing.T) {
			s := testService(t)
			s.publisher = publisherconfig.Config{CurseForgeAPIKey: "test-publisher-key"}
			var entries []string
			// Zero is a reported count, while omitted/null mean unavailable.
			// A count above uint32 also guards against truncating popular files.
			for i, value := range []string{"0", "12345", "4294967297", "", "null"} {
				field, base := "downloads", fmt.Sprintf(`{"id":"%d","project_id":"123","name":"Version %d"`, i, i)
				if source == "curseforge" {
					field, base = "downloadCount", fmt.Sprintf(`{"id":%d,"modId":123,"displayName":"Version %d"`, i, i)
				}
				if value != "" {
					base += fmt.Sprintf(`,"%s":%s`, field, value)
				}
				entries = append(entries, base+"}")
			}
			requests := 0
			s.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				requests++
				body := "[" + strings.Join(entries, ",") + "]"
				switch r.URL.Path {
				case "/v2/project/123/version", "/v1/mods/123/files":
				case "/v2/version/2", "/v1/mods/123/files/2":
					body = entries[2]
				default:
					t.Fatalf("unexpected metadata request: %s", r.URL.Path)
				}
				if source == "curseforge" {
					body = `{"data":` + body + "}"
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			raw, err := s.Dispatch(context.Background(), "versions", string(jsonBytes(t, map[string]string{"source": source, "projectId": "123"})))
			if err != nil {
				t.Fatal(err)
			}
			var versions []ProjectVersion
			if err := json.Unmarshal([]byte(raw), &versions); err != nil || len(versions) != 5 {
				t.Fatalf("invalid version response: %s (%v)", raw, err)
			}
			for i, want := range []int64{0, 12345, 4294967297} {
				if got := versions[i].Downloads; got == nil || *got != want {
					t.Fatalf("version %d lost its own count: %v, want %d", i, got, want)
				}
			}
			if versions[3].Downloads != nil || versions[4].Downloads != nil {
				t.Fatal("missing counts became zero")
			}
			if requests != 1 {
				t.Fatal("download counts should arrive in the existing version list request")
			}
			version, err := s.projectVersion(context.Background(), source, "123", "2")
			if err != nil || version.Downloads == nil || *version.Downloads != 4294967297 {
				t.Fatalf("single version lost its download count: %+v (%v)", version, err)
			}
		})
	}
}
