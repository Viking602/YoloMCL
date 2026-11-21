package launcher

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	horizonModIndexURL      = "https://horizon-rust.github.io/horizon-mods/index.json"
	horizonModIndexCacheTTL = 5 * time.Minute
	horizonModIndexMaxBytes = 16 << 20
	horizonModFileMaxBytes  = 64 << 20
	horizonModSiteBaseURL   = "https://playhorizon.dev/mods/"
)

type horizonModIndex struct {
	Schema    int                      `json:"schema"`
	Generated string                   `json:"generated"`
	Projects  []horizonRegistryProject `json:"projects"`
	Revoked   []horizonRevokedEntry    `json:"revoked"`
}

type horizonRegistryProject struct {
	ID          string                   `json:"id"`
	Kind        string                   `json:"kind"`
	Name        string                   `json:"name"`
	Summary     string                   `json:"summary"`
	Description string                   `json:"description"`
	Authors     []string                 `json:"authors"`
	Owners      []string                 `json:"owners"`
	License     string                   `json:"license"`
	Source      string                   `json:"source"`
	Icon        string                   `json:"icon"`
	Key         string                   `json:"key"`
	Downloads   int64                    `json:"downloads"`
	Stars       int64                    `json:"stars"`
	Updated     string                   `json:"updated"`
	Categories  []string                 `json:"categories"`
	Tags        []string                 `json:"tags"`
	Screenshots []string                 `json:"screenshots"`
	Versions    []horizonRegistryVersion `json:"versions"`
}

type horizonRegistryVersion struct {
	Version      string              `json:"version"`
	Published    string              `json:"published"`
	Changelog    string              `json:"changelog"`
	API          string              `json:"api"`
	Side         string              `json:"side"`
	Capabilities []string            `json:"capabilities"`
	Game         string              `json:"game"`
	File         horizonRegistryFile `json:"file"`
}

type horizonRegistryFile struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
}

type horizonRevokedEntry struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
	Reason string `json:"reason"`
	Date   string `json:"date"`
}

type horizonModIndexCache struct {
	at    time.Time
	index *horizonModIndex
}

func (s *Service) horizonModIndexEndpoint() string {
	if s.horizonTest != nil && s.horizonTest.modIndexURL != "" {
		return s.horizonTest.modIndexURL
	}
	return horizonModIndexURL
}

func (s *Service) horizonModIndex(ctx context.Context, force bool) (*horizonModIndex, error) {
	if !force {
		s.horizonMu.Lock()
		if s.horizonModCache != nil && time.Since(s.horizonModCache.at) < horizonModIndexCacheTTL {
			idx := s.horizonModCache.index
			s.horizonMu.Unlock()
			return idx, nil
		}
		s.horizonMu.Unlock()
	}
	endpoint := s.horizonModIndexEndpoint()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "YoloMCL/"+appVersion)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, horizonErr("获取 Horizon 模组目录失败: "+err.Error(), "Failed to fetch Horizon mod index: "+err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, horizonErr(
			fmt.Sprintf("获取 Horizon 模组目录失败（HTTP %d）", resp.StatusCode),
			fmt.Sprintf("Failed to fetch Horizon mod index (HTTP %d)", resp.StatusCode),
		)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, horizonModIndexMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > horizonModIndexMaxBytes {
		return nil, horizonErr("Horizon 模组索引体积超出上限", "Horizon mod index exceeds size limit")
	}
	var idx horizonModIndex
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, horizonErr("解析 Horizon 模组目录失败: "+err.Error(), "Failed to parse Horizon mod index: "+err.Error())
	}
	if idx.Schema != 1 {
		return nil, horizonErr(
			fmt.Sprintf("不支持的 Horizon 模组索引架构版本 %d，请更新启动器", idx.Schema),
			fmt.Sprintf("Unsupported Horizon mod index schema %d; please update the launcher", idx.Schema),
		)
	}
	s.horizonMu.Lock()
	s.horizonModCache = &horizonModIndexCache{at: time.Now(), index: &idx}
	s.horizonMu.Unlock()
	return &idx, nil
}

