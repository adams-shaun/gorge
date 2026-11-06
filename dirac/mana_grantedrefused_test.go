package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFilterSearchDoesNotRepriceAGrantRefusedActivation: beside Great Divide
// Guide's grant, Rumble Arena's filter and the Cub's watcher, Earthen Ally's
// "{2}{W}{U}{B}{R}{G}: Earthbend 5" costs more than the printed census holds:
// the creatures but the Ally sit tapped (their taps are Taps-deferred gaps
// beside the Cub), the Jasmine Dragon Tea Shop's any-colour unit answers only
// an Ally play, and the pool holds a {C}. grantCap's grant search exhausts
// gorge's 160-node budget on the play and refuses. Gorge searches the play
// once, so the kernel must not price it again in scriptOffers' filter search
// -- not from a fresh budget (script_gen.go capSpent carries the grant
// search's spent) and not from the granted mana floated into the filter
// search's pool (mana_mixgaps.go otherGaps: an activation's granted mana is
// the grant search's). Pre-fix that second search priced the play and the
// priority decision offered it beside gorge's surface.
func TestFilterSearchDoesNotRepriceAGrantRefusedActivation(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Great Divide Guide", Count: 1}, {Name: "Rumble Arena", Count: 1}, {Name: "Earthen Ally", Count: 1},
		{Name: "Bender's Waterskin", Count: 1}, {Name: "Meditation Pools", Count: 1}, {Name: "Jasmine Dragon Tea Shop", Count: 1},
		{Name: "White Lotus Hideout", Count: 1}, {Name: "Badgermole Cub", Count: 1}, {Name: "Island", Count: 20},
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
	var ea Slot
	for _, n := range []string{"Great Divide Guide", "Rumble Arena", "Earthen Ally", "Jasmine Dragon Tea Shop",
		"Badgermole Cub", "Meditation Pools", "Plains", "Mountain"} {
		o := sc.perm(id[n], 0)
		if n == "Earthen Ally" {
			ea = sc.slot(o)
		}
	}
	s, k := sc.s, sc.k
	// the creatures but the Ally sit tapped, as the fixture's mid-game states
	// hold (the Cub's watcher keeps their taps Taps-deferred gaps)
	for b := s.ctrlM[0].and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		if sl == ea || s.cardAt(sl) == id["Badgermole Cub"] {
			continue
		}
		if k.t.Cards[s.cardAt(sl)].Types&kop.TCreature != 0 {
			s.perms[sl].flags |= pfTapped
		}
	}
	s.pl[0].pool[kop.C] = 1

	// precondition: the play is an activation whose cost the grant search
	// refuses within the budget -- and a granted, untapped source holds the
	// mana a floated filter search would tap
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
	a := MakeAction(AkActivate, uint8(s.perms[ea].obj), 0, uint8(ai))
	cost, ex, need, ok := s.grantPlayCost(k, 0, a, true)
	if !ok {
		t.Fatal("grantPlayCost refused the Earthbend activation")
	}
	var g grantSrch
	if s.grantSearch(k, 0, &cost, ex, s.grantPayDesc(a), need, &g) {
		t.Fatal("the grant search pays the play: this board does not refuse it")
	}
	if g.nodes != potentialScriptNodes {
		t.Fatalf("the grant search spent %d nodes, want the %d-node budget exhausted", g.nodes, potentialScriptNodes)
	}
	floated := false
	for b := s.ctrlM[0].and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		if s.grantIdx(k, sl) >= 0 && s.canTapForMana(k, sl) {
			floated = true
		}
	}
	if !floated {
		t.Fatal("no granted, untapped source holds the mana a floated filter search would tap")
	}

	// the play stays refused: the decision does not offer it
	var d Decision
	s.priorityOptions(k, &d)
	pass := false
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == a {
			t.Fatalf("the priority decision offers the Earthbend activation the grant search refused at %d nodes", g.nodes)
		}
		if d.Options[i].Kind() == AkPass {
			pass = true
		}
	}
	if !pass {
		t.Fatal("the priority decision holds no pass: the surface is not the priority one")
	}
}
