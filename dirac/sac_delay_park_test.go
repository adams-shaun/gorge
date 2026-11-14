package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDelayedSacrificeParksOrderAsk: a delayed trigger's sacrifice (an evoke's
// "sacrifice it" line) whose move two exile-instead replacements compete for
// parks for the CR 616.1 order ask: gorge leaves the creature where it is,
// asks its controller which applies, and makes the sacrifice with the move,
// after the answer. The table holds an until-leaves program, as any table with
// an exile-until-leaves effect does, but nothing is held, so the parked move
// cannot mis-order a return. Before the park the sacrifice happened at once
// and dieZone stalled by name (StallReplLeave).
func TestDelayedSacrificeParksOrderAsk(t *testing.T) {
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 8},
	}, func(tbl *kop.Table, id map[string]CardID) {
		// an until-leaves program somewhere, so anyLinkRet is true and
		// holdOpen opens, as Sheltered by Ghosts's exile makes it do
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Link", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell,
				Prog: []kop.Instr{{Op: kop.OpExileUntilLeaves, Obj: kop.RefTarget0}}}}})
	})
	s, k := sc.s, sc.k
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	bears := sc.perm(id["Grizzly Bears"], 1)
	if !k.gen.anyLinkRet {
		t.Fatal("the table has no until-leaves program")
	}
	if !s.graveMoveCompetes(k, bears) {
		t.Fatal("the sacrifice's move does not compete: nothing to park on")
	}
	e := stackEntry{kind: seTrigger, obj: bears, srcVer: s.objs[bears].ver, card: s.objs[bears].card,
		ab: abDelayed, ctrl: 1, trigObj: bears, trigPl: 1, trigAmt: int16(kop.OpSacrifice)}
	s.resolveDelayed(k, &e)
	if s.Stalled() {
		t.Fatalf("the sacrifice stalled: %s", StallName(s.StallReason()))
	}
	if !s.parkedWaiting() || s.cold.parked[0].obj != bears || s.cold.parked[0].kind != parkMass || s.cold.parked[0].fl&parkSacd == 0 {
		t.Fatal("the sacrifice is not parked for its order ask")
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
	if n := s.pl[1].sac[sacAll]; n != 1 {
		t.Fatalf("%d sacrifices tallied, want 1", n)
	}
	if s.Stalled() {
		t.Fatalf("the answer stalled: %s", StallName(s.StallReason()))
	}
}

// TestDelayedSacrificeLinkStalls: the sacrificed permanent holds an
// until-leaves link (it exiled a card until it leaves), so gorge returns that
// card after the whole departure batch — with the parked move, after the ask.
// The kernel cannot place the return once its frame has closed
// (sacReturnHold), so the sacrifice happens now and dieZone stalls by name,
// as before the park.
func TestDelayedSacrificeLinkStalls(t *testing.T) {
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 8},
	}, func(tbl *kop.Table, id map[string]CardID) {
		// an until-leaves program somewhere, so anyLinkRet is true and
		// holdOpen opens, as Sheltered by Ghosts's exile makes it do
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Link", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell,
				Prog: []kop.Instr{{Op: kop.OpExileUntilLeaves, Obj: kop.RefTarget0}}}}})
	})
	s, k := sc.s, sc.k
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	bears := sc.perm(id["Grizzly Bears"], 1)
	s.g.links[0] = struct {
		src  Obj
		sver uint8
		obj  Obj
		ts   uint16
	}{src: bears, sver: s.objs[bears].ver - 1, obj: bears, ts: s.ts}
	s.g.nlinks = 1
	e := stackEntry{kind: seTrigger, obj: bears, srcVer: s.objs[bears].ver, card: s.objs[bears].card,
		ab: abDelayed, ctrl: 1, trigObj: bears, trigPl: 1, trigAmt: int16(kop.OpSacrifice)}
	s.resolveDelayed(k, &e)
	if !s.Stalled() || s.StallReason() != StallReplLeave {
		t.Fatalf("stalled %v reason %s, want StallReplLeave", s.Stalled(), StallName(s.StallReason()))
	}
}
