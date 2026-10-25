package launcher

import "strings"

// Job titles are created in Chinese for logs; the UI renders them from a
// stable key plus argument at display time so they follow the current
// language (and re-render on a live language switch).
var jobTitleKeys = []struct{ prefix, key string }{
	{"下载整合包：", "pack.download"},
	{"导入整合包：", "pack.download"},
	{"导出整合包：", "pack.export"},
	{"更新整合包：", "pack.update"},
	{"回滚整合包：", "pack.rollback"},
	{"安装 Minecraft ", "game.install"},
	{"下载 Java ", "java.download"},
	{"启动 ", "game.launch"},
	{"还原存档 ", "world.restore"},
	{"备份存档 ", "world.backup"},
	{"备份全部存档", "world.backupAll"},
	{"安装模组及前置", "mods.installDeps"},
	{"安装模组及依赖", "mods.installDeps"},
	{"检测模组版本", "mods.detect"},
	{"安装材质包", "addon.resourcepack"},
	{"安装光影包", "addon.shader"},
}

// JobTitleKey splits a job title into its display key and argument; ok is
// false for titles with no key.
func JobTitleKey(title string) (key, arg string, ok bool) {
	for _, k := range jobTitleKeys {
		if a, found := strings.CutPrefix(title, k.prefix); found {
			return k.key, a, true
		}
	}
	return "", title, false
}

// JobTitleKeys lists every key, for translation-coverage tests.
func JobTitleKeys() []string {
	var out []string
	for _, k := range jobTitleKeys {
		out = append(out, k.key)
	}
	return out
}
