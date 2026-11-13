package dirac

import (
	"math"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Combat. gorge poses attackers and blockers as one multi-select each; the
// kernel decomposes them into single picks in ascending slot order (an
// attacker / blocker can only be picked after every lower slot, so each
// declaration has exactly one pick sequence) closed by AkDone. Only the
// defending player, their planeswalkers and the battles their opponent
// protects (face_battle.go) can be attacked.

func (s *State) canAttack(k *prep, sl Slot) bool {
	p := &s.perms[sl]
	if p.flags&(pfTapped|pfAttacking) != 0 || !s.isCreature(k, sl) {
		return false
	}
	kw := s.kw(k, sl)
	if kw&kop.KwDefender != 0 && !(k.gen != nil && k.gen.anyCanAtkDef && s.canAttackDefender(k, sl)) {
		return false
	}
	if p.flags&pfSick != 0 && kw&kop.KwHaste == 0 {
		return false
	}
	if (s.statM.any() || k.embComb && s.anyEmb()) && s.staticFlag(k, kop.StatCantAttack, sl) {
		return false
	}
	if k.gen != nil && k.gen.anyOvCantAtk && s.fxM.has(sl) && s.ovCantAttack(k, sl) {
		return false // filter_combatwords.go
	}
	return true
}

// staticFlag reports whether a static of kind sk applies to slot sl.
func (s *State) staticFlag(k *prep, sk kop.StaticKind, sl Slot) bool {
	if k.gen != nil {
		return s.staticFlagGen(k, sk, sl)
	}
	m := s.statM
	for m.any() {
		src := m.first()
		m = m.rest()
		sp := &s.perms[src]
		if sp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			ab := &cd.Abilities[i]
			if ab.Kind == kop.AbStatic && ab.Static.Kind == sk && s.matchPerm(k, ab.Static.Filter, sl, sp.ctrl, src) {
				return true
			}
		}
	}
	return false
}

// declareMustAttack declares every able creature a MustAttack static
// applies to before the attackers decision (the requirement is satisfied
// by construction; gorge marks such options Required).
func (s *State) declareMustAttack(k *prep) {
	if !s.statM.any() && !(k.embComb && s.anyEmb()) && !goadOn(k) && !attackDutyOn(k) || s.mustAttackAsked() {
		return
	}
	m := s.ctrlM[s.active]
	for m.any() {
		sl := m.first()
		m = m.rest()
		if s.canAttack(k, sl) && (s.staticFlag(k, kop.StatMustAttack, sl) || s.attackDuty(k, sl) || s.mustAttackGoad(k, sl)) {
			if s.aloneAtk(k, sl) && !s.alonePartner(k, sl) {
				continue // CR 508.1d: no declaration obeys it alone (combat_alone.go)
			}
			s.perms[sl].flags |= pfAttacking
			s.perms[sl].atk = PlayerCode(s.nextAlive(s.active)) // two live seats only: mustAttackAsked asks at more
			s.perms[sl].bord = objWord(s.nAttackers)            // declaration order (fireAttacksGen)
			s.touchSl(sl)
			s.nAttackers++
		}
	}
}

func (s *State) anyAttacker(k *prep) bool {
	if atkTaxOn(k) {
		return s.atkTaxAnyOffer(k) // combat_attacktax.go
	}
	m := s.ctrlM[s.active]
	for m.any() {
		sl := m.first()
		m = m.rest()
		if s.canAttack(k, sl) {
			return true
		}
	}
	return false
}

func (s *State) attackerOptions(k *prep, d *Decision) bool {
	d.Kind, d.Player = DkAttackers, s.active
	m := s.ctrlM[s.active]
	def := NextSeat(s.active) // read on the two-seat path only (MaxSeats > 2 offers attackOffersN)
	// one attacker's offers are at most the player and every other
	// permanent (its walkers, any attackable battle)
	room := MaxOptions - 1 - (MaxSeats - 1 + s.occ.count())
	gd := goadOn(k)
	for m.any() {
		sl := m.first()
		m = m.rest()
		if !s.canAttack(k, sl) || s.aloneAtk(k, sl) && !s.alonePartner(k, sl) {
			continue // CR 506.5: no partner can join it (combat_alone.go)
		}
		if int(d.N) > room {
			return s.optionsFull(d)
		}
		o := s.perms[sl].obj
		if MaxSeats > 2 {
			s.attackOffersN(k, d, o, sl, gd && s.goadedAt(k, sl))
			if k.gen != nil && k.gen.anyBattle {
				s.addBattleOffers(k, d, o) // CR 310.7
			}
			continue
		}
		// defenders: the defending player, then each planeswalker they
		// control in slot order
		d.add(MakeAction(AkAttack, TCode(o), PlayerCode(def), 0))
		if gd && s.goadedAt(k, sl) {
			continue // a goad is satisfied by attacking a player only (act_goad.go)
		}
		for pm := s.ctrlM[def]; pm.any(); pm = pm.rest() {
			q := pm.first()
			if s.typesAt(k, q)&kop.TPlaneswalker != 0 {
				d.add(MakeAction(AkAttack, TCode(o), TCode(s.perms[q].obj), 0))
			}
		}
		if k.gen != nil && k.gen.anyBattle {
			s.addBattleOffers(k, d, o) // CR 310.7
		}
	}
	if (!s.mustAttackAsked() || !s.mustAttackLeft(k)) && !s.loneAloneAttacker(k) {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	if atkTaxOn(k) {
		s.atkTaxFilter(k, d) // combat_attacktax.go
	}
	return true
}

// mustAttackAsked reports whether a creature that must attack is declared
// in the attackers decision (Done withheld until it is) rather than before
// it: Config.GorgeMustAttack, and always in a game of more than two seats,
// where the requirement leaves the defender to choose (gorge marks every
// pair of such a creature Required).
func (s *State) mustAttackAsked() bool {
	return s.cfg&cfgGorgeMustAttack != 0 || MaxSeats > 2 && s.nSeats() > 2
}

// attackOffersN appends attacker o's offers at more than two seats: each
// opponent still in the game in ascending seat order (gorge
// attackOffersCompute walks AliveFrom(0)), the player and then each
// planeswalker that player controls in slot order. A goaded attacker
// (the permanent at sl) is offered players only (CR 508.1d, as at two
// seats), and a seat that goaded it is offered only when every other live
// seat goaded it too (CR 701.15b; gorge combat.GoadMayAttack).
//
//go:noinline
func (s *State) attackOffersN(k *prep, d *Decision, o Obj, sl Slot, goaded bool) {
	for q := uint8(0); q < s.nSeats(); q++ {
		if q == s.active || s.out(q) {
			continue
		}
		if goaded && s.goadDefenderBanned(k, sl, q) {
			continue
		}
		d.add(MakeAction(AkAttack, TCode(o), PlayerCode(q), 0))
		if goaded {
			continue
		}
		for pm := s.ctrlM[q]; pm.any(); pm = pm.rest() {
			if w := pm.first(); s.typesAt(k, w)&kop.TPlaneswalker != 0 {
				d.add(MakeAction(AkAttack, TCode(o), TCode(s.perms[w].obj), 0))
			}
		}
	}
}

// goadDefenderBanned reports whether defender seat q is banned for the
// goaded creature at sl (gorge combat.GoadMayAttack): q goaded it and some
// live seat other than the attacker's controller did not, so attacking q
// can satisfy every goad while attacking that other seat would.
func (s *State) goadDefenderBanned(k *prep, sl Slot, q uint8) bool {
	if !s.mSl(k, goadMech(q), sl) {
		return false
	}
	for r := uint8(0); r < s.nSeats(); r++ {
		if r != s.active && !s.out(r) && !s.mSl(k, goadMech(r), sl) {
			return true
		}
	}
	return false
}

// mustAttackLeft reports whether a creature that must attack and can is
// not declared yet (Config.GorgeMustAttack withholds Done).
func (s *State) mustAttackLeft(k *prep) bool {
	if !s.statM.any() && !(k.embComb && s.anyEmb()) && !goadOn(k) && !attackDutyOn(k) {
		return false
	}
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		sl := m.first()
		if s.canAttack(k, sl) && (s.staticFlag(k, kop.StatMustAttack, sl) || s.attackDuty(k, sl) || s.mustAttackGoad(k, sl)) &&
			!(s.aloneAtk(k, sl) && !s.alonePartner(k, sl)) {
			return true
		}
	}
	return false
}

