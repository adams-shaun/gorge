package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// trigger parameters Teamwork$ True (a Taps line that
// fires only for a tap paying a teamwork cost, gorge trigmatch/tap.go:41)
// and GameActivationLimit$ 1 (a line that triggers once per source object,
// gorge trigger_match.go triggerGameActivationLimitAllows; the count is
// committed as the trigger is QUEUED, and a zone change starts a new
// object, CR 400.7). The game records ride the g.used pool (no State
// bytes); an overflow stalls loud (StallUsed).

var DebugTWTapAny bool // mutation: a Teamwork$ Taps line fires on every tap

var DebugGameOnceOff bool // mutation: GameActivationLimit is ignored

const gameOnceMark = 0x40 // usedRec.ab = gameOnceMark|ab (ab < 0x3E; valiantMark 0x7E, rlMark 0x7F, resolveMark 0x80+)

const twTapBit = 0x100 // rides fireTapsBy's amt above the tapper byte

// mtTapBit rides the amt of a TrigTaps fire for a tap paying a mana ability's
// cost (manaTapped): the only taps a Tf5TapsForMana line (gorge Mode$
// TapsForMana, trigmatch tapsMatches' forMana flag) admits.
const mtTapBit = 0x200

// tapsManaOK: a Tf5TapsForMana line admits only a mana tap (mtTapBit), and
// with Tf5TapperNonActive only one by a player other than the active player
// (Activator$ Player.NonActive; the tapper is fireTapsBy's low byte).
func (s *State) tapsManaOK(a *kop.Ability, amt int16) bool {
	f := a.Trig.Flags5
	if f&kop.Tf5TapsForMana == 0 {
		return true
	}
	return amt&mtTapBit != 0 && (f&kop.Tf5TapperNonActive == 0 || uint8(amt) != s.active)
}

// fireTapsTW is fireTaps for a creature tapped to pay a teamwork cost.
//
//go:noinline
func (s *State) fireTapsTW(k *prep, sl Slot) {
	p := &s.perms[sl]
	s.fireObj(k, kop.TrigTaps, sl, p.obj, p.ctrl, int16(p.ctrl)|twTapBit)
}

// twTapOK: a TrigNTeamworkTap line admits only a teamwork tap.
func twTapOK(a *kop.Ability, amt int16) bool {
	return DebugTWTapAny || a.Trig.N != kop.TrigNTeamworkTap || amt&twTapBit != 0
}

// gameOnceUsed / markGameOnce: the source object's game record.
func (s *State) gameOnceUsed(l trigListener) bool {
	return !DebugGameOnceOff && s.usedThisTurn(l.obj, gameOnceMark|l.ab)
}

func (s *State) markGameOnce(l trigListener) {
	s.markUsed(l.obj, gameOnceMark|l.ab)
}

// keepGameOnce is the turn reset of the used pool when a GameOnce line
// exists: keep the gameOnceMark records whose object is still the same
// battlefield object (ver unchanged, zone battlefield), drop the rest.
//
//go:noinline
func (s *State) keepGameOnce() {
	n := 0
	for i := 0; i < int(s.g.nused); i++ {
		r := s.g.used[i]
		if r.ab >= gameOnceMark && r.ab < 0x7E && r.obj < MaxObj &&
			s.objs[r.obj].ver == r.ver && s.objs[r.obj].zone == kop.ZoneBattlefield {
			s.g.used[n] = r
			n++
		}
	}
	for i := n; i < int(s.g.nused); i++ {
		s.g.used[i] = usedRec{}
	}
	s.g.nused = uint8(n)
}

// dropPendingLimits cuts the pending queue to from (a reversed cast's
// CR 733.1 drop, unpushSpell / unpushActivation) and, for every dropped
// entry, gives back the once-per-turn record its line consumed as it was
// queued (gorge abortCast's proposalLimits: "an undone action triggered
// nothing (CR 733.1), so a once-each-turn trigger it queued has not used
// its turn").
func (s *State) dropPendingLimits(k *prep, from uint8) {
	s.dropPendingRange(k, from, s.npend)
}

// dropPendingRange is dropPendingLimits over the entries [from, hi) only: the
// ones after hi move down, in order, to from.
func (s *State) dropPendingRange(k *prep, from, hi uint8) {
	for i := int(from); i < int(hi); i++ {
		pt := s.pend[i]
		if pt.ab >= abCascade {
			continue // a kernel-native trigger line has no limit$
		}
		a := &k.t.Cards[pt.card].Abilities[pt.ab]
		if a.Kind != kop.AbTriggered {
			continue
		}
		if k.gen != nil && k.gen.anyGameOnce && a.Trig.Flags3&kop.Tf3GameOnce != 0 {
			s.unmarkUsed(pt.obj, gameOnceMark|pt.ab)
		}
		if a.Trig.Flags&kop.TfOncePerTurn != 0 {
			s.unmarkUsed(pt.obj, pt.ab)
		}
	}
	n := copy(s.pend[from:], s.pend[hi:s.npend])
	s.npend = from + uint8(n)
}

