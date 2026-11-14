package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDelayedDestroyParksOrderAsk: a delayed trigger's AtEOT destroy whose move
// two exile-instead replacements compete for parks for the CR 616.1 order ask:
// gorge leaves the creature where it is, asks its controller which applies, and
// moves it after the answer. Before the park the destroy happened at once and
// dieZone stalled by name (StallReplLeave).
func TestDelayedDestroyParksOrderAsk(t *testing.T) {
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 8},
	}, func(tbl *kop.Table, id map[string]CardID) {
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Link", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell,
				Prog: []kop.Instr{{Op: kop.OpExileUntilLeaves, Obj: kop.RefTarget0}}}}})
	})
	s, k := sc.s, sc.k
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	bears := sc.perm(id["Grizzly Bears"], 1)
	if !s.graveMoveCompetes(k, bears) {
		t.Fatal("the destroy's move does not compete: nothing to park on")
	}
	e := stackEntry{kind: seTrigger, obj: bears, srcVer: s.objs[bears].ver, card: s.objs[bears].card,
		ab: abDelayed, ctrl: 1, trigObj: bears, trigPl: 1, trigAmt: int16(kop.OpDestroy)}
	s.resolveDelayed(k, &e)
	if s.Stalled() {
		t.Fatalf("the destroy stalled: %s", StallName(s.StallReason()))
	}
	if !s.parkedWaiting() || s.cold.parked[0].obj != bears {
		t.Fatal("the destroy is not parked for its order ask")
	}
	if z := s.objs[bears].zone; z != kop.ZoneBattlefield {
		t.Fatalf("the bears are in zone %d before the answer, want the battlefield", z)
	}
	var d Decision
	s.exileOrderOptions(k, &d)
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.N != 2 || d.Player != 1 {
		t.Fatalf("decision kind %d sub %d n %d player %d, want a two-source order ask of player 1", d.Kind, d.Sub, d.N, d.Player)
	}
	s.applyExileOrder(k, d.Options[0])
	if z := s.objs[bears].zone; z != kop.ZoneExile {
		t.Fatalf("the bears are in zone %d, want exile", z)
	}
	if s.Stalled() {
		t.Fatalf("the answer stalled: %s", StallName(s.StallReason()))
	}
}

// TestDelayedDestroyAtEndStepAsksOrder: the same destroy reached through the
// game's own flow: the next-end-step record fires, its trigger resolves, and
// the controller is asked the CR 616.1 order before the creature moves.
func TestDelayedDestroyAtEndStepAsksOrder(t *testing.T) {
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 8},
	}, nil)
	s := sc.s
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	bears := sc.perm(id["Grizzly Bears"], 1)
	s.dly[0] = delayed{act: uint8(kop.OpDestroy), ctrl: 1, obj: bears, ver: objWord(s.objs[bears].ver), creator: bears}
	s.ndly = 1
	var ask *Decision
	for i := 0; i < 60 && ask == nil; i++ {
		d := s.Step()
		if s.Stalled() {
			t.Fatalf("stalled before the ask: %s", StallName(s.StallReason()))
		}
		switch {
		case d.Kind == DkChoose && d.Sub == ChGenReplOrder:
			dd := d
			ask = &dd
		case d.Kind == DkPriority:
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		default:
			t.Fatalf("unexpected decision kind %d sub %d at step %d", d.Kind, d.Sub, s.step)
		}
	}
	if ask == nil {
		t.Fatal("no order ask came")
	}
	if z := s.objs[bears].zone; z != kop.ZoneBattlefield {
		t.Fatalf("the bears are in zone %d at the ask, want the battlefield", z)
	}
	s.Apply(ask.Options[0])
	if z := s.objs[bears].zone; z != kop.ZoneExile || s.Stalled() {
		t.Fatalf("after the answer: zone %d, stalled %v", z, s.Stalled())
	}
}
