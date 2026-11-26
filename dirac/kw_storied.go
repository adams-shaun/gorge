package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// K:Storied and the enduring story (CR 702.195), copied from gorge
// (engine/rules/storied.go's checkEnduringStoryGrants; the reader
// layers_static.go's condEnduringStory, the arm every EnduringStory
// carrier's gate ends in: continuousConditionHolds for the Continuous and
// Panharmonicon statics, combatBoard.StaticGateHolds for the combat
// restrictions, attack_cost.go's continuousGateHolds for the attack tax,
// activate_static_altcost.go's abilityAlternateCostFor for the attach-cost
// alternative). Cold: a table without a printed Storied card
// (CardDef.Storied) never takes these paths (prep.storied).
//
// State: the latch is one bit per seat that never clears, genState.story
// (its byte sits in the padding after ctrSettle, so State does not grow).
//
//   - 702.195a, the grant: the same three hooks gorge's post-fold emit scan
//     runs from -- a battlefield entry (zones.go enter / storyGrant: spells,
//     tokens, copies, returns), a control change (static_control.go ctrlMove) and
//     the state-based pass (turn.go) -- run storiedScan: each seat without
//     the latch, still in the game, that controls three or more qualifying
//     permanents (artifact, legendary and/or Saga, each permanent counted
//     once, a lost-abilities one too) among which one carries Storied and
//     keeps its abilities gets the
//     latch. A phased-out permanent is skipped (gorge's o.PhasedOut skip;
//     the controller mask is the phased-in board). No spell form exists:
//     Storied prints on permanents only.

// DebugStoriedNone is the mutation switch (TestStaticZonesMutation): the
// enduring story is never granted. It must make the lockstep diverge. Read
// in storiedScan only.
var DebugStoriedNone bool

// storyGrant is the emit-tail grant: gorge's checkEnduringStoryGrants runs on
// the post-fold tail of every battlefield-entry emit (engine/rules/emit.go),
// AFTER that event's triggers were queued (checkTriggers) -- Bifur, Melodic
// Rider's gated Panharmonicon static reads Condition$ EnduringStory while its
// OWN entry's ETB trigger is queued, and must see the pre-grant state (the
// staticzones seed 3 divergence: the kernel echoed the entering Bifur's own
// ETB trigger with the latch that same entry was granting, gorge did not). So
// an entry path calls this only once its ETB triggers are queued: enter after
// its own fire, fireEntries after each held entrant's fire, beside every
// direct fire site (bestow, facedown, the token copies, the entry clone).
//
//go:noinline
func (s *State) storyGrant(k *prep) {
	if k.storied {
		s.storiedScan(k)
	}
}

// storiedNeed is CR 702.195's "three or more" permanents.
const storiedNeed = 3

// storyBit is seat p's bit of genState.story; an out-of-range seat has none.
func storyBit(p uint8) uint8 {
	if p >= MaxSeats {
		return 0
	}
	return 1 << p
}

// hasStory reports whether seat p holds the enduring-story latch. The ONE
// read of genState.story: condEval's CondEnduringStory arm (exprs.go) goes
// through it, the same single read gorge's readers make through
// playerHasEnduringStory.
func (s *State) hasStory(p uint8) bool {
	return s.g.story&storyBit(p) != 0
}

// tableHasStoried reports whether some card of t prints Storied.
func tableHasStoried(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].Storied {
			return true
		}
	}
	return false
}

// storiedScan is gorge checkEnduringStoryGrants: each seat without the
// latch, still in the game, that controls three or more qualifying
// permanents, one of them carrying Storied, gets the latch. The qualifying
// types are read the way gorge counts them (derived types for artifact,
// hasType for legendary): typesAt, the permanent's override-read type mask
// (chars.go). The Saga read is APPROXIMATE: gorge reads the DERIVED type
// (slices.Contains(d.Types, "Saga")), while the kernel reads the printed
// cd.SagaN -- kop.TypeMask carries no Saga bit, so a type-change layer that
// makes a permanent a Saga or un-makes one is unread here. No pool carrier
// prints such a layer beside Storied; gorge's storiedPossible pre-filter
// (faceScanStoried) keeps the printed faces of every table that reaches this
// scan anyway.
//
//go:noinline
func (s *State) storiedScan(k *prep) {
	if DebugStoriedNone {
		return
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.hasStory(p) || s.pl[SeatIx(p)].lost {
			continue
		}
		storied := false
		count := 0
		for m := s.ctrlM[p]; m.any(); m = m.rest() {
			sl := m.first()
			cd := &k.t.Cards[s.cardAt(sl)]
			// losing abilities removes the Storied keyword, not the types the
			// permanent still counts as (CR 613.1d vs 613.1f): gorge's
			// hasKeywordH is the only read that skips it.
			if cd.Storied && !s.genLost(k, sl) {
				storied = true
			}
			if s.typesAt(k, sl)&(kop.TArtifact|kop.TLegendary) != 0 || cd.SagaN != 0 {
				count++
			}
		}
		if storied && count >= storiedNeed {
			s.g.story |= storyBit(p)
		}
	}
}
