package nativeui

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

const numberStagger = 40 * time.Millisecond

var numberRuns = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)
var motionNumber = ui.Animation{Duration: 360 * time.Millisecond, Ease: easeStandard}

type numberRollState struct {
	target, displayed string
	columns           []numberColumns
	digits            map[string]*numberDigitState
}

type numberColumns struct{ integer, fraction int }

type numberPlace struct {
	char  byte
	place int // units = 0, tens = 1; tenths = -1, hundredths = -2
	point bool
}

type numberDigitState struct {
	char     byte
	target   float32
	position float32
	anim     ui.Animation
}

// rollNumber uses independent odometer columns. Only changed places move,
// starting at the least significant place; units and punctuation stay still.
// Every wheel travels at most one revolution, regardless of the numeric delta.
func (a *App) rollNumber(c *ui.Context, key, text string, size float32, weight int, color ui.Color, alignEnd ...bool) ui.Element {
	right := len(alignEnd) > 0 && alignEnd[0]
	if a.numberRolls == nil {
		a.numberRolls = map[string]*numberRollState{}
	}
	state := a.numberRolls[key]
	fresh := state == nil
	if fresh {
		state = &numberRollState{target: text, displayed: text, digits: map[string]*numberDigitState{}}
		a.numberRolls[key] = state
	}
	span := ui.Span{Text: "0", Font: uiSansFamily(), Features: "tnum", Size: size, Weight: weight}
	digitWidth, height := c.MeasureText(0, span)
	_, fullHeight := c.MeasureText(0, ui.Span{Text: text, Font: span.Font, Features: span.Features, Size: size, Weight: weight})
	height = max(height, fullHeight)
	box := ui.Row(c.Key("number-" + key)).Label("动态数字 " + key).Height(height).MinWidth(0).AlignItems(ui.Center)
	if right {
		box.Justify(ui.End)
	}
	settled := true
	box.Children(func() {
		value := ui.Row(c.Key("number-value")).Height(height).AlignItems(ui.Center)
		value.Children(func() {
			targets := numberRuns.FindAllStringIndex(text, -1)
			previous := numberRuns.FindAllString(state.target, -1)
			if len(targets) == 0 {
				numText(c, text, size, color).FontWeight(weight)
				state.columns = nil
				clear(state.digits)
				return
			}
			start := 0
			for run, indexes := range targets {
				numberLiteral(c, key, run, text[start:indexes[0]], span, height, digitWidth, color)
				next, old := text[indexes[0]:indexes[1]], ""
				if run < len(previous) {
					old = previous[run]
				}
				for len(state.columns) <= run {
					state.columns = append(state.columns, numberColumns{})
				}
				places := state.columns[run].places(next)
				if !right {
					// Keep left-aligned counts at the card's leading edge when
					// they shrink, while retaining the unit's reserved space.
					leading := 0
					for leading < len(places) && places[leading].char == ' ' && !places[leading].point {
						leading++
					}
					places = append(places[leading:], places[:leading]...)
				}
				// Rank only changing digits. A lone hundreds-place update starts
				// immediately, while a carry cascades from units to tens and up.
				delays := make([]time.Duration, len(places))
				changed := 0
				for i := len(places) - 1; i >= 0; i-- {
					p := places[i]
					id := fmt.Sprintf("%s-%d-%d", key, run, p.place)
					d := state.digits[id]
					if !p.point && p.char >= '0' && p.char <= '9' && (d == nil || d.char != p.char) {
						delays[i] = time.Duration(min(changed, 3)) * numberStagger
						changed++
					}
				}
				direction := numberDirection(old, next)
				ui.Box(c.Key(fmt.Sprintf("run-%d", run))).Size(float32(len(places))*digitWidth, height).Shrink(0).Children(func() {
					for i, p := range places {
						id := fmt.Sprintf("%s-%d-%d", key, run, p.place)
						if p.point {
							id = fmt.Sprintf("%s-%d", key, run)
						}
						slot := ui.Box(c.Key("digit-"+id)).Label("数字位 "+id).Absolute().Top(0).Size(digitWidth, height).Clip()
						x := float32(i) * digitWidth
						if !right {
							x = slot.AnimateTo("column-x", x, motionFast)
						}
						slot.Left(x)
						if p.point {
							slot.Label("小数点 " + id).Children(func() { numberFace(c, string(p.char), 0, span, digitWidth, height, color) })
						} else if !state.rollDigit(c, slot, id, p.char, direction, delays[i], fresh, span, digitWidth, height, color) {
							settled = false
						}
					}
				})
				start = indexes[1]
			}
			numberLiteral(c, key, len(targets), text[start:], span, height, digitWidth, color)
		})
		if settled {
			state.displayed = text
		}
		value.Label(state.displayed)
	})
	state.target = text
	return box
}

