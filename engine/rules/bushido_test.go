package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestBushidoPumpsOnBlocksAndBecomesBlocked pins CR 702.45a on the two real
// ticket cards: a Bushido N creature that blocks (Samurai of the Pale Curtain,
// bushido 1) or becomes blocked (Jade Avenger, bushido 2) queues one trigger,
// and its resolution pumps the creature +N/+N through the ordinary Pump path.
func TestBushidoPumpsOnBlocksAndBecomesBlocked(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	jadeCard, ok := reg.Lookup("Jade Avenger")
	if !ok {
		t.Fatal("Jade Avenger not in the corpus")
	}
	samuraiCard, ok := reg.Lookup("Samurai of the Pale Curtain")
	if !ok {
		t.Fatal("Samurai of the Pale Curtain not in the corpus")
	}
	for _, w := range []struct {
		name  string
		card  *cards.Card
		param string
	}{{"Jade Avenger", jadeCard, "2"}, {"Samurai of the Pale Curtain", samuraiCard, "1"}} {
		if got, ok := w.card.Faces[0].KeywordParam("Bushido"); !ok || got != w.param {
			t.Fatalf("%s keyword param = %q (%v), want Bushido %s", w.name, got, ok, w.param)
		}
	}
	const bear = "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

	// Blocks half: seat 1 attacks, the Samurai on seat 0 blocks.
	e := blocksCombatEngine(t, 1)
	samurai := onBoardCard(t, e, 0, samuraiCard)
	atk := onBoard(t, e, 1, bear)
	e.G.Obj(atk).SummonSick = false
	e.askAttackers()
	submitAttackersOnly(t, e, atk)
	drainCombatPriority(t, e)
	if d := e.Pending(); d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	submitBlockersOnly(t, e, samurai)
	if len(e.G.Stack) != 1 {
		t.Fatalf("blocks half: stack = %d, want one Bushido trigger", len(e.G.Stack))
	}
	if e.Power(samurai) != 2 || e.Toughness(samurai) != 2 {
		t.Fatalf("blocks half before resolution = %d/%d, want 2/2", e.Power(samurai), e.Toughness(samurai))
	}
	e.resolveTop()
	if e.Power(samurai) != 3 || e.Toughness(samurai) != 3 {
		t.Fatalf("blocks half after resolution = %d/%d, want 3/3", e.Power(samurai), e.Toughness(samurai))
	}

	// Becomes-blocked half: the Jade Avenger attacks, a bear blocks.
	e2 := combatEngine(t)
	jade := onBoardCard(t, e2, 0, jadeCard)
	e2.G.Obj(jade).SummonSick = false
	blk := onBoard(t, e2, 1, bear)
	e2.askAttackers()
	submitAttackersOnly(t, e2, jade)
	drainCombatPriority(t, e2)
	if d := e2.Pending(); d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	submitBlockersOnly(t, e2, blk)
	if len(e2.G.Stack) != 1 {
		t.Fatalf("becomes-blocked half: stack = %d, want one Bushido trigger", len(e2.G.Stack))
	}
	if e2.Power(jade) != 2 || e2.Toughness(jade) != 2 {
		t.Fatalf("becomes-blocked half before resolution = %d/%d, want 2/2", e2.Power(jade), e2.Toughness(jade))
	}
	e2.resolveTop()
	if e2.Power(jade) != 4 || e2.Toughness(jade) != 4 {
		t.Fatalf("becomes-blocked half after resolution = %d/%d, want 4/4", e2.Power(jade), e2.Toughness(jade))
	}
}
