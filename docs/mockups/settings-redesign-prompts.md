# MCL 设置页重构渲染稿

生成方式：内置 imagegen。日期：2026-10-20。

当前设计方向为 `settings-light-v1.png`：雾白背景、冷灰侧栏、低饱和靛蓝重点色；左右分区、统一留白、紧凑按钮。MCL 标识与主标题保留少量像素感，正文和控件采用自然原生风格。服务接入不要求玩家填写应用标识或密钥。

`settings-dark-v1.png` 是第一版深色布局草稿，浅色稿按用户随后提出的整体换色要求调整。这些文件是页面重构预览稿。

## 第一版完整生成提示词

```text
Use case: ui-mockup. Asset type: high-fidelity desktop settings screen redesign, one clean front-facing screenshot, about 1536 x 1024, crisp readable Simplified Chinese.

Input image 1 is the existing MCL native desktop launcher settings page, a reference for the product identity and feature list. Redesign its layout substantially into a polished, realistic, implementable Go native desktop app. This is a design preview of a refactor, not a web page or marketing landing page.

Design direction: calm, balanced, premium and practical. Neutral graphite background #191b20, slightly darker sidebar #14161a, charcoal cards #22252b, ivory text #eae8e2, secondary slate-gray text, restrained warm muted gold #c6ad7a for the main action and selected item. Small gray-blue secondary accents. No green-tinted overall theme and no rainbow cards. Matte surfaces, thin subtle separators, gentle 8–12px corners. Standard natural outline icons, natural rounded native controls. Only the MCL wordmark and main page title have a subtle Chinese pixel-font character; every other label is clean system sans serif. Do not make the whole interface pixel art. No chunky bevels, block icon set, cartoon scenery, glassmorphism, neon glow or oversized pills.

Composition: show the entire application window centered and uncropped, flat front view with a thin dark Windows title bar. No laptop, desk, perspective or explanatory labels outside the app. Slim left navigation about 200px wide. Main content has consistent 32px outer margins, a strong top heading and a clear two-column layout, left column about two thirds width, right column about one third. Use real white space and a disciplined 8px spacing grid, all cards aligned. Nothing collides with the title bar. All icons are optically centered in their containers.

Sidebar: small elegant MCL brand at top with 'Minecraft Launcher' underneath. Navigation labels '游戏库', '模组管理', '探索资源', '角色衣橱', '下载与任务'. '启动器设置' is selected near the bottom with a subdued gold accent. A compact '添加游戏目录' action sits below the main navigation without an enormous blank directory list. At bottom, a compact profile row '添加账号' and tiny 'MCL 0.2.0'. Keep simple line icons.

Main top: breadcrumb '工作台 / 设置', small '添加账号' button on the right. Page title exactly '启动器设置', secondary text '管理启动环境、游戏版本与内置服务'. Remove technical 'NATIVE' badges and developer-only descriptions from product chrome.

Large left card titled '启动环境': small subtitle '为游戏选择合适的 Java 与内存'. Use compact structured form rows rather than stretching every control across the window.
Row 1: label 'Java 运行时', one natural dropdown labeled '自动选择', and small secondary buttons '浏览' and '检测'. Below a subtle description '按游戏版本自动匹配 Java'.
Row 2: label '最大内存', a clean slider with subdued gold active track and circular thumb, compact value '4096 MB', small indication '4 GB'. Everything has sensible spacing and aligned baselines.
A thin divider and a final row '跟随目录设置' with subtle explanation '独立目录参数优先' and a small neutral toggle.

Below the main left card, a shorter card titled '游戏版本', containing a tidy row '显示快照版本', helper '在安装列表中显示开发版本', and a small toggle switched off, aligned right.

Right top card titled '内置服务', subtitle '由启动器统一提供'. Three vertically spaced compact service rows with correctly centered official brand marks: Microsoft's four-color square, the true green Modrinth circular logomark, the true orange CurseForge anvil silhouette. Preserve original brand colors only in these small logos. Labels 'Microsoft', 'Modrinth', 'CurseForge'. Accurate current development statuses: Microsoft '待接入', Modrinth '已连接', CurseForge '待接入', in small neutral status treatments. No form fields for Client ID, API keys, passwords, provider URLs or tokens.

Right lower small card titled '关于 MCL', a small understated brand, version '0.2.0', text 'Minecraft Java 启动器', and compact text button with folder icon '打开数据目录'. Avoid displaying a person's full filesystem path or framework implementation details.

At the bottom of the main content, a restrained horizontal action bar: subtle left hint '设置对新启动的游戏生效', on the right compact secondary '恢复默认' and a normal-sized warm gold primary '保存设置', about 120px wide, not full-width. Make all sections fit naturally in one screen without a scrollbar or cut-off content.

Require readable, correctly spelled Chinese, beautiful restrained typography, practical hierarchy, consistent icon sizes and control heights. This is a substantial page restructuring, not merely a recoloring of the input screenshot.
```

