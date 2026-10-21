package nativeui

import (
	"fmt"
	"mcl/internal/launcher"
	"strings"
)

const (
	LangZH = "zh-CN"
	LangEN = "en-US"
)

type Copy struct {
	Code string

	// Window & Shell
	AppTitle          string
	AppSubtitle       string
	WinMinimize       string
	WinMaximize       string
	WinClose          string
	LangToggleLabel   string
	LangToggleTooltip string
	NavAria           string
	BrandAria         string
	WorkspaceTag      string
	BreadcrumbAria    string
	DropOverlayTitle  string
	DropOverlayDesc   string

	// Navigation sections
	NavHome               string
	NavMods               string
	NavDiscover           string
	NavWardrobe           string
	NavTasks              string
	NavSettings           string
	SettingsAdvancedTitle string
	AccOpenSettingsDesc   string

	// Sidebar & Dock context
	SideInstancesTitle string
	SideAddInstance    string
	SideEmptyHint      string
	SideSelectPrefix   string
	SideAccountAdd     string
	SideAccountManage  string
	TopNewVersion      string
	TopImportPack      string
	TopLaunchNow       string
	TopRunning         string
	TopNoInstance      string
	TopActiveInstance  string
	DismissNotice      string
	NoticeRetry        string
	NoticeLogs         string
	NoticeCopy         string
	NoticeCopied       string
	NoticeError        string
	NoticeWarning      string
	NoticeInfo         string
	WhyBusy            string
	WhyNoFolder        string
	WhyImportMissing   string
	WhyNoVersion       string
	WhyRunning         string
	WhySkinUnsupported string
	WhySkinLoadFailed  string
	WhyNeedName        string
	WhyNeedLogin       string
	WhyPickVersion     string
	WhyInstallInvalid  string
	WardrobeLoadFailed string

	// Home / Launchpad Page
	HomeKicker             string
	HomeTitle              string
	HomeDesc               string
	HomeHeroBadgeJava      string
	HomeHeroBadgePack      string
	HomeHeroBadgeRunning   string
	HomeHeroBadgeReady     string
	HomeHeroEmptyTitle     string
	HomeHeroEmptyDesc      string
	HomeHeroNoVersion      string
	HomeDropHint           string
	HomeLaunchBtn          string
	HomeRunningBtn         string
	HomeStopBtn            string
	HomeInstallBtn         string
	HomeImportPackBtn      string
	HomeOpenFolderBtn      string
	HomeInstanceCfgBtn     string
	HomeVersionCfgBtn      string
	HomeBadgeCustomized    string
	HomeGroupLoaders       string
	AccAddAccountBtn       string
	AccBackBtn             string
	AccSetCurrentBtn       string
	AccTagMicrosoft        string
	AccTagOffline          string
	AccTagThird            string
	AccServerCustom        string
	AccAddServerBtn        string
	AccEditServerBtn       string
	AccBuiltinServer       string
	AccDeleteServerBtn     string
	AccServerNameLabel     string
	AccServerNameHint      string
	AccServerURLLabel      string
	AccFetchNameBtn        string
	AccServerSaveBtn       string
	AccServerCancelBtn     string
	AccServerURLInvalid    string
	DialogTitleDir         string
	HomeDirSettingsBtn     string
	HomeRootEmptyTitle     string
	HomeLogsBtn            string
	HomeVersionLabel       string
	HomeVersionsInRoot     string
	HomeQuickInstallLoader string
	HomeStatInstances      string
	HomeStatVersions       string
	HomeStatMods           string
	HomeStatAccount        string
	HomeInstancesTitle     string
	HomeInstancesSubtitle  string
	HomeAddFolderBtn       string
	HomeEmptyTitle         string
	HomeEmptyDesc          string
	HomeBadgeActive        string
	HomeBadgeUnavailable   string
	HomeBadgeRunning       string
	HomeBadgeCustomJava    string
	HomeBadgeCustomJVM     string
	HomeUseInstanceBtn     string
	HomeActiveInstanceBtn  string
	HomePortalTitle        string
	HomePortalSubtitle     string
	HomeModrinthDesc       string
	HomeCurseForgeDesc     string
	HomeActiveTaskTitle    string
	HomeViewTasksBtn       string

	// Mods, Resource Packs & Shaders Page
	ModsKicker           string
	ModsTitle            string
	ModsDesc             string
	ModsTabMods          string
	ModsTabResourcePack  string
	ModsTabShaders       string
	ModsCheckUpdatesBtn  string
	ModsImportJarBtn     string
	ModsImportPackBtn    string
	ModsExploreMoreBtn   string
	ModsEmptyRootTitle   string
	ModsEmptyRootDesc    string
	ModsContextRoot      string
	ModsContextVersion   string
	ModsSearchHint       string
	ModsSearchAria       string
	ModsStatTotal        string
	ModsStatEnabled      string
	ModsStatUpdatable    string
	ModsLoading          string
	ModsEmptyListTitle   string
	ModsEmptyListDesc    string
	ModsEmptyRPTitle     string
	ModsEmptyRPDesc      string
	ModsEmptyShaderTitle string
	ModsEmptyShaderDesc  string
	ModsTogglePrefix     string
	ModsUpdateBtn        string
	ModsRemoveBtn        string
	ModsPackAuditTitle   string
	ModsPackAuditDesc    string
	ModsPackVerifyBtn    string
	ModsCheckedNotice    string
	ModsUpdateQueued     string

	// Resource Center / Discover Page
	DiscoverKicker         string
	DiscoverTitle          string
	DiscoverDesc           string
	DiscoverTypeModpack    string
	DiscoverTypeMod        string
	DiscoverTypeRP         string
	DiscoverTypeShader     string
	DiscoverSearchHint     string
	DiscoverSearchAria     string
	DiscoverSearchBtn      string
	DiscoverFilterMCHint   string
	DiscoverFilterMCAria   string
	DiscoverFilterLoader   string
	DiscoverApplyFilterBtn string
	DiscoverSortRelevance  string
	DiscoverSortDownloads  string
	DiscoverSortUpdated    string
	DiscoverLoading        string
	DiscoverRetryBtn       string
	DiscoverSelectVerBtn   string
	DiscoverPrevPage       string
	DiscoverNextPage       string
	DiscoverEmptyTitle     string
	DiscoverEmptyDesc      string
	DiscoverIdleTitle      string
	DiscoverIdleDesc       string

	// Wardrobe Page
	WardrobeKicker           string
	WardrobeTitle            string
	WardrobeDesc             string
	WardrobeRefreshBtn       string
	WardrobeNoAccountTitle   string
	WardrobeNoAccountDesc    string
	WardrobeAddAccountBtn    string
	WardrobeOrbitAria        string
	WardrobeOrbitHint        string
	WardrobeZoomLabel        string
	WardrobeZoomAria         string
	WardrobeFrontBtn         string
	WardrobeBackBtn          string
	WardrobeAutoSpin         string
	WardrobeSkinTitle        string
	WardrobeSkinDesc         string
	WardrobeModelLabel       string
	WardrobeModelSteve       string
	WardrobeModelAlex        string
	WardrobeUploadSkinBtn    string
	WardrobeResetSkinBtn     string
	WardrobeCapeTitle        string
	WardrobeCapeMSDesc       string
	WardrobeCapeLocalDesc    string
	WardrobeCapeNone         string
	WardrobeApplyCapeBtn     string
	WardrobeUploadCapeBtn    string
	WardrobeRemoveCapeBtn    string
	WardrobeOfflineNoteTitle string
	WardrobeOfflineNote1     string
	WardrobeOfflineNote2     string
	WardrobeYggNoUpload      string
	WardrobeTextureSaved     string

	// Tasks & Activity Page
	TasksKicker      string
	TasksTitle       string
	TasksDesc        string
	TasksEmptyTitle  string
	TasksEmptyDesc   string
	TasksCancelBtn   string
	TasksLogsBtn     string
	TasksItemsHeader string

	// Settings Page
	SettingsKicker           string
	SettingsTitle            string
	SettingsDesc             string
	SettingsRuntimePanel     string
	SettingsRuntimeTitle     string
	SettingsRuntimeDesc      string
	SettingsJavaLabel        string
	SettingsJavaAria         string
	SettingsJavaAuto         string
	SettingsJavaCustom       string
	SettingsJavaBrowseBtn    string
	SettingsJavaScanBtn      string
	SettingsJavaAutoHint     string
	SettingsJavaScanningHint string
	SettingsMemoryLabel      string
	SettingsMemorySliderAria string
	SettingsMemoryInputAria  string
	SettingsJVMArgsLabel     string
	SettingsJVMArgsAria      string
	SettingsJVMArgsHint      string
	JVMPresetTitle           string
	JVMPresetG1              string
	JVMPresetZGC             string
	JVMPresetCompact         string
	JVMPresetClear           string
	JVMAdviceDefault         string
	JVMAdviceG1              string
	JVMAdviceZGC             string
	JVMAdviceCompact         string
	SettingsRootOverride     string
	SettingsRootOverrideDesc string
	SettingsRootOverrideAria string
	SettingsPrefsPanel       string
	SettingsPrefsTitle       string
	SettingsLanguageLabel    string
	SettingsLanguageDesc     string
	SettingsLanguageAria     string
	SettingsSnapshotsLabel   string
	SettingsSnapshotsDesc    string
	SettingsSnapshotsAria    string
	SettingsServicePanel     string
	SettingsServiceTitle     string
	SettingsServiceDesc      string
	SettingsMSDesc           string
	SettingsMRDesc           string
	SettingsCFDesc           string
	SettingsServiceConnected string
	SettingsServicePending   string
	SettingsAboutPanel       string
	SettingsAboutTitle       string
	SettingsAboutDesc        string
	SettingsOpenDataBtn      string
	SettingsActionBarAria    string
	SettingsDefaultHint      string
	SettingsResetBtn         string
	SettingsResetNotice      string

	// Dialog Titles & Shared
	DialogActiveAria     string
	DialogCloseBtn       string
	DialogBrowseBtn      string
	DialogBusyHint       string
	DialogChooseFolder   string
	DialogTitleAdd       string
	DialogTitleRoot      string
	DialogTitleRemRoot   string
	DialogTitleRemMod    string
	DialogTitleAccount   string
	DialogTitleImport    string
	DialogTitleInstall   string
	DialogTitleProject   string
	DialogTitlePackAudit string
	DialogTitleLogs      string
	DialogTitleJava      string

	// Add & Configure Instance / Per-Version Runtime Dialogs
	AddRootDesc            string
	AddRootFolderLabel     string
	AddRootNameLabel       string
	AddRootNamePlaceholder string
	AddRootConfirmBtn      string
	AddRootSuccess         string
	RootSectionInstance    string
	RootSectionVersion     string
	RootSectionVersionDesc string
	RootCfgVersionLabel    string
	RootCfgVerJavaLabel    string
	RootCfgVerJavaFollow   string
	RootCfgVerMemLabel     string
	RootCfgVerMemFollow    string
	MemModeAuto            string
	MemModeManual          string
	MemAutoValue           string
	MemAutoExplain         string
	RootCfgVerMemHint      string
	RootCfgVerJVMLabel     string
	RootCfgVerJVMHint      string
	RootCfgJVMPresetG1Btn  string
	RootCfgJVMClearBtn     string
	RootCfgJavaLabel       string
	RootCfgJavaHint        string
	RootCfgMemLabel        string
	RootCfgMemHint         string
	RootCfgSaveBtn         string
	RootCfgSaved           string
	RootCfgMemErr          string
	RootCfgInstallVerBtn   string
	RootCfgUnregisterBtn   string
	RemRootConfirmTitle    string
	RemRootSafeHint        string
	RemRootConfirmBtn      string
	RemRootSuccess         string
	RemModConfirmTitle     string
	RemModBackupHint       string
	RemModConfirmBtn       string

	// Account Dialog
	AccSavedSection      string
	AccAddSection        string
	AccTabOffline        string
	AccTabMicrosoft      string
	AccTabThirdParty     string
	AccKindOffline       string
	AccKindMicrosoft     string
	AccKindThirdParty    string
	AccUseBtn            string
	VersionDialogTitle   string
	VersionSnapshotWord  string
	VersionNameLabel     string
	VersionNameHint      string
	AccRemoveConfirm     string
	AccRemoveBtn         string
	AccOfflineDesc       string
	AccOfflineNameLabel  string
	AccOfflineNameHint   string
	AccOfflineAddBtn     string
	AccOfflineAdded      string
	AccMSDesc            string
	AccMSDisabled1       string
	AccMSDisabled2       string
	AccMSStartBtn        string
	AccMSCodePrompt      string
	AccMSCopyCodeBtn     string
	AccMSOpenBrowserBtn  string
	AccMSWaiting         string
	AccMSExpired         string
	AccMSAdded           string
	AccThirdDesc         string
	AccThirdServerLabel  string
	AccThirdUserLabel    string
	AccThirdUserHint     string
	AccThirdPassLabel    string
	AccThirdPassHint     string
	AccThirdProfileLabel string
	AccThirdLoginBtn     string
	AccThirdAdded        string

	// Import Modpack Dialog
	ImportDesc            string
	ImportRecognizedTitle string
	ImportFileLabel       string
	ImportFilePlaceholder string
	ImportFileHint        string
	ImportParentLabel     string
	ImportNameLabel       string
	ImportNamePlaceholder string
	ImportNote            string
	ImportConfirmBtn      string

	// Install Game & Mod Loader Guided Wizard
	InstallDesc               string
	InstallStep1Title         string
	InstallStep2Title         string
	InstallStep3Title         string
	InstallTargetLabel        string
	InstallTargetCurrent      string
	InstallTargetNew          string
	InstallParentLabel        string
	InstallNameLabel          string
	InstallNamePlaceholder    string
	InstallMCLabel            string
	InstallRefreshBtn         string
	InstallKindAria           string
	InstallKindAll            string
	InstallKindRelease        string
	InstallKindSnapshot       string
	InstallKindBeta           string
	InstallKindAlpha          string
	InstallKindOther          string
	InstallQueryAria          string
	InstallQueryHint          string
	InstallVersionAria        string
	InstallFetchingManifest   string
	InstallNoMatch            string
	InstallLoaderLabel        string
	InstallLoaderAria         string
	CardLoaderVersion         string
	InstallLoaderVanillaSub   string
	InstallLoaderFabricSub    string
	InstallLoaderForgeSub     string
	InstallLoaderNeoSub       string
	InstallLoaderQuiltSub     string
	InstallLoaderVerLabel     string
	InstallFetchingLoader     string
	InstallNoLoaderVer        string
	InstallConfirmBtn         string
	InstallTaskCreated        string
	ImportDoneNotice          string
	ImportSizeNote            string
	ImportSourceMultiMC       string
	TaskLaunchBtn             string
	TaskOpenDirBtn            string
	DropNeedRoot              string
	PackSecTitle              string
	PackSecCurrent            string
	PackSecLatest             string
	PackSecUpToDate           string
	PackSecNotChecked         string
	PackSecNoOrigin           string
	PackSecRunning            string
	PackCheckBtn              string
	PackUpdateToPrefix        string
	PackFromFileBtn           string
	PackRollbackPrefix        string
	PackUpdateBadge           string
	PackUpToDateNotice        string
	PackFoundNotice           string
	DialogTitlePackUpdate     string
	DialogTitlePackChanges    string
	DialogTitlePackDrop       string
	PackDropDesc              string
	PackDropUpdateBtn         string
	PackDropImportBtn         string
	PackChAdded               string
	PackChRemoved             string
	PackChUpdated             string
	PackChOverwritten         string
	PackReasonUser            string
	PackReasonModified        string
	PackReasonMissing         string
	PackKeepDisabled          string
	PackDownloadPrefix        string
	PackChangelog             string
	PackNoChangelog           string
	PackGameChange            string
	PackApplyBtn              string
	PackCancelBtn             string
	PackBackupNote            string
	PackUpdateDone            string
	PackRollbackDone          string
	TaskChangesBtn            string
	TaskRollbackBtn           string
	PackShowList              string
	PackHideList              string
	PackNothing               string
	PackUnchangedPrefix       string
	PackCheckingNotice        string
	PackDoneBadge             string
	PackChManual              string
	BackLabel                 string
	PackReasonRenamed         string
	PackReasonKeptOld         string
	PackGameWarn              string
	PackOpenDirBtn            string
	SettingsPackAutoCheck     string
	SettingsPackAutoCheckDesc string
	SettingsPackBeta          string
	SettingsPackBetaDesc      string
	SettingsPackAutoCheckAria string
	SettingsPackBetaAria      string
	InstallLaunchPreparing    string

	// Project & Pack Audit & Logs & Java Dialogs
	ProjectWebsiteBtn       string
	ProjectNoVersions       string
	ProjectVersionLabel     string
	ProjectFilterHint       string
	ProjectFilterAria       string
	ProjectNeedRootHint     string
	ProjectTargetPrefix     string
	ProjectInstallModBtn    string
	ProjectInstallRPBtn     string
	ProjectInstallShaderBtn string
	ProjectInstallPackBtn   string
	PackUpdateFoundPrefix   string
	PackUpdateInstallBtn    string
	PackManualPrefix        string
	PackManualOpenBtn       string
	LogsRefreshBtn          string
	LogsCopyBtn             string
	LogsEmpty               string
	LogsAreaAria            string
	JavaQuickScanBtn        string
	JavaDeepScanBtn         string
	JavaStopScanBtn         string
	JavaScanHint            string
	JavaFilterAria          string
	JavaFilterPlaceholder   string
	JavaUseAutoBtn          string
	JavaUseBtn              string
	JavaEmptyHint           string

	// Loaders
	LoaderVanilla  string
	LoaderAll      string
	LoaderFabric   string
	LoaderQuilt    string
	LoaderForge    string
	LoaderNeoForge string
}

