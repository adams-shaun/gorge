package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// incubateScen builds the incubate pool's table (the Incubator token and
// its back face) plus a CounterAddedOnce watcher, at the start of seat 0's
// main phase 1.
func incubateScen(t *testing.T) (*scen, CardID) {
	t.Helper()
	incubateSet := cardsettest.Set(t, "incubate")
	tbl, err := incubateSet.Pool()
	if err != nil {
		t.Fatal(err)
	}
	var plains CardID
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plains = CardID(i)
		}
	}
	if tbl.Cards[plains].Name != "Plains" {
		t.Fatal("incubate pool has no Plains")
	}
	watcher := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Counter Watcher",
		Types: kop.TCreature,
		Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{
			Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigCounterAddedOnce},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}},
		}},
	})
	sc := newScenTableLib(t, tbl, plains, Config{NoElide: true})
	sc.perm(watcher, 0)
	sc.s.res.e.ctrl = 0
	if !sc.s.subs[kop.TrigCounterAddedOnce].any() {
		t.Fatal("watcher not subscribed")
	}
	front := CardID(0)
	found := false
	for i := range tbl.Cards {
		if tbl.Cards[i].FaceMode == kop.FaceTransform && tbl.Cards[i].Face == 0 {
			front, found = CardID(i), true
		}
	}
	if !found {
		t.Fatal("incubate pool has no Incubator Token")
	}
	return sc, front
}

func (sc *scen) incubate(t *testing.T, front CardID, n, times int) {
	t.Helper()
	in := kop.Instr{Op: kop.OpIncubate, Obj: kop.RefController, A: kop.Const(n), B: kop.Const(times), C: int16(front)}
	sc.s.incubate(sc.k, &in)
}

func (sc *scen) incubatorTokens(front CardID) (out []Obj) {
	for o := Obj(0); int(o) < int(sc.s.nobj); o++ {
		if sc.s.objs[o].zone == kop.ZoneBattlefield && sc.s.objs[o].card == front {
			out = append(out, o)
		}
	}
	return out
}

// TestIncubateMintsWithCounters: A +1/+1 counters on the mint, one
// CounterAddedOnce event.
func TestIncubateMintsWithCounters(t *testing.T) {
	sc, front := incubateScen(t)
	sc.incubate(t, front, 2, 1)
	toks := sc.incubatorTokens(front)
	if len(toks) != 1 {
		t.Fatalf("%d Incubator tokens, want 1", len(toks))
	}
	sl := sc.slot(toks[0])
	if n := sc.s.perms[sl].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("+1/+1 count %d, want 2", n)
	}
	if pw, tg := sc.s.pt(sc.k, sl); pw != 2 || tg != 2 {
		t.Fatalf("P/T %d/%d, want 2/2", pw, tg)
	}
	if sc.s.npend != 1 {
		t.Fatalf("%d pending triggers, want 1 (one counter event)", sc.s.npend)
	}
}

// TestIncubateZeroMintsNoCounterEvent: A = 0 still mints, with no counter
// event.
func TestIncubateZeroMintsNoCounterEvent(t *testing.T) {
	sc, front := incubateScen(t)
	sc.incubate(t, front, 0, 1)
	toks := sc.incubatorTokens(front)
	if len(toks) != 1 {
		t.Fatalf("%d Incubator tokens, want 1", len(toks))
	}
	if n := sc.s.perms[sc.slot(toks[0])].ctrLow(int(kop.CtrP1P1)); n != 0 {
		t.Fatalf("+1/+1 count %d, want 0", n)
	}
	if sc.s.npend != 0 {
		t.Fatalf("%d pending triggers, want none (no counter event)", sc.s.npend)
	}
}

// TestIncubateNegativeCreatesNothing: a negative A creates nothing.
func TestIncubateNegativeCreatesNothing(t *testing.T) {
	sc, front := incubateScen(t)
	sc.incubate(t, front, -1, 1)
	if toks := sc.incubatorTokens(front); len(toks) != 0 {
		t.Fatalf("%d Incubator tokens, want none", len(toks))
	}
	if sc.s.npend != 0 {
		t.Fatalf("%d pending triggers, want none", sc.s.npend)
	}
}

