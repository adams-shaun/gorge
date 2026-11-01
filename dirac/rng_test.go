package dirac

import (
	"math/rand/v2"
	"testing"
)

// TestRNGMatchesGorge pins the kernel RNG to gorge's construction:
// rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)).IntN.
func TestRNGMatchesGorge(t *testing.T) {
	for _, seed := range []uint64{0, 1, 42, 0xdeadbeef, 1 << 63} {
		ref := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
		k := newRNG(seed)
		for i := 0; i < 2000; i++ {
			n := 1 + i%61
			if a, b := ref.IntN(n), k.IntN(n); a != b {
				t.Fatalf("seed %d draw %d n=%d: gorge %d kernel %d", seed, i, n, a, b)
			}
		}
		if k.Draws != 2000 {
			t.Fatalf("draws %d", k.Draws)
		}
	}
}

func TestChanceTranscript(t *testing.T) {
	var lib [40]CardID
	s := NewConfig(testTable(), lib[:], lib[:], 9, Config{RecordChance: true, Mulligans: 1})
	tr, ok := s.ChanceTranscript()
	if !ok || len(tr) != int(s.RNGDraws()) {
		t.Fatalf("transcript ok=%v len %d draws %d", ok, len(tr), s.RNGDraws())
	}
	if tr[0].Bound != 2 || tr[1].Bound != 40 || tr[39].Bound != 2 {
		t.Fatalf("bounds %v %v %v", tr[0], tr[1], tr[39])
	}
	ref := rand.New(rand.NewPCG(9, 9^0x9e3779b97f4a7c15))
	for i, d := range tr {
		if v := ref.IntN(d.Bound); v != d.Value {
			t.Fatalf("draw %d: %d vs %d", i, v, d.Value)
		}
	}
}