var zhCopy = Copy{
	Code: LangZH,

	AppTitle:          "MCL · Minecraft 启动器",
	AppSubtitle:       "Minecraft Java 启动器",
	WinMinimize:       "最小化窗口",
	WinMaximize:       "最大化窗口",
	WinClose:          "关闭窗口",
	LangToggleLabel:   "EN",
	LangToggleTooltip: "Switch interface language to English",
	NavAria:           "主导航",
	BrandAria:         "侧栏品牌",
	WorkspaceTag:      "工作区",
	BreadcrumbAria:    "面包屑标题",
	DropOverlayTitle:  "松开鼠标即可识别并导入",
	DropOverlayDesc:   "支持 Modrinth (.mrpack) / CurseForge (.zip) 整合包、模组 (.jar)、材质光影 (.zip) 或游戏目录",

	NavHome:               "游戏库",
	NavMods:               "模组管理",
	NavDiscover:           "探索资源",
	NavWardrobe:           "角色衣橱",
	NavTasks:              "下载与任务",
	NavSettings:           "设置",
	SettingsAdvancedTitle: "高级",
	AccOpenSettingsDesc:   "Java、内存、语言与服务",

	SideInstancesTitle: "实例列表",
	SideAddInstance:    "添加游戏目录",
	SideEmptyHint:      "尚未添加游戏实例",
	SideSelectPrefix:   "选择 ",
	SideAccountAdd:     "添加账号",
	SideAccountManage:  "管理账号",
	TopNewVersion:      "安装游戏",
	TopImportPack:      "导入整合包",
	TopLaunchNow:       "启动游戏",
	TopRunning:         "运行中",
	TopNoInstance:      "未选择版本",
	TopActiveInstance:  "请先添加目录或安装游戏",
	DismissNotice:      "关闭",
	NoticeRetry:        "重试",
	NoticeLogs:         "查看日志",
	NoticeCopy:         "复制错误",
	NoticeCopied:       "已复制",
	NoticeError:        "错误",
	NoticeWarning:      "注意",
	NoticeInfo:         "提示",
	WhyBusy:            "有任务正在进行，请稍候",
	WhyNoFolder:        "先选择游戏文件夹",
	WhyImportMissing:   "先选择整合包文件、目标目录并填写名称",
	WhyNoVersion:       "先安装或选择一个版本",
	WhyRunning:         "游戏正在运行",
	WhySkinUnsupported: "当前账号类型不支持在启动器内更换",
	WhySkinLoadFailed:  "皮肤信息未能加载，可点击「刷新」重试",
	WhyNeedName:        "先输入玩家名称",
	WhyNeedLogin:       "先填写服务器、账号和密码",
	WhyPickVersion:     "先选择一个版本",
	WhyInstallInvalid:  "先修正上方标红的选项",
	WardrobeLoadFailed: "皮肤信息加载失败",

	HomeKicker:             "",
	HomeTitle:              "游戏版本",
	HomeDesc:               "管理本地游戏目录、版本与整合包",
	HomeHeroBadgeJava:      "Java 版",
	HomeHeroBadgePack:      "整合包",
	HomeHeroBadgeRunning:   "运行中",
	HomeHeroBadgeReady:     "就绪",
	HomeHeroEmptyTitle:     "",
	HomeHeroEmptyDesc:      "",
	HomeHeroNoVersion:      "尚未安装版本",
	HomeDropHint:           "",
	HomeLaunchBtn:          "启动游戏",
	HomeRunningBtn:         "游戏运行中",
	HomeStopBtn:            "停止运行",
	HomeInstallBtn:         "安装游戏",
	HomeImportPackBtn:      "导入整合包",
	HomeOpenFolderBtn:      "打开目录",
	HomeInstanceCfgBtn:     "管理",
	HomeVersionCfgBtn:      "设置",
	HomeBadgeCustomized:    "已自定义",
	HomeGroupLoaders:       "模组加载器",
	AccAddAccountBtn:       "添加账号",
	AccBackBtn:             "返回",
	AccSetCurrentBtn:       "设为当前",
	AccTagMicrosoft:        "正版",
	AccTagOffline:          "离线",
	AccTagThird:            "第三方",
	AccServerCustom:        "自定义地址",
	AccAddServerBtn:        "添加服务器",
	AccEditServerBtn:       "编辑服务器",
	AccBuiltinServer:       "内置",
	AccDeleteServerBtn:     "删除服务器",
	AccServerNameLabel:     "名称",
	AccServerNameHint:      "留空则使用域名",
	AccServerURLLabel:      "认证服务器地址",
	AccFetchNameBtn:        "获取名称",
	AccServerSaveBtn:       "保存",
	AccServerCancelBtn:     "取消",
	AccServerURLInvalid:    "地址必须为有效的 HTTPS URL",
	DialogTitleDir:         "目录设置",
	HomeDirSettingsBtn:     "目录设置",
	HomeRootEmptyTitle:     "此目录还没有版本",
	HomeLogsBtn:            "运行日志",
	HomeVersionLabel:       "启动版本",
	HomeVersionsInRoot:     "已安装版本",
	HomeQuickInstallLoader: "安装新版本",
	HomeStatInstances:      "游戏目录",
	HomeStatVersions:       "已装版本",
	HomeStatMods:           "启用模组",
	HomeStatAccount:        "当前角色",
	HomeInstancesTitle:     "游戏目录",
	HomeInstancesSubtitle:  "",
	HomeAddFolderBtn:       "添加目录",
	HomeEmptyTitle:         "暂无游戏版本",
	HomeEmptyDesc:          "添加本地目录、安装新版本，或直接将整合包拖入窗口",
	HomeBadgeActive:        "当前",
	HomeBadgeUnavailable:   "路径失效",
	HomeBadgeRunning:       "运行中",
	HomeBadgeCustomJava:    "独立 Java",
	HomeBadgeCustomJVM:     "自定义 JVM",
	HomeUseInstanceBtn:     "选择",
	HomeActiveInstanceBtn:  "已选",
	HomePortalTitle:        "社区资源与整合包",
	HomePortalSubtitle:     "浏览并安装来自 Modrinth 与 CurseForge 的精选内容",
	HomeModrinthDesc:       "开源模组与轻量整合包",
	HomeCurseForgeDesc:     "大型整合包与热门模组",
	HomeActiveTaskTitle:    "后台任务进行中",
	HomeViewTasksBtn:       "查看任务",

	ModsKicker:           "MODS · RESOURCE PACKS · SHADERS",
	ModsTitle:            "模组与资源",
	ModsDesc:             "管理当前版本的模组、材质包与光影",
	ModsTabMods:          "模组 (Mods)",
	ModsTabResourcePack:  "材质包 (Resource Packs)",
	ModsTabShaders:       "光影包 (Shaders)",
	ModsCheckUpdatesBtn:  "检测更新",
	ModsImportJarBtn:     "添加模组",
	ModsImportPackBtn:    "导入本地包 (.zip)",
	ModsExploreMoreBtn:   "去探索资源下载",
	ModsEmptyRootTitle:   "请先选择或安装一个游戏版本",
	ModsEmptyRootDesc:    "在游戏库添加目录并选择具体版本后，即可管理该版本的模组、材质包与光影。",
	ModsContextRoot:      "游戏目录",
	ModsContextVersion:   "目标版本",
	ModsSearchHint:       "搜索名称、文件名或 ID…",
	ModsSearchAria:       "搜索已安装模组",
	ModsStatTotal:        "总计",
	ModsStatEnabled:      "已启用",
	ModsStatUpdatable:    "可更新",
	ModsLoading:          "正在读取当前版本目录文件…",
	ModsEmptyListTitle:   "当前版本暂未安装模组",
	ModsEmptyListDesc:    "点击右上角「添加模组」导入本地 JAR（或直接拖入窗口），也可前往「探索资源」在线下载。",
	ModsEmptyRPTitle:     "当前版本暂未安装材质包",
	ModsEmptyRPDesc:      "点击右上角「导入本地包 (.zip)」添加材质包，或前往「探索资源 → 材质包」下载。",
	ModsEmptyShaderTitle: "当前版本暂未安装光影包",
	ModsEmptyShaderDesc:  "点击右上角「导入本地包 (.zip)」添加光影包，或前往「探索资源 → 光影」下载。",
	ModsTogglePrefix:     "启用 ",
	ModsUpdateBtn:        "更新",
	ModsRemoveBtn:        "移除",
	ModsPackAuditTitle:   "整合包完整性校验",
	ModsPackAuditDesc:    "核对整合包受管文件哈希值，并检查是否有新版本发布",
	ModsPackVerifyBtn:    "检测整合包",
	ModsCheckedNotice:    "模组版本检查完成",
	ModsUpdateQueued:     "模组更新已加入任务队列",

	DiscoverKicker:         "COMMUNITY MARKETPLACE",
	DiscoverTitle:          "探索资源",
	DiscoverDesc:           "浏览并下载整合包、模组、材质包与光影",
	DiscoverTypeModpack:    "整合包",
	DiscoverTypeMod:        "模组",
	DiscoverTypeRP:         "材质包",
	DiscoverTypeShader:     "光影",
	DiscoverSearchHint:     "输入名称或关键词搜索…",
	DiscoverSearchAria:     "搜索资源",
	DiscoverSearchBtn:      "搜索",
	DiscoverFilterMCHint:   "游戏版本 (如 1.21.1)",
	DiscoverFilterMCAria:   "Minecraft 版本筛选",
	DiscoverFilterLoader:   "加载器筛选",
	DiscoverApplyFilterBtn: "应用筛选",
	DiscoverSortRelevance:  "综合排序",
	DiscoverSortDownloads:  "下载最多",
	DiscoverSortUpdated:    "最近更新",
	DiscoverLoading:        "正在加载资源列表…",
	DiscoverRetryBtn:       "重试",
	DiscoverSelectVerBtn:   "选择版本",
	DiscoverPrevPage:       "上一页",
	DiscoverNextPage:       "下一页",
	DiscoverEmptyTitle:     "未找到匹配的资源",
	DiscoverEmptyDesc:      "请尝试更换关键词、清空版本筛选或切换平台。",
	DiscoverIdleTitle:      "开始探索资源",
	DiscoverIdleDesc:       "选择平台与类型，输入关键词后按回车或点击“搜索”。",

	WardrobeKicker:           "SKIN & CAPE STUDIO",
	WardrobeTitle:            "角色衣橱",
	WardrobeDesc:             "预览并更换账号皮肤与披风",
	WardrobeRefreshBtn:       "刷新",
	WardrobeNoAccountTitle:   "尚未选择游戏账号",
	WardrobeNoAccountDesc:    "点击左侧栏上方的「添加账号」卡片，即可添加离线角色、Microsoft 正版或外置登录账号。",
	WardrobeAddAccountBtn:    "添加账号",
	WardrobeOrbitAria:        "旋转皮肤预览",
	WardrobeOrbitHint:        "按住鼠标拖拽旋转角色 · 拖动滑杆调整缩放",
	WardrobeZoomLabel:        "缩放",
	WardrobeZoomAria:         "皮肤缩放",
	WardrobeFrontBtn:         "正面",
	WardrobeBackBtn:          "背面",
	WardrobeAutoSpin:         "自动旋转",
	WardrobeSkinTitle:        "角色皮肤",
	WardrobeSkinDesc:         "支持标准 64×64 双层皮肤与传统 64×32 PNG 材质。",
	WardrobeModelLabel:       "手臂模型",
	WardrobeModelSteve:       "经典 Steve (4px 宽臂)",
	WardrobeModelAlex:        "纤细 Alex (3px 细臂)",
	WardrobeUploadSkinBtn:    "选择皮肤",
	WardrobeResetSkinBtn:     "恢复默认",
	WardrobeCapeTitle:        "披风配饰",
	WardrobeCapeMSDesc:       "切换当前 Microsoft 正版账号已拥有的官方披风。",
	WardrobeCapeLocalDesc:    "上传本地 PNG 披风材质供预览与外置登录使用。",
	WardrobeCapeNone:         "不使用披风",
	WardrobeApplyCapeBtn:     "应用披风",
	WardrobeUploadCapeBtn:    "选择披风",
	WardrobeRemoveCapeBtn:    "移除披风",
	WardrobeOfflineNoteTitle: "本机离线材质说明",
	WardrobeOfflineNote1:     "离线角色的皮肤与披风保存在本机数据目录，用于启动器衣橱 3D 预览。",
	WardrobeOfflineNote2:     "若需在离线单人或局域网内显示自定义皮肤，请配合 CustomSkinLoader 等模组。",
	WardrobeYggNoUpload:      "该第三方认证服未开放客户端直接上传材质。",
	WardrobeTextureSaved:     "材质已保存",

	TasksKicker:      "DOWNLOADS & ACTIVITY",
	TasksTitle:       "下载与任务",
	TasksDesc:        "查看版本安装与文件下载进度",
	TasksEmptyTitle:  "暂无后台任务",
	TasksEmptyDesc:   "安装游戏版本、导入整合包或下载模组/材质/光影时，详细下载进度会在此显示。",
	TasksCancelBtn:   "取消",
	TasksLogsBtn:     "查看运行日志",
	TasksItemsHeader: "下载与执行明细：",

	SettingsKicker:           "PREFERENCES & RUNTIME",
	SettingsTitle:            "设置",
	SettingsDesc:             "Java 环境、内存分配与全局偏好",
	SettingsRuntimePanel:     "启动环境面板",
	SettingsRuntimeTitle:     "启动环境",
	SettingsRuntimeDesc:      "配置全局默认 Java、最大堆内存与附加 JVM 参数",
	SettingsJavaLabel:        "Java 运行时",
	SettingsJavaAria:         "Java 运行时选择",
	SettingsJavaAuto:         "自动选择",
	SettingsJavaCustom:       "自定义 Java",
	SettingsJavaBrowseBtn:    "浏览",
	SettingsJavaScanBtn:      "检测",
	SettingsJavaAutoHint:     "按所选 Minecraft 版本自动匹配兼容的 Java 运行时。",
	SettingsJavaScanningHint: "正在后台快速检测本机 Java 环境…",
	SettingsMemoryLabel:      "最大内存",
	SettingsMemorySliderAria: "最大内存滑块",
	SettingsMemoryInputAria:  "最大内存输入",
	SettingsJVMArgsLabel:     "JVM 优化参数",
	SettingsJVMArgsAria:      "全局 JVM 参数输入",
	SettingsJVMArgsHint:      "留空使用默认参数，或点击下方预设一键填入优化方案",
	JVMPresetTitle:           "推荐方案：",
	JVMPresetG1:              "G1GC 均衡 (推荐)",
	JVMPresetZGC:             "ZGC 低延迟 (Java 17+)",
	JVMPresetCompact:         "轻量省内存 (2–4G)",
	JVMPresetClear:           "清空",
	JVMAdviceDefault:         "建议：通用与整合包推荐「G1GC 均衡」降低卡顿；Java 17/21+ 且内存 ≥6G 可选「ZGC 低延迟」。",
	JVMAdviceG1:              "G1GC 均衡：限制单次 GC 停顿上限 50ms 并优化分代比例，兼顾帧率稳定与吞吐量。",
	JVMAdviceZGC:             "ZGC 低延迟：亚毫秒级垃圾回收停顿，适合 Java 17/21+ 与 6G+ 大型整合包。",
	JVMAdviceCompact:         "轻量省内存：启用 G1 字符串去重与 16M 分区，降低 2–4G 内存设备占用。",
	SettingsRootOverride:     "跟随目录设置",
	SettingsRootOverrideDesc: "启用后，各版本与实例配置的独立 Java、内存与 JVM 参数优先于全局设置。",
	SettingsRootOverrideAria: "跟随目录设置开关",
	SettingsPrefsPanel:       "游戏版本面板",
	SettingsPrefsTitle:       "界面与版本偏好",
	SettingsLanguageLabel:    "界面语言 / Language",
	SettingsLanguageDesc:     "实时切换中文与英文界面。",
	SettingsLanguageAria:     "界面语言选择",
	SettingsSnapshotsLabel:   "显示快照版本",
	SettingsSnapshotsDesc:    "安装向导默认包含每周快照与预发布版。",
	SettingsSnapshotsAria:    "显示快照版本开关",
	SettingsServicePanel:     "内置服务面板",
	SettingsServiceTitle:     "内置服务状态",
	SettingsServiceDesc:      "由发行版统一提供，无需手动填写密钥",
	SettingsMSDesc:           "Microsoft 正版设备码登录",
	SettingsMRDesc:           "Modrinth 开源模组与资源库",
	SettingsCFDesc:           "CurseForge 社区模组与整合包库",
	SettingsServiceConnected: "已连接",
	SettingsServicePending:   "待接入",
	SettingsAboutPanel:       "关于 MCL 面板",
	SettingsAboutTitle:       "关于 MCL",
	SettingsAboutDesc:        "纯 Go 原生桌面启动器 · 零 WebView 依赖",
	SettingsOpenDataBtn:      "打开数据目录",
	SettingsActionBarAria:    "设置操作栏",
	SettingsDefaultHint:      "改动立即生效，对后续新启动的游戏进程生效",
	SettingsResetBtn:         "恢复默认",
	SettingsResetNotice:      "已恢复默认值",

	DialogActiveAria:     "活动弹窗",
	DialogCloseBtn:       "关闭",
	DialogBrowseBtn:      "浏览",
	DialogBusyHint:       "正在处理，请稍候…",
	DialogChooseFolder:   "选择文件夹",
	DialogTitleAdd:       "添加游戏目录",
	DialogTitleRoot:      "设置",
	DialogTitleRemRoot:   "移除目录登记",
	DialogTitleRemMod:    "移除资源文件",
	DialogTitleAccount:   "账号管理",
	DialogTitleImport:    "导入整合包",
	DialogTitleInstall:   "安装 Minecraft 与模组加载器向导",
	DialogTitleProject:   "选择资源版本",
	DialogTitlePackAudit: "整合包状态报告",
	DialogTitleLogs:      "游戏运行日志",
	DialogTitleJava:      "Java 运行时检测",

	AddRootDesc:            "支持选择外层游戏根目录、.minecraft 目录、versions 目录，或直接选择 versions 下的具体版本文件夹。",
	AddRootFolderLabel:     "游戏文件夹",
	AddRootNameLabel:       "目录名称",
	AddRootNamePlaceholder: "留空则自动使用文件夹名称",
	AddRootConfirmBtn:      "确认添加目录",
	AddRootSuccess:         "游戏目录已添加",
	RootSectionInstance:    "游戏根目录与默认参数",
	RootSectionVersion:     "单版本独立运行参数 (Java / 内存 / JVM)",
	RootSectionVersionDesc: "针对 versions 下的不同游戏版本，单独配置专属 Java 路径、最大堆内存与附加 JVM 启动参数。",
	RootCfgVersionLabel:    "配置目标版本",
	RootCfgVerJavaLabel:    "版本专属 Java",
	RootCfgVerJavaFollow:   "跟随目录 / 全局设置",
	RootCfgVerMemLabel:     "版本最大内存 (MiB)",
	RootCfgVerMemFollow:    "默认",
	MemModeAuto:            "自动",
	MemModeManual:          "手动",
	MemAutoValue:           "自动（约 %.1f GB）",
	MemAutoExplain:         "按物理内存与当前空闲内存、游戏版本、加载器和模组数量计算，并为系统保留至少 2.5 GB（或 25%）。",
	RootCfgVerMemHint:      "填 0 表示跟随目录或全局内存设置",
	RootCfgVerJVMLabel:     "版本附加 JVM 参数",
	RootCfgVerJVMHint:      "例如 -XX:+UseG1GC",
	RootCfgJVMPresetG1Btn:  "+ 填入 G1GC 优化参数",
	RootCfgJVMClearBtn:     "清空参数",
	RootCfgJavaLabel:       "目录默认 Java 路径",
	RootCfgJavaHint:        "留空则跟随全局 Java 设置",
	RootCfgMemLabel:        "目录默认内存 (MiB)",
	RootCfgMemHint:         "填 0 表示跟随全局内存设置",
	RootCfgSaveBtn:         "保存",
	RootCfgSaved:           "实例与版本设置已保存",
	RootCfgMemErr:          "内存请输入 0 或 512–65536 之间的整数",
	RootCfgInstallVerBtn:   "安装新版本 / 加载器",
	RootCfgUnregisterBtn:   "移除目录",
	RemRootConfirmTitle:    "确认从启动器移除实例「%s」？",
	RemRootSafeHint:        "仅移除启动器内的列表登记，磁盘上的游戏文件、存档与模组均完好保留。",
	RemRootConfirmBtn:      "确认移除",
	RemRootSuccess:         "目录登记已移除",
	RemModConfirmTitle:     "确认移除文件：%s",
	RemModBackupHint:       "原文件会自动备份至该目录的 .mcl/backups 中。",
	RemModConfirmBtn:       "确认移除",

	AccSavedSection:      "已登录账号",
	AccAddSection:        "添加新账号",
	AccTabOffline:        "离线登录",
	AccTabMicrosoft:      "正版登录",
	AccTabThirdParty:     "第三方登录",
	AccKindOffline:       "离线账号",
	AccKindMicrosoft:     "Microsoft 正版",
	AccKindThirdParty:    "外置登录",
	AccUseBtn:            "切换",
	AccRemoveBtn:         "移除账号",
	AccRemoveConfirm:     "确认移除",
	VersionDialogTitle:   "版本设置",
	VersionSnapshotWord:  "快照",
	VersionNameLabel:     "显示名称",
	VersionNameHint:      "留空则使用默认名称",
	AccOfflineDesc:       "无需密码，生成标准 OfflinePlayer UUID 供单人或离线服务器游玩。",
	AccOfflineNameLabel:  "角色名",
	AccOfflineNameHint:   "请输入 3–16 位英文字母、数字或下划线",
	AccOfflineAddBtn:     "添加离线账号",
	AccOfflineAdded:      "离线账号已添加",
	AccMSDesc:            "通过 Microsoft 设备代码在系统浏览器中安全完成正版授权。",
	AccMSDisabled1:       "当前开发构建未注入发行方 Microsoft Client ID，暂未启用正版登录。",
	AccMSDisabled2:       "正式发行版由构建流水线内置服务标识，玩家无需手动填写。",
	AccMSStartBtn:        "使用 Microsoft 登录",
	AccMSCodePrompt:      "请在浏览器打开授权页并输入以下验证码：",
	AccMSCopyCodeBtn:     "复制验证码",
	AccMSOpenBrowserBtn:  "打开授权页面",
	AccMSWaiting:         "正在等待浏览器授权完成，验证通过后自动绑定…",
	AccMSExpired:         "验证码已过期，请重新发起登录",
	AccMSAdded:           "Microsoft 正版账号已添加",
	AccThirdDesc:         "接入支持 Yggdrasil / authlib-injector 的第三方皮肤站（如 LittleSkin）或私服认证。",
	AccThirdServerLabel:  "认证服务器",
	AccThirdUserLabel:    "账号",
	AccThirdUserHint:     "请输入邮箱或角色用户名",
	AccThirdPassLabel:    "密码",
	AccThirdPassHint:     "请输入登录密码",
	AccThirdProfileLabel: "选择绑定角色",
	AccThirdLoginBtn:     "登录并添加",
	AccThirdAdded:        "第三方账号已添加",

	ImportDesc:            "支持导入 Modrinth (.mrpack) 、CurseForge (.zip) 整合包与 MultiMC / Prism 实例 (.zip)，自动识别游戏版本与加载器。",
	ImportRecognizedTitle: "已识别整合包清单：",
	ImportFileLabel:       "整合包文件",
	ImportFilePlaceholder: ".mrpack / .zip 文件",
	ImportFileHint:        "点「浏览」选择，或把文件拖到窗口任意位置（支持 Modrinth、CurseForge、MultiMC/Prism）",
	ImportParentLabel:     "安装到父文件夹",
	ImportNameLabel:       "新目录名称",
	ImportNamePlaceholder: "为该整合包创建独立的实例文件夹名称",
	ImportNote:            "将创建独立目录，安装对应 Minecraft 版本与模组加载器并下载全部整合包文件（可在任务页实时查看逐文件进度）。",
	ImportConfirmBtn:      "导入整合包",

	InstallDesc:               "按步骤引导安装 Mojang 官方原版及 Fabric、Forge、NeoForge、Quilt 模组加载器。",
	InstallStep1Title:         "步骤 1 · 选择安装位置",
	InstallStep2Title:         "步骤 2 · 选择 Minecraft 官方版本",
	InstallStep3Title:         "步骤 3 · 选择模组加载器与版本",
	InstallTargetLabel:        "安装目标位置",
	InstallTargetCurrent:      "当前目录",
	InstallTargetNew:          "新建独立目录",
	InstallParentLabel:        "安装到父文件夹",
	InstallNameLabel:          "新目录名称",
	InstallNamePlaceholder:    "例如：1.21-Fabric-生存实例",
	InstallMCLabel:            "Minecraft 官方版本",
	InstallRefreshBtn:         "更新列表",
	InstallKindAria:           "Minecraft 版本分类",
	InstallKindAll:            "全部版本",
	InstallKindRelease:        "正式版",
	InstallKindSnapshot:       "快照 / 预发布",
	InstallKindBeta:           "历史 Beta",
	InstallKindAlpha:          "历史 Alpha",
	InstallKindOther:          "其他版本",
	InstallQueryAria:          "搜索 Minecraft 版本",
	InstallQueryHint:          "搜索版本号，如 1.21、24w、b1.7.3…",
	InstallVersionAria:        "Minecraft 版本选择",
	InstallFetchingManifest:   "正在同步 Mojang 官方版本清单…",
	InstallNoMatch:            "没有匹配的版本，请切换分类或清空搜索。",
	InstallLoaderLabel:        "模组加载器",
	InstallLoaderAria:         "模组加载器选择",
	InstallLoaderVanillaSub:   "原版",
	CardLoaderVersion:         "加载器",
	InstallLoaderFabricSub:    "轻量现代加载器",
	InstallLoaderForgeSub:     "经典模组生态",
	InstallLoaderNeoSub:       "1.20.1+ 现代加载器",
	InstallLoaderQuiltSub:     "Fabric 分支加载器",
	InstallLoaderVerLabel:     "选择加载器版本",
	InstallFetchingLoader:     "正在获取兼容的加载器版本…",
	InstallNoLoaderVer:        "当前所选 Minecraft 版本暂无该加载器支持，可切换为「原版」或其他加载器。",
	InstallConfirmBtn:         "开始安装",
	InstallTaskCreated:        "安装任务已启动，正在后台下载组件",
	ImportDoneNotice:          "导入完成",
	ImportSizeNote:            "导入时会一并下载完整游戏（客户端、运行库与资源文件），约 %s，完成后可直接启动。",
	ImportSourceMultiMC:       "MultiMC / Prism 实例",
	TaskLaunchBtn:             "启动",
	TaskOpenDirBtn:            "查看目录",
	DropNeedRoot:              "请先添加或安装一个游戏目录，再拖入材质包 / 光影包",
	PackSecTitle:              "整合包",
	PackSecCurrent:            "当前版本",
	PackSecLatest:             "最新版本",
	PackSecUpToDate:           "已是最新",
	PackSecNotChecked:         "尚未检查",
	PackSecNoOrigin:           "未找到在线来源，可使用「从文件更新」",
	PackSecRunning:            "游戏运行中，无法更新整合包",
	PackCheckBtn:              "检查更新",
	PackUpdateToPrefix:        "更新到 ",
	PackFromFileBtn:           "从文件更新",
	PackRollbackPrefix:        "回滚到 ",
	PackUpdateBadge:           "有更新",
	PackUpToDateNotice:        "已是最新版本",
	PackFoundNotice:           "发现新版本：",
	DialogTitlePackUpdate:     "更新整合包",
	DialogTitlePackChanges:    "更新内容",
	DialogTitlePackDrop:       "已安装此整合包",
	PackDropDesc:              "当前目录「%s」已安装此整合包（%s）。要更新它，还是导入为新目录？",
	PackDropUpdateBtn:         "更新当前目录",
	PackDropImportBtn:         "导入为新目录",
	PackChAdded:               "新增",
	PackChRemoved:             "移除",
	PackChUpdated:             "更新",
	PackChOverwritten:         "已覆盖（玩家改过）",
	PackReasonUser:            "玩家添加的同名文件",
	PackReasonModified:        "玩家改过",
	PackReasonMissing:         "已补回",
	PackKeepDisabled:          "保持禁用",
	PackDownloadPrefix:        "需下载 ",
	PackChangelog:             "更新日志",
	PackNoChangelog:           "此版本没有更新日志",
	PackGameChange:            "游戏版本将变更：",
	PackApplyBtn:              "更新整合包",
	PackCancelBtn:             "取消",
	PackBackupNote:            "更新前会自动备份所有将被覆盖或删除的文件（保留最近 3 次），随时可以回滚。你自己添加的模组和存档不会被动到。",
	PackUpdateDone:            "整合包更新完成",
	PackRollbackDone:          "已回滚整合包",
	TaskChangesBtn:            "查看变更",
	TaskRollbackBtn:           "回滚",
	PackShowList:              "展开",
	PackHideList:              "收起",
	PackNothing:               "没有文件需要变动",
	PackUnchangedPrefix:       "未变化 ",
	PackCheckingNotice:        "正在准备更新…",
	PackDoneBadge:             "更新完成",
	PackChManual:              "需手动下载",
	BackLabel:                 "返回",
	PackReasonRenamed:         "文件名已变",
	PackReasonKeptOld:         "暂留旧版本",
	PackGameWarn:              "将改用 %s 启动。旧版本 %s 会保留在 versions 文件夹中，回滚后可继续使用。更换游戏或加载器版本可能让存档无法降级，建议先备份 saves 文件夹。",
	PackOpenDirBtn:            "打开游戏目录",
	SettingsPackAutoCheck:     "启动时检查整合包更新",
	SettingsPackAutoCheckDesc: "每 24 小时最多检查一次，网络不通时不提示",
	SettingsPackBeta:          "整合包更新包含测试版",
	SettingsPackBetaDesc:      "检查更新时也考虑 Beta / Alpha 版本",
	SettingsPackAutoCheckAria: "自动检查整合包更新",
	SettingsPackBetaAria:      "包含测试版",
	InstallLaunchPreparing:    "正在准备启动环境，可在任务页查看进度",

	ProjectWebsiteBtn:       "访问项目主页",
	ProjectNoVersions:       "未找到与当前筛选条件兼容的发行版本。",
	ProjectVersionLabel:     "选择发行版本 (支持滚动与筛选)",
	ProjectFilterHint:       "筛选版本号、游戏版本 (如 1.21.1) 或加载器 (如 Fabric)…",
	ProjectFilterAria:       "筛选资源发行版本",
	ProjectNeedRootHint:     "安装前，请先在游戏库添加并选中一个目标游戏版本。",
	ProjectTargetPrefix:     "安装到当前版本：",
	ProjectInstallModBtn:    "安装模组",
	ProjectInstallRPBtn:     "安装材质包",
	ProjectInstallShaderBtn: "安装光影包",
	ProjectInstallPackBtn:   "下载并创建整合包实例",
	PackUpdateFoundPrefix:   "发现新版本：",
	PackUpdateInstallBtn:    "安装新版本到独立目录",
	PackManualPrefix:        "需在浏览器手动下载：",
	PackManualOpenBtn:       "前往下载页面",
	LogsRefreshBtn:          "刷新日志",
	LogsCopyBtn:             "复制日志",
	LogsEmpty:               "当前版本暂无运行日志输出。",
	LogsAreaAria:            "游戏日志",
	JavaQuickScanBtn:        "快速扫描",
	JavaDeepScanBtn:         "全盘扫描",
	JavaStopScanBtn:         "停止扫描",
	JavaScanHint:            "快速扫描检查 PATH、注册表、常见 JDK 目录与官方运行时；全盘扫描额外检索各磁盘便携版 Java。",
	JavaFilterAria:          "筛选 Java",
	JavaFilterPlaceholder:   "按版本号、发行商（Adoptium / Zulu）或路径筛选…",
	JavaUseAutoBtn:          "使用自动选择",
	JavaUseBtn:              "使用",
	JavaEmptyHint:           "暂未检测到 Java 运行时。可尝试「全盘扫描」或点击「浏览」手动选择 java.exe。",

	LoaderVanilla:  "原版",
	LoaderAll:      "全部加载器",
	LoaderFabric:   "Fabric",
	LoaderQuilt:    "Quilt",
	LoaderForge:    "Forge",
	LoaderNeoForge: "NeoForge",
}

