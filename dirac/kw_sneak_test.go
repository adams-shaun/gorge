package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// ksneakScen appends a sneak creature ({3}{W} 3/3, sneak {W}) and a sneak
// sorcery (Divination, sneak {W}) to a copy of the vanilla table, gives
// seat 0 two Plains, and returns the scenario with the two card ids.
func ksneakScen(t *testing.T) (*scen, CardID, CardID) {
	t.Helper()
	tbl := *testTable()
	tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	tbl.Conds = []kop.Cond{{}} // a non-nil Conds enables prepareGen for the generated arms
	// a battlefield read of Card.sneaked: with it the arena
	// gives mechSneaked an index
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Flags2: kop.F2Sneaked})
	cr := CardID(len(tbl.Cards))
	w := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.W: 1}}
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Sneaker", Types: kop.TCreature, Power: 3, Toughness: 3,
		Cost:      kop.ManaCost{Generic: 3, Pips: [kop.NumManaKinds]uint8{kop.W: 1}},
		HasSneak:  true,
		SneakCost: w,
	})
	so := CardID(len(tbl.Cards))
	div := tbl.Cards[v.Divination]
	div.Name, div.HasSneak, div.SneakCost = "Test Technique", true, w
	tbl.Cards = append(tbl.Cards, div)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.lands(0, v.Plains, 2)
	return sc, cr, so
}

// ksneakToBlockers attacks with attackers, has seat 1 block per blocks,
// and stops at seat 0's first priority in the declare-blockers step.
func ksneakToBlockers(t *testing.T, sc *scen, attackers []Obj, blocks [][2]Obj) {
	t.Helper()
	sc.passUntil(t, kop.StepAttackers)
	for _, a := range attackers {
		sc.do(t, AkAttack, TCode(a), scAny)
	}
	sc.do(t, AkDone, scAny, scAny)
	for i := 0; i < 16; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority && d.Player == 0 && sc.s.step == kop.StepBlockers {
			return
		}
		switch d.Kind {
		case DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		case DkBlockers:
			for _, b := range blocks {
				sc.do(t, AkBlock, TCode(b[0]), TCode(b[1]))
			}
			sc.do(t, AkDone, scAny, scAny)
		default:
			t.Fatalf("decision kind %d before seat 0's blockers priority", d.Kind)
		}
	}
	t.Fatalf("never reached seat 0's priority in the declare-blockers step (step %d)", sc.s.step)
}

func ksneakOffered(d *Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkSneak && Obj(a.Source()) == o {
			return true
		}
	}
	return false
}

// ksneakCast takes the sneak option for o and answers the return pick with
// ret, returning the pick's candidate count.
func ksneakCast(t *testing.T, sc *scen, o, ret Obj) int {
	t.Helper()
	sc.do(t, AkSneak, TCode(o), scAny)
	for i := 0; i < 8; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChCostReturn {
			n := int(d.N)
			sc.s.Apply(MakeAction(AkChoose, TCode(ret), 0, 0))
			return n
		}
		if d.Kind == DkPriority {
			t.Fatalf("no return pick before priority")
		}
		sc.s.Apply(d.Options[0])
	}
	t.Fatalf("no return pick")
	return 0
}

