# cubiomes：用于 mcl 的纯 Go 生物群系库

包路径：`github.com/Viking602/YoloMCL/cubiomes`。只依赖 Go 标准库，无 cgo、动态库、JVM 或运行时下载。mcl 的种子地图、结构候选过滤和出生区域估算已经接入此包。

可选子包 `cubiomes/modjar` 从安装的 JAR 解释受支持的世界生成注册逻辑。TerraBlender 注册结果编译为原生参数，自定义群系源则通过受限的 Go 字节码解释器采样，并接入原生噪声输入。已接通声明式群系注册、部分 Mixin 插件条件及末地密度覆盖；并非完整 JVM 或完整模组加载器。首批注册验证仍是 Forge 1.20.1 / TerraBlender 3.0.1.10 / YUNG's Cave Biomes 2.0.5；独立自定义群系验证使用安装的 C6C 1.2.1 JAR。整包测试仍有失败，不能据此宣称任意模组、所有加载器或全版本均已兼容。详细范围及验证见[模组报告](../docs/seed-map-mods.md)。

支持 Beta 1.7/1.8、正式版 1.0–1.17 的历史生成器、1.18–1.21.11 的现代气候生成器，以及通过官方 Java 对照的 26.1、26.1.1、26.1.2、26.2、26.3。范围是群系预测与已有的结构候选 API；完整结构查找和地形方块生成尚未实现。

## 使用

```go
g, err := cubiomes.New(ctx, cubiomes.Config{
    Version: "1.20.1",
    Seed: 0,
    Dimension: cubiomes.Overworld, // Nether / End
    LargeBiomes: false,
})
if err != nil {
    return err
}

worker := g.NewSampler() // 每个 goroutine 各自创建，之后重复使用
biome, ok := worker.NoiseBiome(0, 64, 0) // minecraft:river, true
_ = biome
_ = ok

pixels := make([]cubiomes.BiomeID, 128*128) // 多次生成可复用
err = worker.GenerateIDs(ctx, cubiomes.Range{
    X: 512, Y: 64, Z: 512,
    SizeX: 128, SizeY: 1, SizeZ: 128,
    StepXZ: 4, StepY: 4,
}, pixels)
name := g.BiomeName(pixels[0])
_ = name
```

`Generator` 不可变，可以跨 goroutine 共享；`Sampler` 持有可变工作内存，不能共享或复制。库不启动 goroutine，mcl 仍控制取消、worker 生命周期和瓦片缓存。

| API | 语义 |
| --- | --- |
| `New(ctx, Config)` | 选择已验证的原版版本、种子和维度；初始化有分配，应在采样循环外调用 |
| `Compile(ctx, docs, Definition)` | 从调用者已合并的注册表及内联设置编译；保留数据包覆盖，不读取文件 |
| `CompileNoiseRouter(ctx, docs, Definition)` | 绑定六路原始密度值，保留双精度和调用者的方块坐标 |
| `CompileCustomBiomeSource(ctx, docs, Definition, source)` | 绑定自定义群系源，检查有效注册表中的群系，给每个 worker 独立状态；支持继续应用数据驱动的群系注入 |
| `Generator.WithSeed(ctx, seed)` | 共享只读规则图和群系树，只重建种子相关噪声；原生成器仍可并发使用 |
| `BuiltinDocuments(version)` | 返回可修改的内置注册表子集副本，供调用者叠加数据包 |
| `Generator.SupportsSurface()` | 识别可用于现代主世界高度估算的深度图；其他维度继续使用群系采样 |
| `Sampler.EstimateSurfaceHeight(x,z)` | 按有效深度图估算高度并插值，保留群系查询历史；不生成地形方块 |
| `Sampler.GenerateSurface(ctx,range,dst)` | 批量生成估算高度与表面群系，支持取消、海平面和 Voronoi；复用有界缓存 |
| `WithTerraBlender(ctx, base, config)` | 在已有气候图上添加加权区域选择；同类模组复用这一引擎扩展 |
| `WithTerraBlenderEnd(ctx, base, config)` | 添加末地区域、群系权重与对应坐标采样规则 |
| `BuiltinTerraBlenderProfileWithToggles` / `BuiltinTerraBlenderEndProfileWithToggles` | 返回指定版本及 BOP 群系开关对应的规则；权重和区域大小可在绑定前调整 |
| `NoiseBiome(x,y,z)` | 方块坐标转所在 quart 格，独立查询；负坐标向下取整 |
| `NextNoiseBiome(x,y,z)` | 连续查询保留前一个叶节点，重现原版等距时的历史行为；`Reset()` 开始新任务 |
| `BiomeAt(x,y,z)` | 使用对应版本的方块群系访问：Beta 1.7 原始气候、1.14 及以前的 2D Voronoi、1.15 起的 SHA-256/3D Voronoi |
| `NoiseBiomeID` / `NextNoiseBiomeID` / `BiomeAtID` | 对应查询的数值 ID 版本；原版沿用 C cubiomes 编号 |
| `Generator.BiomeName(id)` | 按生成器版本解析 ID；旧版用上游 1.17 规范名称，自定义群系从 256 起编号 |
| `Generator.BiomePalette()` | 返回按数值 ID 索引的名称表副本，供绘制端预编译调色板 |
| `Climate(x,y,z)` | 现代六通道气候向量；历史生成器、末地返回不支持 |
| `Generate(ctx, Range, dst)` | 调用者提供缓冲区，按 Y/Z/X 写入，索引为 `(iy*SizeZ+iz)*SizeX+ix` |
| `GenerateIDs(ctx, Range, dst)` | 数值 ID 批量接口；旧版合并计算父图层并复用工作区 |
| `RandomSpread.Candidate` | Java LCG 线性/三角形撒点，返回潜在起始区块中心 |
| `EstimateSpawn(ctx)` | 现代气候目标的水平搜索；历史生成器没有估算结果 |

