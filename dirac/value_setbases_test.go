package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

func setRefOp(r kop.Ref) kop.SetOp { return setOp1(kop.SetRefBase, uint16(r)) }

// TestSetRefBaseEval: SetRefBase names the one object or player its Ref
// reads in the resolving entry (refCode), nothing when the Ref names nobody,
// and refuses a Ref SetRefBaseOK does not admit.
func TestSetRefBaseEval(t *testing.T) {
	sc := newSetScen(t, 2, 0)
	s := sc.s
	L, S, P, me, op := TCode(sc.lions), TCode(sc.self), TCode(sc.plains), PlayerCode(0), PlayerCode(1)
	if sc.self == sc.lions || sc.lions == sc.plains || sc.opp != 1 || s.perms[s.objs[sc.self].slot].attached != NoSlot {
		t.Fatal("precondition: three distinct objects, an unattached source and an opponent at seat 1")
	}
	ref := func(r kop.Ref) []TCode { return sc.eval(setRefOp(r)) }
	setWant(t, "RefSelf", ref(kop.RefSelf), S)
	setWant(t, "RefController", ref(kop.RefController), me)
	setWant(t, "RefTriggerObj", ref(kop.RefTriggerObj), P)
	setWant(t, "RefTriggerPlayer", ref(kop.RefTriggerPlayer), op)
	setWant(t, "RefTriggerObjOwner", ref(kop.RefTriggerObjOwner), op)
	setWant(t, "RefTriggerTgtCtrl", ref(kop.RefTriggerTgtCtrl), op)
	setWant(t, "RefDefendingPlayer", ref(kop.RefDefendingPlayer), op)

	// referents that name nobody push nothing
	setWant(t, "RefAttached unattached", ref(kop.RefAttached))
	setWant(t, "RefAttachedCtrl unattached", ref(kop.RefAttachedCtrl))
	setWant(t, "RefEnchantedCtrl unattached", ref(kop.RefEnchantedCtrl))
	s.res.chosen = objWord(NoObj)
	setWant(t, "RefChosen with no choice", ref(kop.RefChosen))
	setWant(t, "RefResChosenPlayer with no choice", ref(kop.RefResChosenPlayer))
	// a hand table keeps no trigger-source record: the subject read names nobody
	if c := s.refCode(sc.k, kop.RefTriggerSource); c != NoTarget {
		t.Fatalf("precondition: refCode(RefTriggerSource) = %d on a hand table, want NoTarget", c)
	}
	setWant(t, "RefTriggerSource as refCode reads it", ref(kop.RefTriggerSource))
	obj := s.res.e.trigObj
	s.res.e.trigObj = NoObj
	setWant(t, "RefTriggerObj with no trigger object", ref(kop.RefTriggerObj))
	setWant(t, "RefTriggerObjOwner with no trigger object", ref(kop.RefTriggerObjOwner))
	s.res.e.trigObj = obj
	// the triggering permanent left and came back: a new object (CR 400.7)
	s.res.flags |= rfTrigGone
	setWant(t, "RefTriggerObj once gone", ref(kop.RefTriggerObj))
	s.res.flags &^= rfTrigGone

	// attached to the opponent's creature, which the controller now controls
	// but does not own: the object, its controller and its owner differ
	s.perms[s.objs[sc.self].slot].attached = s.objs[sc.lions].slot
	setWant(t, "RefAttached", ref(kop.RefAttached), L)
	setWant(t, "RefAttachedCtrl", ref(kop.RefAttachedCtrl), op)
	setWant(t, "RefEnchantedCtrl", ref(kop.RefEnchantedCtrl), op)
	s.perms[s.objs[sc.lions].slot].ctrl = 0
	setWant(t, "RefEnchantedCtrl of a stolen bearer", ref(kop.RefEnchantedCtrl), me)
	setWant(t, "OwnerOf RefAttached", sc.eval(setRefOp(kop.RefAttached), setOp1(kop.SetOwnerOf, 0)), op)
	setWant(t, "Non RefEnchantedCtrl", sc.eval(setRefOp(kop.RefEnchantedCtrl), setOp1(kop.SetPlayersComplement, 0)), op)

	s.res.chosen = objWord(sc.lions)
	setWant(t, "RefChosen", ref(kop.RefChosen), L)
	s.res.chPl = sc.opp + 1
	setWant(t, "RefResChosenPlayer", ref(kop.RefResChosenPlayer), op)

	for _, r := range []kop.Ref{kop.RefNone, kop.RefTarget0, kop.RefRemembered, kop.RefEach, kop.RefEachPlayer,
		kop.RefTriggerAttackers, kop.RefTargets01, kop.RefSacrificed, kop.RefDiscarded, kop.RefHost} {
		if kop.SetRefBaseOK(r) {
			t.Errorf("SetRefBaseOK(%d) = true, want false", r)
		}
		setPanics(t, "SetRefBase over a refused Ref", "not one admitted referent", func() { ref(r) })
	}
	// an argument past a Ref's byte must not truncate onto an admitted one
	setPanics(t, "SetRefBase past a byte", "not one admitted referent", func() {
		sc.eval(setOp1(kop.SetRefBase, 0x100|uint16(kop.RefSelf)))
	})
	for _, e := range []kop.SetExpr{{setRefOp(kop.RefSelf)}, {{Kind: kop.SetBattlefield}, setOp1(kop.SetAndFilter, uint16(setFCreature))}} {
		if d, ok := e.SetDepth(); !ok || d != 1 {
			t.Errorf("SetDepth(%v) = %d, %v; want 1, true", e, d, ok)
		}
	}
	for k := kop.SetBattlefield + 1; k < kop.SetAndFilter; k++ {
		if _, ok := (kop.SetExpr{{Kind: k}}).SetDepth(); ok || k.SetPushes() {
			t.Errorf("free kind %d is accepted", k)
		}
	}

	// the defending player is the one the attack trigger was queued with
	four := newSetScen(t, 4, 0)
	four.s.res.e.defSeat = 2 + 1
	setWant(t, "RefDefendingPlayer at four seats", four.eval(setRefOp(kop.RefDefendingPlayer)), PlayerCode(2))
}

