package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// terrorLike is a Tolarian Terror-shaped self cost reduction: an EffectZone$
// All ReduceCost on the card itself counting "instant and sorcery cards in
// your graveyard".
const terrorLike = "Name:Terror Probe\nManaCost:3 U\nTypes:Creature Serpent\nPT:5/5\n" +
	"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ X | EffectZone$ All | Description$ costs less\n" +
	"SVar:X:Count$ValidGraveyard Instant.YouOwn,Sorcery.YouOwn\nOracle:x\n"

const stashGrantLike = "Name:Stash Grant\nTypes:Enchantment\n" +
	"S:Mode$ Continuous | MayPlay$ True | Affected$ Card.YouDontOwn | AffectedZone$ Exile | MayPlayIgnoreType$ True\nOracle:x\n"

const sorceryFiller = "Name:Filler Sorcery\nManaCost:R\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

// A spell's own cost static reads "you" as its CASTER (CR 601.2a, 601.2f:
// the card is on the stack under the caster's control when its total cost
// is determined), also when the card is cast from another player's zone
// through a may-play permission (Tinybones' stash). gorge counted the
// OWNER's graveyard for Tolarian Terror cast by the opponent (dirac lockstep
// pairs 1, 8835, 9227, 9239) and the owner's Wizards for Arcane Epiphany
// (pair 4716).
func TestSelfCostStaticReadsTheCaster(t *testing.T) {
	setup := func(casterGrave, ownerGrave int) (*Engine, state.ObjID) {
		e := mayPlayBase(t)
		onBoardGrant(t, e, 0, stashGrantLike)
		o := e.G.AddObject(card(t, terrorLike), 1)
		o.Zone = state.ZExile
		e.G.SetZone(state.ZExile, 1, append(append([]state.ObjID(nil), e.G.Zone(state.ZExile, 1)...), o.ID))
		for i := 0; i < casterGrave; i++ {
			graveCard(e, card(t, sorceryFiller), 0, 0)
		}
		for i := 0; i < ownerGrave; i++ {
			graveCard(e, card(t, sorceryFiller), 1, 1)
		}
		addMana(t, e, 0, "UU")
		e.priorityRound()
		return e, o.ID
	}
	offered := func(e *Engine, id state.ObjID) bool {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("pending = %+v, want priority", d)
		}
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == id {
				return true
			}
		}
		return false
	}
	// {3}{U} less two from the caster's graveyard = {1}{U}: castable with {U}{U}.
	if e, id := setup(2, 0); !offered(e, id) {
		t.Fatalf("cast not offered: the caster's two sorceries must reduce {3}{U} to {1}{U}: %+v", e.Pending().Options)
	}
	// The owner's graveyard is not the caster's: {3}{U} stays, unaffordable.
	if e, id := setup(0, 2); offered(e, id) {
		t.Fatalf("cast offered with {U}{U}: the OWNER's graveyard reduced the opponent's cast")
	}
}
