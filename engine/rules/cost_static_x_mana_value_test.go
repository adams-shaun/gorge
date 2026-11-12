package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A ReduceCost static that tests the spell's mana value reads it with the
// announced {X} in it (CR 202.3e, 601.2b, 601.2f): the X ask prices each X
// through the cost that X gives. A sorcery {X}{R}{R} beside "sorceries of mana
// value 4 or more cost {2} less" with four mana floating is payable at X = 4
// (mana value 6, cost {R}{R} and two more), two past what the unreduced cost
// allows, and the cast charges four.
func TestCostStaticReadsAnnouncedXInManaValue(t *testing.T) {
	t.Parallel()
	burst := card(t, "Name:Test Burst\nManaCost:X R R\nTypes:Sorcery\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ X\nSVar:X:Count$xPaid\nOracle:x\n")
	e := handEngine(t, burst)
	battlefieldFixture(t, e, 0, "Name:Test Warden\nManaCost:1\nTypes:Creature Human\nPT:1/1\nS:Mode$ ReduceCost | ValidCard$ Sorcery.cmcGE4 | Type$ Spell | Activator$ You | Amount$ 2\nOracle:x\n")
	bear := battlefieldFixture(t, e, 1, costredBear)
	e.G.Players[0].Pool[state.MR] = 2
	e.G.Players[0].Pool[state.MG] = 2
	spell := e.G.Zone(state.ZHand, 0)[0]
	if !hasCastOption(e.legalActions(0), spell) {
		t.Fatal("Test Burst not offered")
	}
	castMode(t, e, spell, "")
	var d *decision.Decision
	for i := 0; i < 8; i++ {
		if d = e.Pending(); d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "x" {
			break
		}
		submitChoices(t, e, 0)
	}
	if d == nil || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("no X ask; pending %+v", d)
	}
	if got, want := len(d.Options), 5; got != want {
		t.Fatalf("X ask offers %d values, want %d (0..4)", got, want)
	}
	submitChoices(t, e, 4)
	for i := 0; i < 8; i++ {
		d = e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		pick := 0
		for _, opt := range d.Options {
			if opt.Obj == bear {
				pick = opt.Index
			}
		}
		submitChoices(t, e, pick)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Fatalf("Test Burst in %s after cast, want stack; pool %v", z, e.G.Players[0].Pool)
	}
	if left := e.G.Players[0].Pool.Total(); left != 0 {
		t.Fatalf("pool after cast = %d, want 0 (charged four)", left)
	}
}
