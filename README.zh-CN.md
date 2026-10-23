<p align="center"><img src="docs/logo.png" width="128" alt="YoloMCL"></p>

# YoloMCL

[English (primary)](README.md) / [下载正式版](https://github.com/Viking602/YoloMCL/releases/latest) / [发布与在线更新说明](docs/releases.md)

YoloMCL（应用内简称 MCL）是一个用 Go 编写的 Minecraft: Java 版启动器。界面基于 [MyGo](https://github.com/egoist/mygo) 的原生 UI（`mygo/ui`），没有 HTML、WebView 或本地网页服务，支持 Windows、macOS 和 Linux。

## 功能

- **跨平台原生界面**：标题栏按平台适配（macOS 使用系统红绿灯，位于左侧；Windows/Linux 标题栏与顶部导航合并）；任意窗口尺寸与 DPI 下不失真缩放；自然的页面动效与悬浮滚动条；中英双语默认跟随系统语言，也可在设置中手动选择。Windows 读取显示语言，macOS 读取首选语言顺序，Linux 读取 `LANGUAGE`、`LC_ALL`、`LC_MESSAGES`、`LANG`；未匹配到支持的语言时使用 English。
- **内置字体**：附带 JetBrains Mono，中文按系统回退到对应 CJK 字体。
- **游戏目录**：按平台自动识别默认 `.minecraft` 目录，可添加、切换、改名多个目录，每个目录可单独设置 Java 与内存。
- **版本安装**：从 Mojang 获取完整版本清单（正式版、快照/预发布、历史 Beta/Alpha），按官方哈希校验；支持 Fabric、Quilt 以及 Forge / NeoForge 官方安装器。默认版本名包含版本号与加载器版本，导入的整合包使用整合包自身名称；版本名右上角以小标签显示 MC 版本与加载器。
- **Java**：首帧呈现后在后台检测系统 Java 安装目录、JAVA_HOME/PATH、系统登记位置、已记录路径，以及整合包和启动器内的运行时；实例切换与设置页复用本次检测结果，可手动重新扫描、选择或下载 Temurin。Java 检测独立于整合包内容扫描，不遍历整块磁盘。
- **启动**：实例清单持久缓存、后台按需验证；资源、配置和实例清单准备与原生窗口创建并行，实例列表只构建可见行，筛选时统一计算重名信息。通过本仓库的 [MyGo 启动插件](plugins/startup/README.md) 先显示可交互的真实界面，再开始显卡预热、模组读取和缩略图解码，随后切换 GPU。MyGo 依赖源码保持原样。
- **内存**：读取系统物理内存，支持自动分配内存，也可手动设置。
- **资源与整合包**：在 Modrinth / CurseForge 搜索、下载模组与整合包并处理必需依赖；导入 `.mrpack` 与 CurseForge `.zip` 整合包，支持拖放导入。
- **整合包更新**：检测新版本并展示改动列表；更新时保留玩家自己添加的模组和存档，整合包自带的配置文件始终覆盖为新版本，被覆盖的文件会先备份。
- **模组管理**：导入 JAR、启用/停用、移除前备份。
- **账号**：离线账号；Microsoft 正版登录（设备代码流程，Xbox / XSTS / Minecraft 档案，令牌刷新）；第三方 Yggdrasil / authlib-injector 认证服务器。
- **角色外观**：皮肤预览，上传皮肤与切换披风。

## 构建

需要 Go 1.27.3（见 `go.mod`），无需前端工具链。

```sh
go run .          # 开发运行
go test ./...
go vet ./...
go run ./cmd/check           # 格式、依赖边界、静态检查、开发/发布模式测试
go run ./cmd/check -fix      # 先统一格式及 import，再执行检查
bash scripts/build.sh            # macOS / Linux（支持 GOOS/GOARCH 交叉编译）
```

```powershell
.\scripts\build.ps1           # Windows
```

正式发布使用 `bash scripts/build.sh --release [输出路径]` 或
`.\scripts\build.ps1 -Release -Output [输出路径]`。两个脚本都使用 MyGo
公开的 `mygo_noinspector` 构建标签与 `production=1`，去掉 UI 检查器和开发期句柄检查；
开发构建仍保留这些功能。配置适用于 Windows、macOS、Linux，不修改 MyGo 依赖源码。

两个脚本均委托本仓库的 `cmd/build`，统一读取配置及生成编译参数，不再依赖
Python 或分别实现 `.env` 解析。也可在仓库根目录直接执行
`go run ./cmd/build -release -output build/YoloMCL.exe`；直接调用时，交叉编译目标
使用 `-goos linux -goarch arm64`，脚本继续支持 `GOOS` / `GOARCH`。

## 项目结构与代码规范

模块路径为 `github.com/Viking602/YoloMCL`。主程序入口保留在根目录，开发工具和
资源代理在 `cmd/`，业务实现位于 `internal/`，MyGo 适配插件位于 `plugins/`。
详细职责、依赖方向和平台约定见 [项目架构](docs/architecture.md)。

GoLand 自动读取仓库的 `.editorconfig`。所有 Go 源码使用 `gofmt` / `goimports`，
import 按标准库、第三方库、本仓库分组；检查工具版本独立锁定在 `tools/go.mod`，
不会加入启动器运行时依赖。开发及检查方式见 [贡献规范](CONTRIBUTING.md)。

### 发布用密钥

CurseForge API Key 与 Microsoft Client ID（已通过 Mojang 审核加入允许列表）**在编译时注入**，不写在源码里。构建脚本从环境变量或项目根目录的本地 `.env` 读取：

```sh
cp .env.example .env   # 填入 CURSEFORGE_API_KEY= 与 MS_CLIENT_ID=
```

- `.env` 已在 `.gitignore` 中，**绝对不要提交**；仓库里只有空模板 `.env.example`。
- 构建配置优先级：显式传入的 publisher JSON → 环境变量 → 仓库根目录 `.env`；两种构建脚本完全一致。兼容 `MCL_CURSEFORGE_API_KEY` 与 `MCL_MICROSOFT_CLIENT_ID` 别名。
- 运行时如果设置了 `CURSEFORGE_API_KEY` 环境变量或 `.env`，优先使用它。
- 没有 key 也能编译，但 CurseForge 功能不可用；没有 `MS_CLIENT_ID` 时微软账号登录不可用，离线与外置登录不受影响。
- **Fork 本项目请使用你自己的 CurseForge API Key 和 Azure 应用 Client ID**，不要复用官方构建的凭据。

## 第三方声明

所用第三方组件与字体的许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

## 许可证

[MIT](LICENSE) © 2026 Viking602

---

## English

YoloMCL (shown as MCL in the app) is a cross-platform (Windows / macOS / Linux) Minecraft: Java Edition launcher written in Go with the native MyGo UI and a platform-aware title bar. It installs vanilla, Fabric, Quilt, Forge and NeoForge versions; imports Modrinth / CurseForge modpacks (including drag-and-drop) and updates them while keeping user mods and saves (pack configs are overwritten, with a backup); supports offline, Microsoft and Yggdrasil/authlib-injector accounts; auto-allocates memory; and bundles JetBrains Mono. Build with Go 1.27.3 via `scripts/build.sh` or `scripts/build.ps1`; both delegate to `cmd/build`. Run `go run ./cmd/check` for formatting, architecture, static analysis and tests, or add `-cross` to vet and compile all six supported OS/architecture targets. The CurseForge API key (and Microsoft client ID) are injected at build time from the environment or a local `.env` that must never be committed — see `.env.example`. Forks must use their own CurseForge key and Azure client ID. Licensed under MIT.
