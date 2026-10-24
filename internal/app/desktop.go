package app

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/nativeui"
	"github.com/Viking602/YoloMCL/plugins/startup"
)

func runDesktop(o options) error {
	accelerator := startup.New()
	mygo.Use(accelerator.Plugin())
	// Prepare inventory while the OS creates the window. ContentAfter joins
	// this worker before attaching the view, keeping the first frame complete.
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
		if o.language != "" {
			app.OverrideLanguage(o.language)
		}
		app.BindSystemLanguage()
	}, func(c *ui.Context) { app.View(c) })
	mygo.App.SetName("YoloMCL")
	mygo.App.SetLanguages(nativeui.LangEN, nativeui.LangZH)
	mygo.App.WhenReady(func() {
		// Font registration must follow OS initialization to avoid loader-lock
		// contention. Only application preparation overlaps window creation.
		nativeui.PreloadAssets()
		go func() {
			app, err := newApplication()
			prepared <- result{app, err}
		}()
		mygo.Theme.SetSource(mygo.ThemeLight)
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: "YoloMCL", Width: o.width, Height: o.height,
			MinWidth: 900, MinHeight: 540,
			TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 40,
			TrafficLightPosition: &mygo.Point{X: 18, Y: 20},
			BackgroundColor:      "#f5f7fb", Content: content,
		})
		_ = win.SetIcon(nativeui.BrandIconPNG())
		app.Attach(win)
		app.ShowPage(o.page)
		win.Invalidate()
	})
	return mygo.App.Run()
}
