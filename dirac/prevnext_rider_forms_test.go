package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// installPrevRider installs a recipient-keyed Ext PreventNext shield of
// amount 3 on the bear, as a resolving source does, and returns the shield
// record's index.
func installPrevRider(f *prevXScen, in *kop.Instr, bear Obj) int {
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(f.burn, 0), 0, f.burn
	e.tgt[0] = TCode(bear)
	f.s.prevNextAdd(f.k, in)
	for i := range f.s.g.prevCol {
		if f.s.g.prevCol[i].ctl == TCode(bear) {
			return i
		}
	}
	return -1
}

// TestPrevNextCounterRider: the PutCounter rider puts the amount each
// application prevented as +1/+1 counters on the shield's recipient (Test of
// Faith, Temper).
func TestPrevNextCounterRider(t *testing.T) {
	f := newPrevXScen(t)
	sink := f.perm(f.mite, 1) // a decoy first: the rider must not fall back to it
	bear := f.perm(f.bear, 1)
	in := kop.ExtInstr(kop.ExtPreventNext)
	in.C = int16(kop.PrevNextRiderCounter)
	in.Obj = kop.RefTarget0
	in.B = kop.Const(3)
	if at := installPrevRider(f, &in, bear); at < 0 {
		t.Fatal("precondition: the shield was not installed")
	}
	if n, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(bear), 1, f.slot(bear), false, 2); !ok || n != 0 {
		t.Fatalf("shield prevented %d (applied %v), want all 2", n, ok)
	}
	if n := f.s.perms[f.slot(bear)].ctrN(f.s, int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("+1/+1 counters = %d, want 2", n)
	}
	if n := f.s.perms[f.slot(sink)].ctrN(f.s, int(kop.CtrP1P1)); n != 0 {
		t.Fatalf("decoy got %d +1/+1 counters, want 0", n)
	}
}

// TestPrevNextDelayedCounterRider: the delayed Effect rider accumulates the
// prevented amount and puts it as P0P1 counters at the beginning of the next
// end step (Sacred Boon, Scars of the Veteran).
func TestPrevNextDelayedCounterRider(t *testing.T) {
	f := newPrevXScen(t)
	sink := f.perm(f.mite, 1) // a decoy first: the rider must not fall back to it
	bear := f.perm(f.bear, 1)
	in := kop.ExtInstr(kop.ExtPreventNext)
	in.C = int16(kop.PrevNextRiderDelayedCounter)
	in.Obj = kop.RefTarget0
	in.B = kop.Const(3)
	if at := installPrevRider(f, &in, bear); at < 0 {
		t.Fatal("precondition: the shield was not installed")
	}
	if _, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(bear), 1, f.slot(bear), false, 2); !ok {
		t.Fatal("precondition: the shield did not apply")
	}
	if n := f.s.perms[f.slot(bear)].ctrN(f.s, int(kop.CtrP0P1)); n != 0 {
		t.Fatalf("P0P1 counters before the end step = %d, want 0", n)
	}
	f.s.prevEndStepCounters(f.k)
	if n := f.s.perms[f.slot(bear)].ctrN(f.s, int(kop.CtrP0P1)); n != 2 {
		t.Fatalf("P0P1 counters after the end step = %d, want 2", n)
	}
	if n := f.s.perms[f.slot(sink)].ctrN(f.s, int(kop.CtrP0P1)); n != 0 {
		t.Fatalf("decoy got %d P0P1 counters, want 0", n)
	}
}

// TestPrevNextDamageRider: the DealDamage rider deals the prevented amount to
// the parent ability's target (Acolyte's Reward, Vengeful Archon).
func TestPrevNextDamageRider(t *testing.T) {
	f := newPrevXScen(t)
	bear := f.perm(f.bear, 1)
	in := kop.ExtInstr(kop.ExtPreventNext)
	in.C = int16(kop.PrevNextRiderDamage)
	in.Obj = kop.RefTarget0
	in.B = kop.Const(3)
	in.W = kop.RefTarget1 // the parent ability's target
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(f.burn, 0), 0, f.burn
	e.tgt[0] = TCode(bear)
	e.tgt[1] = PlayerCode(1)
	f.s.prevNextAdd(f.k, &in)
	life := f.s.pl[1].life
	if n, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(bear), 1, f.slot(bear), false, 2); !ok || n != 0 {
		t.Fatalf("shield prevented %d (applied %v), want all 2", n, ok)
	}
	if got := life - f.s.pl[1].life; got != 2 {
		t.Fatalf("rider dealt %d damage to the parent target, want 2", got)
	}
}

// TestPrevNextDividedShares: a divided shield installs one record per chosen
// target with the share announced for it (Angel of Salvation, Embolden,
// Pollen Remedy, Remedy, Serra's Hymn).
func TestPrevNextDividedShares(t *testing.T) {
	f := newPrevXScen(t)
	bear := f.perm(f.bear, 1)
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(f.burn, 0), 0, f.burn
	e.tgt[0] = TCode(bear)
	e.xi = 1
	x := &f.s.g.xtp[0]
	x.n = 1
	x.slot[0] = 0
	x.code[0] = PlayerCode(1)
	x.div[0], x.div[1] = 1, 2 // the first target's share, then the extra target's
	in := kop.ExtInstr(kop.ExtPreventNext)
	in.C = int16(kop.PrevNextDivided)
	in.W = kop.RefTarget0
	in.B = kop.Const(3)
	f.s.prevNextAdd(f.k, &in)
	if n, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(bear), 1, f.slot(bear), false, 5); !ok || n != 4 {
		t.Fatalf("bear shield left %d (applied %v), want 4", n, ok)
	}
	if n, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, PlayerCode(1), 1, NoSlot, false, 5); !ok || n != 3 {
		t.Fatalf("player shield left %d (applied %v), want 3", n, ok)
	}
}

