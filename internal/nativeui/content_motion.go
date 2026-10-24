package nativeui

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// A small content slot fades out before replacing its value, then enters under
// the same fixed parent. Only a changed visible value triggers motion. Unlike
// overlapping Exit ghosts this never paints two versions of text together.
type contentSwapState struct {
	signature      string
	value          any
	serial         uint64
	childrenSerial uint64
	leaving        bool
	until          time.Time
}

// A replaced parent already animates its children together. Drop their old
// staged values so a new server/world can never inherit the previous one's data.
func (a *App) resetContentChildren(parent string, children ...string) {
	if state := a.contentSwaps[parent]; state != nil && state.childrenSerial != state.serial {
		for _, key := range children {
			delete(a.contentSwaps, key)
			delete(a.numberRolls, key)
		}
		state.childrenSerial = state.serial
	}
}

// Guard actions while showing an outgoing value without disabling the painted
// subtree: MyGo's Disabled also halves its opacity, creating a loading veil.
func (a *App) overviewActionReady(key string) bool {
	state := a.contentSwaps[key]
	return !a.overviewHolding && (state == nil || !state.leaving)
}

func swappedContent[T any](a *App, c *ui.Context, key string, signature any, value T) T {
	b, _ := json.Marshal(signature)
	sig := string(b)
	if a.contentSwaps == nil {
		a.contentSwaps = map[string]*contentSwapState{}
	}
	state := a.contentSwaps[key]
	if state == nil {
		a.contentSwaps[key] = &contentSwapState{signature: sig, value: value}
		return value
	}
	if state.signature == sig {
		state.leaving, state.value = false, value
		return value
	}
	if !c.Preferences().ReduceMotion {
		if !state.leaving {
			state.leaving, state.until = true, c.Now().Add(65*time.Millisecond)
		}
		if c.Now().Before(state.until) {
			c.After(state.until.Sub(c.Now()))
			return state.value.(T)
		}
	}
	state.signature, state.value, state.leaving = sig, value, false
	state.serial++
	return value
}

func (a *App) contentBody(c *ui.Context, key string) ui.Element {
	state := a.contentSwaps[key]
	serial := uint64(0)
	if state != nil {
		serial = state.serial
	}
	body := ui.Column(c.Key(fmt.Sprintf("%s-content-%d", key, serial))).Label("局部过渡 " + key).FillWidth().MinWidth(0)
	if c.Preferences().ReduceMotion {
		return body
	}
	pose, motion := ui.Pose{}, motionEnter
	if state != nil && state.leaving {
		pose, motion = ui.Pose{Fade: 1, Y: -4 * uiScale(c)}, ui.Animation{Duration: 65 * time.Millisecond, Ease: easeInCubic}
	}
	body.AnimatePose(pose, motion)
	if serial > 0 {
		body.Transition(ui.ElementTransition{Animation: &motionEnter, Colors: true, Enter: &ui.Motion{Opacity: 0, Y: 8 * uiScale(c)}})
	}
	return body
}

type overviewPanelData struct {
	root   launcher.Instance
	status launcher.InstanceStatus
}

func (a *App) overviewPanelData(c *ui.Context, key string, signature any, r launcher.Instance, st launcher.InstanceStatus) (launcher.Instance, launcher.InstanceStatus) {
	d := swappedContent(a, c, key, signature, overviewPanelData{r, st})
	return d.root, d.status
}
