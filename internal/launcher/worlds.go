package launcher

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// World is one save folder in the instance's saves directory.
type World struct {
	Dir        string    `json:"dir"`
	Name       string    `json:"name"`
	Mode       string    `json:"mode"` // survival, creative, adventure, spectator, hardcore
	LastPlayed time.Time `json:"lastPlayed,omitempty"`
	Size       int64     `json:"size"`
	Backups    int       `json:"backups"`
	HasIcon    bool      `json:"hasIcon"`
}

// WorldBackup is one zip in backups/<world>/.
type WorldBackup struct {
	File string    `json:"file"`
	Time time.Time `json:"time"`
	Size int64     `json:"size"`
	Auto bool      `json:"auto"` // 还原前自动备份
}

// LevelInfo is the part of level.dat the launcher shows.
type LevelInfo struct {
	Name       string
	GameType   int32
	Hardcore   bool
	LastPlayed int64 // ms since epoch
}

// ParseLevelDat reads LevelName, GameType, hardcore and LastPlayed from a
// gzipped NBT level.dat without decoding anything else.
func ParseLevelDat(r io.Reader) (LevelInfo, error) {
	gz, e := gzip.NewReader(r)
	if e != nil {
		return LevelInfo{}, e
	}
	defer gz.Close()
	b, e := io.ReadAll(io.LimitReader(gz, 64<<20))
	if e != nil {
		return LevelInfo{}, e
	}
	p := &nbtReader{b: b}
	var li LevelInfo
	if p.u8() != 10 { // root compound
		return li, errors.New("not an NBT compound")
	}
	p.str()
	p.compound(func(path, name string, tag byte) bool {
		if path != "/Data" {
			return false
		}
		switch {
		case name == "LevelName" && tag == 8:
			li.Name = p.str()
		case name == "GameType" && tag == 3:
			li.GameType = int32(p.u32())
		case name == "hardcore" && tag == 1:
			li.Hardcore = p.u8() != 0
		case name == "LastPlayed" && tag == 4:
			li.LastPlayed = int64(p.u64())
		default:
			return false
		}
		return true
	}, "")
	if p.err != nil {
		return li, p.err
	}
	return li, nil
}

type nbtReader struct {
	b   []byte
	i   int
	err error
}

func (p *nbtReader) take(n int) []byte {
	if p.err != nil || n < 0 || p.i+n > len(p.b) {
		if p.err == nil {
			p.err = errors.New("truncated NBT")
		}
		p.i = len(p.b)
		return make([]byte, max(n, 0))
	}
	v := p.b[p.i : p.i+n]
	p.i += n
	return v
}
func (p *nbtReader) u8() byte    { return p.take(1)[0] }
func (p *nbtReader) u16() uint16 { return binary.BigEndian.Uint16(p.take(2)) }
func (p *nbtReader) u32() uint32 { return binary.BigEndian.Uint32(p.take(4)) }
func (p *nbtReader) u64() uint64 { return binary.BigEndian.Uint64(p.take(8)) }
func (p *nbtReader) str() string { return string(p.take(int(p.u16()))) }

// compound walks a compound's entries; want reads a value itself and
// returns true, else the value is skipped.
func (p *nbtReader) compound(want func(path, name string, tag byte) bool, path string) {
	for p.err == nil {
		tag := p.u8()
		if tag == 0 {
			return
		}
		name := p.str()
		if want(path, name, tag) {
			continue
		}
		p.skip(tag, want, path+"/"+name)
	}
}

func (p *nbtReader) skip(tag byte, want func(string, string, byte) bool, path string) {
	switch tag {
	case 1:
		p.take(1)
	case 2:
		p.take(2)
	case 3, 5:
		p.take(4)
	case 4, 6:
		p.take(8)
	case 7:
		p.take(int(int32(p.u32())))
	case 8:
		p.str()
	case 9:
		t, n := p.u8(), int(int32(p.u32()))
		for i := 0; i < n && p.err == nil; i++ {
			p.skip(t, func(string, string, byte) bool { return false }, path)
		}
	case 10:
		p.compound(want, path)
	case 11:
		p.take(4 * int(int32(p.u32())))
	case 12:
		p.take(8 * int(int32(p.u32())))
	default:
		p.err = fmt.Errorf("bad NBT tag %d", tag)
	}
}

