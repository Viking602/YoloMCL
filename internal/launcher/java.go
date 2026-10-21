package launcher

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type JavaInfo struct {
	Path         string `json:"path"`
	Version      string `json:"version"`
	Major        int    `json:"major"`
	Vendor       string `json:"vendor,omitempty"`
	Architecture string `json:"architecture,omitempty"`
}

func inspectJava(ctx context.Context, path string) (JavaInfo, error) {
	// javaw has no console output; inspect its sibling java executable instead.
	check := path
	if strings.EqualFold(filepath.Base(path), "javaw.exe") {
		check = filepath.Join(filepath.Dir(path), "java.exe")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, check, "-version")
	hideWindow(cmd)
	b, e := cmd.CombinedOutput()
	if e != nil {
		return JavaInfo{}, e
	}
	re := regexp.MustCompile(`(?:version |openjdk )"?([0-9][0-9._+\-a-zA-Z]*)`)
	m := re.FindStringSubmatch(string(b))
	if len(m) < 2 {
		return JavaInfo{}, errors.New("无法识别 Java 版本")
	}
	major := javaMajor(m[1])
	if major == 0 {
		return JavaInfo{}, errors.New("无法识别 Java 主版本")
	}
	return JavaInfo{Path: path, Version: m[1], Major: major}, nil
}
func (s *Service) JavaList(ctx context.Context) []JavaInfo {
	out, _ := s.ScanJava(ctx, false, nil)
	return out
}
func (s *Service) ensureJava(ctx context.Context, r Root, major int, p progress) (string, error) {
	settings := s.settings()
	explicit, _, _, err := r.ResolveRuntime(r.SelectedVersion, settings)
	if err != nil {
		return "", err
	}
	if explicit != "" {
		v, e := inspectJava(ctx, explicit)
		if e != nil {
			return "", fmt.Errorf("Java 路径无效: %w", e)
		}
		if v.Major < major {
			return "", fmt.Errorf("此游戏需要 Java %d，所选版本为 Java %d", major, v.Major)
		}
		return explicit, nil
	}
	for _, v := range s.JavaList(ctx) {
		if v.Major == major {
			return v.Path, nil
		}
	}
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return "", fmt.Errorf("请在设置中指定 Java %d 路径", major)
	}
	p(fmt.Sprintf("正在获取 Java %d 运行环境", major), 0, 0)
	var releases []struct {
		Binary struct {
			Package struct {
				Link     string `json:"link"`
				Checksum string `json:"checksum"`
				Size     int64  `json:"size"`
				Name     string `json:"name"`
			} `json:"package"`
		} `json:"binary"`
	}
	endpoint := fmt.Sprintf("https://api.adoptium.net/v3/assets/latest/%d/hotspot?architecture=x64&image_type=jre&os=windows&vendor=eclipse", major)
	if e := s.getJSON(ctx, endpoint, &releases); e != nil {
		return "", e
	}
	if len(releases) == 0 {
		return "", fmt.Errorf("找不到 Java %d，请在设置中手动指定", major)
	}
	pkg := releases[0].Binary.Package
	archive := filepath.Join(s.dir, "cache", pkg.Name)
	if filepath.Base(pkg.Name) != pkg.Name || pkg.Checksum == "" {
		return "", errors.New("Java 下载元数据无效")
	}
	if e := s.download(ctx, DownloadFile{Name: pkg.Name, URL: pkg.Link, SHA256: pkg.Checksum, Size: pkg.Size}, archive); e != nil {
		return "", e
	}
	stage := filepath.Join(s.dir, "runtimes", ".install-"+newID())
	if e := os.MkdirAll(stage, 0755); e != nil {
		return "", e
	}
	defer os.RemoveAll(stage)
	z, e := zip.OpenReader(archive)
	if e != nil {
		return "", e
	}
	defer z.Close()
	var total uint64
	for _, f := range z.File {
		total += f.UncompressedSize64
		if total > 3<<30 {
			return "", errors.New("Java 压缩包过大")
		}
		dest, e := safeJoin(stage, f.Name)
		if e != nil {
			return "", e
		}
		if e = extractZipEntry(f, dest); e != nil {
			return "", e
		}
	}
	matches, _ := filepath.Glob(filepath.Join(stage, "*", "bin", "java.exe"))
	if len(matches) != 1 {
		return "", errors.New("Java 运行环境结构不受支持")
	}
	base := filepath.Dir(filepath.Dir(matches[0]))
	dest := filepath.Join(s.dir, "runtimes", fmt.Sprintf("java-%d-%s", major, newID()[:6]))
	if e = os.Rename(base, dest); e != nil {
		return "", e
	}
	path := filepath.Join(dest, "bin", "java.exe")
	v, e := inspectJava(ctx, path)
	if e != nil || v.Major != major {
		return "", errors.New("Java 安装后校验失败")
	}
	return path, nil
}
func (s *Service) ensureInjector(ctx context.Context, p progress) (string, error) {
	var meta struct {
		URL       string `json:"download_url"`
		Build     int    `json:"build_number"`
		Checksums struct {
			SHA256 string `json:"sha256"`
		} `json:"checksums"`
	}
	p("获取 authlib-injector", 0, 0)
	if e := s.getJSON(ctx, "https://authlib-injector.yushi.moe/artifact/latest.json", &meta); e != nil {
		return "", e
	}
	if meta.Checksums.SHA256 == "" {
		return "", errors.New("authlib-injector 未提供校验值")
	}
	dest := filepath.Join(s.dir, "cache", fmt.Sprintf("authlib-injector-%d.jar", meta.Build))
	return dest, s.download(ctx, DownloadFile{Name: "authlib-injector", URL: meta.URL, SHA256: meta.Checksums.SHA256}, dest)
}
func (s *Service) LoaderVersions(ctx context.Context, loader, mc string) ([]string, error) {
	out := []string{}
	switch loader {
	case "vanilla":
		return out, nil
	case "fabric", "quilt":
		base := "https://meta.fabricmc.net/v2"
		if loader == "quilt" {
			base = "https://meta.quiltmc.org/v3"
		}
		var r []struct {
			Loader struct {
				Version string `json:"version"`
			} `json:"loader"`
		}
		if e := s.getJSON(ctx, base+"/versions/loader/"+url.PathEscape(mc), &r); e != nil {
			return nil, e
		}
		for _, v := range r {
			out = append(out, v.Loader.Version)
		}
		return out, nil
	case "forge":
		var versions map[string][]string
		if e := s.getJSON(ctx, "https://files.minecraftforge.net/net/minecraftforge/forge/maven-metadata.json", &versions); e != nil {
			return nil, e
		}
		for i := len(versions[mc]) - 1; i >= 0; i-- {
			out = append(out, strings.TrimPrefix(versions[mc][i], mc+"-"))
		}
		return out, nil
	case "neoforge":
		var r struct {
			Versions []string `json:"versions"`
		}
		if e := s.getJSON(ctx, "https://maven.neoforged.net/api/maven/versions/releases/net/neoforged/neoforge", &r); e != nil {
			return nil, e
		}
		parts := strings.Split(mc, ".")
		if len(parts) < 3 {
			return nil, errors.New("NeoForge 需要 Minecraft 1.20.1 或更高版本")
		}
		prefix := parts[1] + "." + parts[2] + "."
		for i := len(r.Versions) - 1; i >= 0; i-- {
			if strings.HasPrefix(r.Versions[i], prefix) {
				out = append(out, r.Versions[i])
			}
		}
		return out, nil
	}
	return nil, errors.New("未知加载器")
}
func (s *Service) installLoader(ctx context.Context, root, mc, loader, version string, p progress) (string, error) {
	if loader == "" || loader == "vanilla" {
		return mc, nil
	}
	if version == "" {
		versions, e := s.LoaderVersions(ctx, loader, mc)
		if e != nil {
			return "", e
		}
		if len(versions) == 0 {
			return "", errors.New("此 Minecraft 版本没有可用加载器")
		}
		version = versions[0]
	}
	if strings.ContainsAny(version, "/\\:") {
		return "", errors.New("加载器版本无效")
	}
	p("安装 "+loader+" "+version, 0, 0)
	if loader == "fabric" || loader == "quilt" {
		base := "https://meta.fabricmc.net/v2"
		if loader == "quilt" {
			base = "https://meta.quiltmc.org/v3"
		}
		var v gameVersion
		if e := s.getJSON(ctx, base+"/versions/loader/"+url.PathEscape(mc)+"/"+url.PathEscape(version)+"/profile/json", &v); e != nil {
			return "", e
		}
		if v.ID == "" || v.MainClass == "" || v.InheritsFrom != mc {
			return "", errors.New("加载器配置无效")
		}
		dest, e := safeJoin(root, "versions/"+v.ID+"/"+v.ID+".json")
		if e != nil {
			return "", e
		}
		return v.ID, atomicJSON(dest, v)
	}
	if loader != "forge" && loader != "neoforge" {
		return "", errors.New("未知加载器")
	}
	base := "https://maven.minecraftforge.net/net/minecraftforge/forge/" + mc + "-" + version + "/forge-" + mc + "-" + version + "-installer.jar"
	if loader == "neoforge" {
		base = "https://maven.neoforged.net/releases/net/neoforged/neoforge/" + version + "/neoforge-" + version + "-installer.jar"
	}
	dest := filepath.Join(s.dir, "cache", loader+"-"+mc+"-"+version+"-installer.jar")
	if e := s.download(ctx, DownloadFile{Name: loader + " 安装器", URL: base}, dest); e != nil {
		return "", e
	}
	v, e := readVersion(root, mc)
	if e != nil {
		return "", e
	}
	major := v.JavaVersion.MajorVersion
	if major == 0 {
		major = 8
	}
	java, e := s.ensureJava(ctx, Root{}, major, p)
	if e != nil {
		return "", e
	}
	profile := filepath.Join(root, "launcher_profiles.json")
	if _, e = os.Stat(profile); os.IsNotExist(e) {
		if e = atomicJSON(profile, map[string]any{"profiles": map[string]any{}}); e != nil {
			return "", e
		}
	}
	cmd := exec.CommandContext(ctx, java, "-jar", dest, "--installClient", root)
	cmd.Dir = root
	hideWindow(cmd)
	logs := &gameLog{lines: []string{}}
	cmd.Stdout = logs
	cmd.Stderr = logs
	if e = cmd.Run(); e != nil {
		_ = atomicJSON(filepath.Join(root, ".mcl", "loader-install-log.json"), logs.read())
		return "", fmt.Errorf("加载器安装失败，详情见 .mcl/loader-install-log.json: %w", e)
	}
	for _, version := range scanVersions(root) {
		if version.Loader == loader && version.Minecraft == mc {
			return version.ID, nil
		}
	}
	return "", errors.New("安装器没有生成可用的客户端版本")
}
func (s *Service) InstallGame(rootID, parent, name, mc, loader, loaderVersion string) (string, error) {
	var r Root
	var e error
	if rootID != "" {
		r, e = s.getRoot(rootID)
		if e != nil {
			return "", e
		}
	}
	return s.startJob("安装 Minecraft "+mc, rootID, func(ctx context.Context, p progress) error {
		if rootID == "" {
			r, e = s.createEmptyRoot(parent, name)
			if e != nil {
				return e
			}
			s.mu.Lock()
			s.rootLocks[r.ID] = true
			s.mu.Unlock()
			defer func() { s.mu.Lock(); delete(s.rootLocks, r.ID); s.mu.Unlock() }()
		}
		if _, e = s.installVanilla(ctx, r.Path, mc, p); e != nil {
			return e
		}
		id, e := s.installLoader(ctx, r.Path, mc, loader, loaderVersion, p)
		if e != nil {
			return e
		}
		v, e := s.resolveVersion(ctx, r.Path, id, p)
		if e != nil {
			return e
		}
		_, natives, e := s.prepareVersion(ctx, r.Path, v, p)
		if natives != "" {
			_ = os.RemoveAll(natives)
		}
		if e != nil {
			return e
		}
		return s.Select(r.ID, "", id)
	})
}
func (s *Service) createEmptyRoot(parent, name string) (Root, error) {
	if strings.TrimSpace(name) == "" || filepath.Base(name) != name || strings.ContainsAny(name, "/\\:") {
		return Root{}, errors.New("请输入有效的文件夹名称")
	}
	parent, e := filepath.Abs(parent)
	if e != nil {
		return Root{}, e
	}
	parent, e = filepath.EvalSymlinks(parent)
	if e != nil {
		return Root{}, e
	}
	dest, e := safeJoin(parent, name)
	if e != nil {
		return Root{}, e
	}
	if _, e = os.Stat(dest); !os.IsNotExist(e) {
		return Root{}, errors.New("目标文件夹已存在，请选择新名称")
	}
	if e = os.Mkdir(dest, 0755); e != nil {
		return Root{}, e
	}
	if e = os.Mkdir(filepath.Join(dest, ".minecraft"), 0755); e != nil {
		return Root{}, e
	}
	return s.AddRoot(dest, name)
}
