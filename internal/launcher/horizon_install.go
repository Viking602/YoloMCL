package launcher

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	horizonMaxEntryBytes   = 2 << 30
	horizonMaxArchiveBytes = 4 << 30
)

func horizonBinaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func horizonClientBinary(buildDir string) string {
	return filepath.Join(buildDir, horizonBinaryName("horizon-client"))
}

func horizonPrepareBinary(buildDir string) string {
	return filepath.Join(buildDir, horizonBinaryName("horizon-prepare"))
}

func (s *Service) horizonBuildDir(buildID string) (string, error) {
	if !validHorizonBuildID(buildID) {
		return "", horizonErr("Horizon 版本标识无效", "Invalid Horizon build identifier")
	}
	return safeJoin(filepath.Join(s.dir, "horizon", "builds"), buildID)
}

func (s *Service) horizonPreparedDir(api horizonPrepareAPI) (string, error) {
	if api.Minecraft == "" || strings.ContainsAny(api.Minecraft, `/\:`) {
		return "", horizonErr("Horizon 锁定的 Minecraft 版本无效", "Invalid pinned Minecraft version in Horizon build")
	}
	name := fmt.Sprintf("%s-s%d-v%d", api.Minecraft, api.Schema, api.OutputVersion)
	return safeJoin(filepath.Join(s.dir, "horizon", "prepared"), name)
}

func (s *Service) horizonServerJarPath(mc string) (string, error) {
	if mc == "" || strings.ContainsAny(mc, `/\:`) {
		return "", horizonErr("Minecraft 版本无效", "Invalid Minecraft version")
	}
	return safeJoin(filepath.Join(s.dir, "horizon", "server-jars"), mc+".jar")
}

func horizonBuildInstalled(buildDir string) bool {
	for _, p := range []string{
		filepath.Join(buildDir, horizonInstalledMarker),
		horizonClientBinary(buildDir),
		horizonPrepareBinary(buildDir),
		filepath.Join(buildDir, "research-lock.json"),
	} {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return false
		}
	}
	return true
}

func (s *Service) lockHorizonPrep(preparedDir string) func() {
	key := filepath.Clean(preparedDir)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	s.horizonMu.Lock()
	if s.horizonPrepMu == nil {
		s.horizonPrepMu = map[string]*sync.Mutex{}
	}
	m := s.horizonPrepMu[key]
	if m == nil {
		m = &sync.Mutex{}
		s.horizonPrepMu[key] = m
	}
	s.horizonMu.Unlock()
	m.Lock()
	return m.Unlock
}

