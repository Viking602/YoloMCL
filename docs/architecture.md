# 项目架构

目录采用 [Go 官方模块布局建议](https://go.dev/doc/modules/layout)。启动器主入口保留在根目录，
使 `go run .`、`go build .` 兼容；辅助命令统一放在 `cmd/`。不为了目录层数创建通用 `utils`
或为每个界面建立一个包，只有职责和依赖边界独立时才分包。

```text
main.go                         原生应用启动、命令行参数和首帧协调
cmd/
  build/                        跨平台构建入口
  release/                      按 tag 打包、签名和校验六平台发布产物
  publish-secrets/              通过 stdin 将发行配置加密上传到 GitHub Secrets
  check/                        格式、包边界、静态检查、测试与交叉编译
  catalog-relay/                可选的发行方资源代理服务
internal/
  nativeui/                     原生界面、交互和视图状态
  launcher/                     Minecraft 业务、实例、文件与后台任务
  distribution/                 发行方身份和资源服务配置
  dotenv/                       仅解析调用方指定的 .env 字段
  buildconfig/                  构建时配置优先级和编译参数
  buildinfo/                    只读版本、提交身份和更新公钥（无运行时 I/O）
  release/                      更新归档、Ed25519 签名和完整性验证
  diagnostics/                  显式调用的诊断报告
  catalogrelay/                 代理服务实现
plugins/startup/                本仓库的 MyGo 首帧适配插件
tools/                          独立锁定的开发工具模块
scripts/                        Windows/Unix 构建薄入口和资源维护工具
docs/                           架构、集成说明与设计参考
build/                          发行产物（忽略）
.cache/                         本地临时产物（忽略）
```

## 依赖方向

- `main` 组装 `nativeui`、`launcher`、`startup`，按需调用 `diagnostics`。
- `nativeui` 依赖业务层 `launcher` 和只读构建身份 `buildinfo`，业务层不导入 UI 工具包。
- `launcher` 依赖 `distribution`；`distribution` 依赖无文件发现能力的 `dotenv`。
- `startup` 只依赖 MyGo 和标准库，不反向导入应用业务或视图。
- `cmd/build → buildconfig → distribution / dotenv / buildinfo`；构建工具不进入启动器依赖图。
- `cmd/release → buildconfig / release / buildinfo`，`release` 只依赖 `buildinfo` 和标准库。
- `cmd/catalog-relay → catalogrelay`，资源代理与原生应用互不依赖。
- `cmd/check` 用标准库解析 import，持续约束这些边界；开发工具依赖位于独立模块。

## 业务层按职责归档

| 文件组 | 职责 |
| --- | --- |
| `store.go`、`roots.go`、`inventory.go` | 启动器配置、目录登记、实例清单与缓存 |
| `mods.go`、`modscan.go`、`mod_metadata.go` | 本地模组操作、增量扫描与归档元数据 |
| `mod_install.go`、`mod_updates.go`、`assetpacks.go` | 模组安装/检查，资源包和光影管理 |
| `minecraft.go`、`game_launch.go`、`game_process.go` | 游戏依赖准备、参数和启动、进程与日志 |
| `packupdate*.go` | 更新发现、计划、应用、备份及回滚，按阶段分文件 |
| `java*.go`、`*_windows.go`、`*_other.go` | Java 安装检测和运行时，系统差异隔离 |
| `worlds.go`、`sessions.go`、`playstats.go` | 存档、游戏会话、游玩记录 |
| `serverstatus.go`、`recentserver.go` | 服务器协议及最近连接信息 |

同一 `Service` 的文件仍在同一包中，避免为拆目录导出内部锁、缓存或事务状态。
若以后拆为子包，应先定义明确的数据与生命周期边界，不通过反向依赖保留旧耦合。

## 界面层按职责归档

`app.go` 保留应用状态及生命周期；`selection.go` 负责实例选择和快照；
`dialog_actions.go`、`launch_actions.go`、`drop.go` 分别负责对话框动作、启动动作及拖放。

界面主体按实例、发现、任务、衣柜等功能归档。`dialogs.go` 只负责通用弹窗容器，
账号、运行时配置、导入、安装和备份报告使用各自文件。大型安装对话框进一步分为
版本选择、依赖、目标目录、公共排版。

`i18n_copy.go` 定义文案结构，`i18n_zh.go` / `i18n_en.go` 保存静态文案；
语言选择、状态翻译和分类翻译分别保留在对应功能文件。文案表较长不等于职责混杂。

工作线程只能通过 `App.post` 交付视图结果；实例切换使用世代标记及缓存，过期结果
不能写回新实例。首帧插件和 Java 检测的启动时机保持独立。局部动效继续由内容变化触发。

## 检查边界

`go run ./cmd/check` 是统一入口；CI 在真实目标系统运行测试，`-cross` 在本机验证六个
目标的编译和 vet。静态检查通过不代表所有运行时能力都已经跨平台等价。

当前仍有既有的平台能力限制：自动 Java 下载仅支持 Windows x64；账号凭据加密使用
Windows DPAPI，其他平台实现会明确返回不支持。此次结构整理没有用明文存储替代加密，
也没有把交叉编译通过表述为这些功能已经完成。后续扩展可在对应平台文件中独立实现。