func (s *Service) worldPaths(rootID, versionID string) (saves, backups string, e error) {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return "", "", e
	}
	d := r.VersionDir()
	saves, e = safeJoin(d, "saves")
	if e != nil {
		return "", "", e
	}
	backups, e = safeJoin(d, "backups")
	return saves, backups, e
}

func validWorldDir(n string) error {
	if n == "" || n != filepath.Base(n) || n == "." || n == ".." || strings.HasPrefix(n, ".") {
		return errors.New("无效的存档名")
	}
	return nil
}

func treeBytes(p string) int64 {
	var n int64
	_ = filepath.WalkDir(p, func(_ string, d fs.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			if fi, e := d.Info(); e == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

// ListWorlds reads every world folder (newest played first).
func (s *Service) ListWorlds(rootID, versionID string) ([]World, error) {
	saves, backups, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return nil, e
	}
	entries, _ := os.ReadDir(saves)
	out := []World{}
	for _, en := range entries {
		if !en.IsDir() || strings.HasPrefix(en.Name(), ".") {
			continue
		}
		w := readWorldSummary(saves, en)
		w.Size = treeBytes(filepath.Join(saves, en.Name()))
		w.Backups = len(listWorldBackups(filepath.Join(backups, en.Name())))
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastPlayed.After(out[j].LastPlayed) })
	return out, nil
}

// readWorldSummary reads only the save's metadata, without walking its regions.
// Both the world list and the overview use the same name and play date.
func readWorldSummary(saves string, en os.DirEntry) World {
	p := filepath.Join(saves, en.Name())
	w := World{Dir: en.Name(), Name: en.Name(), Mode: "survival"}
	level, levelErr := safeJoin(p, "level.dat")
	if levelErr == nil {
		if f, e := os.Open(level); e == nil {
			if li, e := ParseLevelDat(f); e == nil {
				if li.Name != "" {
					w.Name = li.Name
				}
				w.Mode = [...]string{"survival", "creative", "adventure", "spectator"}[min(max(li.GameType, 0), 3)]
				if li.Hardcore {
					w.Mode = "hardcore"
				}
				if li.LastPlayed > 0 {
					w.LastPlayed = time.UnixMilli(li.LastPlayed)
				}
			}
			f.Close()
		}
	}
	if w.LastPlayed.IsZero() {
		if fi, e := en.Info(); e == nil {
			w.LastPlayed = fi.ModTime()
		}
	}
	_, e := os.Stat(filepath.Join(p, "icon.png"))
	w.HasIcon = e == nil
	return w
}

func listWorldBackups(dir string) []WorldBackup {
	entries, _ := os.ReadDir(dir)
	out := []WorldBackup{}
	for _, en := range entries {
		n := en.Name()
		if en.IsDir() || !strings.HasSuffix(n, ".zip") {
			continue
		}
		fi, e := en.Info()
		if e != nil {
			continue
		}
		b := WorldBackup{File: n, Time: fi.ModTime(), Size: fi.Size(), Auto: strings.HasSuffix(n, "-auto.zip")}
		stem := strings.TrimSuffix(strings.TrimSuffix(n, ".zip"), "-auto")
		if i := len(stem) - 15; i > 0 {
			if t, e := time.ParseInLocation("20060102-150405", stem[i:], time.Local); e == nil {
				b.Time = t
			}
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out
}

// WorldBackups lists a world's backups, newest first.
func (s *Service) WorldBackups(rootID, versionID, world string) ([]WorldBackup, error) {
	if e := validWorldDir(world); e != nil {
		return nil, e
	}
	_, backups, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return nil, e
	}
	return listWorldBackups(filepath.Join(backups, world)), nil
}

// WorldIcon returns a world's icon.png.
func (s *Service) WorldIcon(rootID, versionID, world string) ([]byte, error) {
	if e := validWorldDir(world); e != nil {
		return nil, e
	}
	saves, _, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return nil, e
	}
	p := filepath.Join(saves, world, "icon.png")
	if fi, e := os.Stat(p); e != nil || fi.Size() > 1<<20 {
		return nil, errors.New("no icon")
	}
	return os.ReadFile(p)
}

func (s *Service) instanceRunning(rootID, versionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := runKey(rootID, versionID)
	for key := range s.running {
		if key == k || strings.HasPrefix(key, k+"#") {
			return true
		}
	}
	return false
}

// zipWorld writes src into dest atomically (dest.tmp then rename),
// skipping session.lock which the game keeps locked.
func zipWorld(ctx context.Context, src, dest string, onBytes func(int64)) error {
	if e := os.MkdirAll(filepath.Dir(dest), 0o755); e != nil {
		return e
	}
	tmp := dest + ".mcl-part"
	f, e := os.Create(tmp)
	if e != nil {
		return e
	}
	zw := zip.NewWriter(f)
	base := filepath.Base(src)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() || d.Name() == "session.lock" {
			return nil
		}
		rel, _ := filepath.Rel(src, p)
		in, e := os.Open(p)
		if e != nil {
			return e
		}
		defer in.Close()
		fi, _ := d.Info()
		h, _ := zip.FileInfoHeader(fi)
		h.Name = filepath.ToSlash(filepath.Join(base, rel))
		h.Method = zip.Deflate
		w, e := zw.CreateHeader(h)
		if e != nil {
			return e
		}
		n, e := io.Copy(w, in)
		if onBytes != nil {
			onBytes(n)
		}
		return e
	})
	if e := zw.Close(); err == nil {
		err = e
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

// unzipWorld extracts a backup made by zipWorld into dir (its top folder
// is stripped), refusing paths that escape.
func unzipWorld(ctx context.Context, archive, dir string) error {
	zr, e := zip.OpenReader(archive)
	if e != nil {
		return e
	}
	defer zr.Close()
	for _, f := range zr.File {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		name := filepath.ToSlash(f.Name)
		if i := strings.IndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		out := filepath.Join(dir, filepath.FromSlash(name))
		if rel, e := filepath.Rel(dir, out); e != nil || strings.HasPrefix(rel, "..") {
			return errors.New("备份中有不安全的路径")
		}
		if e := os.MkdirAll(filepath.Dir(out), 0o755); e != nil {
			return e
		}
		rc, e := f.Open()
		if e != nil {
			return e
		}
		w, e := os.Create(out)
		if e == nil {
			_, e = io.Copy(w, rc)
			if e2 := w.Close(); e == nil {
				e = e2
			}
		}
		rc.Close()
		if e != nil {
			return e
		}
	}
	return nil
}

func backupName(world string, t time.Time, auto bool) string {
	n := world + "-" + t.Format("20060102-150405")
	if auto {
		n += "-auto"
	}
	return n + ".zip"
}

// BackupWorld zips one world (or every world when world is "") into
// backups/<world>/. A world open in a running game is still backed up
// (session.lock skipped), and the job message says so.
func (s *Service) BackupWorld(rootID, versionID, world string) (string, error) {
	saves, backups, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return "", e
	}
	var worlds []string
	if world == "" {
		ws, _ := s.ListWorlds(rootID, versionID)
		for _, w := range ws {
			worlds = append(worlds, w.Dir)
		}
		if len(worlds) == 0 {
			return "", errors.New("没有可备份的存档")
		}
	} else {
		if e := validWorldDir(world); e != nil {
			return "", e
		}
		worlds = []string{world}
	}
	running := s.instanceRunning(rootID, versionID)
	title := "备份存档 " + world
	if world == "" {
		title = "备份全部存档"
	}
	return s.startWorldJob(rootID, versionID, title, func(ctx context.Context, p progress) error {
		var total int64
		for _, w := range worlds {
			total += treeBytes(filepath.Join(saves, w))
		}
		setJobBytesTotal(ctx, total)
		now := time.Now()
		for i, w := range worlds {
			msg := fmt.Sprintf("正在备份 %s", w)
			if running {
				msg += "（游戏运行中，备份可能不是最新状态）"
			}
			p(msg, i, len(worlds))
			dest := filepath.Join(backups, w, backupName(w, now, false))
			s.trackCreated(ctx, dest)
			s.trackCreated(ctx, dest+".mcl-part")
			if e := zipWorld(ctx, filepath.Join(saves, w), dest, func(n int64) { addJobBytes(ctx, n) }); e != nil {
				return e
			}
		}
		p("备份完成", len(worlds), len(worlds))
		return nil
	})
}

