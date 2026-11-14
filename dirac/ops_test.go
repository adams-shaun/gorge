package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

func (sc *scen) lands(p uint8, c CardID, n int) {
	for i := 0; i < n; i++ {
		sc.perm(c, p)
	}
}

func (sc *scen) passBoth(t *testing.T) {
	t.Helper()
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkPass, scAny, scAny)
}

func TestCancelCountersSpell(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.lands(0, v.Forest, 2)
	sc.lands(1, v.Island, 3)
	bears := sc.hand(v.GrizzlyBears, 0)
	cancel := sc.hand(v.Cancel, 1)
	sc.do(t, AkCast, TCode(bears), scAny)
	sc.do(t, AkPass, scAny, scAny) // seat 0 passes with bears on the stack
	sc.do(t, AkCast, TCode(cancel), scAny)
	sc.do(t, AkTarget, scAny, TCode(bears))
	sc.passBoth(t) // cancel resolves
	if sc.s.objs[bears].zone != kop.ZoneGraveyard || sc.s.objs[cancel].zone != kop.ZoneGraveyard {
		t.Fatalf("bears zone %d cancel zone %d", sc.s.objs[bears].zone, sc.s.objs[cancel].zone)
	}
	if sc.s.sp != 0 {
		t.Fatalf("stack not empty")
	}
}

func TestMindRotDiscardChoice(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.lands(0, v.Swamp, 3)
	mr := sc.hand(v.MindRot, 0)
	h1 := sc.hand(v.GrizzlyBears, 1)
	h2 := sc.hand(v.HillGiant, 1)
	h3 := sc.hand(v.Forest, 1)
	sc.do(t, AkCast, TCode(mr), scAny)
	sc.do(t, AkTarget, scAny, TCode(PlayerTarget1))
	sc.passBoth(t)
	sc.do(t, AkChoose, TCode(h3), scAny)
	sc.do(t, AkChoose, TCode(h1), scAny)
	if sc.s.objs[h1].zone != kop.ZoneGraveyard || sc.s.objs[h3].zone != kop.ZoneGraveyard || sc.s.objs[h2].zone != kop.ZoneHand {
		t.Fatalf("zones %d %d %d", sc.s.objs[h1].zone, sc.s.objs[h2].zone, sc.s.objs[h3].zone)
	}
}

func TestOptScryBottom(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.lands(0, v.Island, 1)
	opt := sc.hand(v.Opt, 0)
	pl := &sc.s.pl[0]
	top := pl.lib[pl.libLo]
	second := pl.lib[pl.libLo+1]
	sc.do(t, AkCast, TCode(opt), scAny)
	sc.passBoth(t)
	sc.do(t, AkDone, scAny, scAny) // keep nothing on top: the card goes to the bottom
	if pl.lib[pl.libHi-1] != top {
		t.Fatalf("scried card not on the bottom")
	}
	if sc.s.objs[second].zone != kop.ZoneHand {
		t.Fatalf("Opt did not draw the next card")
	}
}

func TestRaiseTheAlarmTokensAndAnthem(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.lands(0, v.Plains, 2)
	sc.perm(v.GloriousAnthem, 0)
	ra := sc.hand(v.RaiseTheAlarm, 0)
	sc.do(t, AkCast, TCode(ra), scAny)
	sc.passBoth(t)
	sc.s.Step()
	n := 0
	for m := sc.s.ctrlM[0]; m.any(); m = m.rest() {
		sl := m.first()
		if sc.s.cardAt(sl) == v.SoldierToken {
			n++
			if p, tg := sc.s.pt(sc.k, sl); p != 2 || tg != 2 {
				t.Fatalf("anthem soldier is %d/%d", p, tg)
			}
		}
	}
	if n != 2 {
		t.Fatalf("%d soldier tokens", n)
	}
}

func TestFieryCharmModes(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.lands(0, v.Mountain, 1)
	fc := sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, TCode(fc), scAny)
	// no creature on the board: only mode 1 (lose life) is feasible
	d := sc.s.Step()
	if d.Kind != DkModes || d.N != 1 || d.Options[0].Mode() != 1 {
		t.Fatalf("modes decision %+v", d.Options[:d.N])
	}
	sc.s.Apply(d.Options[0])
	sc.do(t, AkTarget, scAny, TCode(PlayerTarget1))
	sc.passBoth(t)
	sc.s.Step()
	if sc.s.pl[1].life != 18 {
		t.Fatalf("life %d", sc.s.pl[1].life)
	}
}

