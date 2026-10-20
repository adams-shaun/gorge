package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// removalBoard builds the Board the A1 tests read: a castable Destroy spell
// in hand, one opposing creature, and a pass option. The spell's card facts
// are the ones both adapters fill (Castable, RemovalKind, RemovalTargets).
func removalBoard(power, toughness int32, armed bool) Board {
	b := priorityCards(map[state.ObjID]Card{
		10: {CMC: 2, Castable: true, RemovalKind: "destroy", RemovalTargets: "creature"},
	})
	b.Creatures = TableOf(map[state.ObjID]Creature{
		20: {Power: power, Toughness: toughness, Controller: 1},
	})
	b.Life = TableOf(map[state.PlayerID]int32{0: 20, 1: 20})
	if armed {
		b.Removal = RemovalRules{HoldForBigger: true}
	}
	return b
}

func removalOptions() []decision.Option {
	return []decision.Option{
		{Index: 0, Kind: "cast", Obj: 10},
		{Index: 1, Kind: "pass"},
	}
}

// TestRemovalNowHoldsSmallThreat is the A1 firing evidence: against a 3/3
// (threat 21, below the 24 bar) the armed rule holds the spell, so the same
// decision that casts it with the rule off passes instead.
func TestRemovalNowHoldsSmallThreat(t *testing.T) {
	off, _ := castDecision(removalBoard(3, 3, false), removalOptions())
	if off != 0 {
		t.Fatalf("rule off: chose option %d, want the removal cast (0)", off)
	}
	on, _ := castDecision(removalBoard(3, 3, true), removalOptions())
	if on != 1 {
		t.Fatalf("rule on: chose option %d, want the pass (1) while a 3/3 threat is below the bar", on)
	}
}

// TestRemovalNowFiresAtLargeThreat: a 6/6 (threat 36) clears the bar, so the
// rule leaves the cast alone in both configurations.
func TestRemovalNowFiresAtLargeThreat(t *testing.T) {
	for _, armed := range []bool{false, true} {
		got, d := castDecision(removalBoard(6, 6, armed), removalOptions())
		if got != 0 {
			t.Fatalf("armed=%v: chose option %d (obj %d), want the removal cast (0)", armed, got, d.Options[got].Obj)
		}
	}
}

// TestRemovalNowLethalFaceDamageFires is the face half of the A1 rule: a
// bolt (damage 3, any target) against an opponent at 2 life kills them, so
// the winning burn fires now in both configurations, even with a 1/1 on the
// table (threat 11, far below the bar). Without the face candidate the
// armed rule held this exact board.
func TestRemovalNowLethalFaceDamageFires(t *testing.T) {
	for _, armed := range []bool{false, true} {
		b := removalBoard(1, 1, armed)
		b.Cards.Set(10, Card{CMC: 1, ManaCost: "R", Castable: true,
			RemovalKind: "damage", RemovalTargets: "any", RemovalDamage: 3})
		b.Life.Set(1, 2)
		got, d := castDecision(b, removalOptions())
		if got != 0 {
			t.Fatalf("armed=%v: chose option %d (obj %d), want the lethal burn cast (0)", armed, got, d.Options[got].Obj)
		}
	}
}

// TestRemovalNowHoldsNonLethalFaceDamage is the face half's complement: the
// same bolt against an opponent at 20 life with a 1/1 on the table is not a
// face kill and the 1/1 is below the bar, so the armed rule holds it.
func TestRemovalNowHoldsNonLethalFaceDamage(t *testing.T) {
	off, _ := castDecision(removalBoard(1, 1, false), removalOptions())
	if off != 0 {
		t.Fatalf("rule off: chose option %d, want the burn cast (0)", off)
	}
	on, _ := castDecision(removalBoard(1, 1, true), removalOptions())
	if on != 1 {
		t.Fatalf("rule on: chose option %d, want the pass (1): 3 damage at 20 life is no face kill and the 1/1 is below the bar", on)
	}
}

// TestRemovalNowUnknownDamageGetsNoFaceCandidate: a dynamic damage figure
// (NumDmg$ X, RemovalDamage 0) is "unknown", never lethal -- the ft1
// contract's standing rule -- so the armed rule holds the spell against a
// 1/1 even though the opponent sits at 1 life.
func TestRemovalNowUnknownDamageGetsNoFaceCandidate(t *testing.T) {
	b := removalBoard(1, 1, true)
	b.Cards.Set(10, Card{CMC: 1, ManaCost: "R", Castable: true,
		RemovalKind: "damage", RemovalTargets: "any", RemovalDamage: 0})
	b.Life.Set(1, 1)
	if got, _ := castDecision(b, removalOptions()); got != 1 {
		t.Fatalf("rule on, unknown damage: chose option %d, want the pass (1)", got)
	}
}

