package launcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type PackFileStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}
type PackReport struct {
	Pack   *PackLock        `json:"pack"`
	Files  []PackFileStatus `json:"files"`
	Latest *ProjectVersion  `json:"latest,omitempty"`
}

func (s *Service) CheckPack(ctx context.Context, rootID string) (PackReport, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return PackReport{}, e
	}
	lock := readPack(r.Path)
	if lock == nil {
		return PackReport{}, errors.New("此目录没有整合包导入记录")
	}
	out := PackReport{Pack: lock, Files: []PackFileStatus{}}
	remaining := []ManualDownload{}
	changed := false
	for _, f := range lock.Files {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		path, e := safeJoin(r.Path, f.Path)
		if e != nil {
			return out, e
		}
		status := "original"
		if _, e = os.Stat(path); os.IsNotExist(e) {
			if _, e = os.Stat(path + ".disabled"); e == nil {
				path += ".disabled"
				status = "disabled"
			} else {
				status = "missing"
			}
		}
		if status != "missing" && !checkHashes(path, f.SHA1, f.SHA512) {
			status = "modified"
		}
		out.Files = append(out.Files, PackFileStatus{Path: f.Path, Status: status})
	}
	for _, manual := range lock.Manual {
		path, e := safeJoin(r.Path, "mods/"+manual.Name)
		if e != nil {
			return out, e
		}
		found := false
		for _, f := range lock.Files {
			if f.ProjectID == manual.ProjectID && f.VersionID == manual.FileID && checkHashes(path, f.SHA1, f.SHA512) {
				found = true
				break
			}
		}
		if !found && manual.SHA1 != "" && checkHashes(path, manual.SHA1, "") {
			// The player supplied a manual file that replaces an older copy
			// kept by an update: retire the old one now.
			found = true
			replaced := false
			for i, f := range lock.Files {
				if f.Source == "curseforge" && f.ProjectID == manual.ProjectID {
					if !strings.EqualFold(f.Path, "mods/"+manual.Name) {
						if oldPath, e := safeJoin(r.Path, f.Path); e == nil {
							_ = os.Remove(oldPath)
							_ = os.Remove(oldPath + ".disabled")
						}
					}
					lock.Files[i] = PackFile{Kind: "remote", Path: "mods/" + manual.Name, SHA1: manual.SHA1, Source: "curseforge", ProjectID: manual.ProjectID, VersionID: manual.FileID}
					replaced = true
				}
			}
			if !replaced {
				lock.Files = append(lock.Files, PackFile{Kind: "remote", Path: "mods/" + manual.Name, SHA1: manual.SHA1, Source: "curseforge", ProjectID: manual.ProjectID, VersionID: manual.FileID})
			}
		}
		if found {
			changed = true
		} else {
			remaining = append(remaining, manual)
		}
	}
	if changed {
		lock.Manual = remaining
		if e = s.mutateRoot(rootID, func(r Root) error { return atomicJSON(filepath.Join(r.Path, ".mcl", "pack-lock.json"), lock) }); e != nil {
			return out, e
		}
	}
	if lock.ProjectID != "" {
		info, e := s.CheckPackUpdate(ctx, rootID)
		if e != nil {
			return out, e
		}
		if info.HasUpdate {
			out.Latest = info.Latest
		}
	}
	return out, nil
}
