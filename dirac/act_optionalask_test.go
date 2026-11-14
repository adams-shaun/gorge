package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// optaskScen builds a State over the optask pool's table at seat 0's main
// phase 1 of turn 2 (the libraries are Plains), with the deck entries as
// CardIDs in ids.
func optaskScen(t *testing.T, a, b []cardset.Entry) (*scen, map[string]kop.CardID) {
	t.Helper()
	set := cardsettest.Set(t, "optask")
	for _, e := range append(append([]cardset.Entry{}, a...), b...) {
		if _, ok := set.Lookup(e.Name); !ok {
			t.Fatalf("%s not in the optask pool", e.Name)
		}
	}
	tbl, da, db, err := set.Matchup(a, b)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]kop.CardID{}
	for _, c := range append(append([]kop.CardID{}, da...), db...) {
		ids[tbl.Cards[c].Name] = c
	}
	var plains kop.CardID
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plains = kop.CardID(i)
		}
	}
	var lib [20]kop.CardID
	for i := range lib {
		lib[i] = plains
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{})
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn = 2
	s.active = 0
	s.step = kop.StepMain1
	s.prio = 0
	s.passes = 0
	s.ctl = ctlPriority
	return &scen{s: s, k: s.k()}, ids
}

// optaskCast casts o from p's hand and lets it resolve (the ETB trigger
// left pending), failing on any unexpected decision.
func optaskCast(t *testing.T, sc *scen, o Obj) {
	t.Helper()
	sc.do(t, AkCast, TCode(o), scAny)
	for i := 0; i < 30; i++ {
		if sc.s.objs[o].zone != kop.ZoneStack {
			return // resolved; its ETB trigger is pending
		}
		d := sc.s.Step()
		sc.s.Apply(d.Options[0])
	}
	t.Fatal("spell never left the stack")
}

// runTo steps the game to the first DkChoose decision with sub want (the
// optional ask under test), passing priority, combat and elided decisions;
// it fails on any other choice.
func runTo(t *testing.T, sc *scen, want ChoiceKind) *Decision {
	t.Helper()
	for i := 0; i < 300; i++ {
		d := sc.s.Step()
		switch d.Kind {
		case DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		case DkAttackers, DkBlockers:
			sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
		case DkChoose:
			if d.Sub == want {
				dd := d
				return &dd
			}
			t.Fatalf("unexpected DkChoose sub %d player %d", d.Sub, d.Player)
		case DkNone:
			t.Fatal("game ended before the ask")
		default:
			t.Fatalf("unexpected decision kind %d", d.Kind)
		}
	}
	t.Fatal("ask never posed")
	return nil
}