func (s *Service) ensureHorizonBuild(ctx context.Context, buildID, releaseTag string, p progress) (HorizonRelease, string, error) {
	if buildID != "" {
		if dir, err := s.horizonBuildDir(buildID); err == nil && horizonBuildInstalled(dir) {
			return HorizonRelease{BuildID: buildID, ReleaseTag: releaseTag}, dir, nil
		}
	}
	p("获取 Horizon 版本列表", 0, 0)
	releases, err := s.horizonReleases(ctx, false)
	if err != nil {
		return HorizonRelease{}, "", err
	}
	rel, found := selectHorizonRelease(releases, buildID, releaseTag)
	if !found && (buildID != "" || releaseTag != "") {
		if fresh, ferr := s.horizonReleases(ctx, true); ferr == nil {
			rel, found = selectHorizonRelease(fresh, buildID, releaseTag)
		}
	}
	if !found {
		if buildID != "" {
			return HorizonRelease{}, "", horizonErr(
				fmt.Sprintf("未在 GitHub 发布列表中找到 Horizon 版本 %s（Nightly 构建通常保留 7 天）", buildID),
				fmt.Sprintf("Horizon build %s was not found in published releases (Nightly builds are kept for 7 days)", buildID),
			)
		}
		return HorizonRelease{}, "", horizonErr("未找到当前平台可用的 Horizon 发布版本", "No Horizon release available for this platform")
	}
	targetDir, err := s.horizonBuildDir(rel.BuildID)
	if err != nil {
		return HorizonRelease{}, "", err
	}
	if horizonBuildInstalled(targetDir) {
		return rel, targetDir, nil
	}

	p("校验 Horizon 签名 "+rel.BuildID, 0, 0)
	expectedSHA256, err := s.horizonSignedDigest(ctx, rel)
	if err != nil {
		return HorizonRelease{}, "", err
	}

	archivePath, err := safeJoin(filepath.Join(s.dir, "horizon", "cache"), rel.BuildID+"-"+rel.AssetName)
	if err != nil {
		return HorizonRelease{}, "", err
	}
	p("下载 Horizon "+rel.BuildID, 0, 0)
	if err := s.download(ctx, DownloadFile{
		Name:   rel.AssetName,
		URL:    rel.AssetURL,
		SHA256: expectedSHA256,
		Size:   rel.AssetSize,
	}, archivePath); err != nil {
		return HorizonRelease{}, "", err
	}

	buildsRoot := filepath.Join(s.dir, "horizon", "builds")
	if err := os.MkdirAll(buildsRoot, 0o755); err != nil {
		return HorizonRelease{}, "", err
	}
	stageDir, err := safeJoin(buildsRoot, "."+rel.BuildID+".partial-"+newID()[:8])
	if err != nil {
		return HorizonRelease{}, "", err
	}
	s.trackCreated(ctx, stageDir)
	defer os.RemoveAll(stageDir)

	unpackedDir := filepath.Join(stageDir, "unpacked")
	p("解压 Horizon "+rel.BuildID, 0, 0)
	if err := extractHorizonArchive(ctx, archivePath, rel.AssetName, unpackedDir); err != nil {
		return HorizonRelease{}, "", err
	}

	topDir, err := unwrapSingleTopDir(unpackedDir)
	if err != nil {
		return HorizonRelease{}, "", err
	}
	if runtime.GOOS != "windows" {
		for _, bin := range []string{"horizon-client", "horizon-prepare", "horizon-ctl", "horizon-server"} {
			p := filepath.Join(topDir, bin)
			if st, statErr := os.Stat(p); statErr == nil && !st.IsDir() {
				_ = os.Chmod(p, 0o755)
			}
		}
	}
	if runtime.GOOS == "darwin" {
		clearMacQuarantine(ctx, topDir)
	}
	for _, required := range []string{
		horizonClientBinary(topDir),
		horizonPrepareBinary(topDir),
		filepath.Join(topDir, "research-lock.json"),
	} {
		st, statErr := os.Stat(required)
		if statErr != nil || st.IsDir() {
			return HorizonRelease{}, "", horizonErr(
				fmt.Sprintf("Horizon 安装包缺少必需文件: %s", filepath.Base(required)),
				fmt.Sprintf("Horizon archive is missing required file: %s", filepath.Base(required)),
			)
		}
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return HorizonRelease{}, "", err
	}
	if err := os.Rename(topDir, targetDir); err != nil {
		return HorizonRelease{}, "", err
	}
	if err := os.WriteFile(filepath.Join(targetDir, horizonInstalledMarker), nil, 0o644); err != nil {
		return HorizonRelease{}, "", err
	}
	_ = os.Remove(archivePath)
	return rel, targetDir, nil
}

func selectHorizonRelease(releases []HorizonRelease, buildID, releaseTag string) (HorizonRelease, bool) {
	if len(releases) == 0 {
		return HorizonRelease{}, false
	}
	if buildID == "" && releaseTag == "" {
		return releases[0], true
	}
	if buildID != "" {
		for _, r := range releases {
			if r.BuildID == buildID || r.ReleaseTag == buildID {
				return r, true
			}
		}
		return HorizonRelease{}, false
	}
	for _, r := range releases {
		if r.ReleaseTag == releaseTag {
			return r, true
		}
	}
	return HorizonRelease{}, false
}

func unwrapSingleTopDir(unpacked string) (string, error) {
	entries, err := os.ReadDir(unpacked)
	if err != nil {
		return "", err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(unpacked, entries[0].Name()), nil
	}
	return unpacked, nil
}

