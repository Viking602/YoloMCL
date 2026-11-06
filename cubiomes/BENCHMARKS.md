# Go / C cubiomes 性能与延迟对比

2026-10-27，Windows amd64 / Ryzen 7 9800X3D / Go 1.27.3，`GOAMD64=v1`。GCC 16.2.0 已安装用于参考测试；生产库仍是纯 Go，无 cgo。以下均为五轮中位数，单采样 worker，各进程顺序运行；测量期间暂停其他编译和测试。

优化前来自同一工作区保留的测试可执行文件，本次重新测量；其 SHA-256 保存在原始记录中。C 固定到 `e61f90580cbdd883214a8054670dacae655e59c0`，编译选项为 `-std=c11 -O3 -fwrapv -ffp-contract=off`，不含 fast-math、CPU 专用选项或 cgo 调用。

## 相同输出的 128×128 群系栅格

种子 42，方块起点 `(8192,64,-8192)`，XZ 步长 4，共 16,384 格。排除生成器构造和首次工作区扩容，复用 Sampler 和输出缓冲。Go 每轮至少 500 ms，C 校准到每轮至少 150 ms。11 组输出逐组核对完整 C 哈希。

| 版本 / 维度 | 优化前 Go / ms | 优化后 Go / ms | C / ms | Go 耗时变化 | 优化后 Go / C |
| --- | ---: | ---: | ---: | ---: | ---: |
| b1.7.3/Overworld | 2.307 | 2.414 | 3.512 | +4.7% | 0.69× |
| 1.6.4/Overworld | 0.098 | 0.078 | 0.034 | -20.5% | 2.27× |
| 1.7.10/Overworld | 0.185 | 0.165 | 0.060 | -10.9% | 2.75× |
| 1.12.2/Overworld | 0.188 | 0.165 | 0.060 | -12.4% | 2.77× |
| 1.16.5/Overworld | 0.247 | 0.227 | 0.099 | -7.9% | 2.30× |
| 1.17.1/Overworld | 0.249 | 0.226 | 0.099 | -9.1% | 2.29× |
| 1.20.1/Overworld | 27.238 | 22.480 | 36.331 | -17.5% | 0.62× |
| 1.20.1/Large | 22.942 | 19.525 | 29.035 | -14.9% | 0.67× |
| 1.17.1/Nether | 2.047 | 2.094 | 1.758 | +2.3% | 1.19× |
| 1.17.1/End | 1.007 | 0.111 | 0.172 | -89.0% | 0.65× |
| 1.20.1/End | 0.887 | 0.162 | 0.171 | -81.7% | 0.95× |

全部库栅格热路径都是 **0 B/op、0 allocs/op**。Go 使用 uint16 ID，C 使用 int32 ID；已分配的生成器和工作区仍占内存。

现代主世界本负载耗时下降约 17%，末地下降约 82–89%。历史主世界仍慢于 C；Beta 和旧版下界没有本次采样收益，记录中分别约增加 5% 和 2%，不把其他场景的提升推广到它们。单个种子、坐标和缓存状态也不能代表所有地图。

对照语义：历史版本使用原版 C `genBiomes`，Beta 使用 `NO_BETA_OCEAN` 原始气候语义；现代主世界使用保留前一叶节点的 `sampleBiomeNoise`，与 Go 栅格历史顺序一致。未启用高 scale 近似。没有对比现代下界：这个 C 版本的浮点距离与现代 Go 的整数气候量化不同。26.x 另外用官方 Java 验证，不能直接套用旧 C 的成绩。

## 换种子与移动访问

`New` 的测量每次更换种子，复用已经验证、编译的规则；C 测量 `setupGenerator + applySeed`。这不是第一次进程启动的冷加载时间。第一次仍需数据解压、解码、建图，16 项编译缓存被逐出后也会重新编译。

| 配置 | 优化前 Go / μs | 优化后 Go / μs | C / μs | 优化后 B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| 1.12.2/Overworld | 3.35 | 3.54 | 0.30 | 11,504 | 13 |
| 1.20.1/Overworld | 4,318.63 | 42.56 | 13.68 | 28,228 | 281 |
| 1.17.1/End | 19.51 | 2.08 | 0.68 | 5,976 | 12 |
| 1.20.1/End | 2,648.61 | 1.11 | 0.82 | 984 | 9 |

现代主世界换种子约从 **4.32 ms 降到 42.6 μs**，但仍慢于 C 初始化。调用方应复用生成器，用 `WithSeed` 切换种子；不要逐像素调用构造函数。

