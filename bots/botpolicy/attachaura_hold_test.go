package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// auraHoldBoard is a castable one-mana boon Aura (id 10) in hand, with the
// seat's creatures as given. armed turns on HoldOwnAura.
func auraHoldBoard(armed bool, fs ...fact) Board {
	b := priorityCards(map[state.ObjID]Card{
		10: {CMC: 1, ManaCost: "G", Castable: true, AuraLogic: "Pump"},
	})
	b.Creatures = TableOf(map[state.ObjID]Creature{})
	for _, f := range fs {
		b.Creatures.Set(f.id, f.c)
	}
	b.Life = TableOf(map[state.PlayerID]int32{0: 20, 1: 20})
	b.Attach = AttachRules{HoldOwnAura: armed}
	return b
}

func auraHoldCastOptions() []decision.Option {
	return []decision.Option{
		{Index: 0, Kind: "cast", Obj: 10},
		{Index: 1, Kind: "pass"},
	}
}

func TestAuraHoldWithNoUsableCarrier(t *testing.T) {
	wall := atk(1, 0, 4, "Defender")
	off, _ := castDecision(auraHoldBoard(false, wall), auraHoldCastOptions())
	if off != 0 {
		t.Fatalf("rule off: chose option %d, want the cast (0)", off)
	}
	on, _ := castDecision(auraHoldBoard(true, wall), auraHoldCastOptions())
	if on != 1 {
		t.Fatalf("rule on, only a Defender: chose option %d, want the pass (1)", on)
	}
	on, _ = castDecision(auraHoldBoard(true, wall, atk(2, 2, 2)), auraHoldCastOptions())
	if on != 0 {
		t.Fatalf("rule on with an attacker: chose option %d, want the cast (0)", on)
	}
	// A foreign creature is not a carrier.
	on, _ = castDecision(auraHoldBoard(true, def(1, 2, 2)), auraHoldCastOptions())
	if on != 1 {
		t.Fatalf("rule on, only an opposing creature: chose option %d, want the pass (1)", on)
	}
}

func TestAuraHoldGetsNoTap(t *testing.T) {
	opts := []decision.Option{
		{Index: 0, Kind: "activate", Obj: 30},
		{Index: 1, Kind: "pass"},
	}
	build := func(armed bool) Board {
		b := auraHoldBoard(armed, atk(1, 0, 4, "Defender"))
		prod := cards.ManaProduction{}
		prod.Colour[state.MG] = 1
		b.Cards.Set(30, Card{OnBattlefield: true, Produces: prod}) // an untapped Forest
		return b
	}
	off, d := castDecision(build(false), opts)
	if d.Options[off].Kind != "activate" {
		t.Fatalf("rule off: chose %q, want the Forest activation toward the Aura", d.Options[off].Kind)
	}
	on, d := castDecision(build(true), opts)
	if d.Options[on].Kind != "pass" {
		t.Fatalf("rule on: chose %q, want the pass: the held Aura's mana must not float", d.Options[on].Kind)
	}
}

func TestAuraHoldCastEntriesSkipHeld(t *testing.T) {
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: auraHoldCastOptions()}
	off := auraHoldBoard(false, atk(1, 0, 4, "Defender"))
	off.Cast = CastWeights{SetValue: 1}
	if es := off.castEntries(d, false); len(es) != 1 || es[0].obj != 10 {
		t.Fatalf("rule off: castEntries %+v, want exactly the Aura (obj 10)", es)
	}
	on := auraHoldBoard(true, atk(1, 0, 4, "Defender"))
	on.Cast = CastWeights{SetValue: 1}
	if es := on.castEntries(d, false); len(es) != 0 {
		t.Fatalf("rule on: castEntries %+v, want the held Aura absent", es)
	}
}

// TestAuraHoldIgnoresUnknownLogic: only a boon hint is held; a Curse Aura or
// an unhinted card is never held by this rule.
func TestAuraHoldIgnoresUnknownLogic(t *testing.T) {
	for _, logic := range []string{"", "Curse", "Reanimate"} {
		b := auraHoldBoard(true, atk(1, 0, 4, "Defender"))
		b.Cards.Set(10, Card{CMC: 1, ManaCost: "G", Castable: true, AuraLogic: logic})
		if got, _ := castDecision(b, auraHoldCastOptions()); got != 0 {
			t.Errorf("AuraLogic %q: chose option %d, want the cast (0)", logic, got)
		}
	}
}
