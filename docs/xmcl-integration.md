# XMCL 接入方式核对

核对时间：2026-10-20。启动器源码基于提交 `75a52faa73faca04a24e49c58c3afaf79b5bce16`。以下是源码观察，不代表已经为 MCL 获得服务凭据或完成真实账号授权。

## Microsoft 正版登录

- [pluginOfficialUserApi.ts](https://github.com/Voxelum/x-minecraft-launcher/blob/75a52faa73faca04a24e49c58c3afaf79b5bce16/xmcl-runtime/user/pluginOfficialUserApi.ts) 在启动器代码内声明应用 Client ID，并传给 OAuth 客户端；玩家没有填写 Client ID 的步骤。
- [MicrosoftOAuthClient.ts](https://github.com/Voxelum/x-minecraft-launcher/blob/75a52faa73faca04a24e49c58c3afaf79b5bce16/xmcl-runtime/user/accountSystems/MicrosoftOAuthClient.ts) 使用 MSAL 公共客户端，包含缓存 / 静默登录、设备码授权、Windows WAM，以及交互式授权的后备路径。其嵌入式 WebView 授权路径不适用于本项目的无 WebView 要求。
- [MicrosoftAccountSystem.ts](https://github.com/Voxelum/x-minecraft-launcher/blob/75a52faa73faca04a24e49c58c3afaf79b5bce16/xmcl-runtime/user/accountSystems/MicrosoftAccountSystem.ts) 继续完成 Xbox / XSTS、Minecraft 令牌、游戏所有权与档案读取，并区分缺少 Xbox 档案、家庭账号、地区等错误。

MCL：发行时内置自己的公开应用身份；使用原生设备授权界面配合系统浏览器。令牌在 Windows 下通过 DPAPI 保存，启动时刷新。当前没有移植 MSAL / WAM，也没有复制 XMCL 的应用身份。开发构建缺少发行方身份时会明确标示服务尚未启用。

## CurseForge 检索与下载

- [esbuild.config.ts](https://github.com/Voxelum/x-minecraft-launcher/blob/75a52faa73faca04a24e49c58c3afaf79b5bce16/xmcl-electron-app/esbuild.config.ts) 将 `process.env.CURSEFORGE_API_KEY` 在构建阶段替换；[build.yml](https://github.com/Voxelum/x-minecraft-launcher/blob/75a52faa73faca04a24e49c58c3afaf79b5bce16/.github/workflows/build.yml) 从发布流水线 Secret 提供它。Key 由开发者管理。
- [pluginMarketProvider.ts](https://github.com/Voxelum/x-minecraft-launcher/blob/75a52faa73faca04a24e49c58c3afaf79b5bce16/xmcl-runtime/market/pluginMarketProvider.ts) 创建 CurseforgeV1Client，接入下载缓存、临时下载及校验。
- [packages/curseforge/index.ts](https://github.com/Voxelum/x-minecraft-launcher/blob/75a52faa73faca04a24e49c58c3afaf79b5bce16/packages/curseforge/index.ts) 用 `x-api-key` 调用 CurseForge 官方 API。检索使用 `gameId=432`，并传入类别、游戏版本、加载器、排序、`index` 与 `pageSize`。
- [curseforgeSearch.ts](https://github.com/Voxelum/x-minecraft-launcher/blob/75a52faa73faca04a24e49c58c3afaf79b5bce16/xmcl-keystone-ui/src/composables/curseforgeSearch.ts) 将结果的 `logo.url` 作为图标，按返回分页信息处理下一页，并将单页上限限制到 50。
- [curseforgeInstaller.ts](https://github.com/Voxelum/x-minecraft-launcher/blob/75a52faa73faca04a24e49c58c3afaf79b5bce16/xmcl-keystone-ui/src/composables/curseforgeInstaller.ts) 处理安装目录、加载器和必需依赖。

MCL 已对照落实：发行方构建注入 API Key，不需要玩家填写；官方接口检索、Minecraft / 加载器筛选、三种排序、分页长度一致、必需依赖解析、SHA-1 校验及失败回滚。下载使用 API 返回的文件地址；缺少可分发地址的整合包文件显示手动下载信息。

启动器直接访问 CurseForge，Key 随发行程序分发。仓库只保留启动器，不附带代理服务，也不使用 XMCL 的 Key。

## 图标问题定位

项目 API 已正确返回 `icon_url`。本机实测：旧 HTTP/1-only 图像请求访问 Modrinth CDN 时返回 EOF，且失败分支没有清除 pending 状态，导致永久显示占位图。已改为 HTTP/2 协商、有限重试、失败冷却后重试、并发限制及磁盘缓存，并补齐 SVG / WebP 解码支持与原生居中布局。Sodium、Sodium Extra、Reese's Sodium Options 的实际 CDN 图标均已成功取得。
