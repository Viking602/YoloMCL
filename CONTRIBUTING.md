# 开发规范

## Go 与 GoLand

GoLand 是编辑器，不另外定义 Go 语言规范。本仓库以 [Go 官方约定](https://go.dev/doc/effective_go)
及 [GoLand 的 Go 工具集成](https://www.jetbrains.com/help/go/integration-with-go-tools.html)
为依据，命令行和 IDE 使用相同格式。

- `.editorconfig`：UTF-8、LF、末尾换行、Go 使用 Tab；启用 gofmt、goimports 分组。
- import 分为标准库、第三方、本仓库三组，本仓库前缀为 `github.com/Viking602/YoloMCL`。
- 文件按职责命名，如 `mod_install.go`、`log_parser_test.go`，不使用迭代编号作为职责名称。
- 包使用简短小写名称；导出标识符写用途注释；缩写保持 `ID`、`URL`、`HTTP` 等标准形式。
- 错误必须处理或明确说明忽略原因；需要保留错误链时使用 `%w`。不要在错误中输出凭据。
- 变更业务行为时补充能捕获实际回归的测试；单纯搬移文件不复制测试。

GoLand 打开根目录即可应用 [EditorConfig](https://www.jetbrains.com/help/go/editorconfig.html)。
可在保存操作中启用 Reformat code 和 Optimize imports。`tools/` 是独立的工具模块；
若 IDE 提示，可将其作为第二个 Go 模块加载，不需要提交个人 `go.work` 或 `.idea`。

## 本地检查

从仓库根目录执行，三个桌面平台使用相同命令：

```sh
go run ./cmd/check
go run ./cmd/check -fix
go run ./cmd/check -tests=false -cross
```

默认依次检查格式、包依赖方向、两个模块的 tidy 状态、运行依赖校验、`go vet`、
开发/发布模式的 Staticcheck 和测试。`-fix` 只自动处理格式及 import，不自动改写业务。
`-cross` 额外 vet 并编译 Windows/macOS/Linux 的 amd64、arm64；交叉编译不等于在目标系统运行测试。
工具版本锁定在 `tools/go.mod` / `tools/go.sum`，更新工具时显式更新并验证两个文件。

GitHub Actions 在三个系统分别执行同一检查命令。未实际运行的 CI 不能作为测试通过的证据。

## 构建

```powershell
.\scripts\build.ps1
.\scripts\build.ps1 -Release -Output build\YoloMCL.exe
```

```sh
bash scripts/build.sh
bash scripts/build.sh --release build/YoloMCL
GOOS=windows GOARCH=arm64 bash scripts/build.sh --release build/YoloMCL-arm64.exe
```

也可直接使用 `go run ./cmd/build -goos linux -goarch arm64 -output build/YoloMCL-linux-arm64`。
直接调用 Go 构建工具时不要把运行工具本身的 `GOOS` 改成无法在本机执行的系统。

配置仅从显式 publisher 文件、环境变量、仓库 `.env` 读取，按此顺序补齐字段。
`.env` 按字面解析，不执行 shell，不进行变量展开，也不读取父目录或用户目录的配置。
使用引号包含特殊字符；行尾注释放在空格之后。实际配置不进入版本控制。

## 边界与平台

遵守 [项目架构](docs/architecture.md) 中的依赖方向。UI 不直接实现安装、备份或磁盘扫描；
后台任务不直接修改 UI 状态。文件路径使用 `filepath`，协议路径使用 `path`。
系统 API 放入带 GOOS 文件后缀或 build constraint 的文件；公共逻辑避免散落平台分支。

整合包扫描只访问已知 Minecraft 内容和启动器独立配置。Java 检测另行检查安装目录、
环境变量和系统登记位置，首帧之后后台运行，不能因切换实例重复启动。

MyGo 依赖版本固定，适配只在本仓库 `plugins/startup` 中实现；不要修改模块缓存、
vendor MyGo 或用 `replace` 指向私有改动。升级依赖时先检查插件兼容性测试。

构建输出放入 `build/`，本地临时产物放入 `.cache/`，测试使用 `t.TempDir()`。
根目录 `rsrc_windows_amd64.syso` 是 Windows 链接资源输入，不能当作多余可执行文件删除；
它不被 macOS/Linux 或 Windows arm64 构建使用。