// MustAttack reports whether battlefield object o attacks each combat if
// able now (a MustAttack static applies to it).
func (s *State) MustAttack(o Obj) bool {
	ob := &s.objs[o]
	return ob.zone == kop.ZoneBattlefield && ((s.statM.any() || s.k().embComb && s.anyEmb()) && s.staticFlag(s.k(), kop.StatMustAttack, ob.slot) || s.attackDuty(s.k(), ob.slot) || s.mustAttackGoad(s.k(), ob.slot))
}

// attackedPerm returns the slot of the planeswalker or battle attacker a
// attacks, or NoSlot when it attacks a player or the target is gone.
func (s *State) attackedPerm(k *prep, a Slot) Slot {
	t := s.perms[a].atk
	if isPlayerOrNone(t) {
		return NoSlot
	}
	ob := &s.objs[t]
	if ob.zone != kop.ZoneBattlefield || s.typesAt(k, ob.slot)&(kop.TPlaneswalker|kop.TBattle) == 0 {
		return NoSlot
	}
	return ob.slot
}

func (s *State) applyAttack(k *prep, a Action) {
	if a.Kind() == AkDone {
		s.finishAttacks(k)
		return
	}
	sl := s.objs[a.Source()].slot
	s.perms[sl].flags |= pfAttacking
	s.perms[sl].atk = a.Target()
	s.perms[sl].bord = objWord(s.nAttackers) // declaration order (fireAttacksGen)
	s.touchSl(sl)
	s.nAttackers++
	s.lastPick = sl
}

func (s *State) finishAttacks(k *prep) {
	if atkTaxOn(k) && s.nAttackers > 0 {
		s.atkTaxPay(k) // combat_attacktax.go: CR 508.1g-i, before the declaration's taps and triggers
	}
	fs := false
	var tapped slotMask
	fresh := 0 // attackers not declared in an earlier combat this turn
	m := s.ctrlM[s.active]
	if k.anyOppsAtt {
		s.noteOppsAttacked(m) // before any Attacks trigger reads it
	}
	for m.any() {
		sl := m.first()
		m = m.rest()
		p := &s.perms[sl]
		if p.flags&pfAttacking == 0 {
			continue
		}
		if !s.g.declM.has(sl) {
			s.g.declM.set(sl)
			fresh++
		}
		kw := s.kw(k, sl)
		if kw&kop.KwVigilance == 0 {
			if p.flags&pfTapped == 0 {
				tapped = tapped.with(sl)
			}
			p.flags |= pfTapped
			s.touchSl(sl)
		}
		if kw&(kop.KwFirstStrike|kop.KwDoubleStrike) != 0 {
			fs = true
		}
		if k.gen == nil && s.subs[kop.TrigAttacks].any() {
			s.fireObj(k, kop.TrigAttacks, sl, p.obj, p.ctrl, 0)
		}
	}
	s.fsStep = fs
	if k.gen != nil && k.gen.anyPermMark {
		s.markAttackers(k) // filter_permmarks.go
	}
	s.g.attacked[s.active] += uint8(fresh) // each creature once across extra combats (gorge de-duplicates the turn's declared attackers)
	s.declared = s.nAttackers > 0
	if k.gen != nil {
		s.fireAttacksGen(k)
		if k.gen.anyAttackedObj {
			s.markAttackedObj(k) // value_turntallies.go: after the declaration's triggers, so a FirstAttack read sees the earlier declarations only
		}
		if k.gen.embCombatEv&(1<<kop.TrigAttackersDeclared) != 0 {
			n0 := s.npend
			s.fireEmblemsAttack(k) // gorge: the delayed check after the event's face triggers
			if MaxSeats > 2 && !DebugDropDefSeat {
				s.stampDefSeat(n0, s.firstAtkSeat(k)) // the declaration's first event's seat
			}
		}
		if s.subs[kop.TrigTaps].any() {
			tb := s.tapBatchOpen(k)   // the declaration's taps are one tapping action (CR 508.1f; trig_tapall.go)
			s.fireTapsMask(k, tapped) // gorge taps the attackers after the declaration's triggers
			s.tapBatchClose(tb)
		}
		for m := tapped; m.any(); m = m.rest() {
			s.markTapped(k, m.first()) // after the declaration's Taps triggers
		}
	} else if s.nAttackers > 0 && s.subs[kop.TrigAttackersDeclared].any() {
		s.firePlayer(k, kop.TrigAttackersDeclared, s.active, int16(s.nAttackers))
	}
	s.openPriority()
}

