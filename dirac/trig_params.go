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
// ability order; its owner, who discarded it, controls them.
//
//go:noinline
func (s *State) fireSelfDisc(k *prep, o Obj) {
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbTriggered || !selfDiscLine(a) {
			continue
		}
		p := s.objs[o].owner()
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
	var codes [kop.MaxTargets + maxXTgt]uint8
	return (distinctTargets(&codes, s.chainTargets(e, &codes)) == 1) != DebugTrigParams4Flip
}

// singleCauseOK reports whether the resolving spell caused the damage to
// recipient and carries exactly one target, the recipient among it.
//
//go:noinline
func (s *State) singleCauseOK(k *prep, recipient uint8) bool {
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
