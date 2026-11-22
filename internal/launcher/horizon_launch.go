package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func horizonClientEnv(base []string, home string) []string {
	strip := map[string]bool{
		"LIBGL_ALWAYS_SOFTWARE":           true,
		"GALLIUM_DRIVER":                  true,
		"WEBKIT_DISABLE_DMABUF_RENDERER":  true,
		"WEBKIT_DISABLE_COMPOSITING_MODE": true,
		"HORIZON_SESSION_FILE":            true,
		"HORIZON_ACCOUNT_SESSIONS":        true,
		"HORIZON_LAUNCHER_EXE":            true,
		"HORIZON_HOME":                    true,
	}
	out := make([]string, 0, len(base)+1)
	for _, kv := range base {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if strip[strings.ToUpper(k)] {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "HORIZON_HOME="+home)
	return out
}

func (s *Service) launchHorizon(ctx context.Context, r Root, a Account, v gameVersion, p progress) error {
	if v.Horizon == nil || v.Horizon.BuildID == "" {
		return horizonErr("Horizon 版本元数据无效", "Invalid Horizon version metadata")
	}
	if a.Kind != "microsoft" {
		return horizonErr(
			"启动 Horizon 需要已拥有 Minecraft: Java 版的 Microsoft 正版账号，请先切换或登录 Microsoft 账号",
			"Launching Horizon requires a Microsoft account that owns Minecraft: Java Edition. Please sign in or switch to a Microsoft account.",
		)
	}

	_, buildDir, err := s.ensureHorizonBuild(ctx, v.Horizon.BuildID, v.Horizon.ReleaseTag, p)
	if err != nil {
		return err
	}
	prepAPI, err := queryHorizonPrepareAPI(ctx, buildDir)
	if err != nil {
		return err
	}
	preparedDir, err := s.horizonPreparedDir(prepAPI)
	if err != nil {
		return err
	}
	p("检查 Horizon 资源", 0, 0)
	ready, _, err := checkHorizonPrepared(ctx, buildDir, preparedDir)
	if err != nil {
		return err
	}
	if !ready {
		if err := s.prepareHorizonResources(ctx, r, buildDir, prepAPI, preparedDir, p); err != nil {
			return err
		}
	}

	clientAPI, err := queryHorizonClientAPI(ctx, buildDir)
	if err != nil {
		return err
	}
	for _, required := range []string{"--resources", "--game-dir", "--home", "--session-stdin"} {
		if !clientAPI.hasOption(required) {
			return horizonErr(
				fmt.Sprintf("此 Horizon 版本缺少必需启动选项 %s，请重新安装最新版本", required),
				fmt.Sprintf("This Horizon build does not support required launch option %s; please install a newer build", required),
			)
		}
	}

	p("刷新登录状态", 0, 0)
	t, err := s.accountToken(ctx, a)
	if err != nil {
		return err
	}
	if t.Access == "" || a.Name == "" || a.UUID == "" {
		return horizonErr("Microsoft 登录会话无效，请重新登录", "Microsoft sign-in session is invalid; please sign in again")
	}
	sessionPayload, err := json.Marshal(struct {
		Name        string `json:"name"`
		UUID        string `json:"uuid"`
		AccessToken string `json:"access_token"`
		UserType    string `json:"user_type"`
	}{
		Name:        a.Name,
		UUID:        a.UUID,
		AccessToken: t.Access,
		UserType:    "msa",
	})
	if err != nil {
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	gameDir := r.VersionDir()
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		return err
	}

	s.mu.Lock()
	canApplyKeys := !s.rootRunningLocked(r.ID)
	s.mu.Unlock()
	if canApplyKeys {
		if err := applyRootKeyBindings(ctx, r, s.globalManagedOptions()); err != nil {
			return err
		}
	}

	args := []string{
		"--resources", preparedDir,
		"--game-dir", gameDir,
		"--home", gameDir,
	}
	if clientAPI.hasOption("--width") && clientAPI.hasOption("--height") {
		args = append(args, "--width", "1280", "--height", "720")
	}
	args = append(args, "--session-stdin")

	p("启动 Horizon", 0, 0)
	cmd := exec.Command(horizonClientBinary(buildDir), args...)
	cmd.Dir = gameDir
	cmd.Env = horizonClientEnv(os.Environ(), gameDir)
	cmd.Stdin = bytes.NewReader(sessionPayload)
	hideConsole(cmd)
	prepareTree(cmd)

	logs := &gameLog{lines: []string{}, ring: newLineRing(LogRingLines), secret: t.Access}
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		return horizonErr("启动 Horizon 失败: "+err.Error(), "Failed to start Horizon: "+err.Error())
	}

	s.mu.Lock()
	tree, te := attachTree(cmd)
	if te != nil {
		_, _ = logs.Write([]byte("无法跟踪子进程，停止游戏时可能残留: " + te.Error() + "\n"))
	}
	rg := &runningGame{cmd: cmd, tree: tree, log: logs, started: time.Now()}
	key := runKey(r.ID, r.SelectedVersion)
	if s.running[key] != nil {
		key += "#" + newID()[:6]
	}
	s.running[key] = rg
	gs := s.newSessionLocked(r, logs, rg)
	gs.AccountID, gs.AccountName, gs.AccountKind = a.ID, a.Name, a.Kind
	gs.Java = "Horizon (Rust)"
	gs.GameDir = gameDir
	s.mu.Unlock()

	if f, path := sessionLogFile(gameDir, gs.Seq); f != nil {
		logs.mu.Lock()
		logs.file = f
		logs.mu.Unlock()
		s.mu.Lock()
		gs.LogPath = path
		s.mu.Unlock()
	}

	go func() {
		waitErr := cmd.Wait()
		rg.waitTreeGone()
		message := "游戏已退出"
		if waitErr != nil {
			message = "游戏退出: " + waitErr.Error()
		}
		_, _ = logs.Write([]byte(message + "\n"))
		logs.mu.Lock()
		if logs.file != nil {
			_ = logs.file.Close()
			logs.file = nil
		}
		logs.mu.Unlock()
		_ = atomicJSON(filepath.Join(r.Path, ".mcl", "last-log.json"), logs.read())
		s.finishSession(key, rg, gs, waitErr)
	}()

	p("游戏进程已启动", 1, 1)
	return nil
}
