package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A Continuous static's GainsTriggerAbsOf$ (Forge's triggered half of "has all
// activated and triggered abilities of ...", equal to GainsAbilitiesOf$; Koh,
// the Face Stealer's last chosen card): kop.Static.GainsTrig on the
// GainsOf static, and the kernel's gained-trigger walk. Each triggered ability
// of every card the static's GainsOf filter matches right now fires as the
// recipient's (the static's source, AffectedDefined$ Self): the queue carries
// the foreign card's body with the recipient as the trigger source (as
// gainedOffer does for activations, AkActivate mode 0xC0|index), so the
// ability's Card.Self is the recipient, not the lender (gorge's granted-trigger
// walk checkGrantedStaticTriggersUsing: the queue's Source is the visited
// object and triggerMatches resolves Self against it). A lender's own-entering
// or on-death triggers therefore never fire from a foreign zone: nothing the
// recipient does moves the lender, and Card.Self never matches the event. A
// once-each-turn line is counted per (recipient, foreign object, index) over
// the used pool, host = the foreign object plus one, as gorge's
// grantedLimitKey and the gained activations are; that host is the limit key
// only, so the stack entry's trigHost stays 0 (gorge mints the gained trigger
// with no GrantedBy and OriginalHost resolves to the recipient). A printed
// gate (Class level, max speed, Station charge, Saga chapter) is read on the
// recipient, as gorge's triggerMatches resolves it against the visited object.
//
// The walk reads the lenders the same way the offer walk does (gainedFrom):
// battlefield permanents in listener order, or every seat's graveyard/exile
// zone in seat then stored order for a GainsAbilitiesOfZones$ static, the
// filter read with the static's source and controller. genPrep.gainsTrigEv
// arms it: a GainsTrig lender is any table card, so every event the table's
// own triggers subscribe to can carry a gained trigger. A lender in a foreign
// zone is no battlefield subscriber, so the event fire sites' cheap
// subs[ev].any() gates call evLive (below) to admit the recipient, and
// triggers_gen.go eachBattlefieldTrigger adds it to the listener mask beside
// grantTrigHosts.

// DebugNoGainsTrig is the mutation: the gained-trigger walk fires nothing (the
// lockstep's TestKohGainsTrigLockstepMutation must then diverge).
var DebugNoGainsTrig bool

// tableHasGainsTrig reports whether a card of t has a GainsTrig static.
func tableHasGainsTrig(t *kop.Table) bool {
	for i := range t.Cards {
		if cardHasGainsTrig(&t.Cards[i]) {
			return true
		}
	}
	return false
}

// cardHasGainsTrig reports whether c has a Continuous static with a GainsOf
// filter and the GainsTrig flag.
func cardHasGainsTrig(c *kop.CardDef) bool {
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous &&
			a.Static.GainsOf != 0 && a.Static.GainsTrig {
			return true
		}
	}
	return false
}

// prepGainsTrig arms genPrep.gainsTrigEv: a GainsTrig lender is any card of
// the table, so every event the table's own triggers subscribe to can carry a
// gained trigger (the no-subscription events past nSubEvents get their walk
// through their own scans, which read slotTriggers).
func prepGainsTrig(t *kop.Table, g *genPrep) {
	if !tableHasGainsTrig(t) {
		return
	}
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			a := &c.Abilities[j]
			if a.Kind == kop.AbTriggered && a.AFlags&kop.AfLatent == 0 &&
				a.Trig.Event < 32 {
				g.gainsTrigEv |= 1 << a.Trig.Event
			}
			if a.Kind == kop.AbTriggered && a.AFlags&kop.AfLatent == 0 &&
				a.Trig.Event == kop.TrigCounterAddedAll {
				// the counter sites' cheap gate is the Once alias; the
				// watcher scans the battlefield and calls slotTriggers, which
				// runs this walk for the All event (trig_counteraddedall.go)
				g.gainsTrigEv |= 1 << kop.TrigCounterAddedOnce
				g.gainsCtrAll = true
			}
		}
	}
}

// evLive reports whether the subscribed event ev has a listener: a
// battlefield subscriber, or a GainsTrig recipient that can lend a trigger of
// ev. A gained trigger's lender can be in a foreign zone, so its own slot is
// not in subs[ev]; every event fire site's cheap subscription gate must admit
// the recipient through genPrep.gainsTrigEv or the line never queues. ev is a
// subscribed event (< nSubEvents): the no-subscription events past it use
// their own battlefield scans.
func (s *State) evLive(ev kop.TrigEvent) bool {
	if s.subs[ev].any() {
		return true
	}
	k := s.k()
	return k != nil && k.gen != nil && ev < nSubEvents && k.gen.gainsTrigEv&(1<<ev) != 0
}

// evLive2 is evLive for either of two events (a life or damage pair).
func (s *State) evLive2(a, b kop.TrigEvent) bool {
	return s.evLive(a) || s.evLive(b)
}

