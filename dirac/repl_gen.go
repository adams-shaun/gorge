package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Damage, token, counter and zone replacements of a generated table
// (Twinflame Tyrant, Crystal Barricade, Doubling Season, Progenitus) and
// the player hexproof static. They are found by walking the battlefield's
// static-carrying slots when an event happens, not cached in State: the
// walk sees control changes and ability loss (an override that removes
// abilities, genLost) as they are, two copies apply twice (x4), and a
// table without these cards (genPrep.anyRepl == 0) never walks.
//
// The computation order when several apply to one damage event: amount
// changes first (doubling), then prevention. CR 616.1 lets the affected
// player or controller choose the order, and gorge asks it even when every
// order gives the same result: parkNC (noncombat) and the combat pass pose
// that ask, and an event they cannot park stalls.

// replN counts the permanents controlled by p whose card has replacement
// bit b and still has its abilities.
func (s *State) replN(k *prep, p uint8, b uint32) int {
	return s.replNIn(k, s.statM.and(s.ctrlM[p]), b)
}

// replNIn is replN over the permanents of m.
func (s *State) replNIn(k *prep, m slotMask, b uint32) int {
	g := k.gen
	n := 0
	for ; m.any(); m = m.rest() {
		sl := m.first()
		if g.replBits[s.cardAt(sl)]&b != 0 && !s.genLost(k, sl) {
			n++
		}
	}
	return n
}

// replDamage is the amount of damage a source controlled by srcCtrl deals
// to the recipient code (an Obj on the battlefield or PlayerTarget*) after
// the replacements: doubled once per Twinflame Tyrant its controller
// controls when the recipient is that player's opponent or a permanent the
// opponent controls, then prevented (0) when it is noncombat damage to a
// creature whose controller controls another Crystal Barricade.
func (s *State) replDamage(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, code uint8, combat bool, n int) int {
	g := k.gen
	if n <= 0 || srcCtrl > 1 {
		return n
	}
	rcp := uint8(255) // the recipient's controller
	sl := NoSlot
	if isPlayerTarget(code) {
		rcp = CodeSeat(code)
	} else if int(code) < MaxObj && s.objs[code].zone == kop.ZoneBattlefield {
		sl = s.objs[code].slot
		rcp = s.perms[sl].ctrl
	} else {
		return n
	}
	if g.anyRepl&rbPlProt != 0 && sl == NoSlot && s.plProtected(k, srcCtrl, rcp, combat) {
		return 0 // protection from each opponent prevents it first (CR 702.16e; static_playerstats.go)
	}
	if sl == NoSlot && s.plProtAll(k, rcp) && !s.noPrev(k, combat) {
		return 0 // protection from everything prevents it -- the player's only: a walker or battle they control still takes it (CR 702.16e; static_plprot.go)
	}
	if g.anyRepl&rbPrevCol != 0 && !combat {
		// Ext PreventNext's depleting shield (the combat events take it in replCombatGen)
		if m, ok := s.prevNextTake(k, srcCtrl, srcSl, srcCard, code, rcp, sl, false, n); ok {
			if k.anyRedNC && m > 0 {
				s.tallyRedNC(k, srcCtrl, srcSl, srcCard, m)
			}
			return m
		}
	}
	if g.anyRepl&rbPrevCol != 0 && s.prevColHit(k, srcSl, srcCard, prevTgt{rcp, sl}, combat) {
		return 0 // Ext PreventColor (cast_altfree.go)
	}
	if g.anyRepl&rbProtAll != 0 && sl != NoSlot && s.kw(k, sl)&kop.KwProtectionAll != 0 && !s.noPrev(k, combat) {
		return 0 // protection from everything prevents it first (CR 702.16e; gorge emit)
	}
	if g.anyRepl&rbProtCol != 0 && sl != NoSlot && s.protFrom(k, sl, srcSl, srcCard, s.srcColors(k, srcSl, srcCard), combat) {
		return 0 // protection from the source's colour, likewise
	}
	if sl != NoSlot && s.prevAnyTo(k, sl, combat) {
		return 0 // "Prevent all damage that would be dealt to CARDNAME." (combat_prevention.go)
	}
	if g.anyRepl&rbPrevMill != 0 && sl == NoSlot && rcp != srcCtrl && s.preventMill(k, srcCtrl, rcp, combat, n) {
		return 0 // The Mindskinner: prevented, the opponent mills that many (repl_preventmill.go)
	}
	if g.anyRepl&rbSelfDmg != 0 && sl != NoSlot {
		if n = s.selfDamage(k, sl, combat, n); n <= 0 {
			return 0 // Anti-Venom: prevented, that many +1/+1 counters (repl_selfdamage.go)
		}
	}
	if g.anyRepl&rbShield != 0 && !combat && sl != NoSlot && s.shieldN(sl) > 0 && !s.noPrev(k, combat) {
		n = s.shieldNC(k, srcCtrl, srcSl, srcCard, sl, n) // CR 122.1c (combat: shieldCombat)
		if k.anyRedNC && !combat && n > 0 {
			s.tallyRedNC(k, srcCtrl, srcSl, srcCard, n)
		}
		return n
	}
	if g.anyRepl&rbPlusDmg != 0 {
		n = s.replPlus(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, n)
	}
	if g.anyRepl&rbDblDmg != 0 && (rcp != srcCtrl || g.anyFiltRepl || g.anyDmgMark) {
		c := s.dblDmgN2(k, srcCtrl, srcSl, srcCard, rcp, sl, combat)
		if g.anyDmgMark {
			if m := s.dmgMarkN(rcp); m > 0 && s.dmgMarkDirect(k, m, c-m, rcp, sl, combat) {
				return 0 // the mark beside another replacement: gorge asks (repl_dmgmark.go)
			}
		}
		for ; c > 0 && n < 1<<12; c-- {
			n *= 2
		}
	}
	if g.anyRepl&rbPrevNC != 0 && !combat && sl != NoSlot && s.isCreature(k, sl) {
		for m := s.statM.and(s.ctrlM[rcp]); m.any(); m = m.rest() {
			b := m.first()
			if b != sl && g.replBits[s.cardAt(b)]&rbPrevNC != 0 && !s.genLost(k, b) && !s.noPrev(k, combat) {
				return 0
			}
		}
	}
	if k.anyRedNC && !combat && n > 0 {
		s.tallyRedNC(k, srcCtrl, srcSl, srcCard, n) // value_damagetally.go
	}
	return n
}

