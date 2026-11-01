package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestRealKernelVanilla runs the real kernel in lockstep with gorge on the
// dirac/vanilla decks (nil maps the synthetic cards onto corpus cards).
func TestRealKernelVanilla(t *testing.T) {
	t.Parallel()

	c, _ := testCorpus(t)
	cfg := RunConfig{Config: Config{PassElision: false}}
	mk := RealKernel(vanilla.Table, nil, cfg.Config)
	games := 0
	for a := 0; a < len(vanilla.Decks); a++ {
		for b := 0; b < len(vanilla.Decks); b++ {
			if a == b {
				continue
			}
			seed := uint64(1000 + 10*a + b)
			m, err := KernelMatchup(c.Reg, vanilla.Table, [2]string{fmt.Sprint("vanilla", a), fmt.Sprint("vanilla", b)},
				[2][]kop.CardID{vanilla.Decks[a], vanilla.Decks[b]}, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			k, err := mk(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, RandomPolicy(seed), cfg)
			games++
			if res.Div == nil {
				t.Logf("vanilla %d vs %d: no divergence in %d decisions (over=%v stall=%q)", a, b, res.Decisions, res.Over, res.Stall)
				continue
			}
			d := res.Div
			diff := d.Diff
			if len(diff) > 6 {
				diff = diff[:6]
			}
			last := res.Actions
			if len(last) > 4 {
				last = last[len(last)-4:]
			}
			t.Errorf("vanilla %d vs %d: %v\n  after %v\n  diff %v\n  gorge %v\n  kernel %v", a, b, d, actionStrings(last), diff, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games", games)
}

// TestStateOfRealKernel: a RealKernel side hands out its live state, the decision its
// last Step posed and those options as the Canons it would Apply; any other side
// hands out nothing.
func TestStateOfRealKernel(t *testing.T) {
	c, _ := testCorpus(t)
	m, err := KernelMatchup(c.Reg, vanilla.Table, [2]string{"a", "b"}, [2][]kop.CardID{vanilla.Decks[0], vanilla.Decks[1]}, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	k, err := RealKernel(vanilla.Table, nil, Config{})(m, 1)
	if err != nil {
		t.Fatal(err)
	}
	cd := k.Step()
	s, d, canon := KernelStateOf(k)
	if s == nil || d == nil {
		t.Fatal("no state from a RealKernel side")
	}
	if int(d.N) != len(cd.Options) || len(canon) != len(cd.Options) {
		t.Fatalf("decision has %d options, canons %d, Step posed %d", d.N, len(canon), len(cd.Options))
	}
	k.Apply(canon[0])
	g, err := GorgeKernel(Config{})(m, 1)
	if err != nil {
		t.Fatal(err)
	}
	if s, d, canon := KernelStateOf(g); s != nil || d != nil || canon != nil {
		t.Errorf("a gorge side gave %v %v %v", s, d, canon)
	}
}
