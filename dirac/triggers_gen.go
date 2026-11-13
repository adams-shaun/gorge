package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Triggers on a generated table: the listeners are the subscribed
// permanents' own triggered abilities, the abilities granted to them, the
// graveyard triggers of cards in graveyards, and emblems. Intervening-if
// conditions (Trigger.Cond) are checked when the event happens and again
// on resolution (unless AfCheckOnTrigger).

// trigListener is one triggered ability that may react: its source object
// (and slot, NoSlot off the battlefield), controller, card and index.
type trigListener struct {
	obj  Obj
	sl   Slot
	ctrl uint8
	card CardID
	ab   uint8
	host uint8 // the granting permanent plus one (stackEntry.trigHost); 0 = none
}

// eachTrigger calls f for every triggered ability of event ev on the
// battlefield (subscribed slots) and its granted abilities, then for a
// commander's printed command-zone triggers.
//
// A caller whose event also walks the graveyard uses eachBattlefieldTrigger,
// walks the graveyard, then calls eachCommandTrigger: gorge's object walk
// (rules forEachObject) is library, hand, battlefield, graveyard, exile,
// stack, command, so a graveyard listener queues before a command-zone one.
func (s *State) eachTrigger(k *prep, ev kop.TrigEvent, f func(l trigListener, a *kop.Ability)) {
	s.eachBattlefieldTrigger(k, ev, f)
	s.eachCommandTrigger(k, ev, f)
}

// eachBattlefieldTrigger calls f for every triggered ability of event ev on
// the battlefield (subscribed slots) and its granted abilities, in gorge's
// walk order.
func (s *State) eachBattlefieldTrigger(k *prep, ev kop.TrigEvent, f func(l trigListener, a *kop.Ability)) {
	// listeners in gorge's walk order: player 0's battlefield list, then
	// player 1's (rules forEachObject)
	var ord [MaxPerm]Slot
	m := s.subs[ev].and(s.occ)
	if k.gen.grantTrigEv&(1<<ev) != 0 {
		m = s.grantTrigHosts(k, m)
	}
	n := s.listenerOrder(m, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == ev }, f)
	}
}

// anyCommandTrigger reports whether a commander in a command zone has a
// printed trigger for ev. The attack walk's early return uses it: a command
// zone trigger has no battlefield subscription, so the subscription mask alone
// cannot decide whether the event has a listener.
func (s *State) anyCommandTrigger(k *prep, ev kop.TrigEvent) bool {
	if s.cfg&cfgCommander == 0 {
		return false
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		for j := 0; j < 2; j++ {
			o := s.cmdObj(p, j)
			if o == NoObj || s.objs[o].zone != kop.ZoneCommand {
				continue
			}
			cd := &k.t.Cards[s.objs[o].card]
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind == kop.AbTriggered && a.AFlags&kop.AfLatent == 0 &&
					a.Trig.Event == ev && a.Trig.Zone == kop.ZoneCommand {
					return true
				}
			}
		}
	}
	return false
}

// eachCommandTrigger calls f for each printed command-zone triggered ability
// of a commander in its command zone. A commander is not a permanent and has
// no subscribed battlefield slot (CR 903.3), so the ordinary battlefield walks
// never reach an eminence trigger; its Trigger.Zone marks what none of the
// other walks see. The walk is seat order, then commander/partner slot, after
// the battlefield listeners.
func (s *State) eachCommandTrigger(k *prep, ev kop.TrigEvent, f func(l trigListener, a *kop.Ability)) {
	if s.cfg&cfgCommander == 0 {
		return
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		for j := 0; j < 2; j++ {
			o := s.cmdObj(p, j)
			if o == NoObj || s.objs[o].zone != kop.ZoneCommand {
				continue
			}
			cid := s.objs[o].card
			cd := &k.t.Cards[cid]
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind == kop.AbTriggered && a.AFlags&kop.AfLatent == 0 &&
					a.Trig.Event == ev && a.Trig.Zone == kop.ZoneCommand {
					f(trigListener{obj: o, sl: NoSlot, ctrl: s.objs[o].owner(), card: cid, ab: uint8(i)}, a)
				}
			}
		}
	}
}

// listenerOrder writes the slots of m in gorge's trigger walk order
// (player 0's battlefield list in entry order, then player 1's) and returns
// their count.
func (s *State) listenerOrder(m slotMask, ord *[MaxPerm]Slot) int {
	n := s.battlefieldOrder(0, m.and(s.ctrlM[0]), ord[:], 0)
	if MaxSeats == 2 {
		return s.battlefieldOrder(1, m.and(s.ctrlM[1]), ord[:], n)
	}
	// every seat's listeners, seat by seat (a third or fourth seat's
	// permanents trigger too)
	for p := uint8(1); p < s.nSeats(); p++ {
		n = s.battlefieldOrder(p, m.and(s.ctrlM[p]), ord[:], n)
	}
	return n
}