// replTokens is the number of tokens of card tok an effect creating n
// tokens under p's control creates (doubled once per Doubling Season p
// controls; the filtered and x3 multipliers: repl_multipliers.go).
func (s *State) replTokens(k *prep, p uint8, tok CardID, n int) int {
	if k.gen == nil || k.gen.anyRepl&rbDblTok == 0 || n <= 0 || p >= MaxSeats {
		return n
	}
	if k.gen.anyTokRew {
		// a mint site that makes the token it was given: a rewritten card
		// is not read there (repl_tokrewrite.go)
		tc, m, ok := s.replTokensRew(k, p, tok, n)
		if !ok || tc != tok {
			s.capFull(StallTokRewrite)
			return 0
		}
		return m
	}
	if k.gen.anyFiltRepl {
		return s.replTokensMult(k, p, tok, n)
	}
	for c := s.replNIn(k, s.tokSrcM(p), rbDblTok); c > 0 && n < 1<<8; c-- {
		n *= 2
	}
	return n
}

// replCounters is the number of counters of kind an effect putting n
// counters on the permanent at sl puts: Doubling Season's unfiltered
// doubling plus the filtered double/plus/minus replacements (repl_counters.go).
// A non-positive amount is zero (CR 107.1b: an effect's result cannot be
// negative), matching gorge's clamp at the effect entry before any recipient
// sweep or replacement (engine/effects/counters_put.go). The clamp is on the
// amount, not the resulting count, so a negative CounterNum$ never removes
// existing counters; replacements run only on a positive amount. Costs do not
// call it. +1/+1 is the widest count (16 bits; a placement past 0xffff is
// the declared StallCounters capacity, setCtrN), so the loop keeps doubling
// past 255. A doubling may overshoot the bound: setCtrN saturates the total,
// so every doubler still applies even when the current count is already
// large, and a result gorge stores past 0xffff stops the game as the
// declared counter capacity.
func (s *State) replCounters(k *prep, sl Slot, kind, n int) int {
	if n <= 0 {
		return 0
	}
	if k.gen == nil || k.gen.anyRepl&(rbDblCtr|rbCtrOn|rbDblCtrYou|rbPlusCtrEOT) == 0 {
		return n
	}
	return s.replCtrOn(k, sl, kind, n)
}

// playerHexproof: player p has hexproof (controls a permanent with
// StatPlayerHexproof).
func (s *State) playerHexproof(k *prep, p uint8) bool {
	if k.gen == nil || k.gen.anyRepl&rbPlHex == 0 {
		return false
	}
	if k.gen.anyPlHexRec && s.turnPlayerHex(k, p) {
		return true // a Pump's this-turn player grant (repl_turnrepl.go)
	}
	if k.gen.anyYouHex {
		return s.youHexproof(k, p) // ctr_shield.go
	}
	return s.replN(k, p, rbPlHex) > 0
}

// graveRedirect is the zone a card going to zone z really goes to: its
// owner's library (shuffled by the caller) for a ReplGraveToLibrary card
// headed for a graveyard.
func (s *State) graveRedirect(k *prep, c CardID, z kop.Zone) kop.Zone {
	if z == kop.ZoneGraveyard && k.gen != nil && k.gen.anyRepl&rbGraveLib != 0 && k.gen.replBits[c]&rbGraveLib != 0 {
		return kop.ZoneLibrary
	}
	return z
}

// replCombatGen applies protection from everything and the damage-amount
// replacements to each combat
// damage event of r (one per source and recipient, in assignment order),
// before preventCombatGen. Combat damage is never prevented by
// ReplPreventNoncombatToOthers.
func (s *State) replCombatGen(k *prep, r *dmgRec) {
	// a live Ext PreventNext shield depletes across the pass's events, so they
	// are dealt in gorge's assignment order (repl_prevnext.go)
	var ord [MaxPerm]uint8
	ordered := k.gen.anyRepl&rbPrevCol != 0 && !s.g.ro.on && s.pnxLive() && s.replOrderOrder(r, &ord) == r.nev
	for j := 0; j < r.nev; j++ {
		i := j
		if ordered {
			i = int(ord[j])
		}
		ev := &r.ev[i]
		if ev.n <= 0 {
			continue
		}
		src := &s.perms[ev.src]
		var code uint8
		if ev.dst == NoSlot {
			code = src.atk // an attacker's damage to the player it attacks
		} else {
			code = uint8(s.perms[ev.dst].obj)
		}
		if ordered {
			if m, ok := s.prevNextTake(k, src.ctrl, ev.src, s.cardAt(ev.src), code, s.recipientCtl(ev.dst, code), ev.dst, true, int(ev.n)); ok {
				if m == 0 {
					s.preventCombatEv(k, r, ev) // this event only: the shield's pool may cover others
					continue
				}
				s.combatEvLess(r, ev, code, int(ev.n)-m)
				continue
			}
		}
		if k.gen.anyRepl&rbPrevCol != 0 && s.prevColHit(k, ev.src, s.cardAt(ev.src), prevTgt{s.recipientCtl(ev.dst, code), ev.dst}, true) {
			s.preventCombatEv(k, r, ev) // Ext PreventColor: this source's event only
			continue
		}
		if k.gen.anyRepl&rbProtCol != 0 && ev.dst != NoSlot && s.protFrom(k, ev.dst, ev.src, s.cardAt(ev.src), s.colorsGen(k, ev.src), true) {
			// protection from the source's colour prevents this event only
			s.preventCombatEv(k, r, ev) // combat_prevention.go
			continue
		}
		if k.gen.anyRepl&rbHealSelf != 0 && s.selfDamageCombat(k, r, i) {
			return // a heal host's second event of an ordered-walk pass (repl_selfdamage.go)
		}
		n := s.replDamage(k, src.ctrl, ev.src, s.cardAt(ev.src), code, true, int(ev.n))
		if n == 0 && ev.dst != NoSlot && k.gen.anyRepl&rbPrevCtrSelf != 0 && s.selfDamageHost(k, ev.dst, rbPrevCtrSelf) {
			s.preventCombatEv(k, r, ev) // this event only: each puts its own counters (repl_selfdamage.go)
			continue
		}
		if n == 0 && ev.dst != NoSlot {
			r.prevent(ev.dst) // protection from everything: all of its damage
			continue
		}
		d := n - int(ev.n)
		if d == 0 {
			continue
		}
		ev.n = s.sat16(n)
		r.dealt[ev.src] = s.addSat16(r.dealt[ev.src], d)
		if ev.dst == NoSlot {
			r.toPl[CodeSeat(code)] = s.addSat16(r.toPl[CodeSeat(code)], d)
			r.dealtPl[ev.src] = s.addSat16(r.dealtPl[ev.src], d)
		} else {
			r.toPerm[ev.dst] = s.addSat16(r.toPerm[ev.dst], d)
		}
	}
}

// replOrd is gorge's CR 616.1 replacement-order ask on a combat damage
// event: when two or more damage replacements match one event (two
// Twinflame Tyrants, or one with Fleeting Flight's prevention on the
// recipient), gorge asks the affected player which applies next, re-asking
// while two or more remain, and deals the pass's events one at a time in
// its assignment order, so the events before the asked one are already
// dealt (gorge combat.go runCombatAssignments, replacement.go). Every order
// gives the same result, so the kernel poses the same no-op ask, the
// damage already computed by replCombatGen. The walk runs over the pass
// frozen at replOrderFreeze: the pass is recomputed on each resume (the
// blocked attackers' assignments frozen into divAmt first, so the damage
// already dealt cannot change them), and a recomputed pass would have the
// walk deal an attacker's re-trampled player event gorge does not (the events
// not yet dealt of a source in srcDone are dropped below).
type replOrd struct {
	replHdr // live across the noncombat parks too: resets clear it alone
}