// fireAttacksGen fires the declaration's Attacks and AttackersDeclared
// triggers like gorge: one DeclareAttackers event checked by one walk over
// the listeners in battlefield order (trigger_match.go checkTriggers), so a
// "whenever you attack" trigger and an "whenever this attacks" trigger are
// queued in their sources' order, each listener's abilities in ability
// order, an Attacks ability once per matching attacker (attackers in
// battlefield order).
func (s *State) fireAttacksGen(k *prep) {
	if s.nAttackers == 0 {
		return
	}
	m := s.subs[kop.TrigAttacks].or(s.subs[kop.TrigAttackersDeclared]).and(s.occ)
	if k.gen.grantTrigEv&(1<<kop.TrigAttacks|1<<kop.TrigAttackersDeclared) != 0 {
		m = s.grantTrigHosts(k, m)
	}
	emb := k.gen.embCombatEv&(1<<kop.TrigAttacks|1<<kop.TrigAttackersDeclared) != 0 && s.anyEmb()
	if !m.any() && len(k.gen.graveTrig) == 0 && !emb && !s.anyCommandTrigger(k, kop.TrigAttackersDeclared) {
		return
	}
	var atk [MaxPerm]Slot
	na := 0
	for p := uint8(0); p < MaxSeats; p++ {
		var am slotMask
		for x := s.ctrlM[p].and(s.occ); x.any(); x = x.rest() {
			if sl := x.first(); s.perms[sl].flags&pfAttacking != 0 {
				am = am.with(sl)
			}
		}
		na = s.battlefieldOrder(p, am, atk[:], na)
	}
	// gorge queues the per-attacker Attacks triggers in the order the
	// attackers were declared (handleAttackers): bord holds that
	// order for an attacker (it is a blocker-only field otherwise).
	for i := 1; i < na; i++ {
		for j := i; j > 0 && s.perms[atk[j]].bord < s.perms[atk[j-1]].bord; j-- {
			atk[j], atk[j-1] = atk[j-1], atk[j]
		}
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(m, &ord)
	// gorge emits one DeclareAttackers event per defender, sorted by the
	// defending seat and then the attacked planeswalker or battle by object
	// id, the player's own event first (rules/combat.go finishAttackers),
	// and runs the listeners once per event: the attack triggers of one
	// group are all queued before the next group's; the batch "whenever you
	// attack" fires on the first event only. Two-player, every group is the
	// one opponent's. Past two seats each queued trigger keeps its event's
	// defending seat (pendingTrig.defSeat), and an IsPresent$ checked as it
	// fires reads it from g.atkDef.
	var atkQ [MaxPerm]uint8
	for i, as := range atk[:na] {
		atkQ[i] = s.atkSeat(k, as)
	}
	if DebugAtkSeatsMerged {
		q0 := s.firstAtkSeat(k) // a test-only mutation: every attacked player's attackers in one event
		for i, as := range atk[:na] {
			if isPlayerTarget(s.perms[as].atk) {
				atkQ[i] = q0
			}
		}
	}
	first := true
	for q := uint8(0); q < MaxSeats; q++ {
		def := TCode(NoTarget)
		for {
			var grp [MaxPerm]Slot
			ng := 0
			for i, as := range atk[:na] {
				if t := s.perms[as].atk; atkQ[i] == q && (t == def || def == NoTarget && isPlayerOrNone(t)) {
					grp[ng] = as
					ng++
				}
			}
			if ng > 0 {
				n0 := s.npend
				if MaxSeats > 2 {
					s.g.atkDef = q + 1
				}
				s.fireAtkGroup(k, &ord, n, atk[:na], grp[:ng], first, emb)
				if MaxSeats > 2 {
					s.g.atkDef = 0
					if !DebugDropDefSeat {
						s.stampDefSeat(n0, q)
					}
				}
				first = false
			}
			next := TCode(NoTarget)
			for i, as := range atk[:na] {
				if t := s.perms[as].atk; atkQ[i] == q && isObjTarget(t) && (def == NoTarget || t > def) && t < next {
					next = t
				}
			}
			if next == NoTarget {
				break
			}
			def = next
		}
	}
}

// firstAtkSeat is the lowest seat an attacker attacks (gorge's first
// DeclareAttackers event of the declaration), noSeat with none.
func (s *State) firstAtkSeat(k *prep) uint8 {
	q := noSeat
	for x := s.ctrlM[s.active].and(s.occ); x.any(); x = x.rest() {
		if sl := x.first(); s.perms[sl].flags&pfAttacking != 0 {
			q = min(q, s.atkSeat(k, sl))
		}
	}
	return q
}

// stampDefSeat binds defending seat q to the pending triggers from n0 on
// (pendingTrig.defSeat); noSeat binds none.
func (s *State) stampDefSeat(n0 uint8, q uint8) {
	if q == noSeat {
		return
	}
	for i := n0; i < s.npend; i++ {
		s.pend[i].defSeat = q + 1
	}
}

// DebugAtkSeatsMerged is a test-only mutation: the attackers of every
// attacked player make one attack event, not one per defending seat.
var DebugAtkSeatsMerged bool

// DebugDropDefSeat is a test-only mutation: an attack trigger keeps no
// defending seat once queued.
var DebugDropDefSeat bool

// fireAtkGroup runs the listeners in ord[:n] over one DeclareAttackers event
// of fireAttacksGen: grp is the event's attackers, atk the declaration's;
// first marks the declaration's first event.
func (s *State) fireAtkGroup(k *prep, ord *[MaxPerm]Slot, n int, atk, grp []Slot, first, emb bool) {
	isAtk := func(a *kop.Ability) bool {
		return a.Trig.Event == kop.TrigAttacks || a.Trig.Event == kop.TrigAttackersDeclared
	}
	fire := func(l trigListener, a *kop.Ability) {
		if a.Trig.Event == kop.TrigAttackersDeclared {
			if a.Trig.Flags2&kop.Tf2DefPlayer != 0 {
				s.fireDefPlayer(k, l, a, grp) // act_discover.go
				return
			}
			if first {
				n := int16(s.nAttackers)
				if a.Trig.Filter != 0 {
					n = s.attackersMatching(k, l, a, atk) // filter_predicates.go
				}
				if n > 0 {
					s.fireAtkDecl(k, l, a, n, atk) // trig_attackers.go
				}
			}
			return
		}
		if a.Trig.Flags2&(kop.Tf2Alone|kop.Tf2Attacked) != 0 && !s.attacksGate(a.Trig.Flags2, l.ctrl, s.perms[grp[0]].atk) {
			return // gorge Alone$ / Attacked$ (filter_predicates.go)
		}
		if a.Trig.N == kop.TrigNTraining && !s.trainingGate(k, l.sl, atk) {
			return // Training (kw_training.go)
		}
		for _, as := range grp {
			if k.gen.anyAddPhase && a.Trig.Flags3&kop.Tf3FirstAttack != 0 && s.attackedBefore(k, s.perms[as].obj) {
				continue // FirstAttack$ True: AttacksThisTurn == 1 (act_addphase.go)
			}
			s.objTrigGen(k, kop.TrigAttacks, l, a, as, s.perms[as].obj, s.perms[as].ctrl, 0)
		}
	}
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, isAtk, fire)
	}
	if first && len(k.gen.graveTrig) != 0 {
		s.fireGraveAttackers(k, atk) // zone_changezoneparams.go
	}
	s.eachCommandTrigger(k, kop.TrigAttackersDeclared, fire)
	if emb {
		s.emblemAtkListeners(k, fire) // gorge: the delayed registrations are checked after the event's face triggers
	}
}

// canBlock reports whether blocker b may block attacker a.
func (s *State) canBlock(k *prep, b, a Slot) bool {
	akw := s.kw(k, a)
	if akw&kop.KwUnblockable != 0 {
		return false
	}
	bkw := s.kw(k, b)
	if k.gen != nil && k.gen.anyExHaste && s.exhBlocks(k, a, b) {
		return false // Agility Bobblehead, Cavern Stomper: a CantBlockBy ValidBlocker$ record (static_cantblockexcept.go)
	}
	if akw&kop.KwFlying != 0 && bkw&(kop.KwFlying|kop.KwReach) == 0 {
		return false
	}
	if akw&kop.KwProtectionAll != 0 {
		return false
	}
	if (akw|bkw)&(kop.KwFear|kop.KwShadow) != 0 && s.evadesShadowFear(k, b, akw, bkw) {
		return false
	}
	if k.gen != nil && k.gen.anyLandwalk && s.landwalkEvades(k, b, a) {
		return false
	}
	if k.gen != nil && k.gen.anyEvasion && s.evasionEvades(k, b, a) {
		return false
	}
	if k.gen != nil && k.gen.anyProtCol && s.protPrinted(k, a)&s.colorsGen(k, b) != 0 {
		return false // printed protection from the blocker's colour (CR 702.16e)
	}
	if k.gen != nil && k.gen.anyProtF && s.protQuality(k, a, b, s.cardAt(b)) {
		return false // printed protection from a quality the blocker has
	}
	if (s.statM.any() || k.embComb && s.anyEmb()) && s.blockForbidden(k, b, a) {
		return false
	}
	if k.gen != nil && k.gen.anySuspect && s.suspectNoBlock(k, b) {
		return false // a suspected creature can't block (CR 701.60a)
	}
	return true
}

