package nativeui

import (
	"fmt"
	"strings"

	"github.com/Viking602/YoloMCL/internal/i18n"
	"github.com/Viking602/YoloMCL/internal/launcher"
)

const (
	LangZH = i18n.LangZH
	LangEN = i18n.LangEN
)

type Copy = i18n.Copy

var enCopy = i18n.For(true)

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
	case "launcherUpdate":
		if a.launcherUpdateRequired() {
			return a.L("i18n.launcher_update_required")
		}
		return a.L("i18n.launcher_update")
	case "javaGet":
		return a.L("i18n.download_java")
	case "launchAgain":
		return a.L("i18n.run_another")
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
		return a.L("i18n.disable_this_mod")
	case "account":
		return t.DialogTitleAccount
	case "accountAdd":
		return t.AccAddAccountBtn
	case "import":
		return t.DialogTitleImport
	case "export":
		return a.L("i18n.export_modpack")
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
	key := state
	if key == "done" {
		key = "completed"
	}
	if text, ok := i18n.Lookup(a.isEN(), "job.state."+key); ok {
		return text
	}
	return state
}

func (a *App) packFileStatusText(status string) string {
	if text, ok := i18n.Lookup(a.isEN(), "pack.file."+status); ok {
		return text
	}
	return status
}

func (a *App) translatePhase(phase string) string { return i18n.Phase(a.isEN(), phase) }

func (a *App) formatModCount(count int) string { return fmt.Sprintf(a.L("count.mods"), count) }

func (a *App) formatResultCount(total int64) string {
	return fmt.Sprintf(a.L("count.results"), compact(total))
}

func (a *App) formatSystemMemory(totalMB, availMB int) string {
	return fmt.Sprintf(a.L("memory.system"), float64(totalMB)/1024, float64(availMB)/1024)
}

func (a *App) formatMemoryRangeErr(maxMB int) string {
	return fmt.Sprintf(a.L("memory.rangeError"), maxMB)
}

func (a *App) formatJavaScanSummary(count int, elapsedMS int64) string {
	return fmt.Sprintf(a.L("java.scanSummary"), count, float64(elapsedMS)/1000)
}

func (a *App) formatJavaScanProgress(phase string, visited, found int) string {
	return fmt.Sprintf(a.L("java.scanProgress"), a.translatePhase(phase), visited, found)
}

func (a *App) formatVersionTotal(total int) string { return fmt.Sprintf(a.L("version.total"), total) }

func (a *App) formatFilteredVersionSummary(detail string, count int) string {
	return fmt.Sprintf(a.L("version.filtered"), detail, count)
}

func normalizeResourceType(s string) string {
	if key := i18n.Canonical("resourceType", s); key != "" {
		return key
	}
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
	if key := i18n.Canonical("sort", s); key != "" {
		return key
	}
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
	if key := i18n.Canonical("accountTab", s); key != "" {
		return key
	}
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
	if key := i18n.Canonical("skinModel", s); key != "" {
		return key
	}
	v := strings.ToLower(strings.TrimSpace(s))
	if strings.Contains(v, "slim") || strings.Contains(v, "alex") || strings.Contains(v, "纤细") {
		return "slim"
	}
	return "default"
}

func normalizeInstallTarget(s string) string {
	if key := i18n.Canonical("installTarget", s); key != "" {
		return key
	}
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "current" || v == "当前目录" || v == "current instance" {
		return "current"
	}
	return "new"
}

func normalizeVersionKindKey(s string) string {
	if key := i18n.Canonical("versionKind", s); key != "" {
		return key
	}
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
	if a.selectedCape == "" || i18n.Canonical("cape", a.selectedCape) == "none" {
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

// L resolves a message key from the current language's JSON catalog.
func (a *App) L(key string) string {
	return i18n.Text(a.isEN(), key)
}
