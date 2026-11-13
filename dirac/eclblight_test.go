package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPaySubCounterAny: a resolution PaySubCounter with CtrAll reads the
// source's total counters, and asks the kind only when several kinds remain
// (Slumbering Walker's RemoveAnyCounter<1/Any/CARDNAME>).
func TestPaySubCounterAny(t *testing.T) {
	sc, id := manaGateScen(t, []cardset.Entry{
		{Name: "Guiding Hydra", Count: 1},
		{Name: "Plains", Count: 20},
	})
	hydra := sc.perm(id["Guiding Hydra"], 0)
	manaGateRes(sc, hydra, 0)
	sc.k.t.Cards[id["Guiding Hydra"]].Abilities[0].Prog[0].B = kop.Const(int(kop.CtrAll))
	hp := &sc.s.perms[sc.slot(hydra)]
	hp.addCtrN(sc.s, int(kop.CtrP1P1), 1)
	if !sc.s.canPayGen(sc.k, 0, kop.PaySubCounter, 1, int16(kop.CtrAll), 0) {
		t.Error("one +1/+1 counter: CtrAll SubCounter unpayable")
	}
	if sc.s.canPayGen(sc.k, 0, kop.PaySubCounter, 2, int16(kop.CtrAll), 0) {
		t.Error("one +1/+1 counter: CtrAll SubCounter for two payable")
	}

	// One kind: no ask.
	in := kop.Instr{Op: kop.OpIfPaid, Obj: kop.RefController, A: kop.Const(1), B: kop.Const(int(kop.CtrAll)), C: int16(kop.PaySubCounter)}
	if sc.s.execPay(sc.k, &in) {
		t.Fatal("CtrAll SubCounter resolved instead of suspending")
	}
	sc.s.applyResolveGen(sc.k, MakeAction(AkChoose, 0, 0, 1))
	if hp.ctrLow(int(kop.CtrP1P1)) != 0 {
		t.Errorf("one kind: %d +1/+1 counters left, want 0", hp.ctrLow(int(kop.CtrP1P1)))
	}
	if sc.s.res.ask != raNone {
		t.Errorf("one kind: ask %d, want raNone", sc.s.res.ask)
	}

	// Two kinds: the first unit is asked, then the remaining one kind drains.
	sc.s.res.pc = 0
	hp.addCtrN(sc.s, int(kop.CtrP1P1), 2)
	hp.addCtrN(sc.s, int(kop.CtrStun), 1)
	in = kop.Instr{Op: kop.OpIfPaid, Obj: kop.RefController, A: kop.Const(2), B: kop.Const(int(kop.CtrAll)), C: int16(kop.PaySubCounter)}
	if sc.s.execPay(sc.k, &in) {
		t.Fatal("CtrAll SubCounter resolved instead of suspending")
	}
	sc.s.applyResolveGen(sc.k, MakeAction(AkChoose, 0, 0, 1))
	var d Decision
	if !sc.s.resolveOptionsGen(sc.k, &d) {
		t.Fatal("resolveOptionsGen failed")
	}
	if d.Sub != ChCostSubCtr || d.N != 2 {
		t.Fatalf("two kinds: sub %d options %d, want ChCostSubCtr with 2", d.Sub, d.N)
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == uint8(kop.CtrStun) {
			sc.s.applyResolveGen(sc.k, d.Options[i])
			break
		}
	}
	if hp.ctrLow(int(kop.CtrP1P1)) != 1 || hp.ctrLow(int(kop.CtrStun)) != 0 {
		t.Errorf("two kinds: +1/+1 %d stun %d left, want 1 and 0", hp.ctrLow(int(kop.CtrP1P1)), hp.ctrLow(int(kop.CtrStun)))
	}
}

// keclScen builds a eclblight scenario at seat 0's main phase 1 with an
// empty hand and the given cards in the table.
func keclScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	return keclScenCfg(t, deck, Config{NoElide: true})
}

func keclScenCfg(t *testing.T, deck []cardset.Entry, cfg Config) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "eclblight")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	lib, ok := id["Plains"]
	if !ok {
		t.Fatal("Plains not in the eclblight table")
	}
	return newScenTableLib(t, tbl, lib, cfg), id
}