// RestoreWorld replaces a world with a backup: blocked while the instance
// runs, auto-backs up the current world first, extracts to a temp folder
// and swaps it in so a failure never leaves a half world.
func (s *Service) RestoreWorld(rootID, versionID, world, file string) (string, error) {
	if e := validWorldDir(world); e != nil {
		return "", e
	}
	if file != filepath.Base(file) || !strings.HasSuffix(file, ".zip") {
		return "", errors.New("无效的备份")
	}
	saves, backups, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return "", e
	}
	if s.instanceRunning(rootID, versionID) {
		return "", errors.New("游戏正在运行，请先退出游戏再还原存档")
	}
	archive := filepath.Join(backups, world, file)
	if _, e := os.Stat(archive); e != nil {
		return "", errors.New("备份文件不存在")
	}
	return s.startWorldJob(rootID, versionID, "还原存档 "+world, func(ctx context.Context, p progress) error {
		return s.restoreWorld(ctx, p, saves, backups, world, archive)
	})
}

func (s *Service) startWorldJob(rootID, versionID, title string, fn func(context.Context, progress) error) (string, error) {
	id, e := s.startJobForMode(title, "", "", true, fn)
	if e == nil {
		s.mu.Lock()
		s.jobs[id].TargetRootID, s.jobs[id].TargetVersion = rootID, versionID
		s.mu.Unlock()
	}
	return id, e
}

