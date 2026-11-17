package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// ctrAllScen is a scenario table with one synthetic CounterAddedAll watcher
// (matches every counter event) appended to vanilla's cards.
func ctrAllScen(t *testing.T) (*scen, kop.CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append(append([]kop.CardDef{}, v.Table.Cards...), kop.CardDef{
		Name: "Probe Counter Watcher", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{
			Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigCounterAddedAll, N: kop.TrigNAnyCounter},
		}},
	})
	tbl.Exprs = []kop.Expr{} // force prepareGen: the watcher runs in its hook
	return newScenTable(t, &tbl, Config{}), kop.CardID(len(tbl.Cards) - 1)
}

// TestCounterAddedAllMultiplyPerRecipient: a MultiplyCounter sweep is NOT one
// counter-adding action in gorge (effMultiplyCounter never opens
// beginActionBatch), so two recipients each queue their own CounterAddedAll
// trigger. The old batch marker merged the sweep into one trigger.
func TestCounterAddedAllMultiplyPerRecipient(t *testing.T) {
	sc, watch := ctrAllScen(t)
	s, k := sc.s, sc.k
	sc.perm(watch, 0)
	b1, b2 := sc.perm(v.GrizzlyBears, 0), sc.perm(v.GrizzlyBears, 0)
	// Seed one +1/+1 counter on each recipient, then start a clean queue.
	s.bolsterPut(k, s.objs[b1].slot, int(kop.CtrP1P1), 1)
	s.bolsterPut(k, s.objs[b2].slot, int(kop.CtrP1P1), 1)
	s.npend = 0

	s.doubleCountersAll(k, &kop.Instr{F: 0, C: int16(kop.CtrP1P1)})

	if s.npend != 2 {
		t.Fatalf("multiply sweep queued %d CounterAddedAll triggers, want 2 (one per recipient)", s.npend)
	}
	for i := 0; i < 2; i++ {
		if s.pend[i].trigAmt != 1 {
			t.Errorf("trigger %d amount = %d, want 1", i, s.pend[i].trigAmt)
		}
	}
}

// TestCounterAddedAllSupportPicksOneAction: gorge's putCounterSupport runs
// inside effPutCounter's action bracket, so a two-pick Support$ 2 is ONE
// CounterAddedAll trigger whose amount is both picks. OpSupport must open the
// marker for the picks (answerSupport applies them across resumptions), and
// answerSupport's finish path must close it after the last pick so later
// actions are separate. The picks go through answerSupport, not a manual
// close, so deleting the production close leaves the marker behind and fails
// the assertions below.
func TestCounterAddedAllSupportPicksOneAction(t *testing.T) {
	sc, watch := ctrAllScen(t)
	s, k := sc.s, sc.k
	sc.perm(watch, 0)
	src := sc.perm(v.GrizzlyBears, 0)
	b1, b2 := sc.perm(v.GrizzlyBears, 0), sc.perm(v.GrizzlyBears, 0)
	sup := kop.Instr{Op: kop.OpSupport, A: kop.Const(2), C: int16(kop.CtrP1P1) | 0x100}
	// answerSupport reads the resolving program at res.pc; give the probe's
	// ability the support program and push its resolving entry (the m11Start
	// shape) so the finish path can complete.
	k.t.Cards[watch].Abilities[0].Prog = []kop.Instr{sup}
	e := stackEntry{kind: seTrigger, obj: src, srcVer: s.objs[src].ver, card: watch, ctrl: 0, trigObj: NoObj}
	s.sp = 1
	s.stack[0] = e
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}

	if s.support(k, &sup) {
		t.Fatal("eligible creatures: support must ask")
	}
	if s.g.ctrCellHit>>1 == 0 {
		t.Fatal("support did not open the CounterAddedAll batch marker")
	}
	// answerSupport applies each pick as it lands, across resumptions, and
	// closes the marker itself on the last one.
	s.answerSupport(k, MakeAction(AkChoose, TCode(b1), 0, 0))
	if s.g.ctrCellHit>>1 == 0 {
		t.Fatal("the first pick closed the batch before the second pick")
	}
	if s.npend != 1 || s.pend[0].trigAmt != 1 {
		t.Fatalf("after the first pick: %d triggers, amount %d, want 1/1", s.npend, s.pend[0].trigAmt)
	}
	s.answerSupport(k, MakeAction(AkChoose, TCode(b2), 0, 0))
	if s.g.ctrCellHit>>1 != 0 {
		t.Fatal("answerSupport did not close the CounterAddedAll batch marker")
	}
	if s.npend != 1 {
		t.Fatalf("two-pick support queued %d CounterAddedAll triggers, want 1", s.npend)
	}
	if s.pend[0].trigAmt != 2 {
		t.Fatalf("support trigger amount = %d, want 2 (both picks)", s.pend[0].trigAmt)
	}

	// A later placement outside the ask is its own action: the marker is gone.
	s.npend = 0
	s.bolsterPut(k, s.objs[b1].slot, int(kop.CtrP1P1), 1)
	if s.npend != 1 {
		t.Fatalf("a post-support placement queued %d triggers, want 1", s.npend)
	}
}
