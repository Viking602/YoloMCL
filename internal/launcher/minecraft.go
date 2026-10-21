package launcher

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const manifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

// MinecraftVersion is an entry in Mojang's complete Java Edition catalogue.
// Keep the upstream type so historical and newly introduced categories remain
// available to the installer without maintaining a local version allowlist.
type MinecraftVersion struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	SHA1        string `json:"sha1"`
	ReleaseTime string `json:"releaseTime"`
}

type gameManifest struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []MinecraftVersion `json:"versions"`
}
type artifact struct {
	ID   string `json:"id,omitempty"`
	Path string `json:"path"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}
type rule struct {
	Action string `json:"action"`
	OS     struct {
		Name    string `json:"name"`
		Arch    string `json:"arch"`
		Version string `json:"version"`
	} `json:"os"`
	Features map[string]bool `json:"features"`
}
type library struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Downloads struct {
		Artifact    *artifact           `json:"artifact"`
		Classifiers map[string]artifact `json:"classifiers"`
	} `json:"downloads"`
	Natives map[string]string `json:"natives"`
	Rules   []rule            `json:"rules"`
	Extract struct {
		Exclude []string `json:"exclude"`
	} `json:"extract"`
	ClientReq *bool `json:"clientreq"`
}
type gameVersion struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	InheritsFrom       string `json:"inheritsFrom"`
	MainClass          string `json:"mainClass"`
	MinecraftArguments string `json:"minecraftArguments"`
	Jar                string `json:"jar"`
	Arguments          struct {
		JVM  []json.RawMessage `json:"jvm"`
		Game []json.RawMessage `json:"game"`
	} `json:"arguments"`
	Libraries  []library           `json:"libraries"`
	Downloads  map[string]artifact `json:"downloads"`
	Assets     string              `json:"assets"`
	AssetIndex struct {
		ID   string `json:"id"`
		URL  string `json:"url"`
		SHA1 string `json:"sha1"`
		Size int64  `json:"size"`
	} `json:"assetIndex"`
	JavaVersion struct {
		MajorVersion int `json:"majorVersion"`
	} `json:"javaVersion"`
	Logging struct {
		Client struct {
			Argument string   `json:"argument"`
			File     artifact `json:"file"`
		} `json:"client"`
	} `json:"logging"`
}
type runningGame struct {
	cmd     *exec.Cmd
	log     *gameLog
	started time.Time
}
type gameLog struct {
	mu      sync.Mutex
	lines   []string
	secret  string
	pending string
}

func (l *gameLog) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	text := l.pending + string(b)
	parts := strings.Split(text, "\n")
	l.pending = parts[len(parts)-1]
	if len(l.pending) > 16384 {
		l.pending = l.pending[len(l.pending)-16384:]
	}
	for _, v := range parts[:len(parts)-1] {
		if l.secret != "" {
			v = strings.ReplaceAll(v, l.secret, "[REDACTED]")
		}
		l.lines = append(l.lines, strings.TrimRight(v, "\r"))
	}
	if len(l.lines) > 1200 {
		l.lines = l.lines[len(l.lines)-1200:]
	}
	return len(b), nil
}
func (l *gameLog) read() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.lines...)
}
func osName() string {
	switch runtime.GOOS {
	case "darwin":
		return "osx"
	case "windows":
		return "windows"
	}
	return "linux"
}
func rulesAllow(rules []rule) bool {
	if len(rules) == 0 {
		return true
	}
	allow := false
	for _, r := range rules {
		if r.OS.Name != "" && r.OS.Name != osName() {
			continue
		}
		if r.OS.Arch != "" {
			arch := runtime.GOARCH
			ok := r.OS.Arch == arch || r.OS.Arch == "x86_64" && arch == "amd64" || r.OS.Arch == "x86" && arch == "386" || r.OS.Arch == "aarch64" && arch == "arm64"
			if !ok {
				continue
			}
		}
		if r.OS.Version != "" {
			ok, _ := regexp.MatchString(r.OS.Version, "10.0")
			if !ok {
				continue
			}
		}
		match := true
		for _, expected := range r.Features {
			if expected {
				match = false
				break
			}
		}
		if match {
			allow = r.Action == "allow"
		}
	}
	return allow
}
func mavenPath(name string) (string, error) {
	parts := strings.Split(name, ":")
	if len(parts) < 3 || len(parts) > 4 {
		return "", errors.New("无效 Maven 坐标")
	}
	extension := "jar"
	if x := strings.SplitN(parts[len(parts)-1], "@", 2); len(x) == 2 {
		parts[len(parts)-1] = x[0]
		extension = x[1]
	}
	filename := parts[1] + "-" + parts[2]
	if len(parts) == 4 {
		filename += "-" + parts[3]
	}
	return strings.ReplaceAll(parts[0], ".", "/") + "/" + parts[1] + "/" + parts[2] + "/" + filename + "." + extension, nil
}
func libraryArtifact(l library) (artifact, error) {
	if l.Downloads.Artifact != nil {
		return *l.Downloads.Artifact, nil
	}
	path, e := mavenPath(l.Name)
	if e != nil {
		return artifact{}, e
	}
	base := l.URL
	if base == "" {
		base = "https://libraries.minecraft.net/"
	}
	return artifact{Path: path, URL: strings.TrimRight(base, "/") + "/" + path}, nil
}
func (s *Service) Manifest(ctx context.Context) (gameManifest, error) {
	var m gameManifest
	e := s.getJSON(ctx, manifestURL, &m)
	return m, e
}
func (s *Service) installVanilla(ctx context.Context, root, id string, p progress) (gameVersion, error) {
	manifest, e := s.Manifest(ctx)
	if e != nil {
		return gameVersion{}, e
	}
	for _, v := range manifest.Versions {
		if v.ID != id {
			continue
		}
		dest, e := safeJoin(root, "versions/"+id+"/"+id+".json")
		if e != nil {
			return gameVersion{}, e
		}
		p("获取 Minecraft "+id+" 元数据", 0, 0)
		if e = s.download(ctx, DownloadFile{Name: id + ".json", URL: v.URL, SHA1: v.SHA1}, dest); e != nil {
			return gameVersion{}, e
		}
		b, e := os.ReadFile(dest)
		if e != nil {
			return gameVersion{}, e
		}
		var version gameVersion
		e = json.Unmarshal(b, &version)
		return version, e
	}
	return gameVersion{}, errors.New("官方版本列表中找不到此 Minecraft 版本")
}
func readVersion(root, id string) (gameVersion, error) {
	p, e := findVersionJSON(root, id)
	if e != nil {
		return gameVersion{}, e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return gameVersion{}, e
	}
	var v gameVersion
	e = json.Unmarshal(b, &v)
	if e == nil && v.ID == "" {
		v.ID = id
	}
	return v, e
}
func mergeVersion(base, child gameVersion) gameVersion {
	base.ID = child.ID
	if child.MainClass != "" {
		base.MainClass = child.MainClass
	}
	if child.Type != "" {
		base.Type = child.Type
	}
	if child.MinecraftArguments != "" {
		base.MinecraftArguments = child.MinecraftArguments
	}
	if child.JavaVersion.MajorVersion > 0 {
		base.JavaVersion = child.JavaVersion
	}
	if child.Jar != "" {
		base.Jar = child.Jar
	}
	base.Arguments.Game = append(base.Arguments.Game, child.Arguments.Game...)
	base.Arguments.JVM = append(base.Arguments.JVM, child.Arguments.JVM...)
	// Child libraries replace the parent's artifact with the same group/artifact/classifier.
	key := func(name string) string {
		p := strings.Split(name, ":")
		if len(p) >= 3 {
			v := p[0] + ":" + p[1]
			if len(p) > 3 {
				v += ":" + p[3]
			}
			return v
		}
		return name
	}
	childKeys := map[string]bool{}
	for _, l := range child.Libraries {
		childKeys[key(l.Name)] = true
	}
	libs := []library{}
	for _, l := range base.Libraries {
		if !childKeys[key(l.Name)] {
			libs = append(libs, l)
		}
	}
	base.Libraries = append(libs, child.Libraries...)
	return base
}
func (s *Service) resolveVersion(ctx context.Context, root, id string, p progress) (gameVersion, error) {
	var resolve func(string, int) (gameVersion, error)
	seen := map[string]bool{}
	resolve = func(id string, depth int) (gameVersion, error) {
		if depth > 12 || seen[id] {
			return gameVersion{}, errors.New("版本继承链无效")
		}
		seen[id] = true
		v, e := readVersion(root, id)
		if os.IsNotExist(e) {
			v, e = s.installVanilla(ctx, root, id, p)
		}
		if e != nil {
			return v, e
		}
		if v.InheritsFrom == "" {
			if v.Jar == "" {
				v.Jar = id
			}
			return v, nil
		}
		base, e := resolve(v.InheritsFrom, depth+1)
		if e != nil {
			return v, e
		}
		return mergeVersion(base, v), nil
	}
	return resolve(id, 0)
}
func (s *Service) prepareVersion(ctx context.Context, root string, v gameVersion, p progress) ([]string, string, error) {
	files := []struct {
		file DownloadFile
		path string
	}{}
	cp := []string{}
	natives := []artifact{}
	for _, l := range v.Libraries {
		if !rulesAllow(l.Rules) || l.ClientReq != nil && !*l.ClientReq {
			continue
		}
		a, e := libraryArtifact(l)
		if e != nil {
			return nil, "", e
		}
		if l.Downloads.Artifact == nil && len(l.Downloads.Classifiers) > 0 {
			a = artifact{}
		}
		if a.Path != "" {
			path, e := safeJoin(root, "libraries/"+a.Path)
			if e != nil {
				return nil, "", e
			}
			if a.URL != "" {
				cached := false
				if a.SHA1 == "" {
					if existing, err := zip.OpenReader(path); err == nil {
						existing.Close()
						cached = true
					}
				}
				if !cached {
					files = append(files, struct {
						file DownloadFile
						path string
					}{DownloadFile{Name: l.Name, URL: a.URL, SHA1: a.SHA1, Size: a.Size}, path})
				}
			} else if _, e = os.Stat(path); e != nil {
				return nil, "", fmt.Errorf("加载器未生成所需运行库: %s", l.Name)
			}
			cp = append(cp, path)
		}
		if classifier := l.Natives[osName()]; classifier != "" {
			arch := "64"
			if runtime.GOARCH == "386" {
				arch = "32"
			}
			classifier = strings.ReplaceAll(classifier, "${arch}", arch)
			a, ok := l.Downloads.Classifiers[classifier]
			if !ok {
				return nil, "", fmt.Errorf("缺少本机库 %s / %s", l.Name, classifier)
			}
			path, e := safeJoin(root, "libraries/"+a.Path)
			if e != nil {
				return nil, "", e
			}
			files = append(files, struct {
				file DownloadFile
				path string
			}{DownloadFile{Name: a.Path, URL: a.URL, SHA1: a.SHA1, Size: a.Size}, path})
			natives = append(natives, a)
		}
	}
	jar := v.Jar
	if jar == "" {
		jar = v.ID
	}
	client, e := safeJoin(root, "versions/"+jar+"/"+jar+".jar")
	if e != nil {
		return nil, "", e
	}
	if a, ok := v.Downloads["client"]; ok {
		files = append(files, struct {
			file DownloadFile
			path string
		}{DownloadFile{Name: jar + ".jar", URL: a.URL, SHA1: a.SHA1, Size: a.Size}, client})
	} else if _, e = os.Stat(client); e != nil {
		return nil, "", errors.New("版本缺少 Minecraft 客户端 JAR")
	}
	cp = append(cp, client)
	for i, f := range files {
		p("校验 / 下载 "+f.file.Name, i, len(files))
		if e = s.download(ctx, f.file, f.path); e != nil {
			return nil, "", e
		}
	}
	nativeDir, e := safeJoin(root, ".mcl/natives/"+newID())
	if e != nil {
		return nil, "", e
	}
	if e = os.MkdirAll(nativeDir, 0755); e != nil {
		return nil, "", e
	}
	for _, a := range natives {
		path, e := safeJoin(root, "libraries/"+a.Path)
		if e != nil {
			return nil, "", e
		}
		r, e := zip.OpenReader(path)
		if e != nil {
			return nil, "", e
		}
		for _, f := range r.File {
			if f.FileInfo().IsDir() || strings.HasPrefix(f.Name, "META-INF/") {
				continue
			}
			dest, e := safeJoin(nativeDir, f.Name)
			if e != nil {
				r.Close()
				return nil, "", e
			}
			if _, e = os.Stat(dest); e == nil {
				continue
			}
			if e = extractZipEntry(f, dest); e != nil {
				r.Close()
				return nil, "", e
			}
		}
		r.Close()
	}
	if v.AssetIndex.URL != "" {
		indexPath, e := safeJoin(root, "assets/indexes/"+v.AssetIndex.ID+".json")
		if e != nil {
			return nil, "", e
		}
		if e = s.download(ctx, DownloadFile{Name: "资源索引", URL: v.AssetIndex.URL, SHA1: v.AssetIndex.SHA1, Size: v.AssetIndex.Size}, indexPath); e != nil {
			return nil, "", e
		}
		b, e := os.ReadFile(indexPath)
		if e != nil {
			return nil, "", e
		}
		var index struct {
			Objects map[string]struct {
				Hash string `json:"hash"`
				Size int64  `json:"size"`
			} `json:"objects"`
			Virtual        bool `json:"virtual"`
			MapToResources bool `json:"map_to_resources"`
		}
		if e = json.Unmarshal(b, &index); e != nil {
			return nil, "", e
		}
		keys := []string{}
		for k := range index.Objects {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// Bounded workers keep large asset indexes responsive without spawning one goroutine per file.
		assetCtx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		var firstErr error
		var mu sync.Mutex
		done := 0
		work := make(chan string)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for name := range work {
					a := index.Objects[name]
					if len(a.Hash) != 40 {
						mu.Lock()
						if firstErr == nil {
							firstErr = errors.New("资源哈希无效")
						}
						mu.Unlock()
						cancel()
						continue
					}
					path, e := safeJoin(root, "assets/objects/"+a.Hash[:2]+"/"+a.Hash)
					if e == nil {
						e = s.download(assetCtx, DownloadFile{Name: name, URL: "https://resources.download.minecraft.net/" + a.Hash[:2] + "/" + a.Hash, SHA1: a.Hash, Size: a.Size}, path)
					}
					if e == nil && (index.Virtual || index.MapToResources) {
						prefix := "assets/virtual/" + v.AssetIndex.ID
						if index.MapToResources {
							prefix = "resources"
						}
						dest, err := safeJoin(root, prefix+"/"+name)
						e = err
						if e == nil && !checkHashes(dest, a.Hash, "") {
							if _, err = os.Stat(dest); err == nil {
								e = os.Remove(dest)
							}
							if e == nil {
								e = copyFile(path, dest)
							}
						}
					}
					mu.Lock()
					if e != nil && firstErr == nil {
						firstErr = e
						cancel()
					}
					done++
					p(fmt.Sprintf("游戏资源 (%d/%d): %s", done, len(keys), name), done, len(keys))
					mu.Unlock()
				}
			}()
		}
	assetLoop:
		for _, name := range keys {
			select {
			case work <- name:
			case <-assetCtx.Done():
				break assetLoop
			}
		}
		close(work)
		wg.Wait()
		cancel()
		if firstErr != nil {
			return nil, "", firstErr
		}
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
	}
	if f := v.Logging.Client.File; f.URL != "" {
		dest, e := safeJoin(root, "assets/log_configs/client.xml")
		if e != nil {
			return nil, "", e
		}
		if e = s.download(ctx, DownloadFile{Name: "日志配置", URL: f.URL, SHA1: f.SHA1, Size: f.Size}, dest); e != nil {
			return nil, "", e
		}
	}
	return cp, nativeDir, nil
}
func expandArguments(args []json.RawMessage, vars map[string]string) ([]string, error) {
	out := []string{}
	expand := func(v string) (string, error) {
		for key, value := range vars {
			v = strings.ReplaceAll(v, "${"+key+"}", value)
		}
		if strings.Contains(v, "${") {
			return "", fmt.Errorf("不支持的启动参数: %s", v)
		}
		return v, nil
	}
	for _, raw := range args {
		var simple string
		if json.Unmarshal(raw, &simple) == nil {
			v, e := expand(simple)
			if e != nil {
				return nil, e
			}
			out = append(out, v)
			continue
		}
		var conditional struct {
			Rules []rule          `json:"rules"`
			Value json.RawMessage `json:"value"`
		}
		if e := json.Unmarshal(raw, &conditional); e != nil {
			return nil, e
		}
		if !rulesAllow(conditional.Rules) {
			continue
		}
		var values []string
		if json.Unmarshal(conditional.Value, &simple) == nil {
			values = []string{simple}
		} else if e := json.Unmarshal(conditional.Value, &values); e != nil {
			return nil, e
		}
		for _, value := range values {
			v, e := expand(value)
			if e != nil {
				return nil, e
			}
			out = append(out, v)
		}
	}
	return out, nil
}
func (s *Service) Launch(rootID, accountID string) (string, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return "", e
	}
	a, e := s.getAccount(accountID)
	if e != nil {
		return "", e
	}
	if r.SelectedVersion == "" {
		return "", errors.New("请先选择游戏版本")
	}
	return s.startJob("启动 "+r.Name, rootID, func(ctx context.Context, p progress) error {
		if pack := readPack(r.Path); pack != nil && len(pack.Manual) > 0 {
			return errors.New("整合包还有手动下载文件，请完成后在整合包页面重新检查")
		}
		v, e := s.resolveVersion(ctx, r.Path, r.SelectedVersion, p)
		if e != nil {
			return e
		}
		major := v.JavaVersion.MajorVersion
		if major == 0 {
			major = 8
		}
		java, e := s.ensureJava(ctx, r, major, p)
		if e != nil {
			return e
		}
		cp, natives, e := s.prepareVersion(ctx, r.Path, v, p)
		if e != nil {
			return e
		}
		p("刷新登录状态", 0, 0)
		t, e := s.accountToken(ctx, a)
		if e != nil {
			return e
		}
		settings := s.settings()
		_, memory, customJVM, e := r.ResolveRuntime(r.SelectedVersion, settings)
		if e == nil && memory <= 0 {
			memory = s.AutoMemoryFor(r, r.SelectedVersion, java)
		}
		if e != nil {
			return e
		}
		gameDir := r.VersionDir()
		_ = os.MkdirAll(gameDir, 0755)
		classpath := strings.Join(cp, string(os.PathListSeparator))
		vars := map[string]string{"auth_player_name": a.Name, "version_name": v.ID, "game_directory": gameDir, "assets_root": filepath.Join(r.Path, "assets"), "assets_index_name": v.AssetIndex.ID, "auth_uuid": a.UUID, "auth_access_token": t.Access, "user_type": "msa", "version_type": v.Type, "natives_directory": natives, "launcher_name": "MCL", "launcher_version": appVersion, "classpath": classpath, "classpath_separator": string(os.PathListSeparator), "library_directory": filepath.Join(r.Path, "libraries"), "user_properties": "{}", "auth_session": "token:" + t.Access + ":" + a.UUID, "game_assets": filepath.Join(r.Path, "assets", "virtual", v.AssetIndex.ID), "resolution_width": "1280", "resolution_height": "720", "clientid": a.ClientID, "auth_xuid": ""}
		if a.Kind == "yggdrasil" {
			vars["user_type"] = "mojang"
		}
		if a.Kind == "offline" {
			vars["user_type"] = "legacy"
		}
		jvm, e := expandArguments(v.Arguments.JVM, vars)
		if e != nil {
			return e
		}
		if len(jvm) == 0 {
			jvm = []string{"-Djava.library.path=" + natives, "-cp", classpath}
		}
		args := []string{fmt.Sprintf("-Xmx%dM", memory), "-Dminecraft.launcher.brand=MCL", "-Dminecraft.launcher.version=" + appVersion}
		args = append(args, customJVM...)
		args = append(args, jvm...)
		if v.Logging.Client.Argument != "" {
			path, e := safeJoin(r.Path, "assets/log_configs/client.xml")
			if e != nil {
				return e
			}
			args = append(args, strings.ReplaceAll(v.Logging.Client.Argument, "${path}", path))
		}
		if a.Kind == "yggdrasil" {
			injector, e := s.ensureInjector(ctx, p)
			if e != nil {
				return e
			}
			args = append(args, "-javaagent:"+injector+"="+a.Server)
		}
		if v.MainClass == "" {
			return errors.New("版本元数据缺少主类")
		}
		args = append(args, v.MainClass)
		game, e := expandArguments(v.Arguments.Game, vars)
		if e != nil {
			return e
		}
		if len(game) == 0 && v.MinecraftArguments != "" {
			for _, value := range strings.Fields(v.MinecraftArguments) {
				b, _ := json.Marshal(value)
				expanded, e := expandArguments([]json.RawMessage{b}, vars)
				if e != nil {
					return e
				}
				game = append(game, expanded...)
			}
		}
		args = append(args, game...)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		cmd := exec.Command(java, args...)
		cmd.Dir = gameDir
		hideWindow(cmd)
		secret := t.Access
		if a.Kind == "offline" {
			secret = ""
		}
		logs := &gameLog{lines: []string{}, secret: secret}
		cmd.Stdout = logs
		cmd.Stderr = logs
		if e = cmd.Start(); e != nil {
			return fmt.Errorf("启动 Java 失败: %w", e)
		}
		s.mu.Lock()
		s.running[r.ID] = &runningGame{cmd: cmd, log: logs, started: time.Now()}
		s.mu.Unlock()
		go func() {
			e := cmd.Wait()
			message := "游戏已退出"
			if e != nil {
				message = "游戏退出: " + e.Error()
			}
			_, _ = logs.Write([]byte(message + "\n"))
			_ = os.RemoveAll(natives)
			s.mu.Lock()
			delete(s.running, r.ID)
			s.mu.Unlock()
			_ = atomicJSON(filepath.Join(r.Path, ".mcl", "last-log.json"), logs.read())
		}()
		p("游戏进程已启动", 1, 1)
		return nil
	})
}
func (s *Service) Stop(rootID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	game := s.running[rootID]
	if game == nil {
		return errors.New("游戏未运行")
	}
	return game.cmd.Process.Kill()
}
func (s *Service) Logs(rootID string) ([]string, error) {
	s.mu.Lock()
	g := s.running[rootID]
	s.mu.Unlock()
	if g != nil {
		return g.log.read(), nil
	}
	r, e := s.getRoot(rootID)
	if e != nil {
		return nil, e
	}
	var lines []string
	b, e := os.ReadFile(filepath.Join(r.Path, ".mcl", "last-log.json"))
	if os.IsNotExist(e) {
		return []string{}, nil
	}
	if e != nil {
		return nil, e
	}
	e = json.Unmarshal(b, &lines)
	return lines, e
}