// TestSetBattlefieldEval: SetBattlefield lists every permanent in object
// index order, whatever slots they hold, and no object off the battlefield; a
// following SetAndFilter narrows it.
func TestSetBattlefieldEval(t *testing.T) {
	sc := newSetScen(t, 2, 0)
	s := sc.s
	early := sc.obj(vanilla.GrizzlyBears, 0)
	late := sc.obj(vanilla.SavannahLions, 0)
	grave := sc.obj(vanilla.GrizzlyBears, 1)
	s.place(grave, kop.ZoneGraveyard, false)
	// the later object enters first, so slot order is not object index order
	s.enter(sc.k, late, 0, false, kop.ZoneNone)
	s.enter(sc.k, early, 0, false, kop.ZoneNone)
	if !(sc.self < sc.lions && sc.lions < sc.plains && sc.plains < early && early < late) || s.objs[late].slot >= s.objs[early].slot {
		t.Fatalf("precondition: index order self, lions, plains, early, late with late in an earlier slot than early (%d, %d)",
			s.objs[late].slot, s.objs[early].slot)
	}
	if !sc.onBF(early) || !sc.onBF(late) || sc.onBF(grave) {
		t.Fatal("precondition: two more permanents and one card in a graveyard")
	}
	S, L, P, E, N := TCode(sc.self), TCode(sc.lions), TCode(sc.plains), TCode(early), TCode(late)
	field := setOp1(kop.SetBattlefield, 0)
	setWant(t, "SetBattlefield", sc.eval(field), S, L, P, E, N)
	setWant(t, "SetBattlefield ∩ creature", sc.eval(field, setOp1(kop.SetAndFilter, uint16(setFCreature))), S, L, E, N)
	setWant(t, "owners of the creatures", sc.eval(field, setOp1(kop.SetAndFilter, uint16(setFCreature)), setOp1(kop.SetOwnerOf, 0)),
		PlayerCode(0), PlayerCode(1))
}

// TestSetBasesDoNotAllocate: evaluating a SetRefBase or a SetBattlefield
// program allocates nothing.
func TestSetBasesDoNotAllocate(t *testing.T) {
	sc := newSetScen(t, 2, 0)
	s := sc.s
	s.perms[s.objs[sc.self].slot].attached = s.objs[sc.lions].slot
	progs := []kop.SetExpr{
		{setRefOp(kop.RefAttached), setOp1(kop.SetOwnerOf, 0)},
		{setRefOp(kop.RefEnchantedCtrl)},
		{setRefOp(kop.RefTriggerObj)},
		{setOp1(kop.SetBattlefield, 0), setOp1(kop.SetAndFilter, uint16(setFCreature))},
	}
	ctx := setCtx{ev: s.ctxEntry(&s.res.e)}
	members := make([]int, len(progs))
	var list [MaxObj + MaxSeats]TCode
	var vers [MaxObj + MaxSeats]uint8
	allocs := testing.AllocsPerRun(100, func() {
		for i, e := range progs {
			v := s.evalSetExpr(sc.k, e, ctx)
			members[i] = s.setMembers(&v, s.res.e.ctrl, &list, &vers)
		}
	})
	if slices.Contains(members, 0) {
		t.Fatalf("precondition: every program names a member, got %v", members)
	}
	if allocs != 0 {
		t.Errorf("evaluation allocates %v times", allocs)
	}
}
