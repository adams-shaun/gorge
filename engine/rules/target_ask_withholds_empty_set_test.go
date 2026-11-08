package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestTargetAskWithholdsUnpayableEmptyTargetSet pins the target-ask half of the
// multi-target cost-reduction fix. This Town Ain't Big Enough is "up to two
// targets" (TargetMin$ 0) and costs {3} less if it targets a permanent you
// control (CR 601.2f prices the spell for the targets it will have). With only
// {U}{U} floating the printed {4}{U} is unpayable, so the empty (zero-target)
// answer would strand the cast; the ask must raise its minimum to one, exactly
// as an unaffordable target candidate is withheld (CR 601.2h, 733.1).
func TestTargetAskWithholdsUnpayableEmptyTargetSet(t *testing.T) {
	t.Parallel()
	relic := "Name:TTABE Relic\nTypes:Artifact\nOracle:x\n"
	bear := "Name:TTABE Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, cfg, spell := newFixtureDeckWithOpponentCard(t, 41, corpusCardText(t, "t/this_town_aint_big_enough.txt"), relic, bear)
	own := putCreature(t, e, 0, relic)
	opp := putCreature(t, e, 1, bear)
	addMana(t, e, 0, "UU")
	e.priorityRound()
	if e.G.Obj(own).Controller != 0 || e.G.Obj(opp).Controller != 1 {
		t.Fatal("precondition: one own and one opponent nonland permanent")
	}
	if pool := e.G.Players[0].Pool; pool.Total() != 2 {
		t.Fatalf("precondition: exactly two mana floating, pool %+v", pool)
	}
	opt := castByName(t, e, 0, "This Town Ain't Big Enough")
	if opt == nil {
		t.Fatal("precondition: the own-permanent {1}{U} price must make the cast offerable from {U}{U}")
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Source != spell {
		t.Fatalf("target decision = %+v, want the spell's KTarget ask", d)
	}
	if len(d.Options) < 1 {
		t.Fatalf("precondition: the ask must offer candidates: %+v", d.Options)
	}
	if d.Max != 2 {
		t.Fatalf("precondition: ask is up to two targets, got Max=%d", d.Max)
	}
	if d.Min != 1 {
		t.Fatalf("ask Min = %d, want 1: the empty set leaves {4}{U} unpayable from {U}{U}", d.Min)
	}
	replayCheck(t, e, cfg)
}
