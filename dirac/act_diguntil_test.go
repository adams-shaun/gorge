package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestDigUntilNinthCreatureStopsAtNinth: an X=9 reveal-until that finds nine
// creatures must stop at the ninth. The old eight-slot found buffer pushed
// the ninth into the rest pile and kept scanning the whole library, drawing
// RNG for every later card and bottoming the ninth creature (Old Stickfingers
// at X=9, selfcast seed 802: five extra draws).
func TestDigUntilNinthCreatureStopsAtNinth(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	pl := &sc.s.pl[0]
	var bears [9]Obj
	var lands [3]Obj
	for i := range bears {
		bears[i] = sc.obj(v.GrizzlyBears, 0)
		sc.s.objs[bears[i]].zone = kop.ZoneLibrary
	}
	for i := range lands {
		lands[i] = sc.obj(v.Plains, 0)
		sc.s.objs[lands[i]].zone = kop.ZoneLibrary
	}
	for i := len(lands) - 1; i >= 0; i-- {
		pl.libTop(lands[i])
	}
	for i := len(bears) - 1; i >= 0; i-- {
		pl.libTop(bears[i])
	}
	before := pl.libHi - pl.libLo
	draws := sc.s.rng.Draws
	sc.s.res.e = stackEntry{kind: seSpell, ctrl: 0, trigObj: NoObj}
	in := kop.Instr{
		Op:  kop.OpDigUntil,
		Obj: kop.RefController,
		A:   kop.Const(9),
		C:   204,             // found -> graveyard; rest -> bottom, random order
		F:   kop.FilterID(1), // vanilla fCreature
	}
	sc.s.execDigUntil(sc.k, &in)
	for i, o := range bears {
		if z := sc.s.objs[o].zone; z != kop.ZoneGraveyard {
			t.Fatalf("bear %d zone %d, want the graveyard", i, z)
		}
	}
	if got := pl.libHi - pl.libLo; got != before-9 {
		t.Fatalf("library %d cards, want %d (the reveal stops at the ninth creature)", got, before-9)
	}
	if sc.s.rng.Draws != draws {
		t.Fatalf("RNG draws %d -> %d, want no draws: no rest card was revealed", draws, sc.s.rng.Draws)
	}
}

// TestSacrificeSelfBodySacrificesSource: an OpSacrifice with a Self filter
// (Prowling Pangolin's "sacrifice CARDNAME") must sacrifice the source. The
// old NoSlot source made FSelf match nothing, so the body silently did
// nothing.
func TestSacrificeSelfBodySacrificesSource(t *testing.T) {
	tbl := v.NewTable()
	tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FSelf})
	self := kop.FilterID(len(tbl.Filters) - 1)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	src := sc.perm(v.GrizzlyBears, 0)
	other := sc.perm(v.GrizzlyBears, 0)
	sc.s.res.e = stackEntry{kind: seSpell, ctrl: 0, obj: src}
	in := kop.Instr{Op: kop.OpSacrifice, Obj: kop.RefController, A: kop.Const(1), F: self}
	if !sc.s.exec(sc.k, &in) {
		t.Fatal("the sacrifice suspended instead of resolving")
	}
	if z := sc.s.objs[src].zone; z != kop.ZoneGraveyard {
		t.Fatalf("source zone %d, want the graveyard", z)
	}
	if z := sc.s.objs[other].zone; z != kop.ZoneBattlefield {
		t.Fatalf("other zone %d, want the battlefield", z)
	}
}

// TestSacrificeOtherBodyExcludesSource: an OpSacrifice with an Other filter
// must not offer the source. The old NoSlot source matched it too, so the
// sacrifice became an unnecessary choice.
func TestSacrificeOtherBodyExcludesSource(t *testing.T) {
	tbl := v.NewTable()
	tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FOther})
	otherF := kop.FilterID(len(tbl.Filters) - 1)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	src := sc.perm(v.GrizzlyBears, 0)
	other := sc.perm(v.GrizzlyBears, 0)
	sc.s.res.e = stackEntry{kind: seSpell, ctrl: 0, obj: src}
	in := kop.Instr{Op: kop.OpSacrifice, Obj: kop.RefController, A: kop.Const(1), F: otherF}
	if !sc.s.exec(sc.k, &in) {
		t.Fatal("the sacrifice suspended instead of resolving")
	}
	if z := sc.s.objs[src].zone; z != kop.ZoneBattlefield {
		t.Fatalf("source zone %d, want the battlefield", z)
	}
	if z := sc.s.objs[other].zone; z != kop.ZoneGraveyard {
		t.Fatalf("other zone %d, want the graveyard", z)
	}
}

// TestSacrificePayerSourceSlot: the each-player payer path (Prowling
// Pangolin's "any player may sacrifice ...") counts its candidates with the
// resolving source's slot. An Other filter must exclude the source and a
// Self filter must match only it; the old NoSlot source made Other match
// both permanents and Self match none.
func TestSacrificePayerSourceSlot(t *testing.T) {
	tbl := v.NewTable()
	tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FOther})
	otherF := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FSelf})
	selfF := kop.FilterID(len(tbl.Filters) - 1)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	src := sc.perm(v.GrizzlyBears, 0)
	sc.perm(v.GrizzlyBears, 0)
	sc.s.res.e = stackEntry{kind: seTrigger, obj: src, srcVer: sc.s.objs[src].ver, ctrl: 0}
	if n := sc.s.countSacLeft(sc.k, 0, otherF); n != 1 {
		t.Fatalf("Other candidates %d, want 1 (the source excluded)", n)
	}
	if n := sc.s.countSacLeft(sc.k, 0, selfF); n != 1 {
		t.Fatalf("Self candidates %d, want 1 (only the source)", n)
	}
}

// TestTriggerStartUnlessSwitchedIsModes: an UnlessSwitched at the start of a
// trigger is gorge's unless modes ask, not the trigger's own cost.
func TestTriggerStartUnlessSwitchedIsModes(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.s.res.e = stackEntry{kind: seTrigger, ctrl: 0}
	sc.s.res.pc = 0
	in := kop.Instr{Op: kop.OpUnlessSwitched, Obj: kop.RefController, A: kop.Const(1), C: int16(kop.PayLife | 1<<4)}
	if sc.s.execPay(sc.k, &in) {
		t.Fatal("the unless ask resolved without a decision")
	}
	if sc.s.g.pickF&pfModes == 0 {
		t.Fatal("UnlessSwitched at trigger pc 0 asked as a trigger cost")
	}
	if sc.s.g.pickF&pfIfPaid == 0 {
		t.Fatal("UnlessSwitched did not use the if-paid branch")
	}
}

// TestTriggerStartIfPaidStaysTriggerCost: a trigger's own Cost$ is still the
// trigger_cost pay/decline (Sanguine Spy), not the unless modes ask.
func TestTriggerStartIfPaidStaysTriggerCost(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.s.res.e = stackEntry{kind: seTrigger, ctrl: 0}
	sc.s.res.pc = 0
	in := kop.Instr{Op: kop.OpIfPaid, Obj: kop.RefController, A: kop.Const(1), C: int16(kop.PayLife | 1<<4)}
	if sc.s.execPay(sc.k, &in) {
		t.Fatal("the trigger cost resolved without a decision")
	}
	if sc.s.g.pickF&pfModes != 0 {
		t.Fatal("a trigger's own cost asked as an unless")
	}
	if sc.s.g.pickF&pfIfPaid == 0 {
		t.Fatal("a trigger's own cost did not use the if-paid branch")
	}
}