// applyOpt answers d with the first option matching pred.
func applyOpt(t *testing.T, sc *scen, d *Decision, pred func(Action) bool, what string) {
	t.Helper()
	for _, a := range d.Options[:d.N] {
		if pred(a) {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("no %s option in %v", what, d.Options[:d.N])
}

// TestOptaskAugurDecline: Kinetic Augur's ETB "you may discard up to two"
// asks the may-election; "no" discards nothing and draws nothing.
func TestOptaskAugurDecline(t *testing.T) {
	sc, ids := optaskScen(t,
		[]cardset.Entry{{Name: "Kinetic Augur", Count: 1}, {Name: "Mountain", Count: 4}},
		[]cardset.Entry{{Name: "Grizzly Bears", Count: 1}})
	for i := 0; i < 4; i++ {
		sc.perm(ids["Mountain"], 0)
	}
	isl1 := sc.hand(ids["Mountain"], 0)
	isl2 := sc.hand(ids["Mountain"], 0)
	augur := sc.hand(ids["Kinetic Augur"], 0)
	optaskCast(t, sc, augur)

	d := runTo(t, sc, ChOptDiscMay)
	if d.Player != 0 {
		t.Fatalf("election asked player %d, want 0", d.Player)
	}
	applyOpt(t, sc, d, func(a Action) bool { return a.Kind() == AkChoose && a.Mode() == 0 }, "no")
	if sc.s.pl[0].handN != 2 || sc.s.pl[0].graveN != 0 {
		t.Fatalf("hand %d grave %d after declining, want 2/0", sc.s.pl[0].handN, sc.s.pl[0].graveN)
	}
	if sc.s.pl[0].hand[0] != isl1 || sc.s.pl[0].hand[1] != isl2 {
		t.Fatalf("hand %v after declining, want both Mountains", sc.s.pl[0].hand[:2])
	}
	if !sc.onBF(augur) {
		t.Fatal("Augur left the battlefield")
	}
}

// TestOptaskAugurAccept: "yes" poses the Min 1 pick (Max the eligible
// count); picking both cards discards both and draws two.
func TestOptaskAugurAccept(t *testing.T) {
	sc, ids := optaskScen(t,
		[]cardset.Entry{{Name: "Kinetic Augur", Count: 1}, {Name: "Mountain", Count: 4}},
		[]cardset.Entry{{Name: "Grizzly Bears", Count: 1}})
	for i := 0; i < 4; i++ {
		sc.perm(ids["Mountain"], 0)
	}
	isl1 := sc.hand(ids["Mountain"], 0)
	isl2 := sc.hand(ids["Mountain"], 0)
	augur := sc.hand(ids["Kinetic Augur"], 0)
	optaskCast(t, sc, augur)

	d := runTo(t, sc, ChOptDiscMay)
	applyOpt(t, sc, d, func(a Action) bool { return a.Kind() == AkChoose && a.Mode() == 1 }, "yes")
	// the Min 1 pick: pick both, the re-pose offers the second after the first
	d = runTo(t, sc, ChDiscard)
	sc.do(t, AkChoose, TCode(isl1), scAny)
	d = runTo(t, sc, ChDiscard)
	sc.do(t, AkChoose, TCode(isl2), scAny)
	if sc.s.pl[0].graveN != 2 || sc.s.pl[0].handN != 2 {
		t.Fatalf("grave %d hand %d after discarding two, want 2/2 (the two draws)", sc.s.pl[0].graveN, sc.s.pl[0].handN)
	}
	for i := 0; i < int(sc.s.pl[0].graveN); i++ {
		if o := sc.s.pl[0].grave[i]; o != isl1 && o != isl2 {
			t.Fatalf("grave card %d is not a discarded Island", o)
		}
	}
}

// TestOptaskAugurAcceptOne: with a single eligible card a "yes" discards
// it with no pick ask (gorge's strict-supersets rule).
func TestOptaskAugurAcceptOne(t *testing.T) {
	sc, ids := optaskScen(t,
		[]cardset.Entry{{Name: "Kinetic Augur", Count: 1}, {Name: "Mountain", Count: 4}},
		[]cardset.Entry{{Name: "Grizzly Bears", Count: 1}})
	for i := 0; i < 4; i++ {
		sc.perm(ids["Mountain"], 0)
	}
	isl := sc.hand(ids["Mountain"], 0)
	augur := sc.hand(ids["Kinetic Augur"], 0)
	optaskCast(t, sc, augur)

	d := runTo(t, sc, ChOptDiscMay)
	applyOpt(t, sc, d, func(a Action) bool { return a.Kind() == AkChoose && a.Mode() == 1 }, "yes")
	if sc.s.pl[0].graveN != 1 || sc.s.pl[0].grave[0] != isl {
		t.Fatalf("grave %v after accepting with one eligible, want the Island discarded with no pick ask", sc.s.pl[0].grave[:sc.s.pl[0].graveN])
	}
}

// TestOptaskMaggotsAnyNumber: Mind Maggots' ETB "discard any number of
// creature cards" is a Min 0 filtered pick with Done offered; Done
// declines, picks commit on the last pick (two bears: four counters).
func TestOptaskMaggotsAnyNumber(t *testing.T) {
	sc, ids := optaskScen(t,
		[]cardset.Entry{{Name: "Mind Maggots", Count: 1}, {Name: "Swamp", Count: 3}, {Name: "Grizzly Bears", Count: 2}},
		[]cardset.Entry{{Name: "Grizzly Bears", Count: 1}})
	for i := 0; i < 4; i++ {
		sc.perm(ids["Swamp"], 0)
	}
	b1 := sc.hand(ids["Grizzly Bears"], 0)
	b2 := sc.hand(ids["Grizzly Bears"], 0)
	maggots := sc.hand(ids["Mind Maggots"], 0)
	optaskCast(t, sc, maggots)

	// decline
	d := runTo(t, sc, ChDiscard)
	if d.Player != 0 {
		t.Fatalf("pick asked player %d, want 0", d.Player)
	}
	applyOpt(t, sc, d, func(a Action) bool { return a.Kind() == AkDone }, "Done")
	if sc.s.pl[0].graveN != 0 || sc.s.perms[sc.slot(maggots)].ctrLow(int(kop.CtrP1P1)) != 0 {
		t.Fatalf("declined: grave %d counters %d, want 0/0", sc.s.pl[0].graveN, sc.s.perms[sc.slot(maggots)].ctrLow(int(kop.CtrP1P1)))
	}
	if sc.s.pl[0].hand[0] != b1 || sc.s.pl[0].hand[1] != b2 {
		t.Fatalf("declined: hand %v, want both Bears", sc.s.pl[0].hand[:2])
	}

	// accept, in a fresh game
	sc2, ids2 := optaskScen(t,
		[]cardset.Entry{{Name: "Mind Maggots", Count: 1}, {Name: "Swamp", Count: 3}, {Name: "Grizzly Bears", Count: 2}},
		[]cardset.Entry{{Name: "Grizzly Bears", Count: 1}})
	for i := 0; i < 4; i++ {
		sc2.perm(ids2["Swamp"], 0)
	}
	nb1 := sc2.hand(ids2["Grizzly Bears"], 0)
	nb2 := sc2.hand(ids2["Grizzly Bears"], 0)
	nm := sc2.hand(ids2["Mind Maggots"], 0)
	optaskCast(t, sc2, nm)
	d = runTo(t, sc2, ChDiscard)
	sc2.do(t, AkChoose, TCode(nb1), scAny) // the re-pose offers the second bear
	d = runTo(t, sc2, ChDiscard)
	sc2.do(t, AkChoose, TCode(nb2), scAny)
	if sc2.s.pl[0].graveN != 2 || sc2.s.perms[sc2.slot(nm)].ctrLow(int(kop.CtrP1P1)) != 4 {
		t.Fatalf("accepted: grave %d counters %d, want 2/4", sc2.s.pl[0].graveN, sc2.s.perms[sc2.slot(nm)].ctrLow(int(kop.CtrP1P1)))
	}
}

// TestOptaskCrovaxSacrifice: Crovax's upkeep "you may sacrifice a creature"
// is the Min 0 optional sacrifice: Done keeps Crovax (loses a counter), a
// pick sacrifices it and grows Crovax.
func TestOptaskCrovaxSacrifice(t *testing.T) {
	decline := func(t *testing.T) {
		sc, ids := optaskScen(t,
			[]cardset.Entry{{Name: "Crovax the Cursed", Count: 1}, {Name: "Grizzly Bears", Count: 2}},
			[]cardset.Entry{{Name: "Grizzly Bears", Count: 1}})
		crovax := sc.perm(ids["Crovax the Cursed"], 0)
		sc.perm(ids["Grizzly Bears"], 0)
		if n := sc.s.perms[sc.slot(crovax)].ctrLow(int(kop.CtrP1P1)); n != 4 {
			t.Fatalf("Crovax entered with %d counters, want 4", n)
		}
		d := runTo(t, sc, ChSacrifice)
		applyOpt(t, sc, d, func(a Action) bool { return a.Kind() == AkDone }, "Done")
		if !sc.onBF(crovax) || sc.s.perms[sc.slot(crovax)].ctrLow(int(kop.CtrP1P1)) != 3 {
			t.Fatalf("declined: on bf %v counters %d, want true/3", sc.onBF(crovax), sc.s.perms[sc.slot(crovax)].ctrLow(int(kop.CtrP1P1)))
		}
	}
	decline(t)

	accept := func(t *testing.T) {
		sc, ids := optaskScen(t,
			[]cardset.Entry{{Name: "Crovax the Cursed", Count: 1}, {Name: "Grizzly Bears", Count: 2}},
			[]cardset.Entry{{Name: "Grizzly Bears", Count: 1}})
		crovax := sc.perm(ids["Crovax the Cursed"], 0)
		bears := sc.perm(ids["Grizzly Bears"], 0)
		if n := sc.s.perms[sc.slot(crovax)].ctrLow(int(kop.CtrP1P1)); n != 4 {
			t.Fatalf("Crovax entered with %d counters, want 4", n)
		}
		runTo(t, sc, ChSacrifice)
		sc.do(t, AkChoose, TCode(bears), scAny)
		if sc.onBF(bears) || sc.s.pl[0].graveN != 1 {
			t.Fatalf("accepted: bears on bf %v grave %d, want false/1", sc.onBF(bears), sc.s.pl[0].graveN)
		}
		if !sc.onBF(crovax) || sc.s.perms[sc.slot(crovax)].ctrLow(int(kop.CtrP1P1)) != 5 {
			t.Fatalf("accepted: crovax on bf %v counters %d, want true/5", sc.onBF(crovax), sc.s.perms[sc.slot(crovax)].ctrLow(int(kop.CtrP1P1)))
		}
	}
	accept(t)
}

// TestOptaskRepNextSkipsEmpty: startOptSac walks the each-player chain by
// repNext, skipping a player with no eligible permanent.
func TestOptaskRepNextSkipsEmpty(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "optask")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	bear, rite := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Rite", Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Prog: []kop.Instr{{Op: kop.OpSacrifice, Obj: kop.RefController, A: kop.Const(1), C: kop.OptAsk}}}}})
	sc := newScenTable(t, tbl, Config{})
	bears := sc.perm(bear, 1)
	in := kop.Instr{Op: kop.OpSacrifice, Obj: kop.RefController, A: kop.Const(1), C: kop.OptAsk}
	dummy := sc.obj(rite, 0)
	sc.s.res.e = stackEntry{kind: seSpell, obj: dummy, card: rite}
	sc.s.g.repNext = 1
	if sc.s.startOptSac(sc.k, &in, 0) {
		t.Fatal("startOptSac finished without posing the ask")
	}
	if sc.s.res.ask != raOptSac || sc.s.res.askP != 1 {
		t.Fatalf("ask %d player %d, want raOptSac/1", sc.s.res.ask, sc.s.res.askP)
	}
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChSacrifice || d.Player != 1 {
		t.Fatalf("decision kind %d sub %d player %d, want DkChoose ChSacrifice 1", d.Kind, d.Sub, d.Player)
	}
	applyOpt(t, sc, &d, func(a Action) bool { return a.Kind() == AkDone }, "Done")
	if !sc.onBF(bears) {
		t.Fatal("Bears sacrificed on Done")
	}
}

// TestOptaskOildeepSelfTarget: Oildeep Gearhulk's ETB "look at target
// player's hand. You may choose a card from it" targets a player, then
// gorge's discardChooser asks the SOURCE'S CONTROLLER, not the target
// (effects/cardflow.go discardChooser: RevealYouChoose returns
// c.Controller; only RevealTgtChoose returns the target). ValidTgts$ Player
// lowers to both seats, so the controller may target itself; the election
// and the pick must then still go to the controller (p0), and the
// opponent's hand must stay untouched.
func TestOptaskOildeepSelfTarget(t *testing.T) {
	sc, ids := optaskScen(t,
		[]cardset.Entry{{Name: "Oildeep Gearhulk", Count: 1}, {Name: "Island", Count: 2}, {Name: "Swamp", Count: 2}, {Name: "Grizzly Bears", Count: 2}},
		[]cardset.Entry{{Name: "Grizzly Bears", Count: 2}})
	for i := 0; i < 2; i++ {
		sc.perm(ids["Island"], 0)
		sc.perm(ids["Swamp"], 0)
	}
	mine1 := sc.hand(ids["Grizzly Bears"], 0)
	mine2 := sc.hand(ids["Grizzly Bears"], 0)
	theirs := sc.hand(ids["Grizzly Bears"], 1)
	oil := sc.hand(ids["Oildeep Gearhulk"], 0)
	sc.do(t, AkCast, TCode(oil), scAny)

	var d Decision
	for i := 0; i < 40; i++ {
		d = sc.s.Step()
		if d.Kind == DkTarget {
			break
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		sc.s.Apply(d.Options[0])
	}
	if d.Kind != DkTarget || d.Player != 0 {
		t.Fatalf("ETB target: kind %d player %d, want the controller's target decision", d.Kind, d.Player)
	}
	applyOpt(t, sc, &d, func(a Action) bool { return a.Kind() == AkTarget && a.Target() == PlayerTarget0 }, "self target")

	d = *runTo(t, sc, ChDiscard)
	if d.Player != 0 {
		t.Fatalf("chooseMode pick asked player %d, want the controller (0) even when it targeted itself", d.Player)
	}
	sc.do(t, AkChoose, TCode(mine1), scAny)
	if sc.s.objs[mine1].zone != kop.ZoneGraveyard || sc.s.objs[theirs].zone != kop.ZoneHand {
		t.Fatalf("after the pick: mine %d theirs %d, want the controller's card discarded and the opponent's hand intact",
			sc.s.objs[mine1].zone, sc.s.objs[theirs].zone)
	}
	if sc.s.objs[mine2].zone != kop.ZoneHand {
		t.Fatal("the unchosen controller card left the hand")
	}
}