// slotTriggers calls f for the triggered abilities at slot sl that ev
// accepts: the card's own battlefield triggers in ability order (unless it
// lost its abilities), then the abilities granted to it (gorge's face walk,
// then the granted walk).
func (s *State) slotTriggers(k *prep, sl Slot, ev func(a *kop.Ability) bool, f func(l trigListener, a *kop.Ability)) {
	p := &s.perms[sl]
	var own [4]uint8 // a Station threshold's triggers: its own static's grants
	nown := 0
	if !s.genLost(k, sl) { // genLost covers a face-down permanent (CR 708.2)
		cid := s.cardAt(sl)
		cd := &k.t.Cards[cid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbTriggered && ev(a) && a.AFlags&kop.AfLatent == 0 && (a.Trig.Zone == kop.ZoneNone || a.Trig.Zone == kop.ZoneBattlefield) && (!k.anyGate || s.gateOK(sl, a)) {
				if a.Gate.Ctr == kop.CtrCharge && k.gen.anyGrant && nown < len(own) {
					own[nown] = uint8(i)
					nown++
					continue
				}
				f(trigListener{obj: p.obj, sl: sl, ctrl: p.ctrl, card: cid, ab: uint8(i)}, a)
			}
		}
	}
	// a granted Melee is listed apart from the other grants (kw_melee.go)
	melee := k.gen.meleeGrant && ev(&meleeProbe)
	meleeFirst := melee && s.meleeGrantsFirst(k, sl)
	if meleeFirst {
		s.meleeGrantTriggers(k, sl, ev, f)
	}
	if nown != 0 {
		// gorge lists a Station threshold's trigger with the other statics'
		// granted triggers, at its permanent's timestamp: an older static's
		// grant (Supplementation Node's) comes first (CR 613.7)
		cid := s.cardAt(sl)
		s.grantedAbilitiesOwn(k, sl, func() {
			for _, i := range own[:nown] {
				f(trigListener{obj: p.obj, sl: sl, ctrl: p.ctrl, card: cid, ab: i}, &k.t.Cards[cid].Abilities[i])
			}
		}, func(c CardID, ab uint8, n int) {
			a := &k.t.Cards[c].Abilities[ab]
			if a.Kind == kop.AbTriggered && ev(a) && !(melee && a.MeleeGrant) {
				for ; n > 0; n-- {
					f(trigListener{obj: p.obj, sl: sl, ctrl: p.ctrl, card: c, ab: ab, host: s.g.grantSrc}, a)
				}
			}
		})
	} else {
		s.grantedAbilitiesN(k, sl, func(c CardID, ab uint8, n int) {
			a := &k.t.Cards[c].Abilities[ab]
			if a.Kind == kop.AbTriggered && ev(a) && !(melee && a.MeleeGrant) {
				for ; n > 0; n-- {
					f(trigListener{obj: p.obj, sl: sl, ctrl: p.ctrl, card: c, ab: ab, host: s.g.grantSrc}, a)
				}
			}
		})
	}
	if melee && !meleeFirst {
		s.meleeGrantTriggers(k, sl, ev, f)
	}
}

// trigOK checks a listener's once-per-turn limit and intervening-if
// condition when the event happens.
func (s *State) trigOK(k *prep, l trigListener, a *kop.Ability) bool {
	if k.gen.anyGameOnce && a.Trig.Flags3&kop.Tf3GameOnce != 0 && s.gameOnceUsed(l) {
		return false // GameActivationLimit$ 1 (trig_params.go)
	}
	if a.Trig.Flags&kop.TfOncePerTurn != 0 && s.usedThisTurn(l.obj, l.ab) {
		return false
	}
	if a.Trig.Cond != 0 && !s.condEval(k, a.Trig.Cond, evalCtx{src: l.obj, srcSl: l.sl, ctrl: l.ctrl}) {
		return false
	}
	if a.Trig.Flags2&kop.Tf2FirstTime != 0 && a.Trig.Event == kop.TrigCounterAddedOnce && s.counterFirstUsed(k, l) {
		return false // the turn's first matching counter batch already fired (trig_counterfirst.go)
	}
	return true
}

func (s *State) queueTrig(k *prep, l trigListener, a *kop.Ability, tobj Obj, tpl uint8, amt int16) {
	if a.Trig.Flags2&kop.Tf2ResolvedLimit != 0 && s.rlSpent(l.obj) {
		return // ResolvedLimit$: the source's limited lines resolved this turn (act_support.go)
	}
	if a.Trig.Flags&kop.TfOncePerTurn != 0 {
		s.markUsed(l.obj, l.ab)
	}
	if a.Trig.Flags2&kop.Tf2FirstTime != 0 && a.Trig.Event == kop.TrigCounterAddedOnce {
		s.markCounterFirst(k, l) // trig_counterfirst.go
	}
	if k.gen.anyGameOnce && a.Trig.Flags3&kop.Tf3GameOnce != 0 {
		s.markGameOnce(l)
	}
	if k.gift && a.AFlags&kop.AfGift != 0 {
		tpl = s.promisedAt(l.obj) // the promised opponent, snapshotted at queue (kw_gift.go)
	}
	n := s.npend
	s.enqueueLKI(l.obj, s.objs[l.obj].ver, l.card, l.ab, l.ctrl, tobj, tpl, amt)
	if s.npend > n {
		s.pend[n].remVer = l.host // a granted trigger's host rides remVer (stackEntry.trigHost)
	}
	if a.Trig.Flags2&(kop.Tf2XBound|kop.Tf2SpentBound|kop.Tf2Convoked) != 0 {
		s.bindTrigProv(a, tobj) // trig_xpaid.go, trig_gravedrawn.go
	}
	if a.Trig.Flags3 != 0 {
		s.bindTrigConverge(a, tobj) // mana_converge.go
	}
	if k.gen.anyPanharm && s.npend > n && a.Trig.Flags&kop.TfOncePerTurn == 0 && a.Trig.Flags3&kop.Tf3GameOnce == 0 {
		// a line limited to once per turn or per game has spent its one trigger
		// on the original: an additional time would pass the limit again (gorge
		// queueTriggerLimited, CR 603.2d)
		s.panharmEcho(k, l, a.Trig.Event, tobj, tpl, n) // trig_doubling.go
	}
	if k.gen.anyBatchPool && s.npend > n && a.Trig.Flags2&kop.Tf2Batch != 0 && readsBatchPool(a) {
		s.batchMemberNew(n, tobj) // trig_batchmembers.go
	}
}

// enqueueLKI is enqueue with the triggering object's last known information
// snapshot into the pending entry.
func (s *State) enqueueLKI(o Obj, ver uint8, c CardID, ab, ctrl uint8, tobj Obj, tpl uint8, amt int16) {
	n := s.npend
	s.enqueue(o, ver, c, ab, ctrl, tobj, tpl, amt)
	if s.npend > n {
		s.pend[n].li = s.snapLKI(tobj)
		s.snapshotTrigSpellMV(n, c, ab, tobj)
	}
}

