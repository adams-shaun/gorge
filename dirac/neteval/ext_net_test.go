package neteval

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

func TestNetevalExtOnlyForExtNets(t *testing.T) {
	plain := testNet(t)
	if g := plain.enc.ExtGroups(); g != 0 {
		t.Fatalf("plain net: ExtGroups = %v, want 0", g)
	}
	r := rand.New(rand.NewPCG(3, 3))
	w := make([]float32, 64*feat.ExtWidth)
	for i := range w {
		w[i] = (r.Float32()*2 - 1) * 0.25
	}
	p, err := gonettest.WriteExt(t.TempDir(), "ext.dzgw", gonettest.Wide, feat.ExtWidth, w)
	if err != nil {
		t.Fatal(err)
	}
	ws, err := LoadWeights(p)
	if err != nil {
		t.Fatal(err)
	}
	specs := make([]feat.CardSpec, len(v.Table.Cards))
	for i := range specs {
		specs[i] = feat.CardSpec{Name: v.Table.Cards[i].Name}
	}
	en := New(ws, feat.NewPool(v.Table, specs))
	if en.enc.ExtGroups() != feat.ExtAll {
		t.Fatalf("ext net: ExtGroups = %v, want ExtAll", en.enc.ExtGroups())
	}
	differ, nonEmpty := 0, 0
	var d dirac.Decision
	for _, s := range positions(40) {
		s.StepInto(&d)
		plain.NewWorld()
		en.NewWorld()
		a := plain.Value(s, &d, 1-d.Player)
		b := en.Value(s, &d, 1-d.Player)
		if len(en.ext) > 0 {
			nonEmpty++
			if a != b {
				differ++
			}
		}
	}
	if nonEmpty == 0 || differ == 0 {
		t.Fatalf("ext non-empty on %d states, value differed on %d", nonEmpty, differ)
	}

	// The encoded-keyed, materialized path runs state and evalFull: its Ext
	// pass-through must give the parts path's value.
	mat := New(ws, feat.NewPool(v.Table, specs))
	mat.Materialize, mat.Keys = true, KeyEncoded
	checked, maxDiff := 0, 0.0
	for _, s := range positions(20) {
		s.StepInto(&d)
		en.NewWorld()
		mat.NewWorld()
		a := en.Value(s, &d, 1-d.Player)
		b := mat.Value(s, &d, 1-d.Player)
		if len(mat.ext) > 0 {
			checked++
			maxDiff = math.Max(maxDiff, math.Abs(float64(a-b)))
		}
	}
	if checked == 0 {
		t.Fatal("materialized ext net: no non-empty Ext")
	}
	if maxDiff > 1e-5 {
		t.Fatalf("materialized ext differs from the parts path by %v", maxDiff)
	}

	// stateHash names Ext, so an encoded cache key never serves an ext net a
	// non-ext encoding (and vice versa).
	hashNet := New(ws, feat.NewPool(v.Table, specs))
	hs := positions(1)[0]
	hs.StepInto(&d)
	hashNet.state(hs, &d, 1-d.Player)
	h0 := hashNet.stateHash()
	if len(hashNet.ext) == 0 {
		t.Fatal("stateHash test position has an empty Ext")
	}
	extCopy := make([]policynet.Feature, len(hashNet.ext))
	copy(extCopy, hashNet.ext)
	extCopy[0].Value++
	hashNet.ext = extCopy
	if hashNet.stateHash() == h0 {
		t.Fatal("stateHash did not mix Ext")
	}

	// the zero-alloc property holds for the ext net too
	ps := positions(10)
	var logits [dirac.MaxOptions]float32
	run := func() {
		for _, s := range ps {
			s.StepInto(&d)
			en.NewWorld()
			en.Priors(s, &d, nil, logits[:d.N])
			en.Value(s, &d, 1-d.Player)
		}
	}
	run()
	if a := testing.AllocsPerRun(3, run); a != 0 {
		t.Fatalf("ext net allocs = %v", a)
	}
}