// TestPrevNextMultiRecipients: the Defined$ Valid filter form installs one
// shield on every matching permanent, and the player form one on every
// living player (Kitsune Palliator, Samite Censer-Bearer).
func TestPrevNextMultiRecipients(t *testing.T) {
	f := newPrevXScen(t)
	bear := f.perm(f.bear, 1)
	mite := f.perm(f.mite, 1)
	if len(f.k.t.Filters) == 0 {
		f.k.t.Filters = append(f.k.t.Filters, kop.Filter{})
	}
	f.k.t.Filters = append(f.k.t.Filters, kop.Filter{Types: kop.TCreature})
	fid := kop.FilterID(len(f.k.t.Filters) - 1)
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(f.burn, 0), 0, f.burn
	in := kop.ExtInstr(kop.ExtPreventNext)
	in.C = int16(kop.PrevNextMultiObj)
	in.F = fid
	in.B = kop.Const(3)
	f.s.prevNextAdd(f.k, &in)
	if n, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(bear), 1, f.slot(bear), false, 5); !ok || n != 2 {
		t.Fatalf("bear shield left %d (applied %v), want 2", n, ok)
	}
	if n, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(mite), 1, f.slot(mite), false, 5); !ok || n != 2 {
		t.Fatalf("mite shield left %d (applied %v), want 2", n, ok)
	}
	// a fresh scenario: the creature shields already take two of the three
	// records, and the table's capacity stalls the fourth
	g := newPrevXScen(t)
	ge := &g.s.res.e
	ge.kind, ge.obj, ge.ctrl, ge.card = seSpell, g.obj(g.burn, 0), 0, g.burn
	pl := kop.ExtInstr(kop.ExtPreventNext)
	pl.C = int16(kop.PrevNextMultiPlayer)
	pl.B = kop.Const(3)
	g.s.prevNextAdd(g.k, &pl)
	if n, ok := g.s.prevNextTake(g.k, 1, NoSlot, g.burn, PlayerCode(1), 1, NoSlot, false, 5); !ok || n != 2 {
		t.Fatalf("player shield left %d (applied %v), want 2", n, ok)
	}
}

// TestPrevNextRadiance: Radiance installs a shield on the target creature and
// on each other creature sharing a colour with it (Wojek Apothecary).
func TestPrevNextRadiance(t *testing.T) {
	f := newPrevXScen(t)
	f.k.t.Cards[f.bear].Colors = kop.ColR
	f.k.t.Cards[f.mite].Colors = kop.ColR
	bear := f.perm(f.bear, 1)
	mite := f.perm(f.mite, 1)
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(f.burn, 0), 0, f.burn
	e.tgt[0] = TCode(bear)
	in := kop.ExtInstr(kop.ExtPreventNext)
	in.C = int16(kop.PrevNextRadiance)
	in.Obj = kop.RefTarget0
	in.B = kop.Const(3)
	f.s.prevNextAdd(f.k, &in)
	if n, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(bear), 1, f.slot(bear), false, 5); !ok || n != 2 {
		t.Fatalf("bear shield left %d (applied %v), want 2", n, ok)
	}
	if n, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(mite), 1, f.slot(mite), false, 5); !ok || n != 2 {
		t.Fatalf("mite shield left %d (applied %v), want 2", n, ok)
	}
}

// TestPrevNextDelayedCounterCreatureGate: Scars of the Veteran's delayed
// rider puts nothing when its target is no longer a creature at the end step
// (IsPresent$ Creature.IsRemembered); Sacred Boon's ungated rider always
// places.
func TestPrevNextDelayedCounterCreatureGate(t *testing.T) {
	f := newPrevXScen(t)
	bear := f.perm(f.bear, 1)
	in := kop.ExtInstr(kop.ExtPreventNext)
	in.C = int16(kop.PrevNextRiderDelayedCounter)
	in.Obj = kop.RefTarget0
	in.B = kop.Const(3)
	in.W = kop.RefSelf // the lowering's creature-gate marker
	at := installPrevRider(f, &in, bear)
	if at < 0 {
		t.Fatal("precondition: the shield was not installed")
	}
	if _, ok := f.s.prevNextTake(f.k, 1, NoSlot, f.burn, TCode(bear), 1, f.slot(bear), false, 2); !ok {
		t.Fatal("precondition: the shield did not apply")
	}
	p := &f.s.perms[f.slot(bear)]
	p.ovr.flags |= ovTypes // the target is no longer a creature
	p.ovr.types = 0
	f.s.prevEndStepCounters(f.k)
	if n := f.s.perms[f.slot(bear)].ctrN(f.s, int(kop.CtrP0P1)); n != 0 {
		t.Fatalf("gated rider put %d P0P1 counters on a non-creature, want 0", n)
	}
}
