# 整合包种子地图验证：2026-10-28

## 当前状态：按实例选择加载器，隔离发现与执行入口

加载器差异集中到 `cubiomes/modjar/LoaderProfile`。生产路径先解析实例版本继承，
确定 Minecraft、加载器和加载器版本，再选择模组；不会根据 JAR 的元数据切换
实例加载器。资源读取、入口执行、游戏符号映射和依赖类路径使用同一实例上下文。

| 实例加载器 | 模组发现规则 |
| --- | --- |
| Fabric | `fabric.mod.json`；客户端排除 `environment=server` |
| Quilt | 优先 `quilt.mod.json`，兼容 Fabric 声明 |
| Forge 1.13+ | `META-INF/mods.toml` |
| Forge 1.7.10 / 1.12.2 路径 | 按对应命名空间的 `@Mod` 注解发现，`mcmod.info` 为补充信息 |
| NeoForge 1.20.1–1.20.4 | `META-INF/mods.toml`；1.20.1 保留 Forge API 命名空间 |
| NeoForge 1.20.5+ | `META-INF/neoforge.mods.toml` |
| 原版 | 不将 mods 目录中的 JAR 当作有效模组；数据包仍独立加载 |

本轮修正：

- 非当前加载器的模组不再贡献资源或入口。双加载器 JAR 只执行当前加载器的
  `@Mod` / 订阅者或 JSON 入口，不再回退扫描并调用任意静态初始化方法。
- Quilt 独立元数据接通 TerraBlender 专用入口。现代 FML 一个 JAR 声明的多个
  模组 ID 全部保留，避免只识别第一项。
- 声明为 FML 库的文件进入类路径，不作为模组数据包；实例入口和核心加载器
  库优先于名称猜测，共用辅助库不再误判为另一个加载器。
- 旧 Forge 按注解发现模组，支持宽松 JSON、可选元数据、版本 properties 的
  Java 转义及版本优先级。NovaEngineering 识别到 272 个模组，元数据读取
  误报已消除。TerraBlender 只由实际安装的框架激活，旧 BOP 不再误入该流程。
- 增加官方映射、Fabric Intermediary、Forge SRG 的基础符号规范化，保留普通
  字符串常量原值。继承方法映射、完整反射及任意转换器执行仍需补齐。
- 快照 schema 更新为 **20**，加载器版本参与缓存标识。

