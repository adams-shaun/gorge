package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRestrictedGatedColourlessPairView: Shrine of the Forsaken Gods ({T}: Add
// {C}; {T}: Add {C}{C}, spend only to cast colorless spells, activate only if
// you control seven or more lands) is a restricted pair unit whose larger
// alternative is two {C} (wideN 2, wideCol C: colourless is one kind of mana,
// CR 106.1b), and the alternative is there only while the gate holds (CR
// 602.5b, gorge pay.ManaActivationGateHolds). The kernel read the {C}{C} as a
// widened {C} with no larger amount, so it paid a colorless Necropede with the
// Shrine's {C} and a Forest where gorge tapped the Shrine alone (tail-08 pair
// 2, decision 420).
func TestRestrictedGatedColourlessPairView(t *testing.T) {
	set := cardsettest.Set(t, "restrictedgatedpair")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = CardID(c)
	}
	for _, n := range []string{"Shrine of the Forsaken Gods", "Forest", "Necropede", "Grizzly Bears"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("restrictedgatedpair table lacks %s", n)
		}
	}
	shrine := id["Shrine of the Forsaken Gods"]
	sc := newScenTableLib(t, tbl, id["Forest"], Config{})
	g := sc.k.gen
	rec := g.restrRecOf(shrine)
	if rec == nil || rec.kind != rkWiden {
		t.Fatalf("Shrine's restricted record = %+v, want a widened source", rec)
	}
	if v := &g.rs.views[rec.view]; v.wideN != 2 || v.wideCol != kop.ColC {
		t.Fatalf("Shrine's view wideN %d wideCol %v, want 2 of {C}", v.wideN, v.wideCol)
	}
	b := sc.perm(shrine, 0)
	sl := sc.slot(b)
	colourless := payDesc{class: pdSpell, card: id["Necropede"], src: NoSlot}
	green := payDesc{class: pdSpell, card: id["Grizzly Bears"], src: NoSlot}
	for i := 0; i < 5; i++ {
		sc.perm(id["Forest"], 0)
	}
	// six lands: the gate is off, the Shrine is its {C} alone
	if v, _, _ := sc.s.restrView(sc.k, 0, sl, sc.s.manaPCGen(sc.k, sl), &colourless); v != nil {
		t.Fatalf("with six lands the Shrine's gated {C}{C} is a unit of a colorless spell's payment")
	}
	sc.perm(id["Forest"], 0)
	// seven lands: a colorless spell may spend the {C}{C}, a green one may not
	if v, k, _ := sc.s.restrView(sc.k, 0, sl, sc.s.manaPCGen(sc.k, sl), &colourless); v == nil || k != rkWiden || v.wideN != 2 {
		t.Fatalf("with seven lands a colorless spell's payment does not admit the Shrine's {C}{C} (kind %d)", k)
	}
	if v, _, _ := sc.s.restrView(sc.k, 0, sl, sc.s.manaPCGen(sc.k, sl), &green); v != nil {
		t.Fatalf("a green spell's payment admits the Shrine's colorless-only {C}{C}")
	}
	var base avail
	sc.s.availFor(sc.k, 0, &base)
	if base.total != 7 {
		t.Fatalf("the seven lands plan %d mana without the restricted alternative, want 7", base.total)
	}
	if got, _, _ := sc.s.restrSrcAvail(sc.k, 0, colourless, &base, NoSlot, nil, 0); got.total != 8 {
		t.Errorf("restrSrcAvail for a colorless spell: %d mana, want 8 (the Shrine's {C}{C} and six Forests)", got.total)
	}
}