func (st *numberRollState) rollDigit(c *ui.Context, slot ui.Element, id string, char byte, direction float32, delay time.Duration, fresh bool, span ui.Span, width, height float32, color ui.Color) bool {
	d := st.digits[id]
	if d == nil {
		d = &numberDigitState{char: ' ', anim: motionNumber}
		st.digits[id] = d
		slot.Jump("roll", 0)
		slot.Jump("presence", 0)
	}
	position := slot.AnimateTo("roll", d.target, d.anim)
	if position == d.target && (position < 0 || position >= 10) {
		d.target = float32((int(position)%10 + 10) % 10)
		position = slot.Jump("roll", d.target)
	}
	presence := float32(0)
	if char >= '0' && char <= '9' {
		presence = 1
	}
	if fresh || c.Preferences().ReduceMotion {
		if presence == 1 {
			d.target = float32(char - '0')
		}
		position = slot.Jump("roll", d.target)
		slot.Jump("presence", presence)
	} else if d.char != char {
		moving := position != d.target
		d.anim = motionNumber
		if !moving {
			d.anim.Delay = delay
		}
		if presence == 1 {
			// Retarget from the current fractional position, even during the
			// stagger delay. Never queue an obsolete count or restart at zero.
			d.target = float32(math.Floor(float64(position)/10))*10 + float32(char-'0')
			if moving {
				// A new request may reverse a wheel. Take the nearest route from
				// its current position, without pausing for another stagger.
				if d.target-position > 5 {
					d.target -= 10
				} else if d.target-position < -5 {
					d.target += 10
				}
			} else if direction > 0 && d.target < position {
				d.target += 10
			} else if direction < 0 && d.target > position {
				d.target -= 10
			}
			steps := min(7, int(math.Ceil(math.Abs(float64(d.target-position)))))
			d.anim.Duration = 220*time.Millisecond + time.Duration(steps)*20*time.Millisecond
		} else {
			d.target = float32(math.Round(float64(position)))
			slot.Jump("roll", d.target)
		}
		position = slot.AnimateTo("roll", d.target, d.anim)
	}
	d.char, d.position = char, position
	presenceMotion := motionFast
	if presence == 1 {
		presenceMotion.Delay = d.anim.Delay
	}
	alpha := slot.AnimateTo("presence", presence, presenceMotion)
	slot.Opacity(alpha).Children(func() {
		if alpha == 0 {
			return
		}
		base := int(math.Floor(float64(position)))
		fraction := position - float32(base)
		digit := func(n int) string { return string(rune('0' + (n%10+10)%10)) }
		numberFace(c.Key("lower"), digit(base), -fraction*height, span, width, height, color)
		if fraction > 0 {
			numberFace(c.Key("upper"), digit(base+1), (1-fraction)*height, span, width, height, color)
		}
	})
	return position == d.target && alpha == presence
}

// Keep decimal places matched even when precision or integer width changes.
// Retained columns prevent a shrinking value from pulling its unit sideways.
func (columns *numberColumns) places(text string) []numberPlace {
	integer, fraction, point := strings.Cut(text, ".")
	columns.integer = max(columns.integer, len(integer))
	columns.fraction = max(columns.fraction, len(fraction))
	integer = strings.Repeat(" ", columns.integer-len(integer)) + integer
	places := make([]numberPlace, 0, columns.integer+columns.fraction+1)
	for i := range len(integer) {
		places = append(places, numberPlace{char: integer[i], place: len(integer) - 1 - i})
	}
	if columns.fraction > 0 {
		char := byte(' ')
		if point {
			char = '.'
		}
		places = append(places, numberPlace{char: char, point: true})
		fraction += strings.Repeat(" ", columns.fraction-len(fraction))
		for i := range len(fraction) {
			places = append(places, numberPlace{char: fraction[i], place: -1 - i})
		}
	}
	return places
}

// Move native-size glyphs within a clipped column; no 3D text resampling.
func numberFace(c *ui.Context, text string, y float32, span ui.Span, width, height float32, color ui.Color) {
	numText(c, text, span.Size, color).FontWeight(span.Weight).FixedLineHeight(height).
		Absolute().Left(0).Top(y).Size(width, height).TextAlign(ui.Center).Opacity(max(0, 1-max(y, -y)/height))
}

func numberLiteral(c *ui.Context, key string, index int, value string, span ui.Span, height, digitWidth float32, color ui.Color) {
	if value == "" {
		return
	}
	measure := func(text string) float32 {
		sp := span
		sp.Text = "0" + text + "0"
		w, _ := c.MeasureText(0, sp)
		return max(0, w-2*digitWidth)
	}
	left := measure(value[:len(value)-len(strings.TrimLeft(value, " "))])
	ui.Box(c.Key(fmt.Sprintf("literal-%d", index))).Label("数字单位 "+key+fmt.Sprint(index)).Size(measure(value), height).Shrink(0).Children(func() {
		numText(c, strings.TrimSpace(value), span.Size, color).FontWeight(span.Weight).FixedLineHeight(height).Absolute().Left(left).Top(0).Height(height)
	})
}

// Compare decimal text without losing precision for large counts.
func numberDirection(previous, next string) float32 {
	a, af, _ := strings.Cut(previous, ".")
	b, bf, _ := strings.Cut(next, ".")
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	order := len(b) - len(a)
	if order == 0 {
		order = strings.Compare(b, a)
	}
	if order == 0 {
		n := max(len(af), len(bf))
		order = strings.Compare(bf+strings.Repeat("0", n-len(bf)), af+strings.Repeat("0", n-len(af)))
	}
	if order < 0 {
		return -1
	}
	return 1
}
