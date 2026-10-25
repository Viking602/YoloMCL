package launcher

import (
	"context"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Only ordinary enabled add-ons can be replaced by provider references.
// Disabled or nested jars must keep their exact path and state in overrides.
func packExportAddon(name, format string) bool {
	dir, base := path.Split(name)
	if dir == "mods/" && strings.HasSuffix(strings.ToLower(base), ".jar") {
		return true
	}
	return format == "modrinth" && (dir == "resourcepacks/" || dir == "shaderpacks/") && strings.HasSuffix(strings.ToLower(base), ".zip")
}

func exportDownloadURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || strings.ContainsAny(raw, " \r\n\t") {
		return false
	}
	// The mrpack format's download-host allowlist; unmatched sources are
	// packaged locally rather than producing an unimportable remote entry.
	switch strings.ToLower(u.Hostname()) {
	case "cdn.modrinth.com", "github.com", "raw.githubusercontent.com", "gitlab.com":
		return true
	}
	return false
}

func (f *packExportFile) modrinthReference(urls []string) bool {
	var downloads []string
	for _, raw := range urls {
		if exportDownloadURL(raw) {
			downloads = append(downloads, raw)
		}
	}
	if len(downloads) == 0 {
		return false
	}
	f.mr = &mrPackFile{Path: f.path, Size: f.size, Downloads: downloads}
	f.mr.Hashes.SHA1, f.mr.Hashes.SHA512 = f.sha1, f.sha512
	return true
}

func (f *packExportFile) curseForgeReference(project, version string) bool {
	pid, e1 := strconv.Atoi(project)
	fid, e2 := strconv.Atoi(version)
	if e1 != nil || e2 != nil || pid <= 0 || fid <= 0 {
		return false
	}
	f.cf = &cfPackFile{ProjectID: pid, FileID: fid, Required: true}
	return true
}

func hashPackExportFile(ctx context.Context, dir *os.Root, f *packExportFile, buf []byte, full bool) error {
	in, err := openPackExportFile(dir, *f)
	if err != nil {
		return err
	}
	defer in.Close()
	a := sha1.New()
	var out io.Writer = a
	b := sha512.New()
	if full {
		out = io.MultiWriter(a, b)
	}
	n, err := io.CopyBuffer(out, modReader{ctx: ctx, r: io.LimitReader(in, f.size+1)}, buf)
	if err != nil {
		return err
	}
	if n != f.size {
		return fmt.Errorf("导出期间文件发生变化，请重试：%s", f.path)
	}
	if err = checkPackExportFile(in, *f); err != nil {
		return err
	}
	f.sha1 = hex.EncodeToString(a.Sum(nil))
	if full {
		f.sha512 = hex.EncodeToString(b.Sum(nil))
	}
	return nil
}

// resolvePackExportSources holds only compact file identities. The existing
// inventory/JAR parser is deliberately not involved in exporting a pack.
func (s *Service) resolvePackExportSources(ctx context.Context, r Root, dir *os.Root, files []packExportFile, format string, p progress) error {
	lockFiles := map[string]PackFile{}
	if lock := readPack(r.Path); lock != nil && (lock.GameVersion == "" || lock.GameVersion == r.SelectedVersion) {
		for _, f := range lock.Files {
			lockFiles[f.Path] = f
		}
	}
	records := readRecords(dir.Name())
	buf := make([]byte, 64<<10)
	var unknown []*packExportFile
	for i := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		f := &files[i]
		if !packExportAddon(f.path, format) {
			continue
		}
		p("校验导出文件："+f.path, i, len(files))
		if err := hashPackExportFile(ctx, dir, f, buf, format == "modrinth"); err != nil {
			return err
		}
		known := lockFiles[f.path]
		if format == "modrinth" {
			if strings.EqualFold(known.SHA1, f.sha1) && (known.SHA512 == "" || strings.EqualFold(known.SHA512, f.sha512)) && f.modrinthReference(known.URLs) {
				continue
			}
		} else {
			if known.Source == "curseforge" && strings.EqualFold(known.SHA1, f.sha1) && f.curseForgeReference(known.ProjectID, known.VersionID) {
				continue
			}
			record := records[path.Base(f.path)]
			if record.Source == "curseforge" && strings.EqualFold(record.SHA1, f.sha1) && f.curseForgeReference(record.ProjectID, record.VersionID) {
				continue
			}
		}
		unknown = append(unknown, f)
	}
	// Keep request bodies and provider responses bounded, even for huge packs.
	for start := 0; start < len(unknown); start += 64 {
		batch := unknown[start:min(start+64, len(unknown))]
		if err := ctx.Err(); err != nil {
			return err
		}
		p("识别导出文件："+format, start, len(unknown))
		lookup, cancel := context.WithTimeout(ctx, 15*time.Second)
		var err error
		if format == "modrinth" {
			err = s.resolveExportModrinth(lookup, batch)
		} else if s.publisher.HasCurseForge() {
			err = s.resolveExportCurseForge(lookup, dir, batch)
		}
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			// Offline exports still contain every selected file. Stop retrying
			// this provider so a disconnected export does not stall per batch.
			p("来源查询不可用，未识别文件将随包打包", 0, 0)
			break
		}
	}
	return nil
}