func extractHorizonArchive(ctx context.Context, archivePath, assetName, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	lower := strings.ToLower(assetName)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractHorizonZip(ctx, archivePath, destDir)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractHorizonTarGz(ctx, archivePath, destDir)
	default:
		return horizonErr("不支持的 Horizon 压缩包格式: "+assetName, "Unsupported Horizon archive format: "+assetName)
	}
}

func extractHorizonZip(ctx context.Context, archivePath, destDir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()
	var total int64
	for _, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := strings.TrimPrefix(strings.ReplaceAll(f.Name, "\\", "/"), "./")
		name = strings.TrimSuffix(name, "/")
		if name == "" || name == "." {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return errors.New("压缩包包含符号链接")
		}
		if f.UncompressedSize64 > horizonMaxEntryBytes {
			return errors.New("压缩包单文件过大")
		}
		dest, err := safeJoin(destDir, name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		total += int64(f.UncompressedSize64)
		if total > horizonMaxArchiveBytes {
			return errors.New("压缩包解压体积超过限制")
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if runtime.GOOS != "windows" && f.Mode()&0o111 != 0 {
			mode = 0o755
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			r.Close()
			return err
		}
		n, copyErr := io.Copy(w, io.LimitReader(r, int64(f.UncompressedSize64)+1))
		closeErr := w.Close()
		r.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if n != int64(f.UncompressedSize64) {
			return fmt.Errorf("解压文件长度不匹配: %s", f.Name)
		}
	}
	return nil
}

func extractHorizonTarGz(ctx context.Context, archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(strings.ReplaceAll(hdr.Name, "\\", "/"), "./")
		name = strings.TrimSuffix(name, "/")
		if name == "" || name == "." {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			dest, err := safeJoin(destDir, name)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if hdr.Size < 0 || hdr.Size > horizonMaxEntryBytes {
				return errors.New("压缩包单文件过大")
			}
			total += hdr.Size
			if total > horizonMaxArchiveBytes {
				return errors.New("压缩包解压体积超过限制")
			}
			dest, err := safeJoin(destDir, name)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if runtime.GOOS != "windows" && hdr.Mode&0o111 != 0 {
				mode = 0o755
			}
			w, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			n, copyErr := io.Copy(w, io.LimitReader(tr, hdr.Size+1))
			closeErr := w.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if n != hdr.Size {
				return fmt.Errorf("解压文件长度不匹配: %s", hdr.Name)
			}
		case tar.TypeSymlink, tar.TypeLink:
			return errors.New("压缩包包含符号链接")
		case tar.TypeXHeader, tar.TypeXGlobalHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
			continue
		}
	}
	return nil
}

func clearMacQuarantine(ctx context.Context, dir string) {
	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(qctx, "xattr", "-dr", "com.apple.quarantine", dir)
	_ = cmd.Run()
}

func queryHorizonPrepareAPI(ctx context.Context, buildDir string) (horizonPrepareAPI, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(qctx, horizonPrepareBinary(buildDir), "--launch-api", "--root", buildDir)
	cmd.Dir = buildDir
	hideConsole(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return horizonPrepareAPI{}, horizonErr(
			"查询 horizon-prepare Launch API 失败: "+msg,
			"Failed to query horizon-prepare Launch API: "+msg,
		)
	}
	var api horizonPrepareAPI
	if err := json.Unmarshal(stdout.Bytes(), &api); err != nil {
		return horizonPrepareAPI{}, horizonErr("解析 horizon-prepare Launch API 失败", "Failed to parse horizon-prepare Launch API")
	}
	if !compatibleHorizonLaunchAPI(api.LaunchAPI) {
		return horizonPrepareAPI{}, horizonErr(
			fmt.Sprintf("不支持的 Horizon Launch API 版本: %s", api.LaunchAPI),
			fmt.Sprintf("Unsupported Horizon Launch API version: %s", api.LaunchAPI),
		)
	}
	if api.Minecraft == "" {
		return horizonPrepareAPI{}, horizonErr("horizon-prepare 未声明目标 Minecraft 版本", "horizon-prepare did not report a target Minecraft version")
	}
	if api.JavaMajor <= 0 {
		api.JavaMajor = horizonDefaultJavaMajor
	}
	return api, nil
}