| 附加负载 | 优化前 / ms | 优化后 / ms | 加速倍数 |
| --- | ---: | ---: | ---: |
| `BenchmarkEndMoving/1.17.1` | 1.041 | 0.140 | 7.46× |
| `BenchmarkEndMoving/1.20.1` | 1.039 | 0.208 | 5.00× |
| `BenchmarkBuiltinDocuments` | 2.227 | 0.088 | 25.22× |
| `BenchmarkCompileSeed` | 1.122 | 0.108 | 10.38× |

移动末地基准在 32×32 个不同瓦片之间移动，每块间隔 512 方块，不依赖重复返回成品地图。`BuiltinDocuments` 每次返回独立副本；`CompileSeed` 从已加载的有效注册表切换种子，包括内容哈希和种子绑定。

## mcl 绘制和首次图像延迟

瓦片测量包括 RGBA、调色板、图例和结构候选处理，没有成品瓦片缓存命中。旧版位于上述参考坐标；已有现代瓦片基准位于 `(512,64,512)`，不能与不同坐标的库基准直接相减。

| mcl 基准 | 上轮记录 / ms | 本轮 / ms | 本轮 B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkLegacyTile/1.7.10` | 0.488 | 0.280 | 68,907 | 8 |
| `BenchmarkLegacyTile/1.12.2` | 0.498 | 0.279 | 68,927 | 8 |
| `BenchmarkLegacyTile/1.17.1` | 0.573 | 0.341 | 68,718 | 8 |
| `BenchmarkMultiNoiseTile` | 15.808 | 15.002 | 66,600 | 8 |

左列来自[上轮五次原始记录](testdata/benchmarks/2026-10-27/mcl.txt)，不是本轮重测的旧 launcher 二进制。池内工作区可能被 GC 回收，因此 B/op 会波动。数值调色板使每瓦片分配次数从 11 降为 8。

同一优化后的采样器，还比较了完整提交和渐进提交：1.20.1、种子 42、视口 640×384、中心 `(8192,-8192)`、Y=64、4 方块/像素，每次清空瓦片缓存。边缘对齐使这个视口涉及 24 块瓦片。

| 提交方式 | 第一张可用图像 / ms | 完整任务 / ms | 分配 / MiB |
| --- | ---: | ---: | ---: |
| complete | 482.18 | 482.18 | 2.48 |
| progressive | 19.68 | 480.35 | 8.11 |

渐进方式约 20 ms 提供中央部分图像，完整视口仍约半秒；它改善首屏等待，没有把完整计算伪装成已完成。代价是复制不可变部分帧，新增约 6 MiB 临时分配。这里不含 UI 线程调度和 GPU 上传，不是屏幕实测帧延迟。全部缓存命中时只提交一次完整帧。

## 26.3 与真实模组

26.3 使用同样的库栅格坐标、种子和五轮计时，初始化排除；正确性依据独立官方 Java 数据，未列旧 C 对照。

| 26.3 配置 | Go / ms | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| 26.3 Overworld | 21.726 | 0 | 0 |
| 26.3 Large | 21.332 | 0 | 0 |
| 26.3 Nether | 3.274 | 0 | 0 |
| 26.3 End | 0.174 | 0 | 0 |

Terralith 2.6.5+26.3、Tectonic 3.0.29 及组合的 Go 输出通过 442,368 个格子的 Java 对照，另有 49,152 个气候向量。TerraBlender/BOP 使用原生 Go 区域图层和群系查询，三维度已与独立参考逐格对照。完整结果和性能见[模组报告](../docs/seed-map-mods.md)。

## 验证、来源与复现

### 2026-10-27 地形视图与近景补充

以下单独测量三轮中位数，环境同上，各基准顺序执行，无其他测试或编译并行。
`BenchmarkBlockRange` 为 1.21.1、种子 42、128×128、起点 `(-1296,65,2048)`，
开启真实 Voronoi。主世界一步长从逐点版本 **16.268 ms → 2.453 ms，约 6.6×**；
这不等于整张地图的加速倍数。缓存的是气候向量，保持逐像素前一叶节点规则。

新地形 API 的 `BenchmarkSurfaceRange` 使用同样 X/Z 和种子，按估算表面高度取群系，
海平面 63；每轮移动 X，避免只测相同区域。表中比较同一新功能在复用纵列噪声前后：

| 128×128 地形 / 采样间距 | 复用前 / ms | 复用后 / ms |
| --- | ---: | ---: |
| 1 方块 | 6.729 | 6.207 |
| 2 方块 | 13.562 | 12.213 |
| 4 方块 | 33.082 | 20.216 |

两版热路径均为 **0 B/op、0 allocs/op**。纵列复用依据节点的 Y 依赖关系，
三维噪声和 Y 梯度仍按原始精度重新计算；没有通过降低算法精度换取速度。

完整地形视口采用 1.20.1、种子 42、640×384、中心 `(8192,-8192)`，
每次清空成品瓦片缓存，包含取色、明暗、等高线、结构候选和渐进帧复制：

| 地形视图 | 复用前完整 / ms | 复用后完整 / ms | 复用后首批图像 / ms |
| --- | ---: | ---: | ---: |
| 总览，4 方块/像素 | 834.34 | 508.81 | 20.85 |
| 近景，1 方块/像素 | 190.07 | 193.48 | 9.44 |

总览完整采样减少约 39%。近景完整耗时没有改善（本轮约 +1.8%），不将总览收益推广到
所有缩放级别。首批图像不是完整地图；计时不含启动、资源解析、UI 调度和 GPU 上传，
也不代表任意整合包。点击信息从已完成瓦片读取；放大的细等高线由 UI 按屏幕像素绘制。

原始记录：[逐点基线](testdata/benchmarks/2026-10-27-surface/block-before.txt)、
[纵列复用前核心](testdata/benchmarks/2026-10-27-surface/core-before-column.txt)、
[纵列复用后核心](testdata/benchmarks/2026-10-27-surface/core-after-column.txt)、
[视口复用前](testdata/benchmarks/2026-10-27-surface/viewport-before-column.txt)、
[视口复用后](testdata/benchmarks/2026-10-27-surface/viewport-after-column.txt)。

复现命令：`go test ./cubiomes -run '^$' -bench '^BenchmarkBlockRange$|^BenchmarkSurfaceRange$' -benchtime=300ms -count=3 -benchmem`，
然后单独运行 `go test ./internal/launcher -run '^$' -bench '^BenchmarkSeedMapViewportFirstPaint/(surface-overview|surface-detail)$' -benchtime=1s -count=3 -benchmem`。

### 全库验证

正确性覆盖旧版 C 参考的 539 组配置、68,992 个单点、19,739,258 个栅格 ID，以及现代噪声、Voronoi 和末地参考。26.1、26.1.1、26.1.2、26.2、26.3 每版分别检查 1,392 个位置与 168,960 个栅格格子；26.3 另检查 4,096 个等距边界目标。新测试还覆盖重绑定不改变原生成器、末地滑动缓存对照穷举、LCG 跳步、覆盖资源失效、渐进帧不可变与模组文件和配置变化后的缓存失效。

```sh
# 固定版本 C、当前 Go、mcl、新版本和首屏延迟，顺序执行。
go -C tools run ./cmd/cubiomes-benchmark --fetch --cc /path/to/gcc

