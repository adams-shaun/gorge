package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// czScenWith is a Plains-deck scenario whose table gains one sorcery with
// the spell program mk builds (it may append cards and filters first), set
// up as the resolving entry (pc 0) of seat 0.
func czScenWith(t *testing.T, mk func(tbl *kop.Table) []kop.Instr) (*scen, []kop.Instr) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	prog := mk(tbl)
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Cz", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.s.res.e.ctrl, sc.s.res.e.card, sc.s.res.e.ab, sc.s.res.pc = 0, src, 0, 0
	sc.s.sp = 1
	return sc, prog
}

// czScen is czScenWith over a fixed program.
func czScen(t *testing.T, prog []kop.Instr) *scen {
	sc, _ := czScenWith(t, func(*kop.Table) []kop.Instr { return prog })
	return sc
}

// TestMayMillAsksBeforeMilling: OpMill with MvMayMill asks yes/no first
// (gorge mill_optional: yes then no) whenever the amount is above zero,
// even with an empty library; a no mills nothing.
func TestMayMillAsksBeforeMilling(t *testing.T) {
	mill := func(n int) []kop.Instr {
		return []kop.Instr{{Op: kop.OpMill, Obj: kop.RefController, A: kop.Const(n), C: kop.MvMayMill}}
	}
	ask := func(sc *scen, prog []kop.Instr) bool {
		return !sc.s.exec(sc.k, &prog[0]) && sc.s.res.ask == raGenMayMill
	}
	for _, answer := range []uint8{1, 0} {
		prog := mill(2)
		sc := czScen(t, prog)
		s := sc.s
		libN := s.pl[0].libLen()
		if !ask(sc, prog) {
			t.Fatalf("answer %d: no may-mill ask, res.ask %d", answer, s.res.ask)
		}
		var d Decision
		s.resolveOptionsGen(sc.k, &d)
		if d.Sub != ChGenConfirm || d.N != 2 || d.Options[0].Mode() != 1 || d.Options[1].Mode() != 0 {
			t.Fatalf("ask options: sub %d n %d, want confirm yes then no", d.Sub, d.N)
		}
		if s.pl[0].libLen() != libN || s.pl[0].graveN != 0 {
			t.Fatal("milled before the answer")
		}
		s.applyAskCzParams(sc.k, MakeAction(AkChoose, 0, 0, answer))
		want := 0
		if answer == 1 {
			want = 2
		}
		if int(s.pl[0].graveN) != want || s.pl[0].libLen() != libN-want {
			t.Fatalf("answer %d: graveyard %d library %d, want %d milled", answer, s.pl[0].graveN, s.pl[0].libLen(), want)
		}
	}
	prog := mill(0)
	sc := czScen(t, prog)
	if !sc.s.exec(sc.k, &prog[0]) || sc.s.res.ask == raGenMayMill {
		t.Fatal("A=0 asked")
	}
	prog = mill(2)
	sc = czScen(t, prog)
	pl := &sc.s.pl[0]
	pl.libLo = pl.libHi
	if !ask(sc, prog) {
		t.Fatal("an empty library must still be asked")
	}
}

// TestGraveChooseMoveConfirm: OpChooseMove from a graveyard with SrConfirm
// asks yes/no before the pick (gorge hidden_pick_confirm), also with no
// matching card; a no moves nothing and a yes goes on to the pick.
func TestGraveChooseMoveConfirm(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	build := func(withCard bool) (*scen, []kop.Instr, Obj) {
		deck := []cardset.Entry{{Name: "Plains", Count: 20}}
		tbl, _, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Zone: kop.ZoneGraveyard})
		f := kop.FilterID(len(tbl.Filters) - 1)
		prog := []kop.Instr{{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(1), F: f,
			C: int16(int(kop.ZoneHand) | kop.SrConfirm<<3)}}
		src := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Cz", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.s.res.e.ctrl, sc.s.res.e.card, sc.s.res.e.ab, sc.s.res.pc = 0, src, 0, 0
		sc.s.sp = 1
		var o Obj
		if withCard {
			o = sc.grave(sc.s.objs[sc.s.pl[0].lib[sc.s.pl[0].libLo]].card, 0, 1)[0]
		}
		return sc, prog, o
	}
	for _, withCard := range []bool{false, true} {
		sc, prog, o := build(withCard)
		s := sc.s
		if s.exec(sc.k, &prog[0]) || s.res.ask != raGenConfirm {
			t.Fatalf("card %v: ask %d, want the confirm", withCard, s.res.ask)
		}
		s.applyAskCold1(sc.k, MakeAction(AkChoose, 0, 0, 0))
		if withCard && s.objs[o].zone != kop.ZoneGraveyard || s.pl[0].handN != 0 {
			t.Fatalf("card %v: a no moved something (hand %d)", withCard, s.pl[0].handN)
		}
	}
	sc, prog, _ := build(true)
	sc.s.exec(sc.k, &prog[0])
	sc.s.applyAskCold1(sc.k, MakeAction(AkChoose, 0, 0, 1))
	if sc.s.res.ask != raGenSearch {
		t.Fatalf("after a yes: ask %d, want the pick", sc.s.res.ask)
	}
}

