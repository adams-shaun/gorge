package netsimd_test

import (
	"hash/fnv"
	"math"
	"testing"

	"github.com/adams-shaun/gorge/bots/netsimd"
)

// TestCorpusDigest logs one hash of every value and option logit bit over
// the corpus (SIMD and scalar). It asserts nothing on its own: comparing the
// digest across two builds on one network is how a kernel change proves it
// left a shape's arithmetic bit-identical.
func TestCorpusDigest(t *testing.T) {
	recs := corpus(t)
	g := loadSIMD(t, 0)
	t.Cleanup(func() { netsimd.ForceScalar = false })
	for _, scalar := range []bool{false, true} {
		netsimd.ForceScalar = scalar
		var sc netsimd.Scratch
		h := fnv.New64a()
		var b [4]byte
		put := func(v float32) {
			u := math.Float32bits(v)
			b[0], b[1], b[2], b[3] = byte(u), byte(u>>8), byte(u>>16), byte(u>>24)
			h.Write(b[:])
		}
		scores := make([]float32, 64)
		for i := range recs {
			r := &recs[i]
			put(g.Eval(&sc, &r.st, r.opts, scores))
			for _, s := range scores[:len(r.opts)] {
				put(s)
			}
		}
		t.Logf("digest scalar=%v: %016x over %d decisions", scalar, h.Sum64(), len(recs))
	}
}
