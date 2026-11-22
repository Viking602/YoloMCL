package nativeui

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type horizonUpdateState struct {
	info          *launcher.HorizonUpdateInfo
	checked       bool
	selectedBuild string
}

func horizonUpdateKey(rootID, versionID string) string {
	return rootID + "|" + versionID
}

func (a *App) horizonUpdateFor(rootID, versionID string) *horizonUpdateState {
	if a.horizonUpdates == nil {
		return nil
	}
	return a.horizonUpdates[horizonUpdateKey(rootID, versionID)]
}

func (a *App) syncHorizonUpdateState(r launcher.Instance, v launcher.VersionSummary) *horizonUpdateState {
	st := a.horizonUpdateFor(r.ID, v.ID)
	if st == nil || st.info == nil || v.LoaderVersion == "" {
		return st
	}
	if st.info.CurrentBuild != v.LoaderVersion {
		st.info.CurrentBuild = v.LoaderVersion
		st.info.HasUpdate = st.info.Latest != nil && st.info.Latest.BuildID != "" && st.info.Latest.BuildID != v.LoaderVersion
		if st.info.HasUpdate && st.info.Latest != nil {
			st.selectedBuild = st.info.Latest.BuildID
		} else {
			st.selectedBuild = v.LoaderVersion
		}
	}
	return st
}

func (a *App) horizonUpgradeJob(rootID, versionID string) *launcher.Job {
	for i := range a.snapshot.Jobs {
		j := &a.snapshot.Jobs[i]
		if j.State != "running" && j.State != "queued" {
			continue
		}
		if j.TitleKey != "horizon.upgrade" && j.TitleKey != "horizon.upgradeLatest" {
			continue
		}
		if j.TargetRootID == rootID && (j.TargetVersion == "" || j.TargetVersion == versionID) {
			return j
		}
	}
	return nil
}

// autoCheckHorizon quietly checks for Horizon build updates once per session
// per instance without flashing global state or showing offline error toasts.
func (a *App) autoCheckHorizon(r launcher.Instance, v launcher.VersionSummary) {
	if a.service == nil || a.ctx == nil || loaderKey(v.Loader) != "horizon" || v.ID == "" {
		return
	}
	key := horizonUpdateKey(r.ID, v.ID)
	if a.horizonUpdates == nil {
		a.horizonUpdates = map[string]*horizonUpdateState{}
	}
	st := a.horizonUpdates[key]
	if st != nil && st.checked {
		return
	}
	if st == nil {
		st = &horizonUpdateState{}
		a.horizonUpdates[key] = st
	}
	st.checked = true
	rootID, versionID := r.ID, v.ID
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
		defer cancel()
		info, err := a.service.CheckHorizonUpdate(ctx, rootID, versionID, false)
		if err != nil {
			return
		}
		a.post(func() {
			a.storeHorizonUpdate(rootID, versionID, info)
		})
	}()
}

func (a *App) storeHorizonUpdate(rootID, versionID string, info launcher.HorizonUpdateInfo) {
	if a.horizonUpdates == nil {
		a.horizonUpdates = map[string]*horizonUpdateState{}
	}
	key := horizonUpdateKey(rootID, versionID)
	st := a.horizonUpdates[key]
	if st == nil {
		st = &horizonUpdateState{}
		a.horizonUpdates[key] = st
	}
	cp := info
	st.info = &cp
	st.checked = true
	if st.selectedBuild == "" || (info.HasUpdate && info.Latest != nil && st.selectedBuild == info.CurrentBuild) {
		if info.HasUpdate && info.Latest != nil {
			st.selectedBuild = info.Latest.BuildID
		} else {
			st.selectedBuild = info.CurrentBuild
		}
	}
}

func (a *App) checkHorizonUpdate(rootID, versionID string, force, notify bool) {
	a.execute("checkHorizonUpdate", map[string]any{
		"rootId":  rootID,
		"version": versionID,
		"force":   force,
	}, func(raw json.RawMessage) {
		info := decode[launcher.HorizonUpdateInfo](raw)
		a.storeHorizonUpdate(rootID, versionID, info)
		if !notify {
			return
		}
		if info.HasUpdate && info.Latest != nil {
			a.notice(fmt.Sprintf(a.L("horizon.found_update_notice"), info.Latest.BuildID))
		} else {
			a.notice(a.L("horizon.up_to_date_notice"))
		}
	})
}

func (a *App) upgradeHorizon(rootID, versionID, targetBuild string) {
	a.execute("upgradeHorizon", map[string]string{
		"rootId":        rootID,
		"version":       versionID,
		"loaderVersion": targetBuild,
	}, func(json.RawMessage) {
		a.notice(a.L("horizon.upgrade_started"))
	})
}