// TestIncubateTimesRepeats: B mints, each with its own counter event.
func TestIncubateTimesRepeats(t *testing.T) {
	sc, front := incubateScen(t)
	sc.incubate(t, front, 1, 2)
	toks := sc.incubatorTokens(front)
	if len(toks) != 2 {
		t.Fatalf("%d Incubator tokens, want 2", len(toks))
	}
	for _, o := range toks {
		if n := sc.s.perms[sc.slot(o)].ctrLow(int(kop.CtrP1P1)); n != 1 {
			t.Fatalf("+1/+1 count %d, want 1", n)
		}
	}
	if sc.s.npend != 2 {
		t.Fatalf("%d pending triggers, want 2 (one counter event per mint)", sc.s.npend)
	}
}

// TestIncubateDoubledMintsEachGetCounters: Doubling Season doubles the
// mints and the counters on each, one counter event per mint.
func TestIncubateDoubledMintsEachGetCounters(t *testing.T) {
	sc, front := incubateScen(t)
	sc.perm(sc.idByName(t, "Doubling Season"), 0)
	sc.incubate(t, front, 1, 1)
	toks := sc.incubatorTokens(front)
	if len(toks) != 2 {
		t.Fatalf("%d Incubator tokens, want 2 (Doubling Season)", len(toks))
	}
	for _, o := range toks {
		if n := sc.s.perms[sc.slot(o)].ctrLow(int(kop.CtrP1P1)); n != 2 {
			t.Fatalf("+1/+1 count %d, want 2 (Doubling Season)", n)
		}
	}
	if sc.s.npend != 2 {
		t.Fatalf("%d pending triggers, want 2 (one event per mint)", sc.s.npend)
	}
}

func (sc *scen) idByName(t *testing.T, name string) CardID {
	t.Helper()
	for i := range sc.k.t.Cards {
		if sc.k.t.Cards[i].Name == name {
			return CardID(i)
		}
	}
	t.Fatalf("%s not in the table", name)
	return 0
}

// TestIncubatorTransformKeepsCounters: the token's {2}: Transform flips it
// to its back face in place, keeps its counters, and restamps its layer
// timestamp (CR 613.7g); off the battlefield it is front face up again.
func TestIncubatorTransformKeepsCounters(t *testing.T) {
	sc, front := incubateScen(t)
	sc.incubate(t, front, 2, 1)
	toks := sc.incubatorTokens(front)
	if len(toks) != 1 {
		t.Fatalf("%d Incubator tokens, want 1", len(toks))
	}
	o := toks[0]
	sl := sc.slot(o)
	if sc.s.perms[sl].obj != o {
		t.Fatalf("slot object %d, want %d", sc.s.perms[sl].obj, o)
	}
	if sc.s.perms[sl].flags&pfSick == 0 {
		t.Fatal("new token not summoning-sick before the transform")
	}
	before := sc.s.lstamp[sl]
	ts := sc.s.ts
	sc.s.res.e.obj = o
	sc.s.transform(sc.k, &kop.Instr{Op: kop.OpTransform, Obj: kop.RefSelf})
	if sc.s.objs[o].card == front {
		t.Fatal("still the front face")
	}
	if sc.s.objs[o].card != sc.k.t.Cards[front].Faces[1] {
		t.Fatalf("face %d, want the back", sc.s.objs[o].card)
	}
	if sc.s.perms[sl].obj != o {
		t.Fatalf("slot object %d after the transform, want the same object %d", sc.s.perms[sl].obj, o)
	}
	if sc.s.perms[sl].flags&pfSick == 0 {
		t.Fatal("the transform cleared summoning sickness (CR 712.18: the object is unchanged)")
	}
	if n := sc.s.perms[sl].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("+1/+1 count %d, want 2 after transform", n)
	}
	if pw, tg := sc.s.pt(sc.k, sl); pw != 2 || tg != 2 {
		t.Fatalf("P/T %d/%d, want 2/2", pw, tg)
	}
	if sc.s.lstamp[sl] <= before || sc.s.ts <= ts {
		t.Fatalf("layer stamp %d (from %d), ts %d (from %d): want a restamp", sc.s.lstamp[sl], before, sc.s.ts, ts)
	}
	sc.s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	if sc.s.objs[o].card != front {
		t.Fatalf("off the battlefield: face %d, want the front", sc.s.objs[o].card)
	}
}