func (s *Service) restoreWorld(ctx context.Context, p progress, saves, backups, world, archive string) error {
	cur := filepath.Join(saves, world)
	if _, e := os.Stat(cur); e == nil {
		p("还原前自动备份", 0, 3)
		if e := zipWorld(ctx, cur, filepath.Join(backups, world, backupName(world, time.Now(), true)), nil); e != nil {
			return fmt.Errorf("还原前自动备份失败：%w", e)
		}
	}
	p("正在解压备份", 1, 3)
	tag := fmt.Sprint(time.Now().UnixNano())
	tmp := filepath.Join(saves, ".mcl-restore-"+tag)
	s.trackCreated(ctx, tmp)
	if e := unzipWorld(ctx, archive, tmp); e != nil {
		_ = os.RemoveAll(tmp)
		return e
	}
	if _, e := os.Stat(filepath.Join(tmp, "level.dat")); e != nil {
		_ = os.RemoveAll(tmp)
		return errors.New("备份里没有 level.dat，不是有效的存档")
	}
	p("正在替换存档", 2, 3)
	old := filepath.Join(saves, ".mcl-old-"+tag)
	hadOld := false
	if _, e := os.Stat(cur); e == nil {
		if e := os.Rename(cur, old); e != nil {
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("存档被占用，无法替换：%w", e)
		}
		hadOld = true
	}
	if e := os.Rename(tmp, cur); e != nil {
		if hadOld {
			_ = os.Rename(old, cur)
		}
		_ = os.RemoveAll(tmp)
		return e
	}
	if hadOld {
		_ = os.RemoveAll(old)
	}
	p("还原完成", 3, 3)
	return nil
}

// DeleteWorldBackup removes one backup zip.
func (s *Service) DeleteWorldBackup(rootID, versionID, world, file string) error {
	if e := validWorldDir(world); e != nil {
		return e
	}
	if file != filepath.Base(file) || !strings.HasSuffix(file, ".zip") {
		return errors.New("无效的备份")
	}
	_, backups, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return e
	}
	return os.Remove(filepath.Join(backups, world, file))
}
