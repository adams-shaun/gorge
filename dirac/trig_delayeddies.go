package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Delayed "when that creature dies this turn" triggers
// (CR 603.7, 603.7c, 603.10a): gorge's DelayedTrigger Mode$ ChangesZone,
// ValidCard$ Card.IsTriggerRemembered, Origin$ Battlefield, Destination$
// Graveyard, ThisTurn$ True with any body (Grim Javelineer, Turn Inside Out,
// Scarblade's Malice, Felonious Rage, Fatal Fissure). The registration is an
// ordinary dprog record flagged dpDies over the remembered object; it fires
// when that object is put into a graveyard from the battlefield, after the
// move's own dies/leaves triggers (gorge checkTriggers runs the face walks,
// then checkEventDelayedTriggers), once -- gorge consumes it at its
// DelayedPush -- and is dropped at the next turn change (gorge
// collectExpiredDelayedTriggers). gorge's IsTriggerRemembered matches the
// object by id, so the record matches the Obj, not its incarnation.

// dpDies marks a dprog record a delayed dies registration
// (kop.DelayDiesThisTurn); its timing bits stay 0, so it is never due at a
// step (fireDelayedWhen).
const dpDies uint8 = 1 << 3

// DebugDelayDiesKeep is TestDelaydiesMutation's switch: a registration
// survives the turn change. Never set in production.
var DebugDelayDiesKeep bool

// tableDelaysDies reports whether some program of t registers a delayed dies
// program (genPrep.anyDelayDies).
func tableDelaysDies(t *Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpDelayedProgram && in.A.N == kop.DelayDiesThisTurn {
					return true
				}
			}
		}
	}
	return false
}

// fireDiesProgs enqueues, in registration order, every delayed dies
// registration remembering o, which is being put into a graveyard from the
// battlefield, with o as the trigger object and its last controller pl as
// the trigger player (gorge's TriggeredCardController reads the move's LKI:
// Searing Blood, Virulent Wound), and removes it; the rest stay in order
// with their LKI indexes (as fireDelayedWhen). The enqueued entry carries
// o's snapshot taken now, while it is still on the battlefield.
//
//go:noinline
func (s *State) fireDiesProgs(o Obj, pl uint8) {
	n := int(s.g.ndp)
	keep := 0
	for i := 0; i < n; i++ {
		d := s.g.dp[i]
		if d.n&dpDies != 0 && d.objs[0] == o {
			m := s.npend
			s.enqueue(d.src, d.sv, d.card, d.ab, d.ctrl, o, pl, 0)
			if s.npend > m {
				// the dying creature's last known information, taken as it
				// leaves, not at the registration (CR 603.10a, 608.2h: gorge
				// binds TriggeredCard to the move's LKI)
				li := s.snapLKI(o)
				if li == 0 {
					li = s.g.dpLKI[i]
				}
				s.pend[m].li = li
			}
			continue
		}
		s.g.dp[keep], s.g.dpLKI[keep] = d, s.g.dpLKI[i]
		keep++
	}
	for i := keep; i < n; i++ {
		s.g.dp[i], s.g.dpLKI[i] = dprog{}, 0
	}
	s.g.ndp = uint8(keep)
}

// dropDelayedDies drops every delayed dies registration at the turn change
// (ThisTurn$ True), keeping the other records in order.
//
//go:noinline
func (s *State) dropDelayedDies() {
	if DebugDelayDiesKeep {
		return
	}
	n := int(s.g.ndp)
	keep := 0
	for i := 0; i < n; i++ {
		if s.g.dp[i].n&dpDies == 0 {
			s.g.dp[keep], s.g.dpLKI[keep] = s.g.dp[i], s.g.dpLKI[i]
			keep++
		}
	}
	for i := keep; i < n; i++ {
		s.g.dp[i], s.g.dpLKI[i] = dprog{}, 0
	}
	s.g.ndp = uint8(keep)
}
