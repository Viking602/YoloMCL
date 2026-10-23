package startup

import (
	"os"
	"reflect"
	"sync"
	"time"
)

// MyGo 0.5.5 exposes Content.AttachContent but its connection/event types are
// internal. Type inference lets us forward that exact signature without an
// internal import, a forged module path, unsafe casts or a dependency fork.
// Only this version-checked adapter inspects the connection's callback fields.
type content[C any] struct {
	done      <-chan struct{}
	presented func()
	attach    func(*C)
}

func adapt[C any](done <-chan struct{}, presented func(), attach func(*C)) *content[C] {
	return &content[C]{done: done, presented: presented, attach: attach}
}

func (c *content[C]) AttachContent(conn *C) {
	c.attach(conn)
	install(reflect.ValueOf(conn).Elem(), c.done, c.presented)
}

// Values of SurfaceEventKind in the pinned version. The compatibility test
// checks them against the dependency; no renderer state or layout is accessed.
const surfaceFrame, surfaceRenew = 0, 24

func install(conn reflect.Value, prepared <-chan struct{}, presented func()) bool {
	if !conn.IsValid() || conn.Kind() != reflect.Struct {
		return false
	}
	event, ready, detach, post := conn.FieldByName("Event"), conn.FieldByName("Ready"), conn.FieldByName("Detach"), conn.FieldByName("Post")
	if !event.IsValid() || !event.CanSet() || event.Kind() != reflect.Func || event.IsNil() {
		return false
	}
	et := event.Type()
	if et.NumIn() != 1 || et.NumOut() != 1 || et.Out(0).Kind() != reflect.Bool || et.In(0).Kind() != reflect.Struct {
		return false
	}
	kind, ok := et.In(0).FieldByName("Kind")
	if !ok || kind.Type.Kind() != reflect.Uint8 {
		return false
	}
	callback := reflect.TypeFor[func()]()
	if !ready.IsValid() || !ready.CanSet() || ready.Type() != callback || !detach.IsValid() || !detach.CanSet() || detach.Type() != callback || !post.IsValid() || post.Type() != reflect.TypeFor[func(func())]() || post.IsNil() {
		return false
	}
	// Save values, not references to the fields we replace below.
	original := reflect.ValueOf(event.Interface())
	onReady, _ := ready.Interface().(func())
	onDetach, _ := detach.Interface().(func())
	postMain := post.Interface().(func(func()))
	closed := make(chan struct{})
	var closeOnce, readyOnce sync.Once
	// Accessed only on the window's UI thread.
	stopped, pointerDown := false, false
	var lastInput time.Time
	var idleTimer *time.Timer
	event.Set(reflect.MakeFunc(et, func(args []reflect.Value) []reflect.Value {
		k := args[0].FieldByIndex(kind.Index).Uint()
		if k >= 2 && k <= 11 || k >= 21 && k <= 23 {
			lastInput = time.Now()
		}
		if k == 3 {
			pointerDown = true
		}
		if k == 4 || k == 13 {
			pointerDown = false
		}
		if k == surfaceFrame {
			var result []reflect.Value
			softwareFrame(func() { result = original.Call(args) })
			return result
		}
		return original.Call(args)
	}))
	detach.Set(reflect.ValueOf(func() {
		stopped = true
		if idleTimer != nil {
			idleTimer.Stop()
		}
		closeOnce.Do(func() { close(closed) })
		if onDetach != nil {
			onDetach()
		}
	}))
	ready.Set(reflect.ValueOf(func() {
		if onReady != nil {
			onReady()
		}
		readyOnce.Do(func() {
			if presented != nil {
				presented()
			}
			var handoff func()
			handoff = func() {
				if stopped {
					return
				}
				// A driver may still need a short device creation on the UI
				// thread. Do not put that work inside a click, drag or typing.
				const idle = 180 * time.Millisecond
				if wait := idle - time.Since(lastInput); pointerDown || wait > 0 {
					if pointerDown {
						wait = idle
					}
					idleTimer = time.AfterFunc(wait, func() { postMain(handoff) })
					return
				}
				// Remove the reflective event trampoline permanently. User input
				// and subsequent frames have no plugin overhead.
				event.Set(original)
				if os.Getenv("MYGO_GPU") == "0" {
					return
				}
				renew := reflect.New(et.In(0)).Elem()
				renew.FieldByIndex(kind.Index).SetUint(surfaceRenew)
				original.Call([]reflect.Value{renew})
			}
			go func() {
				select {
				case <-closed:
					return
				case <-prepared:
				}
				select {
				case <-closed:
					return
				default:
				}
				postMain(handoff)
			}()
		})
	}))
	return true
}

// GPU selection is read synchronously during the first frame on the UI thread.
// Restore the user's environment before returning; background warmup owns its
// own device and never reads this flag. No process-wide switch is left enabled.
func softwareFrame(draw func()) {
	before, existed := os.LookupEnv("MYGO_GPU")
	if err := os.Setenv("MYGO_GPU", "0"); err != nil {
		draw()
		return
	}
	defer func() {
		if existed {
			_ = os.Setenv("MYGO_GPU", before)
		} else {
			_ = os.Unsetenv("MYGO_GPU")
		}
	}()
	draw()
}