// TestSneakOfferWindow: the sneak cast is offered only in the active
// player's declare-blockers step, with an unblocked attacker and the cost
// payable; not in the damage step, not to the non-active player, not when
// every attacker is blocked (CR 702.190a).
func TestSneakOfferWindow(t *testing.T) {
	sc, cr, _ := ksneakScen(t)
	o := sc.hand(cr, 0)
	bear := sc.perm(v.GrizzlyBears, 0)
	ksneakToBlockers(t, sc, []Obj{bear}, nil)
	d := sc.s.Step()
	if !ksneakOffered(&d, o) {
		t.Fatalf("sneak not offered in the declare-blockers step: %v", d.Options[:d.N])
	}
	s := sc.s
	s.step = kop.StepDamage
	var dd Decision
	s.priorityOptions(s.k(), &dd)
	if ksneakOffered(&dd, o) {
		t.Fatalf("sneak offered in the combat-damage step")
	}
	s.step = kop.StepBlockers
	// the opponent's priority in the step (their own sneak card)
	o1 := sc.hand(cr, 1)
	sc.lands(1, v.Plains, 1)
	s.prio = 1
	dd = Decision{}
	s.priorityOptions(s.k(), &dd)
	if ksneakOffered(&dd, o1) {
		t.Fatalf("sneak offered to the non-active player")
	}
	s.prio = 0
	// every attacker blocked: no return candidate
	s.perms[sc.slot(bear)].flags |= pfBlocked
	dd = Decision{}
	s.priorityOptions(s.k(), &dd)
	if ksneakOffered(&dd, o) {
		t.Fatalf("sneak offered with every attacker blocked")
	}
	s.perms[sc.slot(bear)].flags &^= pfBlocked
	// gorge's surface (Config.GorgeUnpriced): the mana census pays from the
	// untapped lands; the pool-only mutation restores the old offer
	s.cfg |= cfgGorgeUnpriced
	defer func() { DebugFacePoolOnly = false }()
	dd = Decision{}
	s.priorityOptions(s.k(), &dd)
	if !ksneakOffered(&dd, o) {
		t.Fatalf("sneak not offered from the untapped lands under the unpriced surface")
	}
	DebugFacePoolOnly = true
	dd = Decision{}
	s.priorityOptions(s.k(), &dd)
	if ksneakOffered(&dd, o) {
		t.Fatalf("sneak offered with the pool-only mutation and no floating mana")
	}
	s.pl[0].pool[kop.W], s.pl[1].pool[kop.W] = 1, 1
	dd = Decision{}
	s.priorityOptions(s.k(), &dd)
	if !ksneakOffered(&dd, o) {
		t.Fatalf("sneak not offered with the floating pool paying it")
	}
	DebugFacePoolOnly = false
	for _, c := range []struct {
		name  string
		setup func()
		undo  func()
	}{
		{"every attacker blocked", func() { s.perms[sc.slot(bear)].flags |= pfBlocked }, func() { s.perms[sc.slot(bear)].flags &^= pfBlocked }},
		{"first-strike damage step", func() { s.step = kop.StepFirstStrikeDamage }, func() { s.step = kop.StepBlockers }},
		{"combat-damage step", func() { s.step = kop.StepDamage }, func() { s.step = kop.StepBlockers }},
		{"end of combat step", func() { s.step = kop.StepEndCombat }, func() { s.step = kop.StepBlockers }},
	} {
		c.setup()
		dd = Decision{}
		s.priorityOptions(s.k(), &dd)
		if ksneakOffered(&dd, o) {
			t.Fatalf("sneak offered with the pool paying: %s", c.name)
		}
		c.undo()
	}
	// the non-active player's priority with its pool paying, a carrier in
	// hand and an unblocked attacking creature: only p == s.active withholds
	foe := sc.perm(v.GrizzlyBears, 1)
	s.perms[sc.slot(foe)].flags |= pfAttacking
	s.prio = 1
	dd = Decision{}
	s.priorityOptions(s.k(), &dd)
	if ksneakOffered(&dd, o1) {
		t.Fatalf("sneak offered to the non-active player with the pool paying")
	}
	s.perms[sc.slot(foe)].flags &^= pfAttacking
	s.prio = 0
}

