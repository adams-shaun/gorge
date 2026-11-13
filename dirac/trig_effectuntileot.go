package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// an until-end-of-turn Effect's Triggers$ on combat
// events (CR 603.7, CR 114: the kernel's emblem stands in for the Effect
// object). gorge effEffect (engine/effects/effect.go) registers the body as
// a recurring delayed trigger dropped at the turn change (dropEOTEmblems);
// trigger_match.go checks the delayed triggers of a Damage or
// DeclareAttackers event after its face triggers, with "You" the effect's
// controller (trigger_delayed.go effectMatchController) and the printed
// matchers (delayedEventMatches). Mistway Spy, Dalkovan Encampment.

// DebugEffEOTTrigOppOwner is a mutation switch (TestEffeottrigMutation):
// the emblem's source filter is matched for the emblem owner's opponent
// instead of the owner (gorge effectMatchController: "You" is the effect's
// controller). Matching for the source's controller would be inert: an EOT
// emblem lives only on its owner's turn, when every creature dealing combat
// damage to a player is the owner's (CR 506.2, 510.1).
var DebugEffEOTTrigOppOwner bool

// EffEOTTrigStats counts the combat emblem firings (the lockstep's reach).
type EffEOTTrigStats struct {
	DmgFires        int // DamageDone emblem triggers queued
	AtkFires        int // AttackersDeclared emblem triggers queued
	DmgWithListener int // DamageDone emblem triggers queued by an event with a battlefield DamageDone listener (the hook after the listener walk)
	DmgBeside       int // DamageDone emblem triggers queued beside another pending trigger of the owner's (the trigger_order ask)
	DmgMulti        int // DamageDone emblem triggers queued with more than 1 damage dealt
	DmgCapSkips     int // damage events a remembering emblem's trigger passed over because the source was not its remembered object, or the recipient was not its remembered player
}

// DebugInnerLightNoAmount is a mutation switch (TestTheInnerLightMutation):
// the DamageDone emblem trigger is queued with amount 0 again, so a body
// reading the damage amount (The Inner Light's draw) draws nothing.
var DebugInnerLightNoAmount bool

// DebugEffEOTTrigStats is the test hook; nil in normal play.
var DebugEffEOTTrigStats *EffEOTTrigStats

// fireEmblemsDmg fires the emblems' DamageDone triggers for one damage
// event from the permanent at src (to a player when toPlayer, combat
// damage when combat; rcp the recipient code), after the battlefield
// listeners: each emblem trigger whose flags and source filter match for
// the emblem's owner, queued as the creating card's latent command-zone
// copy with the source as its trigger object and amt the damage dealt
// (TriggerCount$DamageAmount, gorge's delayed trigger context). An emblem
// that remembers a target (kop.Emblem.Capture; State.g.embN holds the object
// + 1) fires only for that object as the source (Creature.IsRemembered over
// the registration's Remembered set; gorge matches the object, not a version).
//
//go:noinline
func (s *State) fireEmblemsDmg(k *prep, src Slot, amt int16, toPlayer, combat bool, rcp uint8, listeners bool, latch *dmgLatch) {
	srcObj := s.perms[src].obj
	srcCtrl := s.perms[src].ctrl
	dpl := s.dmgSeat(rcp, srcCtrl)
	if pl, ok := recipientPlayer(rcp); ok {
		dpl = pl
	}
	seen := 0
	if latch != nil {
		seen = latch.n
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			if !e.UntilEOT {
				continue // a permanent emblem's combat damage trigger is not this code's (unverified against gorge)
			}
			// A player-captured emblem scopes "that player": the recipient must
			// be the captured seat + 1. An object-captured emblem keeps the
			// source check. A player capture never fires for damage to a
			// permanent, even when dpl happens to name a seat.
			if e.CapturePlayer != kop.RefNone {
				if !toPlayer || int(s.embNum(pp, i)) != int(dpl)+1 {
					if st := DebugEffEOTTrigStats; st != nil {
						st.DmgCapSkips++
					}
					continue
				}
			} else if e.Capture != kop.RefNone && int(s.embNum(pp, i)) != int(srcObj)+1 {
				if st := DebugEffEOTTrigStats; st != nil {
					st.DmgCapSkips++
				}
				continue
			}
			for j := range e.Abilities {
				ea := &e.Abilities[j]
				if ea.Kind != kop.AbTriggered || ea.Trig.Event != kop.TrigDamageDone && ea.Trig.Event != kop.TrigDamageDealtOnce {
					continue
				}
				t := &ea.Trig
				if t.Flags&kop.TfCombat != 0 && !combat || t.Flags2&kop.Tf2NonCombat != 0 && combat {
					continue
				}
				if t.Flags&kop.TfToPlayer != 0 && !toPlayer && t.Filter2 == 0 {
					continue
				}
				if t.Filter2 != 0 && !s.emblemRecipientOK(k, t, rcp, toPlayer, pp) {
					continue // "a player or a planeswalker": Filter2 beside TfToPlayer
				}
				if t.Flags2&kop.Tf2ToOpponent != 0 && dpl == pp {
					continue
				}
				if t.Flags2&kop.Tf2ToYou != 0 && dpl != pp {
					continue // ValidTarget$ You: the emblem owner is the damaged player
				}
				if t.Flags2&kop.Tf2YourTurn != 0 && s.active != pp {
					continue
				}
				who := pp
				if DebugEffEOTTrigOppOwner {
					who = 1 - pp // seat-ok: a test-only mutation (DebugEffEOTTrigOppOwner)
				}
				if t.Filter != 0 && !s.matchPermGen(k, t.Filter, src, who, NoSlot, layerFull) {
					continue
				}
				c := s.embCard(pp, i)
				cd := &k.t.Cards[c]
				for ab := range cd.Abilities {
					la := &cd.Abilities[ab]
					if la.Kind == kop.AbTriggered && la.AFlags&kop.AfLatent != 0 && la.Trig.Event == t.Event && la.Trig.Zone == kop.ZoneCommand {
						o := s.embObj(pp, i)
						n := amt
						if t.Event == kop.TrigDamageDealtOnce && latch != nil {
							// one trigger per damage batch, carrying the source's
							// batch total (fireDamageEvGen's DamageDealtOnce latch)
							l := trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(ab)}
							if latch.has(l, seen) {
								break
							}
							if n = latch.total; t.Flags&kop.TfToPlayer != 0 {
								n = latch.totalPl
							}
							if latch.n < len(latch.l) {
								latch.l[latch.n] = l
								latch.n++
							} else {
								s.capFull(StallDmgLatch)
							}
						}
						if st := DebugEffEOTTrigStats; st != nil {
							st.DmgFires++
							if amt > 1 {
								st.DmgMulti++
							}
							if listeners {
								st.DmgWithListener++
							}
							for x := 0; x < int(s.npend); x++ {
								if s.pend[x].ctrl == pp {
									st.DmgBeside++
									break
								}
							}
						}
						if DebugInnerLightNoAmount {
							n = 0
						}
						s.enqueueLKI(o, s.objs[o].ver+1, c, uint8(ab), pp, srcObj, s.dmgSeat(rcp, srcCtrl), n)
						break
					}
				}
			}
		}
	}
}

