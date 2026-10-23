package launcher

import (
	"archive/zip"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Java runtimes MCL installs live in <data>/runtime/<name>, each with a
// marker file naming its source; only those may be uninstalled.
const (
	javaSourceMojang  = "mojang"
	javaSourceTemurin = "temurin"
	javaSourceZulu    = "zulu"
	azulPackages      = "https://api.azul.com/metadata/v1/zulu/packages/?java_version=%d&os=windows&arch=x64&archive_type=zip&java_package_type=jre&latest=true&release_status=ga&javafx_bundled=false&crac_supported=false&page_size=1"
	azulPackage       = "https://api.azul.com/metadata/v1/zulu/packages/%s"
	javaMarker        = ".mcl-runtime.json"
	mojangJavaAll     = "https://launchermeta.mojang.com/v1/products/java-runtime/2ec0cc96c44e5a76b9c8b7c39df7210883d12871/all.json"
	adoptiumLatest    = "https://api.adoptium.net/v3/assets/latest/%d/hotspot?architecture=x64&image_type=jre&os=windows&vendor=eclipse"
	adoptiumReleases  = "https://api.adoptium.net/v3/info/available_releases"
	tunaAdoptium      = "https://mirrors.tuna.tsinghua.edu.cn/Adoptium/%d/jre/x64/windows/%s"
)

// NeedJavaError: no installed Java fits; the UI offers a one-click install.
type NeedJavaError struct{ Major int }

func (e *NeedJavaError) Error() string {
	return fmt.Sprintf("需要 Java %d，还没有安装", e.Major)
}

// JavaOffer is one installable runtime.
type JavaOffer struct {
	Major   int    `json:"major"`
	Source  string `json:"source"` // mojang | temurin
	Version string `json:"version"`
	Size    int64  `json:"size,omitempty"` // download bytes, when known
	LTS     bool   `json:"lts,omitempty"`
}

// InstalledJava is a found runtime, with what MCL knows about it.
type InstalledJava struct {
	JavaInfo
	Home    string `json:"home"`
	Size    int64  `json:"size"`
	Managed bool   `json:"managed"`
	Source  string `json:"source,omitempty"`
}

type javaMarkerFile struct {
	Source  string `json:"source"`
	Major   int    `json:"major"`
	Version string `json:"version"`
}

func (s *Service) runtimeDir() string { return filepath.Join(s.dir, "runtime") }

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

// InstallJava starts a cancellable task that installs one runtime.
func (s *Service) InstallJava(major int, source string) (string, error) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return "", errors.New("目前只支持在 Windows x64 上下载 Java")
	}
	if major <= 0 {
		return "", errors.New("所选 Java 版本无效")
	}
	return s.startJob(fmt.Sprintf("下载 Java %d", major), "", func(ctx context.Context, p progress) error {
		_, e := s.installJava(ctx, major, source, p)
		return e
	})
}

// byteProgress reports KB done/total, throttled.
type byteProgress struct {
	done, total atomic.Int64
	last        atomic.Int64
	msg         string
	p           progress
	j           *jobState // the task, for its live speed
}

func (b *byteProgress) add(n int64) {
	d := b.done.Add(n)
	if b.j != nil {
		b.j.bytes.Add(n)
		if t := b.total.Load(); t > 0 {
			b.j.bytesTotal.Store(t)
			b.j.totalFixed.Store(true)
		}
	}
	now := time.Now().UnixMilli()
	if l := b.last.Load(); now-l > 150 && b.last.CompareAndSwap(l, now) {
		b.p(b.msg, int(d>>10), int(b.total.Load()>>10))
	}
}

// full reports a finished bar for the unzip and verify phases, so the
// bar never drops back to zero after the download.
func (b *byteProgress) full() (int, int) {
	t := int(b.total.Load() >> 10)
	if t <= 0 {
		t = max(int(b.done.Load()>>10), 1)
	}
	return t, t
}