// objEventMatch: the event object at evSl matches the listener's filter.
func (s *State) objEventMatch(k *prep, l trigListener, a *kop.Ability, evSl Slot) bool {
	t := &a.Trig
	if t.From != kop.ZoneNone && t.From != kop.ZoneBattlefield {
		return false // a graveyard leave, fired by fireLeavesGraveyard
	}
	self := l.sl == evSl
	if t.Flags&kop.TfSelf != 0 {
		return self && (t.Filter == 0 || s.matchPermGen(k, t.Filter, evSl, l.ctrl, l.sl, layerFull))
	}
	if t.Flags2&kop.Tf2SelfOrFilter != 0 && self {
		return true
	}
	if evSl == NoSlot {
		return t.Filter == 0
	}
	return t.Filter == 0 || s.matchPermGen(k, t.Filter, evSl, l.ctrl, l.sl, layerFull)
}

// fireObjGen is the core's fireObj on a generated table: battlefield listeners
// first, then, for an ETB, the graveyard cards whose Eerie-style line fires
// from there (Fear of Infinity; face_fullyunlock.go), then a commander's
// command-zone trigger (gorge walks the command zone after the graveyard).
func (s *State) fireObjGen(k *prep, ev kop.TrigEvent, evSl Slot, evObj Obj, evCtrl uint8, amt int16) {
	if ev == kop.TrigETB && k.gen.anyDisETB && s.etbDisabled(k, evSl) {
		return // DisableTriggers (trig_disable.go)
	}
	s.eachBattlefieldTrigger(k, ev, func(l trigListener, a *kop.Ability) {
		s.objTrigGen(k, ev, l, a, evSl, evObj, evCtrl, amt)
	})
	if ev == kop.TrigETB && k.gen.graveETB {
		s.fireGraveETB(k, evSl, evObj, evCtrl)
	}
	s.eachCommandTrigger(k, ev, func(l trigListener, a *kop.Ability) {
		s.objTrigGen(k, ev, l, a, evSl, evObj, evCtrl, amt)
	})
	if k.gen.embObjEv&(1<<ev) != 0 && evSl != NoSlot {
		s.fireEmblemsObj(k, ev, evSl, evObj)
	}
}

// objTrigGen queues listener l's ability a for an object event if it
// matches.
func (s *State) objTrigGen(k *prep, ev kop.TrigEvent, l trigListener, a *kop.Ability, evSl Slot, evObj Obj, evCtrl uint8, amt int16) {
	if !s.objEventMatch(k, l, a, evSl) {
		return
	}
	if ev == kop.TrigLeaves && k.gen.anyCzPair {
		if !czLeaveOK(a, amt) {
			return
		}
		amt = 0 // the destination zone, not an amount (trig_zonepairs.go)
	}
	if ev == kop.TrigCounterAddedOnce {
		if a.Trig.N != kop.TrigNAnyCounter && kop.CounterKind(a.Trig.N) != s.g.lastCK {
			return
		}
		if s.g.ctrCellHit && a.Trig.Flags4&kop.Tf4CounterCell != 0 {
			return // this put action already fired it for this recipient (trig_countercell.go)
		}
		// ValidSource$ You: the player putting the counters (the
		// resolving spell's or ability's controller; Fleeting Flight cast
		// by an opponent on Exemplar of Light triggers nothing). The
		// counters a permanent enters with are put by its controller
		// (CR 122.6a; gorge foldEntryWithPlaced): Parting Gust's return
		// under the owner's control is the owner's put, whoever resolves it.
		putter := evCtrl
		if s.res.active && !s.g.ctrEntry {
			putter = s.res.e.ctrl
		}
		if !whoseOK(a.Trig.Whose, l.ctrl, putter) {
			return
		}
		if a.Trig.Flags4&kop.Tf4StateCtr != 0 {
			// a state trigger's threshold is its Cond: the put re-checks it
			// only while no instance is outstanding (CR 603.8, trig_statectr.go)
			if s.stateTrigOutstanding(l.obj, l.ab) {
				return
			}
		} else if a.Gate.Ctr != 0 && !s.crossingOK(l.sl, a, int(amt)) {
			return // a Saga chapter / "Nth counter" line triggers only when its number is reached
		}
	}
	tapAll := false
	if ev == kop.TrigTaps {
		if k.gen.tapsWhose && !s.tapsWhoseOK(l, a, amt) {
			return
		}
		if k.gen.anyTWTap && !twTapOK(a, amt) {
			return // Teamwork$ True: only a teamwork-cost tap (trig_params.go)
		}
		if a.Trig.Flags2&kop.Tf2FirstTime != 0 && s.mSl(k, mechTappedTurn, evSl) {
			return // an earlier tap this turn: FirstTime$ is false (gorge Facts.TappedTurn)
		}
		amt = 0                                  // the tapper (fireTapsBy), not an amount
		tapAll = a.Trig.Flags2&kop.Tf2Batch != 0 // Mode$ TapAll: the tap batch's latch, not the zone batch's (trig_tapall.go)
	}
	if a.Trig.Flags2&(kop.Tf2YourTurn|kop.Tf2OppTurn|kop.Tf2Evolve) != 0 && !s.objTrigGate(k, l, a, evSl) {
		return
	}
	if !s.trigOK(k, l, a) {
		return
	}
	if tapAll {
		if !s.tapBatchOK(l, a) {
			return
		}
	} else if a.Trig.Flags2&(kop.Tf2Batch|kop.Tf2CastFromHand) != 0 && !s.batchOrProvOK(l, a, evObj) {
		return
	}
	if ev == kop.TrigCounterAddedOnce && a.Trig.Flags4&kop.Tf4PerCounter != 0 {
		// Mode$ CounterAdded: one trigger per counter of the batch, each
		// carrying amount 1 (CR 603.2c; gorge queues one instance per counter)
		for i := int16(0); i < amt; i++ {
			s.queueTrig(k, l, a, evObj, evCtrl, 1)
		}
		return
	}
	s.queueTrig(k, l, a, evObj, evCtrl, amt)
}