// replHdr is the ask's header: the candidates, the answers so far and the
// asked event. Every reset (parkNC, the untap ask, the post-resolution ask,
// the answered ask) clears this alone.
type replHdr struct {
	// the asked combat event as it stood when asked (gorge's assignment is
	// fixed then: a source or recipient the state-based actions remove
	// before the answer still has it dealt): its amount, the source's
	// controller, lifelink / deathtouch, the player hit (dst NoSlot)
	an            int16 // before the doublings (gorge applies each pick as made)
	actl, apl, ac uint8 // ac: the candidates asked
	all, adt      bool
	on            bool   // a combat damage pass is paused: the events marked below are dealt
	fin           bool   // the asks for the event (askS, askD) are done: deal it
	n             uint8  // its candidates (0 = no ask in progress)
	used          uint8  // candidates applied so far (bit per cand)
	prev          uint8  // candidates that prevent the damage (bit per cand)
	plus          uint8  // candidates that are a plus-N addition already in the asked amount (bit per cand; the order walk)
	p             uint8  // the affected player, who answers
	cand          [4]Obj // the replacements' sources
	nca           bool   // a parked noncombat event's ask is posed (ncDmg)
	shld          uint8  // candidates that are the SHIELD counter's prevention (bit per cand; ctr_shield.go)
	pcol          uint8  // candidates that are an Ext PreventColor record's prevention (bit per cand; cast_altfree.go)
	pnx           uint8  // candidates that are an Ext PreventNext shield, cand its prevCol index (bit per cand; repl_prevnext.go)
	nfz           uint8  // the frozen events (fz)
	// the dealt events, by identity (an index into the order would shift
	// only across runs, and the walk is frozen, but the marks also gate the
	// events of a source the recomputed pass gives back): every event of a
	// source in srcDone, of the source cur those to a permanent in dstDone,
	// and the sources that hit the player in plDone (a recomputed pass
	// gives a trampler whose blockers died its whole power to the player:
	// no such event is dealt or triggers); a source's events are
	// consecutive in gorge's order
	cur, askS, askD Slot
}

// replBody is the combat damage pass's bulk: all-zero outside a pass (it is
// cleared by replOrderFreeze before it fills and by replOrderEnd; a game
// over mid-pass leaves it, and the next freeze clears it first).
type replBody struct {
	// the frozen pass: its events (fz, indexed like the r it was frozen
	// from) and their walk order (ord), before the replacements. A source's
	// events are consecutive in gorge's order
	fz      [MaxPerm]fzEv
	ord     [MaxPerm]uint8
	srcDone slotMask
	dstDone slotMask
	plDone  slotMask
}

// fzEv is one combat damage event of a pass frozen at replOrderFreeze, as
// it stood before the replacements: its source and recipient, the amount
// assigned, and the source's card, controller, the player hit (dst NoSlot)
// and lifelink / deathtouch, for when the source is gone by the time the
// walk reaches the event (gorge reads the same last known information).
type fzEv struct {
	src, dst Slot // dst NoSlot: the player the source attacks
	n        int16
	card     CardID
	ctrl     uint8 // the source's controller at the freeze
	apl      uint8 // the player hit (dst NoSlot)
	ll, dt   bool
}

// replOrderOrder writes r's event indices in gorge's assignment order:
// attackers in battlefield order, each one's events (blockers in block
// order, then the defender) followed by its blockers' events in block order.
func (s *State) replOrderOrder(r *dmgRec, ord *[MaxPerm]uint8) int {
	var atk [MaxPerm]Slot
	na := s.battlefieldOrder(s.active, s.occ.and(s.ctrlM[s.active]), atk[:], 0)
	n := 0
	var seen slotMask // by event index (r.ev holds at most MaxPerm)
	emit := func(src Slot) {
		for i := 0; i < r.nev; i++ {
			if r.ev[i].src == src && !seen.has(Slot(i)) {
				seen = seen.with(Slot(i))
				ord[n] = uint8(i)
				n++
			}
		}
	}
	for _, a := range atk[:na] {
		if s.perms[a].flags&pfAttacking == 0 {
			continue
		}
		emit(a)
		var bl [MaxPerm]Slot
		for _, b := range bl[:s.liveBlockers(a, &bl)] {
			emit(b)
		}
	}
	for i := 0; i < r.nev; i++ { // none expected
		if !seen.has(Slot(i)) {
			ord[n] = uint8(i)
			n++
		}
	}
	return n
}

// replPlusCand reports whether the permanent at b carries a ReplDamagePlus
// ability that matches n damage from fsl (or the spell's card srcCard when
// fsl is NoSlot) controlled by sc to the recipient (player rcp when dst is
// NoSlot, else the permanent at dst). add is a commuting addition's amount;
// raise is Ojer Axonil's power when its raise body matches, which the
// collectors must not treat as a commuting addition (its order changes the
// amount). It is the candidate collector's copy of replPlusHit.
func (s *State) replPlusCand(k *prep, b Slot, fsl Slot, srcCard CardID, sc, rcp uint8, dst Slot, combat bool, n int) (add, raise int, ok bool) {
	cd := &k.t.Cards[s.cardAt(b)]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDamagePlus {
			continue
		}
		if add, raise, hit := s.replPlusHit(k, b, a, sc, fsl, srcCard, rcp, dst, combat, n); hit {
			return add, raise, true
		}
	}
	return 0, 0, false
}

