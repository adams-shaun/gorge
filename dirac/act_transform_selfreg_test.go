package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTransformSelfReturnedSource: a transform ability whose source left the
// battlefield and returned is a new object, so Self names nothing and the new
// object stays front face up (CR 400.7, 603.7c). The kernel records the
// source's version when the ability is put on the stack; transform must
// compare it, as objRef does.
func TestTransformSelfReturnedSource(t *testing.T) {
	tbl := *v.Table
	tbl.Name = "transformselfreg"
	tbl.Exprs = make([]kop.Expr, 1) // a generated-table prep (prep.gen)
	front := CardID(len(v.Table.Cards))
	back := front + 1
	tbl.Cards = append(slices.Clone(v.Table.Cards),
		kop.CardDef{Name: "Test Turning", Types: kop.TCreature, Power: 1, Toughness: 1,
			FaceMode: kop.FaceTransform, Faces: []CardID{front, back},
			Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{{Op: kop.OpTransform, Obj: kop.RefSelf}}}}},
		kop.CardDef{Name: "Test Turning Back", Types: kop.TCreature, Power: 3, Toughness: 3,
			FaceMode: kop.FaceTransform, Faces: []CardID{front, back}, Face: 1})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	prog := &tbl.Cards[front].Abilities[0].Prog[0]
	o := sc.perm(front, 0)
	sl := sc.slot(o)
	zone, ver := sc.s.objs[o].zone, sc.s.objs[o].ver
	if zone != kop.ZoneBattlefield {
		t.Fatalf("precondition: zone %d, want the battlefield", zone)
	}
	// the live source transforms
	sc.s.res.e = stackEntry{kind: seTrigger, obj: o, srcVer: ver, card: front, ctrl: 0}
	sc.s.exec(sc.k, prog)
	if got := sc.s.objs[o].card; got != back {
		t.Fatalf("live source: card %d, want the back face %d", got, back)
	}
	// it leaves and returns: a new object, front face up (CR 712.8a)
	sc.s.leave(sc.k, sl, kop.ZoneExile, false)
	sc.s.enter(sc.k, o, 0, false, kop.ZoneExile)
	if got := sc.s.objs[o].ver; got == ver {
		t.Fatalf("precondition: ver = %d before and after the blink, want a new object", ver)
	}
	if zone := sc.s.objs[o].zone; zone != kop.ZoneBattlefield {
		t.Fatalf("precondition: zone %d after the blink, want the battlefield", zone)
	}
	if got := sc.s.objs[o].card; got != front {
		t.Fatalf("precondition: card %d after the blink, want the front face %d", got, front)
	}
	// the pending ability still carries the registration version: Self names
	// nothing, and the new object is left alone
	sc.s.res.e = stackEntry{kind: seTrigger, obj: o, srcVer: ver, card: front, ctrl: 0}
	if sc.s.objs[o].ver == sc.s.res.e.srcVer {
		t.Fatalf("precondition: live ver %d equals the registration version, want a mismatch", sc.s.objs[o].ver)
	}
	sc.s.exec(sc.k, prog)
	if got := sc.s.objs[o].card; got != front {
		t.Fatalf("returned source: card %d, want the front face %d left alone", got, front)
	}
}