// lifeCauseSpellOK is a LifeLost trigger's Tf4CauseSpellAb gate: the loss
// was caused by the spell or ability resolving right now (gorge
// lifeLossCauseMatches reads Facts.Damaging, set around every stack
// resolution and cleared after it; combat damage and a loss with no
// resolution in flight -- a cost payment, a mana ability's cost -- match
// none), qualified by the cause's controller.
func (s *State) lifeCauseSpellOK(ctrl uint8, t *kop.Trigger) bool {
	if !s.res.active {
		return false
	}
	switch t.N {
	case kop.TrigNCauseSpellYouCtrl:
		if s.res.e.ctrl != ctrl {
			return false
		}
	case kop.TrigNCauseSpellOppCtrl:
		if s.res.e.ctrl == ctrl {
			return false
		}
	}
	return true
}

// firePlayerGen is the core's firePlayer on a generated table.
func (s *State) firePlayerGen(k *prep, ev kop.TrigEvent, who uint8, amt int16) {
	f := func(l trigListener, a *kop.Ability) {
		s.playerTrigGen(k, ev, l, a, who, amt)
	}
	s.eachBattlefieldTrigger(k, ev, f)
	if ev == kop.TrigDrawn && s.cfgx&cfgxGraveDrawn != 0 {
		s.fireGraveDrawn(k, who, amt) // trig_gravedrawn.go
	}
	s.eachCommandTrigger(k, ev, f)
	if ev == kop.TrigLifeGained {
		// an Effect-created trigger (an emblem) follows the battlefield's listeners
		s.fireEmblemsAmt(k, ev, who, NoObj, amt, func(*kop.Ability) bool { return true })
	}
}

// playerTrigGen queues listener l's ability a for a player event if it
// matches.
func (s *State) playerTrigGen(k *prep, ev kop.TrigEvent, l trigListener, a *kop.Ability, who uint8, amt int16) {
	s.playerTrigGenObj(k, ev, l, a, who, amt, NoObj)
}

// playerTrigGenObj is playerTrigGen with the event's object: the
// ManifestDread marker's milled card (the body's TriggeredCards), NoObj for
// every other player event, which derives its own.
//
//go:noinline
func (s *State) playerTrigGenObj(k *prep, ev kop.TrigEvent, l trigListener, a *kop.Ability, who uint8, amt int16, passed Obj) {
	t := &a.Trig
	if !whoseOK(t.Whose, l.ctrl, who) {
		return
	}
	if t.Flags2&kop.Tf2YourTurn != 0 && s.active != l.ctrl || t.Flags2&kop.Tf2OppTurn != 0 && s.active == l.ctrl {
		return
	}
	tobj := passed
	switch ev {
	case kop.TrigLifeLost:
		// Tf4CauseSpellAb (kop): the ValidCause$ SpellAbility gate (gorge
		// lifeLossCauseMatches): the loss was caused by the spell or
		// ability resolving right now, never by combat damage (gorge's
		// combatDamaging) and never with no resolution in flight -- a cost
		// payment, a mana ability's cost, gorge's Damaging == 0. The
		// kernel's combat damage step applies with no resolving entry
		// (s.res.active false there), so reading the resolving entry is
		// also the combat exclusion.
		if t.Flags4&kop.Tf4CauseSpellAb != 0 && !s.lifeCauseSpellOK(l.ctrl, t) {
			return
		}
	case kop.TrigDrawn:
		if t.Flags&kop.TfSecondDraw != 0 && !s.nthDrawOK(t, who) {
			return
		}
		if !drawStepGateOK(t, s.drawStepFirst(who)) {
			return
		}
	case kop.TrigLifeGained:
		if t.Flags2&kop.Tf2FirstTime != 0 && int(s.pl[who].lifeGained) != int(amt) {
			return
		}
	case kop.TrigAttackersDeclared:
		if t.N > 0 && !trigCmpOK(int(amt), t.N, true) {
			return
		}
	case kop.TrigManaExpend:
		if !s.expendCrossed(t, who, amt) {
			return
		}
	case kop.TrigDiscarded:
		if t.Flags2&kop.Tf2Cycled != 0 {
			return // fired only by a cycle (cycleDiscard)
		}
		tobj = s.g.disc
		mo := tobj
		if mo != NoObj && s.objs[mo].zone != kop.ZoneGraveyard {
			// a discard replaced away from the graveyard (Progenitus) is
			// still a discard: gorge's ValidCard has no zone
			mo = NoObj
		}
		if tobj == NoObj || !s.matchCardGen(k, t.Filter, s.objs[tobj].card, s.objs[tobj].owner(), l.ctrl, mo) {
			return
		}
		if mo == NoObj {
			tobj = NoObj // and Tinybones' "exile it from their graveyard" finds nothing
		}
	}
	if !s.trigOK(k, l, a) {
		return
	}
	s.queueTrig(k, l, a, tobj, who, amt)
}