var enCopy = Copy{
	Code: LangEN,

	AppTitle:          "MCL · Minecraft Launcher",
	AppSubtitle:       "Minecraft Java Launcher",
	WinMinimize:       "Minimize Window",
	WinMaximize:       "Maximize Window",
	WinClose:          "Close Window",
	LangToggleLabel:   "中文",
	LangToggleTooltip: "切换界面语言为简体中文",
	NavAria:           "Main Navigation",
	BrandAria:         "Sidebar Brand",
	WorkspaceTag:      "Workspace",
	BreadcrumbAria:    "Breadcrumb Title",
	DropOverlayTitle:  "Drop Files to Recognize & Install",
	DropOverlayDesc:   "Supports Modrinth (.mrpack) & CurseForge (.zip) modpacks, Mods (.jar), Resource/Shader Packs (.zip), or Game Folders",

	NavHome:               "Instances",
	NavMods:               "Mods",
	NavDiscover:           "Discover",
	NavWardrobe:           "Wardrobe",
	NavTasks:              "Tasks & Logs",
	NavSettings:           "Settings",
	SettingsAdvancedTitle: "Advanced",
	AccOpenSettingsDesc:   "Java, memory, language and services",

	SideInstancesTitle: "INSTANCES",
	SideAddInstance:    "Add Instance Folder",
	SideEmptyHint:      "No instances registered yet",
	SideSelectPrefix:   "Select ",
	SideAccountAdd:     "Add Account",
	SideAccountManage:  "Manage Accounts",
	TopNewVersion:      "Install Game",
	TopImportPack:      "Import Modpack",
	TopLaunchNow:       "Launch Game",
	TopRunning:         "Running",
	TopNoInstance:      "No Version Selected",
	TopActiveInstance:  "Add a folder or install a game to launch",
	DismissNotice:      "Dismiss",
	NoticeRetry:        "Retry",
	NoticeLogs:         "View logs",
	NoticeCopy:         "Copy error",
	NoticeCopied:       "Copied",
	NoticeError:        "Error",
	NoticeWarning:      "Warning",
	NoticeInfo:         "Info",
	WhyBusy:            "A task is running, please wait",
	WhyNoFolder:        "Choose a game folder first",
	WhyImportMissing:   "Choose a modpack file, target folder and name first",
	WhyNoVersion:       "Install or select a version first",
	WhyRunning:         "The game is running",
	WhySkinUnsupported: "This account type cannot change it from the launcher",
	WhySkinLoadFailed:  "Skin info failed to load; press Refresh to retry",
	WhyNeedName:        "Enter a player name first",
	WhyNeedLogin:       "Fill in server, account and password first",
	WhyPickVersion:     "Choose a version first",
	WhyInstallInvalid:  "Fix the highlighted option above first",
	WardrobeLoadFailed: "Could not load skin info",

	HomeKicker:             "",
	HomeTitle:              "Game Instances",
	HomeDesc:               "Manage local game folders, versions, and modpacks",
	HomeHeroBadgeJava:      "Java Edition",
	HomeHeroBadgePack:      "Modpack",
	HomeHeroBadgeRunning:   "Running",
	HomeHeroBadgeReady:     "Ready",
	HomeHeroEmptyTitle:     "",
	HomeHeroEmptyDesc:      "",
	HomeHeroNoVersion:      "No Version",
	HomeDropHint:           "",
	HomeLaunchBtn:          "Launch Game",
	HomeRunningBtn:         "Game Running",
	HomeStopBtn:            "Stop Game",
	HomeInstallBtn:         "Install Game",
	HomeImportPackBtn:      "Import Modpack",
	HomeOpenFolderBtn:      "Open Folder",
	HomeInstanceCfgBtn:     "Configure",
	HomeVersionCfgBtn:      "Settings",
	HomeBadgeCustomized:    "Customized",
	HomeGroupLoaders:       "Mod Loaders",
	AccAddAccountBtn:       "Add Account",
	AccBackBtn:             "Back",
	AccSetCurrentBtn:       "Use",
	AccTagMicrosoft:        "Microsoft",
	AccTagOffline:          "Offline",
	AccTagThird:            "Third-party",
	AccServerCustom:        "Custom address",
	AccAddServerBtn:        "Add Server",
	AccEditServerBtn:       "Edit Server",
	AccBuiltinServer:       "Built-in",
	AccDeleteServerBtn:     "Delete Server",
	AccServerNameLabel:     "Name",
	AccServerNameHint:      "Leave empty to use the host",
	AccServerURLLabel:      "Server address",
	AccFetchNameBtn:        "Fetch Name",
	AccServerSaveBtn:       "Save",
	AccServerCancelBtn:     "Cancel",
	AccServerURLInvalid:    "Address must be a valid HTTPS URL",
	DialogTitleDir:         "Folder Settings",
	HomeDirSettingsBtn:     "Folder settings",
	HomeRootEmptyTitle:     "No versions in this folder yet",
	HomeLogsBtn:            "Game Logs",
	HomeVersionLabel:       "Launch Version",
	HomeVersionsInRoot:     "Installed Versions",
	HomeQuickInstallLoader: "Install Version",
	HomeStatInstances:      "Directories",
	HomeStatVersions:       "Versions",
	HomeStatMods:           "Active Mods",
	HomeStatAccount:        "Player",
	HomeInstancesTitle:     "Game Directories",
	HomeInstancesSubtitle:  "",
	HomeAddFolderBtn:       "Add Folder",
	HomeEmptyTitle:         "No Game Instances",
	HomeEmptyDesc:          "Add a local folder, install a new version, or drop a modpack archive here",
	HomeBadgeActive:        "Active",
	HomeBadgeUnavailable:   "Missing Path",
	HomeBadgeRunning:       "Running",
	HomeBadgeCustomJava:    "Custom Java",
	HomeBadgeCustomJVM:     "Custom JVM",
	HomeUseInstanceBtn:     "Select",
	HomeActiveInstanceBtn:  "Selected",
	HomePortalTitle:        "Community Modpacks & Mods",
	HomePortalSubtitle:     "Browse and install curated projects directly from Modrinth and CurseForge",
	HomeModrinthDesc:       "Open-source mods and lightweight modpacks",
	HomeCurseForgeDesc:     "Massive modpacks and community classics",
	HomeActiveTaskTitle:    "Background Task Running",
	HomeViewTasksBtn:       "View Tasks",

	ModsKicker:           "MODS · RESOURCE PACKS · SHADERS",
	ModsTitle:            "Mod Manager",
	ModsDesc:             "Manage isolated Mods (.jar), Resource Packs (.zip), and Shaders (.zip) for the active game version.",
	ModsTabMods:          "Mods",
	ModsTabResourcePack:  "Resource Packs",
	ModsTabShaders:       "Shaders",
	ModsCheckUpdatesBtn:  "Check Updates",
	ModsImportJarBtn:     "Import JARs",
	ModsImportPackBtn:    "Import Local (.zip)",
	ModsExploreMoreBtn:   "Browse More",
	ModsEmptyRootTitle:   "Select or Install a Game Version First",
	ModsEmptyRootDesc:    "Choose a game directory and version from Instances to manage its mods, resource packs, and shaders.",
	ModsContextRoot:      "Game Root",
	ModsContextVersion:   "Active Version",
	ModsSearchHint:       "Filter by name, filename, or ID…",
	ModsSearchAria:       "Search Installed Mods",
	ModsStatTotal:        "Total",
	ModsStatEnabled:      "Enabled",
	ModsStatUpdatable:    "Updates",
	ModsLoading:          "Scanning version directory…",
	ModsEmptyListTitle:   "No Mods Installed in This Version",
	ModsEmptyListDesc:    "Click \"Import JARs\" (or drag & drop .jar files here), or visit \"Discover\" to download mods.",
	ModsEmptyRPTitle:     "No Resource Packs Installed in This Version",
	ModsEmptyRPDesc:      "Click \"Import Local (.zip)\" to add resource packs, or visit \"Discover → Resource Packs\" to download.",
	ModsEmptyShaderTitle: "No Shader Packs Installed in This Version",
	ModsEmptyShaderDesc:  "Click \"Import Local (.zip)\" to add shader packs, or visit \"Discover → Shaders\" to download.",
	ModsTogglePrefix:     "Toggle ",
	ModsUpdateBtn:        "Update",
	ModsRemoveBtn:        "Remove",
	ModsPackAuditTitle:   "Modpack Integrity Check",
	ModsPackAuditDesc:    "Verify SHA-1 checksums of managed modpack files and check for newer releases",
	ModsPackVerifyBtn:    "Verify Modpack",
	ModsCheckedNotice:    "Mod update check completed",
	ModsUpdateQueued:     "Mod update queued in Tasks",

	DiscoverKicker:         "COMMUNITY MARKETPLACE",
	DiscoverTitle:          "Resource Explorer",
	DiscoverDesc:           "Search and install Modpacks, Mods, Resource Packs, and Shaders from Modrinth and CurseForge.",
	DiscoverTypeModpack:    "Modpacks",
	DiscoverTypeMod:        "Mods",
	DiscoverTypeRP:         "Resource Packs",
	DiscoverTypeShader:     "Shaders",
	DiscoverSearchHint:     "Search by project name or keyword…",
	DiscoverSearchAria:     "Search Resources",
	DiscoverSearchBtn:      "Search",
	DiscoverFilterMCHint:   "Game Version (e.g. 1.21.1)",
	DiscoverFilterMCAria:   "Filter by Minecraft Version",
	DiscoverFilterLoader:   "Filter by Loader",
	DiscoverApplyFilterBtn: "Apply Filters",
	DiscoverSortRelevance:  "Relevance",
	DiscoverSortDownloads:  "Most Downloads",
	DiscoverSortUpdated:    "Recently Updated",
	DiscoverLoading:        "Loading catalog results…",
	DiscoverRetryBtn:       "Retry",
	DiscoverSelectVerBtn:   "Select Version",
	DiscoverPrevPage:       "Previous",
	DiscoverNextPage:       "Next",
	DiscoverEmptyTitle:     "No Matching Projects Found",
	DiscoverEmptyDesc:      "Try broadening your search terms, clearing the version filter, or switching providers.",
	DiscoverIdleTitle:      "Start Exploring",
	DiscoverIdleDesc:       "Pick a platform and type, enter keywords, then press Enter or Search.",

	WardrobeKicker:           "SKIN & CAPE STUDIO",
	WardrobeTitle:            "Character Wardrobe",
	WardrobeDesc:             "Interactive 3D software-rendered skin and cape preview with drag-to-orbit and texture management.",
	WardrobeRefreshBtn:       "Refresh",
	WardrobeNoAccountTitle:   "No Player Account Selected",
	WardrobeNoAccountDesc:    "Click the \"Add Account\" card in the top-left sidebar to sign in with an Offline, Microsoft, or Yggdrasil account.",
	WardrobeAddAccountBtn:    "Add Account",
	WardrobeOrbitAria:        "Rotate Skin Preview",
	WardrobeOrbitHint:        "Drag character to rotate camera · Use slider below to adjust zoom",
	WardrobeZoomLabel:        "Zoom",
	WardrobeZoomAria:         "Skin Zoom",
	WardrobeFrontBtn:         "Front",
	WardrobeBackBtn:          "Back",
	WardrobeAutoSpin:         "Auto Rotate",
	WardrobeSkinTitle:        "Character Skin",
	WardrobeSkinDesc:         "Supports standard 64×64 dual-layer and legacy 64×32 PNG skin textures.",
	WardrobeModelLabel:       "Arm Model",
	WardrobeModelSteve:       "Classic Steve (4px Wide)",
	WardrobeModelAlex:        "Slim Alex (3px Slim)",
	WardrobeUploadSkinBtn:    "Upload Skin",
	WardrobeResetSkinBtn:     "Reset Default",
	WardrobeCapeTitle:        "Cape & Elytra",
	WardrobeCapeMSDesc:       "Equip an official cape owned by this Microsoft account.",
	WardrobeCapeLocalDesc:    "Upload a local PNG cape texture for preview and supported auth servers.",
	WardrobeCapeNone:         "No Cape",
	WardrobeApplyCapeBtn:     "Apply Cape",
	WardrobeUploadCapeBtn:    "Upload Cape",
	WardrobeRemoveCapeBtn:    "Remove Cape",
	WardrobeOfflineNoteTitle: "Offline Texture Storage",
	WardrobeOfflineNote1:     "Offline skins and capes are saved locally for 3D preview inside the launcher.",
	WardrobeOfflineNote2:     "To render custom skins in offline singleplayer or LAN worlds, use a skin mod such as CustomSkinLoader.",
	WardrobeYggNoUpload:      "This Yggdrasil authentication server does not allow direct texture uploads.",
	WardrobeTextureSaved:     "Appearance texture saved",

	TasksKicker:      "DOWNLOADS & ACTIVITY",
	TasksTitle:       "Tasks & Activity",
	TasksDesc:        "Track game installations, modloader builds, modpack imports, and per-file download progress.",
	TasksEmptyTitle:  "No Active or Recent Tasks",
	TasksEmptyDesc:   "Detailed file download logs and installation progress will appear here.",
	TasksCancelBtn:   "Cancel",
	TasksLogsBtn:     "Open Active Version Logs",
	TasksItemsHeader: "Downloaded & Processed Items:",

	SettingsKicker:           "PREFERENCES & RUNTIME",
	SettingsTitle:            "Settings",
	SettingsDesc:             "Configure default Java runtime, JVM memory & arguments, interface language, and built-in services",
	SettingsRuntimePanel:     "Runtime Environment Panel",
	SettingsRuntimeTitle:     "Runtime Environment",
	SettingsRuntimeDesc:      "Configure global Java runtime, maximum heap memory, and default JVM flags",
	SettingsJavaLabel:        "Java Runtime",
	SettingsJavaAria:         "Java Runtime Selection",
	SettingsJavaAuto:         "Auto Select",
	SettingsJavaCustom:       "Custom Java",
	SettingsJavaBrowseBtn:    "Browse",
	SettingsJavaScanBtn:      "Detect",
	SettingsJavaAutoHint:     "Automatically selects a compatible Java runtime for each Minecraft version.",
	SettingsJavaScanningHint: "Scanning local system for installed Java runtimes…",
	SettingsMemoryLabel:      "Max Memory",
	SettingsMemorySliderAria: "Maximum Memory Slider",
	SettingsMemoryInputAria:  "Maximum Memory Input",
	SettingsJVMArgsLabel:     "JVM Arguments",
	SettingsJVMArgsAria:      "Global JVM Arguments Input",
	SettingsJVMArgsHint:      "Leave blank for defaults, or pick a preset below",
	JVMPresetTitle:           "Presets:",
	JVMPresetG1:              "G1GC Balanced (Recommended)",
	JVMPresetZGC:             "ZGC Low-Latency (Java 17+)",
	JVMPresetCompact:         "Compact Low-RAM (2–4G)",
	JVMPresetClear:           "Clear",
	JVMAdviceDefault:         "Tip: Use G1GC Balanced for most profiles; use ZGC Low-Latency on Java 17/21+ with 6G+ RAM.",
	JVMAdviceG1:              "G1GC Balanced: caps GC pauses at 50ms and tunes young generation sizing for smooth frame pacing.",
	JVMAdviceZGC:             "ZGC Low-Latency: sub-millisecond GC pauses for Java 17/21+ modpacks with 6G+ RAM.",
	JVMAdviceCompact:         "Compact Low-RAM: enables string deduplication and 16M regions for 2–4G systems.",
	SettingsRootOverride:     "Per-Instance Overrides",
	SettingsRootOverrideDesc: "Instance and version-specific Java, memory, and JVM args override global defaults.",
	SettingsRootOverrideAria: "Per-Instance Overrides Toggle",
	SettingsPrefsPanel:       "Game Versions Panel",
	SettingsPrefsTitle:       "Interface & Version Defaults",
	SettingsLanguageLabel:    "Language / 界面语言",
	SettingsLanguageDesc:     "Switch interface between Simplified Chinese and English.",
	SettingsLanguageAria:     "Interface Language Selection",
	SettingsSnapshotsLabel:   "Show Snapshot Versions",
	SettingsSnapshotsDesc:    "Include weekly snapshots and pre-releases when installing Minecraft.",
	SettingsSnapshotsAria:    "Show Snapshot Versions Toggle",
	SettingsServicePanel:     "Built-in Services Panel",
	SettingsServiceTitle:     "Integrated Services",
	SettingsServiceDesc:      "Built into official releases; no manual API keys required",
	SettingsMSDesc:           "Microsoft OAuth2 Device Code",
	SettingsMRDesc:           "Modrinth mod & modpack catalog",
	SettingsCFDesc:           "CurseForge mod & modpack catalog",
	SettingsServiceConnected: "Connected",
	SettingsServicePending:   "Pending",
	SettingsAboutPanel:       "About MCL Panel",
	SettingsAboutTitle:       "About MCL",
	SettingsAboutDesc:        "Pure Go Native Desktop UI · Zero WebView Overhead",
	SettingsOpenDataBtn:      "Open Data Folder",
	SettingsActionBarAria:    "Settings Action Bar",
	SettingsDefaultHint:      "Changes apply immediately, to newly launched game processes",
	SettingsResetBtn:         "Restore Defaults",
	SettingsResetNotice:      "Defaults restored",

	DialogActiveAria:     "Active Modal",
	DialogCloseBtn:       "Close",
	DialogBrowseBtn:      "Browse",
	DialogBusyHint:       "Processing request, please wait…",
	DialogChooseFolder:   "Select folder",
	DialogTitleAdd:       "Add Game Instance Folder",
	DialogTitleRoot:      "Settings",
	DialogTitleRemRoot:   "Unregister Instance",
	DialogTitleRemMod:    "Remove Resource File",
	DialogTitleAccount:   "Manage Accounts",
	DialogTitleImport:    "Import Modpack",
	DialogTitleInstall:   "Install Minecraft & Mod Loader Wizard",
	DialogTitleProject:   "Select Project Version",
	DialogTitlePackAudit: "Modpack Verification Report",
	DialogTitleLogs:      "Game Output Logs",
	DialogTitleJava:      "Java Runtime Scanner",

	AddRootDesc:            "Select a game root folder, a .minecraft folder, a versions folder, or a specific version folder inside versions/.",
	AddRootFolderLabel:     "Instance Folder",
	AddRootNameLabel:       "Instance Name",
	AddRootNamePlaceholder: "Leave blank to use folder name",
	AddRootConfirmBtn:      "Add Instance",
	AddRootSuccess:         "Game instance added",
	RootSectionInstance:    "Game Root & Default Parameters",
	RootSectionVersion:     "Per-Version Runtime & JVM Overrides",
	RootSectionVersionDesc: "Configure dedicated Java runtime, maximum heap memory, and custom JVM flags for individual versions in versions/.",
	RootCfgVersionLabel:    "Target Game Version",
	RootCfgVerJavaLabel:    "Version Java Runtime",
	RootCfgVerJavaFollow:   "Follow Instance / Global Default",
	RootCfgVerMemLabel:     "Version Max Memory (MiB)",
	RootCfgVerMemFollow:    "Default",
	MemModeAuto:            "Auto",
	MemModeManual:          "Manual",
	MemAutoValue:           "Auto (≈ %.1f GB)",
	MemAutoExplain:         "Computed from total and free RAM, game version, loader and mod count, keeping at least 2.5 GB (or 25%) for the system.",
	RootCfgVerMemHint:      "Set to 0 to inherit instance or global memory",
	RootCfgVerJVMLabel:     "Version JVM Arguments",
	RootCfgVerJVMHint:      "e.g. -XX:+UseG1GC",
	RootCfgJVMPresetG1Btn:  "+ Insert G1GC Preset",
	RootCfgJVMClearBtn:     "Clear Args",
	RootCfgJavaLabel:       "Instance Default Java Path",
	RootCfgJavaHint:        "Leave blank to use global Java runtime",
	RootCfgMemLabel:        "Instance Default Memory (MiB)",
	RootCfgMemHint:         "Set to 0 to use global memory limit",
	RootCfgSaveBtn:         "Save",
	RootCfgSaved:           "Instance and version settings saved",
	RootCfgMemErr:          "Memory must be 0 or an integer between 512 and 65536 MiB",
	RootCfgInstallVerBtn:   "Install New Version / Loader",
	RootCfgUnregisterBtn:   "Unregister Instance",
	RemRootConfirmTitle:    "Unregister instance \"%s\" from MCL?",
	RemRootSafeHint:        "This only removes the entry from the launcher. All worlds, mods, and files remain untouched on disk.",
	RemRootConfirmBtn:      "Confirm Unregister",
	RemRootSuccess:         "Instance unregistered",
	RemModConfirmTitle:     "Remove file: %s",
	RemModBackupHint:       "The file will be backed up to .mcl/backups inside this instance folder before removal.",
	RemModConfirmBtn:       "Confirm Remove",

	AccSavedSection:      "Saved Accounts",
	AccAddSection:        "Add New Account",
	AccTabOffline:        "Offline",
	AccTabMicrosoft:      "Microsoft",
	AccTabThirdParty:     "Third-Party",
	AccKindOffline:       "Offline Account",
	AccKindMicrosoft:     "Microsoft Official",
	AccKindThirdParty:    "Yggdrasil Auth",
	AccUseBtn:            "Switch",
	AccRemoveBtn:         "Remove account",
	AccRemoveConfirm:     "Confirm remove",
	VersionDialogTitle:   "Version Settings",
	VersionSnapshotWord:  "Snapshot",
	VersionNameLabel:     "Display name",
	VersionNameHint:      "Leave empty for the default name",
	AccOfflineDesc:       "Play locally or on offline-mode servers using a standard OfflinePlayer UUID without a password.",
	AccOfflineNameLabel:  "Player Name",
	AccOfflineNameHint:   "3–16 alphanumeric characters or underscores",
	AccOfflineAddBtn:     "Add Offline Account",
	AccOfflineAdded:      "Offline account added",
	AccMSDesc:            "Sign in securely via Microsoft OAuth2 Device Code in your default system browser.",
	AccMSDisabled1:       "Microsoft sign-in is not configured in this development build.",
	AccMSDisabled2:       "Official release builds embed the publisher Client ID automatically.",
	AccMSStartBtn:        "Sign in with Microsoft",
	AccMSCodePrompt:      "Open the verification page in your browser and enter this one-time code:",
	AccMSCopyCodeBtn:     "Copy Code",
	AccMSOpenBrowserBtn:  "Open Verification Page",
	AccMSWaiting:         "Waiting for browser authorization… Your account will be linked automatically.",
	AccMSExpired:         "Verification code expired. Please start sign-in again.",
	AccMSAdded:           "Microsoft account added",
	AccThirdDesc:         "Sign in with a Yggdrasil / authlib-injector compatible authentication server.",
	AccThirdServerLabel:  "Auth Server URL",
	AccThirdUserLabel:    "Username / Email",
	AccThirdUserHint:     "Account email or username",
	AccThirdPassLabel:    "Password",
	AccThirdPassHint:     "Enter account password",
	AccThirdProfileLabel: "Select Character Profile",
	AccThirdLoginBtn:     "Sign In & Add",
	AccThirdAdded:        "Third-party account added",

	ImportDesc:            "Import local Modrinth (.mrpack) , CurseForge (.zip) modpacks or MultiMC / Prism instance zips with automatic version and loader detection.",
	ImportRecognizedTitle: "Recognized Modpack Manifest:",
	ImportFileLabel:       "Modpack Archive",
	ImportFilePlaceholder: ".mrpack or .zip file",
	ImportFileHint:        "Browse, or drop the file anywhere on the window (Modrinth, CurseForge, MultiMC/Prism)",
	ImportParentLabel:     "Parent Destination Folder",
	ImportNameLabel:       "New Instance Name",
	ImportNamePlaceholder: "Dedicated folder name for this modpack",
	ImportNote:            "Creates an isolated instance directory, installs the required Minecraft & mod loader, and downloads all pack files with live per-file progress in Tasks.",
	ImportConfirmBtn:      "Import Modpack",

	InstallDesc:               "Step-by-step guided installation for official Minecraft releases and Fabric, Forge, NeoForge, or Quilt mod loaders.",
	InstallStep1Title:         "Step 1 · Choose Target Location",
	InstallStep2Title:         "Step 2 · Select Official Minecraft Version",
	InstallStep3Title:         "Step 3 · Choose Mod Loader & Version",
	InstallTargetLabel:        "Installation Target",
	InstallTargetCurrent:      "Current Instance",
	InstallTargetNew:          "New Instance Folder",
	InstallParentLabel:        "Parent Destination Folder",
	InstallNameLabel:          "New Instance Name",
	InstallNamePlaceholder:    "e.g. Survival-1.21-Fabric",
	InstallMCLabel:            "Official Minecraft Version",
	InstallRefreshBtn:         "Refresh List",
	InstallKindAria:           "Minecraft Version Category",
	InstallKindAll:            "All Versions",
	InstallKindRelease:        "Releases",
	InstallKindSnapshot:       "Snapshots",
	InstallKindBeta:           "Old Beta",
	InstallKindAlpha:          "Old Alpha",
	InstallKindOther:          "Other",
	InstallQueryAria:          "Search Minecraft Version",
	InstallQueryHint:          "Search version ID, e.g. 1.21, 24w, b1.7.3…",
	InstallVersionAria:        "Minecraft Version Selection",
	InstallFetchingManifest:   "Fetching official Mojang version manifest…",
	InstallNoMatch:            "No matching versions found. Try switching categories or clearing the search box.",
	InstallLoaderLabel:        "Mod Loader",
	InstallLoaderAria:         "Mod Loader Selection",
	InstallLoaderVanillaSub:   "Vanilla",
	CardLoaderVersion:         "Loader",
	InstallLoaderFabricSub:    "Lightweight modern loader",
	InstallLoaderForgeSub:     "Classic mod ecosystem",
	InstallLoaderNeoSub:       "Modern loader for 1.20.1+",
	InstallLoaderQuiltSub:     "Community fork of Fabric",
	InstallLoaderVerLabel:     "Select Loader Version",
	InstallFetchingLoader:     "Fetching compatible loader versions…",
	InstallNoLoaderVer:        "No compatible loader builds found for this Minecraft version. Choose Vanilla or another loader.",
	InstallConfirmBtn:         "Start Installation",
	InstallTaskCreated:        "Installation task started",
	ImportDoneNotice:          "Import complete",
	ImportSizeNote:            "The full game (client, libraries and assets, about %s) is downloaded during import, so it is ready to launch afterwards.",
	ImportSourceMultiMC:       "MultiMC / Prism instance",
	TaskLaunchBtn:             "Launch",
	TaskOpenDirBtn:            "Open Folder",
	DropNeedRoot:              "Add or install a game directory first, then drop resource / shader packs",
	PackSecTitle:              "Modpack",
	PackSecCurrent:            "Current",
	PackSecLatest:             "Latest",
	PackSecUpToDate:           "Up to date",
	PackSecNotChecked:         "Not checked yet",
	PackSecNoOrigin:           "No online source found — use Update from File",
	PackSecRunning:            "Can't update the modpack while the game is running",
	PackCheckBtn:              "Check for Updates",
	PackUpdateToPrefix:        "Update to ",
	PackFromFileBtn:           "Update from File",
	PackRollbackPrefix:        "Roll Back to ",
	PackUpdateBadge:           "Update",
	PackUpToDateNotice:        "Already up to date",
	PackFoundNotice:           "Update available: ",
	DialogTitlePackUpdate:     "Update Modpack",
	DialogTitlePackChanges:    "Update Changes",
	DialogTitlePackDrop:       "Modpack Already Installed",
	PackDropDesc:              "The current directory “%s” already has this modpack (%s). Update it, or import as a new directory?",
	PackDropUpdateBtn:         "Update This Directory",
	PackDropImportBtn:         "Import as New Directory",
	PackChAdded:               "Added",
	PackChRemoved:             "Removed",
	PackChUpdated:             "Updated",
	PackChOverwritten:         "Overwritten (Changed by You)",
	PackReasonUser:            "your own file at this path",
	PackReasonModified:        "changed by you",
	PackReasonMissing:         "restored",
	PackKeepDisabled:          "stays disabled",
	PackDownloadPrefix:        "Download ",
	PackChangelog:             "Changelog",
	PackNoChangelog:           "No changelog for this version",
	PackGameChange:            "Game version changes: ",
	PackApplyBtn:              "Update Modpack",
	PackCancelBtn:             "Cancel",
	PackBackupNote:            "Everything that will be overwritten or deleted is backed up first (last 3 kept), so you can roll back at any time. Mods and worlds you added yourself are left alone.",
	PackUpdateDone:            "Modpack updated",
	PackRollbackDone:          "Modpack rolled back",
	TaskChangesBtn:            "View Changes",
	TaskRollbackBtn:           "Roll Back",
	PackShowList:              "Show",
	PackHideList:              "Hide",
	PackNothing:               "No files change",
	PackUnchangedPrefix:       "Unchanged ",
	PackCheckingNotice:        "Preparing update…",
	PackDoneBadge:             "Update Complete",
	PackChManual:              "Manual Download Needed",
	BackLabel:                 "Back",
	PackReasonRenamed:         "renamed",
	PackReasonKeptOld:         "old version kept",
	PackGameWarn:              "The pack will launch with %s. The old version %s stays in the versions folder and works again after a rollback. Changing the game or loader version may make worlds impossible to downgrade — back up the saves folder first.",
	PackOpenDirBtn:            "Open Game Folder",
	SettingsPackAutoCheck:     "Check for Modpack Updates at Startup",
	SettingsPackAutoCheckDesc: "At most once every 24 hours; stays silent when offline",
	SettingsPackBeta:          "Include Beta Modpack Versions",
	SettingsPackBetaDesc:      "Also consider Beta / Alpha versions when checking",
	SettingsPackAutoCheckAria: "Auto-check modpack updates",
	SettingsPackBetaAria:      "Include beta versions",
	InstallLaunchPreparing:    "Preparing game launch; check Tasks for live progress",

	ProjectWebsiteBtn:       "Project Page",
	ProjectNoVersions:       "No compatible release files found for the current filters.",
	ProjectVersionLabel:     "Select Release Version (Scrollable & Filterable)",
	ProjectFilterHint:       "Filter by version number, Minecraft version (1.21.1), or loader (Fabric)…",
	ProjectFilterAria:       "Filter Project Releases",
	ProjectNeedRootHint:     "Please add and select a game instance before installing.",
	ProjectTargetPrefix:     "Install into active version: ",
	ProjectInstallModBtn:    "Install Mod",
	ProjectInstallRPBtn:     "Install Resource Pack",
	ProjectInstallShaderBtn: "Install Shader Pack",
	ProjectInstallPackBtn:   "Download & Install Modpack",
	PackUpdateFoundPrefix:   "Update Available: ",
	PackUpdateInstallBtn:    "Install Update to New Instance",
	PackManualPrefix:        "Manual download required: ",
	PackManualOpenBtn:       "Open Download Page",
	LogsRefreshBtn:          "Refresh Logs",
	LogsCopyBtn:             "Copy Logs",
	LogsEmpty:               "No game output logs recorded for this instance yet.",
	LogsAreaAria:            "Game Logs",
	JavaQuickScanBtn:        "Quick Scan",
	JavaDeepScanBtn:         "Full Disk Scan",
	JavaStopScanBtn:         "Stop Scan",
	JavaScanHint:            "Quick Scan inspects PATH, registry, standard JDK directories, and Minecraft runtimes. Full Disk Scan searches fixed and removable drives for portable JDKs.",
	JavaFilterAria:          "Filter Java",
	JavaFilterPlaceholder:   "Filter by version, vendor (Adoptium, Zulu…), or path…",
	JavaUseAutoBtn:          "Use Auto Select",
	JavaUseBtn:              "Use",
	JavaEmptyHint:           "No Java runtimes detected yet. Run a Full Disk Scan or click Browse to pick java.exe.",

	LoaderVanilla:  "Vanilla",
	LoaderAll:      "All Loaders",
	LoaderFabric:   "Fabric",
	LoaderQuilt:    "Quilt",
	LoaderForge:    "Forge",
	LoaderNeoForge: "NeoForge",
}

