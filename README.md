# YoloMCL

YoloMCL（应用内简称 MCL）是一个用 Go 编写的 Minecraft: Java 版启动器。界面基于 [MyGo](https://github.com/egoist/mygo) 的原生 UI（`mygo/ui`），没有 HTML、WebView 或本地网页服务，支持 Windows、macOS 和 Linux。

## 功能

- **跨平台原生界面**：标题栏按平台适配（macOS 使用系统红绿灯，位于左侧；Windows/Linux 标题栏与顶部导航合并）；任意窗口尺寸与 DPI 下不失真缩放；自然的页面动效与悬浮滚动条；中英双语（`zh-CN` / `en-US`）切换。
- **内置字体**：附带 JetBrains Mono，中文按系统回退到对应 CJK 字体。
- **游戏目录**：按平台自动识别默认 `.minecraft` 目录，可添加、切换、改名多个目录，每个目录可单独设置 Java 与内存。
- **版本安装**：从 Mojang 获取完整版本清单（正式版、快照/预发布、历史 Beta/Alpha），按官方哈希校验；支持 Fabric、Quilt 以及 Forge / NeoForge 官方安装器。默认版本名包含版本号与加载器版本，导入的整合包使用整合包自身名称；版本名右上角以小标签显示 MC 版本与加载器。
- **Java**：检测 PATH、JAVA_HOME 及常见安装位置的 Java，可全盘扫描便携版，并可下载 Temurin 运行时。
- **内存**：读取系统物理内存，支持自动分配内存，也可手动设置。
- **资源与整合包**：在 Modrinth / CurseForge 搜索、下载模组与整合包并处理必需依赖；导入 `.mrpack` 与 CurseForge `.zip` 整合包，支持拖放导入。
- **整合包更新**：检测新版本并展示改动列表；更新时保留玩家自己添加的模组和存档，整合包自带的配置文件始终覆盖为新版本，被覆盖的文件会先备份。
- **模组管理**：导入 JAR、启用/停用、移除前备份。
- **账号**：离线账号；Microsoft 正版登录（设备代码流程，Xbox / XSTS / Minecraft 档案，令牌刷新）；第三方 Yggdrasil / authlib-injector 认证服务器。
- **角色外观**：皮肤预览，上传皮肤与切换披风。

## 构建

需要 Go 1.27.1（见 `go.mod`），无需前端工具链。

```sh
go run .          # 开发运行
go test ./...
go vet ./...
./scripts/build.sh            # macOS / Linux（支持 GOOS/GOARCH 交叉编译）
```

```powershell
.\scripts\build.ps1           # Windows
```

### 发布用密钥

CurseForge API Key（以及即将接入的 Microsoft Client ID）**在编译时注入**，不写在源码里。构建脚本从环境变量或项目根目录的本地 `.env` 读取：

```sh
cp .env.example .env   # 填入 CURSEFORGE_API_KEY= 与 MS_CLIENT_ID=
```

- `.env` 已在 `.gitignore` 中，**绝对不要提交**；仓库里只有空模板 `.env.example`。
- 运行时如果设置了 `CURSEFORGE_API_KEY` 环境变量或 `.env`，优先使用它。
- 没有 key 也能编译，但 CurseForge 功能不可用。
- **Fork 本项目请使用你自己的 CurseForge API Key 和 Azure 应用 Client ID**，不要复用官方构建的凭据。

## 第三方声明

所用第三方组件与字体的许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

## 许可证

[MIT](LICENSE) © 2026 Viking602

---

## English

YoloMCL (shown as MCL in the app) is a cross-platform (Windows / macOS / Linux) Minecraft: Java Edition launcher written in Go with the native MyGo UI and a platform-aware title bar. It installs vanilla, Fabric, Quilt, Forge and NeoForge versions; imports Modrinth / CurseForge modpacks (including drag-and-drop) and updates them while keeping user mods and saves (pack configs are overwritten, with a backup); supports offline, Microsoft and Yggdrasil/authlib-injector accounts; auto-allocates memory; and bundles JetBrains Mono. Build with Go 1.27.1 via `scripts/build.sh` or `scripts/build.ps1`. The CurseForge API key (and Microsoft client ID) are injected at build time from the environment or a local `.env` that must never be committed — see `.env.example`. Forks must use their own CurseForge key and Azure client ID. Licensed under MIT.
