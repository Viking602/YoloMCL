package launcher

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// HorizonUpdateInfo describes the current Horizon build of an instance and
// available upstream/cached builds for in-place upgrade or switching.
type HorizonUpdateInfo struct {
	CurrentBuild string           `json:"currentBuild"`
	Latest       *HorizonRelease  `json:"latest,omitempty"`
	HasUpdate    bool             `json:"hasUpdate"`
	Releases     []HorizonRelease `json:"releases,omitempty"`
	LocalBuilds  []string         `json:"localBuilds,omitempty"`
	CheckedAt    string           `json:"checkedAt,omitempty"`
}

// HorizonReleases returns published Horizon builds for the current platform,
// newest first.
func (s *Service) HorizonReleases(ctx context.Context, force bool) ([]HorizonRelease, error) {
	return s.horizonReleases(ctx, force)
}

func (s *Service) localHorizonBuilds() []string {
	entries, err := os.ReadDir(filepath.Join(s.dir, "horizon", "builds"))
	if err != nil {
		return nil
	}
	var out []string
	for _, en := range entries {
		if !en.IsDir() || !validHorizonBuildID(en.Name()) {
			continue
		}
		if horizonBuildInstalled(filepath.Join(s.dir, "horizon", "builds", en.Name())) {
			out = append(out, en.Name())
		}
	}
	slices.SortFunc(out, func(a, b string) int { return strings.Compare(b, a) })
	return out
}

func (s *Service) cachedHorizonRelease(buildID string) (HorizonRelease, bool) {
	if buildID == "" {
		return HorizonRelease{}, false
	}
	s.horizonMu.Lock()
	defer s.horizonMu.Unlock()
	if s.horizonCache == nil {
		return HorizonRelease{}, false
	}
	for _, rel := range s.horizonCache.releases {
		if rel.BuildID == buildID {
			return rel, true
		}
	}
	return HorizonRelease{}, false
}