// grantedHostsFor adds to m the AddTrigger$ grantees and the GainsTrig
// recipients that listen to any event of evs (a TrigEvent bit set).
func (s *State) grantedHostsFor(k *prep, m slotMask, evs uint32) slotMask {
	if k.gen.grantTrigEv&evs != 0 {
		m = s.grantTrigHosts(k, m)
	}
	if k.gen.gainsTrigEv&evs != 0 {
		m = s.gainsTrigHosts(k, m)
	}
	return m
}

// gainsTrigHosts adds to listener mask m the recipients of the table's
// GainsTrig statics: a GainsTrig recipient listens to its lenders' events
// though its own card does not subscribe (the AddTrigger$ pair is
// trig_granted.go grantTrigHosts).
func (s *State) gainsTrigHosts(k *prep, m slotMask) slotMask {
	for g := s.statM; g.any(); g = g.rest() {
		sl := g.first()
		if s.occ.has(sl) && cardHasGainsTrig(&k.t.Cards[s.cardAt(sl)]) {
			m = m.with(sl)
		}
	}
	return m
}

// gainedTriggers calls f for the triggered abilities of event ev each lender
// of a GainsTrig static on slot sl lends: the statics in card order (the
// static walk's), each lender its filter matches right now. gorge's walk
// visits the recipient once per event and runs the granted-trigger loop there,
// so the calls land after the permanent's own triggers in slotTriggers.
func (s *State) gainedTriggers(k *prep, sl Slot, ev func(a *kop.Ability) bool, f func(l trigListener, a *kop.Ability)) {
	if DebugNoGainsTrig || s.genLost(k, sl) {
		return // a recipient that lost all abilities gains nothing (gorge grantLostTo)
	}
	p := &s.perms[sl]
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous ||
			a.Static.GainsOf == 0 || !a.Static.GainsTrig {
			continue
		}
		st := statSrc{a: a, sl: sl, ctrl: p.ctrl, obj: p.obj}
		// the recipient is the static's source (AffectedDefined$ Self, the
		// lowering's shape); its Filter is the lender filter
		if !s.statCond(k, st) {
			continue
		}
		s.gainedLenderTriggers(k, st, sl, ev, f)
	}
}

// gainedLenderTriggers calls f for each triggered ability of event ev of each
// card static st's GainsOf filter matches in the zone st names, queued as the
// recipient's (slot rcpt, its controller; the stack entry carries the foreign
// card's body with the recipient as the source).
func (s *State) gainedLenderTriggers(k *prep, st statSrc, rcpt Slot, ev func(a *kop.Ability) bool, f func(l trigListener, a *kop.Ability)) {
	emit := func(fo Obj, cid CardID) {
		cd := &k.t.Cards[cid]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			// a latent ability is not an ability the card has (it rides a
			// grant record); a trigger of another zone is the printed walk's
			// shape (slotTriggers reads ZoneNone and ZoneBattlefield only)
			if a.Kind != kop.AbTriggered || a.AFlags&kop.AfLatent != 0 || !ev(a) ||
				a.Trig.Zone != kop.ZoneNone && a.Trig.Zone != kop.ZoneBattlefield ||
				k.anyGate && !s.gateOK(rcpt, a) {
				continue
			}
			f(trigListener{obj: s.perms[rcpt].obj, sl: rcpt, ctrl: s.perms[rcpt].ctrl,
				card: cid, ab: uint8(j), host: fo + 1, gained: true}, a)
		}
	}
	if st.a.Static.Zone == kop.ZoneExile || st.a.Static.Zone == kop.ZoneGraveyard {
		// the lender zone the offer walk reads (gainedFrom): every seat's
		// exile/graveyard in seat order, each in its stored order, the owner
		// the zone's seat, the filter read with the static's source
		ctx := evalCtx{src: st.obj, srcSl: st.sl, ctrl: st.ctrl}
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.out(p) {
				continue
			}
			pl := &s.pl[p]
			zone, n := &pl.grave, int(pl.graveN)
			if st.a.Static.Zone == kop.ZoneExile {
				zone, n = &pl.exile, int(pl.exileN)
			}
			for i := 0; i < n; i++ {
				o := (*zone)[i]
				card := s.objs[o].card
				if !s.matchCardGenAt(k, st.a.Static.GainsOf, card, p, st.ctrl, o, &ctx) {
					continue
				}
				emit(o, card)
			}
		}
		return
	}
	var order [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &order)
	for oi := 0; oi < n; oi++ {
		fs := order[oi]
		if s.faceDown(k, fs) || !s.matchPermGen(k, st.a.Static.GainsOf, fs, st.ctrl, st.sl, layerKw) {
			continue // CR 708.2a: a face-down permanent has no abilities to lend
		}
		emit(s.perms[fs].obj, s.cardAt(fs))
	}
}