// keclBuildDone steps the build through its remaining picks to the
// priority round (finishBuild pays the picks).
func keclBuildDone(t *testing.T, sc *scen) {
	t.Helper()
	for i := 0; i < 40; i++ {
		if sc.s.ctl == ctlPriority {
			return
		}
		if d := sc.s.Step(); d.Kind == DkPriority {
			return
		}
	}
	t.Fatal("build did not finish")
}

// keclResolveAll passes both seats until the stack is empty.
func keclResolveAll(t *testing.T, sc *scen) {
	t.Helper()
	for i := 0; i < 80; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			if sc.s.sp == 0 {
				return
			}
			continue
		}
		if d.Kind == DkChoose && d.Sub == ChGenCastFree {
			sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
			continue
		}
		t.Fatalf("unexpected decision kind %d sub %d while resolving", d.Kind, d.Sub)
	}
	t.Fatal("the stack did not empty")
}

// TestBlightCostPick: a Blight cost (CR 701.68) picks one creature the
// payer controls, the source included (Sting-Slinger's tap + Blight 1),
// and is unpayable with no creature (Bogslither's Embrace).
func TestBlightCostPick(t *testing.T) {
	sc, id := keclScen(t, []cardset.Entry{
		{Name: "Sting-Slinger", Count: 1},
		{Name: "Ray Fillet, Man Ray", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Mountain", Count: 20},
		{Name: "Plains", Count: 20},
		{Name: "Island", Count: 20},
	})
	st := sc.perm(id["Sting-Slinger"], 0)
	bear := sc.perm(id["Ray Fillet, Man Ray"], 0)
	sc.perm(id["Grizzly Bears"], 1)
	sc.lands(0, id["Mountain"], 2)
	sc.do(t, AkActivate, uint8(st), 0xff)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.N != 2 {
		t.Fatalf("blight pick kind %d n %d, want 2 candidates", d.Kind, d.N)
	}
	seen := map[uint8]bool{}
	for i := 0; i < int(d.N); i++ {
		seen[uint8(d.Options[i].Source())] = true
	}
	if !seen[uint8(st)] || !seen[uint8(bear)] {
		t.Fatalf("blight candidates %v, want the source and the bears", d.Options[:d.N])
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(bear), 0, 0))
	keclBuildDone(t, sc)
	if n := sc.s.perms[sc.slot(bear)].ctrLow(int(kop.CtrM1M1)); n != 1 {
		t.Fatalf("picked creature has %d -1/-1 counters, want 1", n)
	}
	if sc.s.perms[sc.slot(st)].ctrLow(int(kop.CtrM1M1)) != 0 {
		t.Error("unpicked source got a counter")
	}

	// No creature to blight: Bogslither's Embrace is not offered.
	sc2, id2 := keclScen(t, []cardset.Entry{
		{Name: "Bogslither's Embrace", Count: 1},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Swamp", Count: 20},
		{Name: "Plains", Count: 20},
	})
	sc2.perm(id2["Grizzly Bears"], 1)
	sp := sc2.hand(id2["Bogslither's Embrace"], 0)
	sc2.lands(0, id2["Swamp"], 2)
	d = sc2.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(sp)) {
			t.Fatalf("a Blight cost with no creature is offered: %v", d.Options[:d.N])
		}
	}
}