// TestSneakCastCreature: the return pick sends the attacker to its
// owner's hand, the spell costs the sneak cost, and it resolves tapped and
// attacking the returned creature's defender, undeclared (CR 702.190b).
func TestSneakCastCreature(t *testing.T) {
	sc, cr, _ := ksneakScen(t)
	o := sc.hand(cr, 0)
	bear := sc.perm(v.GrizzlyBears, 0)
	ksneakToBlockers(t, sc, []Obj{bear}, nil)
	s := sc.s
	attacked := s.g.attacked
	atk := s.perms[sc.slot(bear)].atk
	if n := ksneakCast(t, sc, o, bear); n != 1 {
		t.Fatalf("return pick has %d candidates, want 1", n)
	}
	for i := 0; i < 8; i++ {
		d := s.Step()
		if d.Kind == DkPriority {
			break
		}
		s.Apply(d.Options[0])
	}
	if s.objs[o].zone != kop.ZoneStack {
		t.Fatalf("sneak spell zone %d, want stack", s.objs[o].zone)
	}
	if s.objs[bear].zone != kop.ZoneHand {
		t.Fatalf("returned attacker zone %d, want hand", s.objs[bear].zone)
	}
	untapped := 0
	for m := s.ctrlM[0]; m.any(); m = m.rest() {
		if s.perms[m.first()].flags&pfTapped == 0 {
			untapped++
		}
	}
	if untapped != 1 {
		t.Fatalf("%d untapped Plains after the cast, want 1 (sneak cost {W})", untapped)
	}
	if e := &s.stack[s.sp-1]; e.trigPl != trigPlSneak || TCode(e.trigObj) != atk {
		t.Fatalf("stack entry trigPl %#x trigObj %#x, want %#x %#x", e.trigPl, e.trigObj, trigPlSneak, atk)
	}
	sc.passBoth(t)
	if !sc.onBF(o) {
		t.Fatalf("sneak creature not on the battlefield")
	}
	pp := &s.perms[sc.slot(o)]
	if pp.flags&pfTapped == 0 || pp.flags&pfAttacking == 0 || pp.atk != atk || pp.flags&pfBlocked != 0 {
		t.Fatalf("entered flags %#x atk %#x, want tapped, attacking %#x, unblocked", pp.flags, pp.atk, atk)
	}
	if s.g.attacked != attacked {
		t.Fatalf("attacked count %v, want %v (nothing declared)", s.g.attacked, attacked)
	}
	life := s.pl[1].life
	sc.passUntil(t, kop.StepMain2)
	if s.pl[1].life != life-3 {
		t.Fatalf("defender life %d, want %d (the sneaked 3/3 connects)", s.pl[1].life, life-3)
	}
}

// TestSneakCastSorcery: a sneaked sorcery resolves normally in the
// declare-blockers step; no entry hook.
func TestSneakCastSorcery(t *testing.T) {
	sc, _, so := ksneakScen(t)
	o := sc.hand(so, 0)
	bear := sc.perm(v.GrizzlyBears, 0)
	ksneakToBlockers(t, sc, []Obj{bear}, nil)
	s := sc.s
	hand := s.pl[0].handN
	ksneakCast(t, sc, o, bear)
	sc.passBoth(t)
	if s.objs[o].zone != kop.ZoneGraveyard {
		t.Fatalf("sneaked sorcery zone %d, want graveyard", s.objs[o].zone)
	}
	// -1 the sorcery, +1 the bear, +2 drawn
	if s.pl[0].handN != hand+2 {
		t.Fatalf("hand %d, want %d", s.pl[0].handN, hand+2)
	}
}

// TestSneakSinglePickAsked: with two attackers the pick lists both; with
// one it is still asked (gorge returnAsk).
func TestSneakSinglePickAsked(t *testing.T) {
	sc, cr, _ := ksneakScen(t)
	o := sc.hand(cr, 0)
	b1 := sc.perm(v.GrizzlyBears, 0)
	b2 := sc.perm(v.GrizzlyBears, 0)
	ksneakToBlockers(t, sc, []Obj{b1, b2}, nil)
	if n := ksneakCast(t, sc, o, b2); n != 2 {
		t.Fatalf("return pick has %d candidates, want 2", n)
	}
	// TestSneakCastCreature covers the single candidate asked
}

