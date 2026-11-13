package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// charmnumTable is vanilla's table with Test Charm choosing up to pick modes,
// ModesFew of which may be declined, and Elvish Visionary's enter trigger the
// same charm (mode 0 targets an opponent's creature, mode 1 a player).
func charmnumTable(pick, few uint8) *kop.Table {
	t := *v.Table
	t.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	a := t.Cards[v.TestCharm].Abilities[0]
	a.ModesPick, a.ModesFew = pick, few
	t.Cards[v.TestCharm].Abilities = []kop.Ability{a}
	tr := a
	tr.Kind, tr.Trig = kop.AbTriggered, kop.Trigger{Event: kop.TrigETB, Flags: kop.TfSelf}
	tr.Targets[0].Filter = v.Table.Cards[v.PreyUpon].Abilities[0].Targets[1].Filter // an opponent's creature
	t.Cards[v.ElvishVisionary].Abilities = []kop.Ability{tr}
	return &t
}

func modeOpts(d *Decision) (modes uint8, done bool) {
	for i := 0; i < int(d.N); i++ {
		switch d.Options[i].Kind() {
		case AkMode:
			modes |= 1 << d.Options[i].Mode()
		case AkDone:
			done = true
		}
	}
	return
}

// "choose one or both": Done is offered after one pick, not before.
func TestCharmnumDone(t *testing.T) {
	sc := newScenTable(t, charmnumTable(2, 1), Config{NoElide: true})
	sc.lands(0, v.Mountain, 1)
	sc.perm(v.GrizzlyBears, 1)
	fc := sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, uint8(fc), 0xff)
	d := sc.s.Step()
	if m, done := modeOpts(&d); d.Kind != DkModes || m != 3 || done {
		t.Fatalf("first pose %+v: modes %b done %v, want both modes and no Done", d.Options[:d.N], m, done)
	}
	sc.s.Apply(MakeAction(AkMode, 0, 0, 0))
	d = sc.s.Step()
	if m, done := modeOpts(&d); d.Kind != DkModes || m != 2 || !done {
		t.Fatalf("second pose %+v: modes %b done %v, want mode 1 and Done", d.Options[:d.N], m, done)
	}
	sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
	d = sc.s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("after Done: decision kind %d, want the mode 0 target", d.Kind)
	}
	if got := sc.s.BuildModes(); got != 1 {
		t.Fatalf("chosen modes %b, want mode 0 only", got)
	}
}

// "choose up to two": Done at the first pose, then a resolution running no block.
func TestCharmnumUpToZero(t *testing.T) {
	sc := newScenTable(t, charmnumTable(2, 2), Config{NoElide: true})
	sc.lands(0, v.Mountain, 1)
	sc.perm(v.GrizzlyBears, 1)
	fc := sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, uint8(fc), 0xff)
	d := sc.s.Step()
	if m, done := modeOpts(&d); d.Kind != DkModes || m != 3 || !done {
		t.Fatalf("first pose %+v: modes %b done %v, want both modes and Done", d.Options[:d.N], m, done)
	}
	sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
	sc.passBoth(t)
	sc.s.Step()
	if sc.s.pl[1].life != 20 || sc.s.pl[0].life != 20 {
		// life untouched: no mode ran
		t.Fatalf("life %d/%d after a zero-mode resolution", sc.s.pl[0].life, sc.s.pl[1].life)
	}
	if sc.s.objs[fc].zone != kop.ZoneGraveyard {
		t.Fatalf("the spell is in zone %d, want the graveyard", sc.s.objs[fc].zone)
	}
}

// "choose two" with one feasible mode is not offered; with two it is.
func TestCharmnumChooseTwoOffer(t *testing.T) {
	offered := func(withCreature bool) bool {
		sc := newScenTable(t, charmnumTable(2, 0), Config{NoElide: true})
		sc.lands(0, v.Mountain, 1)
		if withCreature {
			sc.perm(v.GrizzlyBears, 1)
		}
		fc := sc.hand(v.TestCharm, 0)
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(fc)) {
				return true
			}
		}
		return false
	}
	if offered(false) {
		t.Fatalf("a choose-two charm with one feasible mode was offered")
	}
	if !offered(true) {
		t.Fatalf("a choose-two charm with two feasible modes was not offered")
	}
}

