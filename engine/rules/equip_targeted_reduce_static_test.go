package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 601.2f, 702.6a: a creature's "Equip abilities that target it cost {2} less"
// (Cloud, Planet's Champion) prices a {3} equip at {1} once Cloud is the
// target, so one untapped land makes the equip offerable.
func TestEquipOfferedWhenTargetCreatureReducesTheCost(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for _, n := range []string{"Red Mage's Rapier", "Cloud, Planet's Champion", "Gohn, Town of Ruin"} {
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the corpus registry", n)
		}
		deck = append(deck, c)
	}
	cfg := Config{Seed: 601, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 38)...), mountainDeck(t, 40)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	rapier := findAndMoveToHand(t, e, 0, "Red Mage's Rapier")
	moveToBattlefield(t, e, rapier)
	cloud := findAndMoveToHand(t, e, 0, "Cloud, Planet's Champion")
	moveToBattlefield(t, e, cloud)
	gohn := findAndMoveToHand(t, e, 0, "Gohn, Town of Ruin")
	moveToBattlefield(t, e, gohn)
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	// Precondition: the equip really is {3} (not free), and Cloud is on board.
	if e.G.Obj(cloud).Zone != state.ZBattlefield || e.G.Obj(rapier).Zone != state.ZBattlefield {
		t.Fatal("fixture permanents are not on the battlefield")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %#v, want priority", d)
	}
	if e.G.Obj(gohn).Tapped {
		t.Fatal("Gohn entered tapped and was not untapped")
	}
	if !hasOptionKindFor(e, "ability", rapier) {
		t.Fatalf("Red Mage's Rapier's equip not offered from Gohn's one mana against Cloud ({3} - {2})")
	}
}
