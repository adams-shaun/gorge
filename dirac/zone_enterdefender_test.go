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
	if p := &s.perms[sc.slot(o)]; p.flags&pfAttacking == 0 || TCode(p.atk) != PlayerTarget0+1 {
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
	if s.res.ask != raGenEnterDefender || s.res.chosen != uint8(NoSlot) || s.objs[o].zone != kop.ZoneGraveyard {
		t.Fatalf("ask %d slot %d zone %d, want the CR 508.4 defender ask posed before the card leaves its graveyard", s.res.ask, s.res.chosen, s.objs[o].zone)
	}
	var d Decision
	if !s.resolveOptionsGen(sc.k, &d) || d.Sub != ChGenEnterDefender || d.N != 4 {
		t.Fatalf("options sub %d n %d, want the entering-defender ask with four options", d.Sub, d.N)
	}
	want := []uint8{uint8(PlayerTarget0 + 1), uint8(wa), uint8(wb), uint8(bat)}
	for i, w := range want {
		if d.Options[i].Source() != TCode(w) {
			t.Fatalf("option %d = %d, want %d (%v)", i, d.Options[i].Source(), w, want)
		}
	}
	if s.objs[o].zone != kop.ZoneGraveyard {
		t.Fatal("the creature entered before the ask was answered")
	}

	// a battle the entrant's own controller protects is not a candidate
	sc, prog, o, ids = enterDefenderScen(t, mk)
	s = sc.s
	sc.perm(ids[3], 1) // protector is seat 0, the entrant's controller
	s.exec(sc.k, &prog[0])
	if s.res.ask != raNone {
		t.Fatalf("ask %d, want none for a battle the controller protects", s.res.ask)
	}
	if p := &s.perms[sc.slot(o)]; TCode(p.atk) != PlayerTarget0+1 {
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

// TestEnteringDefenderCloneReturn: a Clone-like card returned attacking waits in
// its graveyard for the clone election (cloneReturnPark); the CR 508.4 defender
// ask follows that answer when the opponent has a planeswalker, and its answer
// sets the defender of the entered copy.
func TestEnteringDefenderCloneReturn(t *testing.T) {
	var ids [3]CardID
	sc, prog := czScenWith(t, func(tbl *kop.Table) []kop.Instr {
		tbl.TypeVocab = []uint8{1, 2, 3}
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
		f := kop.FilterID(len(tbl.Filters) - 1)
		n := len(tbl.Cards)
		ids = [3]CardID{CardID(n), CardID(n + 1), CardID(n + 2)}
		tbl.Cards = append(tbl.Cards,
			kop.CardDef{Name: "Test Clone", Types: kop.TCreature,
				Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersProgram},
					Prog: []kop.Instr{{Op: kop.OpBecomeCopy, Obj: kop.RefReplaced, C: 1, F: f}}}}},
			kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
			kop.CardDef{Name: "Test Walker A", Types: kop.TPlaneswalker, Loyalty: 3})
		return []kop.Instr{{Op: kop.OpReturnEx, Obj: kop.RefSelf, C: int16(int(kop.ZoneBattlefield) | kop.MvAttacking<<3)}}
	})
	s := sc.s
	o := sc.grave(ids[0], 0, 1)[0]
	bear := sc.perm(ids[1], 0)
	walk := sc.perm(ids[2], 1)
	s.res.e.obj = o
	s.res.active = true
	s.step = kop.StepAttackers
	s.active, s.prio = 0, 0

	s.exec(sc.k, &prog[0])
	if s.res.ask != raGenEnterDefender || s.res.chosen != uint8(NoSlot) || s.objs[o].zone != kop.ZoneGraveyard {
		t.Fatalf("ask %d slot %d zone %d, want the CR 508.4 defender ask first, with the card in its graveyard", s.res.ask, s.res.chosen, s.objs[o].zone)
	}
	var d Decision
	if !s.resolveOptionsGen(sc.k, &d) || d.Sub != ChGenEnterDefender || d.N != 2 {
		t.Fatalf("options sub %d n %d, want the defender ask with two options", d.Sub, d.N)
	}
	s.applyResolveGen(sc.k, d.Options[1])
	if s.res.ask != raGenRetChoice || s.g.pre.kind != akClone {
		t.Fatalf("ask %d pre kind %d, want the clone election with the card in its graveyard", s.res.ask, s.g.pre.kind)
	}
	d = Decision{}
	if !s.resolveOptionsGen(sc.k, &d) || d.Sub != ChGenClone {
		t.Fatalf("options sub %d, want the clone election", d.Sub)
	}
	var pick Action
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Source() == TCode(uint8(bear)) {
			pick = d.Options[i]
		}
	}
	if pick == 0 {
		t.Fatal("the bear is not a clone option")
	}
	s.applyResolveGen(sc.k, pick)
	if s.res.ask == raGenEnterDefender {
		t.Fatal("the defender was asked a second time after the clone answer")
	}
	sl := s.objs[o].slot
	if p := &s.perms[sl]; !s.occ.has(sl) || p.obj != o || p.flags&pfAttacking == 0 || p.atk != uint8(walk) {
		t.Fatalf("atk %d attacking %v, want the entered clone attacking the planeswalker %d", s.perms[sl].atk, s.perms[sl].flags&pfAttacking != 0, walk)
	}
}