// replOrderCandsFz fills s.g.ro's candidates for the frozen combat event fe
// (its amount before the replacements is fe.n) and returns their number.
func (s *State) replOrderCandsFz(k *prep, fe *fzEv) int {
	g := k.gen
	ro := &s.g.ro
	fsl := fe.src
	sc, col := s.perms[fsl].ctrl, s.colorsGen(k, fsl)
	var rcp uint8
	if fe.dst == NoSlot {
		rcp = fe.apl
	} else {
		if s.kw(k, fe.dst)&kop.KwProtectionAll != 0 && !s.noPrev(k, true) || g.anyRepl&rbProtCol != 0 && s.protFrom(k, fe.dst, fsl, fe.card, col, true) {
			return 0 // prevented before the replacements (gorge emit)
		}
		rcp = s.perms[fe.dst].ctrl
	}
	n := 0
	ro.prev, ro.shld, ro.plus, ro.pnx = 0, 0, 0, 0
	if (rcp != sc || g.anyFiltRepl) && sc <= 1 {
		// gorge's scan order: the controller's battlefield in entry order.
		// A table whose only damage replacement is a plus-N addition competes
		// on commuting sums (replPlusOnly): collect it here so the order walk
		// poses gorge's no-op ask (trig_oneormore.go).
		plusOnly := replPlusOnly(g)
		var bf [MaxPerm]Slot
		for _, b := range bf[:s.battlefieldOrder(sc, s.statM.and(s.ctrlM[sc]), bf[:], 0)] {
			if g.replBits[s.cardAt(b)]&rbDblDmg != 0 && !s.genLost(k, b) && (!g.anyFiltRepl || s.dblDmgMatch(k, b, sc, fsl, fe.card, NoObj, rcp, fe.dst, true)) {
				if n == len(ro.cand) {
					s.capFull(StallReplCands) // loud: never truncate the candidates
					return 0
				}
				ro.cand[n] = s.perms[b].obj
				n++
			} else if plusOnly && g.replBits[s.cardAt(b)]&rbPlusDmg != 0 && !s.genLost(k, b) {
				if _, raise, ok := s.replPlusCand(k, b, fsl, fe.card, sc, rcp, fe.dst, true, int(fe.n)); ok && raise == 0 {
					if n == len(ro.cand) {
						s.capFull(StallReplCands) // loud: never truncate the candidates
						return 0
					}
					ro.cand[n] = s.perms[b].obj
					ro.plus |= 1 << n
					n++
				}
			}
		}
	}
	if fe.dst != NoSlot && s.prevCombatTo(k, fe.dst) {
		if n == len(ro.cand) {
			s.capFull(StallShield) // loud: never truncate the candidates
			return n
		}
		ro.cand[n] = s.perms[fe.dst].obj
		ro.prev |= 1 << n
		n++
	}
	if g.anyRepl&rbPrevCol != 0 {
		// Ext PreventColor's effect records (cast_altfree.go)
		ro.pcol = 0
		var cand [4]Obj
		var pm uint8
		if n = s.prevColCands(k, col, fsl, prevTgt{rcp, fe.dst}, true, &ro.cand, n, &cand, &pm); n < 0 {
			return 0
		}
		ro.prev |= pm
		ro.pcol = pm
	}
	// CR 122.1c: gorge appends the shield counter's prevention after the
	// printed and granted-keyword matches (replacement.go), so it is the
	// last candidate here too.
	if fe.dst != NoSlot && g.anyRepl&rbShield != 0 && s.shieldN(fe.dst) > 0 && !s.noPrev(k, true) {
		if n == len(ro.cand) {
			s.capFull(StallShield) // loud: never truncate the candidates
			return n
		}
		ro.cand[n] = s.perms[fe.dst].obj
		ro.prev |= 1 << n
		ro.shld |= 1 << n
		n++
	}
	if g.anyRepl&rbPrevCol != 0 {
		// Ext PreventNext's shields on the recipient (repl_prevnext.go)
		code := PlayerCode(rcp)
		if fe.dst != NoSlot {
			code = uint8(s.perms[fe.dst].obj)
		}
		ro.pnx = 0
		if n = s.prevNextCands(k, code, true, &ro.cand, n, &ro.pnx); n < 0 {
			return 0
		}
		ro.prev &^= ro.pnx
	}
	if g.anyDmgMark && s.dmgMarkCompetes(rcp, n) {
		return 0 // repl_dmgmark.go
	}
	ro.p = rcp
	return n
}

// replOrderFreeze starts a paused pass when an event of r (before the
// replacements) will be asked: the pass's events and their walk order are
// frozen (gorge's assignment is fixed then, and a recomputed pass loses the
// events of a creature the state-based actions at a pause remove, while it
// would give a trampler whose blockers died its whole power to the player),
// and the blocked attackers' assignments are frozen into divAmt / divDone,
// as a chosen division is.
func (s *State) replOrderFreeze(k *prep, r *dmgRec, first bool) {
	ro := &s.g.ro
	bd := s.roBody()
	*bd = replBody{} // the body is all-zero outside a pass; this is the only clear before it fills
	nfz := uint8(s.replOrderOrder(r, &bd.ord))
	for i := 0; i < r.nev; i++ {
		ev := r.ev[i]
		fe := &bd.fz[i]
		fe.src, fe.dst, fe.n = ev.src, ev.dst, ev.n
		fe.card = s.cardAt(ev.src)
		fe.ctrl = s.perms[ev.src].ctrl
		if ev.dst == NoSlot {
			fe.apl = CodeSeat(s.perms[ev.src].atk)
		}
		kw := s.kw(k, ev.src)
		fe.ll, fe.dt = kw&kop.KwLifelink != 0, kw&kop.KwDeathtouch != 0
	}
	ask := false
	for i := 0; i < int(nfz) && !ask; i++ {
		ask = s.replOrderCandsFz(k, &bd.fz[bd.ord[i]]) >= 2
	}
	if !ask {
		*bd = replBody{}
		return
	}
	var fz slotMask
	for am := s.occ.and(s.ctrlM[s.active]); am.any(); am = am.rest() {
		a := am.first()
		ap := &s.perms[a]
		if ap.flags&pfAttacking == 0 || ap.flags&pfBlocked == 0 || s.combatArm().divDone.has(a) || !s.dealsNow(k, a, first) {
			continue
		}
		var bl [MaxPerm]Slot
		for _, b := range bl[:s.liveBlockers(a, &bl)] {
			*s.divAmtFor(b, a) = 0
		}
		fz = fz.with(a)
		s.combatArm().divDone.set(a)
	}
	for i := 0; i < r.nev; i++ {
		if ev := r.ev[i]; fz.has(ev.src) && ev.dst != NoSlot && s.perms[ev.dst].flags&pfBlocking != 0 {
			*s.divAmtFor(ev.dst, ev.src) += uint8(ev.n)
		}
	}
	ro.replHdr = replHdr{on: true, nfz: nfz, cur: NoSlot}
}

