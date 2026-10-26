package nativeui

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// javaSelectBox shares the installed-runtime artwork across all Java pickers.
// Resolve icons by executable path, never the displayed name: two installations
// can have the same version/vendor, and a browsed path may not be scanned yet.
func (a *App) javaSelectBox(c *ui.Context, key, labelText string, selected *string, options []string, paths map[string]string) ui.Element {
	s := uiScale(c)
	return selectBoxImpl(c, key, labelText, selected, options, nil, nil, func(option string) {
		path := paths[option]
		if path == "" {
			ui.Box(c).Label("Java 自动选择图标").Size(20*s, 20*s).Shrink(0).Radius(5*s).Background(panel).Border(1, line.Alpha(.65)).Center().Children(func() {
				ui.Icon(c, icons["settings"]).Size(13*s, 13*s).TextColor(accent)
			})
			return
		}
		j := launcher.InstalledJava{Home: filepath.Dir(filepath.Dir(path))}
		for _, known := range a.java {
			if known.Path == path || runtime.GOOS == "windows" && strings.EqualFold(known.Path, path) {
				j.JavaInfo = known
				break
			}
		}
		javaBrandIcon(c, javaVendor(a, j), 20*s)
	}, nil)
}

// Keep known runtimes visible during a partial/cancelled refresh. A completed
// scan replaces the list so removed installations are still dropped.
func mergeJavaScanRows(known, found []launcher.JavaInfo) []launcher.JavaInfo {
	key := func(p string) string {
		p = filepath.Clean(p)
		if runtime.GOOS == "windows" {
			p = strings.ToLower(p)
		}
		return p
	}
	out := append([]launcher.JavaInfo(nil), known...)
	index := make(map[string]int, len(out))
	for i, j := range out {
		index[key(j.Path)] = i
	}
	for _, j := range found {
		if i, ok := index[key(j.Path)]; ok {
			out[i] = j
		} else {
			index[key(j.Path)] = len(out)
			out = append(out, j)
		}
	}
	return out
}

// MyGo calls this after presenting the first frame on every desktop platform.
// Initial page selection and rendering may request Java before then; defer all
// cache validation and discovery until this gate opens.
func (a *App) startBackgroundJava() {
	if a.javaReadyToScan {
		return
	}
	a.javaReadyToScan = true
	a.ensureJavaScan()
}

func (a *App) ensureJavaScan() {
	if a.javaReadyToScan && !a.javaScannedOnce && !a.javaScanning {
		a.startJavaScan(false)
	}
}

func (a *App) startJavaScan(deep bool) {
	if a.service == nil || a.ctx.Err() != nil {
		return
	}
	if a.javaScanning {
		if !deep || a.javaDeep {
			return
		}
		if a.javaScanCancel != nil {
			a.javaScanCancel()
		}
	}
	a.javaScanRevision++
	revision := a.javaScanRevision
	var ctx context.Context
	var cancel context.CancelFunc
	if deep {
		ctx, cancel = context.WithCancel(a.ctx)
	} else {
		ctx, cancel = context.WithTimeout(a.ctx, 12*time.Second)
	}
	a.javaScanCancel = cancel
	a.javaScanning = true
	a.javaDeep = deep
	a.javaPhase = "正在检测已安装的 Java"
	a.javaScanned = 0
	a.javaElapsed = 0
	previous := append([]launcher.JavaInfo(nil), a.java...)
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer cancel()
		if len(previous) == 0 {
			previous = a.service.CachedJavaList()
			cached := append([]launcher.JavaInfo(nil), previous...)
			a.post(func() {
				if revision == a.javaScanRevision {
					a.java = cached
				}
			})
		}
		_, e := a.service.ScanJava(ctx, deep, func(p launcher.JavaScanProgress) {
			keepPrevious := p.Running || ctx.Err() != nil
			a.post(func() {
				if revision != a.javaScanRevision {
					return
				}
				if keepPrevious {
					a.java = mergeJavaScanRows(previous, p.Found)
				} else {
					a.java = p.Found
				}
				a.javaScanning = p.Running
				a.javaScanned = p.Visited
				a.javaElapsed = p.ElapsedMS
				a.javaPhase = p.Phase
				a.javaScannedOnce = true
			})
		})
		a.post(func() {
			if revision != a.javaScanRevision {
				return
			}
			a.javaScanning = false
			if e == context.Canceled {
				a.javaPhase = "扫描已停止"
			} else if e != nil {
				a.javaPhase = a.L("java_settings.quick_scan_finished_check_deeper_in_the_same")
			} else {
				a.javaPhase = a.formatJavaScanSummary(len(a.java), a.javaElapsed)
			}
		})
	}()
}

func (a *App) javaDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ui.Row(c).Gap(10 * s).Children(func() {
		if button(c, "java-quick", t.JavaQuickScanBtn, "search", !a.javaDeep, a.javaScanning && !a.javaDeep) {
			a.startJavaScan(false)
		}
		if button(c, "java-deep", t.JavaDeepScanBtn, "folder", a.javaDeep, a.javaScanning && a.javaDeep) {
			a.startJavaScan(true)
		}
		if a.javaScanning && button(c, "java-stop", t.JavaStopScanBtn, "cross", false, false) && a.javaScanCancel != nil {
			a.javaScanCancel()
		}
	})
	status := a.translatePhase(a.javaPhase)
	if a.javaScanning {
		status = a.formatJavaScanProgress(a.javaPhase, a.javaScanned, len(a.java))
	}
	statusLine(c, status, muted, a.javaScanning)
	label(c, t.JavaScanHint).FontSize(typeCallout * s)
	textField(c, &a.javaFilter).Label(t.JavaFilterAria).Placeholder(t.JavaFilterPlaceholder).FillWidth()
	if button(c, "java-auto", t.JavaUseAutoBtn, "check", a.settings.JavaPath == "", false) {
		a.settings.JavaPath = ""
		a.persistSettings("")
		a.closeDialog()
	}
	for _, item := range a.java {
		j := item
		if a.javaFilter != "" && !strings.Contains(strings.ToLower(j.Path+" "+j.Version+" "+j.Vendor), strings.ToLower(a.javaFilter)) {
			continue
		}
		ui.Row(c.Key(j.Path)).FillWidth().Gap(14*s).Padding(14*s).Radius(16*s).Background(raised).Border(1, glassEdge).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Clip().Gap(5 * s).Children(func() {
				ui.Text(c, "Java "+j.Version).Bold().TextColor(ink)
				meta := javaDisplay(j.Vendor)
				if j.Architecture != "" {
					meta += " / " + j.Architecture
				}
				if meta != "" {
					label(c, meta)
				}
				tooltip(c, label(c, j.Path).FillWidth().FontSize(typeCallout*s).SingleLine(), j.Path)
			})
			if button(c, "pick-"+j.Path, t.JavaUseBtn, "check", a.settings.JavaPath == j.Path, false) {
				a.settings.JavaPath = j.Path
				a.persistSettings("")
				a.closeDialog()
				if a.javaDeep && a.javaScanCancel != nil {
					a.javaScanCancel()
				}
			}
		})
	}
	if len(a.java) == 0 && !a.javaScanning {
		statusLine(c, t.JavaEmptyHint, muted, false)
	}
}