type countingWriter struct {
	w io.Writer
	b *byteProgress
}

func (c countingWriter) Write(p []byte) (int, error) {
	n, e := c.w.Write(p)
	c.b.add(int64(n))
	return n, e
}

// fetchVerified downloads url to dest, checking size and the hex digest.
func (s *Service) fetchVerified(ctx context.Context, url, dest string, size int64, sum string, h hash.Hash, bp *byteProgress) error {
	if e := validateURL(url); e != nil {
		return e
	}
	res, e := s.request(ctx, "GET", url, nil, nil)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if e := os.MkdirAll(filepath.Dir(dest), 0o755); e != nil {
		return e
	}
	f, e := os.Create(dest)
	if e != nil {
		return e
	}
	before := bp.done.Load()
	n, e := io.Copy(io.MultiWriter(countingWriter{f, bp}, h), io.LimitReader(res.Body, 1<<31))
	f.Close()
	if e == nil && size > 0 && n != size {
		e = fmt.Errorf("下载不完整: %s", javaFileName(dest))
	}
	if e == nil && sum != "" && !strings.EqualFold(sum, hex.EncodeToString(h.Sum(nil))) {
		e = fmt.Errorf("校验失败: %s", javaFileName(dest))
	}
	if e != nil {
		bp.done.Add(before - bp.done.Load()) // a retry counts again
		os.Remove(dest)
	}
	return e
}

func (s *Service) installJava(ctx context.Context, major int, source string, p progress) (string, error) {
	if source == "" {
		source = javaSourceMojang
	}
	root := s.runtimeDir()
	if e := os.MkdirAll(root, 0o755); e != nil {
		return "", e
	}
	stage := filepath.Join(root, ".tmp-"+jobKey(ctx)) // reused by 重试
	s.trackCreated(ctx, stage)
	if e := os.MkdirAll(stage, 0o755); e != nil {
		return "", e
	}
	defer func() {
		if ctx.Err() != nil {
			_ = os.RemoveAll(stage) // cancelled: nothing to resume
		}
	}()
	bp := &byteProgress{msg: fmt.Sprintf("正在下载 Java %d", major), p: p, j: jobOf(ctx)}
	p("正在获取 Java 信息", 0, 0)
	var name, version, home string
	switch source {
	case javaSourceMojang:
		byMajor, comps, e := s.mojangComponents(ctx)
		if e != nil {
			return "", e
		}
		comp, ok := byMajor[major]
		if !ok {
			return "", fmt.Errorf("未找到 Mojang 官方 Java %d，可以换用 Temurin", major)
		}
		name, version, home = comp, comps[comp].Version.Name, stage
		if e := s.mojangInstall(ctx, comps[comp], stage, bp); e != nil {
			return "", e
		}
	case javaSourceTemurin, javaSourceZulu:
		pkg, ver, e := s.temurinLatest(ctx, major)
		if source == javaSourceZulu {
			pkg, ver, e = s.zuluLatest(ctx, major)
		}
		if e != nil {
			return "", e
		}
		if filepath.Base(pkg.Name) != pkg.Name || pkg.Checksum == "" {
			return "", errors.New("获取的 Java 下载信息无效")
		}
		name, version = fmt.Sprintf("%s-%d", source, major), ver
		bp.total.Store(pkg.Size)
		archive := filepath.Join(stage, ".download.zip")
		want := pkg.Size
		if source == javaSourceZulu {
			want = 0 // Azul's listed size can be a few bytes off; the SHA-256 decides
		}
		e = s.fetchVerified(ctx, pkg.Link, archive, want, pkg.Checksum, sha256.New(), bp)
		if e != nil && ctx.Err() == nil && source == javaSourceTemurin {
			// The mirror serves the same file; the checksum still comes
			// from Adoptium, so a mirror cannot swap it.
			e = s.fetchVerified(ctx, fmt.Sprintf(tunaAdoptium, major, pkg.Name), archive, pkg.Size, pkg.Checksum, sha256.New(), bp)
		}
		if e != nil {
			return "", e
		}
		{
			d, t := bp.full()
			p("正在解压 Java", d, t)
		}
		if e := unzipInto(ctx, archive, stage); e != nil {
			return "", e
		}
		os.Remove(archive)
		m, _ := filepath.Glob(filepath.Join(stage, "*", "bin", "java.exe"))
		if len(m) != 1 {
			return "", errors.New("不支持此 Java 运行环境结构")
		}
		home = filepath.Dir(filepath.Dir(m[0]))
	default:
		return "", errors.New("未知的 Java 来源")
	}
	{
		d, t := bp.full()
		p("正在校验 Java", d, t)
	}
	exe := filepath.Join(home, "bin", "java.exe")
	v, e := inspectJava(ctx, exe)
	if e != nil || v.Major != major {
		return "", errors.New("安装后的 Java 校验失败")
	}
	if e := atomicJSON(filepath.Join(home, javaMarker), javaMarkerFile{Source: source, Major: major, Version: version}); e != nil {
		return "", e
	}
	dest := filepath.Join(root, name)
	old := ""
	if _, e := os.Stat(dest); e == nil {
		old = dest + ".old-" + newID()[:6]
		if e := os.Rename(dest, old); e != nil {
			return "", fmt.Errorf("旧的 Java 正在使用，请先关闭游戏: %w", e)
		}
	}
	if e := os.Rename(home, dest); e != nil {
		if old != "" {
			_ = os.Rename(old, dest)
		}
		return "", e
	}
	if old != "" {
		_ = os.RemoveAll(old)
	}
	_ = os.RemoveAll(stage)
	path := filepath.Join(dest, "bin", "java.exe")
	_, _ = s.ScanJava(ctx, false, nil) // register it in the Java list
	return path, nil
}