// firePhaseGen is the core's firePhase on a generated table: battlefield
// triggers, then graveyard triggers (Flamewake Phoenix), then a commander's
// command-zone triggers, then emblems.
func (s *State) firePhaseGen(k *prep, st kop.Step) {
	f := func(l trigListener, a *kop.Ability) {
		if a.Trig.Step != st || !whoseOK(a.Trig.Whose, l.ctrl, s.active) && !(a.Trig.Whose == kop.WhoseEnchCtrl && s.enchCtrlIs(l.sl, s.active)) && !(a.Trig.Whose == kop.WhoseChosen && s.chosenIs(l.sl, s.active)) || !s.trigOK(k, l, a) {
			return
		}
		s.queueTrig(k, l, a, NoObj, s.active, 0)
	}
	s.eachBattlefieldTrigger(k, kop.TrigPhase, f)
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if s.objs[o].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind != kop.AbTriggered || a.Trig.Zone != kop.ZoneGraveyard || a.Trig.Event != kop.TrigPhase || a.Trig.Step != st {
						continue
					}
					l := trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}
					if !whoseOK(a.Trig.Whose, pp, s.active) || !s.trigOK(k, l, a) {
						continue
					}
					s.queueTrig(k, l, a, NoObj, s.active, 0)
				}
			}
		}
	}
	s.eachCommandTrigger(k, kop.TrigPhase, f)
	s.fireEmblems(k, kop.TrigPhase, s.active, NoObj, func(a *kop.Ability) bool { return a.Trig.Step == st })
}

// fireEmblems queues the emblem triggers of event ev.
func (s *State) fireEmblems(k *prep, ev kop.TrigEvent, who uint8, tobj Obj, extra func(a *kop.Ability) bool) {
	s.fireEmblemsAmt(k, ev, who, tobj, 0, extra)
}

// fireEmblemsAmt is fireEmblems for an event that carries an amount (life
// gained: Professor Dellian Fel's emblem reads TriggerCount$LifeAmount).
func (s *State) fireEmblemsAmt(k *prep, ev kop.TrigEvent, who uint8, tobj Obj, amt int16, extra func(a *kop.Ability) bool) {
	var done [MaxSeats]uint8 // OneOff emblems that fired (trig_emblemonce.go)
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			c := s.embCard(pp, i)
			cd := &k.t.Cards[c]
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			for j := range e.Abilities {
				ea := &e.Abilities[j]
				if ea.Kind != kop.AbTriggered || ea.Trig.Event != ev || !whoseOK(ea.Trig.Whose, pp, who) || !extra(ea) {
					continue
				}
				// the emblem's trigger runs as the creating card's latent copy
				for ab := range cd.Abilities {
					la := &cd.Abilities[ab]
					if la.Kind == kop.AbTriggered && la.AFlags&kop.AfLatent != 0 && la.Trig.Event == ev && la.Trig.Zone == kop.ZoneCommand {
						o := s.embObj(pp, i)
						n0 := s.npend
						s.enqueueLKI(o, s.objs[o].ver+1, c, uint8(ab), pp, tobj, who, s.embTrigAmt(e, pp, i, amt))
						if e.Once && s.npend > n0 && i < 8 {
							done[pp] |= 1 << uint(i)
						}
						break
					}
				}
			}
		}
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if done[pp] != 0 {
			s.embCompact(pp, done[pp])
		}
	}
}

// fireDamageGen is the core's fireDamage on a generated table: one damage event
// of amt from src, which outside a damage batch is its own batch, so
// DamageDealtOnce fires per event too.
func (s *State) fireDamageGen(k *prep, src Slot, amt int16, toPlayer, combat bool, rcp uint8) {
	s.fireDamageEvGen(k, src, amt, toPlayer, combat, nil, rcp, nil)
}

// dmgLatch is one source's DamageDealtOnce latch within a combat damage
// batch (gorge trigger_match.go damageBatchKey): the trigger lines already
// queued for the source, and the amounts the queued referent carries once
// the batch closes (closeDamageBatch patches in the batch total).
type dmgLatch struct {
	n       int
	l       [8]trigListener
	total   int16 // all the source's damage this batch
	totalPl int16 // the part dealt to players
}

// has reports whether l latched before the event whose walk began with seen
// entries. Entries of the event in progress do not count: each granted instance of
// one line is its own ability (CR 113.2c) and they all trigger on the event.
func (d *dmgLatch) has(l trigListener, seen int) bool {
	for i := 0; i < seen; i++ {
		if d.l[i].obj == l.obj && d.l[i].card == l.card && d.l[i].ab == l.ab {
			return true
		}
	}
	return false
}

