package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// sacXBoard is Krark-Clan Ironworks, then an artifact Forest wearing Abundant
// Growth (the grant), p0's, in that battlefield order.
func sacXBoard(t *testing.T) (sc *scen, art Obj) {
	t.Helper()
	set := cardsettest.Synth(t, "dr-mbc-bbbb")
	var deck []cardset.Entry
	for _, n := range []string{"Krark-Clan Ironworks", "Abundant Growth", "Forest"} {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tbl.Cards[id["Forest"]].Types |= kop.TArtifact // an artifact land: a sacrifice candidate and a grant holder
	sc = newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(id["Krark-Clan Ironworks"], 0)
	art = sc.perm(id["Forest"], 0)
	aura := sc.perm(id["Abundant Growth"], 0)
	sc.s.attachTo(sc.k, sc.slot(aura), sc.slot(art))
	return sc, art
}

// TestGrantSacXNeverTapsASacrificedCandidate: Ironworks' "Sacrifice an
// artifact: Add {C}{C}" beside Abundant Growth's granted ability pays {2}{G}
// (the sacrifice, then the artifact land's mana) and not {2}{G}{G}: a
// permanent a stepSacX sacrificed is gone, so no later step of the prefix taps
// it for mana (mana_grantedoffers.go visit).
func TestGrantSacXNeverTapsASacrificedCandidate(t *testing.T) {
	sc, art := sacXBoard(t)
	s, k := sc.s, sc.k
	if !s.sacXBesideGrant(k, 0) {
		t.Fatal("setup: no sacrifice source beside a live grant")
	}
	var need [6]bool
	need[kop.G] = true
	pay := func(g int) (ok bool, gs grantSrch) {
		cost := kop.ManaCost{Generic: 2}
		cost.Pips[kop.G] = uint8(g)
		return s.grantSearch(k, 0, &cost, NoSlot, payDesc{}, need, &gs), gs
	}
	ok, g := pay(1)
	if !ok {
		t.Fatal("setup: {2}{G} is not paid")
	}
	gone := map[Slot]bool{}
	sacs := 0
	for i := 0; i < g.np; i++ {
		st := g.path[i]
		if gone[st.sl] || (st.gi != stepSacX && gone[st.cand]) {
			t.Errorf("step %d taps slot %d/%d after a step sacrificed it", i, st.sl, st.cand)
		}
		if st.gi == stepSacX {
			gone[st.cand] = true
			sacs++
		}
	}
	if sacs == 0 {
		t.Fatalf("setup: the payment sacrifices nothing (%d steps); %d is the artifact", g.np, art)
	}
	if ok, _ := pay(2); ok {
		t.Error("{2}{G}{G} is paid: four mana from Ironworks and one artifact land")
	}
}

// TestGrantSacrificedCountsASacXStep: a path holding a stepSacX step is one
// that sacrificed a permanent, so visit's skip of a gone candidate runs.
func TestGrantSacrificedCountsASacXStep(t *testing.T) {
	var g grantSrch
	g.path[0] = grantStep{gi: stepSacX, cand: 3}
	g.np = 1
	if !g.sacrificed() {
		t.Fatal("a stepSacX step does not count as a sacrifice")
	}
	g.path[0] = grantStep{gi: 0}
	if g.sacrificed() {
		t.Fatal("a plain granted tap counts as a sacrifice")
	}
}
