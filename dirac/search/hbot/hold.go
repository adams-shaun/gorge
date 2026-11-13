package hbot

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// hold.go: gorge's opt-in TimingRules.HoldInstant (botpolicy timing.go's
// !instantNow). The rule holds an instant-speed cast outside the windows
// where it is worth casting: an opponent's top of stack, an opponent's main
// 2 or end step, or a combat trick during combat. A held card is never a
// cast ranking candidate and never a mana-tap intent.

// instantHeld is botpolicy's !instantNow: c's cast is held right now. It is
// evaluated in gorge's order and returns at the first match. The Land term
// is omitted: a land is never a cast candidate and has cmc 0, and every
// tap-intent fold skips cmc <= 0.
func (b *board) instantHeld(me uint8, c *cardF) bool {
	if !c.instant {
		return false
	}
	if c.f.FaceDmg > 0 && b.oppLifeAtMost(me, int32(c.f.FaceDmg)) {
		return false
	}
	if b.nStack > 0 && b.stack[b.nStack-1].ctrl != me {
		return false
	}
	if c.counter {
		return true
	}
	if !b.myTur && (b.step == kop.StepMain2 || b.step == kop.StepEnd) {
		return false
	}
	return !(c.f.Trick && b.step >= kop.StepBlockers && b.step <= kop.StepEndCombat)
}

// timedTapWould is gorge's timedTap read as a tap gate: outside a main phase
// the manual seat may float mana toward the best non-held, castable-now
// instant the pool does not pay. The kernel has no manual mana, so the
// caller answers with the manual list instead. No scoring is needed:
// gorge's tapToward taps for any intent card when a source is offered.
func (bt *Bot) timedTapWould(s *dirac.State, d *dirac.Decision) bool {
	if s.FeatManaSources(d.Player) == 0 {
		return false
	}
	b := &bt.b
	off := bt.offeredColours(s, d.Player)
	for i := 0; i < b.nCard; i++ {
		c := &b.card[i]
		if !c.own || !c.castable || !c.instant || c.cmc <= 0 || !bt.tapTarget(d, c) {
			continue
		}
		if b.poolPaysCard(c) || !b.offeredClosesPips(c, off) {
			continue
		}
		if bt.castableNowCard(d.Player, c) && !b.instantHeld(d.Player, c) {
			return true
		}
	}
	return false
}