## 浅色主题完整编辑提示词

```text
Use case: ui-mockup. Edit input image 1, the newly redesigned MCL native desktop settings screen, into a clearly different fresh LIGHT THEME. Input image 2 is an exact official brand-logo reference sheet only, not extra content to add to the interface.

Preserve the clean two-column settings layout, all feature labels, their Chinese wording and semantic values, the main sidebar, the thin Windows native title bar, alignment, margins, sensible control sizes, compact action buttons and fully visible uncropped application window. This remains a practical native Minecraft desktop app UI render, not a website. The user explicitly wants the overall theme colors changed from the dark warm gold scheme.

Replace the entire dark palette, including window title bar, sidebar, cards, fields, footer and buttons:
- Application canvas soft mist white #F4F5F7.
- Left sidebar cool light gray #EAEDF2.
- Cards clean matte white #FFFFFF.
- Form field surfaces very light gray #F0F2F6, borders #DCE1E8.
- Main text rich graphite #252C38.
- Secondary text balanced slate gray #78818F.
- Primary accent muted indigo/slate blue #586DA6, not vivid electric blue.
- Selected sidebar item uses a very pale blue background #DCE4F2 and indigo icon/text.
- Main '保存设置' button solid muted indigo with white text.
- Memory slider active track muted indigo.
- Secondary buttons white/gray with subtle borders and graphite text.
- Switches off are neutral gray.
Use very subtle surface shadows and natural 8–12px rounding. Overall airy, elegant, legible, calm and balanced. No gold/brown primary surfaces, no dark panels, no green overall tint, no rainbow or multicolored feature cards, no neon or decorative glow. Microsoft, Modrinth and CurseForge keep their actual small brand colors only.

Important brand correction: in the '内置服务' card, faithfully reproduce the EXACT Modrinth and CurseForge marks shown in input image 2, centered in each logo area. Modrinth is the specific segmented circular green logomark in the reference, not a target, molecule or generic spiral. CurseForge is the exact orange anvil silhouette shown in the reference. Do not stylize or pixelate these official logos. Microsoft's four colored squares remain unchanged.

Typography: keep subtle pixel styling only for the MCL wordmark. Main title '启动器设置' can have a very slight pixel character but must stay clean and readable; use regular native system sans serif for all other text and icons. No pixel-art controls or blocky decorative imagery.

Retain statuses accurately: Microsoft '待接入', Modrinth '已连接', CurseForge '待接入'. No API Key or Client ID fields. Retain main content labels exactly: '启动环境', 'Java 运行时', '自动选择', '浏览', '检测', '最大内存', '4096', 'MB', '4 GB', '跟随目录设置', '独立目录参数优先', '游戏版本', '显示快照版本', '内置服务', '由启动器统一提供', '关于 MCL', '打开数据目录', '恢复默认', '保存设置'. Keep the entire screen crisp at about 1536x1024, front facing, no laptop, no surrounding scene or presentation labels.
```