func unzipInto(ctx context.Context, archive, dir string) error {
	z, e := zip.OpenReader(archive)
	if e != nil {
		return e
	}
	defer z.Close()
	var total uint64
	for _, f := range z.File {
		if ctx.Err() != nil {
			return ctx.Err() // cancelled mid-extract; the stage folder is removed
		}
		total += f.UncompressedSize64
		if total > 3<<30 {
			return errors.New("下载的 Java 压缩包过大")
		}
		dest, e := safeJoin(dir, f.Name)
		if e != nil {
			return e
		}
		if e = extractZipEntry(f, dest); e != nil {
			return e
		}
	}
	return nil
}

// mojangInstall fetches every file of a Mojang runtime manifest, each
// checked against its SHA-1, a few at a time.
func (s *Service) mojangInstall(ctx context.Context, rt mojangRuntime, dir string, bp *byteProgress) error {
	var man struct {
		Files map[string]struct {
			Type       string `json:"type"`
			Executable bool   `json:"executable"`
			Downloads  struct {
				Raw struct {
					SHA1 string `json:"sha1"`
					Size int64  `json:"size"`
					URL  string `json:"url"`
				} `json:"raw"`
			} `json:"downloads"`
		} `json:"files"`
	}
	if e := s.getJSON(ctx, rt.Manifest.URL, &man); e != nil {
		return e
	}
	type job struct {
		path, url, sum string
		size           int64
	}
	var jobs []job
	for name, f := range man.Files {
		dest, e := safeJoin(dir, name)
		if e != nil {
			return e
		}
		switch f.Type {
		case "directory":
			if e := os.MkdirAll(dest, 0o755); e != nil {
				return e
			}
		case "file":
			jobs = append(jobs, job{dest, f.Downloads.Raw.URL, f.Downloads.Raw.SHA1, f.Downloads.Raw.Size})
			bp.total.Add(f.Downloads.Raw.Size)
		}
	}
	if len(jobs) == 0 {
		return errors.New("获取的 Java 清单为空")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan job)
	var wg sync.WaitGroup
	var first error
	var once sync.Once
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				var e error
				for try := 0; try < 3; try++ {
					if e = s.fetchVerified(ctx, j.url, j.path, j.size, j.sum, sha1.New(), bp); e == nil || ctx.Err() != nil {
						break
					}
				}
				if e != nil {
					once.Do(func() { first = e; cancel() })
				}
			}
		}()
	}
	for _, j := range jobs {
		select {
		case ch <- j:
		case <-ctx.Done():
		}
	}
	close(ch)
	wg.Wait()
	if first != nil {
		return first
	}
	return ctx.Err()
}