func (idx *horizonModIndex) revokedReason(projectID, sha256Hex string) (string, bool) {
	sha := strings.ToLower(strings.TrimSpace(sha256Hex))
	for _, r := range idx.Revoked {
		if r.SHA256 != "" && sha != "" && strings.EqualFold(strings.TrimSpace(r.SHA256), sha) {
			return r.Reason, true
		}
		if r.SHA256 == "" && r.ID == projectID {
			return r.Reason, true
		}
	}
	return "", false
}

// horizonDiscoverType maps a Horizon registry kind to YoloMCL's Discover type.
// native-mod requires rebuilding Horizon from source with a Rust toolchain and
// is excluded from direct launcher installation per Horizon Content API.
func horizonDiscoverType(kind string) string {
	switch kind {
	case "mod":
		return "mod"
	case "shader", "shaderpack":
		return "shader"
	case "resourcepack":
		return "resourcepack"
	default:
		return ""
	}
}

func horizonProjectUpdated(p horizonRegistryProject) string {
	if p.Updated != "" {
		return p.Updated
	}
	best := ""
	for _, v := range p.Versions {
		if v.Published > best {
			best = v.Published
		}
	}
	return best
}

func (idx *horizonModIndex) activeVersions(p horizonRegistryProject) []horizonRegistryVersion {
	if _, revoked := idx.revokedReason(p.ID, ""); revoked {
		return nil
	}
	out := make([]horizonRegistryVersion, 0, len(p.Versions))
	for _, v := range p.Versions {
		if _, revoked := idx.revokedReason(p.ID, v.File.SHA256); revoked {
			continue
		}
		out = append(out, v)
	}
	return out
}

func horizonToProject(p horizonRegistryProject) Project {
	urlStr := horizonModSiteBaseURL + url.PathEscape(p.ID)
	if p.ID == "" {
		urlStr = p.Source
	}
	return Project{
		ID:          p.ID,
		Source:      "horizon",
		Type:        horizonDiscoverType(p.Kind),
		Title:       p.Name,
		Description: p.Summary,
		Icon:        p.Icon,
		Downloads:   p.Downloads,
		Author:      strings.Join(p.Authors, ", "),
		Categories:  append([]string(nil), p.Categories...),
		URL:         urlStr,
		Updated:     horizonProjectUpdated(p),
	}
}

func horizonToProjectVersion(p horizonRegistryProject, v horizonRegistryVersion) ProjectVersion {
	games := []string{}
	if strings.TrimSpace(v.Game) != "" {
		games = []string{strings.TrimSpace(v.Game)}
	}
	loaders := []string{}
	if p.Kind == "mod" || strings.HasSuffix(strings.ToLower(v.File.Name), ".hshader") {
		loaders = []string{"horizon"}
	}
	name := v.Version
	if p.Name != "" && !strings.Contains(strings.ToLower(v.Version), strings.ToLower(p.Name)) {
		name = p.Name + " " + v.Version
	}
	return ProjectVersion{
		ID:           v.Version,
		ProjectID:    p.ID,
		Source:       "horizon",
		Name:         name,
		Number:       v.Version,
		Published:    v.Published,
		GameVersions: games,
		Loaders:      loaders,
		Type:         "release",
		Changelog:    v.Changelog,
		Files: []DownloadFile{{
			Name:    v.File.Name,
			URL:     v.File.URL,
			SHA256:  strings.ToLower(strings.TrimSpace(v.File.SHA256)),
			Size:    v.File.Size,
			Primary: true,
		}},
		Dependencies: []Dependency{},
	}
}

func horizonMatchScore(p horizonRegistryProject, query string) (int, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return 1, true
	}
	name := strings.ToLower(p.Name)
	id := strings.ToLower(p.ID)
	summary := strings.ToLower(p.Summary)
	switch {
	case name == q || id == q:
		return 100, true
	case strings.HasPrefix(name, q) || strings.HasPrefix(id, q):
		return 80, true
	case strings.Contains(name, q) || strings.Contains(id, q):
		return 60, true
	}
	for _, tag := range p.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return 45, true
		}
	}
	for _, cat := range p.Categories {
		if strings.Contains(strings.ToLower(cat), q) {
			return 40, true
		}
	}
	for _, author := range p.Authors {
		if strings.Contains(strings.ToLower(author), q) {
			return 35, true
		}
	}
	if strings.Contains(summary, q) {
		return 20, true
	}
	return 0, false
}

