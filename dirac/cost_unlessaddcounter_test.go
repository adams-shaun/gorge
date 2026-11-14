package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// addCounterScen builds the freeplay scenario with a synthetic resolving
// ability (the m11Scen pattern) and a Grizzly Bears lookup for a target.
func addCounterScen(t *testing.T, prog []kop.Instr) (*scen, CardID, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Island", Count: 20}, {Name: "Grizzly Bears", Count: 3}}
	tbl, da, _, err := cardsettest.Set(t, "freeplay").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	lib, ok := id["Island"]
	if !ok {
		t.Fatal("Island not in the freeplay matchup")
	}
	synth := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:      "Unless AddCounter Test",
		Types:     kop.TCreature,
		Power:     1,
		Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered, Prog: prog}},
	})
	return newScenTableLib(t, tbl, lib, Config{NoElide: true}), synth, id["Grizzly Bears"]
}

// addCounterStart sets the resolving source and, when target is not NoObj,
// its target slot 0.
func addCounterStart(sc *scen, synth CardID, source, target Obj) {
	m11Start(sc, synth, func(e *stackEntry) {
		e.obj, e.srcVer, e.card, e.ctrl = source, sc.s.objs[source].ver, synth, 0
		e.trigPl = 0
		if target != NoObj {
			e.nt, e.tgt[0], e.tver[0] = 1, TCode(target), sc.s.objs[target].ver
		}
	})
}

// unlessAddCounterInstr builds the kernel instruction the kgen build emits for
// an unless AddCounter<N/KIND/recipient>.
func unlessAddCounterInstr(ck kop.CounterKind, ref kop.Ref) kop.Instr {
	n, ok := kop.PayAddCounterB(ck, ref)
	if !ok {
		panic("unlessAddCounterInstr: recipient ref")
	}
	return kop.Instr{
		Op:  kop.OpUnlessSwitched,
		Obj: kop.RefController,
		A:   kop.Const(1),
		B:   kop.Const(n),
		C:   int16(kop.PayTapX | 1<<4), // skip the one body instruction when unpaid
	}
}

// packedB is the B.N the build emits for an unless AddCounter.
func packedB(ck kop.CounterKind, ref kop.Ref) int16 {
	n, _ := kop.PayAddCounterB(ck, ref)
	return int16(n)
}

// TestUnlessAddCounter: an unless AddCounter pays pay/decline, the switch's
// body follows a payment (the Crowd's mana, the Wall's second stun counter),
// a decline does neither, and a gone target makes the Wall's option
// unpayable as gorge's candidate filter does.
func TestUnlessAddCounter(t *testing.T) {
	t.Run("source and body", func(t *testing.T) {
		program := []kop.Instr{
			unlessAddCounterInstr(kop.CtrP1P1, kop.RefSelf),
			{Op: kop.OpAddMana, Obj: kop.RefController, A: kop.Const(1), C: int16(kop.ColC)},
		}
		sc, synth, _ := addCounterScen(t, program)
		source := sc.perm(synth, 0)
		addCounterStart(sc, synth, source, NoObj)
		if !sc.s.canPayGen(sc.k, 0, payKindAddCounter, 1, packedB(kop.CtrP1P1, kop.RefSelf), 0) {
			t.Fatal("source recipient not payable")
		}
		d := sc.s.Step()
		if d.Kind != DkChoose || d.N < 2 {
			t.Fatalf("decision kind %d options %d, want the pay/decline ask", d.Kind, d.N)
		}
		sc.s.Apply(MakeAction(AkChoose, 0, 0, 1))
		if n := sc.s.perms[sc.slot(source)].ctrN(sc.s, int(kop.CtrP1P1)); n != 1 {
			t.Errorf("paid: %d +1/+1 counters, want 1", n)
		}
		if p := sc.s.pl[0].pool[kop.C]; p != 1 {
			t.Errorf("paid: %d colourless mana, want the body's 1", p)
		}
	})

	t.Run("decline does neither", func(t *testing.T) {
		program := []kop.Instr{
			unlessAddCounterInstr(kop.CtrP1P1, kop.RefSelf),
			{Op: kop.OpAddMana, Obj: kop.RefController, A: kop.Const(1), C: int16(kop.ColC)},
		}
		sc, synth, _ := addCounterScen(t, program)
		source := sc.perm(synth, 0)
		addCounterStart(sc, synth, source, NoObj)
		d := sc.s.Step()
		if d.Kind != DkChoose || d.N < 2 {
			t.Fatalf("decision kind %d options %d, want the pay/decline ask", d.Kind, d.N)
		}
		sc.s.Apply(MakeAction(AkChoose, 0, 0, 0))
		if n := sc.s.perms[sc.slot(source)].ctrN(sc.s, int(kop.CtrP1P1)); n != 0 {
			t.Errorf("declined: %d +1/+1 counters, want 0", n)
		}
		if p := sc.s.pl[0].pool[kop.C]; p != 0 {
			t.Errorf("declined: %d colourless mana, want 0", p)
		}
	})

	t.Run("target and second counter", func(t *testing.T) {
		program := []kop.Instr{
			unlessAddCounterInstr(kop.CtrStun, kop.RefTarget0),
			{Op: kop.OpPutCounter, Obj: kop.RefSelf, A: kop.Const(1), C: int16(kop.CtrStun)},
		}
		sc, synth, bears := addCounterScen(t, program)
		source := sc.perm(synth, 0)
		target := sc.perm(bears, 1)
		addCounterStart(sc, synth, source, target)
		d := sc.s.Step()
		if d.Kind != DkChoose || d.N < 2 {
			t.Fatalf("decision kind %d options %d, want the pay/decline ask", d.Kind, d.N)
		}
		sc.s.Apply(MakeAction(AkChoose, 0, 0, 1))
		if n := sc.s.perms[sc.slot(target)].ctrN(sc.s, int(kop.CtrStun)); n != 1 {
			t.Errorf("target: %d stun counters, want 1", n)
		}
		if n := sc.s.perms[sc.slot(source)].ctrN(sc.s, int(kop.CtrStun)); n != 1 {
			t.Errorf("source: %d stun counters, want the body's 1", n)
		}
	})

	t.Run("gone target", func(t *testing.T) {
		program := []kop.Instr{
			unlessAddCounterInstr(kop.CtrStun, kop.RefTarget0),
			{Op: kop.OpPutCounter, Obj: kop.RefSelf, A: kop.Const(1), C: int16(kop.CtrStun)},
		}
		sc, synth, bears := addCounterScen(t, program)
		source := sc.perm(synth, 0)
		target := sc.perm(bears, 1)
		sc.s.objs[target].zone = kop.ZoneGraveyard // the target left before payment
		addCounterStart(sc, synth, source, target)
		if sc.s.canPayGen(sc.k, 0, payKindAddCounter, 1, packedB(kop.CtrStun, kop.RefTarget0), 0) {
			t.Fatal("gone target: the option is payable")
		}
		d := sc.s.Step()
		if d.Kind != DkChoose || d.N != 1 {
			t.Fatalf("gone target: decision kind %d options %d, want decline only", d.Kind, d.N)
		}
	})
}