// unmarkUsed removes the newest once-per-turn record for (o, ab) — the one
// queueTrig appended when the dropped entry was queued (usedThisTurn reads
// the pool linearly, so record order is not observable).
func (s *State) unmarkUsed(o Obj, ab uint8) {
	for i := int(s.g.nused) - 1; i >= 0; i-- {
		if s.g.used[i].obj == o && s.g.used[i].ab == ab {
			copy(s.g.used[i:], s.g.used[i+1:s.g.nused])
			s.g.used[s.g.nused-1] = usedRec{}
			s.g.nused--
			return
		}
	}
}

// selfDiscLine: a card's own "when you discard this card" trigger (gorge
// trigger_zone_gate.go: a Mode$ Discarded line without TriggerZones$ fires
// on its own card's discard wherever the discard put it). kgen lowers it as
// TrigDiscarded with TfSelf and Zone graveyard; a Cycled line is
// fireCycled's.
func selfDiscLine(a *kop.Ability) bool {
	t := &a.Trig
	return t.Event == kop.TrigDiscarded && t.Flags&kop.TfSelf != 0 && t.Zone == kop.ZoneGraveyard && t.Flags2&kop.Tf2Cycled == 0
}

// fireSelfDisc fires the discarded card's own selfDiscLine triggers, in
// ability order; its owner, who discarded it, controls them. effect is false
// for a cost discard, which has no causing spell or ability (gorge
// trigmatch.DiscardCauseAdmits): a Tf5OppCause line fires only when the
// resolving spell or ability is an opponent's of the owner's.
//
//go:noinline
func (s *State) fireSelfDisc(k *prep, o Obj, effect bool) {
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbTriggered || !selfDiscLine(a) {
			continue
		}
		p := s.objs[o].owner()
		if a.Trig.Flags5&kop.Tf5OppCause != 0 && !(effect && s.res.active && s.res.e.ctrl != p) {
			continue
		}
		l := trigListener{obj: o, sl: NoSlot, ctrl: p, card: c, ab: uint8(j)}
		if !s.trigOK(k, l, a) {
			continue
		}
		s.queueTrig(k, l, a, o, p, 0)
	}
}

// the single-target trigger qualifiers. Cold and out of
// line; the hot sites test Trigger.Flags4 once.
//
//   - singleTargetOK is IsSingleTarget$ True (gorge trigmatch.TargetShapeMatches):
//     the triggering spell's whole target chain (root Targets plus every
//     SubAbility$ link's SubTargets, gorge) names exactly one DISTINCT
//     target; a multi-pick slot counts every pick. See trig_targetchain.go.
//   - singleCauseOK is DamageDone's ValidCause$ Spell.numTargets EQ1 +
//     TargetRelativeToCause$ Creature.targetedBy (gorge damageCauseMatches,
//     cause_qualifiers.go L71-101): the cause is the resolving spell
//     (stackCauseAdmits L24-40), it carries exactly one target in total
//     (Targets plus SubTargets, targetCountAdmits L43-50), and the damaged
//     creature is that target. Outside a resolution, from an ability, from a
//     noncreature recipient or with any other target count it fails.
var DebugTrigParams4Flip bool

// singleTargetOK reports whether e's target chain names exactly one distinct
// target (chainTargets, distinctTargets). e is a spell being cast
// (castTrigGen); a mid-resolution declaration has not been chosen yet, so only
// cast-time slots are set.
//
//go:noinline
func (s *State) singleTargetOK(k *prep, e *stackEntry) bool {
	var codes [kop.MaxTargets + maxXTgt]TCode
	return (distinctTargets(&codes, s.chainTargets(e, &codes)) == 1) != DebugTrigParams4Flip
}

// singleCauseOK reports whether the resolving spell caused the damage to
// recipient and carries exactly one target, the recipient among it.
//
//go:noinline
func (s *State) singleCauseOK(k *prep, recipient TCode) bool {
	ok := false
	if s.res.active && s.res.e.kind == seSpell && isObjTarget(recipient) {
		ob := &s.objs[recipient]
		if ob.zone == kop.ZoneBattlefield && s.isCreature(k, ob.slot) {
			e := &s.res.e
			n, hit := 0, false
			for i := 0; i < int(e.nt); i++ {
				c := e.tgt[i]
				if c == NoTarget {
					continue
				}
				n++
				hit = hit || c == recipient
			}
			x := s.xtOf(e) // Targets+SubTargets: gorge counts every pick too
			for j := 0; j < int(x.n); j++ {
				c := x.code[j]
				if c == NoTarget {
					continue
				}
				n++
				hit = hit || c == recipient
			}
			ok = n == 1 && hit
		}
	}
	return ok != DebugTrigParams4Flip
}
