package rules

import "testing"

// CR 609.4b, 601.2b: a sorcery whose target is bounded by its {X} ("Gain control
// of target creature with mana value X or less", Blue Sun's Twilight) cast from a
// zone a "mana of any type can be spent" grant opens (Gonti, Lord of Luxury's
// exiled card) is offered at X=0 when {U}{U} is payable only through the rider.
// The X-bound payability proof once read the planner's rider-blind "insufficient"
// for two Mountains and dropped the cast (decision 1092 of tail-28-c000 pair 8).
func TestMayPlayAnyTypeXBoundOfferedThroughRider(t *testing.T) {
	const spell = "Name:Xbound Grab\nManaCost:X U U\nTypes:Sorcery\n" +
		"A:SP$ GainControl | ValidTgts$ Creature.cmcLEX | TgtPrompt$ Select target creature with mana value X or less\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	build := func(rider string) (*Engine, bool) {
		e := mayPlayBase(t)
		onBoardGrant(t, e, 0, "Name:Rider Grant\nManaCost:1\nTypes:Artifact\n"+
			"S:Mode$ Continuous | Affected$ Sorcery.YouOwn | MayPlay$ True | "+rider+
			"EffectZone$ Battlefield | AffectedZone$ Graveyard\nOracle:x\n")
		onBoard(t, e, 0, apMountain)
		onBoard(t, e, 0, apMountain)
		onBoard(t, e, 1, "Name:Zero Bear\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
		card := graveyardCard(t, e, spell)
		e.pending = nil
		e.askPriority(0)
		offered, _ := mayPlayOffered(e, card)
		return e, offered
	}
	if _, offered := build("MayPlayIgnoreType$ True | "); !offered {
		t.Fatalf("X=0 cast of {X}{U}{U} from two Mountains not offered under the any-type rider")
	}
	if _, offered := build(""); offered {
		t.Fatalf("X=0 cast of {X}{U}{U} from two Mountains offered with no rider")
	}
}