func (s *State) blockerReady(k *prep, b Slot) bool {
	p := &s.perms[b]
	if p.flags&(pfTapped|pfBlocking) != 0 || !s.isCreature(k, b) {
		return false
	}
	return s.kw(k, b)&kop.KwCantBlock == 0
}

// menaceImpossible: a menace attacker that fewer than two of the
// defender's creatures could block is never offered (gorge
// blockPairScopeFor's minImpossible, counted over the whole declaration).
func (s *State) menaceImpossible(k *prep, a Slot) bool {
	need := s.minBlockers(k, a)
	if need == 0 {
		return false
	}
	if need >= 2 && k.gen != nil && k.gen.anyMaxBlk && s.maxOneBlocker(k, a) {
		return true // a floor above the cap: gorge's max < min, never offered (combat_maxblockers.go)
	}
	n := 0
	for bm := s.ctrlM[s.blockDef()]; bm.any(); bm = bm.rest() {
		b := bm.first()
		p := &s.perms[b]
		if p.flags&pfTapped != 0 || !s.isCreature(k, b) || s.kw(k, b)&kop.KwCantBlock != 0 {
			continue
		}
		if s.canBlock(k, b, a) {
			n++
			if n >= need {
				return false
			}
		}
	}
	return true
}

// blockForbidden applies CantBlock / CantBlockBy statics.
func (s *State) blockForbidden(k *prep, b, a Slot) bool {
	if k.gen != nil {
		return s.blockForbiddenGen(k, b, a)
	}
	m := s.statM
	for m.any() {
		src := m.first()
		m = m.rest()
		sp := &s.perms[src]
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			ab := &cd.Abilities[i]
			if ab.Kind != kop.AbStatic {
				continue
			}
			switch ab.Static.Kind {
			case kop.StatCantBlock:
				if s.matchPerm(k, ab.Static.Filter, b, sp.ctrl, src) {
					return true
				}
			case kop.StatCantBlockBy:
				if s.matchPerm(k, ab.Static.Filter, a, sp.ctrl, src) && s.matchPerm(k, ab.Static.Filter2, b, sp.ctrl, src) {
					return true
				}
			}
		}
	}
	return false
}

func (s *State) anyBlock(k *prep) bool {
	def := s.blockDef()
	bm := s.ctrlM[def]
	for bm.any() {
		b := bm.first()
		bm = bm.rest()
		if !s.blockerRoom(k, b) {
			continue
		}
		am := s.blockAtkM(k)
		for am.any() {
			a := am.first()
			am = am.rest()
			if s.perms[a].flags&pfAttacking != 0 && !s.blocks(b, a) && s.canBlock(k, b, a) && !s.menaceImpossible(k, a) && (!blkTaxOn(k) || s.blkTaxAdmits(k, b, a)) {
				return true
			}
		}
	}
	return false
}

