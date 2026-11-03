package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A caster's first spell of the turn is not "the second spell you cast"
// (Highspire Bell-Ringer): the announcement of {X} is made after the spell
// moved to the stack, and its own push is not yet a cast for pricing (CR
// 601.2f). With four mana, Spectral Denial's {X}{U} therefore offers X up to
// 3, and the potential-target pricing the X menu uses must not read the
// spell's own push as the earlier cast Bell-Ringer's gate wants.
func TestXAnnouncementDoesNotCountTheSpellBeingCast(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	denial := mustCorpusCard(t, reg, "Spectral Denial")
	ringer := mustCorpusCard(t, reg, "Highspire Bell-Ringer")
	bear := card(t, "Name:Bear\nManaCost:R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e := counterHands(t, []*cards.Card{bear}, []*cards.Card{denial}, nil, []*cards.Card{ringer})
	bearID, denialID := handObj(t, e, 0, "Bear"), handObj(t, e, 1, "Spectral Denial")
	e.G.Players[0].Pool[state.MR] = 1
	e.G.Players[1].Pool[state.MU] = 4
	e.askPriority(0)
	submitChoices(t, e, passToCast(t, e, bearID))
	castFirst(t, e, "pass")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 1 {
		t.Fatalf("expected seat 1's priority with the Bear on the stack, got %+v", d)
	}
	submitChoices(t, e, passToCast(t, e, denialID))
	if m := maxXOf(xAskOptions(t, e)); m != 3 {
		t.Fatalf("Spectral Denial as its caster's first spell with four mana: max X = %d, want 3", m)
	}
}
