package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// filterFeeBoard is boundBoard with a Grizzly Bears in seat 1's deck, moved
// to its battlefield: the target of a creature-targeting sorcery.
func filterFeeBoard(t *testing.T, board, hand []string) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for _, n := range append(append([]string{}, board...), hand...) {
		deck = append(deck, lookup(t, reg, n))
	}
	opp := append([]*cards.Card{lookup(t, reg, "Grizzly Bears")}, mountainDeck(t, 39)...)
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), opp},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	for _, n := range board {
		moveToBattlefield(t, e, findAndMoveToHand(t, e, 0, n))
	}
	for _, n := range hand {
		findAndMoveToHand(t, e, 0, n)
	}
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		e.G.Obj(id).SummonSick = false
	}
	moveToBattlefield(t, e, findAndMoveToHand(t, e, 1, "Grizzly Bears"))
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.Face() != nil && o.Face().Name == "Island" {
			o.Tapped = false
		}
	}
	return e
}

// Hydraulic Helper's {U} (artifact spells only, CR 106.6) still pays the {1}
// of Unknown Shores' "{1}, {T}: Add one mana of any color", an activated
// ability and not a spell: Helper -> the land's fee -> {U}, plus three
// Swamps, casts a {3}{U} spell.
func TestRestrictedSourceFundsAFilterFee(t *testing.T) {
	t.Parallel()
	e := filterFeeBoard(t,
		[]string{"Swamp", "Swamp", "Swamp", "Unknown Shores", "Hydraulic Helper", "Rewrite History", "Island", "Project Deathlok Soldier", "Stature, Size Shifter"},
		[]string{"Trickster's Stratagem"})
	strat := boundHandID(t, e, "Trickster's Stratagem")
	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	t.Errorf("census complete %v relaxable %v gaps %v", census.complete, census.relaxable, boundCensusGapNames(t, e, census))
	option := boundPlayOption(t, e, strat)
	_, ok := e.potentialPrefixScript(0, option, census.gaps, census.fees())
	t.Errorf("prefix script ok=%v", ok)
	plans := e.PotentialPaymentPlans(0)
	for i := range plans {
		if plans[i].Action.Kind == "cast" && plans[i].Action.Obj == strat {
			t.Errorf("plan reason %q detail %q script %d", plans[i].Reason, plans[i].Detail, len(plans[i].Script))
		}
	}
}

func TestRestrictedSourceFilterFeeOffered(t *testing.T) {
	t.Parallel()
	e := filterFeeBoard(t, []string{"Swamp", "Swamp", "Swamp", "Unknown Shores", "Hydraulic Helper", "Rewrite History", "Island", "Project Deathlok Soldier", "Stature, Size Shifter"}, []string{"Trickster's Stratagem"})
	strat := boundHandID(t, e, "Trickster's Stratagem")
	t.Errorf("offered casts: %d", len(castOptionsFor(e, strat)))
}