var dirSizes sync.Map // home -> int64

func dirSize(ctx context.Context, p string) int64 {
	if v, ok := dirSizes.Load(p); ok {
		return v.(int64)
	}
	var n int64
	err := filepath.WalkDir(p, func(_ string, d fs.DirEntry, e error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e == nil && !d.IsDir() {
			if i, e := d.Info(); e == nil {
				n += i.Size()
			}
		}
		return nil
	})
	if err == nil && ctx.Err() == nil {
		dirSizes.Store(p, n)
	}
	return n
}

func javaHome(exe string) string { return filepath.Dir(filepath.Dir(exe)) }

// isManagedJava: inside MCL's runtime folders and carrying the marker
// (older auto-installs in runtimes/ count too).
func (s *Service) isManagedJava(home string) (bool, string) {
	for _, base := range []string{s.runtimeDir()} {
		rel, e := filepath.Rel(base, home)
		if e != nil || rel == "." || strings.HasPrefix(rel, "..") || strings.ContainsAny(rel, `/\`) {
			continue
		}
		var m javaMarkerFile
		if b, e := os.ReadFile(filepath.Join(home, javaMarker)); e == nil && json.Unmarshal(b, &m) == nil {
			return true, m.Source
		}
	}
	return false, ""
}

// InstalledJavas is the Java list with sizes and which ones MCL installed.
func (s *Service) InstalledJavas(ctx context.Context) []InstalledJava {
	return s.InstalledJavaDetails(ctx, s.JavaList(ctx))
}

// InstalledJavaDetails adds display metadata to an existing discovery result.
// It does not inspect environment/registry paths or restart Java discovery.
func (s *Service) InstalledJavaDetails(ctx context.Context, known []JavaInfo) []InstalledJava {
	out := make([]InstalledJava, 0, len(known))
	for _, j := range known {
		if ctx.Err() != nil {
			break
		}
		home := javaHome(j.Path)
		m, src := s.isManagedJava(home)
		out = append(out, InstalledJava{JavaInfo: j, Home: home, Size: dirSize(ctx, home), Managed: m, Source: src})
	}
	return out
}

// UninstallJava removes a runtime MCL installed; any other is refused.
func (s *Service) UninstallJava(home string) error {
	home = filepath.Clean(home)
	if ok, _ := s.isManagedJava(home); !ok {
		return errors.New("只能卸载 YoloMCL 安装的 Java")
	}
	s.mu.Lock()
	for _, g := range s.running {
		if g != nil && g.cmd != nil && g.cmd.Path != "" && strings.HasPrefix(strings.ToLower(filepath.Clean(g.cmd.Path)), strings.ToLower(home)) {
			s.mu.Unlock()
			return errors.New("这个 Java 正在运行游戏，请先关闭游戏")
		}
	}
	s.mu.Unlock()
	tmp := home + ".del-" + newID()[:6]
	if e := os.Rename(home, tmp); e != nil {
		return fmt.Errorf("卸载失败，Java 可能正在使用: %w", e)
	}
	dirSizes.Delete(home)
	go os.RemoveAll(tmp)
	_, _ = s.ScanJava(context.Background(), false, nil)
	return nil
}

func javaFileName(p string) string {
	if strings.HasPrefix(filepath.Base(p), ".download") {
		return "Java 压缩包"
	}
	return filepath.Base(p)
}