// TestBlightAltAddPayable: Bogslither's Embrace's additional cost "blight 1
// or pay {3}" offers each payable part, and the blight part is withheld
// with no creature.
func TestBlightAltAddPayable(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Bogslither's Embrace", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Swamp", Count: 20},
		{Name: "Plains", Count: 20},
	}
	sc, id := keclScenCfg(t, deck, Config{NoElide: true, GorgeAltAdd: true})
	bear := sc.perm(id["Grizzly Bears"], 0)
	tgt := sc.perm(id["Grizzly Bears"], 1)
	sc.lands(0, id["Swamp"], 5)
	sp := sc.hand(id["Bogslither's Embrace"], 0)
	sc.do(t, AkCast, uint8(sp), 0xff)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChAltAdd || d.N != 2 {
		t.Fatalf("altadd decision kind %d sub %d n %d, want both parts", d.Kind, d.Sub, d.N)
	}
	var blight Action
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Target() == 0 {
			blight = d.Options[i]
		}
	}
	if blight.Kind() == 0 {
		t.Fatalf("no blight part in %v", d.Options[:d.N])
	}
	sc.s.Apply(blight)
	sc.do(t, AkTarget, 0xff, uint8(tgt))
	keclBuildDone(t, sc)
	keclResolveAll(t, sc)
	if n := sc.s.perms[sc.slot(bear)].ctrLow(int(kop.CtrM1M1)); n != 1 {
		t.Errorf("blight part: %d -1/-1 counters, want 1", n)
	}
	if sc.onBF(tgt) {
		t.Error("the targeted creature was not exiled")
	}

	// No creature: only the {3} part is offered.
	sc2, id2 := keclScen(t, deck)
	tgt2 := sc2.perm(id2["Grizzly Bears"], 1)
	sc2.lands(0, id2["Swamp"], 5)
	sp2 := sc2.hand(id2["Bogslither's Embrace"], 0)
	sc2.do(t, AkCast, uint8(sp2), 0xff)
	d = sc2.s.Step()
	if d.Kind != DkChoose || d.Sub != ChAltAdd || d.N != 1 || d.Options[0].Target() != 1 {
		t.Fatalf("no creature: altadd %v, want the {3} part alone", d.Options[:d.N])
	}
	sc2.s.Apply(d.Options[0])
	sc2.do(t, AkTarget, 0xff, uint8(tgt2))
	keclBuildDone(t, sc2)
	keclResolveAll(t, sc2)
	if sc2.onBF(tgt2) {
		t.Error("the {3} part did not resolve the exile")
	}

	// No creature and no {3}: the spell is not offered at all.
	sc3, id3 := keclScen(t, deck)
	sc3.perm(id3["Grizzly Bears"], 1)
	sc3.lands(0, id3["Swamp"], 2)
	sp3 := sc3.hand(id3["Bogslither's Embrace"], 0)
	d = sc3.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(sp3)) {
			t.Fatalf("an unpayable either-or Blight spell is offered")
		}
	}
}

