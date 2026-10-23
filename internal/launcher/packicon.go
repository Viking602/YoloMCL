package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// savePackIcon stores the modpack project's logo as <root>/.mcl/icon.png so
// the list and hero show the pack's own icon. Best effort: failures are
// ignored and the loader logo is used instead.
func (s *Service) savePackIcon(ctx context.Context, source, projectID, root string) {
	if projectID == "" || root == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var logo string
	switch source {
	case "curseforge":
		var r struct {
			Data struct {
				Logo struct {
					URL          string `json:"url"`
					ThumbnailURL string `json:"thumbnailUrl"`
				} `json:"logo"`
			} `json:"data"`
		}
		if s.cfJSON(ctx, "GET", "/mods/"+url.PathEscape(projectID), nil, &r) != nil {
			return
		}
		logo = r.Data.Logo.ThumbnailURL
		if logo == "" {
			logo = r.Data.Logo.URL
		}
	case "modrinth":
		var r struct {
			Icon string `json:"icon_url"`
		}
		if s.getJSON(ctx, modrinthAPI+"/project/"+url.PathEscape(projectID), &r) != nil {
			return
		}
		logo = r.Icon
	}
	if logo == "" {
		return
	}
	res, e := s.request(ctx, "GET", logo, nil, nil)
	if e != nil {
		return
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if e != nil || !isImageBytes(b) {
		return
	}
	dir := filepath.Join(root, ".mcl")
	if os.MkdirAll(dir, 0755) != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, "icon.png"), b, 0644)
}

func isImageBytes(b []byte) bool {
	return bytes.HasPrefix(b, []byte("\x89PNG")) || bytes.HasPrefix(b, []byte("\xff\xd8\xff")) || bytes.HasPrefix(b, []byte("GIF8"))
}

// EnsurePackIcon backfills the icon for a directory imported before icons
// were saved. It is a no-op when the icon exists or the pack has no project.
func (s *Service) EnsurePackIcon(rootID string) {
	if _, done := packIconTried.LoadOrStore(rootID, true); done {
		return
	}
	r, e := s.getRoot(rootID)
	if e != nil {
		return
	}
	if _, e := os.Stat(filepath.Join(r.Path, ".mcl", "icon.png")); e == nil {
		return
	}
	var lock PackLock
	b, e := os.ReadFile(filepath.Join(r.Path, ".mcl", "pack-lock.json"))
	if e != nil || json.Unmarshal(b, &lock) != nil {
		return
	}
	s.savePackIcon(context.Background(), lock.Source, lock.ProjectID, r.Path)
}

var packIconTried sync.Map
