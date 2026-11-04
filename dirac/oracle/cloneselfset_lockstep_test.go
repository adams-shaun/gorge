package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Gwyndala, the Progeny's {2}: each Equipment token its controller controls
// becomes a copy of target Equipment they control until end of turn. The
// target is one of those tokens when it is a Neuroflux Heirloom, and a copy of
// itself changes nothing (CR 707.2), so the kernel skips it rather than
// stalling on a permanent copying itself; the other tokens become copies of
// a nontoken target (Tricorder, Type-2 Phaser, Bat'leth) and go back at
// cleanup. Every game must agree, and none may stop on something the kernel
// does not model.
func TestCloneSelfSetLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	decks := [2][]cardset.Entry{
		{
			{Name: "Gwyndala, the Progeny", Count: 12}, {Name: "Tricorder", Count: 4},
			{Name: "Type-2 Phaser", Count: 4},
			{Name: "Island", Count: 20},
		},
		{
			{Name: "Gwyndala, the Progeny", Count: 12}, {Name: "Bat'leth", Count: 4},
			{Name: "Tricorder", Count: 3},
			{Name: "Island", Count: 14}, {Name: "Swamp", Count: 7},
		},
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-trk")
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var st dirac.CloneCopyStats
	dirac.DebugCloneCopyStats = &st
	defer func() { dirac.DebugCloneCopyStats = nil }()
	capStalls := map[string]int{}
	for seed := uint64(1); seed <= 200; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"selfset-a", "selfset-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
		if res.StallUnmodelled {
			t.Fatalf("seed %d: kernel stopped on something it does not model: %s", seed, res.Stall)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			capStalls[res.Stall]++
		}
	}
	if st.EOT == 0 {
		t.Fatalf("no Equipment token became a copy: %+v", st)
	}
	t.Logf("clone copies: %+v; capacity stalls: %v", st, capStalls)
}
