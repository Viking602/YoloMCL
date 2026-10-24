// Package app assembles the launcher and owns its process lifecycle.
package app

import (
	"errors"
	"flag"
	"os"

	"github.com/egoist/mygo"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/nativeui"
)

type options struct {
	render, page, language, diagnostic string
	width, height                      int
}

func parseOptions(args []string) (options, error) {
	var o options
	flags := flag.NewFlagSet("YoloMCL", flag.ContinueOnError)
	flags.StringVar(&o.render, "render", "", "Export a native UI screenshot")
	flags.StringVar(&o.page, "page", "home", "Initial launcher page")
	flags.StringVar(&o.language, "lang", "", "Override UI language (zh-CN or en-US)")
	flags.IntVar(&o.width, "width", 1536, "Window width in DIPs")
	flags.IntVar(&o.height, "height", 1024, "Window height in DIPs")
	flags.StringVar(&o.diagnostic, "diag", "", "Write mod list timings and icon coverage (QA)")
	err := flags.Parse(args)
	return o, err
}

// Run starts the desktop application or an explicitly requested QA operation.
func Run(args []string) error {
	o, err := parseOptions(args)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	mygo.App.SetVersion(buildinfo.Current().Version)
	if o.diagnostic != "" {
		s, err := launcher.New(os.Getenv("MCL_DATA_DIR"))
		if err != nil {
			return err
		}
		if scan := os.Getenv("MCL_DIAG_SCAN"); scan != "" {
			return writeScanDiagnosticReport(s, o.diagnostic, scan)
		}
		return writeDiagnosticReport(s, o.diagnostic)
	}
	if o.render != "" {
		nativeui.PreloadAssets()
		app, err := newApplication()
		if err != nil {
			return err
		}
		if o.language != "" {
			app.OverrideLanguage(o.language)
		}
		return app.RenderPage(o.render, o.page, o.width, o.height)
	}
	return runDesktop(o)
}

func newApplication() (*nativeui.App, error) {
	s, err := launcher.New(os.Getenv("MCL_DATA_DIR"))
	if err != nil {
		return nil, err
	}
	return nativeui.New(s)
}
