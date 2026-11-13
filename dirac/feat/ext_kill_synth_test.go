package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// killFix builds a hand-made position for extKill: the seat is 0, "me" is
// players[0], "opp" players[1]. It exists to pin the K rules that the random
// games reach too rarely to fail a wrong rule (evasive clocks, reach, 488,
// same-controller counters).
type killFix struct {
	e *Encoder
}

func newKillFix(meLife, oppLife int32) *killFix {
	e := &Encoder{}
	e.v.n = 2
	e.v.players[0].life, e.v.players[1].life = meLife, oppLife
	return &killFix{e: e}
}

// creature puts a creature on pl's battlefield.
func (f *killFix) creature(pl uint8, obj uint8, power int32, kw kop.KwMask, tapped bool) {
	f.e.v.cards = append(f.e.v.cards, vcard{
		vcore: &vcore{c: &card{creature: true}, ctrl: pl, power: power, kw: kw, tapped: tapped},
		obj:   obj,
	})
	pv := &f.e.v.players[pl]
	pv.bf = append(pv.bf, int32(len(f.e.v.cards)-1))
}

// hand puts a spell card of the given effect in pl's hand.
func (f *killFix) hand(pl uint8, mv int32, ef EffectSpec) {
	f.e.v.cards = append(f.e.v.cards, vcard{vcore: &vcore{c: &card{mv: mv, eff: ef}, ctrl: pl}})
	pv := &f.e.v.players[pl]
	pv.hand = append(pv.hand, int32(len(f.e.v.cards)-1))
}

// spell pushes a stack spell (bottom first) controlled by ctrl, with source
// object src and the given target bytes, returning its stack index.
func (f *killFix) spell(ctrl, src uint8, ef EffectSpec, tgt ...uint8) int {
	f.e.v.cards = append(f.e.v.cards, vcard{vcore: &vcore{c: &card{eff: ef}, ctrl: ctrl}})
	f.e.v.stack = append(f.e.v.stack, vstack{cv: int32(len(f.e.v.cards) - 1), ctrl: ctrl})
	se := &f.e.kp.Stack[f.e.kp.NStack]
	se.Kind, se.Source, se.Ctrl = 0, dirac.Obj(src), ctrl
	for i := range se.Targets {
		se.Targets[i] = dirac.TCode(dirac.NoTarget)
	}
	for i, c := range tgt {
		se.Targets[i] = dirac.TCode(c)
	}
	f.e.kp.NStack++
	return int(f.e.kp.NStack) - 1
}

// run encodes group K for seat 0 with the given mz mine-attack power.
func (f *killFix) run(minePower int32) {
	f.e.extKill(0, &f.e.v.players[0], &f.e.v.players[1], &agg{canAttackPower: minePower}, &agg{})
}

func (f *killFix) set(slot int) bool { return f.e.extAcc[slot] != 0 }

// onlySlot requires exactly the slot in [base, base+9) to be set.
func (f *killFix) onlySlot(t *testing.T, base, want int, what string) {
	t.Helper()
	for j := 0; j <= 8; j++ {
		if got := f.set(base + j); got != (j == want) {
			t.Fatalf("%s: slot %d set=%v, want %v", what, base+j, got, j == want)
		}
	}
}

var (
	meP  = dirac.PlayerTarget0     // the seat-0 player target
	oppP = dirac.PlayerTarget0 + 1 // the seat-1 player target
)

func burnSpec(amount int16) EffectSpec {
	return EffectSpec{Class: EffDamage, Amount: amount, TgtPlayer: true, TgtCreat: true}
}