`Range` 的起点和步长都是方块单位，与 C API 的 `scale` 参数不同。`Voronoi: true` 启用方块访问；批量查询保留栅格顺序的历史判定，每次生成自动重置历史。最多生成 16,777,216 格，输出阶段每 64 格检查取消，历史图层计算也按行检查取消。错误或取消可能留下部分输出。首次采样、较大范围会扩容工作区；复用相同范围的热路径零分配。稀疏大跨度查询不会分配整个世界大小的矩形。

## 支持范围

- Java Beta 1.7–1.7.3、Beta 1.8/1.8.1；正式版 1.0–1.17.1，按 cubiomes 历史版本族区分规则，含 1.16.1 与 1.16.2+ 的差异。
- Java 1.18–1.18.2、1.19–1.19.4、1.20–1.20.6、1.21–1.21.11；现代气候参数树。
- Java 26.1、26.1.1、26.1.2、26.2、26.3 使用分别固定校验值的官方注册表；26.3 独立处理 float 运算、噪声格式、末地远距离半径和 19 分支群系树。
- 主世界和下界覆盖上述版本；末地从 1.0 开始，外岛群系从 1.9 开始；大型群系从 1.3 开始。
- Beta 1.7 使用 C 的 `NO_BETA_OCEAN` 语义，返回原始气候群系，没有海平面地形估算/海洋着色。
- Xoroshiro128++、Java 48 位 LCG、ImprovedNoise、NormalNoise、Simplex、六通道气候、版本化参数树、原版等距规则。
- 通过 `Compile` 支持已实现的 JSON 密度运算、样条、显式群系参数、噪声和末地侵蚀通道覆盖；放大化设置也由注册表路径处理。
- 旧版 `Compile` 支持原版 `vanilla_layered`、下界预设和末地。旧版自定义 noise settings、`legacy_biome_init_layer`/`default_1_1`、旧 flat/customized 字符串设置尚未支持；mcl 显示覆盖缺口。
- 基岩版、快照、未列出的版本、完整结构查找器、强要塞环、表面方块和实际建筑生成尚未实现。未知版本或 Java 自定义生成逻辑不会被当作已支持版本。
- 结构坐标只是候选；还需要群系、频率、排除区域和地形条件检查。出生区域没有地形检查或准确 Y。
- 新世界算法不代表已保存区块、旧版本混合区或未适配模组注入后的结果。已验证的 TerraBlender/BOP 注入由原生适配器处理；mcl 地图继续使用 quart 噪声群系，尚未切换为方块 Voronoi 显示。

## 性能与验证

优化包括历史图层批处理、精确距离剪枝、64 字节连续树节点、跨种子共享已编译规则、LCG 跳步、工作内存复用，以及末地滑动行缓存与岛屿占用位图。共享编译缓存最多 16 项，键包含实际使用的注册表内容；公开文档和调色板返回独立副本。采样热路径不加锁；独立 Sampler 可由调用者并行运行。mcl 使用数值调色板，优先绘制视口中央并提交不可变的渐进帧。

最新同机优化前后、Go/C 对比、移动末地瓦片与初始化成本见 [BENCHMARKS.md](BENCHMARKS.md)，瓶颈分析和后续方向见 [OPTIMIZATION.md](OPTIMIZATION.md)。参考栅格的热采样为 **0 B/op、0 allocs/op**；初始化、已分配的工作区和 mcl 图片另外计费。首次冷加载、换种子、重复瓦片与移动访问使用不同基准，不能混同。

```sh
go test ./cubiomes ./internal/launcher
go test -race ./cubiomes
go test ./cubiomes -run '^$' -bench . -benchmem
go test ./internal/launcher -run '^$' -bench BenchmarkMultiNoiseTile -benchmem
```

GCC 16.2.0 已安装，用于 C 对照和 Windows 竞态检测；库正常构建和使用仍不需要 C 编译器。

本次全量开发/发布检查、Windows/macOS/Linux 的 amd64/arm64 构建，以及库和 launcher 的 `-race` 检查均通过。

独立 C 参考数据新增 21 个历史版本配置、539 个版本/种子/维度/大型群系组合，共 68,992 个单点群系 ID、19,739,258 个栅格 ID（全格哈希比较）。保留现代 792 个群系结果及气候向量、154 个末地查询、RNG/噪声向量、1,792 个 Voronoi 查询。性能基准的 11 组输出也逐组校验 C 哈希。测试覆盖并发、零分配、缓存碰撞/高度、取消、范围溢出和等距历史；mcl 集成覆盖旧存档 world type、旧版瓦片、数据包、结构过滤及 worker 生命周期。

算法和数据源固定到 [Cubitect/cubiomes e61f905](https://github.com/Cubitect/cubiomes/tree/e61f90580cbdd883214a8054670dacae655e59c0)；1.21.5+ 参数树使用已记录的 PR #162 贡献。保留上游 MIT [许可证](data/LICENSE-cubiomes.txt)。详见[数据来源](data/README.md)和[参考数据生成](testdata/README.md)。

26.x 使用分别校验官方服务器哈希的注册表与 Java 参考；每版检查 170,352 个位置，26.3 另有 4,096 个等距边界目标。26.3 浮点噪声参考 C2ME 的公开 MIT 源码并保留许可证。详见[新版对照验证](../docs/worldgen-reference.md)。Terralith/Tectonic 的数据包规则和 BOP/TerraBlender 的注入规则均由原生 Go 计算，版本和对照结果见[模组报告](../docs/seed-map-mods.md)。
