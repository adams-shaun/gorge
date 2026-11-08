package neteval

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
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
