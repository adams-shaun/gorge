package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
)

// mustBeBlockedByBladeSrc is an inline Equipment granting the filtered
// "MustBeBlockedBy Creature.Wall" requirement to its attacking bearer.
const mustBeBlockedByBladeSrc = "Name:Test Wall Lure Blade\nManaCost:1\nTypes:Artifact Equipment\nK:Equip:1\n" +
	"S:Mode$ Continuous | AffectedDefined$ Equipped | Affected$ Creature.attacking | AddKeyword$ MustBeBlockedBy Creature.Wall | Description$ x\nOracle:x\n"

// TestMustBeBlockedByCreatureIsMustBlock: the unfiltered grant decodes to the
// plain must-be-blocked flag, and a filter keeps its text verbatim.
func TestMustBeBlockedByCreatureIsMustBlock(t *testing.T) {
	t.Parallel()
	f := combat.ParseHiddenKeyword("MustBeBlockedBy Creature")
	if !f.MustBlock || f.MustBlockSpec != "" || f.AllMustBlock || f.MustBeBlockedByOne {
		t.Fatalf("MustBeBlockedBy Creature = %+v, want MustBlock with no spec", f)
	}
	g := combat.ParseHiddenKeyword("MustBeBlockedBy Creature.Dalek")
	if !g.MustBlock || g.MustBlockSpec != "Creature.Dalek" {
		t.Fatalf("MustBeBlockedBy Creature.Dalek = %+v, want MustBlock with spec Creature.Dalek", g)
	}
	// The siblings keep their own meaning.
	if h := combat.ParseHiddenKeyword("MustBeBlockedByOne"); h.MustBlock || !h.MustBeBlockedByOne {
		t.Fatalf("MustBeBlockedByOne = %+v", h)
	}
	if h := combat.ParseHiddenKeyword("MustBeBlockedByAll:Creature.Wall:x"); h.MustBlock || !h.AllMustBlock {
		t.Fatalf("MustBeBlockedByAll = %+v", h)
	}
}

// TestMasamuneAttackerMustBeBlocked: the real The Masamune makes its attacking
// bearer must-be-blocked; unattached, the empty declaration is legal.
func TestMasamuneAttackerMustBeBlocked(t *testing.T) {
	t.Parallel()
	run := func(attached bool) (*Engine, *decision.Decision, state.ObjID, state.ObjID) {
		e := threeSeatEngine(t)
		sword := onBoardCard(t, e, 1, mustBeBlockedCorpusCard(t, "The Masamune"))
		atk := onBoard(t, e, 1, statBearSrc)
		blk := onBoard(t, e, 0, statBearSrc)
		if attached {
			e.G.Obj(sword).AttachedTo = atk
		}
		attackSeat0(t, e, atk)
		d := askBlockersFresh(t, e)
		if d == nil {
			t.Fatal("no declare-blockers decision")
		}
		return e, d, atk, blk
	}
	e, d, atk, blk := run(true)
	if !combat.DerivedHiddenFlags(asBoard(e), atk).MustBlock {
		t.Fatal("precondition: the equipped attacker does not derive MustBlock")
	}
	opt := findBlockOption(d, blk, atk)
	if opt == nil {
		t.Fatal("precondition: the pair is not offered")
	}
	if !opt.AttackMust {
		t.Fatalf("the equipped attacker's pair carries no AttackMust: %+v", opt)
	}
	if err := e.validateBlockers(d, decision.Intent{Seq: d.Seq, Player: d.Player}); err == nil {
		t.Fatal("the empty declaration was accepted while a legal block exists")
	}
	if err := e.validateBlockers(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatalf("the block was rejected: %v", err)
	}

	e, d, atk, blk = run(false)
	if combat.DerivedHiddenFlags(asBoard(e), atk).MustBlock {
		t.Fatal("precondition: the unequipped attacker derives MustBlock")
	}
	if opt := findBlockOption(d, blk, atk); opt == nil || opt.AttackMust {
		t.Fatalf("unattached pair must be offered unbound: %+v", opt)
	}
	if err := e.validateBlockers(d, decision.Intent{Seq: d.Seq, Player: d.Player}); err != nil {
		t.Fatalf("the empty declaration was rejected without the Equipment: %v", err)
	}
}

// TestMustBeBlockedByFilterGrantBindsMatchingBlocker: a filtered grant is
// obeyed only by a blocker matching the filter.
func TestMustBeBlockedByFilterGrantBindsMatchingBlocker(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	blade := onBoardCard(t, e, 1, card(t, mustBeBlockedByBladeSrc))
	atk := onBoard(t, e, 1, statBearSrc)
	wall := onBoard(t, e, 0, wallSrc)
	bear := onBoard(t, e, 0, statBearSrc)
	e.G.Obj(blade).AttachedTo = atk
	attackSeat0(t, e, atk)
	d := askBlockersFresh(t, e)
	if d == nil {
		t.Fatal("no declare-blockers decision")
	}
	fl := combat.DerivedHiddenFlags(asBoard(e), atk)
	if !fl.MustBlock || fl.MustBlockSpec != "Creature.Wall" {
		t.Fatalf("precondition: derived flags %+v, want MustBlock with spec Creature.Wall", fl)
	}
	wo, bo := findBlockOption(d, wall, atk), findBlockOption(d, bear, atk)
	if wo == nil || bo == nil {
		t.Fatalf("precondition: both pairs must be offered: wall=%v bear=%v", wo, bo)
	}
	if !wo.AttackMust {
		t.Fatalf("the Wall's pair carries no AttackMust: %+v", wo)
	}
	if bo.AttackMust {
		t.Fatalf("the non-Wall's pair carries AttackMust: %+v", bo)
	}
	check := func(want bool, choices ...int) {
		t.Helper()
		err := e.validateBlockers(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices})
		if (err == nil) != want {
			t.Fatalf("choices %v: accepted=%v, want %v (%v)", choices, err == nil, want, err)
		}
	}
	check(false)
	check(false, bo.Index)
	check(true, wo.Index)
}