// A "choose two" trigger with one eligible mode is removed, not resolved with one.
func TestCharmnumTriggerRemoved(t *testing.T) {
	sc := newScenTable(t, charmnumTable(2, 0), Config{NoElide: true})
	sc.lands(0, v.Forest, 2)
	ev := sc.hand(v.ElvishVisionary, 0)
	sc.do(t, AkCast, uint8(ev), 0xff)
	sc.passBoth(t) // the creature resolves; its trigger finds one eligible mode (no opposing creature)
	d := sc.s.Step()
	if d.Kind == DkModes {
		t.Fatalf("the trigger asked for modes: %+v", d.Options[:d.N])
	}
	if sc.s.sp != 0 {
		t.Fatalf("stack holds %d entries, want the trigger removed", sc.s.sp)
	}
}

// The combined target ask's occurrence byte counts the earlier chosen slots
// that list the same candidate.
func TestCharmnumCombinedTargets(t *testing.T) {
	var bufs [kop.MaxTargets][MaxOptions]uint8
	var ns [kop.MaxTargets]int
	bufs[0][0], bufs[0][1], ns[0] = 5, 7, 2
	bufs[1][0], bufs[1][1], ns[1] = 7, 9, 2
	bufs[2][0], ns[2] = 7, 1
	const mask = 0b111
	for _, c := range []struct {
		slot int
		cand uint8
		want uint8
	}{{0, 7, 0}, {1, 7, 1}, {1, 9, 0}, {2, 7, 2}} {
		if got := charmOcc(mask, c.slot, c.cand, &bufs, &ns); got != c.want {
			t.Errorf("slot %d candidate %d: occurrence %d, want %d", c.slot, c.cand, got, c.want)
		}
	}
	// an unchosen slot does not count
	if got := charmOcc(0b101, 2, 7, &bufs, &ns); got != 1 {
		t.Errorf("with slot 1 unchosen: occurrence %d, want 1", got)
	}
	a := &kop.Ability{NTargets: 3, TgtModes: [kop.MaxTargets]uint8{1, 2, 4}}
	if m, n := charmSlots(a, 5); m != 0b101 || n != 2 {
		t.Errorf("charmSlots = %b, %d, want 101, 2", m, n)
	}
}

// charmnumPickMode applies the mode option for m.
func charmnumPickMode(t *testing.T, sc *scen, m uint8) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkMode && a.Mode() == m {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("mode %d not offered: %+v", m, d.Options[:d.N])
}

// charmnumOrderTable makes TestCharm's two modes share one creature target:
// mode 0 destroys it, mode 1 regenerates it. Printed order kills the creature;
// answer order (regenerate first) would save it.
func charmnumOrderTable() *kop.Table {
	t := *v.Table
	t.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	a := t.Cards[v.TestCharm].Abilities[0]
	a.NModes, a.ModesPick, a.ModesFew = 2, 2, 0
	a.Targets = [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject,
		Filter: v.Table.Cards[v.PressurePoint].Abilities[0].Targets[0].Filter}}
	a.NTargets = 1
	a.TgtModes = [kop.MaxTargets]uint8{3}
	a.Prog = []kop.Instr{
		{Op: kop.OpCharm, A: kop.Const(1), C: 0},
		{Op: kop.OpDestroy, Obj: kop.RefTarget0},
		{Op: kop.OpCharm, A: kop.Const(1), C: 1},
		{Op: kop.OpRegenerate, Obj: kop.RefTarget0},
	}
	t.Cards[v.TestCharm].Abilities = []kop.Ability{a}
	return &t
}

