package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// setOpRun casts a free sorcery whose one instruction is in (Obj RefSet) over
// set, on the setPos position, and resolves it. fOpp is the filter of creatures
// an opponent controls, built into the table.
type setOpRun struct {
	*setPos
	st *setTbl
}

func newSetOpRun(t *testing.T, set kop.SetExpr, in kop.Instr, tweak func(*setTbl)) *setOpRun {
	t.Helper()
	st := newSetTbl(t, []kop.SetExpr{set})
	if tweak != nil {
		tweak(st)
	}
	in.Obj = kop.RefSet
	first := len(st.t.Cards)
	st.t.Cards = append(append([]kop.CardDef(nil), st.t.Cards...), kop.CardDef{Name: "Test Set Op", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{in}, SetRefs: []kop.SetRef{{PC: 0, Set: 1}}}}})
	st.cards = append(st.cards, CardID(first))
	return &setOpRun{setPos: newSetPos(t, st), st: st}
}

// run casts the spell and passes until the stack is empty.
func (r *setOpRun) run(t *testing.T) {
	t.Helper()
	o := r.sc.hand(r.st.cards[len(r.st.cards)-1], 0)
	r.sc.castCard(t, o)
	r.sc.resolveAll(t)
	if r.sc.s.Stalled() {
		t.Fatalf("stalled %s", StallName(r.sc.s.StallReason()))
	}
}

func (r *setOpRun) tapped(o Obj) bool {
	return r.sc.onBF(o) && r.sc.s.perms[r.sc.slot(o)].flags&pfTapped != 0
}

var bfZone = int16(kop.ZoneBattlefield)

func yourCreatures(st *setTbl) kop.SetExpr {
	return kop.SetExpr{Ops: []kop.SetOp{{Kind: kop.SoBaseValid, F: st.fCre, Arg: bfZone}}, Order: kop.OrderObj}
}

func anyCreatures(st *setTbl) kop.SetExpr {
	return kop.SetExpr{Ops: []kop.SetOp{{Kind: kop.SoBaseValid, F: st.fAnyC, Arg: bfZone}}, Order: kop.OrderObj}
}

// TestSetOpsObjectArms: each object op, run over a RefSet, acts on exactly the
// members: A and B (seat 0's creatures), never C (seat 1's) or a land.
func TestSetOpsObjectArms(t *testing.T) {
	mk := func(t *testing.T, in kop.Instr) *setOpRun {
		t.Helper()
		st := newSetTbl(t, nil)
		return newSetOpRun(t, yourCreatures(st), in, nil)
	}
	t.Run("tap", func(t *testing.T) {
		r := mk(t, kop.Instr{Op: kop.OpTap})
		r.run(t)
		if !r.tapped(r.a) || !r.tapped(r.b) || r.tapped(r.c) || r.tapped(r.l0) {
			t.Errorf("tapped a %v b %v c %v land %v, want true true false false", r.tapped(r.a), r.tapped(r.b), r.tapped(r.c), r.tapped(r.l0))
		}
	})
	t.Run("untap", func(t *testing.T) {
		r := mk(t, kop.Instr{Op: kop.OpUntap})
		for _, o := range []Obj{r.a, r.b, r.c} {
			r.sc.s.perms[r.sc.slot(o)].flags |= pfTapped
		}
		r.run(t)
		if r.tapped(r.a) || r.tapped(r.b) || !r.tapped(r.c) {
			t.Errorf("tapped a %v b %v c %v, want false false true", r.tapped(r.a), r.tapped(r.b), r.tapped(r.c))
		}
	})
	t.Run("pump", func(t *testing.T) {
		r := mk(t, kop.Instr{Op: kop.OpPumpUntilEOT, A: kop.Const(2), B: kop.Const(1)})
		pw := func(o Obj) int { return r.sc.s.power(r.sc.k, r.sc.slot(o)) }
		before := pw(r.a)
		r.run(t)
		if pw(r.a) != before+2 || pw(r.b) != before+2 || pw(r.c) != before {
			t.Errorf("power a %d b %d c %d, want %d %d %d", pw(r.a), pw(r.b), pw(r.c), before+2, before+2, before)
		}
	})
	t.Run("destroy", func(t *testing.T) {
		r := mk(t, kop.Instr{Op: kop.OpDestroy})
		r.run(t)
		if r.sc.onBF(r.a) || r.sc.onBF(r.b) || !r.sc.onBF(r.c) || !r.sc.onBF(r.l0) {
			t.Errorf("on battlefield a %v b %v c %v land %v, want false false true true", r.sc.onBF(r.a), r.sc.onBF(r.b), r.sc.onBF(r.c), r.sc.onBF(r.l0))
		}
		if r.sc.s.objs[r.a].zone != kop.ZoneGraveyard || r.sc.s.objs[r.b].zone != kop.ZoneGraveyard {
			t.Error("the destroyed creatures are not in the graveyard")
		}
	})
	t.Run("exile", func(t *testing.T) {
		r := mk(t, kop.Instr{Op: kop.OpExile})
		r.run(t)
		if r.sc.s.objs[r.a].zone != kop.ZoneExile || r.sc.s.objs[r.b].zone != kop.ZoneExile || !r.sc.onBF(r.c) {
			t.Errorf("zones a %d b %d, c on battlefield %v; want exile, exile, true", r.sc.s.objs[r.a].zone, r.sc.s.objs[r.b].zone, r.sc.onBF(r.c))
		}
	})
	t.Run("bounce", func(t *testing.T) {
		r := mk(t, kop.Instr{Op: kop.OpBounce})
		r.run(t)
		if r.sc.s.objs[r.a].zone != kop.ZoneHand || r.sc.s.objs[r.b].zone != kop.ZoneHand || !r.sc.onBF(r.c) {
			t.Errorf("zones a %d b %d, c on battlefield %v; want hand, hand, true", r.sc.s.objs[r.a].zone, r.sc.s.objs[r.b].zone, r.sc.onBF(r.c))
		}
	})
	t.Run("gain control", func(t *testing.T) {
		oppCre := kop.FilterID(len(v.Table.Filters) + 2) // the two filters newSetTbl appends, then this one
		r := newSetOpRun(t, kop.SetExpr{Ops: []kop.SetOp{{Kind: kop.SoBaseValid, F: oppCre, Arg: bfZone}}, Order: kop.OrderObj},
			kop.Instr{Op: kop.OpGainControl, A: kop.ValueRef{N: kop.DurPermanent}, C: int16(kop.RefNone)},
			func(st *setTbl) {
				st.t.Filters = append(st.t.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlOpp})
			})
		if got := r.sc.s.perms[r.sc.slot(r.c)].ctrl; got != 1 {
			t.Fatalf("precondition: C is controlled by %d, want 1", got)
		}
		r.run(t)
		if got := r.sc.s.perms[r.sc.slot(r.c)].ctrl; got != 0 {
			t.Errorf("C is controlled by seat %d after the control grant, want 0", got)
		}
		if got := r.sc.s.perms[r.sc.slot(r.a)].ctrl; got != 0 {
			t.Errorf("A is controlled by seat %d, want 0", got)
		}
	})
}

