package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// prevSpellShield installs Dromoka's Command's first mode (a spell-keyed
// chosen-source shield, kop.PrevNextChosenSpell) over a burn spell of seat 1
// on the stack, as the Command of seat 0 resolves, and returns the burn
// spell. The burn spell is then the resolving source of the damage.
func prevSpellShield(f *prevXScen) Obj {
	burn := f.obj(f.burn, 1)
	f.s.objs[burn].zone = kop.ZoneStack
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(f.chant, 0), 0, f.chant
	e.tgt[0] = TCode(burn)
	in := kop.ExtInstr(kop.ExtPreventNext)
	in.C = int16(kop.PrevNextAll | kop.PrevNextFrom | kop.PrevNextChosen | kop.PrevNextChosenAny | kop.PrevNextChosenTurn | kop.PrevNextChosenSpell)
	in.Obj = kop.RefTarget0
	in.B = kop.Const(0)
	f.s.prevNextAdd(f.k, &in)
	e.kind, e.obj, e.ctrl, e.card = seSpell, burn, 1, f.burn
	f.s.res.active = true
	return burn
}

// TestPrevSpellShield: "prevent all damage target instant or sorcery spell
// would deal this turn" (Dromoka's Command): every damage event from the
// targeted spell while it is on the stack is prevented, none is spent, and a
// later cast of the same card (its version moved on) is another source.
func TestPrevSpellShield(t *testing.T) {
	f := newPrevXScen(t)
	burn := prevSpellShield(f)
	hit := func() (int, bool) {
		return f.s.prevNextTake(f.k, 1, NoSlot, f.burn, PlayerTarget0, 0, NoSlot, false, 3)
	}
	for i := 0; i < 2; i++ {
		if n, ok := hit(); !ok || n != 0 {
			t.Fatalf("event %d: prevented to %d (applied %v), want all 3 and every event", i, n, ok)
		}
	}
	f.s.objs[burn].ver++
	if n, ok := hit(); ok || n != 3 {
		t.Fatalf("the card cast again: prevented to %d (applied %v), want the damage dealt", n, ok)
	}
}

// TestPrevSpellShieldNeedsASpellOnTheStack: a target that is no longer on the
// stack, and the mutation switch, install nothing.
func TestPrevSpellShieldNeedsASpellOnTheStack(t *testing.T) {
	for _, mutate := range []bool{false, true} {
		f := newPrevXScen(t)
		burn := f.obj(f.burn, 1) // in no zone: not a spell on the stack
		e := &f.s.res.e
		e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(f.chant, 0), 0, f.chant
		e.tgt[0] = TCode(burn)
		if mutate {
			f.s.objs[burn].zone = kop.ZoneStack
			DebugNoPrevSpell = true
		}
		in := kop.ExtInstr(kop.ExtPreventNext)
		in.C = int16(kop.PrevNextAll | kop.PrevNextFrom | kop.PrevNextChosen | kop.PrevNextChosenAny | kop.PrevNextChosenTurn | kop.PrevNextChosenSpell)
		in.Obj = kop.RefTarget0
		in.B = kop.Const(0)
		f.s.prevNextAdd(f.k, &in)
		DebugNoPrevSpell = false
		for i := range f.s.g.prevCol {
			if f.s.g.prevCol[i].sc != 0 || f.s.g.prevCol[i].col != 0 {
				t.Fatalf("mutation %v: a shield was installed: %+v", mutate, f.s.g.prevCol[i])
			}
		}
	}
}
