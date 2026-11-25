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
	BlockedFires    int // become-blocked emblem triggers queued (fireEmblemsBlocked)
	BlockedMulti    int // of those, for an attacker blocked by more than one creature
	BlockedLapsed   int // become-blocked emblem hook runs (a block with attackers) with the emblem's capture lapsed (the object left the battlefield)
	DmgCapSkips     int // damage events a remembering emblem's trigger passed over because the source was not its remembered object, the recipient was not its remembered player, or the damaged permanent was not its remembered object (the recipient capture)
	BlocksFires     int // blocks emblem triggers queued (fireEmblemsBlocks)
	UnblockedFires  int // unblocked-attacker emblem triggers queued (fireEmblemsUnblocked)
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
// + 1) fires only for that object on its captured side: as the damage SOURCE
// (the ValidSource$ ...IsRemembered shape; Creature.IsRemembered over the
// registration's Remembered set, gorge matches the object, not a version), or,
// on the recipient-captured shape (kgen's emblemCaptureFilter leaves the
// object filter on the line's F2; embRecipientCapture), as the damaged
// RECIPIENT (emblemTriggerObj).
//
//go:noinline
func (s *State) fireEmblemsDmg(k *prep, src Slot, amt int16, toPlayer, combat bool, rcp TCode, listeners bool, latch *dmgLatch, rl *refLatch) {
	s.fireEmblemsDmgFrom(k, src, s.perms[src].obj, s.perms[src].ctrl, amt, toPlayer, combat, rcp, listeners, latch, rl)
}

// embDmg reports whether some emblem trigger listens to damage dealt, so a
// damage site that gates its fire on the battlefield listeners must fire too.
func (k *prep) embDmg() bool {
	return k.gen != nil && k.gen.embCombatEv&(1<<kop.TrigDamageDone) != 0
}

// fireEmblemsDmgSpell is fireEmblemsDmg for noncombat damage whose source is
// no permanent (a spell such as Cave-In, or an ability that left the
// battlefield: fireDamageSrc): the emblem trigger fires for the damaged
// permanent as for any other source (CR 603.2). A source-restricted line
// matches the source's card, never a battlefield object.
//
//go:noinline
func (s *State) fireEmblemsDmgSpell(k *prep, o Obj, ctrl uint8, amt int16, toPlayer bool, rcp TCode, rl *refLatch) {
	s.fireEmblemsDmgFrom(k, NoSlot, o, ctrl, amt, toPlayer, false, rcp, false, nil, rl)
}

// fireEmblemsDmgFrom is the shared walk: src is NoSlot for a source that is no
// permanent, srcObj and srcCtrl then name the spell or its last known controller.
func (s *State) fireEmblemsDmgFrom(k *prep, src Slot, srcObj Obj, srcCtrl uint8, amt int16, toPlayer, combat bool, rcp TCode, listeners bool, latch *dmgLatch, rl *refLatch) {
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
			// A player-captured emblem scopes "that player": the recipient must
			// be the captured seat + 1. An object capture names the damage
			// source, or, on the recipient-captured shape (ValidTarget$
			// ...IsRemembered; F2 is then the object filter), the damaged
			// permanent. A player capture never fires for damage to a
			// permanent, even when dpl happens to name a seat.
			if e.CapturePlayer != kop.RefNone {
				if !toPlayer || int(s.embNum(pp, i)) != int(dpl)+1 {
					if st := DebugEffEOTTrigStats; st != nil {
						st.DmgCapSkips++
					}
					continue
				}
			} else if e.Capture != kop.RefNone && !s.emblemObjectCaptureOK(k, e, pp, i, srcObj, rcp) {
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
				if !s.emblemSourceOK(k, t, src, srcObj, srcCtrl, who) {
					continue
				}
				c := s.embCard(pp, i)
				cd := &k.t.Cards[c]
				for ab := range cd.Abilities {
					la := &cd.Abilities[ab]
					if la.Kind == kop.AbTriggered && la.AFlags&kop.AfLatent != 0 && la.Trig.Event == t.Event && la.Trig.Zone == kop.ZoneCommand {
						o := s.embObj(pp, i)
						n := amt
						if t.Event == kop.TrigDamageDone && t.Flags2&kop.Tf2PerReferent != 0 {
							// Mode$ DamageDoneOnce on an EOT Effect: one trigger
							// per damaged referent per damage batch (CR 603.2c,
							// the per-referent line lower_damagedoneonce.go
							// lowers). The refLatch is the step's shared latch,
							// so two sources hitting one player in a combat
							// damage step queue the body once. The aggregate
							// fire is not a damage event: the per-recipient
							// pass queues the referent lines.
							if rl != nil && rl.agg {
								break
							}
							l := trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(ab)}
							if !s.emblemDdoLatch(l, rcp, rl) {
								break
							}
						}
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
						s.enqueueLKI(o, s.objs[o].ver+1, c, uint8(ab), pp, s.emblemTriggerObj(k, e, srcObj, rcp), s.dmgSeat(rcp, srcCtrl), n)
						break
					}
				}
			}
		}
	}
}

