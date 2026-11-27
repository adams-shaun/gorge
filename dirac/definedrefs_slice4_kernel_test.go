package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTriggerSpellTargetsSet: the plural ref names every distinct object
// target of the triggering spell, in target order, skipping a player target
// and a repeat (the single ref still refuses a set).
func TestTriggerSpellTargetsSet(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	a := sc.perm(v.TyphoidRats, 1)
	b := sc.perm(v.TyphoidRats, 1)
	if a == b {
		t.Fatal("precondition: two distinct permanents")
	}
	spell := sc.obj(v.TyphoidRats, 0)
	s.objs[spell].zone = kop.ZoneStack
	e := stackEntry{kind: seSpell, obj: spell}
	e.tgt[0], e.tgt[1], e.tgt[2], e.nt = TCode(a), TCode(b), TCode(a), 3
	s.sp = 0
	s.push(e)
	s.res.e = stackEntry{kind: seTrigger, obj: a, trigObj: spell}
	var out [kop.MaxTargets + maxXTgt]Obj
	if n := s.triggerSpellTargetList(spell, &out); n != 2 || out[0] != a || out[1] != b {
		t.Fatalf("target list = %v (%d), want [%d %d]", out[:n], n, a, b)
	}
	if got := s.triggerSpellTarget(spell); got != NoObj {
		t.Errorf("single ref over two targets = %d, want none", got)
	}

	// A player target is skipped by the set, and makes the single ref none.
	e.tgt[0], e.tgt[1], e.nt = TCode(PlayerTarget0), TCode(b), 2
	s.sp = 0
	s.push(e)
	if n := s.triggerSpellTargetList(spell, &out); n != 1 || out[0] != b {
		t.Errorf("target list with a player = %v (%d), want [%d]", out[:n], n, b)
	}
	if got := s.triggerSpellTarget(spell); got != NoObj {
		t.Errorf("single ref over a player and an object = %d, want none", got)
	}
}

// TestEquippedByEnchantedRef: the Aura's enchanted Equipment's bearer is the
// named object; a missing link names none.
func TestEquippedByEnchantedRef(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	aura := sc.perm(v.GrizzlyBears, 0)
	eq := sc.perm(v.GrizzlyBears, 0)
	creature := sc.perm(v.TyphoidRats, 0)
	s.perms[sc.slot(aura)].attached = sc.slot(eq)
	s.perms[sc.slot(eq)].attached = sc.slot(creature)
	s.res.e = stackEntry{kind: seTrigger, obj: aura}
	if got := s.objRefObj(kop.RefEquippedByEnchanted); got != creature {
		t.Fatalf("RefEquippedByEnchanted = %d, want %d", got, creature)
	}
	s.perms[sc.slot(eq)].attached = NoSlot
	if got := s.objRefObj(kop.RefEquippedByEnchanted); got != NoObj {
		t.Errorf("unattached Equipment: %d, want none", got)
	}
}

// TestDmgFromEachAttached: the RepeatEach source deals its amount to the
// permanent it is attached to (Aura Barbs' second line), and nothing when it
// is unattached.
func TestDmgFromEachAttached(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	aura := sc.perm(v.GrizzlyBears, 0)
	creature := sc.perm(v.TyphoidRats, 1)
	s.perms[sc.slot(aura)].attached = sc.slot(creature)
	s.res.eachObj = aura
	in := kop.Instr{Op: kop.OpDealDamageFrom, Obj: kop.RefEach, A: kop.Const(2), B: kop.Const(kop.DmgFromEachAttached)}
	s.dmgSrcAttached(sc.k, &in)
	if got := s.perms[sc.slot(creature)].damage; got != 2 {
		t.Fatalf("attached bearer damage = %d, want 2", got)
	}
	s.perms[sc.slot(aura)].attached = NoSlot
	other := sc.perm(v.TyphoidRats, 1)
	s.res.eachObj = aura
	s.dmgSrcAttached(sc.k, &in)
	if got := s.perms[sc.slot(other)].damage; got != 0 {
		t.Errorf("unattached source dealt %d to an unrelated permanent", got)
	}
}
