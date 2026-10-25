package app

import (
	"log"
	"os"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/nativeui"
)

func runDesktop(o options) error {
	mygo.App.SetName("YoloMCL")
	mygo.App.SetLanguages(nativeui.LangEN, nativeui.LangZH)
	mygo.App.WhenReady(func() {
		// Font registration must follow native application initialization.
		nativeui.PreloadAssets()
		app, err := newApplication()
		if err != nil {
			log.Fatal(err)
		}
		if err := configureRendering(app.HardwareAcceleration()); err != nil {
			log.Printf("configure launcher renderer: %v", err)
		}
		if o.language != "" {
			app.OverrideLanguage(o.language)
		}
		app.BindSystemLanguage()
		mygo.Theme.SetSource(mygo.ThemeLight)
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: "YoloMCL", Width: o.width, Height: o.height,
			MinWidth: 900, MinHeight: 540,
			TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 40,
			TrafficLightPosition: &mygo.Point{X: 18, Y: 20},
			BackgroundColor:      "#f5f7fb", Content: ui.View(app.View),
		})
		_ = win.SetIcon(nativeui.BrandIconPNG())
		app.Attach(win)
		app.ShowPage(o.page)
		win.Invalidate()
	})
	return mygo.App.Run()
}

// configureRendering applies the Windows preference before creating a window.
// Other platforms retain their default renderer; an explicit MYGO_GPU wins.
func configureRendering(hardwareAcceleration bool) error {
	if runtime.GOOS != "windows" || hardwareAcceleration {
		return nil
	}
	if _, explicit := os.LookupEnv("MYGO_GPU"); explicit {
		return nil
	}
	return os.Setenv("MYGO_GPU", "0")
}
