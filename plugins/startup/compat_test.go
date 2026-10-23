package startup

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

type testEvent struct {
	Kind uint8
	Text string
}
type testConnection struct {
	Event         func(testEvent) bool
	Ready, Detach func()
	Post          func(func())
}

func nextPost(t *testing.T, posted <-chan func()) func() {
	t.Helper()
	select {
	case f := <-posted:
		return f
	case <-time.After(time.Second):
		t.Fatal("handoff was not scheduled")
		return nil
	}
}

func TestSoftwareFrameRestoresEnvironment(t *testing.T) {
	for _, value := range []string{"1", "custom"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("MYGO_GPU", value)
			softwareFrame(func() {
				if os.Getenv("MYGO_GPU") != "0" {
					t.Fatal("first frame used GPU")
				}
			})
			if os.Getenv("MYGO_GPU") != value {
				t.Fatal("environment changed")
			}
			func() { defer func() { _ = recover() }(); softwareFrame(func() { panic("draw failed") }) }()
			if os.Getenv("MYGO_GPU") != value {
				t.Fatal("panic did not restore environment")
			}
		})
	}
	t.Setenv("MYGO_GPU", "1")
	_ = os.Unsetenv("MYGO_GPU")
	softwareFrame(func() {})
	if _, ok := os.LookupEnv("MYGO_GPU"); ok {
		t.Fatal("introduced an environment override")
	}
}

func TestHandoffPreservesCallbacksAndInput(t *testing.T) {
	t.Setenv("MYGO_GPU", "1")
	prepared := make(chan struct{})
	posted := make(chan func(), 8)
	ready, detached, renew := 0, 0, 0
	text := ""
	gpuFlags := []string{}
	conn := testConnection{
		Event: func(e testEvent) bool {
			if e.Kind == surfaceFrame {
				gpuFlags = append(gpuFlags, os.Getenv("MYGO_GPU"))
			}
			if e.Kind == surfaceRenew {
				renew++
			}
			if e.Kind == 10 {
				text += e.Text
			}
			return true
		},
		Ready: func() { ready++ }, Detach: func() { detached++ }, Post: func(f func()) { posted <- f },
	}
	original := reflect.ValueOf(conn.Event).Pointer()
	if !install(reflect.ValueOf(&conn).Elem(), prepared, nil) {
		t.Fatal("adapter rejected connection")
	}
	if !conn.Event(testEvent{Kind: surfaceFrame}) {
		t.Fatal("lost return value")
	}
	conn.Ready()
	conn.Event(testEvent{Kind: 10, Text: "input before GPU"})
	conn.Event(testEvent{Kind: surfaceFrame})
	if len(posted) != 0 {
		t.Fatal("GPU requested before warmup finished")
	}
	close(prepared)
	nextPost(t, posted)() // recent typing defers transition until idle
	if renew != 0 {
		t.Fatal("interrupted typing")
	}
	nextPost(t, posted)()
	conn.Event(testEvent{Kind: surfaceFrame})
	if renew != 1 || ready != 1 || text != "input before GPU" {
		t.Fatalf("lost state: renew=%d ready=%d text=%q", renew, ready, text)
	}
	if !reflect.DeepEqual(gpuFlags, []string{"0", "0", "1"}) {
		t.Fatalf("wrong renderer selection: %v", gpuFlags)
	}
	if reflect.ValueOf(conn.Event).Pointer() != original {
		t.Fatal("event trampoline remained installed")
	}
	conn.Detach()
	if detached != 1 {
		t.Fatal("original cleanup not called")
	}
}

func TestCloseBeforeQueuedHandoff(t *testing.T) {
	prepared := make(chan struct{})
	posted := make(chan func(), 8)
	renew, detached := 0, 0
	conn := testConnection{Event: func(e testEvent) bool {
		if e.Kind == surfaceRenew {
			renew++
		}
		return false
	}, Detach: func() { detached++ }, Post: func(f func()) { posted <- f }}
	if !install(reflect.ValueOf(&conn).Elem(), prepared, nil) {
		t.Fatal("adapter rejected connection")
	}
	conn.Ready()
	close(prepared)
	handoff := nextPost(t, posted)
	conn.Detach()
	handoff()
	if renew != 0 || detached != 1 {
		t.Fatal("handoff touched a closed window")
	}
}

func TestHandoffWaitsForPointerRelease(t *testing.T) {
	prepared := make(chan struct{})
	close(prepared)
	posted := make(chan func(), 8)
	renew := 0
	conn := testConnection{Event: func(e testEvent) bool {
		if e.Kind == surfaceRenew {
			renew++
		}
		return false
	}, Post: func(f func()) { posted <- f }}
	if !install(reflect.ValueOf(&conn).Elem(), prepared, nil) {
		t.Fatal("adapter rejected connection")
	}
	conn.Event(testEvent{Kind: 3})
	conn.Ready()
	nextPost(t, posted)()
	nextPost(t, posted)()
	if renew != 0 {
		t.Fatal("interrupted an active drag")
	}
	conn.Event(testEvent{Kind: 4})
	for renew == 0 {
		nextPost(t, posted)()
	}
	if renew != 1 {
		t.Fatal("handoff was not one-shot")
	}
	conn.Detach()
}

func TestExplicitSoftwareModeIsRespected(t *testing.T) {
	t.Setenv("MYGO_GPU", "0")
	prepared := make(chan struct{})
	close(prepared)
	posted := make(chan func(), 1)
	renew := 0
	conn := testConnection{Event: func(e testEvent) bool {
		if e.Kind == surfaceRenew {
			renew++
		}
		return false
	}, Post: func(f func()) { posted <- f }}
	if !install(reflect.ValueOf(&conn).Elem(), prepared, nil) {
		t.Fatal("adapter rejected connection")
	}
	conn.Ready()
	nextPost(t, posted)()
	conn.Detach()
	if renew != 0 || os.Getenv("MYGO_GPU") != "0" {
		t.Fatal("overrode software selection")
	}
}

func TestInvalidConnectionFailsWithoutMutation(t *testing.T) {
	conn := testConnection{Event: func(testEvent) bool { return true }}
	original := reflect.ValueOf(conn.Event).Pointer()
	if install(reflect.ValueOf(&conn).Elem(), nil, nil) {
		t.Fatal("accepted missing Post")
	}
	if reflect.ValueOf(conn.Event).Pointer() != original {
		t.Fatal("partially patched invalid connection")
	}
	if install(reflect.ValueOf(1), nil, nil) {
		t.Fatal("accepted invalid value")
	}
}

func TestPinnedMyGoConnectionShape(t *testing.T) {
	// Obtain the real parameter type through the public method; no internal
	// package imports or synthetic module aliases are used by this test.
	typ := reflect.TypeOf(ui.View(func(*ui.Context) {}).AttachContent).In(0).Elem()
	conn := reflect.New(typ).Elem()
	event := conn.FieldByName("Event")
	event.Set(reflect.MakeFunc(event.Type(), func([]reflect.Value) []reflect.Value { return []reflect.Value{reflect.ValueOf(false)} }))
	conn.FieldByName("Post").Set(reflect.ValueOf(func(func()) {}))
	if !install(conn, nil, nil) {
		t.Fatal("MyGo connection contract changed")
	}
	conn.FieldByName("Detach").Interface().(func())()
}
