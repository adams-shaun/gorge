package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Araumi of the Dead Tide's pump lowers whole. Its cost {T}
// ExileFromGrave<X/Card> (SVar X PlayerCountOpponents$Amount) is the fixed
// one-card exile of a two-seat card (lower_exilex.go; gorge pay.FixLifeXCost
// drops the announcement), and the pump's body is the whole Encore keyword grant on a graveyard card
// (gorge registers the derived keyword [Encore:CardManaCost] and offers no
// activation of it — kw_granted.go synthesizes no Encore body — so
// ExtGrantEncore encodes the grant alone, kw_encoregrant.go).
func TestAraumiGrantsEncore(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lower := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}

	ara := lower("Araumi of the Dead Tide")
	if len(ara.Refusals) != 0 {
		t.Fatalf("Araumi refusals = %v, want none", ara.Refusals)
	}
	if len(ara.Abilities) != 1 {
		t.Fatalf("Araumi abilities = %d, want 1", len(ara.Abilities))
	}
	ab := ara.Abilities[0]
	c := ab.Cost
	if !c.Tap {
		t.Fatalf("Araumi cost taps nothing: %s", costString(c))
	}
	if c.ExileG == nil || c.ExileGX || c.ExileGN != 1 {
		t.Fatalf("Araumi cost is not the fixed one-card exile: %s", costString(c))
	}
	if c.Mana.X {
		t.Fatalf("Araumi cost carries a mana {X}: %s", costString(c))
	}
	if len(ab.Prog) != 1 {
		t.Fatalf("Araumi program = %v, want the one GrantEncore", ab.Prog)
	}
	in := ab.Prog[0]
	if in.Op != "GrantEncore" || in.Obj.Kind != rTarget || in.Obj.N != 0 {
		t.Fatalf("Araumi program instruction %+v, want GrantEncore on target 0", in)
	}
	if len(ab.Targets) != 1 {
		t.Fatalf("Araumi targets = %d, want 1", len(ab.Targets))
	}
	tg := ab.Targets[0]
	if tg.Zone != zGraveyard {
		t.Fatalf("Araumi target zone = %d, want the graveyard", tg.Zone)
	}
	if tg.Min.Kind != vConst || tg.Min.N != 1 || tg.Max.Kind != vConst || tg.Max.N != 1 {
		t.Fatalf("Araumi target slot bounds %v..%v, want 1..1", tg.Min, tg.Max)
	}

	// The class stays refused: a Count$xPaid X is the paid mana X the
	// ability's body reads at resolution (NumAtt$ -X), which the kernel's
	// ability body cannot read, and Wire Surgeons' static Encore grant is a
	// different card family (out of scope).
	for _, name := range []string{"Painbringer", "Taigam, Sidisi's Hand", "Necropolis Fiend", "Wire Surgeons"} {
		lc := lower(name)
		if len(lc.Refusals) == 0 {
			t.Fatalf("%s lowers: want the refusal", name)
		}
	}
}