// fireDamageEvGen checks one damage event's triggers in gorge's walk: every
// listener in battlefield order, each with its DamageDone and
// DamageDealtOnce lines in ability order (rules checkTriggers walks the
// listeners once per event). With a latch (a combat damage batch) a
// DamageDealtOnce line queues at its first matching event of the batch and
// carries the batch total; without one the event is its own batch.
func (s *State) fireDamageEvGen(k *prep, src Slot, amt int16, toPlayer, combat bool, latch *dmgLatch, rcp uint8, rl *refLatch) {
	m := s.subs[kop.TrigDamageDone].or(s.subs[kop.TrigDamageDealtOnce]).and(s.occ)
	if k.gen.grantTrigEv&(1<<kop.TrigDamageDone|1<<kop.TrigDamageDealtOnce) != 0 {
		m = s.grantTrigHosts(k, m)
	}
	if !m.any() {
		if k.gen.graveDmg && (rl == nil || !rl.agg) { // a batch's aggregate fire is not a damage event
			s.fireGraveDamage(k, src, s.perms[src].obj, s.perms[src].ctrl, amt, dmgPlayers(toPlayer, s.perms[src].ctrl, rcp), combat, rcp)
		}
		if k.gen.embCombatEv&(1<<kop.TrigDamageDone) != 0 {
			s.fireEmblemsDmg(k, src, amt, toPlayer, combat, rcp, false, latch)
		}
		return
	}
	srcObj := s.perms[src].obj
	srcCtrl := s.perms[src].ctrl
	dpl := s.dmgSeat(rcp, srcCtrl) // the player dealt damage (a to-player event)
	if pl, ok := recipientPlayer(rcp); ok {
		dpl = pl
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(m, &ord)
	isDmg := func(a *kop.Ability) bool {
		return a.Trig.Event == kop.TrigDamageDone || a.Trig.Event == kop.TrigDamageDealtOnce
	}
	seen := 0
	if latch != nil {
		seen = latch.n
	}
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, isDmg, func(l trigListener, a *kop.Ability) {
			t := &a.Trig
			if rl.batchLineSkips(t) {
				return
			}
			if t.Flags&kop.TfCombat != 0 && !combat || t.Flags2&kop.Tf2NonCombat != 0 && combat {
				return
			}
			if t.Flags&kop.TfToPlayer != 0 && !toPlayer && t.Filter2 == 0 {
				return
			}
			if t.Flags2&kop.Tf2ToOpponent != 0 && dpl == l.ctrl {
				return
			}
			if t.Flags2&kop.Tf2YourTurn != 0 && s.active != l.ctrl {
				return
			}
			once := t.Event == kop.TrigDamageDealtOnce && latch != nil
			if once && latch.has(l, seen) {
				return
			}
			if !s.objEventMatch(k, l, a, src) || !s.trigOK(k, l, a) {
				return
			}
			if t.Flags4 != 0 && !s.singleCauseOK(k, rcp) {
				// ValidCause$ Spell.numTargets EQ1 + TargetRelativeToCause$
				// (trig_params.go): the cause is the resolving
				// spell, so a permanent-source event fails it like gorge's
				// damageCauseMatches.
				return
			}
			tobj, tpl, n := srcObj, s.dmgSeat(rcp, srcCtrl), amt
			if k.gen.anyDmgRef {
				var ok bool
				if n, tobj, tpl, ok = s.ddoLine(k, l, a, src, srcObj, srcCtrl, amt, rcp, rl); !ok {
					return
				}
			}
			if t.Flags2&(kop.Tf2Batch|kop.Tf2PerReferent) == kop.Tf2Batch && !s.batchTrigOK(l, a, NoObj) {
				return
			}
			if once {
				n = latch.total
				if t.Flags&kop.TfToPlayer != 0 {
					n = latch.totalPl
				}
				if latch.n < len(latch.l) {
					latch.l[latch.n] = l
					latch.n++
				} else {
					s.capFull(StallDmgLatch) // loud: a ninth listener is not latched
				}
			}
			s.queueTrig(k, l, a, tobj, tpl, n)
		})
	}
	if k.gen.graveDmg && (rl == nil || !rl.agg) { // a batch's aggregate fire is not a damage event
		s.fireGraveDamage(k, src, srcObj, srcCtrl, amt, dmgPlayers(toPlayer, srcCtrl, rcp), combat, rcp)
	}
	if k.gen.embCombatEv&(1<<kop.TrigDamageDone) != 0 {
		s.fireEmblemsDmg(k, src, amt, toPlayer, combat, rcp, true, latch)
	}
}

// dmgPlayers is the player mask a permanent-source damage event names: the
// damaged player when rcp is one, else (an aggregate event to players) the
// source controller's opponent.
func dmgPlayers(toPlayer bool, srcCtrl uint8, rcp uint8) uint8 {
	if !toPlayer {
		return 0
	}
	if pl, ok := recipientPlayer(rcp); ok {
		return 1 << pl
	}
	return 1 << NextSeat(srcCtrl) // the aggregate: dmgSeat's stand-in
}

// dmgPl is the player mask fireDamageSrc takes for one damage target code.
func dmgPl(code uint8) uint8 {
	if isPlayerOrNone(code) {
		return 1 << CodeSeat(code)
	}
	return 0
}

// fireDamageSrc fires the damage triggers for noncombat damage whose
// source is not a permanent: a spell (Niv-Mizzet, Visionary's "a source
// you control deals noncombat damage to an opponent"), or an ability whose
// source left the battlefield (gorge reads its last known information:
// a sacrificed Fanatical Firebrand's ping, pair 3334), matched by its card.
func (s *State) fireDamageSrc(k *prep, o Obj, ctrl uint8, amt int16, plM uint8, rcp uint8, rl *refLatch) {
	toPlayer := plM != 0 // plM: the players dealt damage (bit p)
	if o == NoObj {
		return
	}
	evs := [2]kop.TrigEvent{kop.TrigDamageDone, kop.TrigDamageDealtOnce}
	f := func(l trigListener, a *kop.Ability) {
		t := &a.Trig
		if rl.batchLineSkips(t) {
			return
		}
		// ValidSource$ ...inZoneBattlefield: this source is no permanent
		if t.Flags2&kop.Tf2SrcInPlay != 0 {
			return
		}
		if t.Flags&(kop.TfCombat|kop.TfSelf) != 0 || t.Flags&kop.TfToPlayer != 0 && !toPlayer && t.Filter2 == 0 {
			return
		}
		// "to an opponent": one of the listener's opponents was dealt it
		// (a Firebrand pinging its own controller is not: pair 63811)
		if t.Flags2&kop.Tf2ToOpponent != 0 && !oppDamaged(plM, l.ctrl) || t.Flags2&kop.Tf2YourTurn != 0 && s.active != l.ctrl {
			return
		}
		if t.Filter != 0 && !s.matchCardGen(k, t.Filter, s.objs[o].card, ctrl, l.ctrl, o) || !s.trigOK(k, l, a) {
			return
		}
		if t.Flags4 != 0 && !s.singleCauseOK(k, rcp) {
			return // ValidCause$ Spell.numTargets EQ1 + TargetRelativeToCause$ (trig_params.go)
		}
		tobj, tpl, n := o, s.dmgSeat(rcp, ctrl), amt
		if k.gen.anyDmgRef {
			var ok bool
			if n, tobj, tpl, ok = s.ddoLine(k, l, a, NoSlot, o, ctrl, amt, rcp, rl); !ok {
				return
			}
		}
		s.queueTrig(k, l, a, tobj, tpl, n)
	}
	for _, ev := range evs {
		if s.subs[ev].any() {
			s.eachBattlefieldTrigger(k, ev, f)
		}
	}
	if k.gen.graveDmg && (rl == nil || !rl.agg) { // a batch's aggregate fire is not a damage event
		s.fireGraveDamage(k, NoSlot, o, ctrl, amt, plM, false, rcp)
	}
	for _, ev := range evs {
		if s.subs[ev].any() {
			s.eachCommandTrigger(k, ev, f)
		}
	}
}

