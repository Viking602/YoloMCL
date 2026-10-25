package nativeui

import (
	"bytes"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestContentSwapKeepsLatestTargetAndUnchangedSlots(t *testing.T) {
	a := &App{}
	target := "First"
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Label("fixed parent").Fill().Background(panel).Children(func() {
			text := swappedContent(a, c, "value", target, target)
			a.contentBody(c, "value").Children(func() { ui.Text(c, text) })
		})
	}, 300, 120)
	initial := append([]byte(nil), tt.Image().Pix...)
	tt.Advance(300 * time.Millisecond)
	if !bytes.Equal(initial, tt.Image().Pix) {
		t.Fatal("unchanged content animated")
	}
	target = "Intermediate"
	tt.Frame()
	tt.Advance(20 * time.Millisecond)
	target = "Latest"
	tt.Frame()
	tt.Advance(46 * time.Millisecond)
	if !tt.HasText("Latest") || tt.HasText("First") || tt.HasText("Intermediate") {
		t.Fatal("rapid switching displayed an obsolete target")
	}
	tt.Advance(300 * time.Millisecond)
	settled := append([]byte(nil), tt.Image().Pix...)
	// Returning to the visible value while it is fading out cancels the
	// replacement, rather than flashing the abandoned target later.
	target = "Abandoned"
	tt.Frame()
	tt.Advance(20 * time.Millisecond)
	target = "Latest"
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !bytes.Equal(settled, tt.Image().Pix) || !tt.HasText("Latest") {
		t.Fatal("interrupted content did not return to its settled appearance")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	target = "Reduced motion"
	tt.Frame()
	if !tt.HasText(target) {
		t.Fatal("reduced motion retained the outgoing value")
	}
	still := append([]byte(nil), tt.Image().Pix...)
	tt.Advance(400 * time.Millisecond)
	if !bytes.Equal(still, tt.Image().Pix) {
		t.Fatal("reduced motion still animated a content slot")
	}
}

func TestContentSwapDistinguishesFieldBoundaries(t *testing.T) {
	a := &App{}
	signature, value, shown := [2]string{"a b", "c"}, "first", ""
	tt := ui.NewTester(func(c *ui.Context) {
		shown = swappedContent(a, c, "fields", signature, value)
	}, 200, 100)
	// Both tuples used to format as "[a b c]", suppressing the transition.
	signature, value = [2]string{"a", "b c"}, "second"
	tt.Frame()
	if shown != "first" || !a.contentSwaps["fields"].leaving {
		t.Fatal("distinct fields were treated as the same content")
	}
	tt.Advance(66 * time.Millisecond)
	if shown != "second" || a.contentSwaps["fields"].serial != 1 {
		t.Fatal("changed content did not finish its transition")
	}
}
