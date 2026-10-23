package launcher

import (
	"context"
	"sort"
	"strconv"
	"sync"
)

// Category is one filterable tag on a platform. ID is what Search takes:
// the slug on Modrinth, the numeric id on CurseForge.
type Category struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

var catCache sync.Map // source|type -> []Category

var cfClass = map[string]string{"modpack": "4471", "resourcepack": "12", "shader": "6552", "mod": "6"}

// Categories lists a platform's categories for one project type, cached for
// the life of the process.
func (s *Service) Categories(ctx context.Context, source, kind string) ([]Category, error) {
	if _, ok := cfClass[kind]; !ok {
		kind = "mod"
	}
	key := source + "|" + kind
	if v, ok := catCache.Load(key); ok {
		return v.([]Category), nil
	}
	out := []Category{}
	if source == "curseforge" {
		var r struct {
			Data []struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				Slug     string `json:"slug"`
				IsClass  bool   `json:"isClass"`
				ClassID  int    `json:"classId"`
				ParentID int    `json:"parentCategoryId"`
			} `json:"data"`
		}
		if e := s.cfJSON(ctx, "GET", "/categories?gameId=432&classId="+cfClass[kind], nil, &r); e != nil {
			return nil, e
		}
		for _, c := range r.Data {
			if c.IsClass || strconv.Itoa(c.ClassID) != cfClass[kind] {
				continue
			}
			out = append(out, Category{ID: strconv.Itoa(c.ID), Slug: c.Slug, Name: c.Name})
		}
	} else {
		var r []struct {
			Name        string `json:"name"`
			ProjectType string `json:"project_type"`
			Header      string `json:"header"`
		}
		if e := s.getJSON(ctx, modrinthAPI+"/tag/category", &r); e != nil {
			return nil, e
		}
		for _, c := range r {
			if c.ProjectType != kind || (c.Header != "categories" && c.Header != "features") {
				continue
			}
			out = append(out, Category{ID: c.Name, Slug: c.Name, Name: c.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	catCache.Store(key, out)
	return out, nil
}