var zhCategories = map[string]string{
	// Mods & Modpacks
	"adventure":      "冒险探索",
	"magic":          "魔法奥术",
	"technology":     "科技工业",
	"optimization":   "性能优化",
	"transportation": "运输传送",
	"utility":        "实用工具",
	"worldgen":       "世界生成",
	"storage":        "仓储物流",
	"equipment":      "武器装备",
	"food":           "农业美食",
	"decoration":     "建筑装饰",
	"library":        "前置核心",
	"mobs":           "生物群系",
	"cursed":         "趣味恶搞",
	"economy":        "经济贸易",
	"social":         "社交联机",
	"game-mechanics": "机制调整",
	"management":     "服主管理",
	"minigame":       "小游戏",
	"multiplayer":    "多人联机",
	"kitchen-sink":   "全能大型包",
	"lightweight":    "轻量流畅",
	"quests":         "任务引导",
	"challenging":    "硬核生存",
	"combat":         "战斗革新",
	// Shaders & Resource Packs
	"realistic":        "写实风格",
	"semi-realistic":   "半写实",
	"atmosphere":       "氛围光影",
	"fantasy":          "奇幻风格",
	"simplistic":       "简约风格",
	"cartoon":          "卡通渲染",
	"cel-shading":      "描边渲染",
	"tweaks":           "原版微调",
	"vanilla-like":     "原版增强",
	"pbr":              "PBR 材质",
	"foliage":          "动态植物",
	"gui":              "界面美化",
	"bloom":            "泛光特效",
	"colored-lighting": "彩色光照",
	"path-tracing":     "路径追踪",
	"reflections":      "水面反射",
	"shadows":          "动态阴影",
	"water":            "水体优化",
	"clouds":           "体积云",
	"core-shaders":     "核心着色",
	"screenshot":       "摄影专用",
	"high":             "高配画质",
	"medium":           "中配均衡",
	"low":              "低配流畅",
	"potato":           "核显流畅",
	"8x-":              "8x 像素",
	"16x":              "16x 原版级",
	"32x":              "32x 高清",
	"64x":              "64x 精细",
	"128x":             "128x 超清",
	"256x":             "256x 极清",
	"512x+":            "512x+ 影视级",
	"audio":            "音效音乐",
	"blocks":           "方块纹理",
	"entities":         "生物模型",
	"environment":      "环境天空",
	"fonts":            "字体美化",
	"items":            "物品纹理",
	"locale":           "中文汉化",
	"models":           "3D 模型",
	"ores":             "矿物高亮",
	"themed":           "主题材质",
	"datapack":         "数据包",
	"modded":           "模组兼容",
	// Loaders & Shader Loaders
	"fabric":   "Fabric",
	"forge":    "Forge",
	"neoforge": "NeoForge",
	"quilt":    "Quilt",
	"iris":     "Iris",
	"optifine": "OptiFine",
	"canvas":   "Canvas",
}

