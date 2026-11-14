package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRelaxRestrCountsRelaxableFilters: Clement's creature-spell-only {G} or
// {U} on a Frog beside Hidden Grotto's "{1},{T}: Add one mana of any color"
// and one untapped Forest. Lilypad Village's "{U},{T}: Surveil 2" is an
// ability, so the grant's restricted {U} is refused it; gorge's relaxed proof
// (paymentPlanRelaxProof) still pays {U} from the Grotto subset, its {1} fee
// charged against the Forest, so the play is not proven insufficient and its
// prefix search runs. relaxRestrRefuses must count that filter in both reads.
func TestRelaxRestrCountsRelaxableFilters(t *testing.T) {
	tbl, err := cardsettest.Set(t, "relaxrestrfilter").Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	sl := map[string]Slot{}
	for _, n := range []string{"Clement, the Worrywort", "Hidden Grotto", "Lilypad Village", "Forest"} {
		sl[n] = sc.slot(sc.perm(id[n], 0))
	}
	s, k := sc.s, sc.k

	// the activation: Lilypad Village's {U} surveil
	village := id["Lilypad Village"]
	ai := -1
	for i := range k.t.Cards[village].Abilities {
		if k.t.Cards[village].Abilities[i].Cost.Mana.Pips[kop.U] > 0 {
			ai = i
			break
		}
	}
	if ai < 0 {
		t.Fatal("Lilypad Village's {U} ability not found in the table")
	}
	a := MakeAction(AkActivate, TCode(s.perms[sl["Lilypad Village"]].obj), 0, uint8(ai))
	cost, _, _, ok := s.grantPlayCost(k, 0, a, true)
	if !ok {
		t.Fatal("grantPlayCost refused the surveil activation")
	}
	d := s.grantPayDesc(k, a)

	// preconditions: a restricted grant is live, Clement's mana is refused the
	// activation, the Grotto is a relaxable filter, and the grant alone is the
	// census's only source of {U}
	if !k.gen.anyGrantRestr() || !s.liveGrantGap(k, 0) {
		t.Fatal("no live restricted grant on this board")
	}
	gs := grantSrch{k: k, p: 0, d: d}
	if g := s.grantIdx(k, sl["Clement, the Worrywort"]); g < 0 || k.gen.grantRestr[g] == 0 || gs.grantAdmits(s, sl["Clement, the Worrywort"], int16(g)) {
		t.Fatal("Clement's grant is not a restricted one this activation is refused")
	}
	f := s.scriptFilt(k, 0)
	if !f.has(sl["Hidden Grotto"]) || !s.filtRelaxable(k, f) {
		t.Fatal("Hidden Grotto is not a relaxable filter of the census")
	}
	if s.perms[sl["Forest"]].flags&pfTapped != 0 {
		t.Fatal("the Forest that funds the filter's fee is tapped")
	}

	if s.relaxRestrRefuses(k, 0, &cost, d) {
		t.Fatal("relaxRestrRefuses proves the play insufficient though the Grotto subset pays {U} (gorge's relaxed proof searches it)")
	}

	// without the filter the same two reads differ: the grant is the {U}
	// source the payment refuses, which the filter then makes up
	var all, kept avail
	prev := s.g.availX
	s.g.availX = availPlain | availWalk
	s.availFor(k, 0, &all)
	kept.xd, kept.xdOn = d, true
	s.availFor(k, 0, &kept)
	s.g.availX = prev
	pm := pipMaskOf(&cost)
	if !all.affords(&cost, pm, 0) || kept.affords(&cost, pm, 0) {
		t.Fatalf("filterless reads: all affords %v, kept affords %v, want true, false", all.affords(&cost, pm, 0), kept.affords(&cost, pm, 0))
	}
}
