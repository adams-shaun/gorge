package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFilterSearchDoesNotRepriceAGrantRefusedActivation: beside Great Divide
// Guide's grant, Rumble Arena's filter and the Cub's watcher, Earthen Ally's
// "{2}{W}{U}{B}{R}{G}: Earthbend 5" costs more than the printed census holds
// (Jasmine Dragon Tea Shop's any-colour unit answers only an Ally play, the
// pool holds a {C}). grantCap's grant search exhausts gorge's 160-node budget
// on the play and refuses. Gorge searched the play once, so the kernel's
// filter search must not price it again on a fresh budget: legal.go caps the
// refused play (grantCap's capSpent) and scriptOffers skips it there. Its
// funding would be the granted mana — the printed census cannot pay it
// (affordsUngrantedPlay), and the filter search lists the granted sources as
// other gaps (mana_mixgaps.go's live-grant block) — so a second search that
// did not see the grant would price the very play the cap drops.
func TestFilterSearchDoesNotRepriceAGrantRefusedActivation(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Great Divide Guide", Count: 1}, {Name: "Rumble Arena", Count: 1}, {Name: "Earthen Ally", Count: 1},
		{Name: "Bender's Waterskin", Count: 1}, {Name: "Meditation Pools", Count: 1}, {Name: "Jasmine Dragon Tea Shop", Count: 1},
		{Name: "White Lotus Hideout", Count: 1}, {Name: "Badgermole Cub", Count: 4},
		{Name: "Plains", Count: 1}, {Name: "Mountain", Count: 1}, {Name: "Island", Count: 20},
	}
	tbl, _, _, err := cardsettest.Synth(t, "std-tla").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sl := map[string]Slot{}
	for _, n := range []string{"Great Divide Guide", "Rumble Arena", "Earthen Ally", "Jasmine Dragon Tea Shop",
		"Badgermole Cub", "Badgermole Cub", "Badgermole Cub", "Meditation Pools", "Plains", "Mountain", "White Lotus Hideout"} {
		o := sc.perm(id[n], 0)
		sl[n] = sc.slot(o)
	}
	s, k := sc.s, sc.k
	s.pl[0].pool[kop.C] = 1

	// precondition: the play is an activation whose cost the grant search
	// refuses within the budget, the printed census does not price it, and
	// granted, untapped sources hold mana the filter search would otherwise
	// float
	var ai int = -1
	for i := range k.t.Cards[id["Earthen Ally"]].Abilities {
		if k.t.Cards[id["Earthen Ally"]].Abilities[i].Cost.Mana.Generic > 0 {
			ai = i
			break
		}
	}
	if ai < 0 {
		t.Fatal("Earthen Ally's Earthbend ability not found in the table")
	}
	a := MakeAction(AkActivate, TCode(s.perms[sl["Earthen Ally"]].obj), 0, uint8(ai))
	cost, ex, need, ok := s.grantPlayCost(k, 0, a, true)
	if !ok {
		t.Fatal("grantPlayCost refused the Earthbend activation")
	}
	var g grantSrch
	if s.grantSearch(k, 0, &cost, ex, s.grantPayDesc(k, a), need, &g) {
		t.Fatal("the grant search pays the play: this board does not refuse it")
	}
	if g.nodes != potentialScriptNodes {
		t.Fatalf("the grant search spent %d nodes, want the %d-node budget exhausted", g.nodes, potentialScriptNodes)
	}
	if s.affordsUngrantedPlay(k, 0, a, &cost, ex, s.grantPayDesc(k, a)) {
		t.Fatal("the printed sources price the play: the grant search's refusal is not the board's")
	}
	granted := func() slotMask {
		var m slotMask
		for b := s.ctrlM[0].and(s.occ); b.any(); b = b.rest() {
			if sl := b.first(); s.grantIdx(k, sl) >= 0 && s.canTapForMana(k, sl) {
				m = m.with(sl)
			}
		}
		return m
	}
	gr := granted()
	if !gr.any() {
		t.Fatal("no granted, untapped source holds mana the float would tap")
	}
	// gorge runs one prefix search over the grants and the filters, so an
	// activation's filter search lists the granted sources beside the filter
	// and spends the same one budget (the uncapped check below)
	otherAct := s.otherGaps(k, 0, a)
	if !otherAct.and(gr).any() {
		t.Fatal("the activation's otherGaps lists no granted source: the re-pricing is not exercised here")
	}
	if !s.otherGaps(k, 0, MakeAction(AkPass, 0, 0, 0)).and(gr).any() {
		t.Fatal("the cast's otherGaps lists no granted source: the float is not exercised here")
	}

	// the cap: grantCap records the refused play and scriptOffers skips it
	var d, capSpent Decision
	d.add(a)
	s.grantCap(k, &d, 0, &capSpent)
	s.scriptOffers(k, &d, &capSpent, 0)
	if !hasOption(&capSpent, a) {
		t.Fatal("grantCap did not record the play its grant search refused")
	}
	if hasOption(&d, a) {
		t.Fatal("scriptOffers re-offers the play the grant search refused within gorge's budget")
	}

	// uncapped, the filter search now lists the granted sources as other gaps,
	// so it spends the same one budget and refuses too: pre-fix the split
	// search priced the play on a fresh budget
	var d2, freshCap Decision
	s.scriptOffers(k, &d2, &freshCap, 0)
	if hasOption(&d2, a) {
		t.Fatal("the filter search prices a play its granted sources put past gorge's budget")
	}
}
