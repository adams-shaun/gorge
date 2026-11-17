package rules

// CR 701.47a: amass needs an "Army creature you control". An Army token under
// an Aura that makes it a noncreature enchantment (One with the Stars,
// RemoveCardTypes) is no creature, and CR 205.1b/205.1a drop its creature
// subtypes with the type, so it is no Army either: the second amass must
// create a fresh Army token, not put counters on the enchanted one.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestAmassIgnoresAnArmyAuraMadeNoncreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	amass := card(t, amassFixtureSrc)
	aura := lookup(t, reg, "One with the Stars")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{amass, amass, aura}, nil)
	moveByName(t, e, 0, "AmassMe", state.ZHand)

	addMana(t, e, 0, "U")
	castNamed(t, e, "AmassMe")
	passUntilStackEmpty(t, e, 60)

	var army state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && e.matchesSpecFrom("Creature.Army", id, 0, 0) {
			army = id
		}
	}
	if army == 0 {
		t.Fatal("precondition: first amass made no Army creature token")
	}
	lossAttach(t, e, "One with the Stars", 0, army)

	// Preconditions: the token is under the Aura where the lookup scans, and
	// it is no longer a creature nor an Army.
	if o := e.G.Obj(army); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Army token = %v, want on the battlefield", o)
	}
	if e.matchesSpecFrom("Creature", army, 0, 0) || e.matchesSpecFrom("Army", army, 0, 0) {
		t.Fatalf("precondition: enchanted token still creature=%v Army=%v (types %v)",
			e.matchesSpecFrom("Creature", army, 0, 0), e.matchesSpecFrom("Army", army, 0, 0), e.Derived(army).Types)
	}
	before := len(e.G.Zone(state.ZBattlefield, 0))
	counters := e.G.Obj(army).Counter("P1P1")

	moveByName(t, e, 0, "AmassMe", state.ZHand)
	addMana(t, e, 0, "U")
	castNamed(t, e, "AmassMe")
	passUntilStackEmpty(t, e, 60)

	if got := e.G.Obj(army).Counter("P1P1"); got != counters {
		t.Fatalf("the enchanted noncreature token has %d +1/+1 counters, want %d", got, counters)
	}
	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != before+1 {
		t.Fatalf("battlefield went %d -> %d objects, want one new Army token", before, got)
	}
	replayCheck(t, e, cfg)
}
