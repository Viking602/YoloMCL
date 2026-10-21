package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const modrinthAPI = "https://api.modrinth.com/v2"
const curseAPI = "https://api.curseforge.com/v1"

type SearchQuery struct {
	Source    string `json:"source"`
	Query     string `json:"query"`
	Type      string `json:"type"`
	Minecraft string `json:"minecraft"`
	Loader    string `json:"loader"`
	Offset    int    `json:"offset"`
	Limit     int    `json:"limit"`
	Sort      string `json:"sort"`
}
type SearchResult struct {
	Projects []Project `json:"projects"`
	Total    int       `json:"total"`
	Offset   int       `json:"offset"`
	Limit    int       `json:"limit"`
}
type mrVersion struct {
	ID           string   `json:"id"`
	ProjectID    string   `json:"project_id"`
	Name         string   `json:"name"`
	Number       string   `json:"version_number"`
	Published    string   `json:"date_published"`
	GameVersions []string `json:"game_versions"`
	Loaders      []string `json:"loaders"`
	Type         string   `json:"version_type"`
	Changelog    string   `json:"changelog"`
	Files        []struct {
		Filename string `json:"filename"`
		URL      string `json:"url"`
		Size     int64  `json:"size"`
		Primary  bool   `json:"primary"`
		Hashes   struct {
			SHA1   string `json:"sha1"`
			SHA512 string `json:"sha512"`
		} `json:"hashes"`
	} `json:"files"`
	Dependencies []struct {
		ProjectID string `json:"project_id"`
		VersionID string `json:"version_id"`
		Kind      string `json:"dependency_type"`
	} `json:"dependencies"`
}

func (v mrVersion) version() ProjectVersion {
	out := ProjectVersion{ID: v.ID, ProjectID: v.ProjectID, Name: v.Name, Number: v.Number, Source: "modrinth", Published: v.Published, GameVersions: v.GameVersions, Loaders: v.Loaders, Type: v.Type, Changelog: v.Changelog, Files: []DownloadFile{}, Dependencies: []Dependency{}}
	for _, f := range v.Files {
		out.Files = append(out.Files, DownloadFile{Name: f.Filename, URL: f.URL, Size: f.Size, Primary: f.Primary, SHA1: f.Hashes.SHA1, SHA512: f.Hashes.SHA512})
	}
	for _, d := range v.Dependencies {
		out.Dependencies = append(out.Dependencies, Dependency{ProjectID: d.ProjectID, VersionID: d.VersionID, Kind: d.Kind})
	}
	return out
}

type cfFile struct {
	ID           int      `json:"id"`
	ModID        int      `json:"modId"`
	DisplayName  string   `json:"displayName"`
	FileName     string   `json:"fileName"`
	FileDate     string   `json:"fileDate"`
	FileLength   int64    `json:"fileLength"`
	DownloadURL  string   `json:"downloadUrl"`
	GameVersions []string `json:"gameVersions"`
	Fingerprint  uint32   `json:"fileFingerprint"`
	ReleaseType  int      `json:"releaseType"`
	Hashes       []struct {
		Value string `json:"value"`
		Algo  int    `json:"algo"`
	} `json:"hashes"`
	Dependencies []struct {
		ModID        int `json:"modId"`
		RelationType int `json:"relationType"`
	} `json:"dependencies"`
}

func (v cfFile) version() ProjectVersion {
	out := ProjectVersion{ID: strconv.Itoa(v.ID), ProjectID: strconv.Itoa(v.ModID), Name: v.DisplayName, Number: v.DisplayName, Source: "curseforge", Type: map[int]string{1: "release", 2: "beta", 3: "alpha"}[v.ReleaseType], Published: v.FileDate, GameVersions: []string{}, Loaders: []string{}, Dependencies: []Dependency{}}
	for _, g := range v.GameVersions {
		switch strings.ToLower(g) {
		case "fabric", "forge", "neoforge", "quilt":
			out.Loaders = append(out.Loaders, strings.ToLower(g))
		default:
			out.GameVersions = append(out.GameVersions, g)
		}
	}
	f := DownloadFile{Name: v.FileName, URL: v.DownloadURL, Size: v.FileLength, Primary: true}
	for _, h := range v.Hashes {
		if h.Algo == 1 {
			f.SHA1 = h.Value
		}
	}
	out.Files = []DownloadFile{f}
	for _, d := range v.Dependencies {
		if d.RelationType == 3 {
			out.Dependencies = append(out.Dependencies, Dependency{ProjectID: strconv.Itoa(d.ModID), Kind: "required"})
		}
	}
	return out
}