// TestTransformDFCTokenRestamps: a FaceTransform flip restamps the layer
// timestamp (CR 613.7g); a FaceModal flip does not.
func TestTransformDFCTokenRestamps(t *testing.T) {
	sc, front := incubateScen(t)
	sc.incubate(t, front, 1, 1)
	toks := sc.incubatorTokens(front)
	if len(toks) != 1 {
		t.Fatalf("%d Incubator tokens, want 1", len(toks))
	}
	o := toks[0]
	sl := sc.slot(o)
	before := sc.s.lstamp[sl]
	ts := sc.s.ts
	sc.s.res.e.obj = o
	sc.s.transform(sc.k, &kop.Instr{Op: kop.OpTransform, Obj: kop.RefSelf})
	if sc.s.lstamp[sl] <= before || sc.s.ts <= ts {
		t.Fatalf("FaceTransform: layer stamp %d (from %d), ts %d (from %d): want a restamp", sc.s.lstamp[sl], before, sc.s.ts, ts)
	}

	setstateSet := cardsettest.Set(t, "setstate")
	mtbl, err := setstateSet.Pool()
	if err != nil {
		t.Fatal(err)
	}
	var plains CardID
	var modal CardID = CardID(0xffff)
	for i := range mtbl.Cards {
		switch mtbl.Cards[i].Name {
		case "Plains":
			plains = CardID(i)
		}
		if mtbl.Cards[i].FaceMode == kop.FaceModal && mtbl.Cards[i].Face == 0 && len(mtbl.Cards[i].Faces) > 1 && modal == CardID(0xffff) {
			modal = CardID(i)
		}
	}
	if modal == CardID(0xffff) {
		t.Fatal("setstate pool has no modal DFC")
	}
	msc := newScenTableLib(t, mtbl, plains, Config{NoElide: true})
	mo := msc.perm(modal, 0)
	msl := msc.slot(mo)
	mbefore := msc.s.lstamp[msl]
	mts := msc.s.ts
	msc.s.res.e.obj = mo
	msc.s.transform(msc.k, &kop.Instr{Op: kop.OpTransform, Obj: kop.RefSelf})
	if msc.s.lstamp[msl] != mbefore || msc.s.ts != mts {
		t.Fatalf("FaceModal: layer stamp %d (from %d), ts %d (from %d): want no restamp", msc.s.lstamp[msl], mbefore, msc.s.ts, mts)
	}
}

// TestIncubatorFaceResetOffBattlefield: the token shows its front face again
// once it leaves the battlefield.
func TestIncubatorFaceResetOffBattlefield(t *testing.T) {
	sc, front := incubateScen(t)
	sc.incubate(t, front, 1, 1)
	toks := sc.incubatorTokens(front)
	if len(toks) != 1 {
		t.Fatalf("%d Incubator tokens, want 1", len(toks))
	}
	o := toks[0]
	sc.s.res.e.obj = o
	sc.s.transform(sc.k, &kop.Instr{Op: kop.OpTransform, Obj: kop.RefSelf})
	if sc.s.objs[o].card == front {
		t.Fatal("still the front face on the battlefield")
	}
	sc.s.leave(sc.k, sc.slot(o), kop.ZoneGraveyard, false)
	if sc.s.objs[o].card != front {
		t.Fatalf("off the battlefield: face %d, want the front", sc.s.objs[o].card)
	}
}
