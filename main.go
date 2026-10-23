package main

import (
	"flag"
	"log"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
	"github.com/Viking602/YoloMCL/internal/diagnostics"
	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/nativeui"
	"github.com/Viking602/YoloMCL/plugins/startup"
)

func main() {
	mygo.App.SetVersion(buildinfo.Current().Version)
	renderPath := flag.String("render", "", "Export a native UI screenshot")
	page := flag.String("page", "home", "Initial launcher page")
	lang := flag.String("lang", "", "Override UI language (zh-CN or en-US)")
	width := flag.Int("width", 1536, "Window width in DIPs")
	height := flag.Int("height", 1024, "Window height in DIPs")
	diagPath := flag.String("diag", "", "Write mod list timings and icon coverage (QA)")
	flag.Parse()
	accelerator := startup.New()
	if *renderPath == "" && *diagPath == "" {
		mygo.Use(accelerator.Plugin())
	}
	if *diagPath != "" {
		s, e := launcher.New(os.Getenv("MCL_DATA_DIR"))
		if e != nil {
			log.Fatal(e)
		}
		if f := os.Getenv("MCL_DIAG_SCAN"); f != "" {
			if e = diagnostics.WriteScanReport(s, *diagPath, f); e != nil {
				log.Fatal(e)
			}
			return
		}
		if e = diagnostics.WriteReport(s, *diagPath); e != nil {
			log.Fatal(e)
		}
		return
	}
	if *renderPath != "" {
		nativeui.PreloadAssets()
		app, e := newApplication()
		if e != nil {
			log.Fatal(e)
		}
		if *lang != "" {
			app.OverrideLanguage(*lang)
		}
		if e := app.RenderPage(*renderPath, *page, *width, *height); e != nil {
			log.Fatal(e)
		}
		return
	}
	// Configuration/inventory and immutable assets can be prepared while the
	// OS creates the native window. ContentAfter joins this worker before the
	// UI engine attaches, so the first frame still contains the real page.
	type result struct {
		app *nativeui.App
		err error
	}
	prepared := make(chan result, 1)
	var app *nativeui.App
	content := accelerator.ContentAfter(func() {
		r := <-prepared
		if r.err != nil {
			log.Fatal(r.err)
		}
		app = r.app
		if *lang != "" {
			app.OverrideLanguage(*lang)
		}
		app.BindSystemLanguage()
	}, func(c *ui.Context) { app.View(c) })
	mygo.App.SetName("YoloMCL")
	mygo.App.SetLanguages(nativeui.LangEN, nativeui.LangZH)
	mygo.App.WhenReady(func() {
		// Finish platform initialization before registering fonts. Loading
		// system font libraries concurrently with it can contend on the OS
		// loader lock. The worker instead overlaps the native window below.
		nativeui.PreloadAssets()
		go func() {
			app, err := newApplication()
			prepared <- result{app, err}
		}()
		mygo.Theme.SetSource(mygo.ThemeLight)
		win := mygo.NewWindow(mygo.WindowOptions{Title: "YoloMCL", Width: *width, Height: *height, MinWidth: 900, MinHeight: 540, TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 40, TrafficLightPosition: &mygo.Point{X: 18, Y: 20}, BackgroundColor: "#f5f7fb", Content: content})
		_ = win.SetIcon(nativeui.BrandIconPNG())
		app.Attach(win)
		app.ShowPage(*page)
		// Initial pages without background work still need an immediate frame.
		// Do not depend on Java results or the periodic snapshot to wake drawing.
		win.Invalidate()
	})
	if e := mygo.App.Run(); e != nil {
		log.Fatal(e)
	}
}

func newApplication() (*nativeui.App, error) {
	s, e := launcher.New(os.Getenv("MCL_DATA_DIR"))
	if e != nil {
		return nil, e
	}
	app, e := nativeui.New(s)
	return app, e
}