// TestBlightResolutionPay: Blighted Blackthorn's "you may blight 2. If you
// do, draw a card and lose 1 life" pays a Blight 2 as a resolution cost
// (OpIfPaid), and declining skips the payoff.
func TestBlightResolutionPay(t *testing.T) {
	sc, id := keclScen(t, []cardset.Entry{
		{Name: "Blighted Blackthorn", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Swamp", Count: 20},
		{Name: "Plains", Count: 20},
	})
	bt := sc.perm(id["Blighted Blackthorn"], 0)
	bear := sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Grizzly Bears"], 1)
	sc.s.fireObj(sc.k, kop.TrigETB, sc.slot(bt), bt, 0, 0)
	var d Decision
	for i := 0; i < 12; i++ {
		d = sc.s.Step()
		if d.Kind == DkChoose {
			break
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	if d.Kind != DkChoose || d.Sub != ChGenPay {
		t.Fatalf("enter trigger decision kind %d sub %d, want the pay ask", d.Kind, d.Sub)
	}
	if len(d.Options) < 2 {
		t.Fatalf("blight cost not offered: %v", d.Options[:d.N])
	}
	sc.s.Apply(d.Options[1]) // pay the Blight 2
	d = sc.s.Step()
	if d.Kind != DkChoose || d.N != 2 {
		t.Fatalf("blight pick kind %d n %d, want 2 candidates", d.Kind, d.N)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(bear), 0, 0))
	keclBuildDone(t, sc)
	keclResolveAll(t, sc)
	if sc.onBF(bear) {
		t.Fatal("a 2/2 with two -1/-1 counters survived")
	}
	if !sc.onBF(bt) {
		t.Fatal("the Blackthorn died")
	}
	if got := sc.s.pl[0].life; got != 19 {
		t.Errorf("life %d, want 19", got)
	}
	if got := sc.s.pl[0].handN; got != 1 {
		t.Errorf("hand %d, want the drawn card", got)
	}

	// Declining skips the blight and the payoff.
	sc2, id2 := keclScen(t, []cardset.Entry{
		{Name: "Blighted Blackthorn", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 20},
	})
	bt2 := sc2.perm(id2["Blighted Blackthorn"], 0)
	bear2 := sc2.perm(id2["Grizzly Bears"], 0)
	sc2.s.fireObj(sc2.k, kop.TrigETB, sc2.slot(bt2), bt2, 0, 0)
	for i := 0; i < 12; i++ {
		d = sc2.s.Step()
		if d.Kind == DkChoose {
			break
		}
		if d.Kind == DkPriority {
			sc2.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	if d.Kind != DkChoose || d.Sub != ChGenPay {
		t.Fatalf("second trigger decision kind %d sub %d", d.Kind, d.Sub)
	}
	sc2.s.Apply(d.Options[0]) // decline
	keclResolveAll(t, sc2)
	if n := sc2.s.perms[sc2.slot(bear2)].ctrLow(int(kop.CtrM1M1)); n != 0 {
		t.Errorf("decline: %d -1/-1 counters, want 0", n)
	}
	if got := sc2.s.pl[0].life; got != 20 {
		t.Errorf("decline: life %d, want 20", got)
	}
	if got := sc2.s.pl[0].handN; got != 0 {
		t.Errorf("decline: hand %d, want 0", got)
	}

	// The source itself is a legal Blight pick: with no other creature the
	// choice is unasked, the Blackthorn takes both counters and dies, and the
	// payoff still resolves.
	sc3, id3 := keclScen(t, []cardset.Entry{
		{Name: "Blighted Blackthorn", Count: 1},
		{Name: "Plains", Count: 20},
	})
	bt3 := sc3.perm(id3["Blighted Blackthorn"], 0)
	sc3.s.fireObj(sc3.k, kop.TrigETB, sc3.slot(bt3), bt3, 0, 0)
	for i := 0; i < 12; i++ {
		d = sc3.s.Step()
		if d.Kind == DkChoose {
			break
		}
		if d.Kind == DkPriority {
			sc3.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	if d.Kind != DkChoose || d.Sub != ChGenPay {
		t.Fatalf("source-only trigger decision kind %d sub %d", d.Kind, d.Sub)
	}
	if len(d.Options) < 2 {
		t.Fatalf("source-only Blight not offered: %v", d.Options[:d.N])
	}
	sc3.s.Apply(d.Options[1]) // pay the Blight 2
	keclBuildDone(t, sc3)
	keclResolveAll(t, sc3)
	if !sc3.onBF(bt3) {
		t.Fatal("the source-only Blackthorn left the battlefield")
	}
	if n := sc3.s.perms[sc3.slot(bt3)].ctrLow(int(kop.CtrM1M1)); n != 2 {
		t.Fatalf("source-only Blackthorn has %d -1/-1 counters, want 2", n)
	}
	if got := sc3.s.pl[0].life; got != 19 {
		t.Errorf("source-only: life %d, want 19", got)
	}
	if got := sc3.s.pl[0].handN; got != 1 {
		t.Errorf("source-only: hand %d, want the drawn card", got)
	}
}

// TestOpBlight: OpBlight's player blights N -- the Chaos Spewer's "pay {2}
// or blight 2" with several creatures, none and one, and High Perfect
// Morcant's each-opponent blight.
func TestOpBlight(t *testing.T) {
	// Several creatures (the Spewer is one too): the player is asked.
	sc, id := keclScen(t, []cardset.Entry{
		{Name: "Chaos Spewer", Count: 1},
		{Name: "Grizzly Bears", Count: 3},
		{Name: "Mountain", Count: 20},
		{Name: "Plains", Count: 20},
	})
	sp := sc.perm(id["Chaos Spewer"], 0)
	b0 := sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Grizzly Bears"], 0)
	sc.s.fireObj(sc.k, kop.TrigETB, sc.slot(sp), sp, 0, 0)
	var d Decision
	for i := 0; i < 12; i++ {
		d = sc.s.Step()
		if d.Kind == DkChoose {
			break
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	if d.Kind != DkChoose || d.Sub != ChGenPayMode {
		t.Fatalf("Spewer decision kind %d sub %d, want the pay ask", d.Kind, d.Sub)
	}
	sc.s.Apply(d.Options[0]) // decline the {2}: blight 2
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenBlightPick || d.N != 3 {
		t.Fatalf("blight ask kind %d sub %d n %d, want ChGenBlightPick with 3", d.Kind, d.Sub, d.N)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(b0), 0, 0))
	keclResolveAll(t, sc)
	if sc.onBF(b0) {
		t.Error("a 2/2 with two -1/-1 counters survived")
	}

	// No creature and one creature: unasked (execBlight's strict superset).
	sc2, id2 := keclScen(t, []cardset.Entry{
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 20},
	})
	sc2.s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpBlight, Obj: kop.RefController, A: kop.Const(2)}
	if !sc2.s.execBlight(sc2.k, &in) {
		t.Fatal("no creature: asked")
	}
	bear := sc2.perm(id2["Grizzly Bears"], 0)
	if !sc2.s.execBlight(sc2.k, &in) {
		t.Fatal("one creature: asked")
	}
	if n := sc2.s.perms[sc2.slot(bear)].ctrLow(int(kop.CtrM1M1)); n != 2 {
		t.Errorf("the lone candidate has %d -1/-1 counters, want 2", n)
	}

	// Each opponent: High Perfect Morcant blights the opponent's creature.
	sc3, id3 := keclScen(t, []cardset.Entry{
		{Name: "High Perfect Morcant", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 20},
	})
	hp := sc3.perm(id3["High Perfect Morcant"], 0)
	opp := sc3.perm(id3["Grizzly Bears"], 1)
	sc3.perm(id3["Grizzly Bears"], 1)
	sc3.s.fireObj(sc3.k, kop.TrigETB, sc3.slot(hp), hp, 0, 0)
	for i := 0; i < 12; i++ {
		d = sc3.s.Step()
		if d.Kind == DkChoose {
			break
		}
		if d.Kind == DkPriority {
			sc3.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	if d.Kind != DkChoose || d.Sub != ChGenBlightPick || d.N != 2 {
		t.Fatalf("Morcant ask kind %d sub %d n %d, want ChGenBlightPick with 2", d.Kind, d.Sub, d.N)
	}
	sc3.s.Apply(MakeAction(AkChoose, TCode(opp), 0, 0))
	keclResolveAll(t, sc3)
	if !sc3.onBF(opp) {
		t.Error("the opponent's 2/2 died to one -1/-1 counter")
	}
	if n := sc3.s.perms[sc3.slot(opp)].ctrLow(int(kop.CtrM1M1)); n != 1 {
		t.Errorf("the opponent's creature has %d -1/-1 counters, want 1", n)
	}
}

// TestSubCtrPickAbility: the counter costs the Standard carriers add --
// filtered SubCounter (Sunstar Chaplain), RemoveAnyCounter on the source
// (Brambleback Brute) and a self AddCounter (Jessica Jones).
func TestSubCtrPickAbility(t *testing.T) {
	// Sunstar Chaplain: {2}, remove a +1/+1 counter from a creature you
	// control: tap target artifact or creature.
	sc, id := keclScen(t, []cardset.Entry{
		{Name: "Sunstar Chaplain", Count: 1},
		{Name: "Grizzly Bears", Count: 3},
		{Name: "Plains", Count: 20},
		{Name: "Island", Count: 20},
	})
	ch := sc.perm(id["Sunstar Chaplain"], 0)
	b0 := sc.perm(id["Grizzly Bears"], 0)
	b1 := sc.perm(id["Grizzly Bears"], 0)
	tgt := sc.perm(id["Grizzly Bears"], 1)
	sc.lands(0, id["Plains"], 2)
	sc.s.perms[sc.slot(b0)].addCtrN(sc.s, int(kop.CtrP1P1), 1)
	sc.s.perms[sc.slot(b1)].addCtrN(sc.s, int(kop.CtrP1P1), 1)
	sc.s.perms[sc.slot(b1)].addCtrN(sc.s, int(kop.CtrStun), 1)
	sc.do(t, AkActivate, uint8(ch), 0xff)
	d := sc.s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("Sunstar activation decision kind %d, want the target ask", d.Kind)
	}
	sc.do(t, AkTarget, 0xff, uint8(tgt))
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChCostSubCtr || d.N != 2 {
		t.Fatalf("counter pick kind %d sub %d n %d, want two +1/+1 creatures", d.Kind, d.Sub, d.N)
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() != uint8(kop.CtrP1P1) {
			t.Errorf("offered counter kind %d, want +1/+1", d.Options[i].Mode())
		}
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(b0), 0, uint8(kop.CtrP1P1)))
	keclBuildDone(t, sc)
	keclResolveAll(t, sc)
	if n := sc.s.perms[sc.slot(b0)].ctrLow(int(kop.CtrP1P1)); n != 0 {
		t.Errorf("chosen creature has %d +1/+1 counters, want 0", n)
	}
	if sc.s.perms[sc.slot(tgt)].flags&pfTapped == 0 {
		t.Error("the target was not tapped")
	}

	// Brambleback Brute: {1}{R}, remove a counter from this creature:
	// target creature can't block. CtrAll asks the kind when two remain.
	sc2, id2 := keclScen(t, []cardset.Entry{
		{Name: "Brambleback Brute", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Mountain", Count: 20},
		{Name: "Plains", Count: 20},
	})
	br := sc2.perm(id2["Brambleback Brute"], 0)
	tgt2 := sc2.perm(id2["Grizzly Bears"], 1)
	sc2.lands(0, id2["Mountain"], 2)
	bp := &sc2.s.perms[sc2.slot(br)]
	bp.addCtrN(sc.s, int(kop.CtrStun), 1) // two kinds: -1/-1 from the replacement and stun
	sc2.do(t, AkActivate, uint8(br), 0xff)
	d = sc2.s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("Brute activation decision kind %d, want the target ask", d.Kind)
	}
	sc2.do(t, AkTarget, 0xff, uint8(tgt2))
	d = sc2.s.Step()
	if d.Kind != DkChoose || d.Sub != ChCostSubCtr || d.N != 2 {
		t.Fatalf("RemoveAnyCounter kind %d sub %d n %d, want two kinds", d.Kind, d.Sub, d.N)
	}
	sc2.s.Apply(MakeAction(AkChoose, TCode(br), 0, uint8(kop.CtrStun)))
	keclBuildDone(t, sc2)
	if bp.ctrLow(int(kop.CtrStun)) != 0 || bp.ctrLow(int(kop.CtrM1M1)) != 2 {
		t.Errorf("after removing stun: stun %d -1/-1 %d", bp.ctrLow(int(kop.CtrStun)), bp.ctrLow(int(kop.CtrM1M1)))
	}

	// Jessica Jones: {T}, put a stun counter on Jessica Jones.
	sc3, id3 := keclScen(t, []cardset.Entry{
		{Name: "Jessica Jones, Private Eye", Count: 1},
		{Name: "Plains", Count: 20},
	})
	jj := sc3.perm(id3["Jessica Jones, Private Eye"], 0)
	sc3.do(t, AkActivate, uint8(jj), 0xff)
	keclBuildDone(t, sc3)
	if n := sc3.s.perms[sc3.slot(jj)].ctrLow(int(kop.CtrStun)); n != 1 {
		t.Errorf("Jessica Jones has %d stun counters, want the cost's 1", n)
	}
}

// TestBlightCountersBeforeSacrifice: gorge emits a Blight cost's counters
// (cast_commit.go's emitBlightCostCounters) before emitSacrifices, so a
// source that is its own blight pick dies carrying them. Moving the
// counters after the sacrifice drops them (the pick is no longer on the
// battlefield).
func TestBlightCountersBeforeSacrifice(t *testing.T) {
	sc, id := keclScen(t, []cardset.Entry{
		{Name: "Sting-Slinger", Count: 1},
		{Name: "Mountain", Count: 20},
		{Name: "Plains", Count: 20},
	})
	st := sc.perm(id["Sting-Slinger"], 0)
	sl := sc.slot(st)
	cd := &sc.k.t.Cards[id["Sting-Slinger"]]
	cd.Abilities[0].Cost = kop.Cost{Flags: kop.CostSacSelf | kop.CostBlight, N: 1, Filter: kop.FilterID(1)}
	sc.do(t, AkActivate, uint8(st), 0xff)
	keclBuildDone(t, sc)
	if sc.onBF(st) {
		t.Fatal("the source was not sacrificed")
	}
	if n := sc.s.perms[sl].ctrLow(int(kop.CtrM1M1)); n != 1 {
		t.Fatalf("the blighted source died with %d -1/-1 counters, want 1 (counters precede the sacrifice)", n)
	}
}
