package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The "counter target spell if it's blue" gate: ConditionDefined$ Targeted |
// ConditionPresent$ Spell.Blue on the counter's own node (Pyroblast,
// Hydroblast, Burnout; Ertai's Trickery's Spell.kicked).
// dirac/kgen/lower_condtargetspell.go lowers the Spell base away: the
// node's one spell-only target is on the stack whenever the gate is read.
// The decks cast blue and red creatures, so the counters meet both a
// matching and a non-matching spell, in two-seat games and four-seat pods.
var targetedSpellGateDeck = []cardset.Entry{
	{Name: "Pyroblast", Count: 5}, {Name: "Hydroblast", Count: 4}, {Name: "Burnout", Count: 3},
	{Name: "Ertai's Trickery", Count: 2}, {Name: "Goblin Bushwhacker", Count: 3},
	{Name: "Wind Drake", Count: 3}, {Name: "Coral Merfolk", Count: 3},
	{Name: "Hill Giant", Count: 3}, {Name: "Gray Ogre", Count: 3}, {Name: "Opt", Count: 3},
	{Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
}

var targetedSpellGateCards = []string{"Pyroblast", "Hydroblast", "Burnout", "Ertai's Trickery"}

func targetedSpellGateTally(g *state.Game) int {
	n := 0
	for _, c := range targetedSpellGateCards {
		n += cedhZoneCount(g, state.ZGraveyard, c)
	}
	return n
}

func TestTargetedSpellGateLockstep(t *testing.T) {
	t.Parallel()
	d := targetedSpellGateDeck
	two := cedhPodRun(t, "tgtspellgate", [][]cardset.Entry{d, d}, targetedSpellGateCards, 30, targetedSpellGateTally)
	four := cedhPodRun(t, "tgtspellgate", [][]cardset.Entry{d, d, d, d}, targetedSpellGateCards, 12, targetedSpellGateTally)
	t.Logf("gated counters resolved or spent: %d in two-seat games, %d in pods", two, four)
	if two < 20 || four < 20 {
		t.Fatalf("too little evidence: %d two-seat, %d four-seat (want 20 each)", two, four)
	}
}
