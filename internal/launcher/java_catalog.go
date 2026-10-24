package launcher

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type mojangRuntime struct {
	Manifest struct {
		SHA1 string `json:"sha1"`
		Size int64  `json:"size"`
		URL  string `json:"url"`
	} `json:"manifest"`
	Version struct {
		Name string `json:"name"`
	} `json:"version"`
}

// mojangComponents maps a Java major to Mojang's newest component for it.
func (s *Service) mojangComponents(ctx context.Context) (map[int]string, map[string]mojangRuntime, error) {
	var all map[string]map[string][]mojangRuntime
	if e := s.getJSON(ctx, mojangJavaAll, &all); e != nil {
		return nil, nil, e
	}
	comps := all["windows-x64"]
	byMajor, info := map[int]string{}, map[string]mojangRuntime{}
	names := make([]string, 0, len(comps))
	for n := range comps {
		names = append(names, n)
	}
	sort.Strings(names) // gamma after beta: the later component wins
	for _, n := range names {
		v := comps[n]
		if len(v) == 0 || strings.Contains(n, "snapshot") || n == "minecraft-java-exe" {
			continue
		}
		m := javaMajor(v[0].Version.Name)
		if m == 0 {
			continue
		}
		byMajor[m], info[n] = n, v[0]
	}
	return byMajor, info, nil
}

type adoptiumPkg struct {
	Link     string `json:"link"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
	Name     string `json:"name"`
}

func (s *Service) temurinLatest(ctx context.Context, major int) (adoptiumPkg, string, error) {
	var rel []struct {
		Binary struct {
			Package adoptiumPkg `json:"package"`
		} `json:"binary"`
		Version struct {
			Semver string `json:"semver"`
		} `json:"version"`
	}
	if e := s.getJSON(ctx, fmt.Sprintf(adoptiumLatest, major), &rel); e != nil {
		return adoptiumPkg{}, "", e
	}
	if len(rel) == 0 {
		return adoptiumPkg{}, "", fmt.Errorf("未找到 Temurin Java %d", major)
	}
	return rel[0].Binary.Package, rel[0].Version.Semver, nil
}

// zuluLatest is Azul's newest JRE zip for major, with its SHA-256.
func (s *Service) zuluLatest(ctx context.Context, major int) (adoptiumPkg, string, error) {
	var list []struct {
		UUID string `json:"package_uuid"`
	}
	if e := s.getJSON(ctx, fmt.Sprintf(azulPackages, major), &list); e != nil {
		return adoptiumPkg{}, "", e
	}
	if len(list) == 0 {
		return adoptiumPkg{}, "", fmt.Errorf("未找到 Zulu Java %d", major)
	}
	var d struct {
		URL     string `json:"download_url"`
		Name    string `json:"name"`
		SHA256  string `json:"sha256_hash"`
		Size    int64  `json:"size"`
		Version []int  `json:"java_version"`
	}
	if e := s.getJSON(ctx, fmt.Sprintf(azulPackage, list[0].UUID), &d); e != nil {
		return adoptiumPkg{}, "", e
	}
	parts := make([]string, len(d.Version))
	for i, v := range d.Version {
		parts[i] = itoa(v)
	}
	return adoptiumPkg{Link: d.URL, Checksum: d.SHA256, Size: d.Size, Name: d.Name}, strings.Join(parts, "."), nil
}

// JavaOffers lists what can be installed: 8, 17, 21 and the newest LTS,
// from Mojang first and Temurin as the alternative.
func (s *Service) JavaOffers(ctx context.Context) ([]JavaOffer, error) {
	lts := 25
	var info struct {
		MostRecentLTS int `json:"most_recent_lts"`
	}
	if s.getJSON(ctx, adoptiumReleases, &info) == nil && info.MostRecentLTS > 21 {
		lts = info.MostRecentLTS
	}
	majors := []int{8, 17, 21, lts}
	var out []JavaOffer
	byMajor, comps, e1 := s.mojangComponents(ctx)
	var e2 error
	for _, m := range majors {
		if n, ok := byMajor[m]; ok {
			out = append(out, JavaOffer{Major: m, Source: javaSourceMojang, Version: comps[n].Version.Name, LTS: m == lts})
		}
		if pkg, ver, e := s.temurinLatest(ctx, m); e == nil {
			out = append(out, JavaOffer{Major: m, Source: javaSourceTemurin, Version: ver, Size: pkg.Size, LTS: m == lts})
		} else {
			e2 = e
		}
		if pkg, ver, e := s.zuluLatest(ctx, m); e == nil {
			out = append(out, JavaOffer{Major: m, Source: javaSourceZulu, Version: ver, Size: pkg.Size, LTS: m == lts})
		} else {
			e2 = e
		}
	}
	if len(out) == 0 {
		if e1 != nil {
			return nil, e1
		}
		return nil, e2
	}
	return out, nil
}

// JavaOffer is one installable runtime.
type JavaOffer struct {
	Major   int    `json:"major"`
	Source  string `json:"source"` // mojang | temurin
	Version string `json:"version"`
	Size    int64  `json:"size,omitempty"` // download bytes, when known
	LTS     bool   `json:"lts,omitempty"`
}
