<p align="center"><img src="logo.png" width="128" alt="YoloMCL"></p>

# YoloMCL

[English (primary)](../README.md) / [下载正式版](https://github.com/Viking602/YoloMCL/releases/latest) / [开发规范](CONTRIBUTING.md) / [项目架构](architecture.md) / [发布与在线更新说明](releases.md)

YoloMCL 是一个用 Go 编写的 Minecraft: Java 版启动器，使用 [MyGo](https://github.com/egoist/mygo) 原生界面（`mygo/ui`），支持 Windows、macOS 和 Linux。

## 界面预览

### 实例概览

![实例概览：已安装内容、最近存档、Java 运行环境与存储占用](screenshots/instance-overview.png)

### 模组管理

![模组管理：搜索、启用与更新筛选，以及单个模组的操作](screenshots/mod-management.png)

### 资源发现

![资源发现：Modrinth 与 CurseForge 整合包浏览、游戏版本筛选和分类](screenshots/resource-discovery.png)

## 功能

- **原生桌面界面**：支持高分屏缩放，内置 JetBrains Mono 并按系统回退中文字体；中英双语默认跟随系统语言，也可在设置中切换。Windows 默认使用软件渲染，可在设置中开启界面硬件加速（重启生效）。
- **目录与实例**：自动识别默认 `.minecraft` 目录，可管理多个游戏目录并分别设置 Java 与内存。支持安装官方正式版、快照、远古版，以及 Fabric、Quilt、Forge、NeoForge。
- **Java 与内存**：自动检测本机与内置 Java 运行时，支持手动选择或下载 Temurin；按物理内存与整合包规模自动分配内存，也可手动调整。
- **资源与整合包**：搜索并安装 Modrinth / CurseForge 的模组、资源包、光影与整合包，自动解析依赖；支持导入、拖放导入、导出 `.mrpack` 与 CurseForge `.zip` 整合包，以及备份式整合包更新与模组管理。
- **存档、日志与服务器**：管理存档与备份、截图、最近多人游戏服务器，以及游戏日志与崩溃分析。
- **账号与外观**：支持离线账号、Microsoft 正版登录与 Yggdrasil / authlib-injector 外置登录；支持皮肤预览、上传与披风切换。
- **签名更新**：通过 GitHub Releases 检查稳定版更新，支持签名最低版本策略；安装包经 Ed25519 验签，下载与重启均需确认。配置方式见[发布说明](releases.md#require-a-security-update)。

在实例的「设置 → 导出整合包」中可导出 Modrinth `.mrpack` 或 CurseForge `.zip`。默认勾选模组、配置与脚本，存档、截图、服务器列表和个人选项按需勾选；不包含账号、运行环境、游戏依赖、日志与缓存。平台已识别文件写入下载引用，本地或修改过的文件放入 `overrides/`，禁用模组保留禁用后缀；保存位置需在游戏目录之外。

## 构建

需要 Go 1.27.3（见 `go.mod`）、GNU Make 和 Bash；检查和签名发布还需要 OpenSSL 3、`tar`、`gzip`、`zip`（Windows 使用系统自带 `tar`）。Windows 请在 Git Bash 中运行并安装 GNU Make。

```sh
make build                  # 开发构建，输出至 .cache/
make check                  # 格式、依赖边界、静态检查、脚本及 Go 测试
make fmt                    # 统一格式并检查 import 边界
make check CHECK_ARGS="--cross"  # 额外验证六个平台目标的编译
```

```sh
make build BUILD_ARGS="--release --output build/YoloMCL.exe"
make build BUILD_ARGS="--goos linux --goarch arm64 --output build/YoloMCL-linux-arm64"
```

开发构建输出至 `.cache/MCL-dev.exe`（Windows）或 `.cache/MCL-dev`（macOS/Linux）。`--release` 会关闭 UI 检查器并要求注入发布配置；正式签名发布使用 `make release TAG=v1.1.0`，要求工作区干净且位于对应标签。交叉编译可通过 `BUILD_ARGS="--goos ... --goarch ..."` 或 `GOOS` / `GOARCH` 指定。

## 项目结构与代码规范

模块路径为 `github.com/Viking602/YoloMCL`：根目录 `main.go` 为唯一入口，`internal/app` 负责启动与运行模式，`internal/nativeui` 负责原生界面与多语言，`internal/launcher` 负责游戏安装、实例、账号与后台任务。包边界与架构细节见 [项目架构](architecture.md)，代码规范与检查项见 [开发规范](CONTRIBUTING.md)。

### 发布用密钥

CurseForge API Key 与 Microsoft Client ID（已通过 Mojang 审核加入允许列表）**在编译时注入**，不写在源码里。构建脚本从环境变量或项目根目录的本地 `.env` 读取：

```sh
cp .env.example .env   # 填入 CURSEFORGE_API_KEY= 与 MS_CLIENT_ID=
```

- `.env` 已在 `.gitignore` 中，**绝对不要提交**；仓库里只有空模板 `.env.example`。
- 构建配置优先级：环境变量 → 仓库根目录 `.env`。兼容 `MCL_CURSEFORGE_API_KEY` 与 `MCL_MICROSOFT_CLIENT_ID` 别名。
- 没有配置密钥也能本地编译；缺少 `CURSEFORGE_API_KEY` 时 CurseForge 不可用，缺少 `MS_CLIENT_ID` 时微软登录不可用，离线与外置登录不受影响。
- **Fork 本项目请使用你自己的 CurseForge API Key 和 Azure 应用 Client ID**，不要复用官方构建的凭据。

## 第三方声明

所用第三方组件与字体的许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

## 许可证

[MIT](../LICENSE) © 2026 Viking602

Detailed English documentation is in [README.md](../README.md).