// heroHorizonStatus shows an inline update chip in the instance hero for
// Horizon instances, matching heroPackStatus's weight and spacing.
func (a *App) heroHorizonStatus(c *ui.Context, r launcher.Instance, v launcher.VersionSummary) {
	s := uiScale(c)
	a.autoCheckHorizon(r, v)
	if a.horizonUpgradeJob(r.ID, v.ID) != nil || a.instancePending("upgradeHorizon") {
		ui.Row(c.Key("hero-horizon-upgrading")).Height(24*s).PaddingX(8*s).Gap(5*s).Radius(radiusChip*s).Background(lobeLight.InfoBg).Border(1, lobeLight.InfoBorder).AlignItems(ui.Center).Shrink(0).Children(func() {
			statusSpinner(c, 11*s)
			ui.Text(c, a.L("horizon.upgrading")).FontSize(typeFootnote * s).FontWeight(550).TextColor(lobeLight.InfoText).NoWrap()
		})
		return
	}
	st := a.syncHorizonUpdateState(r, v)
	if st == nil || st.info == nil || st.info.Latest == nil {
		return
	}
	info := st.info
	if info.Latest.BuildID != "" && info.Latest.BuildID != v.LoaderVersion {
		text := a.L("horizon.update_to") + info.Latest.BuildID
		b := tooltip(c, ui.ButtonBase(c.Key("hero-horizon-update")).Label(text), text+"\n"+a.L("horizon.review_and_update")).
			Height(24*s).MinWidth(0).PaddingX(8*s).Gap(4*s).Radius(radiusChip*s).
			Background(warning.Alpha(.14)).Border(1, warning.Alpha(.30)).AlignItems(ui.Center).Shrink(1)
		if b.Children(func() {
			ui.Icon(c, icons["download"]).Size(12*s, 12*s).TextColor(ui.Hex("#b26a00")).Shrink(0)
			ui.Text(c, text).FontSize(typeFootnote * s).FontWeight(650).TextColor(ui.Hex("#b26a00")).SingleLine().MinWidth(0).Shrink(1)
		}).Clicked() && !a.instancePending("upgradeHorizon") {
			a.upgradeHorizon(r.ID, v.ID, info.Latest.BuildID)
		}
		return
	}
	heroChip(c, "check", a.L("horizon.up_to_date"), false)
}

// ovHorizonActions keeps in-place Horizon update actions in one compact row
// inside the Overview Runtime panel (fits within the fixed 340px panel height).
func (a *App) ovHorizonActions(c *ui.Context, r launcher.Instance, v launcher.VersionSummary) {
	s := uiScale(c)
	a.autoCheckHorizon(r, v)
	st := a.syncHorizonUpdateState(r, v)
	upgrading := a.horizonUpgradeJob(r.ID, v.ID) != nil || a.instancePending("upgradeHorizon")
	checking := a.instancePending("checkHorizonUpdate")
	hasUpdate := st != nil && st.info != nil && st.info.Latest != nil && st.info.Latest.BuildID != "" && st.info.Latest.BuildID != v.LoaderVersion

	ui.Box(c).FillWidth().Height(1).Shrink(0).Background(line).Margin(4*s, 0, 6*s, 0)
	ui.Row(c.Key("ov-horizon-actions")).FillWidth().Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		if hasUpdate {
			lbl := a.L("horizon.upgrade_latest")
			if upgrading {
				lbl = a.L("horizon.upgrading")
			}
			if fixedButton(c, "horizon-upgrade-latest", lbl, "download", 0, btnSm*s, true, upgrading) && a.overviewActionReady("ov-run") {
				a.upgradeHorizon(r.ID, v.ID, st.info.Latest.BuildID)
			}
		}
		checkLbl := a.L("horizon.check_update")
		if checking {
			checkLbl = a.L("horizon.checking")
		}
		if fixedButton(c, "horizon-check-update", checkLbl, "refresh", 0, btnSm*s, false, checking || upgrading) && a.overviewActionReady("ov-run") {
			a.checkHorizonUpdate(r.ID, v.ID, true, true)
		}
		if fixedButton(c, "horizon-switch-build", a.L("horizon.switch_build"), "layers", 0, btnSm*s, false, upgrading) && a.overviewActionReady("ov-run") {
			a.setInstTab("settings")
		}
	})
}

