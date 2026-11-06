package cubiomes

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestIntervalSelectJavaOracle(t *testing.T) {
	raw, err := os.ReadFile("testdata/interval-select-26.2.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version    string
		Thresholds []float64
		Functions  []float64
		Samples    []struct{ Input, Value float64 }
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, sample := range fixture.Samples {
		data, _ := json.Marshal(map[string]any{"type": "minecraft:interval_select", "input": sample.Input, "thresholds": fixture.Thresholds, "functions": fixture.Functions})
		c := newDensityCompiler(context.Background(), nil, 0, false)
		c.version = fixture.Version
		n, err := c.compile(data, 0)
		if err != nil {
			t.Fatal(err)
		}
		p := densitySample{seen: make([]uint32, c.nodes), values: make([]float64, c.nodes)}
		p.at(-17, 64, 31)
		if got := n.eval(&p); got != sample.Value {
			t.Fatalf("input %.17g: got %g want %g", sample.Input, got, sample.Value)
		}
	}
}

func TestIntervalSelectValidationAndLazyBranches(t *testing.T) {
	for _, raw := range []string{
		`{"type":"interval_select","input":0,"thresholds":null,"functions":[1]}`,
		`{"type":"interval_select","input":0,"thresholds":[null],"functions":[1,2]}`,
		`{"type":"interval_select","input":0,"thresholds":[1,0],"functions":[1,2,3]}`,
		`{"type":"interval_select","input":0,"thresholds":[0],"functions":[1]}`,
		`{"type":"interval_select","input":0,"thresholds":[],"functions":[]}`,
	} {
		c := newDensityCompiler(context.Background(), nil, 0, false)
		if _, err := c.compile(json.RawMessage(raw), 0); err == nil {
			t.Fatalf("accepted invalid interval: %s", raw)
		}
	}
	for _, raw := range []string{
		`{"type":"interval_select","input":-1,"thresholds":[0],"functions":[17,{"type":"invert","argument":0}]}`,
		`{"type":"interval_select","input":0,"thresholds":[],"functions":[17]}`,
	} {
		c := newDensityCompiler(context.Background(), nil, 0, false)
		n, err := c.compile(json.RawMessage(raw), 0)
		if err != nil {
			t.Fatal(err)
		}
		p := densitySample{seen: make([]uint32, c.nodes), values: make([]float64, c.nodes)}
		p.at(0, 0, 0)
		if got := n.eval(&p); got != 17 || math.IsNaN(got) {
			t.Fatalf("unselected branch affected result: %g", got)
		}
	}
}