// CurseForgeAvailable reports whether this build can reach the CurseForge
// API (via the distribution proxy or an API key).
func (s *Service) CurseForgeAvailable() bool {
	return s.distribution.CurseForgeProxyURL != "" || s.distribution.CurseForgeAPIKey != ""
}

const curseForgeUnavailable = "此构建未配置 CurseForge 服务（缺少 API 密钥），无法下载 CurseForge 整合包中的模组；请改用 Modrinth 版本的整合包"

func (s *Service) cfJSON(ctx context.Context, method, path string, body, out any) error {
	if s.distribution.CurseForgeProxyURL != "" {
		return s.jsonRequest(ctx, method, s.distribution.CurseForgeProxyURL+path, body, nil, out)
	}
	if key := s.distribution.CurseForgeAPIKey; key != "" {
		return cfError(path, s.jsonRequest(ctx, method, curseAPI+path, body, map[string]string{"x-api-key": key}, out))
	}
	return errors.New("此开发构建的 CurseForge 服务尚未启用，请使用 Modrinth 获取资源")
}

// cfError turns CurseForge 403s into a message the player can act on. The
// key itself is never part of any error.
func cfError(path string, e error) error {
	if e == nil || !strings.Contains(e.Error(), "HTTP 403") {
		return e
	}
	if strings.HasPrefix(path, "/mods/search") {
		return errors.New("CurseForge 拒绝了搜索请求（HTTP 403）：当前 API 密钥没有搜索权限。可以改用 Modrinth 搜索，或直接导入 CurseForge 整合包文件")
	}
	return errors.New("CurseForge 拒绝了请求（HTTP 403）：API 密钥无效或权限不足")
}
func loaderType(loader string) string {
	switch loader {
	case "forge":
		return "1"
	case "fabric":
		return "4"
	case "quilt":
		return "5"
	case "neoforge":
		return "6"
	}
	return ""
}
func (s *Service) Search(ctx context.Context, q SearchQuery) (SearchResult, error) {
	switch q.Type {
	case "modpack", "mod", "resourcepack", "shader":
	default:
		q.Type = "mod"
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.Limit <= 0 {
		q.Limit = 18
	}
	q.Limit = min(q.Limit, 50)
	out := SearchResult{Projects: []Project{}, Offset: q.Offset, Limit: q.Limit}
	if q.Source == "curseforge" {
		sort := "2"
		switch q.Sort {
		case "downloads":
			sort = "6"
		case "updated":
			sort = "3"
		}
		v := url.Values{"gameId": {"432"}, "searchFilter": {q.Query}, "pageSize": {strconv.Itoa(q.Limit)}, "index": {strconv.Itoa(q.Offset)}, "sortField": {sort}, "sortOrder": {"desc"}}
		switch q.Type {
		case "modpack":
			v.Set("classId", "4471")
		case "resourcepack":
			v.Set("classId", "12")
		case "shader":
			v.Set("classId", "6552")
		default:
			v.Set("classId", "6")
		}
		if q.Minecraft != "" {
			v.Set("gameVersion", q.Minecraft)
		}
		if q.Type == "mod" || q.Type == "modpack" {
			if l := loaderType(q.Loader); l != "" {
				v.Set("modLoaderType", l)
			}
		}
		var r struct {
			Data []struct {
				ID            int     `json:"id"`
				Name          string  `json:"name"`
				Summary       string  `json:"summary"`
				DownloadCount float64 `json:"downloadCount"`
				Logo          struct {
					URL string `json:"url"`
				} `json:"logo"`
				Links struct {
					WebsiteURL string `json:"websiteUrl"`
				} `json:"links"`
				Authors []struct {
					Name string `json:"name"`
				} `json:"authors"`
				Categories []struct {
					Name string `json:"name"`
				} `json:"categories"`
			} `json:"data"`
			Pagination struct {
				TotalCount int `json:"totalCount"`
			} `json:"pagination"`
		}
		if e := s.cfJSON(ctx, "GET", "/mods/search?"+v.Encode(), nil, &r); e != nil {
			return out, e
		}
		out.Total = r.Pagination.TotalCount
		for _, p := range r.Data {
			a := ""
			if len(p.Authors) > 0 {
				a = p.Authors[0].Name
			}
			c := []string{}
			for _, cat := range p.Categories {
				c = append(c, cat.Name)
			}
			out.Projects = append(out.Projects, Project{ID: strconv.Itoa(p.ID), Source: q.Source, Type: q.Type, Title: p.Name, Description: p.Summary, Icon: p.Logo.URL, Downloads: int64(p.DownloadCount), Author: a, Categories: c, URL: p.Links.WebsiteURL})
		}
		return out, nil
	}
	facets := [][]string{{"project_type:" + q.Type}}
	if q.Minecraft != "" {
		facets = append(facets, []string{"versions:" + q.Minecraft})
	}
	if (q.Type == "mod" || q.Type == "modpack") && q.Loader != "" && q.Loader != "vanilla" {
		facets = append(facets, []string{"categories:" + q.Loader})
	}
	fb, _ := json.Marshal(facets)
	sort := "relevance"
	switch q.Sort {
	case "downloads":
		sort = "downloads"
	case "updated":
		sort = "updated"
	}
	v := url.Values{"query": {q.Query}, "facets": {string(fb)}, "limit": {strconv.Itoa(q.Limit)}, "offset": {strconv.Itoa(q.Offset)}, "index": {sort}}
	var r struct {
		Hits []struct {
			ID          string   `json:"project_id"`
			Slug        string   `json:"slug"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Icon        string   `json:"icon_url"`
			Downloads   int64    `json:"downloads"`
			Author      string   `json:"author"`
			Categories  []string `json:"categories"`
		} `json:"hits"`
		Total int `json:"total_hits"`
	}
	if e := s.getJSON(ctx, modrinthAPI+"/search?"+v.Encode(), &r); e != nil {
		return out, e
	}
	out.Total = r.Total
	for _, p := range r.Hits {
		out.Projects = append(out.Projects, Project{ID: p.ID, Source: "modrinth", Type: q.Type, Title: p.Title, Description: p.Description, Icon: p.Icon, Downloads: p.Downloads, Author: p.Author, Categories: p.Categories, URL: "https://modrinth.com/" + q.Type + "/" + p.Slug})
	}
	return out, nil
}
func (s *Service) Versions(ctx context.Context, source, project, mc, loader string) ([]ProjectVersion, error) {
	out := []ProjectVersion{}
	if project == "" {
		return out, errors.New("项目 ID 不能为空")
	}
	if source == "curseforge" {
		v := url.Values{"pageSize": {"50"}, "index": {"0"}}
		if mc != "" {
			v.Set("gameVersion", mc)
		}
		if l := loaderType(loader); l != "" {
			v.Set("modLoaderType", l)
		}
		var r struct {
			Data []cfFile `json:"data"`
		}
		if e := s.cfJSON(ctx, "GET", "/mods/"+url.PathEscape(project)+"/files?"+v.Encode(), nil, &r); e != nil {
			return out, e
		}
		for _, f := range r.Data {
			out = append(out, f.version())
		}
		return out, nil
	}
	v := url.Values{}
	if mc != "" {
		b, _ := json.Marshal([]string{mc})
		v.Set("game_versions", string(b))
	}
	if loader != "" && loader != "vanilla" {
		b, _ := json.Marshal([]string{loader})
		v.Set("loaders", string(b))
	}
	var r []mrVersion
	if e := s.getJSON(ctx, modrinthAPI+"/project/"+url.PathEscape(project)+"/version?"+v.Encode(), &r); e != nil {
		return out, e
	}
	for _, v := range r {
		out = append(out, v.version())
	}
	return out, nil
}
func (s *Service) projectVersion(ctx context.Context, source, project, id string) (ProjectVersion, error) {
	if source == "curseforge" {
		var r struct {
			Data cfFile `json:"data"`
		}
		e := s.cfJSON(ctx, "GET", "/mods/"+url.PathEscape(project)+"/files/"+url.PathEscape(id), nil, &r)
		return r.Data.version(), e
	}
	var r mrVersion
	e := s.getJSON(ctx, modrinthAPI+"/version/"+url.PathEscape(id), &r)
	return r.version(), e
}
func primaryFile(v ProjectVersion) (DownloadFile, error) {
	for _, f := range v.Files {
		if f.Primary {
			return f, nil
		}
	}
	if len(v.Files) > 0 {
		return v.Files[0], nil
	}
	return DownloadFile{}, fmt.Errorf("版本 %s 没有可下载文件", v.Name)
}
func contains(v []string, s string) bool {
	for _, x := range v {
		if x == s {
			return true
		}
	}
	return false
}
func compatible(v ProjectVersion, mc, loader string) bool {
	return (mc == "" || contains(v.GameVersions, mc)) && (loader == "" || loader == "vanilla" || contains(v.Loaders, loader))
}