// emblemRecipientOK is the recipient half of an emblem's DamageDone line
// that carries a Filter2: with TfToPlayer a player always passes (Tf2ToYou
// and Tf2ToOpponent are tested by the caller), and a damaged permanent must
// be a battlefield object matching Filter2 for the emblem owner; without
// TfToPlayer only such a permanent passes.
func (s *State) emblemRecipientOK(k *prep, t *kop.Trigger, rcp uint8, toPlayer bool, owner uint8) bool {
	if toPlayer {
		return t.Flags&kop.TfToPlayer != 0
	}
	if !isObjTarget(TCode(rcp)) {
		return false
	}
	ob := &s.objs[rcp]
	return ob.zone == kop.ZoneBattlefield && s.matchPermGen(k, t.Filter2, ob.slot, owner, NoSlot, layerFull)
}

// fireEmblemsAttack fires the emblems' AttackersDeclared triggers for the
// active player's declaration (after the battlefield listeners, before the
// attackers' tap triggers), gated on the attacker count as playerTrigGen
// gates a printed line.
//
//go:noinline
func (s *State) fireEmblemsAttack(k *prep) {
	if s.nAttackers == 0 {
		return
	}
	n, before := int(s.nAttackers), s.npend
	s.fireEmblems(k, kop.TrigAttackersDeclared, s.active, NoObj, func(a *kop.Ability) bool {
		if a.Trig.Flags2&kop.Tf2DefPlayer != 0 {
			return false // a per-defender line is walked with the declaration's groups (fireEmblemGroup)
		}
		return a.Trig.N == 0 || trigCmpOK(n, a.Trig.N, true)
	})
	if st := DebugEffEOTTrigStats; st != nil {
		st.AtkFires += int(s.npend) - int(before)
	}
}

// emblemAtkListeners calls f for each emblem trigger on the declaration's
// combat events, as a listener of the emblem's owner (no source permanent):
// the creating card's latent command-zone copy, queued with the emblem's own
// object version after the card left (as fireEmblems does). It reaches the
// per-attacker Attacks lines and the per-defender AttackersDeclared lines
// (AttackedTarget$); a line with no defender is fireEmblemsAttack's.
func (s *State) emblemAtkListeners(k *prep, f func(l trigListener, a *kop.Ability)) {
	if DebugIgnoreAtkEmblem {
		return
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			for j := range e.Abilities {
				ea := &e.Abilities[j]
				if ea.Kind != kop.AbTriggered || ea.Trig.Event != kop.TrigAttacks && !(ea.Trig.Event == kop.TrigAttackersDeclared && ea.Trig.Flags2&kop.Tf2DefPlayer != 0) {
					continue
				}
				c := s.embCard(pp, i)
				cd := &k.t.Cards[c]
				for ab := range cd.Abilities {
					la := &cd.Abilities[ab]
					if la.Kind == kop.AbTriggered && la.AFlags&kop.AfLatent != 0 && la.Trig.Event == ea.Trig.Event && la.Trig.Zone == kop.ZoneCommand {
						n0 := s.npend
						f(trigListener{obj: s.embObj(pp, i), sl: NoSlot, ctrl: pp, card: c, ab: uint8(ab)}, la)
						for x := n0; x < s.npend; x++ {
							s.pend[x].srcVer++ // the emblem's creator left its zone: the entry carries the version after it
						}
						break
					}
				}
			}
		}
	}
}

// DebugIgnoreAtkEmblem makes the per-attacker and per-defender emblem hooks
// queue nothing (the oracle's mutation test: the lockstep must diverge).
var DebugIgnoreAtkEmblem bool
