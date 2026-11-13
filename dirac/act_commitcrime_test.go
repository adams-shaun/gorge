package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func crimeScen(t *testing.T) (*scen, func(string) CardID) {
	t.Helper()
	set := cardsettest.Set(t, "crime")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the pool", name)
		}
		return CardID(set.PoolID(i))
	}
	sc := newScenTableLib(t, tbl, id("Swamp"), Config{})
	if !sc.k.gen.crimeOn {
		t.Fatal("the crime table does not turn crimeOn on")
	}
	return sc, id
}

// crimeStackSpell puts a spell of card c controlled by p on the stack.
func (sc *scen) crimeStackSpell(c CardID, p uint8) Obj {
	o := sc.hand(c, p)
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: c, ctrl: p, srcVer: sc.s.objs[o].ver}}
	sc.s.bld().e.tgt = noTargets
	sc.s.pushBuild(sc.k)
	sc.s.ctl = ctlPriority
	*sc.s.bld() = build{}
	return o
}

// TestCrimeTargetKinds is the CR 700.13 table, one target at a time (gorge
// targetCommitsCrime): a player, a battlefield object or a spell another
// player controls, a card in another player's graveyard. Exile and hand
// never count.
func TestCrimeTargetKinds(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	bear := id("Hardbristle Bandit")
	myPerm, theirPerm := sc.perm(bear, 0), sc.perm(bear, 1)
	stolen := sc.perm(bear, 1) // their card under my control
	s.perms[sc.slot(stolen)].ctrl = 0
	myGrave := sc.grave(bear, 0, 1)[0]
	theirGrave := sc.grave(bear, 1, 1)[0]
	myExile, theirExile := sc.obj(bear, 0), sc.obj(bear, 1)
	s.place(myExile, kop.ZoneExile, false)
	s.place(theirExile, kop.ZoneExile, false)
	theirHand := sc.hand(bear, 1)
	burst := id("Burst Lightning")
	mySpell, theirSpell := sc.crimeStackSpell(burst, 0), sc.crimeStackSpell(burst, 1)
	s.ctl = ctlPriority
	cases := []struct {
		name string
		t    TCode
		want bool
	}{
		{"the opponent", TCode(PlayerTarget0 + 1), true},
		{"you", TCode(PlayerTarget0), false},
		{"their permanent", TCode(theirPerm), true},
		{"your permanent", TCode(myPerm), false},
		{"their card you control", TCode(stolen), false},
		{"their spell on the stack", TCode(theirSpell), true},
		{"your spell on the stack", TCode(mySpell), false},
		{"a card in their graveyard", TCode(theirGrave), true},
		{"a card in your graveyard", TCode(myGrave), false},
		{"your card in exile", TCode(myExile), false},
		{"their card in exile", TCode(theirExile), false},
		{"their card in hand", TCode(theirHand), false},
		{"no target", NoTarget, false},
	}
	for _, c := range cases {
		s.g.crime = 0
		if got := s.noteCrime(sc.k, 0, c.t); got != c.want {
			t.Errorf("%s: crime %v, want %v", c.name, got, c.want)
		}
		if bit := s.g.crime != 0; bit != c.want {
			t.Errorf("%s: ledger %b, want set %v", c.name, s.g.crime, c.want)
		}
	}
	// the other seat: my permanent is theirs to attack
	s.g.crime = 0
	if !s.noteCrime(sc.k, 1, TCode(myPerm)) || s.g.crime != 2 {
		t.Errorf("player 1 targeting player 0's permanent: ledger %b, want 10", s.g.crime)
	}
}

// TestCrimeOncePerObject: two criminal targets on one ability queue one
// trigger per listener (gorge commitCrimeMatches checks the object's earlier
// targets); Marchesa (no activation limit) queues once, not twice.
func TestCrimeOncePerObject(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	march, rattle := sc.perm(id("Marchesa, Dealer of Death"), 0), sc.perm(id("Rattleback Apothecary"), 0)
	_, _ = march, rattle
	a, b := sc.perm(id("Hardbristle Bandit"), 1), sc.perm(id("Hardbristle Bandit"), 1)
	run := func() int {
		s.npend, s.g.crime = 0, 0
		e := stackEntry{kind: seAbility, ctrl: 0, nt: 2, obj: march, trigObj: NoObj}
		e.tgt = [kop.MaxTargets]TCode{TCode(a), TCode(b), TCode(NoTarget)}
		s.checkWard(sc.k, &e)
		return int(s.npend)
	}
	if n := run(); n != 2 {
		t.Fatalf("two criminal targets queued %d triggers, want 2 (one per listener)", n)
	}
	DebugCrimeEachTarget = true
	defer func() { DebugCrimeEachTarget = false }()
	if n := run(); n != 4 {
		t.Fatalf("mutation: %d triggers, want 4", n)
	}
}

