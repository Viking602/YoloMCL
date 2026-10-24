package nativeui

import (
	"fmt"
	"strings"

	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/nativeui/i18n"
)

const (
	LangZH = i18n.LangZH
	LangEN = i18n.LangEN
)

type Copy = i18n.Copy

var enCopy = &i18n.EN

func (a *App) lang() string {
	if a.languageOverride != "" {
		return a.languageOverride
	}
	if v := launcher.NormalizeLanguage(a.settings.Language); v != "" {
		return v
	}
	if a.systemLanguage != "" {
		return a.systemLanguage
	}
	return LangEN
}

func (a *App) isEN() bool {
	return a.lang() == LangEN
}

func (a *App) tr() *Copy {
	return i18n.For(a.isEN())
}

func (a *App) translateCategory(cat string) string {
	return i18n.TranslateCategory(a.isEN(), cat)
}

func (a *App) setLanguage(lang string) {
	norm := launcher.NormalizeLanguage(lang)
	if a.settings.Language == norm && a.snapshot.Settings.Language == norm && a.languageOverride == "" {
		return
	}
	a.languageOverride = ""
	a.settings.Language = norm
	a.snapshot.Settings.Language = norm
	a.normalizeStateValues()
	a.saveSettingsDraft("")
}

func (a *App) toggleLanguage() {
	if a.isEN() {
		a.setLanguage(LangZH)
	} else {
		a.setLanguage(LangEN)
	}
}

func (a *App) dialogTitle(dialog string) string {
	t := a.tr()
	switch dialog {
	case "javaGet":
		return a.L("下载 Java", "Download Java")
	case "launchAgain":
		return a.L("再开一个", "Run another")
	case "add":
		return t.DialogTitleAdd
	case "root":
		return t.DialogTitleDir
	case "version":
		return t.VersionDialogTitle
	case "removeRoot":
		return t.DialogTitleRemRoot
	case "removeMod":
		return t.DialogTitleRemMod
	case "disableMod":
		return a.L("禁用这个模组？", "Disable this mod?")
	case "account":
		return t.DialogTitleAccount
	case "accountAdd":
		return t.AccAddAccountBtn
	case "import":
		return t.DialogTitleImport
	case "install":
		return t.DialogTitleInstall
	case "project":
		return t.DialogTitleProject
	case "packReport":
		return t.DialogTitlePackAudit
	case "packUpdate":
		if a.packPlanReport {
			return t.DialogTitlePackChanges
		}
		return t.DialogTitlePackUpdate
	case "packDrop":
		return t.DialogTitlePackDrop
	case "logs":
		return t.DialogTitleLogs
	case "java":
		return t.DialogTitleJava
	}
	return dialog
}

func (a *App) loaderDisplay(loader string) string {
	t := a.tr()
	switch loaderKey(loader) {
	case "fabric":
		return t.LoaderFabric
	case "quilt":
		return t.LoaderQuilt
	case "forge":
		return t.LoaderForge
	case "neoforge":
		return t.LoaderNeoForge
	}
	return t.LoaderVanilla
}

func (a *App) jobStateText(state string) string {
	if a.isEN() {
		switch state {
		case "running":
			return "In Progress"
		case "done", "completed":
			return "Completed"
		case "failed":
			return "Failed"
		case "cancelled":
			return "Cancelled"
		case "queued":
			return "Queued"
		}
		return state
	}
	switch state {
	case "running":
		return "进行中"
	case "done", "completed":
		return "已完成"
	case "failed":
		return "失败"
	case "cancelled":
		return "已取消"
	case "queued":
		return "排队中"
	}
	return state
}

func (a *App) packFileStatusText(status string) string {
	if a.isEN() {
		switch status {
		case "original":
			return "Intact"
		case "disabled":
			return "Disabled"
		case "missing":
			return "Missing"
		case "modified":
			return "Modified"
		}
		return status
	}
	switch status {
	case "original":
		return "一致"
	case "disabled":
		return "已停用"
	case "missing":
		return "缺失"
	case "modified":
		return "已修改"
	}
	return status
}

func (a *App) translatePhase(phase string) string {
	if !a.isEN() {
		return phase
	}
	switch phase {
	case "正在检测已安装的 Java":
		return "Detecting installed Java runtimes"
	case "快速扫描环境变量与常见目录":
		return "Checking configured Java runtimes"
	case "正在扫描整合包与启动器运行时":
		return "Scanning Minecraft and launcher runtimes"
	case "扫描已停止":
		return "Scan stopped"
	case "快速扫描已结束，可继续深入扫描":
		return "Quick scan finished; run Deeper Scan for deep search"
	}
	return phase
}

func (a *App) formatModCount(count int) string {
	if a.isEN() {
		return fmt.Sprintf("%d items", count)
	}
	return fmt.Sprintf("%d 项", count)
}

func (a *App) formatResultCount(total int64) string {
	if a.isEN() {
		return fmt.Sprintf("%s results", compact(total))
	}
	return fmt.Sprintf("%s 个结果", compact(total))
}

func (a *App) formatSystemMemory(totalMB, availMB int) string {
	if a.isEN() {
		return fmt.Sprintf("System %.1f GB, available %.1f GB", float64(totalMB)/1024, float64(availMB)/1024)
	}
	return fmt.Sprintf("系统 %.1f GB，可用 %.1f GB", float64(totalMB)/1024, float64(availMB)/1024)
}

func (a *App) formatMemoryRangeErr(maxMB int) string {
	if a.isEN() {
		return fmt.Sprintf("Memory must be an integer between 512 and %d MB", maxMB)
	}
	return fmt.Sprintf("内存请输入 512–%d MB 之间的整数", maxMB)
}

