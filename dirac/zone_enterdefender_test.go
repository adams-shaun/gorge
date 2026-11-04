package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 508.4 defender of a creature put onto the battlefield attacking is
// the opponent player (gorge's fallback) or, when two or more defenders are
// legal, the one the controller picks: gorge's entering_defender ask. The
// candidates are gorge enteringDefenders' list: the opponent player, then the
// opponent's face-up planeswalkers in battlefield order, then every battle the
// opponent protects (CR 310.7).

// enterDefenderScen builds the scenario: a spell whose single instruction is
// an OpReturnEx with MvAttacking, a graveyard creature to return, and the
// step/active player set for combat.
func enterDefenderScen(t *testing.T, mk func(*kop.Table, *[4]CardID) []kop.Instr) (*scen, []kop.Instr, Obj, [4]CardID) {
	t.Helper()
	var ids [4]CardID
	sc, prog := czScenWith(t, func(tbl *kop.Table) []kop.Instr {
		ids = [4]CardID{CardID(len(tbl.Cards)), CardID(len(tbl.Cards) + 1), CardID(len(tbl.Cards) + 2), CardID(len(tbl.Cards) + 3)}
		tbl.Cards = append(tbl.Cards,
			kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
			kop.CardDef{Name: "Test Walker A", Types: kop.TPlaneswalker, Loyalty: 3},
			kop.CardDef{Name: "Test Walker B", Types: kop.TPlaneswalker, Loyalty: 2},
			kop.CardDef{Name: "Test Battle", Types: kop.TBattle, Defense: 3})
		return mk(tbl, &ids)
	})
	o := sc.grave(ids[0], 0, 1)[0]
	sc.s.res.e.obj = o
	sc.s.step = kop.StepAttackers
	sc.s.active, sc.s.prio = 0, 0
	return sc, prog, o, ids
}

// TestEnteringDefenderCandidates checks the candidate order and the fallback:
// with no other defender the creature attacks the opponent player without an
// ask; one planeswalker, a second planeswalker and a protected battle are
// then listed in gorge's order.
func TestEnteringDefenderCandidates(t *testing.T) {
	mk := func(*kop.Table, *[4]CardID) []kop.Instr {
		return []kop.Instr{{Op: kop.OpReturnEx, Obj: kop.RefSelf, C: int16(int(kop.ZoneBattlefield) | kop.MvAttacking<<3)}}
	}
	sc, prog, o, ids := enterDefenderScen(t, mk)
	s := sc.s
	s.exec(sc.k, &prog[0])
	if s.res.ask != raNone {
		t.Fatalf("ask %d, want none with only the opponent player to attack", s.res.ask)
	}
	if p := &s.perms[sc.slot(o)]; p.flags&pfAttacking == 0 || p.atk != PlayerTarget0+1 {
		t.Fatalf("atk %d attacking %v, want the opponent player with no ask", p.atk, p.flags&pfAttacking != 0)
	}

	// a second creature, now with two planeswalkers (seat 1) and a battle the
	// opponent protects (seat 0) on the battlefield
	sc, prog, o, ids = enterDefenderScen(t, mk)
	s = sc.s
	wa, wb := sc.perm(ids[1], 1), sc.perm(ids[2], 1)
	bat := sc.perm(ids[3], 0)
	if _, ok := s.protector(sc.slot(bat)); !ok {
		t.Fatal("the battle has no protector")
	}
	s.exec(sc.k, &prog[0])
	if s.res.ask != raGenEnterDefender || s.res.chosen != uint8(sc.slot(o)) {
		t.Fatalf("ask %d slot %d, want the CR 508.4 defender ask for slot %d", s.res.ask, s.res.chosen, sc.slot(o))
	}
	var d Decision
	if !s.resolveOptionsGen(sc.k, &d) || d.Sub != ChGenEnterDefender || d.N != 4 {
		t.Fatalf("options sub %d n %d, want the entering-defender ask with four options", d.Sub, d.N)
	}
	want := []uint8{PlayerTarget0 + 1, uint8(wa), uint8(wb), uint8(bat)}
	for i, w := range want {
		if d.Options[i].Source() != w {
			t.Fatalf("option %d = %d, want %d (%v)", i, d.Options[i].Source(), w, want)
		}
	}
	if s.perms[sc.slot(o)].flags&pfAttacking != 0 {
		t.Fatal("the creature attacked before the ask was answered")
	}

	// a battle the entrant's own controller protects is not a candidate
	sc, prog, o, ids = enterDefenderScen(t, mk)
	s = sc.s
	sc.perm(ids[3], 1) // protector is seat 0, the entrant's controller
	s.exec(sc.k, &prog[0])
	if s.res.ask != raNone {
		t.Fatalf("ask %d, want none for a battle the controller protects", s.res.ask)
	}
	if p := &s.perms[sc.slot(o)]; p.atk != PlayerTarget0+1 {
		t.Fatalf("atk %d, want the opponent player", p.atk)
	}
}

// TestEnteringDefenderAnswers checks both answers: the opponent player and a
// planeswalker, each setting the entered permanent's attacking defender.
func TestEnteringDefenderAnswers(t *testing.T) {
	mk := func(*kop.Table, *[4]CardID) []kop.Instr {
		return []kop.Instr{{Op: kop.OpReturnEx, Obj: kop.RefSelf, C: int16(int(kop.ZoneBattlefield) | kop.MvAttacking<<3)}}
	}
	for _, pickWalker := range []bool{false, true} {
		sc, prog, o, ids := enterDefenderScen(t, mk)
		s := sc.s
		walk := sc.perm(ids[1], 1)
		s.exec(sc.k, &prog[0])
		if s.res.ask != raGenEnterDefender {
			t.Fatalf("pickWalker=%v: ask %d, want the CR 508.4 defender ask", pickWalker, s.res.ask)
		}
		var d Decision
		s.resolveOptionsGen(sc.k, &d)
		a := d.Options[0]
		want := uint8(PlayerTarget0 + 1)
		if pickWalker {
			a, want = d.Options[1], uint8(walk)
		}
		s.applyAskCzParams(sc.k, a)
		p := &s.perms[sc.slot(o)]
		if p.flags&pfAttacking == 0 || p.atk != want || p.bord != s.nAttackers-1 {
			t.Fatalf("pickWalker=%v: atk %d bord %d attacking %v, want atk %d", pickWalker, p.atk, p.bord, p.flags&pfAttacking != 0, want)
		}
	}
}
