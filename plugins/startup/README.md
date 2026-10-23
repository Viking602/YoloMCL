# MCL startup plugin

This plugin lives in the MCL repository and registers through `mygo.Use` as
`mcl-startup`. It does not modify, replace or vendor the MyGo Go module. There
are no `github.com/egoist/mygo/internal/...` imports or fabricated module paths.

The main window first paints its real, interactive widget tree with MyGo's
software renderer. After that frame is presented, Windows warms an independent
D3D11 device and a 16×16 invisible draw in a worker. Waiting for presentation
keeps driver loading and native initialization from competing with the first
frame; it uses a lifecycle signal, not a fixed delay. When preparation completes and input is idle, the
same UI engine switches to the normal GPU renderer. Widgets, scroll position,
focus and animations are retained. No second visible window, splash image or
loading overlay is used.

`ContentAfter(prepare, view)` also allows application configuration, the cached
instance inventory and immutable assets to load in a worker while MyGo creates
the native window. The worker starts after platform initialization, so loading
font libraries does not compete with that initialization. The preparation callback joins
that worker on the UI thread, immediately before the UI engine attaches. It runs
once, including if the content is reattached. The first frame is still the real
page; there is no placeholder frame. Window, locale and graphics APIs stay on
the UI thread on Windows, macOS and Linux.

```go
accelerator := startup.New()
mygo.Use(accelerator.Plugin()) // before loading application data
// WhenReady:
win := mygo.NewWindow(mygo.WindowOptions{
    Content: accelerator.Content(app.View),
})
```

## Compatibility boundary

MyGo 0.5.5 has no public renderer reset API. `compat.go` therefore contains a
small, explicitly version-pinned compatibility bridge: it forwards the public
`Content.AttachContent` signature using Go type inference and inspects the
connection's exported event/lifecycle callbacks using reflection. After the
initial software frames, it sends the existing `SurfaceRenew` event. It does
not inspect or alter the UI engine or GPU renderer's private memory.

The plugin is disabled if the dependency version differs from `v0.5.5`, a Go
module replacement is present, or the callback signatures do not match. The
normal MyGo startup and its GPU/software fallback then remain available.
`dependency_test.go` verifies the event numbers and the exact shader assets
against the pinned upstream source. Review this bridge when upgrading MyGo;
prefer a public renderer preparation/reset API if one becomes available.

`MYGO_GPU=0` continues to select software rendering. `MCL_STARTUP_GPU=0`
disables the plugin for diagnostics. The first frame's temporary `MYGO_GPU`
override is restored synchronously on the UI thread, including after a panic.
Window closure cancels pending handoffs and idle timers. Driver calls may not
be cancellable, but they never hold the UI thread during warmup.

## Platforms

- Windows AMD64/ARM64: independent, thread-owned D3D11 warmup. DLLs load from
  the system directory. Resources are released when preparation finishes.
- macOS AMD64/ARM64: software first frame, then normal Metal initialization
  on MyGo's UI thread. No Cocoa or window objects are used by a worker.
- Linux AMD64/ARM64: software first frame, then normal OpenGL initialization
  on MyGo's UI thread with the surface's own context current.

The driver still creates the real window's GPU device during handoff. A short
pause can remain; the plugin waits for a release of held pointer buttons and
180 ms without input before doing this work. Performance numbers from Windows
are not evidence of macOS/Linux runtime performance.

This plugin runs after the OS starts the process. It cannot reduce time spent
inside Windows `CreateProcess` before the Go runtime executes. Startup reports
must distinguish launch request → process creation, process creation → Go main,
and Go main → presented frame. New-binary first execution is not equivalent to
a reboot with a cold filesystem cache. Do not attribute pre-execution delays to
antivirus without a privileged trace, or change security settings for a score.

The launcher's release scripts also select upstream's `mygo_noinspector` build
tag and `github.com/egoist/mygo.production=1` linker setting. This removes UI
inspector code and development handle checks on all platforms. It does not
bypass OS image validation or guarantee a shorter antivirus scan for a new
binary. Development builds retain the inspector.

## Assets and scanning

`assets/mygo-0.5.5/{vertex,pixel}.dxbc` are unmodified compiled bytecode from
MyGo's `internal/gpu/d3d11/shaders.go`, under the included MIT license. They
are embedded only in Windows builds. The input layout and native structures
are checked by tests. No shader compiler, shader source discovery or disk
scan runs at startup. The plugin does not read game files or user directories.
The launcher's inventory cache and bounded Minecraft/runtime scans remain in
`internal/launcher` and apply to all platforms.
