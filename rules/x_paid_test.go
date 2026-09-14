package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// TestActivatedAbilityWithXInCostResolvesItsPaidX is the activated-ability
// sibling of the spell-side CR 107.3i binding: an ability whose Cost$
// carries {X} is asked (xAsk runs on the shared cast flow for abilities
// too), the payment charges WithX, and commitCast records the paid value on
// the minted ability stack object with a CastInfo event so resolveTop's
// ability branch can bind it into effects.Ctx.X. CounterNum$ X then answers
// the chosen value, not 0 -- before the CastInfo emission the AbilityPush's
// Amount (the ability index) was the only thing on the wire and o.X stayed
// 0 through resolution.
func TestActivatedAbilityWithXInCostResolvesItsPaidX(t *testing.T) {
	src := "Name:Ballista\nManaCost:X X\nTypes:Artifact Creature Construct\nPT:2/2\n" +
		"A:AB$ PutCounter | Cost$ X G | CounterType$ P1P1 | CounterNum$ X | Defined$ Self | SpellDescription$ Put X +1/+1 counters on CARDNAME.\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 113, src)
	id = putCreature(t, e, 0, src)
	addMana(t, e, 0, "GGG") // X = 2: {2}{G}
	e.Advance()
	opt := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "x" {
		t.Fatalf("X decision %+v", d)
	}
	submitChoices(t, e, 2)
	if len(e.G.Stack) != 1 {
		t.Fatalf("ability stack objects = %v", e.G.Stack)
	}
	abilityObj := e.G.Obj(e.G.Stack[0])
	if abilityObj.X != 2 {
		t.Fatalf("ability stack object X = %d, want 2 (CastInfo did not record the paid X)", abilityObj.X)
	}
	passUntilStackEmpty(t, e, 20)
	p1p1 := int32(0)
	for _, c := range e.G.Obj(id).Counters {
		if c.Kind == "P1P1" {
			p1p1 += c.N
		}
	}
	if p1p1 != 2 {
		t.Fatalf("+1/+1 counters on Ballista = %d, want 2", p1p1)
	}
	replayCheck(t, e, cfg)
}