func (s *State) blockerOptions(k *prep, d *Decision) bool {
	def := s.blockDef()
	d.Kind, d.Player = DkBlockers, def
	if k.gen != nil && k.gen.anyMustBlock {
		if mb := s.mustBlockAttackers(k); mb.any() || s.blockDuty(k).any() {
			if k.gen.anyExtraBlock && s.extraBlockReady(k) {
				return s.stallExtraBlockReq(d) // combat_extrablock.go
			}
			return s.blockerOptionsReq(k, d, mb) // combat_mustbeblocked.go
		}
	}
	if k.gen != nil && k.gen.anyLure {
		if lure := s.lureAttackers(k); lure.any() {
			if k.gen.anyExtraBlock && s.extraBlockReady(k) {
				return s.stallExtraBlockReq(d) // combat_extrablock.go
			}
			return s.blockerOptionsLure(k, d, lure) // combat_lure.go
		}
	}
	bm := s.ctrlM[def]
	for bm.any() {
		b := bm.first()
		bm = bm.rest()
		if !s.blockerRoom(k, b) {
			continue
		}
		am := s.blockAtkM(k)
		for am.any() {
			a := am.first()
			am = am.rest()
			if !s.blocks(b, a) && s.aloneBlockOfferable(k, b, a) && s.perms[a].flags&pfAttacking != 0 && s.canBlock(k, b, a) && !s.menaceImpossible(k, a) &&
				s.blocksCompletable(k, b, a) && !(k.gen != nil && k.gen.anyMaxBlk && s.blockCapped(k, a)) {
				if int(d.N) >= MaxOptions-1 { // keep room for Done
					return s.optionsFull(d)
				}
				d.add(MakeAction(AkBlock, TCode(s.perms[b].obj), TCode(s.perms[a].obj), 0))
			}
		}
	}
	if s.blocksCompletable(k, NoSlot, NoSlot) && s.menaceDeficit(k, NoSlot, NoSlot, nil) == 0 && !s.loneAloneBlocker(k) {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return true
}

// optionsFull ends the game at the kernel's capacity when a combat
// declaration has more (attacker, defender) or (blocker, attacker) pairs
// than a decision holds (gorge offers every pair): a stall (StallOptions),
// as a charm's or convoke's overfull ask does, instead of a decision cut
// short. Cold: only a board of dozens of creatures reaches it.
//
//go:noinline
func (s *State) optionsFull(d *Decision) bool {
	s.stall(StallOptions)
	d.N = 0
	d.Kind = DkNone
	return true
}

// minBlockers is the fewest creatures that may block attacker a: 2 for
// menace (CR 702.111b), a printed MinMaxBlocker Min$ (CR 509.1b,
// CardDef.MinBlockers) unless a has lost its abilities, the larger of the
// two (gorge blockPairScopeFor folds menace into the bound); 0 for none.
// Min$ All (kop.MinBlockersAll) is the attacked player's creature count.
func (s *State) minBlockers(k *prep, a Slot) int {
	n := 0
	if s.kw(k, a)&kop.KwMenace != 0 {
		n = 2
	}
	if k.gen != nil && k.gen.anyMinBlk {
		m := int(k.t.Cards[s.cardAt(a)].MinBlockers)
		if m == int(kop.MinBlockersAll) {
			m = s.defCreatures(k, a)
		}
		if m > n && !s.genLost(k, a) {
			n = m
		}
	}
	return n
}

// defCreatures is Min$ All's floor: every creature the player attacker a
// attacks controls, tapped or able to block or not (gorge
// combat.DefenderCreatureCount: a creature that cannot block still does not
// block, so menaceImpossible's able-blocker count falls short of it and the
// attacker is not offered, gorge blockPairScopeFor's minImpossible). An
// attacker with no defending player has none.
func (s *State) defCreatures(k *prep, a Slot) int {
	def := s.atkSeat(k, a)
	if def == noSeat {
		return 0
	}
	n := 0
	for m := s.ctrlM[def]; m.any(); m = m.rest() {
		if s.isCreature(k, m.first()) {
			n++
		}
	}
	return n
}

// menaceDeficit lists (into out, when non-nil) one entry per blocker still
// missing from each attacker that would be blocked by fewer creatures than
// its minimum (menace's two, a MinMaxBlocker Min$) with the extra block
// (xb blocks xa) added, and returns how many are missing.
func (s *State) menaceDeficit(k *prep, xb, xa Slot, out *[MaxPerm]Slot) int {
	return s.menaceDeficit2(k, xa, NoSlot, out)
}

// menaceDeficit2 is menaceDeficit with two extra blocks, on xa and ya.
func (s *State) menaceDeficit2(k *prep, xa, ya Slot, out *[MaxPerm]Slot) int {
	n := 0
	for am := s.ctrlM[s.active]; am.any(); am = am.rest() {
		a := am.first()
		ap := &s.perms[a]
		if ap.flags&pfAttacking == 0 {
			continue
		}
		nb := int(ap.nblock)
		if a == xa {
			nb++
		}
		if a == ya {
			nb++
		}
		if nb == 0 {
			continue
		}
		if need := s.minBlockers(k, a); nb < need {
			for ; nb < need; nb++ {
				if out != nil && n < len(out) {
					out[n] = a
				}
				n++
			}
		}
	}
	return n
}

// blocksCompletable: with the extra block (xb blocks xa; NoSlot for none)
// the declaration can still be completed into a legal one -- every
// attacker blocked by fewer than its minimum (menace, MinMaxBlocker) can get
// the missing blockers, distinct and unused (a bipartite matching, one
// deficit entry per missing blocker). The oracle's multi-pick emulation of
// gorge's KBlockers offers only completable picks, and Done only once the
// declaration is valid.
func (s *State) blocksCompletable(k *prep, xb, xa Slot) bool {
	return s.blocksCompletable2(k, xb, xa, NoSlot, NoSlot)
}

// blocksCompletable2 is blocksCompletable with two extra blocks (xb blocks
// xa, yb blocks ya; NoSlot for none).
func (s *State) blocksCompletable2(k *prep, xb, xa, yb, ya Slot) bool {
	var def [MaxPerm]Slot
	nd := s.menaceDeficit2(k, xa, ya, &def)
	if nd == 0 {
		return true
	}
	var free [MaxPerm]Slot
	nf := 0
	for bm := s.ctrlM[s.blockDef()]; bm.any(); bm = bm.rest() {
		b := bm.first()
		if b != xb && b != yb && s.blockerReady(k, b) {
			free[nf] = b
			nf++
		}
	}
	if nf < nd {
		return false
	}
	var match [MaxPerm]int8 // free blocker index -> deficit index, -1 none
	for i := range match[:nf] {
		match[i] = -1
	}
	for i := 0; i < nd; i++ {
		var seen [MaxPerm]bool
		if !s.augment(k, i, &def, &free, nf, &match, &seen) {
			return false
		}
	}
	return true
}

func (s *State) augment(k *prep, i int, def, free *[MaxPerm]Slot, nf int, match *[MaxPerm]int8, seen *[MaxPerm]bool) bool {
	for j := 0; j < nf; j++ {
		if seen[j] || !s.canBlock(k, free[j], def[i]) {
			continue
		}
		seen[j] = true
		if match[j] < 0 || s.augment(k, int(match[j]), def, free, nf, match, seen) {
			match[j] = int8(i)
			return true
		}
	}
	return false
}

func (s *State) applyBlock(k *prep, a Action) {
	if a.Kind() == AkDone {
		if blkTaxOn(k) && !s.blkTaxPay(k) {
			s.dropBlockDeclaration(k) // combat_blocktax.go: CR 509.1d, the unpayable declaration is declined
		}
		if MaxSeats > 2 {
			s.blockRound(k, s.g.blkDef)
			return
		}
		s.finishBlocks(k)
		return
	}
	b := s.objs[a.Source()].slot
	at := s.objs[a.Target()].slot
	bp := &s.perms[b]
	s.blockSeq++
	if bp.flags&pfBlocking != 0 {
		s.xblkRecs()[b] = xblkRec{a: uint8(at) + 1, ord: s.blockSeq} // the second attacker (combat_extrablock.go)
	} else {
		bp.flags |= pfBlocking
		bp.blocking = at
		bp.bord = objWord(s.blockSeq)
	}
	s.perms[at].nblock++
	s.touchSl(b)
	s.touchSl(at)
	s.lastPick = b
}

// blockRound asks the next defending player after seat from, in turn order
// (CR 802.4: APNAP from the active player), that has a legal block, and
// finishes the declaration once none is left (gorge askBlockers skips a
// defender with no attacker or no legal block option).
//
// The walk is over seat indices up to the active seat's, skipping departed
// seats, rather than over nextAlive until it returns the active seat: an
// active player who has left the game (CR 800.4a: the turn continues without
// one) is never returned by nextAlive, and that loop never ended -- a spinning
// goroutine kfuzz's watchdog could report as a hang but never stop.
//
//go:noinline
func (s *State) blockRound(k *prep, from uint8) {
	n := s.nSeats()
	for q := from; ; {
		if q++; q >= n {
			q = 0
		}
		if q == s.active || q == from {
			break
		}
		if s.out(q) {
			continue
		}
		s.g.blkDef = q
		if s.anyBlock(k) {
			s.ctl = ctlBlockers
			return
		}
	}
	s.finishBlocks(k)
}

// finishBlocks validates menace (an attacker with menace blocked by exactly
// one creature has that block removed), marks blocked attackers, fires
// block triggers and opens the step's priority round.
func (s *State) finishBlocks(k *prep) {
	am := s.ctrlM[s.active]
	for am.any() {
		a := am.first()
		am = am.rest()
		ap := &s.perms[a]
		if ap.flags&pfAttacking == 0 {
			continue
		}
		if ap.nblock > 0 && int(ap.nblock) < s.minBlockers(k, a) {
			bm := s.oppsM(s.active)
			for bm.any() {
				b := bm.first()
				bm = bm.rest()
				if s.blocks(b, a) {
					s.forgetBlocked(b, a)
					if s.perms[b].blocking == NoSlot {
						s.perms[b].flags &^= pfBlocking
					}
				}
			}
			ap.nblock = 0
			s.touchSl(a)
		}
		if ap.nblock > 0 {
			ap.flags |= pfBlocked
			s.touchSl(a)
		}
	}
	if k.gen != nil && s.subs[kop.TrigBecomesBlocked].any() {
		s.fireBlockedGen(k)
	}
	if k.gen != nil && s.subs[kop.TrigBlocks].any() {
		s.fireBlocksGen(k)
	}
	if k.gen != nil && k.gen.anyUnblocked {
		s.fireUnblocked(k)
	}
	// first strike among blockers
	bm := s.oppsM(s.active)
	for bm.any() {
		b := bm.first()
		bm = bm.rest()
		if s.perms[b].flags&pfBlocking != 0 {
			if s.kw(k, b)&(kop.KwFirstStrike|kop.KwDoubleStrike) != 0 {
				s.fsStep = true
			}
			if k.gen == nil && s.subs[kop.TrigBlocks].any() {
				s.fireObj(k, kop.TrigBlocks, b, s.perms[b].obj, s.perms[b].ctrl, 0)
			}
		}
	}
	s.openPriority()
}

// sat16 clamps a damage amount to the int16 range of the damage record. An
// amount past it is a declared capacity stall (StallPT), never a silent wrap.
func (s *State) sat16(n int) int16 {
	if n > math.MaxInt16 {
		s.capFull(StallPT)
		return math.MaxInt16
	}
	if n < 0 {
		return 0
	}
	return int16(n)
}

// addSat16 is a + n saturated at the int16 range (see sat16).
func (s *State) addSat16(a int16, n int) int16 {
	return s.sat16(int(a) + n)
}

// dmgRec accumulates simultaneous combat damage.
type dmgRec struct {
	toPerm  [MaxPerm]int16
	dtPerm  slotMask // dealt deathtouch damage
	toPl    [MaxSeats]int16
	poison  [MaxSeats]int16 // infect/toxic poison, added after the pass (kw_infect.go)
	gain    [MaxSeats]int16 // lifelink
	dealt   [MaxPerm]int16
	dealtPl [MaxPerm]int16
	// ev lists each (source, amount) damage event in assignment order (a
	// generated table's lifelink sums them per source into one life event)
	ev  [MaxPerm]dmgEv
	nev int
	// infM: sources whose damage to a player is poison, not life loss
	// (infect; kw_infect.go infectCombat)
	infM slotMask
}

type dmgEv struct {
	src, dst Slot // dst: the damaged permanent, NoSlot for a player
	n        int16
}

// dtEvents counts the damage events r deals sl from deathtouch sources
// (gorge marks each: rules/combat.go).
func (s *State) dtEvents(k *prep, r *dmgRec, sl Slot) int {
	if k.gen != nil && k.gen.anyRepl&rbHealSelf != 0 && s.selfDamageHost(k, sl, rbHealSelf) {
		return 1 // each event healed the marks before it: only the last survives
	}
	n := 0
	for i := 0; i < r.nev; i++ {
		if ev := &r.ev[i]; ev.dst == sl && ev.n > 0 && s.kw(k, ev.src)&kop.KwDeathtouch != 0 {
			n++
		}
	}
	return max(n, 1)
}

func (r *dmgRec) note(src, dst Slot, n int16) {
	if n > 0 && r.nev < len(r.ev) {
		r.ev[r.nev] = dmgEv{src, dst, n}
		r.nev++
	}
}

// prevent removes all the damage assigned to permanent sl: none of it is
// dealt (no lifelink, no damage-dealt trigger for it).
func (r *dmgRec) prevent(sl Slot) {
	r.toPerm[sl] = 0
	r.dtPerm.clr(sl)
	for i := 0; i < r.nev; i++ {
		if ev := &r.ev[i]; ev.dst == sl && ev.n > 0 {
			r.dealt[ev.src] -= ev.n
			ev.n = 0
		}
	}
}

// dealsNow: does sl deal combat damage in this damage step?
func (s *State) dealsNow(k *prep, sl Slot, first bool) bool {
	kw := s.kw(k, sl)
	fs := kw&(kop.KwFirstStrike|kop.KwDoubleStrike) != 0
	if first {
		return fs
	}
	if kw&kop.KwDoubleStrike != 0 {
		return true
	}
	return s.perms[sl].flags&pfDealtFirst == 0
}

// combatDamage assigns and deals one combat damage step. Attacker damage
// to its blockers goes in block order, lethal to each (1 with deathtouch)
// and the rest to the last blocker, or with trample to the player (gorge's
// automatic trample assignment and greedy fallback; gorge poses a division
// choice for a non-trampler with 2+ blockers, the kernel does not).
func (s *State) combatDamage(k *prep, first bool) bool {
	var r dmgRec
	am := s.ctrlM[s.active]
	for am.any() {
		a := am.first()
		am = am.rest()
		ap := &s.perms[a]
		if ap.flags&pfAttacking == 0 || !s.dealsNow(k, a, first) {
			continue
		}
		if first {
			ap.flags |= pfDealtFirst
			s.touchSl(a)
		}
		pw := s.power(k, a)
		if k.gen != nil && k.gen.anyCdmgTough {
			pw = s.combatAmtTough(k, a)
		}
		if pw <= 0 {
			continue
		}
		kw := s.kw(k, a)
		dt := kw&kop.KwDeathtouch != 0
		if ap.flags&pfBlocked == 0 {
			s.toDefender(k, &r, a, pw)
			continue
		}
		// blockers in block order
		var bl [MaxPerm]Slot
		nb := 0
		bm := s.oppsM(s.active)
		for bm.any() {
			b := bm.first()
			bm = bm.rest()
			if s.blocks(b, a) {
				bl[nb] = b
				nb++
			}
		}
		for i := 1; i < nb; i++ {
			for j := i; j > 0 && s.perms[bl[j]].bord < s.perms[bl[j-1]].bord; j-- {
				bl[j], bl[j-1] = bl[j-1], bl[j]
			}
		}
		if nb == 0 {
			if kw&kop.KwTrample != 0 {
				s.toDefender(k, &r, a, pw)
			}
			continue
		}
		if s.combatArm().divDone.has(a) {
			sum := 0
			for i := 0; i < nb; i++ {
				b := bl[i]
				amt := int(*s.divAmtFor(b, a))
				sum += amt
				r.toPerm[b] = s.addSat16(r.toPerm[b], amt)
				if dt && amt > 0 {
					r.dtPerm.set(b)
				}
				r.dealt[a] = s.addSat16(r.dealt[a], amt)
				r.note(a, b, s.sat16(amt))
			}
			if kw&kop.KwTrample != 0 && pw > sum {
				s.toDefender(k, &r, a, pw-sum) // an assignment frozen by replOrderFreeze
			}
			continue
		}
		rem := pw
		trample := kw&kop.KwTrample != 0
		for i := 0; i < nb && rem > 0; i++ {
			b := bl[i]
			_, t := s.pt(k, b)
			lethal := t - int(s.perms[b].damage) - int(r.toPerm[b])
			if dt {
				// A deathtouch source owes the blocker only 1, or nothing
				// once the blocker already carries a Deathtouched mark: the
				// mark alone is lethal (CR 704.5h), so a second-strike
				// trampler may spill over without assigning to it again
				// (CR 702.19b; gorge's lethalDamageNeed).
				if s.perms[b].flags&pfDeathtouched != 0 {
					lethal = 0
				} else if lethal > 1 {
					lethal = 1
				}
			}
			if lethal < 0 {
				lethal = 0
			}
			amt := lethal
			if i == nb-1 && !trample {
				amt = rem
			}
			if amt > rem {
				amt = rem
			}
			r.toPerm[b] = s.addSat16(r.toPerm[b], amt)
			if dt && amt > 0 {
				r.dtPerm.set(b)
			}
			r.dealt[a] = s.addSat16(r.dealt[a], amt)
			r.note(a, b, s.sat16(amt))
			rem -= amt
		}
		if rem > 0 {
			if trample {
				s.toDefender(k, &r, a, rem)
			} else {
				r.toPerm[bl[nb-1]] = s.addSat16(r.toPerm[bl[nb-1]], rem)
				r.dealt[a] = s.addSat16(r.dealt[a], rem)
				r.note(a, bl[nb-1], s.sat16(rem))
			}
		}
	}
	// blockers
	bm := s.oppsM(s.active)
	for bm.any() {
		b := bm.first()
		bm = bm.rest()
		bp := &s.perms[b]
		if bp.flags&pfBlocking == 0 || bp.blocking == NoSlot || !s.dealsNow(k, b, first) {
			continue
		}
		if first {
			bp.flags |= pfDealtFirst
			s.touchSl(b)
		}
		pw := s.power(k, b)
		if k.gen != nil && k.gen.anyCdmgTough {
			pw = s.combatAmtTough(k, b)
		}
		if pw <= 0 {
			continue
		}
		if s.blockedTwo(b) {
			s.blockerTwoDamage(k, &r, b, pw) // CR 510.1d (combat_extrablock.go)
			continue
		}
		a := bp.blocking
		r.toPerm[a] = s.addSat16(r.toPerm[a], pw)
		if s.kw(k, b)&kop.KwDeathtouch != 0 {
			r.dtPerm.set(a)
		}
		r.dealt[b] = s.addSat16(r.dealt[b], pw)
		r.note(b, a, s.sat16(pw))
	}
	if k.gen != nil {
		// Freeze first: when any event of this pass poses the replacement
		// order ask, every event of the pass is dealt in that ordered walk,
		// so shieldCombat must leave all the shields to it (a shield on a
		// later event is a candidate of the same ask, not an automatic
		// spend before the ask is answered).
		if (k.gen.anyRepl&(rbDblDmg|rbShield|rbPrevCol) != 0 || replPlusOnly(k.gen)) && !s.g.ro.on {
			s.replOrderFreeze(k, &r, first) // gorge's replacement-order ask
		}
		if k.gen.anyRepl&rbShield != 0 {
			s.shieldCombat(k, &r) // CR 122.1c, event by event (ctr_shield.go)
		}
		if k.gen.anyRepl&(rbDblDmg|rbProtAll|rbProtCol|rbPlusDmg|rbPrevCol|rbPrevMill|rbSelfDmg) != 0 {
			s.replCombatGen(k, &r) // amounts first, then prevention
		}
		s.preventCombatGen(k, &r)
		if k.gen.anyRepl&rbHealSelf != 0 && !s.g.ro.on {
			s.healCombat(k, &r) // a heal host's events heal each other (repl_selfdamage.go)
		}
		s.noteCmdDamagePass(&r)
		if s.g.ro.on {
			return s.replOrderCombat(k, &r)
		}
		if k.gen.anyInfect {
			s.infectCombat(k, &r)
		}
	}
	if k.gen == nil {
		s.noteCmdDamagePass(&r)
	}
	// apply simultaneously
	m := s.occ
	for m.any() {
		sl := m.first()
		m = m.rest()
		p := &s.perms[sl]
		if n := r.toPerm[sl]; n > 0 {
			s.touchSl(sl)
			s.dmgPerm(k, sl, p, int(n))
			if r.dtPerm.has(sl) {
				// a planeswalker too (gorge effects/damage.go, combat.go: any
				// damaged permanent), one mark per deathtouch source's event
				p.markDT(s.dtEvents(k, &r, sl))
			}
		}
		if n := r.dealt[sl]; n > 0 && s.kw(k, sl)&kop.KwLifelink != 0 {
			r.gain[p.ctrl] = s.addSat16(r.gain[p.ctrl], int(n))
		}
	}
	if k.gen != nil {
		s.combatEventsGen(k, &r, false)
		for pp := uint8(0); pp < MaxSeats; pp++ {
			if r.poison[pp] > 0 {
				if MaxSeats == 2 {
					s.g.ctrAdder = 2 - pp // the attackers' controller puts them (1-pp, +1)
				} else {
					s.g.ctrAdder = s.active + 1 // the attackers' controller puts them
				}
				s.addPoison(pp, int(r.poison[pp]))
				s.g.ctrAdder = 0
			}
		}
		if k.anyInit {
			s.initCombat(k, &r) // CR 726.2 (act_initiative.go)
		}
		if k.gen.anyMonarch {
			s.monarchCombat(&r) // CR 725.2 (act_monarch.go)
		}
		return false
	}
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
	if s.subs[kop.TrigDamageDone].any() {
		m = s.occ
		for m.any() {
			sl := m.first()
			m = m.rest()
			if r.dealt[sl] > 0 {
				s.fireDamage(k, sl, r.dealt[sl], r.dealtPl[sl] > 0, true, TCode(NoTarget))
			}
		}
	}
	return false
}

// toDefender routes n combat damage from attacker a to what it attacks:
// the defending player, or its planeswalker (none if that left).
func (s *State) toDefender(k *prep, r *dmgRec, a Slot, n int) {
	t := s.perms[a].atk
	if isPlayerOrNone(t) {
		r.toPl[CodeSeat(t)] = s.addSat16(r.toPl[CodeSeat(t)], n)
		r.dealtPl[a] = s.addSat16(r.dealtPl[a], n)
		r.dealt[a] = s.addSat16(r.dealt[a], n)
		r.note(a, NoSlot, s.sat16(n))
		return
	}
	if q := s.attackedPerm(k, a); q != NoSlot {
		r.toPerm[q] = s.addSat16(r.toPerm[q], n)
		r.dealt[a] = s.addSat16(r.dealt[a], n)
		r.note(a, q, s.sat16(n))
	}
}

// fireDamage enqueues DamageDone triggers for source sl.
func (s *State) fireDamage(k *prep, src Slot, amt int16, toPlayer, combat bool, rcp TCode) {
	if k.gen != nil {
		s.fireDamageGen(k, src, amt, toPlayer, combat, rcp)
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.subs[kop.TrigDamageDone].and(s.occ), &ord)
	for _, ls := range ord[:n] {
		lp := &s.perms[ls]
		cd := &k.t.Cards[s.cardAt(ls)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigDamageDone {
				continue
			}
			f := a.Trig.Flags
			if f&kop.TfCombat != 0 && !combat || f&kop.TfToPlayer != 0 && !toPlayer {
				continue
			}
			if f&kop.TfSelf != 0 {
				if ls != src {
					continue
				}
			} else if !s.matchPerm(k, a.Trig.Filter, src, lp.ctrl, ls) {
				continue
			}
			s.enqueue(lp.obj, s.objs[lp.obj].ver, s.cardAt(ls), uint8(i), lp.ctrl, s.perms[src].obj, s.dmgSeat(rcp, s.perms[src].ctrl), amt)
		}
	}
}

func (s *State) loseLife(k *prep, p uint8, n int) {
	if n <= 0 {
		return
	}
	if k.gen != nil && k.gen.anyRepl&rbDblLoss != 0 {
		n = s.replLoss(k, p, n) // repl_lifelossdouble.go
	}
	s.pl[p].life -= int16(n)
	s.pl[p].lifeLost += int16(n)
	if s.subs[kop.TrigLifeLost].any() {
		s.firePlayer(k, kop.TrigLifeLost, p, int16(n))
	}
	if k.speed {
		s.speedLoss(p) // kw_speed.go
	}
}

func (s *State) gainLife(k *prep, p uint8, n int) {
	if n <= 0 {
		return
	}
	if k.gen != nil && k.gen.anyCantGain && s.playerBanned(k, kop.StatCantGainLife) {
		return // static_cantdraw.go: swallowed before any gain replacement
	}
	if k.gen != nil && k.gen.anyRepl&(rbDblLife|rbPlusLife) != 0 {
		n = s.replLife(k, p, n) // act_discover.go
	}
	s.pl[p].life += int16(n)
	s.pl[p].lifeGained += int16(n)
	if s.subs[kop.TrigLifeGained].any() || k.gen != nil && k.gen.embCombatEv&(1<<kop.TrigLifeGained) != 0 {
		s.firePlayer(k, kop.TrigLifeGained, p, int16(n))
	}
}

// damageStep runs a combat damage step: first every damage division the
// attackers' controller must choose (attackers in slot order), then the
// damage, then the step's continuation. ans: the step runs on from an
// answer (a division or a replacement order; replOrderCombat).
func (s *State) damageStep(k *prep, first, ans bool) {
	s.divFirst = first
	if !ans {
		s.scrCombatEnter() // arm A of scr starts afresh (scratch.go)
		s.xblkDivReset()
	}
	// api:Fog (gorge damageStep's fogActive): no damage pass at all; the
	// damage divisions are still asked first (gorge asks them before its
	// damageStep, which then returns at fogActive; kverify-c4)
	fog := k.gen != nil && s.g.fog
	if fog {
		s.fogNoPrev(k) // static_cantprevent.go
	}
	if s.cfg&cfgAutoDivide == 0 && s.nextDivision(k, first) {
		s.ctl = ctlDivide
		return
	}
	if !fog && s.combatDamage(k, first) {
		return // paused on a replacement-order ask
	}
	s.combatArm().divDone = slotMask{}
	s.xblkDivReset()
	// state-based actions run right after each damage pass (gorge
	// completeCombatPass -> checkStateBased); a legend-rule ask waits for
	// the next priority check
	ask := uint8(0)
	if ctl := s.ctl; s.sba(k) {
		if s.over {
			return
		}
		// a casualty's order ask is posed inside the check (CR 616.1)
		switch s.ctl {
		case ctlExileOrder, ctlCmdZone0, ctlCmdZone1, ctlCmdZone2, ctlCmdZone3, ctlCmdZone4, ctlCmdZone5, ctlCmdZone6, ctlCmdZone7:
			ask = s.ctl
		}
		s.ctl = ctl
	}
	// CR 510.3: a priority round in the combat damage step, so its damage
	// triggers resolve before end of combat's
	s.openPriority()
	if ask != 0 {
		s.ctl = ask
	}
}

// liveBlockers writes attacker a's blockers in block order.
func (s *State) liveBlockers(a Slot, bl *[MaxPerm]Slot) int {
	nb := 0
	for bm := s.oppsM(s.active); bm.any(); bm = bm.rest() {
		b := bm.first()
		if s.blocks(b, a) {
			bl[nb] = b
			nb++
		}
	}
	for i := 1; i < nb; i++ {
		for j := i; j > 0 && s.perms[bl[j]].bord < s.perms[bl[j-1]].bord; j-- {
			bl[j], bl[j-1] = bl[j-1], bl[j]
		}
	}
	return nb
}

// splits is the number of ways to divide p damage among n blockers,
// C(p+n-1, n-1), saturating above 128.
func splits(p, n int) int {
	c := 1
	for i := 1; i < n; i++ {
		c = c * (p + i) / i
		if c > 128 {
			return 129
		}
	}
	return c
}

// nextDivision finds the next attacker (slot order) whose damage its
// controller divides: dealing damage now, power > 0, no trample, two or
// more blockers, at most 128 splits (gorge combat.go:1483-1505,
// 1652-1700; above 128 gorge falls back to the automatic assignment).
func (s *State) nextDivision(k *prep, first bool) bool {
	// attackers in battlefield (entry) order, as gorge askNextDivision
	var order [MaxPerm]Slot
	no := 0
	for am := s.ctrlM[s.active]; am.any(); am = am.rest() {
		a := am.first()
		if s.perms[a].flags&pfAttacking == 0 {
			continue
		}
		j := no
		for j > 0 && s.perms[order[j-1]].ts > s.perms[a].ts {
			order[j] = order[j-1]
			j--
		}
		order[j] = a
		no++
	}
	for oi := 0; oi < no; oi++ {
		a := order[oi]
		ap := &s.perms[a]
		if ap.flags&pfAttacking == 0 || ap.flags&pfBlocked == 0 || s.combatArm().divDone.has(a) {
			continue
		}
		if ap.nblock < 2 || !s.dealsNow(k, a, first) || s.kw(k, a)&kop.KwTrample != 0 {
			continue
		}
		pw := s.power(k, a)
		if k.gen != nil && k.gen.anyCdmgTough {
			pw = s.combatAmtTough(k, a)
		}
		if pw <= 0 {
			continue
		}
		var bl [MaxPerm]Slot
		nb := s.liveBlockers(a, &bl)
		if nb < 2 || splits(pw, nb) > 128 {
			continue
		}
		s.divA, s.divI, s.divRem = a, 0, uint8(pw)
		return true
	}
	return s.nextBlockerDivision(k, first, &order, no) // combat_extrablock.go
}

// divideOptions poses gorge's division decision (combat.go
// divisionOptions): one option per split of the attacker's power among its
// blockers in block order, every nonnegative split in lexicographic
// ascending order of (first blocker's share, second's, ...). An option is
// AkChoose{source: attacker, mode: split index}; SplitAt decodes it.
func (s *State) divideOptions(k *prep, d *Decision) {
	if s.perms[s.divA].flags&pfBlocking != 0 {
		s.blockerDivideOptions(d) // combat_extrablock.go
		return
	}
	var bl [MaxPerm]Slot
	nb := s.liveBlockers(s.divA, &bl)
	d.Kind, d.Sub, d.Player = DkChoose, ChDivide, s.active
	n := splits(int(s.divRem), nb)
	o := s.perms[s.divA].obj
	for i := 0; i < n; i++ {
		d.add(MakeAction(AkChoose, TCode(o), 0, uint8(i)))
	}
}

// SplitAt writes split number idx of p damage among n blockers
// (lexicographic ascending order) into out[:n].
func SplitAt(p, n, idx int, out []uint8) {
	for b := 0; b < n-1; b++ {
		for x := 0; x <= p; x++ {
			c := splits(p-x, n-1-b)
			if idx < c {
				out[b] = uint8(x)
				p -= x
				break
			}
			idx -= c
		}
	}
	out[n-1] = uint8(p)
}

func (s *State) applyDivide(k *prep, a Action) {
	if s.perms[s.divA].flags&pfBlocking != 0 {
		s.applyBlockerDivide(a) // combat_extrablock.go
		return
	}
	var bl [MaxPerm]Slot
	nb := s.liveBlockers(s.divA, &bl)
	var amt [MaxPerm]uint8
	SplitAt(int(s.divRem), nb, int(a.Mode()), amt[:nb])
	for i := 0; i < nb; i++ {
		*s.divAmtFor(bl[i], s.divA) = amt[i]
	}
	s.combatArm().divDone.set(s.divA)
	s.divA = NoSlot
}

// anyFirstStrike: an attacker or one of its live blockers has first strike
// or double strike, read when the combat damage step begins (gorge
// anyFirstStrike), so a first striker removed before damage does not make
// a first-strike step.
func (s *State) anyFirstStrike(k *prep) bool {
	const fs = kop.KwFirstStrike | kop.KwDoubleStrike
	for am := s.ctrlM[s.active]; am.any(); am = am.rest() {
		a := am.first()
		if s.perms[a].flags&pfAttacking == 0 {
			continue
		}
		if s.kw(k, a)&fs != 0 {
			return true
		}
		for bm := s.oppsM(s.active); bm.any(); bm = bm.rest() {
			b := bm.first()
			if s.blocks(b, a) && s.kw(k, b)&fs != 0 {
				return true
			}
		}
	}
	return false
}