// replOrderCombat deals the paused pass's frozen events not yet dealt in
// the pass's frozen order, posing the order ask before each event two or
// more replacements match; true when an ask is posed. Nobody receives
// priority at a pause, so the events dealt so far kill nothing before it is
// answered (CR 704.3), and every event's source and recipient are still there.
// The recomputed pass r is only the accounting: each event is dealt as it
// stands there. At the end of the pass the damage triggers fire as
// replCombatGen's caller would fire them.
func (s *State) replOrderCombat(k *prep, r *dmgRec) bool {
	ro := &s.g.ro
	bd := s.roBody()
	if ro.fin {
		ro.fin = false
		s.replOrderMark(ro.askS, ro.askD)
		s.dealAsked(k, r)
	}
	for j := 0; j < int(ro.nfz); j++ {
		fe := &bd.fz[bd.ord[j]]
		if bd.srcDone.has(fe.src) || fe.dst == NoSlot && bd.plDone.has(fe.src) || fe.src == ro.cur && fe.dst != NoSlot && bd.dstDone.has(fe.dst) {
			s.preventFrozenEv(k, r, fe) // dealt (or prevented): fe holds what it dealt
			continue
		}
		if c := s.replOrderCandsFz(k, fe); c >= 2 {
			ro.askS, ro.askD, ro.n, ro.used = fe.src, fe.dst, uint8(c), 0
			ro.an, ro.ac, ro.apl = fe.n, uint8(c), fe.apl
			if ro.plus != 0 {
				// the plus additions are commutative and were already applied by
				// replCombatGen (replPlus defers to this walk only for the ask)
				for i := 0; i < r.nev; i++ {
					if ev := r.ev[i]; ev.src == fe.src && ev.dst == fe.dst {
						ro.an = ev.n
						break
					}
				}
			}
			skw := s.kw(k, fe.src)
			ro.actl = s.perms[fe.src].ctrl
			ro.all, ro.adt = skw&kop.KwLifelink != 0, skw&kop.KwDeathtouch != 0
			s.g.ctlRet = ctlDivide
			s.ctl = ctlGenAsk
			return true
		}
		s.replOrderMark(fe.src, fe.dst)
		for i := 0; i < r.nev; i++ {
			if r.ev[i].src == fe.src && r.ev[i].dst == fe.dst {
				s.dealCombatEvent(k, r, i, fe)
				break
			}
		}
	}
	for i := 0; i < r.nev; i++ {
		if ev := &r.ev[i]; ev.dst == NoSlot && !bd.plDone.has(ev.src) {
			r.dealt[ev.src] -= ev.n
			r.dealtPl[ev.src] -= ev.n
			ev.n = 0
		}
	}
	s.replOrderEnd(k, r)
	return false
}

// replOrderMark marks the event from src to dst dealt.
func (s *State) replOrderMark(src, dst Slot) {
	ro := &s.g.ro
	bd := s.roBody()
	if src != ro.cur {
		if ro.cur != NoSlot {
			bd.srcDone.set(ro.cur)
		}
		ro.cur, bd.dstDone = src, slotMask{}
	}
	if dst == NoSlot {
		bd.plDone.set(src)
	} else {
		bd.dstDone.set(dst)
	}
}

// replCandLive: asked candidate i of the combat event still matches it (a
// Twinflame Tyrant still on the battlefield under the source's controller
// with its abilities, a recipient still preventing its combat damage; the
// recipient a permanent still).
func (s *State) replCandLive(k *prep, i int) bool {
	ro := &s.g.ro
	if ro.pnx&(1<<i) != 0 {
		return s.g.prevCol[ro.cand[i]].col != 0 && (ro.askD == NoSlot || s.occ.has(ro.askD)) && !s.noPrev(k, true)
	}
	if ro.pcol&(1<<i) != 0 {
		return (ro.askD == NoSlot || s.occ.has(ro.askD)) && !s.noPrev(k, true) // an effect record: it lasts the turn
	}
	ob := &s.objs[ro.cand[i]]
	if ob.zone != kop.ZoneBattlefield || ro.askD != NoSlot && !s.occ.has(ro.askD) {
		return false // gone, or the recipient is (no permanent to match)
	}
	sl := ob.slot
	if ro.shld&(1<<i) != 0 {
		return s.occ.has(ro.askD) && s.shieldN(ro.askD) > 0 && !s.noPrev(k, true)
	}
	if ro.plus&(1<<i) != 0 {
		return s.perms[sl].ctrl == ro.actl && k.gen.replBits[s.cardAt(sl)]&rbPlusDmg != 0 && !s.genLost(k, sl)
	}
	if ro.prev&(1<<i) != 0 {
		return s.prevCombatTo(k, sl)
	}
	return s.perms[sl].ctrl == ro.actl && k.gen.replBits[s.cardAt(sl)]&rbDblDmg != 0 && !s.genLost(k, sl) &&
		(!k.gen.anyFiltRepl || s.dblDmgLive(k, sl))
}

// dealAsked deals the answered combat event as it was asked: to a
// recipient still on the battlefield (damage, loyalty, deathtouch), or
// the player's life loss. It returns the amount dealt; the source's
// lifelink is one gain per source at the end of the pass (CR 702.15e,
// replOrderGains).
func (s *State) dealAsked(k *prep, r *dmgRec) int {
	if fe := s.replOrderFz(s.g.ro.askS, s.g.ro.askD); fe != nil {
		fe.n = 0 // dealt now: what it gains is written below
	}
	ro := &s.g.ro
	n := int(ro.an)
	// CR 122.1c: the shield's prevention applies when it was picked, or
	// when no other prevention was picked first (then it is the last
	// remaining candidate and applies automatically). Another prevention
	// picked first stops the event, so the shield is not spent.
	if ro.shld != 0 {
		other := false
		for j := 0; j < int(ro.ac); j++ {
			if ro.prev&(1<<j) != 0 && ro.shld&(1<<j) == 0 &&
				(ro.used&(1<<j) != 0 || s.replCandLive(k, j)) {
				other = true
				break
			}
		}
		for i := 0; i < int(ro.ac); i++ {
			if ro.shld&(1<<i) == 0 {
				continue
			}
			if !s.replCandLive(k, i) {
				break
			}
			if ro.used&(1<<i) != 0 || !other {
				s.spendShield(ro.askD)
				n = 0
			}
			break
		}
	}
	// gorge (replacement_damage.go handleDamageReplacementChoice) applies
	// each picked replacement as it is picked and re-collects the rest
	// after the last pick: a candidate the state-based actions at a pause
	// removed applies only if it was picked
	for i := 0; i < int(ro.ac) && n > 0; i++ {
		if ro.plus&(1<<i) != 0 || ro.pnx&(1<<i) != 0 {
			continue // already in ro.an (replCombatGen applied the additions; a shield was spent as picked)
		}
		if ro.used&(1<<i) == 0 && !s.replCandLive(k, i) {
			continue
		}
		if ro.prev&(1<<i) != 0 {
			n = 0
		} else if n < 1<<12 {
			n *= 2
		}
	}
	if k.gen.anyPrevFrom && s.occ.has(ro.askS) && s.prevCombatFrom(k, ro.askS) {
		n = 0 // the source's prevention (preventFromGen)
	}
	if n <= 0 {
		// the event was prevented (the shield, another prevention): take it
		// out of the pass's accounting so its damage triggers do not fire
		for i := 0; i < r.nev; i++ {
			if e := &r.ev[i]; e.src == ro.askS && e.dst == ro.askD {
				s.preventCombatEv(k, r, e) // combat_prevention.go
				break
			}
		}
		return 0
	}
	if ro.pnx != 0 {
		// a shield prevented part of it as picked: the pass's accounting loses that
		for i := 0; i < r.nev; i++ {
			if e := &r.ev[i]; e.src == ro.askS && e.dst == ro.askD {
				code := PlayerCode(ro.apl)
				if ro.askD != NoSlot {
					code = uint8(s.perms[ro.askD].obj)
				}
				if d := int(e.n) - n; d > 0 {
					s.combatEvLess(r, e, code, d)
				}
				break
			}
		}
	}
	if ro.askD == NoSlot {
		s.noteCmdDamage(ro.askS, ro.apl, n)
		s.loseLife(k, ro.apl, n)
	} else if sl := ro.askD; s.occ.has(sl) {
		p := &s.perms[sl]
		s.touchSl(sl)
		s.dmgPerm(k, sl, p, n)
		if ro.adt {
			p.markDT(1) // a planeswalker too (gorge effects/damage.go, combat.go: any damaged permanent)
		}
	}
	if fe := s.replOrderFz(ro.askS, ro.askD); fe != nil {
		fe.n, fe.ll, fe.ctrl = int16(n), ro.all, ro.actl // a gone recipient's event still gains (as asked)
	}
	return n
}

