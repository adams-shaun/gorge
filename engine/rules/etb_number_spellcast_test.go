package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A number chosen as a permanent enters (CR 614.12, an as-enters choice) is
// read by that permanent's own SpellCast trigger: "Card.cmcEQChosen" in
// ValidCard$ compares the cast spell's mana value with the trigger source's
// chosen number (Nyota Uhura, Communications; Talion, the Kindly Lord). A
// spell of another mana value does not trigger it (CR 603.2).
func TestEntryNumberGatesTheSpellCastTrigger(t *testing.T) {
	t.Parallel()
	officer := "Name:Signal Officer\nManaCost:2 U\nTypes:Legendary Creature Human Officer\nPT:2/4\n" +
		"K:ETBReplacement:Other:PickNum\n" +
		"SVar:PickNum:DB$ ChooseNumber | Min$ 0 | Max$ 10 | Defined$ You | SpellDescription$ As this enters, pick a number from zero to ten.\n" +
		"T:Mode$ SpellCast | ValidCard$ Card.cmcEQChosen | ValidActivatingPlayer$ Opponent | TriggerZones$ Battlefield | Execute$ OfficerDraws | " +
		"TriggerDescription$ When a foe casts a spell of the picked mana value, draw.\n" +
		"SVar:OfficerDraws:DB$ Draw\nOracle:x\n"
	jab := "Name:Ember Jab\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Player | NumDmg$ 1\nOracle:x\n"
	lash := "Name:Ember Lash\nManaCost:1 R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Player | NumDmg$ 2\nOracle:x\n"
	e, cfg, find := etbConfig(t, 44, []string{officer}, []string{jab, lash})
	off := find("Signal Officer", 0)
	addMana(t, e, 0, "UUU")
	castFirst(t, e, "cast")
	d := passUntilNonPriority(t, e, 40)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" || len(d.Options) != 11 || d.Options[0].Kind != "number" {
		t.Fatalf("expected the 0..10 entry number ask, got %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Amount == 2 {
			pick = o.Index
		}
	}
	submitChoices(t, e, pick)
	if e.G.Obj(off).ChosenNumber != 2 {
		t.Fatalf("chosen number = %d, want 2", e.G.Obj(off).ChosenNumber)
	}
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(off).Zone != state.ZBattlefield {
		t.Fatalf("officer in %s, want the battlefield", e.G.Obj(off).Zone)
	}
	oppCasts := func(id state.ObjID, mana string) {
		t.Helper()
		addMana(t, e, 1, mana)
		e.pending = nil
		e.emit(events.Event{Kind: events.Priority, Player: 0})
		e.askPriority(0)
		passToPlayerOne(t, e)
		castObj(t, e, id)
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	oppCasts(find("Ember Jab", 1), "R")
	if n := len(e.G.Zone(state.ZHand, 0)); n != hand {
		t.Fatalf("a mana value 1 spell drew: hand %d -> %d", hand, n)
	}
	oppCasts(find("Ember Lash", 1), "RR")
	if n := len(e.G.Zone(state.ZHand, 0)); n != hand+1 {
		t.Fatalf("a mana value 2 spell: hand %d -> %d, want one card drawn", hand, n)
	}
	replayCheck(t, e, cfg)
}

// Talion, the Kindly Lord's trigger reads three alternatives of one filter:
// mana value, power or toughness equal to the chosen number. A creature
// spell's printed power or toughness counts (CR 208.1); a spell matching none
// of the three does not trigger, and a spell matching two triggers once (one
// trigger, one filter; CR 603.2).
func TestEntryNumberMatchesASpellsPowerOrToughness(t *testing.T) {
	t.Parallel()
	lord := "Name:Kindly Fae\nManaCost:2 U B\nTypes:Legendary Creature Faerie Noble\nPT:3/4\n" +
		"K:ETBReplacement:Other:PickNum\n" +
		"SVar:PickNum:DB$ ChooseNumber | Min$ 1 | Max$ 10 | Defined$ You | SpellDescription$ As this enters, pick a number from one to ten.\n" +
		"T:Mode$ SpellCast | ValidCard$ Card.cmcEQChosen,Card.powerEQChosen,Card.toughnessEQChosen | ValidActivatingPlayer$ Opponent | TriggerZones$ Battlefield | Execute$ FaeDrain | " +
		"TriggerDescription$ When a foe casts a spell with mana value, power or toughness equal to the pick, they lose 2 life and you draw.\n" +
		"SVar:FaeDrain:DB$ LoseLife | Defined$ TriggeredActivator | LifeAmount$ 2 | SubAbility$ FaeDraw\nSVar:FaeDraw:DB$ Draw\nOracle:x\n"
	jab := "Name:Ember Jab\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Player | NumDmg$ 1\nOracle:x\n"
	brute := "Name:Tall Brute\nManaCost:1 R\nTypes:Creature Giant\nPT:3/1\nK:Flash\nOracle:x\n"
	wall := "Name:Squat Wall\nManaCost:R\nTypes:Creature Wall\nPT:1/3\nK:Flash\nOracle:x\n"
	triple := "Name:Triple Brute\nManaCost:2 R\nTypes:Creature Giant\nPT:3/3\nK:Flash\nOracle:x\n"
	e, cfg, find := etbConfig(t, 44, []string{lord}, []string{jab, brute, wall, triple})
	addMana(t, e, 0, "UUBB")
	castFirst(t, e, "cast")
	d := passUntilNonPriority(t, e, 40)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" || len(d.Options) != 10 || d.Options[0].Amount != 1 {
		t.Fatalf("expected the 1..10 entry number ask, got %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Amount == 3 {
			pick = o.Index
		}
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 20)
	oppCasts := func(id state.ObjID, mana string) {
		t.Helper()
		addMana(t, e, 1, mana)
		e.pending = nil
		e.emit(events.Event{Kind: events.Priority, Player: 0})
		e.askPriority(0)
		passToPlayerOne(t, e)
		castObj(t, e, id)
	}
	for _, c := range []struct {
		name, mana string
		hits       int
	}{
		{"Ember Jab", "R", 0},      // mana value 1, no power or toughness
		{"Tall Brute", "RR", 1},    // power 3
		{"Squat Wall", "R", 1},     // toughness 3
		{"Triple Brute", "RRR", 1}, // all three: one trigger
	} {
		hand, life := len(e.G.Zone(state.ZHand, 0)), e.G.Players[1].Life
		oppCasts(find(c.name, 1), c.mana)
		if n, l := len(e.G.Zone(state.ZHand, 0)), e.G.Players[1].Life; n != hand+c.hits || l != life-2*int32(c.hits) {
			t.Fatalf("%s: hand %d -> %d, opponent life %d -> %d; want %d trigger(s)", c.name, hand, n, life, l, c.hits)
		}
	}
	replayCheck(t, e, cfg)
}