// TestExtKillCounterSamePlayer: a counter only prevents lethal when it and
// the countered entry belong to different players. The opponent countering
// its own lethal burn must not set 490 or 491, nor my counter on my own
// damage; the cross-side counter is the positive control.
func TestExtKillCounterSamePlayer(t *testing.T) {
	counter := EffectSpec{Class: EffCounter, Amount: -2}

	// control: my counter on the opponent's lethal burn -> 490, not 491
	f := newKillFix(3, 20)
	f.spell(1, 10, burnSpec(3), uint8(meP))
	f.spell(0, 11, counter, 10)
	f.run(0)
	if !f.set(xKStackOppDmgMe) || !f.set(xKCounterMe) || f.set(xKCounterOpp) {
		t.Fatalf("control (my counter on their burn): 486=%v 490=%v 491=%v, want true true false",
			f.set(xKStackOppDmgMe), f.set(xKCounterMe), f.set(xKCounterOpp))
	}

	// the opponent counters its own lethal burn: neither side is saved
	f = newKillFix(3, 20)
	f.spell(1, 10, burnSpec(3), uint8(meP))
	f.spell(1, 11, counter, 10)
	f.run(0)
	if !f.set(xKStackOppDmgMe) {
		t.Fatal("same-side opp: 486 not set, the precondition of the test")
	}
	if f.set(xKCounterMe) || f.set(xKCounterOpp) {
		t.Fatalf("opponent counters its own burn: 490=%v 491=%v, want both clear", f.set(xKCounterMe), f.set(xKCounterOpp))
	}

	// I counter my own lethal damage
	f = newKillFix(20, 3)
	f.spell(0, 10, burnSpec(3), uint8(oppP))
	f.spell(0, 11, counter, 10)
	f.run(0)
	if !f.set(xKStackMeDmgOpp) {
		t.Fatal("same-side me: 487 not set, the precondition of the test")
	}
	if f.set(xKCounterMe) || f.set(xKCounterOpp) {
		t.Fatalf("I counter my own burn: 490=%v 491=%v, want both clear", f.set(xKCounterMe), f.set(xKCounterOpp))
	}

	// mirror control: their counter on my lethal burn -> 491, not 490
	f = newKillFix(20, 3)
	f.spell(0, 10, burnSpec(3), uint8(oppP))
	f.spell(1, 11, counter, 10)
	f.run(0)
	if !f.set(xKCounterOpp) || f.set(xKCounterMe) {
		t.Fatalf("mirror control: 490=%v 491=%v, want false true", f.set(xKCounterMe), f.set(xKCounterOpp))
	}
}

// TestExtKillEvasiveClock: the evasive clock counts only flyers, and is off
// (bucket 8) once the defender controls a flyer or a reach creature.
func TestExtKillEvasiveClock(t *testing.T) {
	// me: a 3-power flyer and a 2-power ground creature; opp life 9, no
	// flying/reach blockers, no creatures at all. plain P=5 -> t=2, evasive
	// P=3 -> t=3.
	f := newKillFix(20, 9)
	f.creature(0, 1, 3, kop.KwFlying, false)
	f.creature(0, 2, 2, 0, false)
	f.run(0)
	f.onlySlot(t, xKClockMe, 2, "plain clock")
	f.onlySlot(t, xKEvClockMe, 3, "evasive clock, no blockers")
	// opp has no creatures: no clock for it
	f.onlySlot(t, xKEvClockOpp, 8, "opp evasive clock with no creatures")

	// a reach creature on the opponent's side turns my evasive clock off
	f = newKillFix(20, 9)
	f.creature(0, 1, 3, kop.KwFlying, false)
	f.creature(0, 2, 2, 0, false)
	f.creature(1, 3, 1, kop.KwReach, false)
	f.run(0)
	f.onlySlot(t, xKEvClockMe, 8, "evasive clock vs a reach blocker")
	f.onlySlot(t, xKClockMe, 2, "plain clock is unaffected by reach")

	// and so does a flyer
	f = newKillFix(20, 9)
	f.creature(0, 1, 3, kop.KwFlying, false)
	f.creature(1, 3, 1, kop.KwFlying, true)
	f.run(0)
	f.onlySlot(t, xKEvClockMe, 8, "evasive clock vs a flying blocker (tapped still counts)")

	// the opponent's side: 4-power flyer vs my life 8, I have no flyer/reach
	f = newKillFix(8, 20)
	f.creature(1, 3, 4, kop.KwFlying, false)
	f.creature(0, 1, 2, 0, false)
	f.run(0)
	f.onlySlot(t, xKEvClockOpp, 2, "opp evasive clock")
	// ...but my reach creature stops it
	f = newKillFix(8, 20)
	f.creature(1, 3, 4, kop.KwFlying, false)
	f.creature(0, 1, 2, kop.KwReach, false)
	f.run(0)
	f.onlySlot(t, xKEvClockOpp, 8, "opp evasive clock vs my reach")
}

