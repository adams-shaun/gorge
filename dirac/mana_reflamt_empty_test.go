package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReflAmtChainTapEmptyCandidates: a lowered reflected Amount$ source
// (Incubation Druid) whose filter matches no permanent (its controller has no
// land) stays in caGaps — the evaluated amount is positive — and the chain's
// answer count floors at one, so scriptGapTap reaches reflAmtChainTap with no
// colour to name. It taps for no mana (windowReflAmtTap's empty branch,
// gorge's Note) instead of indexing the empty candidate list.
func TestReflAmtChainTapEmptyCandidates(t *testing.T) {
	deck := []cardset.Entry{{Name: "Incubation Druid", Count: 1}, {Name: "Forest", Count: 19}}
	tbl, _, _, err := cardsettest.Set(t, "manariders").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true, GorgeLowering: true})
	druid := sc.perm(id["Incubation Druid"], 0)
	spell := sc.hand(id["Forest"], 0)
	s, k := sc.s, sc.k
	sl := sc.slot(druid)
	if !k.isReflAmt(id["Incubation Druid"]) {
		t.Fatal("precondition: Incubation Druid is not a reflected Amount$ census gap")
	}
	if n := s.reflAmtCols(k, sl); n != 0 {
		t.Fatalf("precondition: %#x candidate colours with no land on the board, want none", n)
	}
	if n := s.caAmount(k, sl); n <= 0 {
		t.Fatalf("precondition: evaluated amount %d, want positive (the gap stays in caGaps)", n)
	}
	if !s.caGaps(k, 0).has(sl) {
		t.Fatal("precondition: the Druid is not in caGaps")
	}
	a := MakeAction(AkCast, TCode(spell), 0, 0)
	f := s.fork()
	var spill [kop.NumManaKinds]uint8
	if !f.scriptGapTap(k, 0, a, sl, slotMask{}, slotMask{}, slotMask{}, slotMask{}, slotBit(sl), 0, &spill) {
		t.Fatal("the Druid's gap tap reported failure")
	}
	if f.perms[sl].flags&pfTapped == 0 {
		t.Error("the Druid is untapped after its activation")
	}
	if f.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Errorf("pool %v after an empty reflection, want no mana", f.pl[0].pool)
	}
	s.unfork()
}
