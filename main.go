package main

import (
	"flag"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"log"
	"mcl/internal/launcher"
	"mcl/internal/nativeui"
	"os"
)

func main() {
	renderPath := flag.String("render", "", "Export a native UI screenshot")
	page := flag.String("page", "home", "Initial launcher page")
	lang := flag.String("lang", "", "Override UI language (zh-CN or en-US)")
	width := flag.Int("width", 1536, "Window width in DIPs")
	height := flag.Int("height", 1024, "Window height in DIPs")
	flag.Parse()
	s, e := launcher.New(os.Getenv("MCL_DATA_DIR"))
	if e != nil {
		log.Fatal(e)
	}
	app, e := nativeui.New(s)
	if e != nil {
		log.Fatal(e)
	}
	if *lang != "" {
		app.SetLanguage(*lang)
	}
	if *renderPath != "" {
		if e = app.RenderPage(*renderPath, *page, *width, *height); e != nil {
			log.Fatal(e)
		}
		return
	}
	mygo.App.SetName("MCL")
	mygo.App.WhenReady(func() {
		mygo.Theme.SetSource(mygo.ThemeLight)
		win := mygo.NewWindow(mygo.WindowOptions{Title: "MCL · Minecraft Launcher", Width: *width, Height: *height, MinWidth: 900, MinHeight: 540, TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 42, TrafficLightPosition: &mygo.Point{X: 20, Y: 21}, BackgroundColor: "#f5f7fb", Content: ui.View(app.View)})
		app.Attach(win)
		app.ShowPage(*page)
	})
	if e = mygo.App.Run(); e != nil {
		log.Fatal(e)
	}
}