// dealCombatEvent deals event i of r on its own: the damage (loyalty for a
// planeswalker), deathtouch and the life loss of a player hit. It returns
// the amount dealt; the source's lifelink is one gain per source at the end
// of the pass (CR 702.15e, gorge; replOrderGains).
func (s *State) dealCombatEvent(k *prep, r *dmgRec, i int, fe *fzEv) int {
	ev := r.ev[i]
	n := int(ev.n)
	fe.n = 0
	if n <= 0 {
		return 0
	}
	src := &s.perms[ev.src]
	skw := s.kw(k, ev.src)
	if s.shieldFrozen(k, src.ctrl, ev.src, s.cardAt(ev.src), ev.dst, n) {
		s.preventCombatEv(k, r, &r.ev[i]) // combat_prevention.go
		return 0
	}
	if k.gen.anyRepl&rbPrevCol != 0 {
		// a lone Ext PreventNext shield, spent as the event is dealt (repl_prevnext.go)
		code := src.atk // the player the source attacks
		if ev.dst != NoSlot {
			code = uint8(s.perms[ev.dst].obj)
		}
		if m, ok := s.prevNextTake(k, src.ctrl, ev.src, s.cardAt(ev.src), code, s.recipientCtl(ev.dst, code), ev.dst, true, n); ok {
			if m == 0 {
				s.preventCombatEv(k, r, &r.ev[i])
				return 0
			}
			s.combatEvLess(r, &r.ev[i], code, n-m)
			n = m
		}
	}
	if ev.dst == NoSlot {
		s.noteCmdDamage(ev.src, CodeSeat(src.atk), n)
		s.loseLife(k, CodeSeat(src.atk), n)
	} else {
		sl := ev.dst
		p := &s.perms[sl]
		s.touchSl(sl)
		s.dmgPerm(k, sl, p, n)
		if skw&kop.KwDeathtouch != 0 {
			p.markDT(1) // a planeswalker too (gorge effects/damage.go, combat.go: any damaged permanent)
		}
	}
	fe.n, fe.ll, fe.ctrl = int16(n), skw&kop.KwLifelink != 0, src.ctrl
	return n
}

// replOrderOptions poses the order ask: the affected player picks the
// replacement that applies next among those not yet applied.
func (s *State) replOrderOptions(d *Decision) {
	ro := &s.g.ro
	d.Kind, d.Sub, d.Player = DkChoose, ChGenReplOrder, ro.p
	for i := 0; i < int(ro.n); i++ {
		if ro.used&(1<<i) == 0 {
			o := ro.cand[i]
			if ro.pnx&(1<<i) != 0 {
				o = s.g.prevCol[o].obj // a shield's option names its source
			}
			d.add(MakeAction(AkChoose, uint8(o), 0, uint8(i))) // mode: gorge's scan index
		}
	}
}

// replOrderPick records the pick; true when the ask is posed again (a
// doubling picked, two or more replacements left).
func (s *State) replOrderPick(a Action) bool {
	ro := &s.g.ro
	if i := a.Mode(); i < ro.n {
		ro.used |= 1 << i
		if ro.pnx&(1<<i) != 0 {
			return s.prevNextPick(int(i))
		}
		if ro.prev&(1<<i) != 0 {
			if !ro.on {
				s.replNCPickShield(int(i))
			}
			return false
		}
		left := 0
		for j := 0; j < int(ro.n); j++ {
			// gorge re-collects the rest after each pick: in combat a
			// candidate removed at a pause no longer counts
			if ro.used&(1<<j) == 0 && (!ro.on || s.replCandLive(s.k(), j)) {
				left++
			}
		}
		if left < 2 && !ro.on {
			s.replNCAutoShield()
		}
		return left >= 2
	}
	return false
}

// applyReplOrder applies the pick: a prevention ends the event's asks, a
// doubling asks again while two or more remain; then the pass resumes.
func (s *State) applyReplOrder(a Action) {
	ro := &s.g.ro
	if s.replOrderPick(a) {
		return
	}
	if !ro.on {
		s.prevNextNC()
		ro.replHdr = replHdr{} // a noncombat event: deal it, then the state-based actions
		s.dealNC(s.k())
		s.ctl = s.g.ctlRet
		return
	}
	ro.n, ro.fin = 0, true
	s.ctl = ctlDivide // the damage step continues (divA = NoSlot)
}

// ncDmg is a noncombat damage event parked on gorge's replacement-order
// ask (replacement.go: two or more damage replacements match, so the event
// is held as a Note and the competition queued): the resolution pauses
// after the op (raGenReplOrder; the spell stays on the stack), the answer
// deals the damage and the resolution runs on. One event per op: a second
// one the same op would ask is dealt at once (a deviation, not seen). An
// event parked outside a program step (no raGenReplOrder pause) is asked
// once the resolution is done, before priority (stepGen, ro.nca).
type ncDmg struct {
	code  uint8 // the recipient (an Obj or PlayerTarget*); n == 0: none parked
	src   Obj   // the source: a permanent's object, or the resolving spell's (spell)
	ctrl  uint8
	ll    bool // lifelink, deathtouch: the source's, when the event happened
	dt    bool
	spell bool
	n     int16 // the damage after the replacements (every order gives it)
}

// damageSeqStart freezes a multi-event damage op's recipients in gorge's
// sweep order (effDamageAll: seat 0's battlefield in entry order, then seat
// 1's, then the players; players last when players is set).
func (s *State) damageSeqStart(k *prep, hit slotMask, players bool) {
	s.g.dmgSeqOn, s.g.dmgSeqN, s.g.dmgSeqI, s.g.dmgSeqPl = true, 0, 0, 0
	for p := uint8(0); p < s.nSeats(); p++ {
		s.g.dmgSeqN = uint8(s.battlefieldOrder(p, hit.and(s.ctrlM[p]), s.resArm().dmgSeqOrd[:], int(s.g.dmgSeqN)))
	}
	if players {
		if MaxSeats == 2 {
			s.g.dmgSeqPl = 3
		} else {
			s.g.dmgSeqPl = seatBits(s.aliveFrom(0))
		}
	}
}

