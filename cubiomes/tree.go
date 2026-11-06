package cubiomes

import "math"

// Each node occupies 64 bytes. Siblings are contiguous; there are no child
// pointers or strings in the search loop. Only construction uses climateNode.
type compactClimateNode struct {
	space [7][2]int32
	first uint32
	count uint16
	biome uint16
}

type climateTree struct {
	nodes  []compactClimateNode
	biomes []string
}

func compactClimateTree(root *climateNode, biomes []string) *climateTree {
	t := &climateTree{nodes: make([]compactClimateNode, 1), biomes: biomes}
	ids := make(map[string]uint16, len(biomes))
	for i, biome := range biomes {
		ids[biome] = uint16(i)
	}
	var emit func(*climateNode, int)
	emit = func(n *climateNode, index int) {
		var node compactClimateNode
		for i, r := range n.Space {
			node.space[i] = [2]int32{int32(r.Min), int32(r.Max)}
		}
		node.biome = ids[n.Biome]
		node.first, node.count = uint32(len(t.nodes)), uint16(len(n.Children))
		t.nodes[index] = node
		t.nodes = append(t.nodes, make([]compactClimateNode, len(n.Children))...)
		for i, child := range n.Children {
			emit(child, int(node.first)+i)
		}
	}
	emit(root, 0)
	return t
}

func (n *compactClimateNode) distance(p *climatePoint) int64 {
	var sum int64
	for i := range n.space {
		r := &n.space[i]
		d := max(int64(r[0])-p[i], p[i]-int64(r[1]), 0)
		sum += d * d
	}
	return sum
}

func (n *compactClimateNode) distanceBelow(p *climatePoint, limit int64) int64 {
	var sum int64
	for i := range n.space {
		r := &n.space[i]
		d := max(int64(r[0])-p[i], p[i]-int64(r[1]), 0)
		sum += d * d
		// Remaining squared distances are nonnegative; equality cannot beat
		// the previous leaf. This preserves vanilla's strict tie handling.
		if sum >= limit {
			return sum
		}
	}
	return sum
}

// search preserves strict comparison and stored sibling order, including a
// previous leaf that lies outside the subtree being traversed.
func (t *climateTree) search(p *climatePoint, previous int) int {
	best, distance := previous, int64(math.MaxInt64)
	if best >= 0 {
		distance = t.nodes[best].distance(p)
		if distance == 0 {
			return best
		}
	}
	return t.searchNode(0, p, best, &distance)
}

func (t *climateTree) searchNode(index int, p *climatePoint, best int, distance *int64) int {
	n := &t.nodes[index]
	if n.count == 0 {
		return index
	}
	for child := int(n.first); child < int(n.first)+int(n.count); child++ {
		d := t.nodes[child].distanceBelow(p, *distance)
		if d >= *distance {
			continue
		}
		if t.nodes[child].count == 0 {
			best, *distance = child, d
		} else {
			best = t.searchNode(child, p, best, distance)
		}
	}
	return best
}