// TestEnterAttackingMarksAttacker: OpReturnEx with MvAttacking puts the
// creature onto the battlefield attacking the opponent during the
// controller's declare-attackers step (gorge TokenAttacks fold), declared
// nothing; outside combat it just enters; with an opposing planeswalker the
// CR 508.4 defender is asked (gorge entering_defender), the opponent player
// listed first.
func TestEnterAttackingMarksAttacker(t *testing.T) {
	var bear, walker CardID
	build := func() (*scen, []kop.Instr, Obj) {
		sc, prog := czScenWith(t, func(tbl *kop.Table) []kop.Instr {
			bear = CardID(len(tbl.Cards))
			walker = bear + 1
			tbl.Cards = append(tbl.Cards,
				kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
				kop.CardDef{Name: "Test Walker", Types: kop.TPlaneswalker, Loyalty: 3})
			return []kop.Instr{{Op: kop.OpReturnEx, Obj: kop.RefSelf, C: int16(int(kop.ZoneBattlefield) | kop.MvAttacking<<3)}}
		})
		o := sc.grave(bear, 0, 1)[0]
		sc.s.res.e.obj = o
		return sc, prog, o
	}
	sc, prog, o := build()
	sc.s.step = kop.StepAttackers
	sc.s.nAttackers = 1
	sc.s.exec(sc.k, &prog[0])
	if !sc.onBF(o) {
		t.Fatal("the creature did not enter")
	}
	p := &sc.s.perms[sc.slot(o)]
	if p.flags&pfAttacking == 0 || TCode(p.atk) != PlayerTarget0+1 || p.bord != 1 || sc.s.nAttackers != 2 {
		t.Fatalf("attacking %v atk %d bord %d nAttackers %d, want attacking the opponent as attacker 1", p.flags&pfAttacking != 0, p.atk, p.bord, sc.s.nAttackers)
	}
	if sc.s.npend != 0 {
		t.Fatalf("%d triggers queued: an entering attacker declares nothing", sc.s.npend)
	}
	sc, prog, o = build() // main phase: it enters and does not attack
	sc.s.exec(sc.k, &prog[0])
	if !sc.onBF(o) || sc.s.perms[sc.slot(o)].flags&pfAttacking != 0 || sc.s.nAttackers != 0 {
		t.Fatal("outside combat the creature must enter without attacking")
	}
	sc, prog, o = build() // the opponent's combat
	sc.s.step = kop.StepAttackers
	sc.s.active = 1
	sc.s.exec(sc.k, &prog[0])
	if sc.s.perms[sc.slot(o)].flags&pfAttacking != 0 {
		t.Fatal("in the opponent's combat the controller's creature must not attack")
	}
	sc, prog, o = build()
	sc.s.step = kop.StepAttackers
	sc.s.nAttackers = 0
	walk := sc.perm(walker, 1)
	if sc.s.exec(sc.k, &prog[0]) {
		t.Fatal("the entry should suspend at the CR 508.4 defender ask")
	}
	var d Decision
	if sc.s.res.ask != raGenEnterDefender || sc.s.res.chosen != objWord(uint8(sc.slot(o))) {
		t.Fatalf("ask %d slot %d, want the CR 508.4 defender ask for slot %d", sc.s.res.ask, sc.s.res.chosen, sc.slot(o))
	}
	if !sc.s.resolveOptionsGen(sc.k, &d) || d.N != 2 || d.Sub != ChGenEnterDefender ||
		d.Options[0].Source() != PlayerTarget0+1 || d.Options[1].Source() != TCode(uint8(walk)) {
		t.Fatalf("options %+v, want the opponent player then the walker", d.Options[:d.N])
	}
	sc.s.applyAskCold1(sc.k, d.Options[1]) // answer: the walker
	p = &sc.s.perms[sc.slot(o)]
	if p.flags&pfAttacking == 0 || p.atk != TCode(uint8(walk)) || p.bord != 0 || sc.s.nAttackers != 1 {
		t.Fatalf("attacking %v atk %d bord %d nAttackers %d, want attacking the walker", p.flags&pfAttacking != 0, p.atk, p.bord, sc.s.nAttackers)
	}
}