// At more than two seats dmgSeqPl is a seat mask (bit q: seat q is still to
// be dealt damage), dealt in turn order from the resolving controller, the
// order gorge's ValidPlayers$ lists them (definedSpec: AliveFrom the
// controller). At two seats it keeps its active/other bits below.

// seatBits is the seat mask of q.
func seatBits(q seatSeq) uint8 {
	var m uint8
	for i := uint8(0); i < q.n; i++ {
		m |= 1 << q.s[i]
	}
	return m
}

// dmgSeqCtrlFirst orders the sequence's two players controller first, as
// gorge's ValidPlayers$ Player lists them (AliveFrom the controller).
func (s *State) dmgSeqCtrlFirst(ctrl uint8) {
	if MaxSeats != 2 {
		return // the seat mask is always dealt from the controller
	}
	if ctrl != s.active {
		s.g.dmgSeqPl |= 4
	}
}

// damageSeqNext deals a multi-event damage op's remaining events in the op's
// order (the recipients frozen when it started, then the players). An event
// two replacements match is parked by damageTo (parkNC): the op stops there
// and resumes from the next recipient after the answer, so every matching
// event poses its own order ask while the spell is still resolving.
func (s *State) damageSeqNext(k *prep, n int) {
	for {
		if s.g.dmgSeqI < s.g.dmgSeqN {
			sl := s.resArm().dmgSeqOrd[s.g.dmgSeqI]
			s.g.dmgSeqI++
			s.damageTo(k, uint8(s.perms[sl].obj), n)
			if s.g.nc.n != 0 {
				return // parked: resume after the answer
			}
			continue
		}
		if MaxSeats != 2 && s.g.dmgSeqPl != 0 {
			q := s.aliveFrom(s.res.e.ctrl)
			for i := uint8(0); i < q.n; i++ {
				if p := q.s[i]; s.g.dmgSeqPl&(1<<p) != 0 {
					s.g.dmgSeqPl &^= 1 << p
					s.damageTo(k, PlayerCode(p), n)
					break
				}
			}
			if s.g.nc.n != 0 {
				return
			}
			if s.g.dmgSeqPl&seatBits(q) != 0 {
				continue
			}
			s.g.dmgSeqPl = 0 // a seat that has left is dealt nothing
		}
		if pl := s.g.dmgSeqPl & 3; MaxSeats == 2 && pl != 0 {
			// bit 0 the active player, bit 1 the other; bit 2 deals the
			// other first (the controller first off its turn: dmgSeqCtrlFirst)
			b := uint8(1)
			if pl&1 == 0 || pl&2 != 0 && s.g.dmgSeqPl&4 != 0 {
				b = 2
			}
			s.g.dmgSeqPl &^= b
			p := s.active
			if b == 2 {
				p = NextSeat(s.active)
			}
			s.damageTo(k, PlayerCode(p), n)
			if s.g.nc.n != 0 {
				return
			}
			continue
		}
		s.g.dmgSeqOn, s.g.dmgSeqPl = false, 0
		return
	}
}

