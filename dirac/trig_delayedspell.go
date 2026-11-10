package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Delayed SpellCast triggers (CR 603.7, 603.7b):
// "when you next cast an instant or sorcery spell this turn, copy that
// spell" (gorge effDelayedTriggerSpellCast). The registration is an
// ordinary dprog record flagged dpSpell; its filter is the latent ability's
// Trig.Filter. It fires on its controller's next matching cast (gorge
// checkEventDelayedTriggers, after the cast's face triggers and before its
// cascade triggers), once -- gorge consumes it at its DelayedPush, and no
// cast can come between the queue and the push -- and is dropped at the
// next turn change (ThisTurn$ True: gorge collectExpiredDelayedTriggers).
// A spell copy is never cast (mintSpellCopy does not reach onCast), so it
// never fires one, as gorge's PutOnStack-only match.

// dpSpell marks a dprog record a delayed SpellCast registration
// (kop.DelaySpellCast); its timing bits stay 0. Bits 1-5 of dprog.n are
// free of the object count (bit 0) and the timing (bits 6-7).
const dpSpell uint8 = 1 << 5

// Debug switches for TestDelayspellMutation; never set in production.
var (
	DebugDelaySpellKeep      bool // a fired registration is not removed
	DebugDelaySpellNoExpire  bool // no drop at the turn change
	DebugDelaySpellAnyCaster bool // fires on any player's cast
)

// tableDelaysSpell reports whether some program of t registers a delayed
// SpellCast program (genPrep.anyDelaySpell).
func tableDelaysSpell(t *Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpDelayedProgram && (in.A.N == kop.DelaySpellCast || in.A.N == kop.DelaySpellUncounter) {
					return true
				}
			}
		}
	}
	return false
}

// fireDelayedSpell enqueues, in registration order, every delayed SpellCast
// registration of e's caster whose filter matches e, with e as the
// triggering spell, and removes it; the rest stay in order with their LKI
// indexes (as fireDelayedWhen).
//
//go:noinline
func (s *State) fireDelayedSpell(k *prep, e *stackEntry) {
	s.dropUncounterGone()
	n := int(s.g.ndp)
	keep := 0
	for i := 0; i < n; i++ {
		d := s.g.dp[i]
		// The filter's source-relative bound (Loki's cmcLEX) reads the
		// registration's source, not the resolving entry's context: at a
		// cast nothing is resolving, and a source off the battlefield has
		// no slot (NoSlot reads its card's printed power, as the cast does
		// in gorge's spec context).
		srcSl := NoSlot
		if d.src != NoObj {
			srcSl = s.slotIfSame(d.src, s.objs[d.src].ver)
		}
		ctx := evalCtx{src: d.src, srcSl: srcSl, ctrl: d.ctrl}
		var se stackEntry
		if srcSl == NoSlot && d.src != NoObj {
			// the source has left: its bound reads the departure snapshot
			// (selfPTLKI skips a seSpell entry, so the local one is a trigger)
			if v := s.objs[d.src].ver; v > 0 {
				se = stackEntry{kind: seTrigger, obj: d.src, srcVer: v - 1}
				if s.srcDepartIdx(&se) != 0 {
					ctx.e = &se
				}
			}
		}
		due := d.n&dpSpell != 0 && (d.ctrl == e.ctrl || DebugDelaySpellAnyCaster) &&
			s.matchCardGenAt(k, k.t.Cards[d.card].Abilities[d.ab].Trig.Filter, e.card, e.ctrl, d.ctrl, e.obj, &ctx)
		if due && d.ext == dpExtUncounter {
			// gorge's Static$ fire: the body resolves at the match with no
			// stack object; the record becomes the Effect it creates
			// (repl_uncounterable.go), held while e stays on the stack
			d = dprog{card: d.card, ab: d.ab, ctrl: d.ctrl, src: d.src, n: 1, ext: dpExtUncounter}
			d.objs[0], d.vers[0] = e.obj, s.objs[e.obj].ver
			s.g.dp[keep], s.g.dpLKI[keep] = d, 0
			keep++
			continue
		}
		if due {
			m := s.npend
			s.enqueue(d.src, s.objs[d.src].ver, d.card, d.ab, d.ctrl, e.obj, e.ctrl, 0)
			if s.npend > m {
				s.pend[m].li = s.g.dpLKI[i]
			}
		}
		if !due || DebugDelaySpellKeep {
			s.g.dp[keep], s.g.dpLKI[keep] = d, s.g.dpLKI[i]
			keep++
		}
	}
	for i := keep; i < n; i++ {
		s.g.dp[i], s.g.dpLKI[i] = dprog{}, 0
	}
	s.g.ndp = uint8(keep)
}

// dropDelayedSpell drops every delayed SpellCast registration at the turn
// change (ThisTurn$ True), keeping the other records in order. A fired
// uncounter record has no turn bound: it stays while its spell is on the
// stack (gorge's Effect forgets the spell only as it leaves).
//
//go:noinline
func (s *State) dropDelayedSpell() {
	s.dropUncounterGone()
	if DebugDelaySpellNoExpire {
		return
	}
	n := int(s.g.ndp)
	keep := 0
	for i := 0; i < n; i++ {
		if s.g.dp[i].n&dpSpell == 0 {
			s.g.dp[keep], s.g.dpLKI[keep] = s.g.dp[i], s.g.dpLKI[i]
			keep++
		}
	}
	for i := keep; i < n; i++ {
		s.g.dp[i], s.g.dpLKI[i] = dprog{}, 0
	}
	s.g.ndp = uint8(keep)
}

// dprogReferenced reports whether a delayed program names o as its source
// (objReferenced: a token source is not recycled while a record names it),
// a fired uncounter record excepted (gorge's Effect is its own source), or
// a delayed dies program remembers it (gorge matches the id, which is
// never reused; trig_delayeddies.go).
//
//go:noinline
func (s *State) dprogReferenced(o Obj) bool {
	for i := 0; i < int(s.g.ndp); i++ {
		if d := &s.g.dp[i]; d.src == o && !dpUncounterFired(d) || d.n&dpDies != 0 && d.objs[0] == o {
			return true
		}
	}
	return false
}