func (s *Service) searchHorizon(ctx context.Context, q SearchQuery) (SearchResult, error) {
	out := SearchResult{Projects: []Project{}, Offset: q.Offset, Limit: q.Limit}
	if q.Type == "modpack" {
		return out, nil
	}
	idx, err := s.horizonModIndex(ctx, false)
	if err != nil {
		return out, err
	}
	type scored struct {
		proj    Project
		score   int
		updated string
	}
	var matched []scored
	for _, p := range idx.Projects {
		if horizonDiscoverType(p.Kind) != q.Type {
			continue
		}
		versions := idx.activeVersions(p)
		if len(versions) == 0 {
			continue
		}
		if q.Minecraft != "" {
			hasMC := false
			for _, v := range versions {
				if v.Game == q.Minecraft {
					hasMC = true
					break
				}
			}
			if !hasMC {
				continue
			}
		}
		if len(q.Categories) > 0 {
			catsOK := true
			for _, want := range q.Categories {
				if want == "" {
					continue
				}
				found := false
				for _, have := range p.Categories {
					if strings.EqualFold(have, want) {
						found = true
						break
					}
				}
				if !found {
					catsOK = false
					break
				}
			}
			if !catsOK {
				continue
			}
		}
		score, ok := horizonMatchScore(p, q.Query)
		if !ok {
			continue
		}
		proj := horizonToProject(p)
		matched = append(matched, scored{proj: proj, score: score, updated: proj.Updated})
	}
	sort.SliceStable(matched, func(i, j int) bool {
		a, b := matched[i], matched[j]
		switch q.Sort {
		case "updated":
			if a.updated != b.updated {
				return a.updated > b.updated
			}
			if a.proj.Downloads != b.proj.Downloads {
				return a.proj.Downloads > b.proj.Downloads
			}
		case "downloads":
			if a.proj.Downloads != b.proj.Downloads {
				return a.proj.Downloads > b.proj.Downloads
			}
			if a.updated != b.updated {
				return a.updated > b.updated
			}
		default:
			if a.score != b.score {
				return a.score > b.score
			}
			if a.proj.Downloads != b.proj.Downloads {
				return a.proj.Downloads > b.proj.Downloads
			}
			if a.updated != b.updated {
				return a.updated > b.updated
			}
		}
		return strings.ToLower(a.proj.Title) < strings.ToLower(b.proj.Title)
	})
	out.Total = len(matched)
	if q.Offset >= len(matched) {
		return out, nil
	}
	end := min(len(matched), q.Offset+q.Limit)
	for _, m := range matched[q.Offset:end] {
		out.Projects = append(out.Projects, m.proj)
	}
	return out, nil
}

