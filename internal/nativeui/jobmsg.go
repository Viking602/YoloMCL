package nativeui

import "strings"

// jobMsgPrefixes translates launcher progress text (written in Chinese) at
// display time. Longest prefixes first; the rest of the line (counts, file
// names) is kept as-is.
var jobMsgPrefixes = [][2]string{
	{"下载整合包文件 ", "Downloading pack files "},
	{"下载新版本文件 ", "Downloading new files "},
	{"解析整合包模组 ", "Resolving pack mods "},
	{"正在通过文件哈希识别 Modrinth 模组", "Identifying Modrinth mods by file hash"},
	{"正在查询 CurseForge 文件指纹", "Querying CurseForge fingerprints"},
	{"更新失败，正在恢复原状", "Update failed, restoring"},
	{"备份将被修改的文件", "Backing up files to be changed"},
	{"获取 authlib-injector", "Fetching authlib-injector"},
	{"检测 CurseForge：", "Checking CurseForge: "},
	{"模组及依赖已安装", "Mods and dependencies installed"},
	{"模组及前置已安装", "Mods and dependencies installed"},
	{"正在获取 Java 信息", "Getting Java info"},
	{"还原前自动备份", "Auto backup before restore"},
	{"游戏进程已启动", "Game process started"},
	{"刷新登录状态", "Refreshing sign-in"},
	{"更新完成：", "Update done: "},
	{"已回滚到 ", "Rolled back to "},
	{"正在回滚 ", "Rolling back "},
	{"应用更新 ", "Applying update "},
	{"游戏资源 ", "Game assets "},
	{"正在校验 Java", "Verifying Java"},
	{"正在解压 Java", "Extracting Java"},
	{"正在解压备份", "Extracting backup"},
	{"正在替换存档", "Replacing world"},
	{"获取 Minecraft ", "Fetching Minecraft "},
	{"校验 / 下载 ", "Verify / download "},
	{"正在下载 ", "Downloading "},
	{"正在识别 ", "Identifying "},
	{"正在检查", "Checking"},
	{"正在准备", "Preparing"},
	{"下载整合包", "Downloading pack"},
	{"备份完成", "Backup done"},
	{"导入完成", "Import done"},
	{"还原完成", "Restore done"},
	{"已完成", "Done"},
	{"已取消", "Cancelled"},
	{"已安装 ", "Installed "},
	{"游戏已退出", "Game exited"},
	{"下载 ", "Download "},
	{"安装 ", "Install "},
	{"启用 ", "Enable "},
	{"项目 #", "project #"},
}

// jobMsg renders a job message or item line in the UI language.
func (a *App) jobMsg(m string) string {
	if !a.isEN() {
		return m
	}
	for _, p := range jobMsgPrefixes {
		if rest, ok := strings.CutPrefix(m, p[0]); ok {
			return p[1] + strings.ReplaceAll(rest, "项目 #", "project #")
		}
	}
	return m
}
