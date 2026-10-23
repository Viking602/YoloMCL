package nativeui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

var numberRuns = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)
var motionNumber = ui.Animation{Duration: 360 * time.Millisecond, Ease: ui.CubicBezier(.4, 0, .2, 1)}

type numberFlipState struct {
	from, target string
	turn         float32
	columns      []int
}

// flipNumber turns only the changing digits around their horizontal axis.
// The fixed outer slot, units and unchanged digits never fade or slide.
func (a *App) flipNumber(c *ui.Context, key, text string, size float32, weight int, color ui.Color, alignEnd ...bool) ui.Element {
	right := len(alignEnd) > 0 && alignEnd[0]
	if a.numberFlips == nil {
		a.numberFlips = map[string]*numberFlipState{}
	}
	state := a.numberFlips[key]
	if state == nil {
		state = &numberFlipState{from: text, target: text}
		a.numberFlips[key] = state
	}
	span := ui.Span{Text: "0", Font: uiSansFamily(), Features: "tnum", Size: size, Weight: weight}
	digitWidth, height := c.MeasureText(0, span)
	_, fullHeight := c.MeasureText(0, ui.Span{Text: text, Font: span.Font, Features: span.Features, Size: size, Weight: weight})
	height = max(height, fullHeight)
	box := ui.Row(c.Key("number-" + key)).Label("数字翻牌 " + key).Height(height).MinWidth(0).AlignItems(ui.Center)
	if right {
		box.Justify(ui.End)
	}
	angle := box.AnimateTo("turn", state.turn, motionNumber)
	if c.Preferences().ReduceMotion {
		state.from, state.target = text, text
		angle = box.Jump("turn", state.turn)
	} else if angle == state.turn && state.target != text {
		state.from, state.target = state.target, text
		state.turn += 180
		angle = box.AnimateTo("turn", state.turn, motionNumber)
	} else if angle < state.turn-90 {
		// The incoming face is still hidden. Rapid selections can replace it
		// without restarting the turn or showing intermediate values.
		state.target = text
	}
	rotation := float32(180)
	if angle != state.turn {
		rotation = angle - (state.turn - 180)
	}
	visible := state.target
	if rotation < 90 {
		visible = state.from
	}
	box.Children(func() {
		ui.Row(c.Key("number-value")).Label(visible).Height(height).AlignItems(ui.Center).Children(func() {
			targets := numberRuns.FindAllStringIndex(state.target, -1)
			oldNumbers := numberRuns.FindAllString(state.from, -1)
			oldLiterals := numberRuns.Split(state.from, -1)
			if len(targets) == 0 {
				numText(c, state.target, size, color).FontWeight(weight)
				return
			}
			start := 0
			literal := func(index int, value string) {
				if rotation < 90 && index < len(oldLiterals) && len(oldNumbers) == len(targets) {
					value = oldLiterals[index]
				}
				if value != "" {
					measure := func(text string) float32 {
						sp := span
						sp.Text = "0" + text + "0"
						w, _ := c.MeasureText(0, sp)
						return max(0, w-2*digitWidth)
					}
					left := measure(value[:len(value)-len(strings.TrimLeft(value, " "))])
					ui.Box(c.Key(fmt.Sprintf("literal-%d", index))).Label("数字单位 "+key+fmt.Sprint(index)).Size(measure(value), height).Shrink(0).Children(func() {
						numText(c, strings.TrimSpace(value), size, color).FontWeight(weight).FixedLineHeight(height).Absolute().Left(left).Top(0).Height(height)
					})
				}
			}
			for run, indexes := range targets {
				literal(run, state.target[start:indexes[0]])
				next, previous := state.target[indexes[0]:indexes[1]], ""
				if run < len(oldNumbers) {
					previous = oldNumbers[run]
				}
				for len(state.columns) <= run {
					state.columns = append(state.columns, 0)
				}
				width := max(state.columns[run], len(next), len(previous))
				state.columns[run] = width
				pad := func(value string) string {
					space := strings.Repeat(" ", width-len(value))
					if right {
						return space + value
					}
					return value + space
				}
				next, previous = pad(next), pad(previous)
				for digit := 0; digit < width; digit++ {
					old, new := string(previous[digit]), string(next[digit])
					id := fmt.Sprintf("%s-%d-%d", key, run, digit)
					ui.Box(c.Key("digit-"+id)).Label("数字位 "+id).Size(digitWidth, height).Shrink(0).Children(func() {
						face := func(name, value string, turn float32) {
							numText(c.Key(name), value, size, color).FontWeight(weight).FixedLineHeight(height).
								Absolute().Left(0).Top(0).Size(digitWidth, height).TextAlign(ui.Center).
								RotateX(turn).Perspective(size * 12).BackfaceHidden()
						}
						if old == new || rotation >= 180 {
							face("settled", new, 0)
						} else {
							face("outgoing", old, -rotation)
							face("incoming", new, 180-rotation)
						}
					})
				}
				start = indexes[1]
			}
			literal(len(targets), state.target[start:])
		})
	})
	return box
}