// CheckHorizonUpdate compares an installed Horizon version against published
// Horizon releases and also lists locally cached builds for offline switching.
func (s *Service) CheckHorizonUpdate(ctx context.Context, rootID, versionID string, force bool) (HorizonUpdateInfo, error) {
	r, err := s.rootAt(rootID, versionID)
	if err != nil {
		return HorizonUpdateInfo{}, err
	}
	if r.SelectedVersion == "" {
		return HorizonUpdateInfo{}, horizonErr("请先选择一个已安装的 Horizon 版本", "Please select an installed Horizon version first")
	}
	_, cur, err := findVersionJSON(r.Path, r.SelectedVersion)
	if err != nil {
		return HorizonUpdateInfo{}, err
	}
	if cur.Horizon == nil || cur.Horizon.BuildID == "" {
		return HorizonUpdateInfo{}, horizonErr("此版本不是 Horizon 实例", "This version is not a Horizon instance")
	}
	info := HorizonUpdateInfo{
		CurrentBuild: cur.Horizon.BuildID,
		LocalBuilds:  s.localHorizonBuilds(),
		CheckedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	releases, err := s.horizonReleases(ctx, force)
	if err != nil {
		if !force && len(info.LocalBuilds) > 0 {
			return info, nil
		}
		return info, err
	}
	info.Releases = releases
	if len(releases) > 0 {
		latest := releases[0]
		info.Latest = &latest
		info.HasUpdate = latest.BuildID != "" && latest.BuildID != cur.Horizon.BuildID
	}
	return info, nil
}

// UpgradeHorizon upgrades (or switches) an existing Horizon version in place
// to targetBuild (or the latest published build when targetBuild is empty).
// Saves, mods, resource packs, shaders, and configuration in the version
// directory remain untouched; only the shared engine build/prepared assets and
// the version JSON metadata are updated.
func (s *Service) UpgradeHorizon(rootID, versionID, targetBuild string) (string, error) {
	r, err := s.rootAt(rootID, versionID)
	if err != nil {
		return "", err
	}
	if r.SelectedVersion == "" {
		return "", horizonErr("请先选择一个已安装的 Horizon 版本", "Please select an installed Horizon version first")
	}
	targetBuild = strings.TrimSpace(targetBuild)
	if targetBuild != "" && !validHorizonBuildID(targetBuild) {
		return "", horizonErr("Horizon 版本标识无效", "Invalid Horizon build identifier")
	}
	_, cur, err := findVersionJSON(r.Path, r.SelectedVersion)
	if err != nil {
		return "", err
	}
	if cur.Horizon == nil || cur.Horizon.BuildID == "" {
		return "", horizonErr("此版本不是 Horizon 实例", "This version is not a Horizon instance")
	}
	title := "更新 Horizon"
	if targetBuild != "" {
		title = "更新 Horizon " + targetBuild
	}
	targetVersion := r.SelectedVersion
	return s.startJobAllowRunning(title, r.ID, func(ctx context.Context, p progress) error {
		s.mu.Lock()
		if j := jobOf(ctx); j != nil {
			j.TargetRootID, j.TargetVersion = r.ID, targetVersion
		}
		s.mu.Unlock()

		wantBuild := targetBuild
		wantTag := ""
		if wantBuild == "" {
			p("获取 Horizon 版本列表", 0, 0)
			releases, err := s.horizonReleases(ctx, true)
			if err != nil {
				return err
			}
			if len(releases) == 0 {
				return horizonErr("未找到当前平台可用的 Horizon 发布版本", "No Horizon release available for this platform")
			}
			wantBuild = releases[0].BuildID
			wantTag = releases[0].ReleaseTag
		}

		rel, buildDir, err := s.ensureHorizonBuild(ctx, wantBuild, wantTag, p)
		if err != nil {
			return err
		}
		if rel.ReleaseTag == "" || rel.Name == "" || rel.AssetName == "" {
			if cached, ok := s.cachedHorizonRelease(rel.BuildID); ok {
				rel = mergeHorizonReleaseMeta(rel, cached)
			}
		}

		prepAPI, err := queryHorizonPrepareAPI(ctx, buildDir)
		if err != nil {
			return err
		}
		preparedDir, err := s.horizonPreparedDir(prepAPI)
		if err != nil {
			return err
		}
		if err := s.prepareHorizonResources(ctx, r, buildDir, prepAPI, preparedDir, p); err != nil {
			return err
		}

		jsonPath, meta, err := findVersionJSON(r.Path, targetVersion)
		if err != nil {
			return err
		}
		if meta.Horizon == nil {
			return horizonErr("此版本不是 Horizon 实例", "This version is not a Horizon instance")
		}
		if meta.ID == "" {
			meta.ID = targetVersion
		}
		meta.ClientVersion = prepAPI.Minecraft
		if meta.Type == "" {
			meta.Type = "release"
		}
		meta.JavaVersion.MajorVersion = prepAPI.JavaMajor
		prev := meta.Horizon
		next := &horizonVersionMeta{
			BuildID:     rel.BuildID,
			ReleaseTag:  rel.ReleaseTag,
			ReleaseName: rel.Name,
			Minecraft:   prepAPI.Minecraft,
			JavaMajor:   prepAPI.JavaMajor,
			Asset:       rel.AssetName,
		}
		if prev != nil && prev.BuildID == rel.BuildID {
			if next.ReleaseTag == "" {
				next.ReleaseTag = prev.ReleaseTag
			}
			if next.ReleaseName == "" {
				next.ReleaseName = prev.ReleaseName
			}
			if next.Asset == "" {
				next.Asset = prev.Asset
			}
		}
		meta.Horizon = next
		if err := atomicJSON(jsonPath, meta); err != nil {
			return err
		}
		s.invalidateStatus(r.VersionDir())
		s.mu.Lock()
		if j := jobOf(ctx); j != nil && j.TitleKey == "horizon.upgradeLatest" {
			j.Title = "更新 Horizon " + rel.BuildID
			j.TitleKey, j.TitleArg, _ = JobTitleKey(j.Title)
		}
		s.mu.Unlock()
		p("已更新到 Horizon "+rel.BuildID, 1, 1)
		return nil
	})
}