func (a *App) translateCategory(cat string) string {
	raw := strings.TrimSpace(cat)
	if raw == "" {
		return ""
	}
	if a.isEN() {
		return raw
	}
	key := strings.ToLower(raw)
	if v, ok := zhCategories[key]; ok {
		return v
	}
	return raw
}

func (a *App) lang() string {
	if v := launcher.NormalizeLanguage(a.settings.Language); v != "" {
		return v
	}
	return LangZH
}

func (a *App) isEN() bool {
	return a.lang() == LangEN
}

func (a *App) tr() *Copy {
	if a.isEN() {
		return &enCopy
	}
	return &zhCopy
}

func (a *App) setLanguage(lang string) {
	norm := launcher.NormalizeLanguage(lang)
	if a.settings.Language == norm && a.snapshot.Settings.Language == norm {
		return
	}
	a.settings.Language = norm
	a.snapshot.Settings.Language = norm
	a.normalizeStateValues()
	go func() {
		_ = a.service.SaveSettings(a.settings)
		snap := a.service.Snapshot()
		a.post(func() {
			a.snapshot = snap
		})
	}()
}

func (a *App) toggleLanguage() {
	if a.isEN() {
		a.setLanguage(LangZH)
	} else {
		a.setLanguage(LangEN)
	}
}

