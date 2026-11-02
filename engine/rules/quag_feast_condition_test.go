package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A chain link reads its own cast-time target in its condition: Quag Feast mills
// two first, then destroys the chosen permanent only if its mana value is at most
// the cards in your graveyard. With an empty graveyard, mana value 3 survives the
// two milled cards and mana value 2 does not (CR 608.2b).
func TestLinkConditionReadsTheLinksOwnTarget(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		victim    string
		destroyed bool
	}{{"Hill Giant", false}, {"Grizzly Bears", true}} {
		t.Run(tc.victim, func(t *testing.T) {
			t.Parallel()
			e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Quag Feast")}, []*cards.Card{lookup(t, reg, tc.victim)})
			victim := moveCorpusCard(t, e, tc.victim, 1, state.ZBattlefield)
			feast := moveCorpusCard(t, e, "Quag Feast", 0, state.ZHand)
			addMana(t, e, 0, "1B")
			castSpellAtTarget(t, e, feast, victim)
			if got, want := e.G.Obj(victim).Zone == state.ZGraveyard, tc.destroyed; got != want {
				t.Fatalf("%s destroyed = %v, want %v", tc.victim, got, want)
			}
			replayCheck(t, e, cfg)
		})
	}
}
