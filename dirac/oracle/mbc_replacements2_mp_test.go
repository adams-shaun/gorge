//go:build dirac_mp

package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestFourSeatMBCReplacements2Lowers plays four-seat pods of the mbcrepl2
// fixture pool (mbc_replacements2_test.go's decks, alternating around the
// table, so seats 2 and 3 hold Fire Servant and Charging Tuskodon too). Past
// two seats the damage doublers of seats 2 and 3 apply as gorge's do: the
// kernel's replDamage, combat candidate walk and dblDmgN read "is a seat" as
// MaxSeats, not 1. No divergence and no kernel stop, and a seat past the
// first two lands a doubled hit.
func TestFourSeatMBCReplacements2Lowers(t *testing.T) {
	reg := synthReg(t)
	set, err := cardset.RawByName("mbcrepl2")
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	for s, d := range kRepl2Decks {
		for _, e := range d {
			names[s] = deckOf(e.Count, e.Name, names[s]...)
		}
	}
	decks := [4][]string{names[0], names[1], names[0], names[1]}
	var lists [][]cardset.Entry
	for _, d := range decks {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	late := 0 // doubled hits by a source seat 2 or 3 controls
	play := func(seed uint64) Result {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 30
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		probe := &repl2Probe{base: castFirstPolicy(seed)}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), rc)
		if g := probe.gs; g != nil {
			for _, ev := range g.E.L.Events {
				if ev.Kind != events.DamageProvenance || ev.Amount <= 0 {
					continue
				}
				src := g.E.G.Obj(ev.Obj)
				if src == nil || src.Face() == nil || src.Controller < 2 {
					continue
				}
				if p := repl2Printed[src.Face().Name]; p != 0 && ev.Amount == 2*p ||
					src.Face().Name == "Charging Tuskodon" && ev.Amount > 4 {
					late++
				}
			}
		}
		return res
	}
	fourSeatAtkClean(t, play, 16)
	if late == 0 {
		t.Errorf("no doubled hit by a source seat 2 or 3 controls in 16 pods")
	}
	t.Logf("doubled hits by seats 2 and 3: %d", late)
}
