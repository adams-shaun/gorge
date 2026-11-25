package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Three Forge parameters gorge used to drop on the floor, each pinned against
// Forge's own handling (authored fixtures; never corpus text):
//   - LethalDamageByPower$ (Zilortha, Strength Incarnate): Card.getLethal
//     returns net power, read by the CR 704.5g sweep and the attacker's
//     lethal-assignment minimum.
//   - NoColoredMana$ on a SpellCast trigger (Void Mirror):
//     TriggerSpellAbilityCastOrCopy rejects a cast paid with any coloured mana.
//   - OptionalDecider$ on ImmediateTrigger (Etched Slith, Dream Eater):
//     ImmediateTriggerEffect copies the line's params onto the minted
//     trigger, so the reflexive ability is optional (CR 603.5).

const lethalTitan = "Name:Power Titan\nManaCost:3 R G\nTypes:Creature Dinosaur\nPT:7/3\n" +
	"S:Mode$ Continuous | Affected$ Creature.YouCtrl | LethalDamageByPower$ True | Description$ x\nOracle:x\n"

const lethalPup = "Name:Tough Pup\nManaCost:1 G\nTypes:Creature Dog\nPT:1/4\nOracle:x\n"

// TestLethalDamageByPowerJudgesByPower: under the static a 7/3 survives 3
// damage (lethal is its power, 7) and a 1/4 dies to 1; an opponent's 1/4 is
// not affected (Creature.YouCtrl) and survives 1.
func TestLethalDamageByPowerJudgesByPower(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	titan := onBoard(t, e, 0, lethalTitan)
	pup := onBoard(t, e, 0, lethalPup)
	oppPup := onBoard(t, e, 1, lethalPup)
	if got := e.LethalDamageThreshold(titan); got != 7 {
		t.Fatalf("titan lethal threshold = %d, want its power 7", got)
	}
	e.emit(events.Event{Kind: events.Damage, Obj: titan, Amount: 3})
	e.emit(events.Event{Kind: events.Damage, Obj: pup, Amount: 1})
	e.emit(events.Event{Kind: events.Damage, Obj: oppPup, Amount: 1})
	e.checkStateBased()
	if z := e.G.Obj(titan).Zone; z != state.ZBattlefield {
		t.Fatalf("titan zone = %s: 3 damage is not lethal to a 7-power creature under LethalDamageByPower$", z)
	}
	if z := e.G.Obj(pup).Zone; z != state.ZGraveyard {
		t.Fatalf("pup zone = %s: 1 damage is lethal to a 1-power creature under LethalDamageByPower$", z)
	}
	if z := e.G.Obj(oppPup).Zone; z != state.ZBattlefield {
		t.Fatalf("opponent's pup zone = %s: the static affects only its controller's creatures", z)
	}
	if need := e.lethalDamageNeed(titan, false); need != 4 {
		t.Fatalf("lethal assignment to the titan = %d, want 7 power - 3 marked = 4", need)
	}
}

const mirrorGlass = "Name:Hollow Glass\nManaCost:2\nTypes:Artifact\n" +
	"T:Mode$ SpellCast | ValidCard$ Card | ValidActivatingPlayer$ Player | TriggerZones$ Battlefield | Execute$ TrigVoid | NoColoredMana$ True | TriggerDescription$ x\n" +
	"SVar:TrigVoid:DB$ Counter | Defined$ TriggeredSpellAbility\nOracle:x\n"

const mirrorBauble = "Name:Plain Bauble\nManaCost:2\nTypes:Artifact\nOracle:x\n"

// noColoredManaCast casts the generic-cost Bauble paid with pay and reports
// whether the Glass's trigger joined it on the stack.
func noColoredManaCast(t *testing.T, seed uint64, pay string) bool {
	t.Helper()
	e, cfg, spell := newFixtureDeck(t, seed, mirrorBauble, mirrorGlass)
	moveSeeded(t, e, 0, mirrorGlass, state.ZBattlefield)
	addMana(t, e, 0, pay)
	submitChoices(t, e, castOptionFor(t, e, spell).Index)
	for i := 0; i < 10 && e.Pending() != nil && e.Pending().Kind != decision.KPriority; i++ {
		submitChoices(t, e, 0)
	}
	fired := len(e.G.Stack) == 2
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)
	return fired
}

// TestNoColoredManaGatesTheCastTrigger: a spell paid with colourless mana
// triggers the NoColoredMana$ reader; the same spell paid with blue does not.
func TestNoColoredManaGatesTheCastTrigger(t *testing.T) {
	if !noColoredManaCast(t, 431, "CC") {
		t.Fatal("paid {C}{C}: no coloured mana was spent, the trigger must fire")
	}
	if noColoredManaCast(t, 432, "UU") {
		t.Fatal("paid {U}{U}: coloured mana was spent, the trigger must not fire")
	}
}

const reflexMaybe = "Name:Maybe Mist\nManaCost:1 U\nTypes:Instant\n" +
	"A:SP$ GainLife | Defined$ You | LifeAmount$ 1 | SubAbility$ DBWhen | SpellDescription$ x\n" +
	"SVar:DBWhen:DB$ ImmediateTrigger | Execute$ TrigNap | OptionalDecider$ You | TriggerDescription$ When you do, you may tap target creature.\n" +
	"SVar:TrigNap:DB$ Tap | ValidTgts$ Creature | TgtPrompt$ Select target creature\n" +
	"Oracle:x\n"

// TestReflexiveOptionalDeciderAsksAtResolution: the reflexive half of an
// ImmediateTrigger carrying OptionalDecider$ You is an optional trigger; its
// controller is asked as it resolves and a no leaves the target untouched.
// The ask is not an api:Effect election (EffectOptional stays false).
func TestReflexiveOptionalDeciderAsksAtResolution(t *testing.T) {
	for _, accept := range []bool{true, false} {
		e, cfg, spell := newFixtureDeck(t, 433, reflexMaybe, reflexOx)
		ox := moveSeeded(t, e, 0, reflexOx, state.ZBattlefield)
		d := castReflex(t, e, spell)
		if d.Kind != decision.KTarget {
			t.Fatalf("after the spell resolved: %s, want the reflexive target ask", d.Kind)
		}
		submitChoices(t, e, reflexTargetOption(t, d, ox))
		asked := false
		for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
			d = e.Pending()
			if d == nil {
				t.Fatal("no decision while the reflexive ability is on the stack")
			}
			if d.Kind == decision.KTriggerOptional {
				asked = true
				if d.Player != 0 || d.EffectOptional {
					t.Fatalf("optional ask = player %d EffectOptional %v, want seat 0, not an Effect election", d.Player, d.EffectOptional)
				}
				pick := 1
				if accept {
					pick = 0
				}
				submitChoices(t, e, pick)
				continue
			}
			passPriority(t, e)
		}
		if !asked {
			t.Fatal("the reflexive ability resolved without asking its OptionalDecider$")
		}
		if got := e.G.Obj(ox).Tapped; got != accept {
			t.Fatalf("accept=%v: ox tapped = %v", accept, got)
		}
		replayCheck(t, e, cfg)
	}
}