func (s *Service) resolveExportModrinth(ctx context.Context, files []*packExportFile) error {
	hashes := make([]string, 0, len(files))
	for _, f := range files {
		hashes = append(hashes, f.sha1)
	}
	// Ignore release histories, changelogs and dependencies in this endpoint.
	var known map[string]struct {
		Files []struct {
			URL    string `json:"url"`
			Size   int64  `json:"size"`
			Hashes struct {
				SHA1   string `json:"sha1"`
				SHA512 string `json:"sha512"`
			} `json:"hashes"`
		} `json:"files"`
	}
	if err := s.jsonRequest(ctx, "POST", modrinthAPI+"/version_files", map[string]any{"hashes": hashes, "algorithm": "sha1"}, nil, &known); err != nil {
		return err
	}
	for _, f := range files {
		for _, remote := range known[f.sha1].Files {
			if strings.EqualFold(remote.Hashes.SHA1, f.sha1) && remote.Size == f.size && (remote.Hashes.SHA512 == "" || strings.EqualFold(remote.Hashes.SHA512, f.sha512)) && f.modrinthReference([]string{remote.URL}) {
				break
			}
		}
	}
	return nil
}

func (s *Service) resolveExportCurseForge(ctx context.Context, dir *os.Root, files []*packExportFile) error {
	fps := make([]uint32, 0, len(files))
	for _, f := range files {
		if f.size > maxFingerprintJar {
			continue
		}
		// safeJoin rejects links; OpenRoot confines all subsequent content IO.
		name, err := safeJoin(dir.Name(), f.path)
		if err != nil {
			return err
		}
		fp, err := fingerprintFile(ctx, name)
		if err != nil {
			return err
		}
		fps = append(fps, fp)
	}
	if len(fps) == 0 {
		return nil
	}
	var matches struct {
		Data struct {
			ExactMatches []struct {
				File cfFile `json:"file"`
			} `json:"exactMatches"`
		} `json:"data"`
	}
	if err := s.cfJSON(ctx, "POST", "/fingerprints", map[string]any{"fingerprints": fps}, &matches); err != nil {
		return err
	}
	byHash := make(map[string]cfFile, len(matches.Data.ExactMatches))
	for _, match := range matches.Data.ExactMatches {
		for _, h := range match.File.Hashes {
			if h.Algo == 1 {
				byHash[strings.ToLower(h.Value)] = match.File
			}
		}
	}
	for _, f := range files {
		remote, ok := byHash[f.sha1]
		// CF manifests cannot preserve renamed files. Fingerprints alone are
		// not cryptographic verification; require SHA-1, size and exact name.
		if ok && remote.FileLength == f.size && remote.FileName == filepath.Base(f.path) {
			f.curseForgeReference(fmt.Sprint(remote.ModID), fmt.Sprint(remote.ID))
		}
	}
	return nil
}