func (a *App) pageTitle(page string) string {
	t := a.tr()
	switch page {
	case "home":
		return t.NavHome
	case "mods":
		return t.NavMods
	case "discover":
		return t.NavDiscover
	case "wardrobe":
		return t.NavWardrobe
	case "tasks":
		return t.NavTasks
	case "settings":
		return t.NavSettings
	}
	return t.NavHome
}

func (a *App) dialogTitle(dialog string) string {
	t := a.tr()
	switch dialog {
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
	case "account":
		return t.DialogTitleAccount
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

func (a *App) accountKindText(kind string) string {
	t := a.tr()
	switch kind {
	case "microsoft":
		return t.AccKindMicrosoft
	case "yggdrasil":
		return t.AccKindThirdParty
	}
	return t.AccKindOffline
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

func (a *App) modStatusText(status string) string {
	if a.isEN() {
		switch status {
		case "unchecked":
			return "Unchecked"
		case "invalid":
			return "Invalid Metadata"
		case "current", "latest":
			return "Up to Date"
		case "unknown":
			return "Local / Unlisted"
		case "disabled":
			return "Disabled"
		case "update":
			return "Update Available"
		}
		return status
	}
	switch status {
	case "unchecked":
		return "尚未检测"
	case "invalid":
		return "元数据异常"
	case "current", "latest":
		return "已是最新"
	case "unknown":
		return "本地 / 未收录"
	case "disabled":
		return "已停用"
	case "update":
		return "有更新"
	}
	return status
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
		return "Quick scanning PATH, registry, and standard JDK directories"
	case "全盘扫描固定磁盘与可移动磁盘":
		return "Full disk scan across fixed and removable drives"
	case "扫描已停止":
		return "Scan stopped"
	case "快速扫描已结束，可继续全盘扫描":
		return "Quick scan finished; run Full Disk Scan for deep search"
	}
	return phase
}

func (a *App) formatVersionsAndMods(versions, mods int) string {
	if a.isEN() {
		return fmt.Sprintf("%d versions  ·  %d mods", versions, mods)
	}
	return fmt.Sprintf("%d 个版本  ·  %d 个模组", versions, mods)
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

func (a *App) formatDownloads(downloads int64) string {
	if a.isEN() {
		return compact(downloads) + " downloads"
	}
	return compact(downloads) + " 次下载"
}

func (a *App) formatPagination(page, totalPages int) string {
	if totalPages < 1 {
		totalPages = 1
	}
	if a.isEN() {
		return fmt.Sprintf("Page %d / %d", page, totalPages)
	}
	return fmt.Sprintf("第 %d / %d 页", page, totalPages)
}

func (a *App) formatSystemMemory(totalMB, availMB int) string {
	if a.isEN() {
		return fmt.Sprintf("System %.1f GB · Available %.1f GB", float64(totalMB)/1024, float64(availMB)/1024)
	}
	return fmt.Sprintf("系统 %.1f GB · 可用 %.1f GB", float64(totalMB)/1024, float64(availMB)/1024)
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
		return fmt.Sprintf("%s · Checked %d folders · Found %d Java", p, visited, found)
	}
	return fmt.Sprintf("%s · 已检查 %d 个目录 · 找到 %d 个 Java", p, visited, found)
}

func (a *App) formatVersionTotal(total int) string {
	if a.isEN() {
		return fmt.Sprintf("%d versions total", total)
	}
	return fmt.Sprintf("共 %d 个版本", total)
}

func (a *App) formatFilteredVersionSummary(detail string, count int) string {
	if a.isEN() {
		return fmt.Sprintf("%s · %d matching versions", detail, count)
	}
	return fmt.Sprintf("%s · 当前筛选 %d 个版本", detail, count)
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