// TestExtKillBurnReach: 484 is mine.canAttackPower + the castable hand burn
// >= opp life; 485 is hand burn + my stacked damage at the opponent.
func TestExtKillBurnReach(t *testing.T) {
	build := func(oppLife int32) *killFix {
		f := newKillFix(20, oppLife)
		f.e.v.players[0].avail = 2
		f.hand(0, 1, burnSpec(2))
		f.hand(0, 1, burnSpec(3))
		f.hand(0, 3, burnSpec(9)) // uncastable: mv 3 > avail 2
		return f
	}
	// hand burn 5, attack power 3: 8 reaches life 8 but not 9
	f := build(8)
	f.run(3)
	if !f.set(xKReach) {
		t.Fatal("484 clear at 3 attack + 5 burn vs life 8")
	}
	if f.set(xKBurnReach) {
		t.Fatal("485 set at 5 burn vs life 8 with nothing stacked")
	}
	f = build(9)
	f.run(3)
	if f.set(xKReach) {
		t.Fatal("484 set at 3 attack + 5 burn vs life 9")
	}
	// 485: hand 5 + a stacked 3 aimed at the opponent
	f = build(8)
	f.spell(0, 10, burnSpec(3), uint8(oppP))
	f.run(0)
	if !f.set(xKBurnReach) {
		t.Fatal("485 clear at 5 hand burn + 3 stacked vs life 8")
	}
	// the same 3 aimed at me does not count
	f = build(8)
	f.spell(0, 10, burnSpec(3), uint8(meP))
	f.run(0)
	if f.set(xKBurnReach) {
		t.Fatal("485 set for stacked damage aimed at me")
	}
	f = build(9)
	f.spell(0, 10, burnSpec(3), uint8(oppP))
	f.run(0)
	if f.set(xKBurnReach) {
		t.Fatal("485 set at 8 total vs life 9")
	}
}

// TestExtKillRemovesLastBlocker: 488 needs an opponent kill/bounce aimed at
// my only untapped creature while the opponent's next-turn power is lethal.
func TestExtKillRemovesLastBlocker(t *testing.T) {
	kill := EffectSpec{Class: EffKill, TgtCreat: true}
	build := func(meLife int32, second bool, target uint8) *killFix {
		f := newKillFix(meLife, 20)
		f.creature(0, 1, 2, 0, false)
		if second {
			f.creature(0, 2, 2, 0, false)
		} else {
			f.creature(0, 2, 2, 0, true) // tapped: not a blocker
		}
		f.creature(1, 5, 4, 0, false)
		f.spell(1, 10, kill, target)
		f.run(0)
		return f
	}
	if f := build(4, false, 1); !f.set(xKStackOppKills) {
		t.Fatal("488 clear: kill on my only untapped creature, opp P 4 >= life 4")
	}
	if f := build(5, false, 1); f.set(xKStackOppKills) {
		t.Fatal("488 set although opp P 4 < life 5")
	}
	if f := build(4, true, 1); f.set(xKStackOppKills) {
		t.Fatal("488 set with two untapped creatures")
	}
	if f := build(4, false, 2); f.set(xKStackOppKills) {
		t.Fatal("488 set for a kill on a tapped creature, not the last blocker")
	}
}