func (s *Service) horizonCategories(ctx context.Context, kind string) ([]Category, error) {
	if kind == "modpack" {
		return []Category{}, nil
	}
	idx, err := s.horizonModIndex(ctx, false)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []Category{}
	for _, p := range idx.Projects {
		if horizonDiscoverType(p.Kind) != kind || len(idx.activeVersions(p)) == 0 {
			continue
		}
		for _, c := range p.Categories {
			slug := strings.ToLower(strings.TrimSpace(c))
			if slug == "" || seen[slug] {
				continue
			}
			seen[slug] = true
			out = append(out, Category{ID: slug, Slug: slug, Name: slug})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Service) horizonLookup(ctx context.Context, projectID, versionID string) (*horizonModIndex, horizonRegistryProject, horizonRegistryVersion, error) {
	idx, err := s.horizonModIndex(ctx, false)
	if err != nil {
		return nil, horizonRegistryProject{}, horizonRegistryVersion{}, err
	}
	for _, p := range idx.Projects {
		if p.ID != projectID {
			continue
		}
		if reason, revoked := idx.revokedReason(p.ID, ""); revoked {
			return nil, horizonRegistryProject{}, horizonRegistryVersion{}, horizonErr(
				"该项目已被 Horizon 官方下架: "+reason,
				"This project has been revoked by Horizon: "+reason,
			)
		}
		if versionID == "" {
			if vs := idx.activeVersions(p); len(vs) > 0 {
				return idx, p, vs[0], nil
			}
			return nil, horizonRegistryProject{}, horizonRegistryVersion{}, horizonErr("该项目暂无可用版本", "No available version for this project")
		}
		for _, v := range p.Versions {
			if v.Version != versionID {
				continue
			}
			if reason, revoked := idx.revokedReason(p.ID, v.File.SHA256); revoked {
				return nil, horizonRegistryProject{}, horizonRegistryVersion{}, horizonErr(
					"该版本已被 Horizon 官方撤回: "+reason,
					"This version has been revoked by Horizon: "+reason,
				)
			}
			return idx, p, v, nil
		}
		return nil, horizonRegistryProject{}, horizonRegistryVersion{}, horizonErr("未找到指定的 Horizon 资源版本", "Horizon project version not found")
	}
	return nil, horizonRegistryProject{}, horizonRegistryVersion{}, horizonErr("未在 Horizon 目录中找到该项目", "Project not found in Horizon registry")
}

func (s *Service) horizonVersions(ctx context.Context, projectID, mc, loader string) ([]ProjectVersion, error) {
	idx, err := s.horizonModIndex(ctx, false)
	if err != nil {
		return nil, err
	}
	for _, p := range idx.Projects {
		if p.ID != projectID {
			continue
		}
		out := []ProjectVersion{}
		for _, v := range idx.activeVersions(p) {
			pv := horizonToProjectVersion(p, v)
			if mc != "" && !slices.Contains(pv.GameVersions, mc) {
				continue
			}
			if loader != "" && loader != "vanilla" && len(pv.Loaders) > 0 && !slices.Contains(pv.Loaders, strings.ToLower(loader)) {
				continue
			}
			out = append(out, pv)
		}
		return out, nil
	}
	return []ProjectVersion{}, nil
}

func (s *Service) horizonProjectVersion(ctx context.Context, projectID, versionID string) (ProjectVersion, error) {
	_, p, v, err := s.horizonLookup(ctx, projectID, versionID)
	if err != nil {
		return ProjectVersion{}, err
	}
	return horizonToProjectVersion(p, v), nil
}

func (s *Service) horizonProjectMetas(ctx context.Context, ids []string) map[string]projectMeta {
	out := map[string]projectMeta{}
	idx, err := s.horizonModIndex(ctx, false)
	if err != nil {
		return out
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	for _, p := range idx.Projects {
		if want[p.ID] {
			out[p.ID] = projectMeta{Title: p.Name, Slug: p.ID, Icon: p.Icon}
		}
	}
	return out
}

func (s *Service) verifyHorizonRegistryDownload(p horizonRegistryProject, v horizonRegistryVersion, expectedExts ...string) (DownloadFile, error) {
	f := v.File
	if filepath.Base(f.Name) != f.Name || f.Name == "" || f.Name == "." || f.Name == ".." {
		return DownloadFile{}, horizonErr("Horizon 索引返回了无效文件名", "Horizon index returned an invalid filename")
	}
	extOK := len(expectedExts) == 0
	lowName := strings.ToLower(f.Name)
	for _, ext := range expectedExts {
		if strings.HasSuffix(lowName, ext) {
			extOK = true
			break
		}
	}
	if !extOK {
		return DownloadFile{}, horizonErr("文件扩展名与 Horizon 资源类型不匹配: "+f.Name, "File extension does not match Horizon asset kind: "+f.Name)
	}
	if f.Size <= 0 || f.Size > horizonModFileMaxBytes {
		return DownloadFile{}, horizonErr("Horizon 资源文件体积无效或超过 64 MiB 上限", "Horizon asset file size is invalid or exceeds 64 MiB limit")
	}
	sha := strings.ToLower(strings.TrimSpace(f.SHA256))
	if len(sha) != 64 {
		return DownloadFile{}, horizonErr("Horizon 资源缺少有效的 SHA-256 校验值", "Horizon asset is missing a valid SHA-256 digest")
	}
	if err := s.verifyHorizonAssetURL(p.Source, f.URL); err != nil {
		return DownloadFile{}, err
	}
	if strings.TrimSpace(p.Key) != "" && strings.TrimSpace(f.Signature) == "" {
		return DownloadFile{}, horizonErr("该项目要求 minisign 签名，但当前版本缺少签名", "Project requires a minisign signature, but this version is unsigned")
	}
	return DownloadFile{
		Name:    f.Name,
		URL:     f.URL,
		SHA256:  sha,
		Size:    f.Size,
		Primary: true,
	}, nil
}

func (s *Service) verifyHorizonAssetURL(sourceRepoURL, rawURL string) error {
	if s.horizonTest != nil && s.horizonTest.modIndexURL != "" {
		if testBase, err := url.Parse(s.horizonTest.modIndexURL); err == nil {
			if target, err := url.Parse(rawURL); err == nil && target.Host == testBase.Host {
				return nil
			}
		}
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") {
		return horizonErr("拒绝不受信任的 Horizon 资源下载地址", "Refused untrusted Horizon asset download URL")
	}
	if strings.HasPrefix(u.Path, "/horizon-rust/horizon-mods/releases/download/") {
		return nil
	}
	if src, err := url.Parse(sourceRepoURL); err == nil && src.Scheme == "https" && strings.EqualFold(src.Host, "github.com") {
		parts := strings.Split(strings.Trim(src.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			prefix := "/" + parts[0] + "/" + strings.TrimSuffix(parts[1], ".git") + "/releases/download/"
			if strings.HasPrefix(strings.ToLower(u.Path), strings.ToLower(prefix)) {
				return nil
			}
		}
	}
	return horizonErr("下载地址不属于 Horizon 官方仓库或项目源码 GitHub Releases", "Download URL is not a Horizon registry or project source GitHub Release")
}

// verifyMinisign verifies a minisign / rsign2 signature (both prehashed "ED"
// BLAKE2b-512 and legacy "Ed") against a base64 minisign public key.
func verifyMinisign(pubKeyB64, sigText string, data []byte) error {
	pubRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pubKeyB64))
	if err != nil || len(pubRaw) != 42 || pubRaw[0] != 'R' || pubRaw[1] != 'W' {
		return errors.New("invalid minisign public key")
	}
	keyID := pubRaw[2:10]
	pub := ed25519.PublicKey(pubRaw[10:42])

	lines := []string{}
	for _, l := range strings.Split(strings.ReplaceAll(sigText, "\r\n", "\n"), "\n") {
		if s := strings.TrimSpace(l); s != "" {
			lines = append(lines, s)
		}
	}
	if len(lines) < 4 || !strings.HasPrefix(lines[0], "untrusted comment:") || !strings.HasPrefix(lines[2], "trusted comment:") {
		return errors.New("invalid minisign signature format")
	}
	sigRaw, err := base64.StdEncoding.DecodeString(lines[1])
	if err != nil || len(sigRaw) != 74 {
		return errors.New("invalid minisign signature payload")
	}
	if !slices.Equal(sigRaw[2:10], keyID) {
		return errors.New("minisign key id mismatch")
	}
	sig := sigRaw[10:74]
	var msg []byte
	switch string(sigRaw[:2]) {
	case "ED":
		sum := blake2b512(data)
		msg = sum[:]
	case "Ed":
		msg = data
	default:
		return errors.New("unsupported minisign algorithm")
	}
	if !ed25519.Verify(pub, msg, sig) {
		return errors.New("minisign file signature verification failed")
	}
	trustedComment := strings.TrimPrefix(strings.TrimPrefix(lines[2], "trusted comment:"), " ")
	globalSig, err := base64.StdEncoding.DecodeString(lines[3])
	if err != nil || len(globalSig) != ed25519.SignatureSize {
		return errors.New("invalid minisign global signature")
	}
	globalMsg := make([]byte, len(sig)+len(trustedComment))
	copy(globalMsg, sig)
	copy(globalMsg[len(sig):], trustedComment)
	if !ed25519.Verify(pub, globalMsg, globalSig) {
		return errors.New("minisign trusted comment verification failed")
	}
	return nil
}

// blake2b512 computes an unkeyed RFC 7693 BLAKE2b-512 digest without external deps.
func blake2b512(data []byte) [64]byte {
	iv := [8]uint64{
		0x6a09e667f3bcc908, 0xbb67ae8584caa73b, 0x3c6ef372fe94f82b, 0xa54ff53a5f1d36f1,
		0x510e527fade682d1, 0x9b05688c2b3e6c1f, 0x1f83d9abfb41bd6b, 0x5be0cd19137e2179,
	}
	sigma := [10][16]uint8{
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		{14, 10, 4, 8, 9, 15, 13, 6, 1, 12, 0, 2, 11, 7, 5, 3},
		{11, 8, 12, 0, 5, 2, 15, 13, 10, 14, 3, 6, 7, 1, 9, 4},
		{7, 9, 3, 1, 13, 12, 11, 14, 2, 6, 5, 10, 4, 0, 15, 8},
		{9, 0, 5, 7, 2, 4, 10, 15, 14, 1, 11, 12, 6, 8, 3, 13},
		{2, 12, 6, 10, 0, 11, 8, 3, 4, 13, 7, 5, 15, 14, 1, 9},
		{12, 5, 1, 15, 14, 13, 4, 10, 0, 7, 6, 3, 9, 2, 8, 11},
		{13, 11, 7, 14, 12, 1, 3, 9, 5, 0, 15, 4, 8, 6, 2, 10},
		{6, 15, 14, 9, 11, 3, 0, 8, 12, 2, 13, 7, 1, 4, 10, 5},
		{10, 2, 8, 4, 7, 6, 1, 5, 15, 11, 9, 14, 3, 12, 13, 0},
	}
	h := iv
	h[0] ^= 0x01010040 // fanout=1, depth=1, digest=64

	compress := func(block []byte, t uint64, last bool) {
		var m [16]uint64
		for i := range 16 {
			m[i] = binary.LittleEndian.Uint64(block[i*8:])
		}
		var v [16]uint64
		copy(v[:8], h[:])
		copy(v[8:], iv[:])
		v[12] ^= t
		if last {
			v[14] = ^v[14]
		}
		g := func(a, b, c, d int, x, y uint64) {
			v[a] += v[b] + x
			v[d] = bits.RotateLeft64(v[d]^v[a], -32)
			v[c] += v[d]
			v[b] = bits.RotateLeft64(v[b]^v[c], -24)
			v[a] += v[b] + y
			v[d] = bits.RotateLeft64(v[d]^v[a], -16)
			v[c] += v[d]
			v[b] = bits.RotateLeft64(v[b]^v[c], -63)
		}
		for r := range 12 {
			s := sigma[r%10]
			g(0, 4, 8, 12, m[s[0]], m[s[1]])
			g(1, 5, 9, 13, m[s[2]], m[s[3]])
			g(2, 6, 10, 14, m[s[4]], m[s[5]])
			g(3, 7, 11, 15, m[s[6]], m[s[7]])
			g(0, 5, 10, 15, m[s[8]], m[s[9]])
			g(1, 6, 11, 12, m[s[10]], m[s[11]])
			g(2, 7, 8, 13, m[s[12]], m[s[13]])
			g(3, 4, 9, 14, m[s[14]], m[s[15]])
		}
		for i := range 8 {
			h[i] ^= v[i] ^ v[i+8]
		}
	}

	var offset uint64
	for len(data) > 128 {
		offset += 128
		compress(data[:128], offset, false)
		data = data[128:]
	}
	var tail [128]byte
	copy(tail[:], data)
	offset += uint64(len(data))
	compress(tail[:], offset, true)

	var out [64]byte
	for i := range 8 {
		binary.LittleEndian.PutUint64(out[i*8:], h[i])
	}
	return out
}