// TestSearchDifferentNames: with SrDifferentNames (carried in B) a library
// of Forest, Forest, Island and a count of 3 is clamped to the two distinct
// names, and after a Forest the other Forest is no longer a candidate.
func TestSearchDifferentNames(t *testing.T) {
	var forest, island, bear CardID
	sc, prog := czScenWith(t, func(tbl *kop.Table) []kop.Instr {
		forest, island, bear = CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Forest", Types: kop.TLand}, kop.CardDef{Name: "Test Island", Types: kop.TLand},
			kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand})
		return []kop.Instr{{Op: kop.OpSearchEx, Obj: kop.RefController, A: kop.Const(3), B: kop.Const(kop.SrDifferentNames),
			F: kop.FilterID(len(tbl.Filters) - 1), C: int16(kop.ZoneHand)}}
	})
	s := sc.s
	pl := &s.pl[0]
	for i, o := range pl.lib[pl.libLo:pl.libHi] {
		c := bear
		switch i {
		case 0, 1:
			c = forest
		case 2:
			c = island
		}
		s.objs[o].card = c
	}
	if s.exec(sc.k, &prog[0]) || s.res.ask != raGenSearch {
		t.Fatalf("ask %d, want the search pick", s.res.ask)
	}
	if s.g.pickLeft != 2 {
		t.Fatalf("count %d, want 2 (distinct names)", s.g.pickLeft)
	}
	var d Decision
	s.resolveOptionsGen(sc.k, &d)
	if d.N != 4 { // three lands and Done
		t.Fatalf("%d options, want Forest, Forest, Island and Done", d.N)
	}
	s.applyResolveGen(sc.k, d.Options[0])
	d = Decision{}
	s.resolveOptionsGen(sc.k, &d)
	if d.N != 2 || s.objs[d.Options[0].Source()].card != island {
		t.Fatalf("after a Forest: %d options, want only the Island and Done", d.N)
	}
	s.applyResolveGen(sc.k, d.Options[0])
	if pl.handN != 2 {
		t.Fatalf("hand %d, want the two lands found", pl.handN)
	}
}

// TestThisDefinedAndTgtsSelf: OpReturnEx with MvAlsoSelf returns the target,
// then the source (gorge appends ThisDefinedAndTgts$ Self after the chosen
// targets); a source that has left the graveyard stays where it is.
func TestThisDefinedAndTgtsSelf(t *testing.T) {
	build := func() (*scen, []kop.Instr, Obj, Obj) {
		sc, prog := czScenWith(t, func(*kop.Table) []kop.Instr {
			return []kop.Instr{{Op: kop.OpReturnEx, Obj: kop.RefTarget0,
				C: int16(int(kop.ZoneBattlefield) | (kop.MvAlsoSelf|kop.MvTapped)<<3)}}
		})
		land := sc.s.objs[sc.s.pl[0].lib[sc.s.pl[0].libLo]].card
		tgt, src := sc.grave(land, 0, 1)[0], sc.grave(land, 0, 1)[0]
		sc.s.res.e.obj = src
		sc.s.res.e.tgt[0], sc.s.res.e.nt = TCode(tgt), 1
		return sc, prog, tgt, src
	}
	sc, prog, tgt, src := build()
	sc.s.exec(sc.k, &prog[0])
	if !sc.onBF(tgt) || !sc.onBF(src) {
		t.Fatal("the target and the source must both return")
	}
	if sc.slot(tgt) >= sc.slot(src) {
		t.Fatalf("target slot %d, source slot %d: the target enters first", sc.slot(tgt), sc.slot(src))
	}
	if !sc.tapped(tgt) || !sc.tapped(src) {
		t.Fatal("both enter tapped")
	}
	sc, prog, tgt, src = build()
	sc.s.placeFrom(src, kop.ZoneExile, false, sc.s.detach(src), false) // no longer in the graveyard
	sc.s.exec(sc.k, &prog[0])
	if !sc.onBF(tgt) || sc.onBF(src) {
		t.Fatalf("source gone: target on battlefield %v, source %v, want true false", sc.onBF(tgt), sc.onBF(src))
	}
}
