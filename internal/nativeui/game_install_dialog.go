package nativeui

import (
	"github.com/egoist/mygo/ui"
)

func (a *App) installDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	label(c, t.InstallDesc)

	// Step 1: Target Location
	ui.Column(c).FillWidth().Gap(10*s).Padding(16*s).Radius(12*s).Background(raised).Border(1, line).Children(func() {
		ui.Text(c, t.InstallStep1Title).FontSize(typeSubhead * s).Bold().TextColor(ink)
		if a.root() != nil {
			if selectBox(c, "install-target", t.InstallTargetLabel, &a.form.Target, []string{t.InstallTargetCurrent, t.InstallTargetNew}).FillWidth().Changed() {
				a.form.Name = ""
			}
		}
		if normalizeInstallTarget(a.form.Target) == "current" && a.root() != nil {
			pathLabel(c, a.root().Path, 13*s)
		} else {
			a.folderField(c, t.InstallParentLabel, &a.form.Parent)
			field(c, t.InstallNameLabel, &a.form.Name, t.InstallNamePlaceholder)
		}
	})

	// Step 2: Official Minecraft Version
	ui.Column(c).FillWidth().Gap(10*s).Padding(16*s).Radius(12*s).Background(raised).Border(1, line).Children(func() {
		ui.Text(c, t.InstallStep2Title).FontSize(typeSubhead * s).Bold().TextColor(ink)
		a.minecraftVersionPicker(c)
	})

	// Step 3: Mod Loader & Version with Interactive Icon Cards + Scrollable Version Selector
	ui.Column(c).FillWidth().Gap(10*s).Padding(16*s).Radius(12*s).Background(raised).Border(1, line).Children(func() {
		ui.Text(c, t.InstallStep3Title).FontSize(typeSubhead * s).Bold().TextColor(ink)
		ui.Row(c).FillWidth().Gap(8 * s).Children(func() {
			for _, item := range []struct {
				name, sub string
			}{
				{t.LoaderVanilla, t.InstallLoaderVanillaSub},
				{t.LoaderFabric, t.InstallLoaderFabricSub},
				{t.LoaderForge, t.InstallLoaderForgeSub},
				{t.LoaderNeoForge, t.InstallLoaderNeoSub},
				{t.LoaderQuilt, t.InstallLoaderQuiltSub},
			} {
				ld := item
				selected := loaderKey(a.form.Loader) == loaderKey(ld.name)
				cardBg := panel
				borderCol := line
				if selected {
					cardBg = accentSoft
					borderCol = accent
				}
				b := ui.ButtonBase(c.Key("loader-card-"+loaderKey(ld.name))).Label(ld.name).Grow(1).MinWidth(0).Padding(10*s, 8*s).Radius(10*s).Border(1, borderCol).AlignItems(ui.Center).Justify(ui.Center)
				if b.Hovered() && !selected {
					cardBg = fieldColor
				}
				b.Background(b.AnimateColor("bg", cardBg, fastAnim))
				if b.Children(func() {
					ui.Column(c).AlignItems(ui.Center).Gap(5 * s).Children(func() {
						if !loaderLogo(c, ld.name, 34*s) {
							ui.Icon(c, icons[loaderIconName(ld.name)]).Size(22*s, 22*s).TextColor(loaderTone(ld.name))
						}
						ui.Text(c, ld.name).FontSize(typeCallout * s).Bold().TextColor(ink).NoWrap()
						label(c, ld.sub).FontSize(typeCaption2 * s).SingleLine()
					})
				}).Clicked() && !selected {
					a.form.Loader = ld.name
					a.loadLoaderVersions()
				}
			}
		})
		if loaderSelectBox(c, "install-loader-select", t.InstallLoaderAria, &a.form.Loader, []string{t.LoaderVanilla, t.LoaderFabric, t.LoaderForge, t.LoaderNeoForge, t.LoaderQuilt}).FillWidth().Changed() {
			a.loadLoaderVersions()
		}
		isVanilla := loaderKey(a.form.Loader) == "vanilla"
		if !isVanilla {
			label(c, t.InstallLoaderVerLabel).FontWeight(600)
			if a.loaderLoading {
				statusLine(c, t.InstallFetchingLoader, muted, true)
			} else if a.loaderError != "" {
				if a.loadFailed(c, "loader-failed", failInline, a.loaderLoading) {
					a.loadLoaderVersions()
				}
			} else if len(a.loaders) == 0 {
				statusLine(c, t.InstallNoLoaderVer, muted, false)
			} else {
				selectBox(c, "install-loader-ver", t.InstallLoaderVerLabel, &a.form.LoaderVersion, a.loaders).FillWidth()
			}
		}
	})

	isCurrent := normalizeInstallTarget(a.form.Target) == "current" && a.root() != nil
	isVanilla := loaderKey(a.form.Loader) == "vanilla"
	invalid := a.form.Minecraft == "" || (!isVanilla && a.form.LoaderVersion == "") || (!isCurrent && (a.form.Parent == "" || a.form.Name == ""))
	if buttonWhy(c, "install-game-now", t.InstallConfirmBtn, "download", true, whyOf(a.dialogPending(), t.WhyBusy, invalid, t.WhyInstallInvalid), true) {
		root := ""
		if isCurrent {
			root = a.root().ID
		}
		a.execute("installGame", map[string]string{"rootId": root, "parent": a.form.Parent, "name": a.form.Name, "minecraft": a.form.Minecraft, "loader": loaderKey(a.form.Loader), "loaderVersion": a.form.LoaderVersion}, a.taskAdded)
	}
}