// horizonBuildOptions combines published releases and locally cached builds
// into one deduplicated selector list.
func (a *App) horizonBuildOptions(v launcher.VersionSummary, info *launcher.HorizonUpdateInfo) (options []string, latestID string, cachedOnly map[string]bool) {
	seen := map[string]bool{}
	cachedOnly = map[string]bool{}
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		options = append(options, id)
	}
	if info != nil {
		if info.Latest != nil {
			latestID = info.Latest.BuildID
		}
		for _, rel := range info.Releases {
			add(rel.BuildID)
		}
		for _, loc := range info.LocalBuilds {
			if !seen[loc] {
				cachedOnly[loc] = true
				add(loc)
			}
		}
	}
	if v.LoaderVersion != "" && !seen[v.LoaderVersion] {
		add(v.LoaderVersion)
	}
	return options, latestID, cachedOnly
}

// horizonBuildUpgradeSection renders the in-place build upgrade & switcher
// inside Instance Settings -> Native Runtime.
func (a *App) horizonBuildUpgradeSection(c *ui.Context, r launcher.Instance, v launcher.VersionSummary) {
	s := uiScale(c)
	a.autoCheckHorizon(r, v)
	key := horizonUpdateKey(r.ID, v.ID)
	if a.horizonUpdates == nil {
		a.horizonUpdates = map[string]*horizonUpdateState{}
	}
	st := a.syncHorizonUpdateState(r, v)
	if st == nil {
		st = &horizonUpdateState{selectedBuild: v.LoaderVersion}
		a.horizonUpdates[key] = st
	}
	var info *launcher.HorizonUpdateInfo
	if st.info != nil {
		info = st.info
	}
	options, latestID, cachedOnly := a.horizonBuildOptions(v, info)
	if st.selectedBuild == "" || !slices.Contains(options, st.selectedBuild) {
		if latestID != "" && latestID != v.LoaderVersion {
			st.selectedBuild = latestID
		} else if len(options) > 0 {
			st.selectedBuild = options[0]
		}
	}
	upgrading := a.horizonUpgradeJob(r.ID, v.ID) != nil || a.instancePending("upgradeHorizon")
	checking := a.instancePending("checkHorizonUpdate")
	hasUpdate := latestID != "" && latestID != v.LoaderVersion

	ui.Box(c).FillWidth().Height(1).Background(line)
	ui.Column(c.Key("horizon-build-upgrade")).FillWidth().Gap(10 * s).Children(func() {
		ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			label(c, a.L("horizon.build_section")).FontSize(typeFootnote * s).FontWeight(600)
			ui.Spacer(c).Grow(1)
			switch {
			case upgrading:
				quietTag(c, a.L("horizon.upgrading"), accent)
			case hasUpdate:
				quietTag(c, fmt.Sprintf(a.L("horizon.update_available"), latestID), warning)
			case info != nil && info.Latest != nil:
				quietTag(c, a.L("horizon.up_to_date"), success)
			}
		})

		display := func(opt string) string {
			var tags []string
			if opt == latestID {
				tags = append(tags, a.L("horizon.latest_tag"))
			}
			if opt == v.LoaderVersion {
				tags = append(tags, a.L("horizon.current_tag"))
			}
			if cachedOnly[opt] {
				tags = append(tags, a.L("horizon.local_cached_tag"))
			}
			if len(tags) == 0 {
				return opt
			}
			return opt + "  (" + strings.Join(tags, " / ") + ")"
		}

		ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			if len(options) > 0 {
				selectBoxFull(c, "horizon-build-select", a.L("horizon.target_build"), &st.selectedBuild, options, nil, display).Grow(1).MinWidth(0)
			}
			checkLbl := a.L("horizon.check_update")
			if checking {
				checkLbl = a.L("horizon.checking")
			}
			if fixedButton(c, "horizon-settings-check", checkLbl, "refresh", 0, btnMd*s, false, checking || upgrading) {
				a.checkHorizonUpdate(r.ID, v.ID, true, true)
			}
		})

		ui.Row(c).FillWidth().Wrap().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			if st.selectedBuild != "" && st.selectedBuild != v.LoaderVersion {
				btnLbl := a.L("horizon.apply_build")
				if st.selectedBuild == latestID {
					btnLbl = a.L("horizon.upgrade_latest")
				}
				if upgrading {
					btnLbl = a.L("horizon.upgrading")
				}
				if fixedButton(c, "horizon-settings-upgrade", btnLbl, "download", 0, btnMd*s, true, upgrading) {
					a.upgradeHorizon(r.ID, v.ID, st.selectedBuild)
				}
			} else if hasUpdate {
				btnLbl := a.L("horizon.upgrade_latest")
				if upgrading {
					btnLbl = a.L("horizon.upgrading")
				}
				if fixedButton(c, "horizon-settings-upgrade", btnLbl, "download", 0, btnMd*s, true, upgrading) {
					a.upgradeHorizon(r.ID, v.ID, latestID)
				}
			}
		})

		label(c, a.L("horizon.upgrade_keep_data")).FontSize(typeCaption * s).FillWidth().MinWidth(0)
	})
}
