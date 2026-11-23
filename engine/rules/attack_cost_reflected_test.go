package rules

// A reflected-mana source (Fellwar Stone: "add one mana of any color that a
// land an opponent controls could produce") with two or more colours on offer
// must tap for mana in the attack-cost and block-cost payment windows. The
// window resolves the tap inline with no suspended resolution, so an unpinned
// colour ask (askManaColor) is displaced by the window's next tap ask and the
// tap adds nothing. The membership walk pins the first producible colour
// (CR 605.3a, 106.7, 508.1h), as it does for choice-shaped sources.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestAttackTaxPaidWithFellwarStoneTwoColours(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, bear := attackPropSeat(t, "Ghostly Prison", 0)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Mountain"))
	onBoardCard(t, e, 2, mustCorpusCard(t, reg, "Forest"))
	var stones []state.ObjID
	for i := 0; i < 2; i++ {
		stones = append(stones, onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Fellwar Stone")))
	}
	if got := e.attackBudget(1); got != 2 {
		t.Fatalf("precondition: attackBudget = %d, want 2", got)
	}

	e.askAttackers()
	d := e.Pending()
	opt := findAttackOption(d, bear, 0)
	if opt == nil {
		t.Fatalf("pair not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatalf("submit charged attack: %v", err)
	}
	for round := 0; round < 2; round++ {
		w := e.Pending()
		if w == nil || w.Kind != decision.KChoose || len(w.Options) == 0 || w.Options[0].Kind != "attack_mana" {
			t.Fatalf("round %d: expected the attack payment window, got %+v", round, w)
		}
		if err := e.Submit(decision.Intent{Seq: w.Seq, Player: w.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("tap Fellwar Stone: %v", err)
		}
	}
	for _, id := range stones {
		if !e.G.Obj(id).Tapped {
			t.Fatalf("Fellwar Stone %d not tapped", id)
		}
	}
	if !e.G.Obj(bear).IsAttacking {
		t.Fatal("charged attacker never declared: the tax was not paid")
	}
	if got := e.G.Players[1].Pool.Total(); got != 0 {
		t.Fatalf("pool after paying = %d, want 0", got)
	}
	drainCombatPriority(t, e)
}

func TestBlockTaxPaidWithFellwarStoneTwoColours(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := threeSeatEngine(t)
	qal := onBoardCard(t, e, 0, corpusAlternativeCard(t, "Qal Sisma Behemoth"))
	var stones []state.ObjID
	for i := 0; i < 2; i++ {
		stones = append(stones, onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Fellwar Stone")))
	}
	onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Mountain"))
	onBoardCard(t, e, 2, mustCorpusCard(t, reg, "Forest"))
	bear := onBoardReady(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:0/4\nOracle:x\n")
	attackSeat0(t, e, bear)

	d := askBlockersFresh(t, e)
	if d == nil {
		t.Fatal("block pair not offered although two Fellwar Stones can pay {2}")
	}
	opt := findBlockOption(d, qal, bear)
	if opt == nil {
		t.Fatalf("pair not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatalf("submit charged block: %v", err)
	}
	for round := 0; round < 2; round++ {
		w := e.Pending()
		if w == nil || w.Kind != decision.KChoose || len(w.Options) == 0 || w.Options[0].Kind != "block_mana" {
			t.Fatalf("round %d: expected the block payment window, got %+v", round, w)
		}
		if err := e.Submit(decision.Intent{Seq: w.Seq, Player: w.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("tap Fellwar Stone: %v", err)
		}
	}
	for _, id := range stones {
		if !e.G.Obj(id).Tapped {
			t.Fatalf("Fellwar Stone %d not tapped", id)
		}
	}
	blocked := false
	for _, b := range e.G.Obj(bear).BlockedBy {
		if b == qal {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("charged block never committed (BlockedBy %v)", e.G.Obj(bear).BlockedBy)
	}
}
