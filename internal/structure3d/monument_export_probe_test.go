package structure3d

// ProbePiece is a monument piece id and box, for tests.
type ProbePiece struct {
	ID string
	BB [6]int
}

// MonumentPieceList exposes createMonumentPieces to external tests.
func MonumentPieceList(seed int64, cx, cz int32, sea, yOff int) []ProbePiece {
	_, _, pcs := createMonumentPieces(seed, cx, cz, sea, yOff)
	out := make([]ProbePiece, 0, len(pcs))
	for _, p := range pcs {
		out = append(out, ProbePiece{ID: p.id, BB: [6]int(p.bb)})
	}
	return out
}
