package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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
	return s.launch(rootID, accountID, false)
}

// LaunchAgain starts one more copy of a pack that is already running.
// Each copy gets its own natives folder, log file and session; the game
// folder (options, config, saves) is shared, as with two launches of the
// official launcher.
func (s *Service) LaunchAgain(rootID, accountID string) (string, error) {
	return s.launch(rootID, accountID, true)
}

func (s *Service) launch(rootID, accountID string, again bool) (string, error) {
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
	lv := r.SelectedVersion
	if again {
		lv = ""
	}
	return s.startJobForMode("启动 "+r.Name, rootID, lv, again, func(ctx context.Context, p progress) error {
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
		vars := map[string]string{"auth_player_name": a.Name, "version_name": v.ID, "game_directory": gameDir, "assets_root": filepath.Join(r.Path, "assets"), "assets_index_name": v.AssetIndex.ID, "auth_uuid": a.UUID, "auth_access_token": t.Access, "user_type": "msa", "version_type": v.Type, "natives_directory": natives, "launcher_name": "YoloMCL", "launcher_version": appVersion, "classpath": classpath, "classpath_separator": string(os.PathListSeparator), "library_directory": filepath.Join(r.Path, "libraries"), "user_properties": "{}", "auth_session": "token:" + t.Access + ":" + a.UUID, "game_assets": filepath.Join(r.Path, "assets", "virtual", v.AssetIndex.ID), "resolution_width": "1280", "resolution_height": "720", "clientid": a.ClientID, "auth_xuid": ""}
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
		args := []string{fmt.Sprintf("-Xmx%dM", memory), "-Dminecraft.launcher.brand=YoloMCL", "-Dminecraft.launcher.version=" + appVersion, "-Dstdout.encoding=UTF-8", "-Dstderr.encoding=UTF-8"}
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
		prepareTree(cmd)
		secret := t.Access
		if a.Kind == "offline" {
			secret = ""
		}
		logs := &gameLog{lines: []string{}, ring: newLineRing(LogRingLines), secret: secret}
		cmd.Stdout = logs
		cmd.Stderr = logs
		if e = cmd.Start(); e != nil {
			return fmt.Errorf("启动 Java 失败: %w", e)
		}
		s.mu.Lock()
		tree, te := attachTree(cmd)
		if te != nil {
			_, _ = logs.Write([]byte("无法跟踪子进程，停止游戏时可能残留: " + te.Error() + "\n"))
		}
		rg := &runningGame{cmd: cmd, tree: tree, log: logs, started: time.Now()}
		key := runKey(r.ID, r.SelectedVersion)
		if s.running[key] != nil {
			key += "#" + newID()[:6] // a second copy of the same pack
		}
		s.running[key] = rg
		gs := s.newSessionLocked(r, logs, rg)
		gs.AccountID, gs.AccountName, gs.AccountKind = a.ID, a.Name, a.Kind
		gs.MemoryMB, gs.GameDir = memory, gameDir
		s.mu.Unlock()
		go func(path string) {
			if info, e := inspectJava(context.Background(), path); e == nil {
				s.mu.Lock()
				gs.Java = info.Version
				s.mu.Unlock()
			}
		}(java)
		if f, path := sessionLogFile(gameDir, gs.Seq); f != nil {
			logs.mu.Lock()
			logs.file = f
			logs.mu.Unlock()
			s.mu.Lock()
			gs.LogPath = path
			s.mu.Unlock()
		}
		go func() {
			e := cmd.Wait()
			rg.waitTreeGone()
			message := "游戏已退出"
			if e != nil {
				message = "游戏退出: " + e.Error()
			}
			_, _ = logs.Write([]byte(message + "\n"))
			s.endSession(gs, e)
			logs.mu.Lock()
			if logs.file != nil {
				_ = logs.file.Close()
				logs.file = nil
			}
			logs.mu.Unlock()
			_ = os.RemoveAll(natives)
			s.mu.Lock()
			if s.running[key] == rg {
				delete(s.running, key)
			}
			s.mu.Unlock()
			_ = atomicJSON(filepath.Join(r.Path, ".mcl", "last-log.json"), logs.read())
		}()
		p("游戏进程已启动", 1, 1)
		return nil
	})
}

func (s *Service) Stop(rootID string) error {
	r, _ := s.getRoot(rootID)
	s.mu.Lock()
	defer s.mu.Unlock()
	game := s.running[runKey(rootID, r.SelectedVersion)]
	if game == nil {
		for k, g := range s.running { // the selected version is not the one running
			if strings.HasPrefix(k, rootID+"|") {
				game = g
			}
		}
	}
	if game == nil {
		return errors.New("游戏未运行")
	}
	for _, x := range s.sessions {
		if x.game == game {
			x.stopped = true
		}
	}
	return game.killTree()
}

func (s *Service) Logs(rootID string) ([]string, error) {
	r0, _ := s.getRoot(rootID)
	s.mu.Lock()
	g := s.running[runKey(rootID, r0.SelectedVersion)]
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
