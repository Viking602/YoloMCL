package nativeui

import (
	"math"
	"testing"
)

func TestOrbitPitchBothWays(t *testing.T) {
	p := 0.55
	if got := orbitPitch(p, 20); got <= p {
		t.Fatalf("drag down should raise pitch: %v -> %v", p, got)
	}
	if got := orbitPitch(p, -20); got >= p {
		t.Fatalf("drag up should lower pitch: %v -> %v", p, got)
	}
	below := p
	for i := 0; i < 20; i++ {
		below = orbitPitch(below, -20)
	}
	if below >= 0 {
		t.Fatalf("pitch should pass the horizon to look from below, got %v", below)
	}
	lo, hi := orbitPitch(0, -1e6), orbitPitch(0, 1e6)
	if math.Abs(lo+hi) > 1e-9 || hi >= math.Pi/2 || hi < 88*math.Pi/180 {
		t.Fatalf("clamp not symmetric about ±89°: %v %v", lo, hi)
	}
}