// TestCrimeSpellReversed: a spell's criminal pick sets the ledger before
// payment and a CR 733.1 reversal keeps it (gorge crimeSeatsThisTurn is
// never rolled back) while queueing nothing.
func TestCrimeSpellReversed(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	sc.perm(id("Rattleback Apothecary"), 0)
	burst := id("Burst Lightning")
	o := sc.hand(burst, 0)
	*s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: burst, ctrl: 0, srcVer: s.objs[o].ver}}
	s.bld().e.tgt = noTargets
	s.pushBuild(sc.k)
	s.ctl = ctlBuild
	s.applyBuild(sc.k, MakeAction(AkTarget, 0, PlayerTarget0+1, 0))
	if s.g.crime != 1 || s.bld().bfl&bflCrime == 0 {
		t.Fatalf("after the pick: ledger %b flags %b, want bit 0 and bflCrime", s.g.crime, s.bld().bfl)
	}
	s.npend = 0
	s.reverseCast(sc.k)
	if s.g.crime != 1 {
		t.Fatalf("the reversal rolled the ledger back: %b", s.g.crime)
	}
	if s.npend != 0 {
		t.Fatalf("a reversed cast queued %d triggers", s.npend)
	}
}

// TestCrimeCopyRetarget: a copy's new-target answer is a crime for the
// copy's controller (gorge resolution.go records the answer once).
func TestCrimeCopyRetarget(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	sc.perm(id("Rattleback Apothecary"), 0)
	burst := id("Burst Lightning")
	o := sc.crimeStackSpell(burst, 0)
	e := &s.stack[s.sp-1]
	e.nt, e.tgt[0] = 1, TCode(PlayerTarget0) // aimed at its own controller
	s.g.crime, s.npend = 0, 0
	s.setCopyTargetsQ(sc.k, e, 0, []Obj{Obj(PlayerTarget0 + 1)})
	s.crimeCopy(sc.k, e)
	if s.g.crime != 1 || s.npend != 1 {
		t.Fatalf("copy retargeted at the opponent: ledger %b, %d triggers, want bit 0 and 1 (obj %d)", s.g.crime, s.npend, o)
	}
}

// TestCrimeResolutionTargetNotACrime: a mid-resolution pick
// (OpChooseTargets) is no crime in gorge (its pre-ask emits no
// TargetsChosen).
func TestCrimeResolutionTargetNotACrime(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	burst := id("Burst Lightning")
	opp := sc.perm(id("Hardbristle Bandit"), 1)
	o := sc.crimeStackSpell(burst, 0)
	e := s.stack[s.sp-1]
	s.res = resCtx{e: e, active: true, ask: raGenResTarget, lastTok: NoObj, eachObj: NoObj}
	s.res.e.obj = o
	s.g.crime = 0
	s.applyResolveGen(sc.k, MakeAction(AkChoose, TCode(opp), 0, 0))
	if s.g.crime != 0 {
		t.Fatalf("a resolution-time pick set the ledger: %b", s.g.crime)
	}
}

// TestCrimeTurnReset: newTurnGen clears the ledger.
func TestCrimeTurnReset(t *testing.T) {
	sc, _ := crimeScen(t)
	sc.s.g.crime = 3
	sc.s.newTurnGen()
	if sc.s.g.crime != 0 {
		t.Fatalf("ledger %b survived the turn change", sc.s.g.crime)
	}
}

// TestCrimeCount: ExCommittedCrime picks the arm by the evaluating
// controller's bit.
func TestCrimeCount(t *testing.T) {
	sc, _ := crimeScen(t)
	s := sc.s
	tbl := sc.k.t
	x := &kop.Expr{Kind: kop.ExCommittedCrime, Sub: [2]kop.ValueRef{kop.Const(1), kop.Const(0)}}
	_ = tbl
	for _, c := range []struct {
		ledger uint8
		p      uint8
		want   int
	}{{0, 0, 0}, {1, 0, 1}, {1, 1, 0}, {2, 1, 1}, {2, 0, 0}, {3, 0, 1}, {3, 1, 1}} {
		s.g.crime = c.ledger
		if got := s.committedCrime(sc.k, x, evalCtx{src: NoObj, srcSl: NoSlot, ctrl: c.p}); got != c.want {
			t.Errorf("ledger %b player %d: %d, want %d", c.ledger, c.p, got, c.want)
		}
	}
}

// TestCrimeModalAbilityFiresOnce: CR 700.13 is one crime per activation; a
// modal ability with a criminal root target queues each listener once (gorge
// agrees).
func TestCrimeModalAbilityFiresOnce(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	march := sc.perm(id("Marchesa, Dealer of Death"), 0)
	a := sc.perm(id("Hardbristle Bandit"), 1)
	s.npend, s.g.crime = 0, 0
	e := stackEntry{kind: seAbility, ctrl: 0, nt: 1, mode: 1, obj: march, trigObj: NoObj}
	e.tgt = [kop.MaxTargets]TCode{TCode(a), TCode(NoTarget), TCode(NoTarget), TCode(NoTarget), TCode(NoTarget)}
	s.checkWard(sc.k, &e)
	if s.npend != 1 || s.g.crime != 1 {
		t.Fatalf("modal ability queued %d (ledger %b), want 1 (ledger 1)", s.npend, s.g.crime)
	}
}
