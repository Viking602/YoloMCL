# 26.x 种子地图兼容修复（2026-11-06）

本轮从本机 XPlus 26.2 地图只显示灰格的问题复现，沿相同的数据加载与编译路径修复：

- 安装的游戏 JAR 用 `{"preset":"minecraft:overworld"}` 等记录引用 Java 内置群系预设，覆盖了随程序附带的显式参数列表。此前解析误入仅支持 1.x 的分支。现在从对应 26.x 版本的已验证内置列表解析，并保留数据包覆盖与内置预设的区别。
- 26.2 的洞穴密度使用 `interval_select`，原生编译器此前未接入，主世界的方块地形与周围地形预览因此不可用。现编译为有界的平衡分支图，保留阈值的上区间边界与惰性求值。
- 新版 `preliminary_surface_level` 被当作缺失的旧字段 `initial_density_without_jaggedness`，静默替换为零。现读取新版高度函数，接入 `find_top_surface` 和 `invert`，旧版继续扫描初步密度。关闭含水层时也会编译地表规则所需的初步高度。
- 26.1+ 把世界生成设置拆到 `data/minecraft/world_gen_settings.dat`。地图、种子选择列表和存档摘要现在通过同一入口读取种子与维度，并识别 `Data.spawn` 的世界出生点。独立文件存在时优先使用其数据，包括零种子；损坏文件不会悄悄回退到旧种子或原版维度。
- 26.2 地表规则中的 `biome_is` 支持单一 ID、列表和标签；此前只接受列表。三维噪声条件的 `is_3d` 也不再被忽略。
- 没有群系预测的维度直接显示不可用提示，点击“查看原因”打开地图信息；有效地图不显示该提示。

快照 schema 从 20 更新为 21。修改未向启动器或 Go 测试引入 Java 运行时依赖。

## 数值证据

对本机安装的官方 26.2 客户端校验 SHA-1：
`2dc72797acbc1b63fc16a11c4ac393605f453754`，与安装清单一致。
独立 Java 参考程序仅调用公开的 `Bootstrap`、`VanillaRegistries`、
`RandomState`、`NoiseRouter` 与 `DensityFunctions` 接口；未启动游戏服务器或生成区块。

- `cubiomes/testdata/interval-select-26.2.json`：18 个输入，包含每个阈值本身、相邻 IEEE double 值、正负零和远端值。
- `cubiomes/testdata/terrain-26.2.json.gz`：5 组设置（普通、巨型群系、放大化、下界、末地）× 6 个种子（含有符号 64 位极值）× 10 个位置，共 300 个位置。检查原始最终密度、初步地表高度以及二维／三维地表噪声条件；位置含负坐标与世界边界。
- 既有五个 26.x 版本的独立 Java 群系对照，新增了安装 JAR 中的预设引用格式；旧的显式参数列表路径同时保留检查。

这些检查不等于完整含水层、地表材质、结构生成或任意模组兼容性的逐方块证明。
26.3 完整浮点地形与任意自定义 Java 生成器仍保留原有能力边界。

数据格式依据还包括 [NeoForge 26.2 迁移说明](https://github.com/neoforged/.github/blob/2bb6f9f3bdaec6efe2b69c3241b15fb47b64fc53/primers/26.2/index.md)与 [26.1 存档拆分说明](https://github.com/neoforged/.github/blob/2bb6f9f3bdaec6efe2b69c3241b15fb47b64fc53/primers/26.1/index.md#splitting-the-primary-level-data-into-saved-data)。

## 回归入口

```text
go test ./cubiomes -run 'TestLatest|TestIntervalSelect|TestModernTerrainJavaOracle|TestPreliminarySurface|TestSurfaceBiome' -count=1
go test ./internal/nativeui -run TestSeedMapJSONPreviewAndOpaqueBadge -count=1
go test ./internal/launcher -run 'TestSplitWorldMetadata|TestLevelDatStructuredWorldSpawn' -count=1
```

本机原版客户端数据检查覆盖 1.21.1、1.21.4、26.1.2、26.2 的三个原版维度。
安装实例回归还读取实际 Fabric 模组与 26.1.2 存档，缓存与测试设置写入独立临时目录。
该真实存档为超平坦，修复后仍使用存档中的超平坦生成器，不替换为普通主世界。

最终验证：全量普通构建测试通过；发布构建全量测试通过后，新增存档读取与不可用提示又通过发布标签专项回归。
`go vet ./...`、`staticcheck ./...` 和修改文件的格式检查通过。
Windows 当前没有创建符号链接的权限，新增的越界符号链接用例跳过；生产读取使用 Go 的 `os.Root` 限制在所选存档内。