// parkNC parks the damage event of n damage from the resolving entry
// (controller ctrl, source permanent srcSl or card srcCard) to code when two
// or more replacements match it; false when the event is dealt now. Gorge's candidates are the
// matching replacements in its scan order (each player's battlefield in
// entry order, player 0 first, repl_zoneskip.go forEachReplacementSourceFor):
// a Twinflame Tyrant of the source's controller when the recipient is the
// opponent or the opponent's permanent, a Crystal Barricade of a creature
// recipient's controller other than the creature itself (noncombat only),
// and the recipient's SHIELD counter (CR 122.1c, appended last as gorge
// appends it after the printed matches). The recipient's own printed
// K:Prevent "all damage" line (Dawn Elemental) is one of those printed
// matches: it is collected in the scan too, so it is a candidate here before
// the shield. Protection prevents the damage before the replacements
// (gorge emit).
func (s *State) parkNC(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, code uint8, n int, kw kop.KwMask) bool {
	g := k.gen
	ro := &s.g.ro
	plusOnly := replPlusOnly(g)
	// parked: an earlier event of this op is parked on its ask (nc.n -1 is a
	// parked, prevented one), and ro holds that ask
	parked := s.g.nc.n != 0
	if !plusOnly && g.anyRepl&(rbDblDmg|rbPrevNC|rbShield|rbPrevCol|rbPrevDmgTo) == 0 || n <= 0 || ctrl >= MaxSeats || ro.on || ro.n > 0 && !parked {
		return false
	}
	rcp := uint8(255)
	sl := NoSlot
	if isPlayerTarget(code) {
		rcp = CodeSeat(code)
		if g.anyRepl&rbPlProt != 0 && s.plProtected(k, ctrl, rcp, false) {
			return false // prevented before the replacements (replDamage)
		}
		if s.plProtAll(k, rcp) && !s.noPrev(k, false) {
			return false // protection from everything prevents it before the replacements (replDamage)
		}
	} else if int(code) < MaxObj && s.objs[code].zone == kop.ZoneBattlefield {
		sl = s.objs[code].slot
		rcp = s.perms[sl].ctrl
		if g.anyRepl&rbProtAll != 0 && s.kw(k, sl)&kop.KwProtectionAll != 0 && !s.noPrev(k, false) ||
			g.anyRepl&rbProtCol != 0 && s.protFrom(k, sl, srcSl, srcCard, s.srcColors(k, srcSl, srcCard), false) {
			return false
		}
	} else {
		return false
	}
	crt := sl != NoSlot && s.isCreature(k, sl)
	c := 0
	plusAdd := 0
	var prev, shld, plus uint8
	var cand [4]Obj
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var bf [MaxPerm]Slot
		for _, b := range bf[:s.battlefieldOrder(p, s.statM.and(s.ctrlM[p]), bf[:], 0)] {
			rb := g.replBits[s.cardAt(b)]
			if s.genLost(k, b) {
				continue
			}
			if rb&rbDblDmg != 0 && p == ctrl && (rcp != ctrl && !g.anyFiltRepl || g.anyFiltRepl && s.dblDmgMatch(k, b, ctrl, srcSl, srcCard, NoObj, rcp, sl, false)) {
				if c == len(cand) {
					s.capFull(StallReplCands) // loud: never truncate the candidates
					return false
				}
				cand[c] = s.perms[b].obj
				c++
			} else if rb&rbPrevNC != 0 && crt && p == rcp && b != sl && !s.noPrev(k, false) {
				if c == len(cand) {
					s.capFull(StallReplCands) // loud: never truncate the candidates
					return false
				}
				cand[c] = s.perms[b].obj
				prev |= 1 << c
				c++
			} else if rb&rbPlusDmg != 0 && p == ctrl {
				// an addition (Fated Firepower, Hawkeye): gorge collects it
				// beside every other match, so two or more are asked as its
				// CR 616.1 order (replPlus); beside a prevention every order
				// gives 0, beside a doubling replDamage stalls loudly
				add, raise, ok := s.replPlusCand(k, b, srcSl, srcCard, ctrl, rcp, sl, false, n)
				if ok {
					if raise != 0 {
						// A raise (Ojer Axonil) competing with the additions
						// does not commute with them, so this sum-only park
						// would drop it. Leave the event to damageTo's
						// replDamage/replPlus, which caps StallReplPlus loudly.
						return false
					}
					if c == len(cand) {
						s.capFull(StallReplCands) // loud: never truncate the candidates
						return false
					}
					cand[c] = s.perms[b].obj
					plus |= 1 << c
					plusAdd += add
					c++
				}
			}
		}
	}
	// the recipient's own printed K:Prevent "all damage" line (replDamage's
	// prevAnyTo arm): a printed match, so it is offered before the shield
	// (gorge appends the shield after the printed matches)
	if sl != NoSlot && s.prevAnyTo(k, sl, false) {
		if c == len(cand) {
			s.capFull(StallReplCands) // loud: never truncate the candidates
			return false
		}
		cand[c] = s.perms[sl].obj
		prev |= 1 << c
		c++
	}
	var pcol uint8
	if g.anyRepl&rbPrevCol != 0 {
		// Ext PreventColor's effect records (cast_altfree.go)
		var unused [4]Obj
		if c = s.prevColCands(k, s.srcColors(k, srcSl, srcCard), srcSl, prevTgt{rcp, sl}, false, &cand, c, &unused, &pcol); c < 0 {
			return false
		}
		prev |= pcol
	}
	if sl != NoSlot && g.anyRepl&rbShield != 0 && s.shieldN(sl) > 0 && !s.noPrev(k, false) {
		if c == len(cand) {
			s.capFull(StallShield) // loud: never truncate the candidates
			return false
		}
		cand[c] = s.perms[sl].obj
		prev |= 1 << c
		shld |= 1 << c
		c++
	}
	var pnx uint8
	if g.anyRepl&rbPrevCol != 0 {
		// Ext PreventNext's shields on the recipient (repl_prevnext.go)
		if c = s.prevNextCands(k, code, false, &cand, c, &pnx); c < 0 {
			return false
		}
		prev &^= pnx
		if pnx != 0 && c >= 2 && s.prevNextCompetes(k, srcSl, srcCard, rcp, sl, false) {
			s.capFull(StallPrevNext)
			return false
		}
	}
	if g.anyDmgMark && s.dmgMarkCompetes(rcp, c) {
		return false // repl_dmgmark.go
	}
	if c < 2 {
		return false
	}
	if parked {
		// gorge asks this event's order after the parked one is answered and
		// dealt; one op parks one event, so a second competing event of the
		// same op is not modelled: loud, never dealt ahead of the first
		s.capFull(StallReplPlus)
		return false
	}
	// A shield candidate (or another prevention) stops the whole event, so
	// skip replDamage: it would spend the counter while parking and stall on
	// the very competition being asked. A pure-plus competition is a sum,
	// so its amount is the same in every order and is known here.
	m := 0
	if plusOnly {
		m = n + plusAdd
	} else if pnx != 0 {
		m = n // the shields are spent as picked (applyReplOrder sets what is left)
	} else if shld == 0 {
		m = s.replDamage(k, ctrl, srcSl, srcCard, code, false, n)
	}
	ro.replHdr = replHdr{n: uint8(c), prev: prev, plus: plus, shld: shld, pcol: pcol, pnx: pnx, p: rcp, cand: cand}
	if pnx != 0 {
		ro.an = int16(min(n, 1<<14))
	}
	nc := ncDmg{code: code, ctrl: ctrl, ll: kw&kop.KwLifelink != 0, dt: kw&kop.KwDeathtouch != 0, n: int16(m)}
	if srcSl != NoSlot {
		nc.src = s.perms[srcSl].obj
	} else {
		nc.src, nc.spell = s.res.e.obj, s.res.e.kind == seSpell
	}
	if m <= 0 {
		nc.n = -1 // prevented: the ask is still posed, nothing is dealt
	}
	s.g.nc = nc
	return true
}

// dealNC deals the parked noncombat damage after the ask is answered: the
// damage (loyalty for a planeswalker), deathtouch, a player's life loss, the
// source's lifelink gain and its damage triggers.
func (s *State) dealNC(k *prep) {
	nc := s.g.nc
	s.g.nc = ncDmg{}
	n := int(nc.n)
	if n <= 0 {
		return
	}
	code := nc.code
	if isPlayerOrNone(code) {
		s.loseLife(k, CodeSeat(code), n)
	} else {
		ob := &s.objs[code]
		if ob.zone != kop.ZoneBattlefield {
			return
		}
		sl := ob.slot
		s.touch(Obj(code))
		p := &s.perms[sl]
		s.dmgPerm(k, sl, p, n)
		if nc.dt {
			p.markDT(1) // a planeswalker too (gorge effects/damage.go, combat.go: any damaged permanent)
		}
	}
	if nc.ll {
		s.gainLife(k, nc.ctrl, n)
	}
	if k.gen.anyPermMark {
		if ob := &s.objs[nc.src]; !nc.spell && ob.zone == kop.ZoneBattlefield {
			s.markDealt(k, ob.slot, false) // filter_permmarks.go
		}
	}
	if s.subs[kop.TrigDamageDone].any() || s.subs[kop.TrigDamageDealtOnce].any() || k.gen != nil && k.gen.graveDmg {
		if ob := &s.objs[nc.src]; !nc.spell && ob.zone == kop.ZoneBattlefield {
			s.fireDamage(k, ob.slot, int16(n), isPlayerOrNone(code), false, code)
		} else if nc.spell || ob.zone != kop.ZoneBattlefield { // an ability whose source is gone too (pair 3334)
			s.fireDamageSrc(k, nc.src, nc.ctrl, int16(n), dmgPl(code), code, nil)
		}
	}
}

// protColKw are the protection-from-a-colour keywords.
const protColKw = kop.KwProtBlack | kop.KwProtRed

// protColFrom reports whether permanent sl has protection from one of the
// colours col (CR 702.16e: damage from such a source is prevented; gorge
// emit's protectedFrom, before the replacements).
func (s *State) protColFrom(k *prep, sl Slot, col kop.ColorMask) bool {
	kw := s.kw(k, sl)
	return kw&kop.KwProtBlack != 0 && col&kop.ColB != 0 || kw&kop.KwProtRed != 0 && col&kop.ColR != 0 ||
		k.gen.anyProtCol && s.protPrinted(k, sl)&col != 0
}

// srcColors is a damage source's colours: the permanent's when it is on
// the battlefield, else its card's printed colours (a spell, or a source
// that left).
func (s *State) srcColors(k *prep, sl Slot, c CardID) kop.ColorMask {
	if sl != NoSlot {
		return s.colorsGen(k, sl)
	}
	return k.t.Cards[c].Colors
}
