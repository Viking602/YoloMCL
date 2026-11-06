package cubiomes

import (
	"encoding/json"
	"errors"
	"math"
)

// 26.2's interval_select chooses the first function whose threshold is above
// the input, or the final function. Lower it to a balanced range-choice graph
// so existing lazy evaluation, reseeding and dependency analysis all apply.
func (c *densityCompiler) compileIntervalSelect(raw json.RawMessage, fields map[string]json.RawMessage, depth int) (*densityNode, error) {
	var thresholds []*float64
	var functions []json.RawMessage
	if json.Unmarshal(fields["thresholds"], &thresholds) != nil || thresholds == nil ||
		json.Unmarshal(fields["functions"], &functions) != nil || len(functions) != len(thresholds)+1 || len(functions) > c.nodeLimit() {
		return nil, errors.New("invalid interval_select lists")
	}
	for i, value := range thresholds {
		if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || i > 0 && *value <= *thresholds[i-1] {
			return nil, errors.New("interval_select thresholds must increase")
		}
	}
	input, err := c.compile(fields["input"], depth+1)
	if err != nil {
		return nil, err
	}
	branches := make([]*densityNode, len(functions))
	for i, function := range functions {
		branches[i], err = c.compile(function, depth+1)
		if err != nil {
			return nil, err
		}
	}
	var build func(int, int, int) (*densityNode, error)
	build = func(lo, hi, depth int) (*densityNode, error) {
		if lo == hi {
			return branches[lo], nil
		}
		mid := (lo + hi) / 2
		left, err := build(lo, mid, depth+1)
		if err != nil {
			return nil, err
		}
		right, err := build(mid+1, hi, depth+1)
		if err != nil {
			return nil, err
		}
		if err := c.budget(depth); err != nil {
			return nil, err
		}
		n := &densityNode{id: c.nodes, kind: "range_choice", a: input, b: left, c: right, p: math.Inf(-1), q: *thresholds[mid], single: latestFloatRelease(c.version)}
		c.nodes++
		return n, nil
	}
	n, err := build(0, len(thresholds), depth)
	if err == nil {
		c.intern[string(raw)] = n
	}
	return n, err
}
