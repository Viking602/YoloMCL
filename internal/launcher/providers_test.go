package launcher

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/distribution"
)

func TestCurseForgeSearchUsesPublisherKeyAndConsistentPageSize(t *testing.T) {
	s := testService(t)
	s.distribution = distribution.Config{CurseForgeAPIKey: "test-publisher-key"}
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
	s.distribution = distribution.Config{MicrosoftClientID: "12345678-1234-1234-1234-123456789abc", CurseForgeAPIKey: "private-build-key"}
	_, e := s.Dispatch(context.Background(), "settings", `{"settings":{"memoryMB":4096,"microsoftClientId":"player-input","curseForgeApiKey":"player-input"},"curseForgeApiKey":"player-input"}`)
	if e != nil {
		t.Fatal(e)
	}
	if s.distribution.CurseForgeAPIKey != "private-build-key" || s.distribution.MicrosoftClientID == "player-input" {
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