// castTrigGen queues listener l's SpellCast ability a if spell e matches
// it (a generated table).
func (s *State) castTrigGen(k *prep, l trigListener, a *kop.Ability, e *stackEntry) {
	t := &a.Trig
	if !whoseOK(t.Whose, l.ctrl, e.ctrl) {
		return
	}
	if t.Whose == kop.WhoseNonActive && s.nonActiveCast(e) {
		return // trig_notyourturn.go: Player.NonActive, the caster is the active player
	}
	if t.Flags2&kop.Tf2OppTurn != 0 && s.active == l.ctrl || t.Flags2&kop.Tf2YourTurn != 0 && s.active != l.ctrl {
		return
	}
	if t.Flags4&kop.Tf4NamedCast != 0 && !s.castNamedOK(l, e) {
		return // repl_choosename.go: a spell with the source's chosen name
	}
	if !(s.castFilterOK(k, t.Filter, e, l) || t.Flags3&kop.Tf3OrTargets != 0 && s.castTargetsAny(k, t.Filter2, e, l)) || !s.trigOK(k, l, a) {
		return
	}
	if t.Flags2&(kop.Tf2CastFromHand|kop.Tf2ManaFromSelf) != 0 && !s.castProvOK(t, l, e) {
		return // trig_gravedrawn.go, mana_spendtrigger.go
	}
	if t.Flags3&kop.Tf3ManaFromTreasure != 0 && !s.castTreasureOK(e) {
		return // trig_validsa.go
	}
	if t.Flags2&kop.Tf2Increment != 0 && !s.incrementAdmits(k, l, e) {
		return // kw_increment.go: the cast's spend against the source's P/T (gorge incrementAdmits)
	}
	if t.N != 0 && t.Flags2&kop.Tf2NthSpell == 0 && k.gen.anySpentGate && !s.spentGateOK(k, t, e) {
		return // trig_spentvalid.go: the cast's spend against the Spell.ManaSpent gate (gorge spellCastMatches)
	}
	if t.Flags4&kop.Tf4NthFiltered != 0 {
		if !trigCmpOK(s.castPushesMatching(k, t.Filter, e.ctrl, l), t.N, false) {
			return
		}
	} else if t.Flags2&kop.Tf2NthSpell != 0 && !trigCmpOK(int(s.g.hist[e.ctrl][histSpells]), t.N, false) {
		return
	}
	if t.Flags2&kop.Tf2TargetsFilter2 != 0 && !s.castTargetsAll(k, t.Filter2, e, l) {
		return
	}
	if t.Flags4&^(kop.Tf4NthFiltered|kop.Tf4NamedCast) != 0 && !s.singleTargetOK(k, e) {
		return // IsSingleTarget$ (trig_params.go)
	}
	if t.Flags3&kop.Tf3MayPlaySrc != 0 && !s.mayPlaySpell(e) {
		MayPlayStats.OrdinaryCasts.Add(1)
		return // ValidSA$ Spell.MayPlaySource: not a may-play cast (trig_mayplaystatic.go)
	}
	amt := int16(0)
	if t.Flags2&kop.Tf2CastSnapshot != 0 {
		amt = int16(s.g.isCast[l.ctrl]) // gorge TriggerObjectsCurrentCastSpells: the casts up to this one
	}
	if t.Flags2&kop.Tf2CastSpent != 0 {
		amt = s.castSpentAmt(k, e) // gorge TriggerManaSpent: the spend as the trigger fires (opus)
	}
	if t.Flags2&kop.Tf2Increment != 0 {
		amt = int16(s.castSpent(e.obj)) // kw_increment.go: the spend the resolution recheck compares
	}
	if t.Flags4&kop.Tf4NamedCast != 0 {
		noteNamedCast(true)
	}
	s.queueTrig(k, l, a, e.obj, e.ctrl, amt)
}

// fireUntapsGen fires Untaps triggers for the active player's permanents
// that were tapped (pre) and are untapped now.
func (s *State) fireUntapsGen(k *prep, pre slotMask) {
	if !s.subs[kop.TrigUntaps].any() {
		return
	}
	for m := pre; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].flags&pfTapped == 0 {
			s.fireObjGen(k, kop.TrigUntaps, sl, s.perms[sl].obj, s.perms[sl].ctrl, 0)
		}
	}
}