// TestSneakReversal: a sneak cast reversed before payment returns
// nothing: the attacker stays on the battlefield, attacking (CR 733.1).
func TestSneakReversal(t *testing.T) {
	sc, cr, _ := ksneakScen(t)
	o := sc.hand(cr, 0)
	bear := sc.perm(v.GrizzlyBears, 0)
	ksneakToBlockers(t, sc, []Obj{bear}, nil)
	s := sc.s
	sc.do(t, AkSneak, TCode(o), scAny)
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChCostReturn {
		t.Fatalf("decision kind %d sub %d, want the return pick", d.Kind, d.Sub)
	}
	s.reverseCast(s.k())
	if s.objs[o].zone != kop.ZoneHand {
		t.Fatalf("reversed sneak card zone %d, want hand", s.objs[o].zone)
	}
	if !sc.onBF(bear) || s.perms[sc.slot(bear)].flags&pfAttacking == 0 {
		t.Fatalf("attacker moved by a reversed sneak cast")
	}
}

// TestSneakCarrierCoverage casts every carrier in the pool for its sneak
// cost from a constructed declare-blockers window: each is offered and the
// stack entry is marked as a sneak cast. The lockstep tally counts the same
// casts through gorge's CastInfo; this pins the kernel side so a carrier
// that never appears in the random games cannot hide a dead sneak arm.
func TestSneakCarrierCoverage(t *testing.T) {
	set := cardsettest.Set(t, "sneak")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	plains := kop.CardID(0)
	for i, n := range set.Names() {
		if n == "Plains" {
			plains = set.PoolID(i)
		}
	}
	carriers := []string{
		"Donatello's Technique", "Donatello, Gadget Master",
		"Elektra, Daughter of the Hand", "Jennika's Technique",
		"Leonardo's Technique", "Leonardo, Big Brother",
		"Leonardo, Cutting Edge", "New Generation's Technique",
		"Oroku Saki, Shredder Rising", "Raphael, the Nightwatcher",
		"Shredder, Unrelenting", "Splinter's Technique",
		"Splinter, Hamato Yoshi",
	}
	id := func(name string) kop.CardID {
		for i, n := range set.Names() {
			if n == name {
				return set.PoolID(i)
			}
		}
		t.Fatalf("%s not in the sneak pool", name)
		return 0
	}
	attacker := id("Savannah Lions")
	for _, name := range carriers {
		t.Run(name, func(t *testing.T) {
			sc := newScenTableLib(t, tbl, plains, Config{NoElide: true, GorgeUnpriced: true})
			o := sc.hand(id(name), 0)
			atk := sc.perm(attacker, 0)
			s := sc.s
			s.step = kop.StepBlockers
			s.active = 0
			s.prio = 0
			s.passes = 0
			s.ctl = ctlPriority
			s.perms[sc.slot(atk)].flags |= pfAttacking
			// some carrier spells target a creature card in the graveyard
			gv := sc.obj(attacker, 0)
			s.place(gv, kop.ZoneGraveyard, false)
			def := tbl.Cards[id(name)]
			for k := 0; k < kop.NumManaKinds; k++ {
				s.pl[0].pool[k] = def.SneakCost.Pips[k]
			}
			s.pl[0].pool[kop.W] += uint8(def.SneakCost.Generic)
			var d Decision
			s.priorityOptions(s.k(), &d)
			if !ksneakOffered(&d, o) {
				t.Fatalf("sneak not offered: %v", d.Options[:d.N])
			}
			sc.do(t, AkSneak, TCode(o), scAny)
			ksneakReturn(t, sc, atk)
			if e := &s.stack[s.sp-1]; e.trigPl != trigPlSneak {
				t.Fatalf("stack entry trigPl %#x, want sneak", e.trigPl)
			}
		})
	}
}

// ksneakReturn answers the return pick with ret.
func ksneakReturn(t *testing.T, sc *scen, ret Obj) {
	t.Helper()
	for i := 0; i < 8; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChCostReturn {
			sc.s.Apply(MakeAction(AkChoose, TCode(ret), 0, 0))
			return
		}
		sc.s.Apply(d.Options[0])
	}
	t.Fatalf("no return pick")
}
