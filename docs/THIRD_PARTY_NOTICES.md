# Third-party assets

- MyGo Native UI: [egoist/mygo](https://github.com/egoist/mygo), MIT. See module license.
- GitHub mark: [Primer Octicons](https://github.com/primer/octicons/blob/main/icons/mark-github-16.svg), Copyright (c) 2026 GitHub Inc., MIT. Full license embedded at `internal/nativeui/assets/LICENSES/octicons/LICENSE.txt`. The mark identifies the source repository link.
- Fusion Pixel Font 12px, simplified Chinese: [TakWolf/fusion-pixel-font](https://github.com/TakWolf/fusion-pixel-font), 2026.10.16 release. The bundled `pixel.otf` is a modified subset named **MCL Pixel Brand**, containing only the `YoloMCL` wordmark and space; glyph outlines and advances are unchanged. Rebuild it with `scripts/subset-brand-font.py` and the upstream OTF. It remains under SIL Open Font License 1.1; the license and incorporated font notices are included in `internal/nativeui/assets/OFL.txt` and `internal/nativeui/assets/LICENSES/`. User text uses the separate UI fonts, not this subset.
- Modrinth official logomark: original SVG from [Modrinth's source](https://github.com/modrinth/code/blob/main/apps/frontend/src/assets/images/logo.svg). Its brand color variable is resolved to Modrinth green for native rendering. Logo and trademark rights remain with their owner.
- CurseForge official anvil logomark: unmodified brand SVG from [Modrinth's external brand assets](https://github.com/modrinth/code/blob/main/apps/frontend/src/assets/images/external/curseforge.svg). Logo and trademark rights remain with their owner.
- Navigation outline icons and landscape illustration are drawn directly by this application.

## Default player skins

`internal/launcher/assets/skins/steve.png` and `alex.png` are unmodified Minecraft 26.1.2 textures, extracted from the installed official client. They are embedded for offline wardrobe previews and default account avatars. © Mojang AB / Microsoft; all rights remain with their owners. MCL is not affiliated with or endorsed by Mojang or Microsoft.

| Asset | Original client entry | SHA-256 |
| --- | --- | --- |
| `steve.png` | `assets/minecraft/textures/entity/player/wide/steve.png` | `06628f9cef88520da742ab280a01677ce17c99717f964ed89ed791673121094c` |
| `alex.png` | `assets/minecraft/textures/entity/player/slim/alex.png` | `99391ed9dff9581ec88daf23b9b028adc9ac91f52889edcff1c982d0923c0cff` |

MCL is an independent project. Minecraft, Microsoft, Modrinth and CurseForge names and logos identify their respective integrations and remain the property of their owners.

## Mod loader and game logos

`internal/nativeui/assets/loaders/*.png` are the projects' own official logos, embedded unmodified except for resizing (Fabric's 32 px pixel art is scaled up with nearest-neighbour, the Quilt logo is scaled down to 256 px). They identify the corresponding software in the version installer and game list only. All names and logos are trademarks of their respective owners; MCL is not affiliated with or endorsed by them.

| File | Owner | Source |
| --- | --- | --- |
| `vanilla.png` | Minecraft, © Mojang AB / Microsoft. "Minecraft" is a trademark of Mojang Synergies AB. | Official game window icon `icons/icon_256x256.png` from the Mojang asset index for 26.3: https://resources.download.minecraft.net/80/8030dd9dc315c0381d52c4782ea36c6baf6e8135 |
| `fabric.png` | FabricMC | Fabric Loader icon, https://github.com/FabricMC/fabric-loader/blob/master/src/main/resources/assets/fabricloader/icon.png (Apache-2.0 repository) |
| `forge.png` | MinecraftForge | Official site icon, https://files.minecraftforge.net/static/images/apple-touch-icon.png |
| `neoforge.png` | The NeoForged Project | Official site icon, https://neoforged.net/img/authors/neoforged.png |
| `quilt.png` | The Quilt Project | QuiltMC brand art, https://github.com/QuiltMC/art/blob/master/brand/1024png/quilt_logo_transparent.png |

## Built-in authentication server icons

Bundled in `internal/nativeui/assets/authicons/` so built-in servers show their icon offline. They are the sites' own favicons, used only to identify the service. All rights belong to their owners.

- `littleskin.png` — LittleSkin, fetched 2026-10-21 from https://littleskin.cn/favicon.png (unchanged).
- `ely.png` — Ely.by, fetched 2026-10-21 from https://ely.by/favicon.ico?v=2 (16×16 ICO converted to PNG).

## Java distribution icons

`internal/nativeui/assets/java/*.png` identify installed Java distributions and download sources. These official project or vendor marks were fetched on 2026-10-24 and resized proportionally to 64×64 pixels; SVG sources were rasterized without changing their artwork. They are embedded for offline display and shared across runtime rows. All names, logos, and trademarks remain the property of their respective owners; their use does not imply endorsement.

| File | Project or vendor | Original asset |
| --- | --- | --- |
| `zulu.png` | Azul Systems, for Azul Zulu | [Azul site icon](https://www.azul.com/wp-content/themes/azul/assets/img/azul-favicon-block.png) |
| `temurin.png` | Eclipse Foundation, Eclipse Temurin | [Temurin project mark](https://adoptium.net/images/projects/temurin-project.svg), [logo guide](https://adoptium.net/docs/logo-styleguide/) |
| `corretto.png` | Amazon Web Services, for Amazon Corretto | [AWS site icon](https://a0.awsstatic.com/libra-css/images/site/touch-icon-ipad-144-smile.png) |
| `liberica.png` | BellSoft, for Liberica JDK | [BellSoft site icon](https://bell-sw.com/assets/images/favicon.png) |
| `graalvm.png` | Oracle, GraalVM | [GraalVM project mark](https://www.graalvm.org/resources/img/home/graalvm_rabbit_icon.svg) |
| `jbr.png` | JetBrains, for JetBrains Runtime | [JetBrains site icon](https://www.jetbrains.com/apple-touch-icon.png?r=1234), [brand guidelines](https://www.jetbrains.com/company/brand/) |
| `oracle.png` | Oracle, for Oracle JDK | [Oracle site icon](https://www.oracle.com/asset/web/favicons/favicon-128.png) |

Copyright © 2026 [JetBrains s.r.o.](https://www.jetbrains.com/). JetBrains and the JetBrains logo are trademarks of JetBrains s.r.o.

Microsoft builds reuse the Microsoft four-square mark already used in account sign-in. Mojang's official launcher runtime source reuses the Minecraft game icon credited above. Unrecognized distributions use the application's generic runtime icon.

## JetBrains Mono

- Version: 2.304 (variable font `JetBrainsMono[wght].ttf`, upright only), from the official release
  https://github.com/JetBrains/JetBrainsMono/releases/tag/v2.304
  (`JetBrainsMono-2.304.zip`, SHA-256 `6f6376c6ed2960ea8a963cd7387ec9d76e3f629125bc33d1fdcd7eb7012f7bbf`).
- Embedded as `internal/nativeui/assets/JetBrainsMono.ttf`, registered under the family name "MCL JetBrains Mono".
- **Modified Version** (OFL §1): `head.unitsPerEm` changed 1000 → 1080 (glyphs render at 0.926× nominal size, to balance with the CJK fallback face) and the name table family renamed to "MCL JetBrains Mono". Outlines are unchanged. The original font declares no Reserved Font Name. The modified font remains under the SIL OFL 1.1.
- Copyright 2020 The JetBrains Mono Project Authors (https://github.com/JetBrains/JetBrainsMono).
- License: SIL Open Font License, Version 1.1 — full text in `internal/nativeui/assets/LICENSES/jetbrains-mono/OFL.txt`.