func queryHorizonClientAPI(ctx context.Context, buildDir string) (horizonClientAPI, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(qctx, horizonClientBinary(buildDir), "--launch-api")
	cmd.Dir = buildDir
	hideConsole(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return horizonClientAPI{}, horizonErr(
			"查询 horizon-client Launch API 失败: "+msg,
			"Failed to query horizon-client Launch API: "+msg,
		)
	}
	var api horizonClientAPI
	if err := json.Unmarshal(stdout.Bytes(), &api); err != nil {
		return horizonClientAPI{}, horizonErr("解析 horizon-client Launch API 失败", "Failed to parse horizon-client Launch API")
	}
	if !compatibleHorizonLaunchAPI(api.LaunchAPI) {
		return horizonClientAPI{}, horizonErr(
			fmt.Sprintf("不支持的 Horizon Launch API 版本: %s", api.LaunchAPI),
			fmt.Sprintf("Unsupported Horizon Launch API version: %s", api.LaunchAPI),
		)
	}
	return api, nil
}

func checkHorizonPrepared(ctx context.Context, buildDir, preparedDir string) (bool, string, error) {
	qctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(qctx, horizonPrepareBinary(buildDir), "--check", "--root", buildDir, "--output", preparedDir)
	cmd.Dir = buildDir
	hideConsole(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, "", nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, strings.TrimSpace(stdout.String()), nil
	}
	msg := strings.TrimSpace(stderr.String())
	if msg == "" {
		msg = err.Error()
	}
	return false, "", horizonErr("检查 Horizon 资源失败: "+msg, "Failed to check Horizon resources: "+msg)
}

