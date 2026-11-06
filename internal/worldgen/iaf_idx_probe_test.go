package worldgen_test

import (
	"fmt"
	"math"
	"os"
	"testing"

	wg "github.com/Viking602/YoloMCL/internal/worldgen"
)

func shellR(r int) int {
	h := r / 5
	v := float64(float32(float32(2*r+h)*float32(0.333) + float32(0.5)))
	return int(math.Sqrt(float64(int(math.Floor(v * v)))))
}
func TestIafVerify76(t *testing.T) {
	if os.Getenv("MCL_IAF_IDX") == "" {
		t.Skip()
	}
	seed := int64(-8842458691844027981)
	step := wg.StepSurfaceStructures
	n := int(wg.NewDecorationRandom(seed, 218, -552, 76, step).NextInt(8))
	r := 12 + n
	sr := shellR(r)
	x, y, z := 218*16+7, 70, -552*16+7
	fmt.Printf("roost218 R=%d shell=%d BB=[%d %d %d %d %d %d] want=[3486 70 -8834 3504 73 -8816]\n",
		r, sr, x-sr, y, z-sr, x+sr, y+3, z+sr)
	n = int(wg.NewDecorationRandom(seed, 229, -562, 76, step).NextInt(8))
	r = 12 + n
	sr = shellR(r)
	x, y, z = 229*16+7, 70, -562*16+7
	fmt.Printf("roost229 R=%d shell=%d BB=[%d %d %d %d %d %d] want=[3661 70 -8995 3681 73 -8975]\n",
		r, sr, x-sr, y, z-sr, x+sr, y+3, z+sr)
	up := 1 + int(wg.NewDecorationRandom(seed, -321, -268, 84, step).NextInt(4))
	x, y0, z := -321*16+7, 62, -268*16+7
	fmt.Printf("siren up=%d BB=[%d %d %d %d %d %d] want=[-5132 62 -4284 -5126 66 -4278]\n",
		up, x-up, y0, z-up, x+up, y0+up+1, z+up)
}
