package rules

// A land tapped under an additional-mana `TapsForMana | Static$ True` trigger
// produces the extra mana as part of the same tap (CR 106.12a, 605.1b, 605.4a), so a
// spell the land alone could not pay is offered when land plus the extra
// covers it. The offer path must price the folded production the way
// PlanCastPayment already does.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestTapManaFoldOfferedWhenExtraCompletesCost(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		groundchk bool
		want      bool
	}{
		{"extra_green_pays_the_second_pip", true, true},
		{"no_extra_control", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, spell := newFixtureDeck(t, 9921, lrSpell("1 G"))
			onBoard(t, e, 0, lrPlains)
			if tc.groundchk {
				lrCorpus(t, e, "Groundchuck & Dirtbag")
			}
			if got := e.G.Obj(spell).Zone; got != state.ZHand {
				t.Fatalf("spell zone %v, want hand", got)
			}
			offered := false
			for _, a := range e.PotentialActions(0) {
				if a.Kind == "cast" && a.Obj == spell {
					offered = true
				}
			}
			if offered != tc.want {
				t.Fatalf("PotentialActions offers the cast = %v, want %v", offered, tc.want)
			}
			if tc.want && e.PlanCastPayment(0, paymentCast(spell)).Plan == nil {
				t.Fatal("the offered cast has no payment plan")
			}
		})
	}
}