规则依据：[NeoForge 20.5 公告](https://neoforged.net/news/20.5release/)、
[Forge 1.12 元数据解析](https://github.com/MinecraftForge/MinecraftForge/blob/1.12.x/src/main/java/net/minecraftforge/fml/common/MetadataCollection.java)、
[Forge 1.12 版本优先级](https://github.com/MinecraftForge/MinecraftForge/blob/1.12.x/src/main/java/net/minecraftforge/fml/common/FMLModContainer.java)。
宽松 JSON 的 10 个有效/无效用例已用本机游戏依赖 Gson 2.8.0 静默交叉验证。
该 Java 调用仅用于独立测试，启动器和预测引擎仍为 Go；本轮未启动 Minecraft。

验证结果：

- 普通和 `mygo_noinspector` 发布标签全量 Go 测试通过。
- `cubiomes/modjar` race 测试通过；格式、架构、依赖、vet、staticcheck 通过。
- 九个安装包用例 **5 通过、4 失败**，用时 26.17 秒，非单独性能基准。
  仍失败：Mechanomania、ATM10、FarmingTales、GregTech Odyssey。
  原因仍包括噪声变换、动态依赖定位器、共用 Java API 和注册顺序，不能称为
  所有模组的初始化或地图预测已经正确。
- 宝可梦真实存档对照 886,464 个群系点：**886,055 严格相同，409 个等距边界
  候选不同**。该用例显式允许等距候选，因此测试通过；这 409 点不能计入
  严格一致，也不能据此宣称全部预测正确。用时 15.02 秒。

日志在 `.cache/seedmap-coverage-audit-20261028/`：`profiles-all-2.log`、
`profiles-release-tests-1.log`、`profiles-check-2.log`、`profiles-race-1.log`、
`profiles-matrix-2.log`、`profiles-save-1.log`。

当前测试构建：`build/latest/YoloMCL-loader-profiles.exe`，21,051,392 bytes，
Windows amd64，`CGO_ENABLED=0`，`mygo_noinspector`。
SHA-256：`51FC07535E0AE2C1D7283BA6079AFC4D989C65C8AFD68299D36AD75FF0C73887`。

## 先前记录：共享依赖类路径和执行修复

完整通用加载流程**仍未完成**。此前的主要遗漏是：生产执行类路径没有完整接入
版本清单声明的游戏依赖库，注册对象的原生状态和真实 JAR 字段状态又没有正确分派。
因此，已经能读到模组并不等于其初始化、注入和预测流程已经能运行。

本轮实际改动：

- 按已安装版本的继承关系、运行平台规则和客户端标记选择依赖库；注册和群系
  引导使用同一个类路径构建函数。依赖库提供类和服务资源，不作为额外模组或
  数据包加入资源优先级。没有下载依赖或启动 Java。
- 类路径读取失败会传递给需要执行代码的路径，不再以不完整依赖继续编译。
  纯数据路径不会因为未使用的代码库而主动初始化无关模组。
- 修正原生对象和真实依赖类之间的虚方法分派，避免配置 `ImmutablePair`
  从未初始化的 Java 字段读取出 null；模组自己的方法覆盖仍然执行。
- 修正 NeoForge 延迟注册的协变返回类型、虚拟 holder 工厂、显式 super 调用，
  以及接口的传递继承关系。支持延迟 holder 按注册表和资源键进行集合查找。
- 补充 Class 名称、日志对象、TimeUnit 数值换算、Map 空判断等共用运行库行为。
  TimeUnit 的 7×7 单位组合使用任意精度整数交叉验证截断与溢出饱和。
- 声明在主 JAR 中的 Mixin 配置也可从已选依赖类路径读取；缺失或冲突的配置
  仍然报错。未把未声明的内嵌 JAR 全部当作有效依赖。
- 快照 schema 升级为 **19**，淘汰旧缓存。

新增路径没有按具体模组 ID 分支。既有 BOP 固定配置仍在；生产 TerraBlender
路径仍按提供者执行，完整共享生命周期、全部游戏符号映射、通用 ASM/Mixin
变换和动态依赖定位器尚未接通。

最新安装包回归为 **5/9 通过、4/9 失败**（测试用时 17.89 秒，非性能基准）：

| 结果 | 用例 / 本次剩余失败 |
| --- | --- |
| 通过 | 宝可梦包手动种子、宝可梦包存档、NAST、NovaEngineering、FTB-SKY |
| 失败 | Mechanomania：C2ME 改变有效末地噪声路径的变换尚未执行 |
| 失败 | ATM10：游戏实体/材质注册表、集合迭代、Guava 缓存初始化等；新增共享依赖已让执行越过旧错误 |
| 失败 | FarmingTales：Quark 已越过代理和 Multimap 初始化，当前到达进程时钟；另有 SRG 映射、Forge API 和 JarJar 选择缺口 |
| 失败 | GregTech Odyssey：Forge API 和注册初始化缺口 |

ATM10 额外自定义维度还涉及动态依赖发现：本地 CrashAssistant 的根 JAR 声明了
Mixin 配置和 `IDependencyLocator` 服务，实际配置位于内嵌 JAR；仅解析 JarJar
元数据不足以复现这种定位器。此处没有加入模组名称特例或跳过该变换。

这是**读取、初始化和编译回归**，不是游戏内坐标或完整世界生成正确率。
本轮没有启动游戏，也没有完成用户要求的全部整包实机验证。

当前证据在 `.cache/seedmap-coverage-audit-20261028/`，文件前缀为 `libraries-`：
`libraries-matrix-final.log` 保留全部失败链；`libraries-tests-final.log` 为核心测试。
普通和发布标签全量 Go 测试通过（`libraries-all-tests-final.log`、
`libraries-release-tests-final.log`）；包含实际 C6C JAR 的 modjar 竞态检查通过
（`libraries-race-final.log`）。609 个 Go 文件的格式、架构、依赖、vet 和
staticcheck 检查通过（`libraries-checks-final.log`）。

更新的阶段测试构建：`build/latest/YoloMCL-generic-runtime.exe`，20,987,904 bytes，
Windows amd64，发布标签 `mygo_noinspector`，`CGO_ENABLED=0`，SHA-256：
`8B632AA7383CE560E2B10E7F77A2E23F9CA6F40B00553FB1E599D1BE0CE7C054`。
构建日志为 `libraries-build-final.log`；没有重启正在运行的启动器。
下文上一阶段记录保留作对比，不应被解读为全部兼容。

## 上一阶段：通用执行能力增量

当前阶段是**共用执行环境开发和整包回归**，并非全部加载器、版本、模组已兼容。
下文旧检查和旧构建按历史记录保留；当前结果以本节为准。

本轮已实现并接入生产注册路径的能力：

- 按声明解析内嵌 JAR、Maven 版本范围和服务配置，惰性执行服务提供者。
- 补充集合、UUID、明确种子的 Java Random、配置构建及客户端/服务端 Supplier 分派。
- 加入受限字段及构造器反射；修正 `Class[]` 类型判断导致的错误 `ArrayStoreException`。
  Quark 的代理构造已通过，当前继续至 Guava `ArrayListMultimap` 初始化依赖。
- 将存档 `serverconfig` / `defaultconfigs` 接入配置读取；保留初始化异常的实际原因链。
- 依据最终六项气候通道判断噪声注入的作用范围。Mechanomania 的 KubeJS 配置让末地
  使用普通噪声，因此仍需要执行 C2ME 对该噪声的变换，不能直接忽略。

以上按 Java、加载器和框架 API 复用，没有为这些修复增加新的模组 ID 分支。
既有 BOP 固定验证配置仍存在；完整共享注册会话尚未替换逐提供者的生产路径。
任意 Mixin/ASM、全部符号映射、跨提供者初始化顺序和全部内嵌数据包资源仍未完成。

实际安装包的严格三维度矩阵仍为 **5/9 通过、4/9 失败**，本次用时 17.61 秒：

| 当前结果 | 用例 / 剩余缺口 |
| --- | --- |
| 通过 | 宝可梦包手动种子与存档两例、NAST、NovaEngineering、FTB-SKY |
| 失败 | Mechanomania：有效噪声路径中的 C2ME Perlin 注入 |
| 失败 | ATM10：游戏注册表及集合/缓存等初始化依赖 |
| 失败 | FarmingTales、GregTech Odyssey：Guava/Forge API、符号映射及内嵌依赖选择 |

这是加载与编译检查，不是完整世界生成正确率；本轮没有启动游戏。
独立数值参考验证了 1,600 组 Maven 版本比较、240 组 Random 序列的 1,440 项结果；
C6C 的 37,178 个恒定气候查询和 680 个带种子查询也通过。后两组仍不能代替
整包 KubeJS 气候、Mixin、结构及地表方块的游戏内核对。

当前记录位于 `.cache/seedmap-coverage-audit-20261028/`：
`runtime-matrix-final.log`、`runtime-tests-final.log`、`runtime-release-tests-final.log`、
`runtime-core-tests-final.log`、`runtime-race-final.log`、`runtime-checks-final.log`。
数值输入及复现方法见[参考记录](worldgen-reference.md#shared-java-runtime-numerical-references)。

普通与发布标签全量 Go 测试已通过；最后的反射补充也通过普通/发布核心测试、
包含安装 C6C JAR 的竞态检查和格式/架构/依赖/vet/staticcheck。
发布核心记录为 `runtime-release-core-final.log`。

当前阶段构建为 `build/latest/YoloMCL-generic-runtime.exe`，20,957,696 bytes，
Windows amd64，`CGO_ENABLED=0`，发布标签 `mygo_noinspector`，SHA-256：
`D1D12AF9344D0EE0A1A498DC9D3791073617E574771548F4296B49EC134068CE`。
Apache Maven 的 LICENSE / NOTICE 位于同一输出目录及仓库的测试数据目录。
构建没有启动或替换正在使用的启动器，也未加入 JVM 或 HTTP 生成服务。

## 先前预览修复记录

截图中的 **Lets go cobblemon master [5.9.2-魔改科技]** 已恢复主世界、下界和末地预览。手动种子与“新的世界”分别加载 5,289 和 4,231 项有效资源，识别 220 个模组。存档启用列表不同，资源数量不应强行相同。

## 空白预览的原因与修复

- 大型 JAR / ZIP 超出旧索引限制，部分使用 ZIP64。现在支持有界 ZIP64 索引，最多 131,072 个条目、32 MiB 中央目录、8 GiB 文件；只读取需要的资源，不解压整个包。
- 大型汉化文件占用了世界生成预算。地图标签与生成资源分别计量；每个 JSON 仍最多 16 MiB，世界生成总量仍最多 32 MiB。
- Global Packs 的配置路径及 Paxi 的加载设置未完整参与合并。现在读取明确配置的目录、必选/可选列表和加载顺序；已存世界按其启用列表合并。
- 补充 Fabric 1.21.1 的 TerraBlender 4.1.0.8 / BOP 21.1.0.14 原生配置，以及 Lithostitched 1.7.13 的群系注入和 cellular noise。这使 Legendary Monuments 的数据规则可以直接驱动 Go 引擎。
- 普通附属模组的不同依赖表格式、无关模组重复 ID 不再导致整个资源被跳过或禁用生成器。
- 维度上限从 32 调整到 128，并优先保留主世界、下界、末地，修复 GregTech Odyssey 的下界被自定义维度挤掉的问题。

生产路径仍为原生 Go，没有 JVM、Java 子工程或 HTTP 世界生成服务。参考 Java 编译程序仅位于仓库外的开发目录。

## 多包加载检查

以下保留上一轮使用本机实际实例、模组和配置的 9 个用例、8 个整合包加载记录。表中“可加载”表示当时生成了预览瓦片，不代表所有注入、坐标、结构和自定义生成器均已与游戏核对。**后续通用注册检查发现更多缺口，见下文；不能把这张旧表当作当前的兼容保证。**

| 整合包 | 版本 / 加载器 | 主世界、下界、末地 | 本机读取和编译耗时 |
| --- | --- | --- | --- |
| Lets go cobblemon master 5.9.2，手动种子 0 | 1.21.1 / Fabric | 均可加载 | 785 ms |
| 同包，“新的世界” | 1.21.1 / Fabric | 均可加载 | 479 ms |
| Mechanomania 1.1.6 | 1.21.1 / NeoForge | 主世界、下界可加载；C6C 末地未实现 | 695 ms |
| All the Mods 10 | 1.21.1 / NeoForge | 末地可加载；主世界、下界区域注册未完整实现 | 1,190 ms |
| FarmingTales ForgeⅡ v1.3.3 | 1.20.1 / Forge | 均可加载 | 531 ms |
| GregTech Odyssey | 1.20.1 / Forge | 均可加载 | 265 ms |
| NAST easy c1.8.2 | 1.21.1 / NeoForge | 均可加载 | 382 ms |
| NovaEngineering-World | 1.12.2 / Forge | 原版生成路径可加载；未验证旧版模组注入 | 117 ms |
| FTB-SKY | 1.21.1 / NeoForge | 均可加载 | 705 ms |

单次测量，Windows amd64 / Ryzen 7 9800X3D / Go 1.27.3；使用临时启动器配置和空快照缓存，操作系统文件缓存可能已热。耗时包括资源读取和引擎创建，不包括首屏完整绘制。各包检查没有出现 `source_unreadable`。

宝可梦包还通过了原生界面的无窗口回归：实际有效快照、种子 0、地表视图、结构列表、隐藏标记和连续放大均能绘制。截图位于本地 `.cache/seedmap-cobblemon-ui/`；测试外壳标题是测试实例，地图数据来自真实整合包。

## 与游戏存档比对

只读宝可梦包“新的世界”的已生成 MCA 区块，种子 `-5116646549048618101`。覆盖 4,617 个完整区块中选定高度段的 **886,464 个原始 quart 群系值**，共 30 种群系；其中包含模组群系及 Legendary Monuments 注入结果。测试不改原存档，也不占用游戏窗口。

- **886,055 点**：Go 群系名与存档完全一致。
- **409 点**：名称不同，但已逐点确认存档群系属于同一最小气候距离的并列候选。
- **0 点**：其他差异。

并列候选差异没有计为完全一致。它们涉及最近邻查询历史及生成时的遍历顺序，因此本结果不是“每个坐标唯一群系名 100% 一致”的证明，也不是高度、地表方块或结构的全量验证。先前 221,616 点抽样结果为 221,502 个严格一致和 114 个并列候选；上述结果覆盖全部已提取样本。

## 独立算法验证与范围

- 原样编译公开 BOP 区域 / 群系构建源码，使用真实 Minecraft / TerraBlender API 和明确配置输入。92 组配置、2,311,868 项参数映射与 Go 的完整区域表哈希一致。参考方法并非完整 Fabric 启动，详情见[来源记录](../cubiomes/data/terrablender-1.21.1.provenance.json)。
- 原样编译公开 FastNoiseLite 实现，5,600 个样本的浮点位完全一致；包含负种子、64 位种子边界、远坐标和多种 jitter。
- Lithostitched 参考公开提交 `cd5482eeb09d8bc7a1db33a3fa1df057db40160b`。该公开提交覆盖多个发布变体；实际安装的 1.7.13 另由上述存档结果校验。
- 1.21.1 TerraBlender 末地选择按公开源码实现，包括该版本使用 edge 表填充 island 区域的行为；尚缺完整独立末地栅格基准。

仍需继续验证或实现：C6C 自定义末地、ATM10 的完整 NeoForge 区域注册、宝可梦包的 raid / distortion 自定义生成器、未实现的 Lithostitched codec 和加载谓词、重叠等优先级注入、系统级 Global Packs、嵌套 JAR 的注册表资源、旧版 Java 注入逻辑。部分结构也仍只有候选位置，不能保证实际落地。

Global Packs 的公开源码地址当前不可访问，其手动新世界默认排序依据这个实际安装包的存档启用顺序；已有存档按记录顺序处理。Paxi 的目录和显式顺序规则依据公开 1.21.1 源码。

## 通用加载补充检查

新增加载流程按框架入口、配置和实际注册调用复用逻辑，不为新模组增加名称白名单或群系参数表。实现细节和限制见[通用注册加载说明](seed-map-mods.md#shared-registration-loader)。此轮检查读取 JAR、配置和现有日志，没有重新启动游戏，也没有改动整合包或存档。

ATM10 自动发现 5 个外部 TerraBlender 区域提供者：`ars_nouveau`、`biomeswevegone`、`ars_elemental`、`integrateddynamics`、`regions_unexplored`。它们与该实例实际游戏日志中的提供者集合相符。此前只按依赖声明识别，遗漏了其中 3 个可选集成；遗漏会改变区域索引，不能作为正确预测。

发现不等于执行成功。当前完整初始化仍分别遇到尚未绑定的 `ServiceLoader`、`UUID`、`ArrayList`、Guava 集合和 Minecraft `Biomes` API。Ars Nouveau 原先因无关类挤占 16 MiB 执行预算而提前失败；按需读取已解除这一限制，但尚未补齐其实际初始化依赖。多提供者之间的加载顺序和共享初始化状态也未完成。BWG 初始化流程核对的是公开 2.5.1 标签，提交 `97c0e4026f63495e26d9b1e10942ddf75bc73683`，不是其他分支或猜测参数。

C6C 1.2.1 的资源已读到 `c6c:new_end_biome_source`。用户已确认没有公开源码，现已通过通用 Go 字节码探针直接执行安装 JAR 的 codec 和群系选择方法。与项目外的独立 Java API 调用比较，37,178 组区块四分位坐标及恒定气候输入、全部 12 种返回群系和 codec 的 12 个 holder 顺序均一致。Java 参考使用空的 FML 加载目录和 Minecraft bootstrap，不是整包启动；探针尚未接入地图的真实噪声输入、最终注册表和维度路由，也没有覆盖 Mixin。这个结果验证了无源码 JAR 的直接执行路线，不代表 C6C 末地预览已经可用。[来源与范围](../cubiomes/modjar/testdata/biome-source-c6c-1.2.1.json)。

同一组 9 个实例用例重新检查后，7 个通过原有的局部维度要求，2 个失败：FarmingTales 识别出 Quark、BWG、Nature's Spirit、Youkai's Feasts 的区域注册，GregTech Odyssey 识别出 BWG、Ars Nouveau 注册，尚未完整执行，因而没有返回伪装成已应用这些规则的原版预览。原矩阵对 ATM10 只要求末地，对 Mechanomania 只要求主世界/下界。随后对全部 9 个用例都要求三维度可加载，结果为 5 个通过、4 个失败；新增失败的两个是 ATM10 和 Mechanomania。不能将局部用例通过称为整包正确，更不能称为全版本完成。

本轮证据位于 `.cache/seedmap-coverage-audit-20261028/`：`all-instances.log` 为原有局部矩阵，`all-dimensions.log` 为全部 9 用例的严格三维度矩阵，`registration-tests.log` 为包括实际 YUNG's JAR 默认/修改配置在内的注册回归。`biome-source-comparison.log` 是 C6C 函数级 Java/Go 比对，`biome-probe-tests.log`、`biome-probe-race.log`、`biome-probe-checks.log` 是其后的全项目测试、解释器竞态检查、格式/架构/vet/staticcheck。加载检查只验证瓦片是否可生成；此前的存档及独立源码比对结果仍保留各自的范围。

## 检查和产物

### 自定义群系采样与插件条件增量（2026-10-28）

生产地图现在能根据实际注册的 codec ID 绑定受支持的自定义群系源，读取有效
群系注册表并提供原生六路噪声输入；每个采样 worker 独立，继续应用数据驱动的
群系替换规则。注册发现不使用 C6C 模组 ID 或类名分支。已有 BOP 等固定版本
适配路径仍然存在，尚未完成统一替换。

Mixin 加载读取加载器的显式配置，支持部分 bootstrap 注入、插件启用判断和
共享末地高度函数覆盖。补充了 class 版本判断、字符串拼接、数组协变和受限的
类字段元数据读取。宿主能力错误不能被 Java catch 当作可恢复异常吞掉；失败
的静态初始化再次访问时也不会把半初始化字段当成成功结果。

独立 Java 参考新增 5 个种子、680 个坐标：原始末地侵蚀值逐位一致，群系 ID
全部一致，采到 4 种群系；之前的 37,178 个恒定气候输入覆盖全部 12 种群系。
参考执行实际 C6C JAR 的覆盖函数和 Minecraft 的种子噪声，未执行完整整包的
Mixin 变换。因此这些通过结果只证明受测算法路径。

严格矩阵依然是 **5/9 通过**：宝可梦手动种子/存档两个用例、NAST、
NovaEngineering 和 FTB-SKY 可加载所要求的三维度；Mechanomania、ATM10、
FarmingTales 和 GregTech Odyssey 仍失败。Mechanomania 的整包路径仍遇到
C2ME 插件的非公开反射访问；其他包还有服务发现、集合/配置 API、入口生命周期
等缺口。没有把这些失败改成通过，也没有用原版群系冒充模组结果。

本机矩阵日志为 `.cache/seedmap-coverage-audit-20261028/plugin-matrix.log`；
后续 Mechanomania 检查为 `plugin-mechanomania-final.log`。新算法参考的输入、
哈希和复现方式见[模组报告](seed-map-mods.md#direct-jar-biome-source-execution-experimental)。
这轮仍属于共享运行能力开发和整包验证阶段，并非全模组兼容交付。

### 共享加载会话增量（2026-10-28）

本轮补充了纯 Go 的共享加载会话和内嵌 JAR 类路径。生产启动器接入了后者，
完整共享会话仍是库级接口，没有替换当前逐提供者的注册路径。未新增模组 ID
分支，也没有把现有 BOP 验证配置及旧版兼容路径移除。因此本轮不能交付为
“所有加载器、全版本、全模组无需特化”的完整实现。

真实整合包复测暴露并修正了两类资源读取回归：未使用的重复 class ZIP 条目
导致全局类路径失败，以及 Gson 接受的字符串内控制字符被标准 JSON 拒绝。
还修正了大内嵌 JAR 每跨 64 KiB 窗口就从头解压的问题。相同九用例矩阵的两次
诊断总耗时从 56.13 秒降到 13.75 秒；这是单次加载诊断、有其他进程负载，
不能解释为采样性能倍数或稳定延迟基准。

最终严格三维度矩阵仍为 **5 通过、4 失败**：

- 通过：宝可梦包手动/存档两例、NAST、NovaEngineering、FTB-SKY。
- 失败：Mechanomania/C6C 末地；ATM10 主世界/下界；FarmingTales；GregTech Odyssey。

ATM10 现在继续到实际初始化 API，而不是被无关 ZIP 条目提前阻断；仍缺
ServiceLoader、UUID、集合及部分 Minecraft 注册表语义。Fabric 的通用执行仍
缺 intermediary 映射。C6C 函数探针与完整地图之间仍缺最终注册表、真实噪声
输入和维度路由。加载成功也不等于所有群系、地表或结构已经与游戏逐点一致。

本轮未启动游戏。协议测试使用 Go 构造的测试输入；C6C 使用已有独立 Java
参考哈希复核，不把该函数级证据扩大为整包验证。诊断记录位于
`.cache/seedmap-coverage-audit-20261028/session-all-dimensions-stream.log`；
`session-all-dimensions-final.log` 保留解压优化前的同组诊断。

全量 `go test ./...`、包含安装 C6C 参考用例的解释器 race 测试、
格式/架构/依赖/vet/staticcheck 均通过。对应记录为 `session-tests-final.log`、
`session-race-final.log`、`session-checks-final.log`。这些通过结果不覆盖上面四个
失败整合包的完整预测。

本轮阶段构建：`build/latest/YoloMCL-loader-session.exe`，20,574,720 bytes，
Windows amd64，`CGO_ENABLED=0`，SHA-256：
`8714F281F90067D4564D9D8F1938FB4D28E5C353F6202565872B6D903A68C401`。
构建成功不表示共享会话或自定义群系探针已经接入生产地图的完整生成链。

### 先前构建记录

先前通用加载构建为 `build/latest/YoloMCL-generic-worldgen.exe`，20,517,888 bytes，Windows amd64，`CGO_ENABLED=0`，SHA-256：`2DFD402ABBA8A94F8DD13E0DE1B8B376432DFC796C64D08E05D5BF3E3788A266`。完整 Go 测试、`cubiomes/modjar` 竞态检查、格式/架构/vet/staticcheck 均通过。C6C 的字节码探针没有接入生产地图，本构建仍不代表任意 JAR、全部整合包或全版本已兼容。

已运行普通与发布标签全量 Go 测试、核心引擎和启动器 race 检查、格式/架构/依赖/vet/staticcheck，以及 Windows、Linux、macOS 各 amd64 / arm64 构建检查。新增回归覆盖 ZIP64、依赖声明兼容、全局包顺序、目录注入规则、维度保留、注入阶段顺序和换种子。

本机证据：`.cache/seedmap-matrix-fixed.log`、`.cache/cobblemon-save-full-comparison.log`、`.cache/seedmap-cobblemon-ui.log`、`.cache/seedmap-pack-tests.log`、`.cache/seedmap-pack-release-tests.log`、`.cache/seedmap-pack-race.log`、`.cache/seedmap-pack-check.log`。

可用 `MCL_WORLDGEN_INSTANCES` 指定实例矩阵运行 `TestWorldgenInstalledInstances`；可用 `MCL_WORLDGEN_SAVE_CASE` 指定真实存档提取的坐标运行 `TestWorldgenInstalledSave`。`AllowClimateTies` 默认为 false，只有显式设置才将等距差异单独统计；严格一致数始终独立输出。

发布文件：`build/latest/YoloMCL-seedmap-packs.exe`，20,431,360 bytes，Windows amd64，`CGO_ENABLED=0`。SHA-256：`ACD5957906431DAD943B32466C3550B865B071186F77B42F6F0B78347F577848`。