func (a *App) formatJavaScanSummary(count int, elapsedMS int64) string {
	if a.isEN() {
		return fmt.Sprintf("Found %d Java runtimes in %.1fs", count, float64(elapsedMS)/1000)
	}
	return fmt.Sprintf("找到 %d 个 Java，耗时 %.1f 秒", count, float64(elapsedMS)/1000)
}

func (a *App) formatJavaScanProgress(phase string, visited, found int) string {
	p := a.translatePhase(phase)
	if a.isEN() {
		return fmt.Sprintf("%s, checked %d folders, found %d Java", p, visited, found)
	}
	return fmt.Sprintf("%s，已检查 %d 个目录，找到 %d 个 Java", p, visited, found)
}

func (a *App) formatVersionTotal(total int) string {
	if a.isEN() {
		return fmt.Sprintf("%d versions total", total)
	}
	return fmt.Sprintf("共 %d 个版本", total)
}

func (a *App) formatFilteredVersionSummary(detail string, count int) string {
	if a.isEN() {
		return fmt.Sprintf("%s, %d matching versions", detail, count)
	}
	return fmt.Sprintf("%s，当前筛选 %d 个版本", detail, count)
}

func normalizeResourceType(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "mod", "mods", "模组":
		return "mod"
	case "resourcepack", "resource packs", "材质包":
		return "resourcepack"
	case "shader", "shaders", "光影", "光影包":
		return "shader"
	default:
		return "modpack"
	}
}

func normalizeSortKey(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "downloads", "most downloads", "下载最多":
		return "downloads"
	case "updated", "recently updated", "最近更新":
		return "updated"
	default:
		return "relevance"
	}
}

func normalizeAccountTab(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "microsoft", "官方登录", "正版登录":
		return "microsoft"
	case "third", "third-party", "yggdrasil", "第三方登录":
		return "third"
	default:
		return "offline"
	}
}

func normalizeSkinModel(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	if strings.Contains(v, "slim") || strings.Contains(v, "alex") || strings.Contains(v, "纤细") {
		return "slim"
	}
	return "default"
}

func normalizeInstallTarget(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "current" || v == "当前目录" || v == "current instance" {
		return "current"
	}
	return "new"
}

func normalizeVersionKindKey(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "release", "releases", "正式版":
		return "release"
	case "snapshot", "snapshots", "快照 / 预发布":
		return "snapshot"
	case "old_beta", "old beta", "历史 beta":
		return "old_beta"
	case "old_alpha", "old alpha", "历史 alpha":
		return "old_alpha"
	case "other", "其他版本":
		return "other"
	default:
		return "all"
	}
}

func (a *App) normalizeStateValues() {
	t := a.tr()
	switch normalizeResourceType(a.resourceType) {
	case "mod":
		a.resourceType = t.DiscoverTypeMod
	case "resourcepack":
		a.resourceType = t.DiscoverTypeRP
	case "shader":
		a.resourceType = t.DiscoverTypeShader
	default:
		a.resourceType = t.DiscoverTypeModpack
	}
	switch loaderKey(a.filterLoader) {
	case "fabric":
		a.filterLoader = t.LoaderFabric
	case "forge":
		a.filterLoader = t.LoaderForge
	case "neoforge":
		a.filterLoader = t.LoaderNeoForge
	case "quilt":
		a.filterLoader = t.LoaderQuilt
	default:
		a.filterLoader = t.LoaderAll
	}
	switch normalizeSortKey(a.searchSort) {
	case "downloads":
		a.searchSort = t.DiscoverSortDownloads
	case "updated":
		a.searchSort = t.DiscoverSortUpdated
	default:
		a.searchSort = t.DiscoverSortRelevance
	}
	switch normalizeAccountTab(a.accountTab) {
	case "microsoft":
		a.accountTab = t.AccTabMicrosoft
	case "third":
		a.accountTab = t.AccTabThirdParty
	default:
		a.accountTab = t.AccTabOffline
	}
	if normalizeSkinModel(a.model) == "slim" {
		a.model = t.WardrobeModelAlex
	} else {
		a.model = t.WardrobeModelSteve
	}
	if normalizeInstallTarget(a.form.Target) == "current" {
		a.form.Target = t.InstallTargetCurrent
	} else {
		a.form.Target = t.InstallTargetNew
	}
	switch loaderKey(a.form.Loader) {
	case "fabric":
		a.form.Loader = t.LoaderFabric
	case "forge":
		a.form.Loader = t.LoaderForge
	case "neoforge":
		a.form.Loader = t.LoaderNeoForge
	case "quilt":
		a.form.Loader = t.LoaderQuilt
	default:
		a.form.Loader = t.LoaderVanilla
	}
	a.versionKind = a.versionKindLabel(normalizeVersionKindKey(a.versionKind))
	if a.selectedCape == "不使用披风" || a.selectedCape == "No Cape" || a.selectedCape == "" {
		a.selectedCape = t.WardrobeCapeNone
	}
}

func (a *App) versionKindLabel(kind string) string {
	t := a.tr()
	switch normalizeVersionKindKey(kind) {
	case "release":
		return t.InstallKindRelease
	case "snapshot":
		return t.InstallKindSnapshot
	case "old_beta":
		return t.InstallKindBeta
	case "old_alpha":
		return t.InstallKindAlpha
	case "other":
		return t.InstallKindOther
	default:
		return t.InstallKindAll
	}
}

func (a *App) versionKindOptions() []string {
	t := a.tr()
	return []string{t.InstallKindAll, t.InstallKindRelease, t.InstallKindSnapshot, t.InstallKindBeta, t.InstallKindAlpha, t.InstallKindOther}
}

// L picks the Chinese or English string for short UI-only labels.
func (a *App) L(zh, en string) string {
	if a.isEN() {
		return en
	}
	return zh
}