// trigResolveOK rechecks a resolving trigger's intervening-if condition.
func (s *State) trigResolveOK(k *prep) bool {
	if s.g.trigTurn < 0xFFFF {
		s.g.trigTurn++
	}
	e := &s.res.e
	if e.kind != seTrigger || e.ab >= abCascade {
		return true
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.Trig.Event == kop.TrigSpellCast && a.Trig.Flags2&kop.Tf2Increment != 0 && !s.incrementResolves(k, e) {
		// kw_increment.go: CR 603.4's second check of the spend against the
		// source's P/T. The event test is REQUIRED: bit 24 is shared with
		// Tf2ToYou on DamageDone lines (kop/triggers.go), and this walk sees
		// every trigger, not only castTrigGen's SpellCast path.
		return false
	}
	if a.Trig.Event == kop.TrigETB && a.Trig.Flags2&kop.Tf2Evolve != 0 {
		if !s.evolveResolves(k, e) {
			return false // kw_evolve.go: CR 603.4's second check of the evolve comparison
		}
		s.evolveArm(e, &s.objs[e.obj])
	}
	if k.anyGate && a.AFlags&kop.AfCheckOnTrigger == 0 && !s.gateResolveOK(e, a) {
		return false
	}
	if a.AFlags&kop.AfResolveCount != 0 {
		s.countResolve(k, e)
	}
	if a.Trig.Cond == 0 || a.AFlags&kop.AfCheckOnTrigger != 0 {
		return true
	}
	return s.condEval(k, a.Trig.Cond, s.ctxEntry(e))
}

// combatEventsGen applies combat damage's life changes and fires its
// damage and life triggers on a generated table in gorge's event order
// (combat.go damageStep / runCombatAssignments): the attackers in the
// active player's battlefield order, each followed by its blockers in block
// order; per source its damage event (DamageDone triggers) then its life
// loss (one life event per source, so Bloodthirsty Conqueror triggers per
// source). After every source's assignments, one lifelink gain per
// lifelink source, in the order each source first landed damage (gorge
// flushLifelinkBatch), so Ajani's Pridemate triggers once per source.
func (s *State) combatEventsGen(k *prep, r *dmgRec, dealt bool) {
	if k.gen.anyPermMark {
		s.markDealtRec(k, r) // filter_permmarks.go
	}
	if k.gen.anyCombatHit {
		s.noteCombatHits(k, r) // value_turntallies.go
	}
	if k.gen.dmgBy != nil {
		s.noteDmgByRec(k, r) // DamagedBy marks (filter_combatwords.go)
	}
	var src [MaxPerm]Slot
	n := 0
	var seen slotMask
	add := func(sl Slot) {
		if !seen.has(sl) && r.dealt[sl] > 0 {
			seen = seen.with(sl)
			src[n] = sl
			n++
		}
	}
	var atk [MaxPerm]Slot
	na := s.battlefieldOrder(s.active, s.occ.and(s.ctrlM[s.active]), atk[:], 0)
	for _, a := range atk[:na] {
		if s.perms[a].flags&pfAttacking == 0 {
			continue
		}
		add(a)
		var bl [MaxPerm]Slot
		nb := 0
		for bm := s.occ.and(s.oppsM(s.active)); bm.any(); bm = bm.rest() {
			b := bm.first()
			if !s.blocks(b, a) {
				continue
			}
			j := nb
			for j > 0 && s.perms[bl[j-1]].bord > s.perms[b].bord {
				bl[j] = bl[j-1]
				j--
			}
			bl[j] = b
			nb++
		}
		for _, b := range bl[:nb] {
			add(b)
		}
	}
	// a source the walk missed (none expected) still deals its events
	for i := 0; i < r.nev; i++ {
		add(r.ev[i].src)
	}
	// a gain's plus-one (Angel of Vitality) is per source, each its own event
	life := s.subs[kop.TrigLifeLost].or(s.subs[kop.TrigLifeGained]).any() || k.gen.anyRepl&rbPlusLife != 0 ||
		k.gen.embCombatEv&(1<<kop.TrigLifeGained) != 0
	if !life && !dealt {
		for pp := uint8(0); pp < MaxSeats; pp++ {
			if r.toPl[pp] > 0 {
				s.loseLife(k, pp, int(r.toPl[pp]))
			}
		}
		for pp := uint8(0); pp < MaxSeats; pp++ {
			if r.gain[pp] > 0 {
				s.gainLife(k, pp, int(r.gain[pp]))
			}
		}
	}
	dmg := s.subs[kop.TrigDamageDone].or(s.subs[kop.TrigDamageDealtOnce]).any() || k.gen.embCombatEv&(1<<kop.TrigDamageDone) != 0 || k.gen.graveDmg
	if dmg && k.gen.anyBatch {
		defer s.batchClose(s.batchOpen())
	}
	var ll [MaxPerm]Slot
	nll := 0
	var rlp *refLatch // the step's per-referent latches (trig_damagedonce.go)
	if k.gen.anyDmgRef {
		rlp = &refLatch{r: r}
	}
	for _, sl := range src[:n] {
		loPl := int(s.npend)
		if dmg {
			// one Damage event per recipient, in assignment order (gorge
			// runCombatAssignments emits each assignment): DamageDone per
			// event, DamageDealtOnce at the source's first matching event
			latch := dmgLatch{total: r.dealt[sl], totalPl: r.dealtPl[sl]}
			for i := 0; i < r.nev; i++ {
				if ev := r.ev[i]; ev.src == sl && ev.n > 0 {
					if ev.dst == NoSlot {
						loPl = int(s.npend)
					}
					if rlp != nil {
						rlp.at = uint8(i + 1)
					}
					s.fireDamageEvGen(k, sl, ev.n, ev.dst == NoSlot, true, &latch, s.combatRcp(k, &r.ev[i]), rlp)
				}
			}
		}
		if !life || dealt {
			continue // dealt: the life changes were made per event (dealCombatEvent)
		}
		if r.dealtPl[sl] > 0 && isPlayerOrNone(s.perms[sl].atk) && !r.infM.has(sl) {
			s.loseLife(k, CodeSeat(s.perms[sl].atk), int(r.dealtPl[sl]))
			if int(s.npend)-loPl >= 2 {
				s.pendListenerOrder(loPl)
			}
		}
		if !s.occ.has(sl) || s.kw(k, sl)&kop.KwLifelink == 0 {
			continue
		}
		ll[nll] = sl
		nll++
	}
	for _, sl := range ll[:nll] {
		tot := 0
		for i := 0; i < r.nev; i++ {
			if ev := r.ev[i]; ev.src == sl && ev.n > 0 {
				tot += int(ev.n)
			}
		}
		s.gainLife(k, s.perms[sl].ctrl, tot)
	}
}