func (s *Service) ensureHorizonServerJar(ctx context.Context, mc string, v gameVersion, p progress) (string, error) {
	srv, ok := v.Downloads["server"]
	if !ok || srv.URL == "" {
		return "", horizonErr(
			fmt.Sprintf("Minecraft %s 版本元数据缺少服务端 JAR 下载信息", mc),
			fmt.Sprintf("Minecraft %s version metadata has no server JAR download", mc),
		)
	}
	dest, err := s.horizonServerJarPath(mc)
	if err != nil {
		return "", err
	}
	p("校验 / 下载 Minecraft "+mc+" server.jar", 0, 0)
	if err := s.download(ctx, DownloadFile{
		Name: "minecraft-" + mc + "-server.jar",
		URL:  srv.URL,
		SHA1: srv.SHA1,
		Size: srv.Size,
	}, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func envWithJavaHome(base []string, jHome string) []string {
	binDir := filepath.Join(jHome, "bin")
	out := make([]string, 0, len(base)+2)
	pathSet := false
	for _, kv := range base {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(k, "JAVA_HOME") {
			continue
		}
		if strings.EqualFold(k, "PATH") {
			pathSet = true
			out = append(out, k+"="+binDir+string(os.PathListSeparator)+v)
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "JAVA_HOME="+jHome)
	if !pathSet {
		out = append(out, "PATH="+binDir)
	}
	return out
}

func (s *Service) prepareHorizonResources(ctx context.Context, r Root, buildDir string, prepAPI horizonPrepareAPI, preparedDir string, p progress) error {
	unlock := s.lockHorizonPrep(preparedDir)
	defer unlock()

	p("检查 Horizon 资源", 0, 0)
	if ready, _, err := checkHorizonPrepared(ctx, buildDir, preparedDir); err == nil && ready {
		return nil
	}

	mc := prepAPI.Minecraft
	mcVer, err := readVersion(r.Path, mc)
	if os.IsNotExist(err) {
		mcVer, err = s.installVanilla(ctx, r.Path, mc, p)
	}
	if err != nil {
		return err
	}
	_, natives, err := s.prepareVersion(ctx, r.Path, mcVer, p)
	if natives != "" {
		_ = os.RemoveAll(natives)
	}
	if err != nil {
		return err
	}

	serverJar, err := s.ensureHorizonServerJar(ctx, mc, mcVer, p)
	if err != nil {
		return err
	}

	javaExe, err := s.ensureJava(ctx, r, prepAPI.JavaMajor, p)
	if err != nil {
		return err
	}
	jHome := javaHome(javaExe)
	javaBin := "java"
	if runtime.GOOS == "windows" {
		javaBin = "java.exe"
	}
	if _, statErr := os.Stat(filepath.Join(jHome, "bin", javaBin)); statErr != nil {
		return horizonErr(
			fmt.Sprintf("所选 Java %d 目录中未找到 %s", prepAPI.JavaMajor, javaBin),
			fmt.Sprintf("Could not find %s inside the selected Java %d directory", javaBin, prepAPI.JavaMajor),
		)
	}

	if err := os.MkdirAll(filepath.Dir(preparedDir), 0o755); err != nil {
		return err
	}
	versionJSON, err := safeJoin(r.Path, "versions/"+mc+"/"+mc+".json")
	if err != nil {
		return err
	}
	clientJar, err := safeJoin(r.Path, "versions/"+mc+"/"+mc+".jar")
	if err != nil {
		return err
	}
	libsDir, err := safeJoin(r.Path, "libraries")
	if err != nil {
		return err
	}
	assetsDir, err := safeJoin(r.Path, "assets")
	if err != nil {
		return err
	}

	args := []string{}
	if prepAPI.hasOption("--root") {
		args = append(args, "--root", buildDir)
	}
	if prepAPI.hasOption("--prepare") {
		args = append(args, "--prepare")
	}
	if prepAPI.hasOption("--server-jar") {
		args = append(args, "--server-jar", serverJar)
	}
	if prepAPI.hasOption("--minecraft-dir") {
		args = append(args, "--minecraft-dir", r.Path)
	}
	if prepAPI.hasOption("--version-json") {
		args = append(args, "--version-json", versionJSON)
	}
	if prepAPI.hasOption("--client-jar") {
		args = append(args, "--client-jar", clientJar)
	}
	if prepAPI.hasOption("--libraries-dir") {
		args = append(args, "--libraries-dir", libsDir)
	}
	if prepAPI.hasOption("--assets-dir") {
		args = append(args, "--assets-dir", assetsDir)
	}
	if prepAPI.hasOption("--java-home") {
		args = append(args, "--java-home", jHome)
	}
	if prepAPI.hasOption("--output") {
		args = append(args, "--output", preparedDir)
	}
	if prepAPI.hasOption("--complete") {
		args = append(args, "--complete")
	}
	useProgressJSON := prepAPI.hasOption("--progress")
	if useProgressJSON {
		args = append(args, "--progress", "json")
	}

	p("准备 Horizon 资源", 0, 100)
	cmd := exec.Command(horizonPrepareBinary(buildDir), args...)
	cmd.Dir = buildDir
	cmd.Env = envWithJavaHome(os.Environ(), jHome)
	hideConsole(cmd)
	prepareTree(cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrLogs := &gameLog{lines: []string{}}
	cmd.Stderr = stderrLogs

	if err := cmd.Start(); err != nil {
		return horizonErr("启动 horizon-prepare 失败: "+err.Error(), "Failed to start horizon-prepare: "+err.Error())
	}
	tree, _ := attachTree(cmd)
	doneCh := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if tree != nil {
				_ = tree.Kill()
			} else if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		case <-doneCh:
		}
	}()

	var logLines []string
	var reportedErr string
	lastMsg := "准备 Horizon 资源"
	lastPct := 0
	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if useProgressJSON {
			var ev struct {
				Message  *string  `json:"message"`
				Fraction *float64 `json:"fraction"`
				Error    *string  `json:"error"`
			}
			if json.Unmarshal([]byte(line), &ev) == nil {
				if ev.Message != nil && *ev.Message != "" {
					logLines = append(logLines, *ev.Message)
					lastMsg = "准备 Horizon 资源: " + *ev.Message
					p(lastMsg, lastPct, 100)
				}
				if ev.Fraction != nil {
					pct := int(math.Round(*ev.Fraction * 100))
					pct = max(0, min(100, pct))
					if pct > lastPct {
						lastPct = pct
					}
					p(lastMsg, lastPct, 100)
				}
				if ev.Error != nil && *ev.Error != "" {
					reportedErr = *ev.Error
					logLines = append(logLines, "error: "+*ev.Error)
				}
				continue
			}
		}
		logLines = append(logLines, line)
	}
	waitErr := cmd.Wait()
	close(doneCh)
	if tree != nil {
		tree.Close()
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if waitErr != nil {
		allLogs := append(logLines, stderrLogs.read()...)
		_ = atomicJSON(filepath.Join(r.Path, ".mcl", "horizon-prepare-log.json"), allLogs)
		if reportedErr != "" {
			return horizonErr(
				"准备 Horizon 资源失败: "+reportedErr,
				"Failed to prepare Horizon resources: "+reportedErr,
			)
		}
		return horizonErr(
			"准备 Horizon 资源失败，详情见 .mcl/horizon-prepare-log.json",
			"Failed to prepare Horizon resources; see .mcl/horizon-prepare-log.json for details",
		)
	}
	if !prepAPI.hasOption("--complete") {
		ready, gaps, err := checkHorizonPrepared(ctx, buildDir, preparedDir)
		if err != nil {
			return err
		}
		if !ready {
			return horizonErr(
				"Horizon 资源准备不完整: "+gaps,
				"Horizon resources are incomplete: "+gaps,
			)
		}
	}
	p("准备 Horizon 资源", 100, 100)
	return nil
}

func (s *Service) installHorizon(ctx context.Context, r Root, mc, requestedVersion string, p progress) (string, error) {
	rel, buildDir, err := s.ensureHorizonBuild(ctx, requestedVersion, "", p)
	if err != nil {
		return "", err
	}
	prepAPI, err := queryHorizonPrepareAPI(ctx, buildDir)
	if err != nil {
		return "", err
	}
	if mc != "" && prepAPI.Minecraft != mc {
		return "", horizonErr(
			fmt.Sprintf("Horizon %s 需要 Minecraft %s，与所选版本 %s 不匹配", rel.BuildID, prepAPI.Minecraft, mc),
			fmt.Sprintf("Horizon %s requires Minecraft %s, which does not match selected version %s", rel.BuildID, prepAPI.Minecraft, mc),
		)
	}
	preparedDir, err := s.horizonPreparedDir(prepAPI)
	if err != nil {
		return "", err
	}
	if err := s.prepareHorizonResources(ctx, r, buildDir, prepAPI, preparedDir, p); err != nil {
		return "", err
	}

	id := "horizon-" + rel.BuildID
	vDir, err := safeJoin(r.Path, "versions/"+id)
	if err != nil {
		return "", err
	}
	// Creating horizon-mods/ marks this version directory as isolated even if the
	// parent .minecraft folder holds shared Java mods.
	if err := os.MkdirAll(filepath.Join(vDir, "horizon-mods"), 0o755); err != nil {
		return "", err
	}
	meta := gameVersion{
		ID:            id,
		ClientVersion: prepAPI.Minecraft,
		Type:          "release",
		Horizon: &horizonVersionMeta{
			BuildID:     rel.BuildID,
			ReleaseTag:  rel.ReleaseTag,
			ReleaseName: rel.Name,
			Minecraft:   prepAPI.Minecraft,
			JavaMajor:   prepAPI.JavaMajor,
			Asset:       rel.AssetName,
		},
	}
	meta.JavaVersion.MajorVersion = prepAPI.JavaMajor
	dest, err := safeJoin(r.Path, "versions/"+id+"/"+id+".json")
	if err != nil {
		return "", err
	}
	if err := atomicJSON(dest, meta); err != nil {
		return "", err
	}
	return id, nil
}
