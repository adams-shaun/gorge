package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGraveyardCastNeedPricesGrantedFlashbackAtManaCost pins the granted
// flashback instance on a card whose own graveyard alternative is Escape or
// Beam me up: those offers carry marker targets (castPrintedEscapeTgt,
// castBeamTgt), so a mode-2 target-0 cast of the card is the granted
// instance, which flashbackBase prices at the card's mana cost, not at the
// card's own alternative cost (kw_beammeup.go noteBeamCast, kw_escape.go).
// graveyardCastNeedCost is a colour-ask hint only, but a hint naming the
// alternative cost's pips orders the budget-capped search by a colour the
// cast does not pay. The alternative costs are recoloured so the two
// readings differ whatever the corpus prints.
func TestGraveyardCastNeedPricesGrantedFlashbackAtManaCost(t *testing.T) {
	cases := []struct {
		name string
		set  string
		card string
		own  func(*kop.CardDef) bool // the card's own graveyard alternative
	}{
		{
			name: "beam_me_up",
			set:  "beammeup",
			card: "Open Communications",
			own:  func(cd *kop.CardDef) bool { return cd.BeamMeUp && castBeamTgt != 0 },
		},
		{
			name: "escape",
			set:  "escape",
			card: "Lunar Hatchling",
			own:  func(cd *kop.CardDef) bool { return cd.Escape && castPrintedEscapeTgt != 0 },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deck := []cardset.Entry{
				{Name: tc.card, Count: 2},
				{Name: "Sphinx of Forgotten Lore", Count: 1}, // assigns the granted-flashback planes
				{Name: "Island", Count: 4},
				{Name: "Forest", Count: 4},
			}
			set := cardsettest.Set(t, tc.set)
			tbl, da, _, err := set.Matchup(deck, deck)
			if err != nil {
				t.Fatal(err)
			}
			id := map[string]CardID{}
			for _, c := range da {
				id[tbl.Cards[c].Name] = c
			}
			cd := &tbl.Cards[id[tc.card]]
			if !tc.own(cd) {
				t.Fatalf("precondition: %s has no own graveyard alternative with a marker target", tc.card)
			}
			// recolour the own alternative to a pip the printed cost does not
			// name, so the hint's two readings are distinguishable
			var recolour [kop.NumManaKinds]uint8
			for c := 0; c < 5; c++ {
				if !cd.Cost.HasColour(c) {
					recolour[c] = 1
					break
				}
			}
			cd.AltCost = kop.ManaCost{Pips: recolour}
			if cd.Cost == cd.AltCost {
				t.Fatalf("precondition: %s's printed and alternative costs are equal", tc.card)
			}
			sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
			o := sc.grave(id[tc.card], 0, 1)[0]
			sc.s.grantFB(sc.k, o)
			if !sc.s.fbGranted(sc.k, o) {
				t.Fatal("precondition: the granted flashback instance is not live")
			}
			a := MakeAction(AkCast, TCode(o), 0, 2)
			if _, _, ok := sc.s.playBaseCost(sc.k, a); ok {
				t.Fatal("precondition: playBaseCost prices the mode-2 cast, so the test does not reach the hint")
			}
			got := sc.s.playNeedHint(sc.k, a)
			want := grantNeedColours(cd.Cost)
			if got != want {
				t.Fatalf("playNeedHint of the granted flashback cast = %v, want the mana cost's %v (the alternative cost's %v must not steer it)",
					got, want, grantNeedColours(cd.AltCost))
			}
		})
	}
}
