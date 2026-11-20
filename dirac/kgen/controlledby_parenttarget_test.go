package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// controlledByParentTargetForms: DB$ Tap | ValidTgts$
// Permanent.ControlledBy ParentTarget (Yosei, the Morning Star's dying
// trigger's sub-ability) lowers onto the TargetedPlayerCtrl shape
// (kop.F2TgtPlayerCtrl): gorge's controlReferentPlayers ParentTarget reads
// the parent chain link's chosen PLAYER targets (an object target binds
// nothing, trigger_referents.go), and parentTargetCtrl admits the word only
// while that parent slot is the resolving entry's only player-targeting one,
// so the kernel's entry-wide read is exact. The ParentTargetedController
// spellings keep their unknown-property refusal (no kernel reader carries
// the controller-of-card-target relation on the battlefield).
var controlledByParentTargetForms = []formCase{
	{card: "Yosei, the Morning Star", form: "TrigTap: DB$ Tap Permanent.ControlledBy ParentTarget", lir: "F={types=0x9f ctrl=4}"},
}

func TestControlledByParentTargetForms(t *testing.T) { checkForms(t, controlledByParentTargetForms) }

// TestControlledByParentTargetGate: the rewrite refuses every shape whose
// kernel read (the entry's whole player-target set) would be wider than
// gorge's parent-only read, and lowers the exact one.
func TestControlledByParentTargetGate(t *testing.T) {
	newLC := func() *lowerCtx {
		return &lowerCtx{p: &pool{}, known: func(string) bool { return false }, out: &lCard{}, face: &cards.Face{SVars: map[string]string{}}}
	}
	const expr = "Permanent.ControlledBy ParentTarget"

	// no earlier target slot: nothing to read
	lc := newLC()
	lc.ab = &lAbility{Kind: akTriggered}
	if _, err := lc.filter(expr); err == nil || !strings.Contains(err.Error(), "without an earlier target") {
		t.Errorf("no earlier target: got %v, want a named refusal", err)
	}

	// the parent link's target admits no player: the referent names nobody
	lc = newLC()
	lc.ab = &lAbility{Kind: akTriggered}
	lc.addTarget(lTarget{F: &lFilter{Clauses: []lClause{{Types: tCreature}}}, Zone: zBattlefield})
	if _, err := lc.filter(expr); err == nil || !strings.Contains(err.Error(), "admits no player") {
		t.Errorf("object-only parent: got %v, want a named refusal", err)
	}

	// a second earlier link also targeting a player would widen the kernel's
	// set past gorge's parent-only read
	lc = newLC()
	lc.ab = &lAbility{Kind: akTriggered}
	lc.addTarget(lTarget{F: &lFilter{Players: plBoth}, Zone: zNone})
	lc.addTarget(lTarget{F: &lFilter{Clauses: []lClause{{Types: tCreature}}}, Zone: zBattlefield})
	lc.addTarget(lTarget{F: &lFilter{Players: plBoth}, Zone: zNone})
	if _, err := lc.filter(expr); err == nil || !strings.Contains(err.Error(), "another player-targeting link") {
		t.Errorf("two player-targeting links: got %v, want a named refusal", err)
	}

	// the exact shape: the parent link is the entry's only player-targeting
	// slot, and the word lowers onto whoTgtPlayer
	lc = newLC()
	lc.ab = &lAbility{Kind: akTriggered}
	lc.addTarget(lTarget{F: &lFilter{Players: plBoth}, Zone: zNone})
	f, err := lc.filter(expr)
	if err != nil {
		t.Fatalf("parent player target: %v", err)
	}
	if len(f.Clauses) != 1 || f.Clauses[0].Ctrl != whoTgtPlayer {
		t.Errorf("got %+v, want one clause with ctrl whoTgtPlayer", f.Clauses)
	}

	// the longer spelling is not the word: ParentTargetedController keeps its
	// unknown-property refusal instead of being rewritten mid-token
	lc = newLC()
	lc.ab = &lAbility{Kind: akTriggered}
	lc.addTarget(lTarget{F: &lFilter{Players: plBoth}, Zone: zNone})
	if _, err := lc.filter("Creature.ControlledBy ParentTargetedController"); err == nil ||
		!strings.Contains(err.Error(), `unknown property "ControlledBy ParentTargetedController"`) {
		t.Errorf("ParentTargetedController: got %v, want the unknown-property refusal", err)
	}
}