// TestRemovalNowNonCreatureDestroyFires is the scope half of the A1 rule: a
// Shatter-shaped destroy (ValidTgts$ Artifact, RemovalTargets "") can never
// aim at a creature, so a 1/1 on the table is no reason to hold it, in both
// configurations. Without the scope fact the armed rule held this board.
func TestRemovalNowNonCreatureDestroyFires(t *testing.T) {
	for _, armed := range []bool{false, true} {
		b := removalBoard(1, 1, armed)
		b.Cards.Set(10, Card{CMC: 2, ManaCost: "2", Castable: true,
			RemovalKind: "destroy", RemovalTargets: ""})
		if got, d := castDecision(b, removalOptions()); got != 0 {
			t.Fatalf("armed=%v: chose option %d (obj %d), want the artifact-destroy cast (0)", armed, got, d.Options[got].Obj)
		}
	}
}

// TestRemovalNowIgnoresUntargetableThreat: a hexproof 6/6 (threat 36, well
// above the bar) is not a legal target of an opponent's removal, so the
// armed rule reads only the targetable 1/1 beside it (threat 11) and holds;
// with the rule off the cast fires as always. Without the targetability
// filter the untargetable 6/6 cleared the bar and the rule spent the removal
// on the 1/1.
func TestRemovalNowIgnoresUntargetableThreat(t *testing.T) {
	build := func(armed bool) Board {
		b := removalBoard(1, 1, armed)
		b.Creatures.Set(21, Creature{Power: 6, Toughness: 6, Controller: 1,
			Keywords: []string{"Hexproof"}})
		return b
	}
	off, _ := castDecision(build(false), removalOptions())
	if off != 0 {
		t.Fatalf("rule off: chose option %d, want the removal cast (0)", off)
	}
	on, _ := castDecision(build(true), removalOptions())
	if on != 1 {
		t.Fatalf("rule on: chose option %d, want the pass (1): the hexproof 6/6 is no legal target", on)
	}
}

// TestRemovalNowIndestructibleSurvivesDestroy: an indestructible 6/6 is no
// candidate for a destroy (destruction does nothing to it, CR 702.12), so
// the armed rule holds beside the targetable 1/1 -- and the same board with
// a bounce shape fires, because indestructible does not stop a bounce.
func TestRemovalNowIndestructibleSurvivesDestroy(t *testing.T) {
	b := removalBoard(1, 1, true)
	b.Creatures.Set(21, Creature{Power: 6, Toughness: 6, Controller: 1,
		Keywords: []string{"Indestructible"}})
	if got, _ := castDecision(b, removalOptions()); got != 1 {
		t.Fatalf("rule on, indestructible 6/6 vs destroy: chose option %d, want the pass (1)", got)
	}
	b.Cards.Set(10, Card{CMC: 2, ManaCost: "2", Castable: true,
		RemovalKind: "bounce", RemovalTargets: "creature"})
	if got, _ := castDecision(b, removalOptions()); got != 0 {
		t.Fatalf("rule on, indestructible 6/6 vs bounce: chose option %d, want the bounce cast (0)", got)
	}
}

// TestRemovalNowHeldSpellGetsNoTap is the tap gate's half of the A1 rule:
// the held 3-mana destroy (empty pool, one untapped land, rule on) is not a
// tap need, so the priority decision passes instead of floating mana toward
// a cast chooseCast will refuse. With the rule off the same window taps.
func TestRemovalNowHeldSpellGetsNoTap(t *testing.T) {
	opts := []decision.Option{
		{Index: 0, Kind: "activate", Obj: 30},
		{Index: 1, Kind: "pass"},
	}
	build := func(armed bool) Board {
		b := removalBoard(1, 1, armed) // the 1/1 is below the bar, so the destroy is held
		b.IsMain = true
		b.Cards.Set(10, Card{CMC: 3, ManaCost: "3", Castable: true,
			RemovalKind: "destroy", RemovalTargets: "creature"})
		prod := cards.ManaProduction{}
		prod.Colour[state.MW] = 1
		b.Cards.Set(30, Card{OnBattlefield: true, Produces: prod}) // an untapped Plains
		return b
	}
	off, d := castDecision(build(false), opts)
	if d.Options[off].Kind != "activate" {
		t.Fatalf("rule off: chose %q, want the land activation (0) for the unpayable destroy", d.Options[off].Kind)
	}
	on, d := castDecision(build(true), opts)
	if d.Options[on].Kind != "pass" {
		t.Fatalf("rule on: chose %q, want the pass (1): the destroy is held, its mana must not float", d.Options[on].Kind)
	}
}

// TestRemovalNowCastEntriesSkipHeld is the C11 half of the A1 rule: a tuned
// cast profile (SetValue non-zero) builds castEntries, and the held card
// must be absent from that table -- otherwise setEfficiency counts a cast
// the policy would refuse as follow-up value. With the rule off the card is
// present.
func TestRemovalNowCastEntriesSkipHeld(t *testing.T) {
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: removalOptions()}
	off := removalBoard(3, 3, false)
	off.Cast = CastWeights{SetValue: 1}
	entries := off.castEntries(d, false)
	if len(entries) != 1 || entries[0].obj != 10 {
		t.Fatalf("rule off: castEntries %+v, want exactly the destroy (obj 10)", entries)
	}
	on := removalBoard(3, 3, true)
	on.Cast = CastWeights{SetValue: 1}
	entries = on.castEntries(d, false)
	if len(entries) != 0 {
		t.Fatalf("rule on: castEntries %+v, want the held destroy absent", entries)
	}
}