// TestSetOpsPlayerArms: the player ops over a players set.
func TestSetOpsPlayerArms(t *testing.T) {
	each := func(r kop.Ref) kop.SetExpr {
		return kop.SetExpr{Ops: []kop.SetOp{{Kind: kop.SoBaseRef, R: r, Arg: 1}}, Players: true, Order: kop.OrderSeat}
	}
	t.Run("gain life", func(t *testing.T) {
		r := newSetOpRun(t, each(kop.RefEachPlayer), kop.Instr{Op: kop.OpGainLife, A: kop.Const(3)}, nil)
		l0, l1 := r.sc.s.pl[0].life, r.sc.s.pl[1].life
		r.run(t)
		if r.sc.s.pl[0].life != l0+3 || r.sc.s.pl[1].life != l1+3 {
			t.Errorf("life %d %d, want %d %d", r.sc.s.pl[0].life, r.sc.s.pl[1].life, l0+3, l1+3)
		}
	})
	t.Run("lose life", func(t *testing.T) {
		r := newSetOpRun(t, each(kop.RefEachOpponent), kop.Instr{Op: kop.OpLoseLife, A: kop.Const(2)}, nil)
		l0, l1 := r.sc.s.pl[0].life, r.sc.s.pl[1].life
		r.run(t)
		if r.sc.s.pl[0].life != l0 || r.sc.s.pl[1].life != l1-2 {
			t.Errorf("life %d %d, want %d %d", r.sc.s.pl[0].life, r.sc.s.pl[1].life, l0, l1-2)
		}
		if r.sc.s.g.lostAmt != 2 {
			t.Errorf("ExLastLifeLost %d, want 2", r.sc.s.g.lostAmt)
		}
	})
	t.Run("draw", func(t *testing.T) {
		r := newSetOpRun(t, each(kop.RefEachPlayer), kop.Instr{Op: kop.OpDraw, A: kop.Const(2)}, nil)
		h0, h1 := r.sc.s.pl[0].handN, r.sc.s.pl[1].handN
		r.run(t)
		// the spell was put in seat 0's hand and cast: net zero
		if r.sc.s.pl[1].handN != h1+2 || r.sc.s.pl[0].handN != h0+2 {
			t.Errorf("hands %d %d, want %d %d", r.sc.s.pl[0].handN, r.sc.s.pl[1].handN, h0+2, h1+2)
		}
	})
	t.Run("discard", func(t *testing.T) {
		r := newSetOpRun(t, each(kop.RefEachPlayer), kop.Instr{Op: kop.OpDiscard, A: kop.Const(5), C: int16(kop.ChooseSelf)}, nil)
		r.sc.hand(v.Plains, 0)
		r.sc.hand(v.Plains, 1)
		r.run(t)
		if r.sc.s.pl[0].handN != 0 || r.sc.s.pl[1].handN != 0 {
			t.Errorf("hands %d %d after each player discards up to 5, want 0 0", r.sc.s.pl[0].handN, r.sc.s.pl[1].handN)
		}
	})
	t.Run("sacrifice", func(t *testing.T) {
		var fc kop.FilterID
		r := newSetOpRun(t, each(kop.RefEachOpponent), kop.Instr{Op: kop.OpSacrifice, A: kop.Const(1)}, func(st *setTbl) { fc = st.fAnyC })
		r.st.t.Cards[r.st.cards[len(r.st.cards)-1]].Abilities[0].Prog[0].F = fc
		r.run(t)
		if r.sc.onBF(r.c) || !r.sc.onBF(r.a) {
			t.Errorf("C on battlefield %v, A %v; want C sacrificed (false) and A kept (true)", r.sc.onBF(r.c), r.sc.onBF(r.a))
		}
	})
}
