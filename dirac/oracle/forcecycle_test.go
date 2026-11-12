package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Force cycle lockstep: each Force's "If it's not your turn, you may
// exile a <colour> card from your hand rather than pay this spell's mana
// cost" (an AlternativeCost static whose Condition$ NotPlayerTurn gates the
// offer, Cost$ a bare ExileFromHand<1/Card.<colour>+Other>, CR 118.9) and
// Mine Collapse's PlayerTurn sacrifice twin beside it. Seat 0 pitches Force
// of Negation with Counterspell and Unsummon fodder; seat 1 holds Force of
// Despair (Dark Ritual the black
// fodder), Force of Vigor (a Grizzly Bears or a Forest the green fodder),
// Mine Collapse, Shock and the lands both the sacrifice and the counts
// read. The expected answer at every decision is gorge's. (Misdirection is
// beside the pool: its ChangeTargets spell body still refuses, a reason
// that is not the cost.)
var kforcecycleDecks = [2][]cardset.Entry{
	{
		{Name: "Force of Negation", Count: 4}, {Name: "Counterspell", Count: 8},
		{Name: "Unsummon", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 20},
	},
	{
		{Name: "Force of Despair", Count: 4}, {Name: "Force of Vigor", Count: 4},
		{Name: "Mine Collapse", Count: 4}, {Name: "Dark Ritual", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Forest", Count: 8},
	},
}

func TestForceCycleLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "forcecycle", kforcecycleDecks, 240, "KFORCECYCLE_SEEDS")
}

// TestForceCycleLockstepMutation: with the turn gate dropped (the alternative
// offered on the caster's own turn too), or the whole alternative dropped,
// some seed diverges, so the pool plays the exile cast and the kernel prices
// its condition.
func TestForceCycleLockstepMutation(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name string
		fn   func(*kop.Ability)
	}{
		{"no turn gate", func(a *kop.Ability) { a.Static.Cond2 = 0 }},
		{"no alternative", func(a *kop.Ability) { a.Static.Kind = kop.StatNone }},
	} {
		n := lockstepRun(t, "forcecycle", kforcecycleDecks, 120, false, func(tbl *kop.Table) {
			cards := append([]kop.CardDef(nil), tbl.Cards...)
			for i := range cards {
				abs := append([]kop.Ability(nil), cards[i].Abilities...)
				for j := range abs {
					if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == kop.StatAlternativeCost {
						m.fn(&abs[j])
					}
				}
				cards[i].Abilities = abs
			}
			tbl.Cards = cards
		})
		if n == 0 {
			t.Errorf("%s: no seed diverged: the pool never plays an exile alternative", m.name)
		}
	}
}
