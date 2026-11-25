package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestFlashbackManaPlusLifeSpendsTheMana: CR 601.2f-h, 702.34a -- a
// flashback cost with a mana part and a life part (Deep Analysis's
// "Flashback--{1}{U}, Pay 3 life") is one total cost: the mana floated for
// it is spent and the life is paid. gorge's cast planner admits no life part
// (pay.PlanNonManaAdmissible "cost:life"), so the cast is made from floated
// mana; after it the pool is empty and the caster paid 3 life (the dirac
// lockstep of the cycle: dirac/oracle/flashlife_lowers_test.go).
func TestFlashbackManaPlusLifeSpendsTheMana(t *testing.T) {
	t.Parallel()
	src := "Name:Fb Analysis\nManaCost:3 R\nTypes:Sorcery\nK:Flashback:1 R PayLife<3>\n" +
		"A:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e, cfg, fb := newFixtureDeck(t, 4807, src)
	toMain1(t, e)
	e.emit(events.Event{Kind: events.MoveZone, Obj: fb, From: e.G.Obj(fb).Zone, To: state.ZGraveyard})
	e.pending = nil
	addMana(t, e, 0, "RR")
	life := e.G.Players[0].Life
	var opt *decision.Option
	for _, o := range castOptions(t, e) {
		if o.Mode == "flashback" && o.Obj == fb {
			opt = &o
		}
	}
	if opt == nil {
		t.Fatal("CR 702.34a: flashback not offered with {R}{R} floating")
	}
	submitChoices(t, e, opt.Index)
	if z := e.G.Obj(fb).Zone; z != state.ZStack {
		t.Fatalf("seq %d: Fb Analysis in %s, want the stack", len(e.L.Events), z)
	}
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Errorf("CR 601.2h seq %d: seat 0 pool %v after the cast, want empty (the {R}{R} pays {1}{R})", len(e.L.Events), pool)
	}
	if got := e.G.Players[0].Life; got != life-3 {
		t.Errorf("CR 601.2h seq %d: seat 0 life %d, want %d (Pay 3 life)", len(e.L.Events), got, life-3)
	}
	replayCheck(t, e, cfg)
}