// A reversed pick still resolves in printed order: mode 0 destroys the
// creature before mode 1's regeneration shield can save it.
func TestCharmnumPrintedOrder(t *testing.T) {
	sc := newScenTable(t, charmnumOrderTable(), Config{NoElide: true})
	sc.lands(0, v.Mountain, 1)
	bear := sc.perm(v.GrizzlyBears, 1)
	fc := sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, uint8(fc), 0xff)
	charmnumPickMode(t, sc, 1) // the answer picks the later printed mode first
	charmnumPickMode(t, sc, 0)
	sc.do(t, AkTarget, 0xff, uint8(bear))
	sc.resolveAll(t)
	if sc.s.objs[bear].zone != kop.ZoneGraveyard {
		t.Fatalf("bear zone %d, want the graveyard: the modes resolved in answer order", sc.s.objs[bear].zone)
	}
}

// charmnumSharedTable makes TestCharm's two modes target two slots that both
// admit any creature, so one creature is offered twice (occurrences 0 and 1).
func charmnumSharedTable() *kop.Table {
	t := *v.Table
	t.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	a := t.Cards[v.TestCharm].Abilities[0]
	a.NModes, a.ModesPick, a.ModesFew = 2, 2, 0
	anyCreature := v.Table.Cards[v.PressurePoint].Abilities[0].Targets[0].Filter
	a.Targets = [kop.MaxTargets]kop.TargetSpec{
		{Kind: kop.TgtObject, Filter: anyCreature},
		{Kind: kop.TgtObject, Filter: anyCreature},
	}
	a.NTargets = 2
	a.TgtModes = [kop.MaxTargets]uint8{1, 2}
	a.Prog = []kop.Instr{
		{Op: kop.OpCharm, A: kop.Const(1), C: 0},
		{Op: kop.OpPumpUntilEOT, Obj: kop.RefTarget0, A: kop.Const(1), B: kop.Const(1)},
		{Op: kop.OpCharm, A: kop.Const(1), C: 1},
		{Op: kop.OpPumpUntilEOT, Obj: kop.RefTarget1, A: kop.Const(1), B: kop.Const(1)},
	}
	t.Cards[v.TestCharm].Abilities = []kop.Ability{a}
	return &t
}

// The combined ask offers a shared creature once per chosen slot, and a pick
// tagged occurrence 1 lands in slot 1 even when it is applied first.
func TestCharmnumCombinedTargetsScenario(t *testing.T) {
	sc := newScenTable(t, charmnumSharedTable(), Config{NoElide: true})
	sc.lands(0, v.Mountain, 1)
	bear := sc.perm(v.GrizzlyBears, 1)
	fc := sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, uint8(fc), 0xff)
	charmnumPickMode(t, sc, 0)
	charmnumPickMode(t, sc, 1)
	// The combined step is the generated path; call its two halves directly,
	// as the vanilla table has no generated build state.
	var d Decision
	sc.s.charmTargetsStep(sc.k, &d)
	if d.Kind != DkTarget {
		t.Fatalf("combined target ask kind %d, want DkTarget", d.Kind)
	}
	var occ1 Action
	seen0, seen1 := false, false
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != AkTarget || a.Target() != TCode(uint8(bear)) {
			continue
		}
		switch a.Mode() {
		case 0:
			seen0 = true
		case 1:
			occ1, seen1 = a, true
		}
	}
	if !seen0 || !seen1 {
		t.Fatalf("shared creature occurrences 0/1 offered %v/%v, want both: %+v", seen0, seen1, d.Options[:d.N])
	}
	sc.s.applyCharmTarget(sc.k, occ1)
	if sc.s.bld().e.tgt[1] != uint8(bear) || TCode(sc.s.bld().e.tgt[0]) != NoTarget {
		t.Fatalf("after occurrence 1: tgt %v, want slot 1 only", sc.s.bld().e.tgt)
	}
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkTarget && a.Target() == TCode(uint8(bear)) && a.Mode() == 0 {
			sc.s.applyCharmTarget(sc.k, a)
			break
		}
	}
	if sc.s.bld().e.tgt[0] != uint8(bear) || sc.s.bld().e.tgt[1] != uint8(bear) {
		t.Fatalf("after occurrence 0: tgt %v, want both slots", sc.s.bld().e.tgt)
	}
}