// emblemDdoLatch is the once-per-batch gate of an EOT Effect's
// Mode$ DamageDoneOnce emblem line (Tf2PerReferent): the line queues at the
// first matching event of the open damage batch for the damaged referent
// (rcp) and every later matching event of the batch is the same triggering
// condition (CR 603.2c, 510.2). The refLatch is the step's shared latch, so
// the key spans sources: two Gorgons hitting one player in a combat damage
// step queue one instance. A nil latch (a noncombat event outside a batch) is
// its own batch of one, and an entry of the event in progress does not latch
// (the copies of one line trigger together, as ddoLine's referent latch).
func (s *State) emblemDdoLatch(l trigListener, rcp TCode, rl *refLatch) bool {
	if rl == nil || rl.r == nil && !rl.only {
		return true
	}
	for i := 0; i < rl.n; i++ {
		if e := &rl.e[i]; e.obj == l.obj && e.card == l.card && e.ab == l.ab &&
			e.rcp == rcp && e.at != rl.at {
			return false
		}
	}
	if rl.n == len(rl.e) {
		s.capFull(StallRefLatch)
		return false
	}
	rl.e[rl.n] = refEnt{obj: l.obj, card: l.card, ab: l.ab, rcp: rcp, at: rl.at}
	rl.n++
	return true
}

// emblemSourceOK is the damage-source half of an emblem's DamageDone line: a
// permanent source matches the line's filter as a battlefield object; a
// source that is no permanent (src NoSlot: a spell, or an ability that left
// the battlefield) has no self line and no ValidSource$ ...inZoneBattlefield,
// and matches its filter by its card.
func (s *State) emblemSourceOK(k *prep, t *kop.Trigger, src Slot, srcObj Obj, srcCtrl, who uint8) bool {
	if src != NoSlot {
		return t.Filter == 0 || s.matchPermGen(k, t.Filter, src, who, NoSlot, layerFull)
	}
	if t.Flags&kop.TfSelf != 0 || t.Flags2&kop.Tf2SrcInPlay != 0 {
		return false
	}
	return t.Filter == 0 || !needsAttachedSource(k, t.Filter) && s.matchCardGen(k, t.Filter, s.objs[srcObj].card, srcCtrl, who, srcObj)
}

// emblemObjectCaptureOK reports whether an object-captured emblem fires for
// this damage event: the capture names the damage source, or, on the
// recipient-captured shape (embRecipientCapture), the damaged object.
func (s *State) emblemObjectCaptureOK(k *prep, e *kop.Emblem, pp uint8, i int, srcObj Obj, rcp TCode) bool {
	num := int(s.embNum(pp, i))
	if s.embRecipientCapture(k, e) {
		return isObjTarget(rcp) && num == int(Obj(rcp))+1
	}
	return num == int(srcObj)+1
}

// emblemTriggerObj is the object a fired emblem trigger reads as its trigger
// object: the damage source, or the damaged object on a recipient capture
// (the body's DamageSource$ Remembered).
func (s *State) emblemTriggerObj(k *prep, e *kop.Emblem, srcObj Obj, rcp TCode) Obj {
	if e.Capture != kop.RefNone && s.embRecipientCapture(k, e) && isObjTarget(rcp) {
		return Obj(rcp)
	}
	return srcObj
}

// embRecipientCapture reports whether the emblem's capture names the damage
// RECIPIENT: one of its DamageDone trigger lines carries an object F2 (the
// stripped ValidTarget$ ...IsRemembered; kgen's emblemCaptureFilter) and no
// TfToPlayer. A source-captured emblem's F2 is a player filter or absent, or
// the planeswalker clause beside TfToPlayer (ValidTarget$ Player,Planeswalker,
// Hunter's Insight; kgen build_ab.go): a recipient capture names a card, so
// it never reads players.
func (s *State) embRecipientCapture(k *prep, e *kop.Emblem) bool {
	for j := range e.Abilities {
		a := &e.Abilities[j]
		if a.Kind == kop.AbTriggered && (a.Trig.Event == kop.TrigDamageDone || a.Trig.Event == kop.TrigDamageDealtOnce) &&
			a.Trig.Filter2 != 0 && a.Trig.Flags&kop.TfToPlayer == 0 && k.t.Filters[a.Trig.Filter2].Flags&kop.FPlayer == 0 {
			return true
		}
	}
	return false
}

