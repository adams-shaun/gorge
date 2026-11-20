package rules

// Expel the Unworthy's chained shape: the main ChangeZone targets a creature
// with mana value 3 or less (TargetMin$ X / TargetMax$ X, X = 0 when
// kicked), the Kicked-only DBChangeZone chooses its own target (target
// slot 1), and the trailing D BGainLife reads "its controller" and "its mana
// value" through Defined$ TargetedController and Targeted$CardManaCost. Only
// one of the two target slots exists at resolution: when kicked the root
// chose none (Ctx.Targets stays empty) and the creature is the sub's own
// target link, so the gain must read that link (CR 608.2h's last-known
// information for the exiled creature's controller and mana value).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const expelUnworthyProbeSrc = "Name:Expel the Unworthy\nManaCost:1 W\nTypes:Sorcery\n" +
	"K:Kicker:2 W\n" +
	"A:SP$ ChangeZone | Origin$ Battlefield | Destination$ Exile | ValidTgts$ Creature.cmcLE3 | TargetMin$ X | TargetMax$ X | SubAbility$ DBChangeZone\n" +
	"SVar:DBChangeZone:DB$ ChangeZone | TargetMin$ Y | TargetMax$ Y | Origin$ Battlefield | Destination$ Exile | ValidTgts$ Creature | Condition$ Kicked | SubAbility$ DBGainLife\n" +
	"SVar:DBGainLife:DB$ GainLife | Defined$ TargetedController | LifeAmount$ Z\n" +
	"SVar:X:Count$Kicked.0.1\n" +
	"SVar:Y:Count$Kicked.1.0\n" +
	"SVar:Z:Targeted$CardManaCost\n" +
	"Oracle:x\n"

const expelBurglarSrc = "Name:Test Burglar\nManaCost:3 B\nTypes:Creature Human Rogue\nPT:2/2\nOracle:x\n"
const expelEffigySrc = "Name:Test Effigy\nManaCost:2 B\nTypes:Artifact Creature Scarecrow\nPT:1/1\nOracle:x\n"

// expelCastOption picks the cast option for id with the requested kicker
// mode ("" for the plain cast).
func expelCastOption(t *testing.T, e *Engine, id state.ObjID, mode string) int {
	t.Helper()
	idx := -1
	for _, o := range castOptions(t, e) {
		if o.Obj == id && o.Mode == mode {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for %d with mode %q: %+v", id, mode, castOptions(t, e))
	}
	return idx
}

// expelPickTarget answers the pending target ask with foe.
func expelPickTarget(t *testing.T, e *Engine, foe state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("want a target ask, got %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == foe {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("target ask does not offer object %d: %+v", foe, d.Options)
	}
	submitChoices(t, e, pick)
}

// TestExpelTheUnworthyKickedGainsExiledMV is the kicked arm: the creature
// exiled by the sub link's own target is the one whose controller gains its
// mana value. Before the parent-link fallback, Ctx.Targets was empty (the
// root's "instead" bound resolved to zero targets) so the gain read nobody.
func TestExpelTheUnworthyKickedGainsExiledMV(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeckWithOpponentCard(t, 9311, expelUnworthyProbeSrc, testBearSrc, expelBurglarSrc)
	// A legal root target (mana value <= 3) keeps the kicked cast offer
	// alive; the kicked sub then chooses the mana value 4 creature instead.
	if putCreature(t, e, 0, testBearSrc) == 0 {
		t.Fatal("precondition: root-legal creature not placed")
	}
	foe := putCreature(t, e, 1, expelBurglarSrc)
	if foe == 0 {
		t.Fatal("precondition: opponent creature not placed")
	}
	if mv := e.G.Obj(foe).Face().ManaValue(); mv != 4 {
		t.Fatalf("precondition: want a mana value 4 creature, got %d", mv)
	}
	addMana(t, e, 0, "WWWWW")

	submitChoices(t, e, expelCastOption(t, e, id, "kicked"))
	expelPickTarget(t, e, foe)
	passAll(t, e, 12)

	if o := e.G.Obj(foe); o == nil || o.Zone != state.ZExile {
		t.Fatalf("kicked Expel did not exile the chosen creature: %+v", e.G.Obj(foe))
	}
	if got := e.G.Players[1].Life; got != 24 {
		t.Fatalf("kicked Expel: opponent life = %d, want 24 (20 + exiled creature's mana value 4)", got)
	}
	replayCheck(t, e, cfg)
}

// TestExpelTheUnworthyUnkickedGainsExiledMV is the unkicked control: the
// root's own target is the exiled creature and its controller gains its mana
// value through the same trailing link.
func TestExpelTheUnworthyUnkickedGainsExiledMV(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeckWithOpponentCard(t, 9312, expelUnworthyProbeSrc, testBearSrc, expelEffigySrc)
	foe := putCreature(t, e, 1, expelEffigySrc)
	if foe == 0 {
		t.Fatal("precondition: opponent creature not placed")
	}
	if mv := e.G.Obj(foe).Face().ManaValue(); mv != 3 {
		t.Fatalf("precondition: want a mana value 3 creature, got %d", mv)
	}
	addMana(t, e, 0, "WW")

	submitChoices(t, e, expelCastOption(t, e, id, ""))
	expelPickTarget(t, e, foe)
	passAll(t, e, 12)

	if o := e.G.Obj(foe); o == nil || o.Zone != state.ZExile {
		t.Fatalf("unkicked Expel did not exile the chosen creature: %+v", e.G.Obj(foe))
	}
	if got := e.G.Players[1].Life; got != 23 {
		t.Fatalf("unkicked Expel: opponent life = %d, want 23 (20 + exiled creature's mana value 3)", got)
	}
	replayCheck(t, e, cfg)
}