# 有本次优化前的 Go test 二进制时，可额外重测基线。
go -C tools run ./cmd/cubiomes-benchmark --cc /path/to/gcc --baseline /path/to/before.test.exe

# 只核对 C 编译和全部基准哈希。
go -C tools run ./cmd/cubiomes-benchmark --verify-only --cc /path/to/gcc

go test -race ./cubiomes ./internal/launcher
bash scripts/check.sh --cross
```

原始五轮数据：[优化前](testdata/benchmarks/2026-10-27-optimized/before.txt)、[优化后](testdata/benchmarks/2026-10-27-optimized/go.txt)、[C 栅格](testdata/benchmarks/2026-10-27-optimized/c.csv)、[C 初始化](testdata/benchmarks/2026-10-27-optimized/c-init.csv)、[mcl](testdata/benchmarks/2026-10-27-optimized/mcl.txt)、[26.3](testdata/benchmarks/2026-10-27-optimized/latest.txt)、[首屏](testdata/benchmarks/2026-10-27-optimized/viewport.txt)、[环境](testdata/benchmarks/2026-10-27-optimized/metadata.json)、[中位数汇总](testdata/benchmarks/2026-10-27-optimized/summary.json)。

优化算法、profile 结论和下一步方向见 [OPTIMIZATION.md](OPTIMIZATION.md)。数据和参考生成详见 [data/README.md](data/README.md)、[testdata/README.md](testdata/README.md) 与[新版对照验证](../docs/worldgen-reference.md)。