// emblemRecipientOK is the recipient half of an emblem's DamageDone line
// that carries a Filter2: with TfToPlayer a player always passes (Tf2ToYou
// and Tf2ToOpponent are tested by the caller), and a damaged permanent must
// be a battlefield object matching Filter2 for the emblem owner; without
// TfToPlayer only such a permanent passes.
func (s *State) emblemRecipientOK(k *prep, t *kop.Trigger, rcp TCode, toPlayer bool, owner uint8) bool {
	if toPlayer {
		return t.Flags&kop.TfToPlayer != 0
	}
	if !isObjTarget(rcp) {
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
//
// An Attacks line of an emblem that remembers a target (kop.Emblem.Capture;
// State.g.embN holds the object + 1: Maeve, Insidious Singer's "whenever that
// creature attacks") is handed only the captured attacker of the event's
// group, gorge's ValidCard$ Creature.IsRemembered over the registration's
// Remembered set (kgen's emblemCaptureFilter strips the flag); a group
// without it, or a lapsed capture, fires nothing.
func (s *State) emblemAtkListeners(k *prep, grp []Slot, f func(l trigListener, a *kop.Ability, grp []Slot)) {
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
				g := grp
				if e.Capture != kop.RefNone && ea.Trig.Event == kop.TrigAttacks {
					if g = s.embCapturedAttacker(pp, i, grp); g == nil {
						continue
					}
				}
				c := s.embCard(pp, i)
				cd := &k.t.Cards[c]
				for ab := range cd.Abilities {
					la := &cd.Abilities[ab]
					if la.Kind == kop.AbTriggered && la.AFlags&kop.AfLatent != 0 && la.Trig.Event == ea.Trig.Event && la.Trig.Zone == kop.ZoneCommand {
						n0 := s.npend
						f(trigListener{obj: s.embObj(pp, i), sl: NoSlot, ctrl: pp, card: c, ab: uint8(ab)}, la, g)
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

// embCapturedAttacker is the one-attacker slice of grp holding player p's
// emblem i's captured object, nil when the group does not hold it (or the
// capture lapsed: embNum 0 names no object).
func (s *State) embCapturedAttacker(p uint8, i int, grp []Slot) []Slot {
	n := s.embNum(p, i)
	for x, as := range grp {
		if n != 0 && n == objWord(s.perms[as].obj)+1 {
			return grp[x : x+1]
		}
	}
	return nil
}

// DebugIgnoreAtkEmblem makes the per-attacker and per-defender emblem hooks
// queue nothing (the oracle's mutation test: the lockstep must diverge).
var DebugIgnoreAtkEmblem bool

// DebugIgnoreBlockedEmblem makes the become-blocked emblem hook queue nothing
// (the oracle's mutation test: the lockstep must diverge).
var DebugIgnoreBlockedEmblem bool

// fireEmblemsBlocked fires the end-of-turn emblems' become-blocked triggers
// (gorge checkEffectCombatTriggers, Mode$ AttackerBlocked; Barreling Attack)
// after fireBlockedGen's listener walk: for each emblem, once per DISTINCT
// blocked attacker in atk (the order of their first block), as the creating
// card's latent command-zone copy with the attacker as its trigger object. An
// emblem that remembers a target (kop.Emblem.Capture; State.g.embN holds the
// object + 1) fires only for that object (Card.IsRemembered over the
// registration's Remembered set; a lapsed capture is 0 and fires for none).
//
//go:noinline
func (s *State) fireEmblemsBlocked(k *prep, atk []Slot) {
	if DebugIgnoreBlockedEmblem {
		return
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			if !e.UntilEOT {
				continue
			}
			for j := range e.Abilities {
				ea := &e.Abilities[j]
				if ea.Kind != kop.AbTriggered || ea.Trig.Event != kop.TrigBecomesBlocked || ea.Trig.Flags2&kop.Tf2Unblocked != 0 {
					continue
				}
				c := s.embCard(pp, i)
				cd := &k.t.Cards[c]
				for ab := range cd.Abilities {
					la := &cd.Abilities[ab]
					if la.Kind != kop.AbTriggered || la.AFlags&kop.AfLatent == 0 || la.Trig.Event != kop.TrigBecomesBlocked || la.Trig.Flags2&kop.Tf2Unblocked != 0 || la.Trig.Zone != kop.ZoneCommand {
						continue
					}
					o := s.embObj(pp, i)
					if st := DebugEffEOTTrigStats; st != nil && e.Capture != kop.RefNone && s.embNum(pp, i) == 0 {
						st.BlockedLapsed++
					}
					for _, as := range atk {
						ao := s.perms[as].obj
						if e.Capture != kop.RefNone && int(s.embNum(pp, i)) != int(ao)+1 {
							continue
						}
						if ea.Trig.Filter != 0 && !s.matchPermGen(k, ea.Trig.Filter, as, pp, NoSlot, layerFull) {
							continue
						}
						if st := DebugEffEOTTrigStats; st != nil {
							st.BlockedFires++
							if s.perms[as].nblock > 1 {
								st.BlockedMulti++
							}
						}
						s.enqueueLKI(o, s.objs[o].ver+1, c, uint8(ab), pp, ao, s.perms[as].ctrl, 0)
					}
					break
				}
			}
		}
	}
}
