package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "Whenever one or more opponents each lose exactly 1 life" (gorge Mode$
// LifeLostAll, trigmatch/life.go lifeLostMatches; Ob Nixilis, Captive
// Kingpin). gorge defers the mode inside a simultaneous life-loss group
// (rules/life_batch.go Begin/EndLifeLossBatch) and matches it once as the
// group closes, against each player's summed loss: a player ValidPlayer$
// names who lost a nonzero amount that fails ValidAmountEach$ fails the whole
// line, and the line fires when at least one named player lost life. Outside
// a group each loss is matched alone, its amount the event's.
//
// The kernel's groups mirror the gorge brackets whose amounts can differ from
// the per-event reading at two seats: the combat damage pass (gorge
// combat.go damageStep .. runCombatAssignments: every attacker's hit on the
// defending player sums, CR 510.2), OpLoseLife (effLoseLife) and
// ExtEachDamage (effEachDamage: several damagers can hit one player). The
// other gorge groups (DealDamage, DamageAll) lose life at most once per
// player, so their one loss is matched alone here, as it would be at the
// group's close.
//
// TrigLifeLostAll sits past nSubEvents (no slot subscription): the firing
// walk scans the battlefield as TrigProliferate's does. Trigger.N is
// ValidAmountEach$'s EQ literal (0: any amount).

// useLifeLostAll is the table's genPrep.uses bit for a LifeLostAll line.
var useLifeLostAll = newGenUse(func(t *kop.Table) bool {
	for ci := range t.Cards {
		ab := t.Cards[ci].Abilities
		for ai := range ab {
			if a := &ab[ai]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigLifeLostAll {
				return true
			}
		}
	}
	return false
})

// lossBatch is the open life-loss group and each seat's summed loss in it,
// saturating at 255 (ValidAmountEach$'s literal is far below it). One record
// in the cold buffer of a table with a LifeLostAll line, so State itself does
// not grow (TestNoNewFixedStateFields); the group spans a combat pass's
// replacement-order pause there.
type lossBatch struct {
	on  bool
	amt [MaxSeats]uint8
}

var coldLossB = newCold[lossBatch]("lossB", func(g *genPrep) int {
	if g.uses.has(useLifeLostAll) {
		return 1
	}
	return 0
})

// lossB is the table's group record, nil without a LifeLostAll line.
func (s *State) lossB() *lossBatch {
	if r := coldLossB.of(s); len(r) != 0 {
		return &r[0]
	}
	return nil
}

// lossBatchOpen starts a life-loss group; it reports whether this call opened
// it (a group already open swallows the inner bracket, as gorge's depth does).
func (s *State) lossBatchOpen(k *prep) bool {
	if k.gen == nil || !k.gen.uses.has(useLifeLostAll) {
		return false
	}
	b := s.lossB()
	if b == nil || b.on {
		return false
	}
	*b = lossBatch{on: true}
	return true
}

// lossBatchClose ends the open group and matches the LifeLostAll lines
// against its sums.
func (s *State) lossBatchClose(k *prep) {
	b := s.lossB()
	if b == nil || !b.on {
		return
	}
	amt := b.amt
	*b = lossBatch{}
	s.fireLifeLostAll(k, &amt)
}

// lifeLostAllLoss records seat p's loss of n: summed into an open group, or
// matched alone.
func (s *State) lifeLostAllLoss(k *prep, p uint8, n int) {
	if b := s.lossB(); b != nil && b.on {
		b.amt[p] = uint8(min(int(b.amt[p])+n, 255))
		return
	}
	var amt [MaxSeats]uint8
	amt[p] = uint8(min(n, 255))
	s.fireLifeLostAll(k, &amt)
}

// fireLifeLostAll queues every battlefield LifeLostAll line that matches the
// group's sums.
//
//go:noinline
func (s *State) fireLifeLostAll(k *prep, amt *[MaxSeats]uint8) {
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigLifeLostAll }, func(l trigListener, a *kop.Ability) {
			if who, ok := lifeLostAllMatch(&a.Trig, l.ctrl, amt, s.nSeats()); ok {
				s.playerTrigGen(k, kop.TrigLifeLostAll, l, a, who, int16(amt[who]))
			}
		})
	}
}

// lifeLostAllMatch is gorge lifeLostMatches' group arm: the first player the
// line names who lost life, false when none did or one of them lost an amount
// other than the line's EQ literal.
func lifeLostAllMatch(t *kop.Trigger, ctrl uint8, amt *[MaxSeats]uint8, seats uint8) (uint8, bool) {
	who, matched := uint8(0), false
	for p := uint8(0); p < seats; p++ {
		if amt[p] == 0 || !whoseOK(t.Whose, ctrl, p) {
			continue
		}
		if t.N != 0 && int16(amt[p]) != t.N {
			return 0, false
		}
		if !matched {
			who, matched = p, true
		}
	}
	return who, matched
}