func TestPreyUponFight(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.lands(0, v.Forest, 1)
	sp := sc.perm(v.GiantSpider, 0) // 2/4
	bears := sc.perm(v.GrizzlyBears, 1)
	pu := sc.hand(v.PreyUpon, 0)
	sc.do(t, AkCast, TCode(pu), scAny)
	sc.do(t, AkTarget, scAny, TCode(sp))
	sc.do(t, AkTarget, scAny, TCode(bears))
	sc.passBoth(t)
	sc.s.Step()
	if sc.onBF(bears) || !sc.onBF(sp) || sc.s.perms[sc.slot(sp)].damage != 2 {
		t.Fatalf("fight result wrong")
	}
}

func TestElvishVisionaryETBDraw(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.lands(0, v.Forest, 2)
	ev := sc.hand(v.ElvishVisionary, 0)
	sc.do(t, AkCast, TCode(ev), scAny)
	sc.passBoth(t) // creature resolves, ETB trigger goes on the stack
	h := sc.s.pl[0].handN
	sc.passBoth(t) // trigger resolves
	sc.s.Step()
	if sc.s.pl[0].handN != h+1 {
		t.Fatalf("hand %d -> %d", h, sc.s.pl[0].handN)
	}
}

func TestMulliganRound(t *testing.T) {
	s := NewConfig(v.Table, v.Decks[0], v.Decks[1], 3, Config{Mulligans: 2, NoElide: true})
	first := s.startPl
	d := s.Step()
	if d.Kind != DkMulligan || d.Player != first || d.N != 2 {
		t.Fatalf("first decision %+v", d)
	}
	draws := s.RNGDraws()
	s.Apply(MakeAction(AkMulligan, 0, 0, 0))
	d = s.Step()
	if d.Player != 1-first {
		t.Fatalf("second seat not asked")
	}
	s.Apply(MakeAction(AkKeep, 0, 0, 0))
	// pass over: the mulliganed seat reshuffled (39 draws) and redrew 7
	if s.RNGDraws() != draws+39 || s.pl[first].handN != 7 {
		t.Fatalf("draws %d->%d hand %d", draws, s.RNGDraws(), s.pl[first].handN)
	}
	d = s.Step()
	if d.Kind != DkMulligan || d.Player != first {
		t.Fatalf("second pass not posed to the mulliganed seat")
	}
	s.Apply(MakeAction(AkKeep, 0, 0, 0))
	d = s.Step()
	if d.Kind != DkChoose || d.Player != first || d.N != 7 {
		t.Fatalf("bottom decision %+v", d.Kind)
	}
	o := Obj(d.Options[3].Source())
	s.Apply(d.Options[3])
	pl := &s.pl[first]
	if pl.lib[pl.libHi-1] != o || pl.handN != 6 {
		t.Fatalf("bottomed card not at the library bottom")
	}
	if s.turn != 1 || s.active != first {
		t.Fatalf("turn 1 did not begin for the starting player")
	}
}

func TestDelayedTriggerUsesTheStack(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	g := sc.perm(v.HillGiant, 0)
	sc.s.dly[0] = delayed{act: uint8(kop.OpSacrifice), ctrl: 0, obj: g, ver: objWord(sc.s.objs[g].ver), creator: NoObj}
	sc.s.ndly = 1
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepEnd)
	d := sc.s.Step()
	if d.Kind != DkPriority || sc.s.sp != 1 || sc.s.stack[0].ab != abDelayed || sc.s.stack[0].obj != g {
		t.Fatalf("delayed trigger not on the stack: sp %d", sc.s.sp)
	}
	var p Projection
	sc.s.Project(&p)
	if p.Stack[0].Kind != seTrigger || p.Stack[0].Ability != 255 || p.Stack[0].Source != g {
		t.Fatalf("projection %+v", p.Stack[0])
	}
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	sc.do(t, AkPass, scAny, scAny)
	sc.s.Step()
	if sc.onBF(g) {
		t.Fatalf("delayed sacrifice did not happen")
	}
}

// TestEmptyDelayedTriggerStillFires: a delayed trigger registered over an
// empty set (OpNop act, the source's own) goes on the stack at the end step
// and does nothing when it resolves (CR 603.7; gorge effDelayedTrigger).
func TestEmptyDelayedTriggerStillFires(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	g := sc.perm(v.HillGiant, 0)
	sc.s.dly[0] = delayed{act: uint8(kop.OpNop), ctrl: 0, obj: g, ver: objWord(sc.s.objs[g].ver), creator: NoObj}
	sc.s.ndly = 1
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepEnd)
	d := sc.s.Step()
	if d.Kind != DkPriority || sc.s.sp != 1 || sc.s.stack[0].ab != abDelayed || sc.s.stack[0].obj != g {
		t.Fatalf("empty delayed trigger not on the stack: sp %d", sc.s.sp)
	}
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	sc.do(t, AkPass, scAny, scAny)
	sc.s.Step()
	if !sc.onBF(g) || sc.s.sp != 0 {
		t.Fatalf("empty delayed trigger did something: onBF %v sp %d", sc.onBF(g), sc.s.sp)
	}
}
