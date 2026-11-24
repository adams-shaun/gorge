package dirac

import (
	"math/bits"

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

// selfDmgStep is replDamage's self-damage line (rbSelfDmg: Anti-Venom,
// a Phantom, Istvan): the amount after it and whether it prevented it all.
// A Phantom's line that gorge orders beside other replacements waits for the
// ask (selfPhWaits).
func (s *State) selfDmgStep(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, sl Slot, combat bool, n int) (int, bool) {
	if k.gen.anyRepl&rbSelfDmg == 0 || sl == NoSlot || s.selfPhWaits(k, srcCtrl, srcSl, srcCard, sl, combat, n) {
		return n, false
	}
	n = s.selfDamageAll(k, sl, srcCtrl, srcSl, srcCard, combat, n)
	return n, n <= 0
}

// replDamage is the amount of damage a source controlled by srcCtrl deals
// to the recipient code (an Obj on the battlefield or PlayerTarget*) after
// the replacements: doubled once per Twinflame Tyrant its controller
// controls when the recipient is that player's opponent or a permanent the
// opponent controls, then prevented (0) when it is noncombat damage to a
// creature whose controller controls another Crystal Barricade.
func (s *State) replDamage(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, code TCode, combat bool, n int) int {
	g := k.gen
	if n <= 0 || srcCtrl >= MaxSeats {
		return n // no seat controls the source
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
	if sl == NoSlot && s.plProtPrevent(k, srcCtrl, rcp, combat) {
		return 0 // protection from each opponent, or from everything, prevents it first (CR 702.16e; static_playerstats.go, static_plprot.go)
	}
	if g.anyRepl&rbPrevCol != 0 && !combat {
		if s.statRdrStall(k, srcCtrl, srcSl, srcCard, code, false) {
			return 0 // a static redirect beside a shield: gorge asks the order (repl_staticredirect.go)
		}
		// Ext PreventNext's depleting shield (the combat events take it in replCombatGen)
		if m, ok := s.prevNextTake(k, srcCtrl, srcSl, srcCard, code, rcp, sl, false, n); ok {
			if k.anyRedNC && m > 0 {
				s.tallyRedNC(k, srcCtrl, srcSl, srcCard, m)
			}
			return m
		}
	}
	s.prevRiderApply(k, srcCtrl, srcSl, srcCard, code, rcp, sl, combat, n) // repl_prevx.go
	if g.anyRepl&rbPrevCol != 0 && s.prevColHit(k, srcSl, srcCard, prevTgt{rcp, sl}, combat) {
		if s.statRdrStall(k, srcCtrl, srcSl, srcCard, code, combat) {
			return 0 // a static redirect beside the prevention: gorge asks the order (repl_staticredirect.go)
		}
		if g.anyRedir {
			s.redirBeside(code, rcp) // a redirect is a candidate too: gorge asks the order (repl_dmgredirect.go)
		}
		return 0 // Ext PreventColor (cast_altfree.go)
	}
	if g.anyRepl&rbProtAll != 0 && sl != NoSlot && s.kw(k, sl)&kop.KwProtectionAll != 0 && !s.noPrev(k, combat) {
		return 0 // protection from everything prevents it first (CR 702.16e; gorge emit)
	}
	if g.anyRepl&rbProtCol != 0 && sl != NoSlot && s.protFrom(k, sl, srcSl, srcCard, s.srcColors(k, srcSl, srcCard), combat) {
		return 0 // protection from the source's colour, likewise
	}
	if s.statRdrTake(k, srcCtrl, srcSl, srcCard, code, rcp, sl, combat, n) {
		return 0 // ReplDamageRedirect: the event's damage lands on its recipient instead (repl_staticredirect.go)
	}
	if g.anyRedir && !combat {
		// Ext DmgRedirect: dealt to the record's recipient instead (the combat events: replCombatGen)
		if m, ok := s.redirTake(k, srcCtrl, srcSl, srcCard, code, rcp, sl, n); ok {
			if k.anyRedNC && m > 0 {
				s.tallyRedNC(k, srcCtrl, srcSl, srcCard, m)
			}
			return m
		}
	}
	if sl != NoSlot && s.prevAnyTo(k, sl, combat) {
		return 0 // "Prevent all damage that would be dealt to CARDNAME." (combat_prevention.go)
	}
	if g.anyRepl&rbPrevMill != 0 && sl == NoSlot && rcp != srcCtrl && s.preventMill(k, srcCtrl, rcp, combat, n) {
		return 0 // The Mindskinner: prevented, the opponent mills that many (repl_preventmill.go)
	}
	var stopped bool
	if n, stopped = s.selfDmgStep(k, srcCtrl, srcSl, srcCard, sl, combat, n); stopped {
		return 0 // Anti-Venom: prevented, that many +1/+1 counters; a Phantom: one removed; Istvan, Guardian Naga: prevented (repl_preventtoself.go)
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
		n = s.statPrevNC(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, n) // repl_staticprevent.go
	}
	var minusHits, setHits, dblHits int
	if g.anyRepl2&rb2DmgMinus != 0 {
		n, minusHits = s.dmgMinusN(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, n) // repl_dmgminus.go
	}
	if g.anyRepl2&rb2DmgSet != 0 {
		n, setHits = s.dmgSetN(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, n) // repl_dmgminus.go
	}
	if g.anyRepl&rbDblDmg != 0 && (rcp != srcCtrl || g.anyFiltRepl || g.anyDmgMark) {
		c := s.dblDmgN2(k, srcCtrl, srcSl, srcCard, rcp, sl, combat)
		if g.anyDmgMark {
			if m := s.dmgMarkEv(rcp, srcCtrl); m > 0 && s.dmgMarkDirect(k, m, c-m, rcp, sl, combat) {
				return 0 // the mark beside another replacement: gorge asks (repl_dmgmark.go)
			}
		}
		dblHits = c
		for ; c > 0 && n < 1<<12; c-- {
			n *= 2
		}
	}
	s.dmgRewriteClashAt(k, minusHits, setHits, dblHits, combat, rcp, srcCtrl) // repl_dmgminus.go
	s.dmgRewritePrevClash(k, minusHits+setHits, srcSl, sl, combat)            // repl_dmgminus.go
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
// StatPlayerHexproof, or carries one on a runtime emblem).
func (s *State) playerHexproof(k *prep, p uint8) bool {
	if k.gen == nil {
		return false
	}
	if k.gen.anyPlHexRec && s.turnPlayerHex(k, p) {
		return true // a Pump's this-turn player grant (repl_turnrepl.go)
	}
	if k.gen.anyEmbPlHex && s.embPlHex(k, p) {
		return true // a Pump's until-next-turn player grant (static_emblemphex.go)
	}
	if k.gen.anyRepl&rbPlHex == 0 {
		return false
	}
	if k.gen.anyYouHex {
		return s.youHexproof(k, p) // ctr_shield.go
	}
	return s.replN(k, p, rbPlHex) > 0
}

// graveRedirect is the zone a card going to zone z really goes to: its
// owner's library for a ReplGraveToLibrary card headed for a graveyard from
// the zones the replacement reads (from is the move's origin; a
// battlefield-only line does not apply elsewhere).
func (s *State) graveRedirect(k *prep, c CardID, z kop.Zone, from kop.Zone) kop.Zone {
	if z != kop.ZoneGraveyard || k.gen == nil || k.gen.anyRepl&rbGraveLib == 0 || k.gen.replBits[c]&rbGraveLib == 0 {
		return z
	}
	if k.gen.exileAnyC != nil && k.gen.exileAnyC[c]&eaLibBattlefield != 0 && from != kop.ZoneBattlefield {
		return z
	}
	return kop.ZoneLibrary
}

// graveLibInsert reports how a ReplGraveToLibrary card is put into its
// owner's library: bottom of the library, and whether the owner shuffles
// after a top insert (Progenitus and the other reveal-and-shuffle cards).
func (s *State) graveLibInsert(k *prep, c CardID) (bottom, shuffle bool) {
	if k.gen == nil {
		return false, false
	}
	if k.gen.exileAnyC == nil {
		return false, false
	}
	b := k.gen.exileAnyC[c]
	return b&eaLibBottom != 0, b&eaLibShuffle != 0
}

// prevCtrSelfRecv reports whether an event of r is dealt to a live
// ReplPreventDamageCounters host. Such a host spends its counter on the first
// event it is dealt, so the pass's events must run in gorge's assignment
// order. Without one the order does not matter and the pass stays in slot
// order: an event a redirect appends mid-pass is then appended in that order
// instead of being indexed through the stale walk order.
func (s *State) prevCtrSelfRecv(k *prep, r *dmgRec) bool {
	if k.gen.anyRepl&rbPrevCtrSelf == 0 {
		return false
	}
	for i := 0; i < r.nev; i++ {
		ev := &r.ev[i]
		if ev.n > 0 && ev.dst != NoSlot && s.selfDamageHost(k, ev.dst, rbPrevCtrSelf) {
			return true
		}
	}
	return false
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
	nord := r.nev // the events before a redirect appends one mid-pass
	ordered := k.gen.anyRepl&rbPrevCol != 0 && !s.g.ro.on && s.pnxLive() && s.replOrderOrder(r, &ord) == nord
	// a prevent-and-counters self host (Undergrowth Champion) spends its counter
	// on the first event it is dealt, so the events to it run in that order too.
	// The host must be live: a card of the table that is not on the battlefield
	// arms rbPrevCtrSelf but needs no ordering, and the walk covers only the
	// events that existed before the loop (an appended event keeps its index).
	byOrd := ordered || !s.g.ro.on && s.prevCtrSelfRecv(k, r) && s.replOrderOrder(r, &ord) == nord
	for j := 0; j < r.nev; j++ {
		i := j
		if byOrd && j < nord {
			i = int(ord[j])
		}
		ev := &r.ev[i]
		if ev.n <= 0 {
			continue
		}
		if s.g.ro.on && s.frozenDealtSync(k, r, ev) {
			continue // the CR 616.1 walk already dealt this event (repl_pluswalk.go)
		}
		src := &s.perms[ev.src]
		var code TCode
		if ev.dst == NoSlot {
			code = src.atk // an attacker's damage to the player it attacks
		} else {
			code = TCode(s.perms[ev.dst].obj)
		}
		if k.gen.uses.has(useStatRdr) {
			// ReplDamageRedirect: the combat pass's static redirects (repl_staticredirect.go)
			if s.statRdrCombat(k, r, ev, byOrd, code, s.recipientCtl(ev.dst, code)) && ev.n <= 0 {
				continue
			}
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
		s.prevRiderApply(k, src.ctrl, ev.src, s.cardAt(ev.src), code, s.recipientCtl(ev.dst, code), ev.dst, true, int(ev.n)) // repl_prevx.go
		if k.gen.anyRepl&rbPrevCol != 0 && s.prevColHit(k, ev.src, s.cardAt(ev.src), prevTgt{s.recipientCtl(ev.dst, code), ev.dst}, true) {
			if k.gen.anyRedir && s.redirBeside(code, s.recipientCtl(ev.dst, code)) {
				return // a redirect is a candidate too: gorge asks the order (repl_dmgredirect.go)
			}
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
		if k.gen.anyRedir && s.redirCombat(k, r, ev, byOrd, code, s.recipientCtl(ev.dst, code)) && ev.n <= 0 {
			continue // Ext DmgRedirect: all of it dealt to the record's recipient as a new event
		}
		if s.spCombatEv(k, r, ev, src.ctrl, code) {
			continue // a static prevention took this event's damage (repl_staticprevent.go)
		}
		n := s.replDamage(k, src.ctrl, ev.src, s.cardAt(ev.src), code, true, int(ev.n))
		if n == 0 && ev.dst != NoSlot && k.gen.anyRepl&rbPrevSelf != 0 && s.selfDamageHost(k, ev.dst, rbPrevSelf) {
			s.preventCombatEv(k, r, ev) // this event only: each puts its own counters (repl_selfdamage.go)
			continue
		}
		if n == 0 && ev.dst != NoSlot {
			// this event only: protection from everything zeroes every event to
			// the recipient on its own, and a minus line (Lashknife Barrier)
			// zeroes just the small hit beside a larger one
			s.preventCombatEv(k, r, ev)
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
// replHdr.akw's bits.
const (
	akwLifelink uint8 = 1 << iota
	akwDeathtouch
)

// ro.plus's bits.
const (
	plRaiseShift = 4 // the raise bodies (Ojer Axonil) among the candidates, above the plus-N additions' low nibble: the noncombat walk applies one as picked while the amount is below its host's power (repl_pluswalk.go)
)

// plRaise lifts the plus-N additions' mask b into the raise bodies' nibble.
func plRaise(b uint8) uint8 { return b << plRaiseShift }

type replHdr struct {
	// the asked combat event as it stood when asked (gorge's assignment is
	// fixed then: a source or recipient the state-based actions remove
	// before the answer still has it dealt): its amount, the source's
	// controller, lifelink / deathtouch, the player hit (dst NoSlot)
	an            int16  // before the doublings (gorge applies each pick as made)
	actl, apl, ac uint8  // ac: the candidates asked
	akw           uint8  // the asked source's keywords: akwLifelink, akwDeathtouch (one byte: the header's size is pinned)
	on            bool   // a combat damage pass is paused: the events marked below are dealt
	fin           bool   // the asks for the event (askS, askD) are done: deal it
	n             uint8  // its candidates (0 = no ask in progress)
	used          uint8  // candidates applied so far (bit per cand)
	prev          uint8  // candidates that prevent the damage (bit per cand)
	plus          uint8  // the candidates of the order walk: the plus-N additions already in the asked amount, low nibble (bit per cand), the raise bodies (Ojer Axonil) above them, high nibble (plRaise): the noncombat walk applies a raise as picked, while the amount is below its host's power (repl_pluswalk.go)
	p             uint8  // the affected player, who answers
	cand          [4]Obj // the replacements' sources
	nca           bool   // a parked noncombat event's ask is posed (ncDmg)
	shld          uint8  // candidates that are the SHIELD counter's prevention (bit per cand; ctr_shield.go)
	pcol          uint8  // candidates that are an Ext PreventColor record's prevention (bit per cand; cast_altfree.go)
	pnx           uint8  // candidates that are an Ext PreventNext shield, cand its prevCol index (bit per cand; repl_prevnext.go)
	nfz           uint8  // the frozen events (fz)
	spv           uint8  // candidates that are a static damage prevention, cand its host's object (bit per cand; spOptionalAsk: a lone optional line; repl_staticprevent.go)
	// the dealt events, by identity (an index into the order would shift
	// only across runs, and the walk is frozen, but the marks also gate the
	// events of a source the recomputed pass gives back): every event of a
	// source in srcDone, of the source cur those to a permanent in dstDone,
	// and the sources that hit the player in plDone (a recomputed pass
	// gives a trampler whose blockers died its whole power to the player:
	// no such event is dealt or triggers); a source's events are
	// consecutive in gorge's order
	// cur is a Slot here; the entry-order ask keeps its entering object in it
	// (entObj), so it is an objWord
	cur        objWord
	askS, askD Slot
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

// appendPrevGrants appends one prevention candidate per live granted Effect
// record of sl covering the asked side(s) to ro.cand (from n), and returns
// the new count and how many records it appended. A full candidate list
// stalls loudly (never truncates) and returns -(count)-1.
func (s *State) appendPrevGrants(ro *replOrd, n int, k *prep, sl Slot, from, to bool, stall uint8) (int, int) {
	var gs [8]Obj
	m := s.prevGrantCands(k, sl, from, to, gs[:])
	for _, o := range gs[:m] {
		if n == len(ro.cand) {
			s.capFull(stall)
			return -n - 1, m
		}
		ro.cand[n] = o
		ro.prev |= 1 << n
		n++
	}
	return n, m
}

// replOrderCandsSeat appends the doubling and plus-N candidates on seat p's
// battlefield to ro.cand for the frozen combat event fe, in entry order, and
// returns the new count (or -1 when the list overflows, which has stalled).
// Every seat's battlefield is scanned because a plus-N line need not belong to
// the source's controller: the recipient's Tok-Tok, Volcano Born raises a red
// source's damage to a player beside the source controller's doubler.
func (s *State) replOrderCandsSeat(k *prep, fe *fzEv, p, sc, rcp uint8, n int) int {
	g := k.gen
	ro := &s.g.ro
	var bf [MaxPerm]Slot
	for _, b := range bf[:s.battlefieldOrder(p, s.statM.and(s.ctrlM[p]), bf[:], 0)] {
		if g.replBits[s.cardAt(b)]&rbDblDmg != 0 && !s.genLost(k, b) && s.dblDmgMatch(k, b, sc, fe.src, fe.card, NoObj, rcp, fe.dst, true) {
			if n == len(ro.cand) {
				s.capFull(StallReplCands) // loud: never truncate the candidates
				return -1
			}
			ro.cand[n] = s.perms[b].obj
			n++
		} else if g.replBits[s.cardAt(b)]&rbPlusDmg != 0 && !s.genLost(k, b) {
			if _, raise, ok := s.replPlusCand(k, b, fe.src, fe.card, sc, rcp, fe.dst, true, int(fe.n)); ok && raise == 0 {
				if n == len(ro.cand) {
					s.capFull(StallReplCands) // loud: never truncate the candidates
					return -1
				}
				ro.cand[n] = s.perms[b].obj
				ro.plus |= 1 << n
				n++
			}
		} else if g.anyRepl2&rb2DmgMinus != 0 && s.dmgMinusCand(k, b, fe.src, rcp, fe.dst) {
			// a minus line beside another minus line commutes (their sum,
			// floored) and replCombatGen already applied it: the walk poses
			// gorge's no-op ask, as for two plus-N additions (the plus bit
			// marks a candidate whose amount is already in ro.an)
			if n == len(ro.cand) {
				s.capFull(StallReplCands) // loud: never truncate the candidates
				return -1
			}
			ro.cand[n] = s.perms[b].obj
			ro.plus |= 1 << n
			n++
		}
	}
	return n
}

// minusBesidePlus reports whether the n collected candidates carry a minus line
// and a plus-N addition: the two do not commute on a small amount (n=1: +1 then
// -2 is 0, -2 then +1 is 1), and the kernel applies the addition first
// (replPlus), so the pair stalls rather than guess gorge's order.
func (s *State) minusBesidePlus(k *prep, n int) bool {
	ro := &s.g.ro
	plus, minus := false, false
	for i := 0; i < n; i++ {
		if ro.plus&(1<<i) == 0 {
			continue
		}
		if k.gen.replBits[s.cardAt(s.objs[ro.cand[i]].slot)]&rbPlusDmg != 0 {
			plus = true
		} else {
			minus = true
		}
	}
	return plus && minus
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
	ro.prev, ro.shld, ro.plus, ro.pnx, ro.spv = 0, 0, 0, 0, 0
	if (rcp != sc || g.anyFiltRepl) && sc < MaxSeats {
		// gorge's scan order: every living seat's battlefield in seat order,
		// each in entry order (forEachObject's AliveFrom(0)). The source's
		// controller is not the only one whose plus-N and doubling lines can
		// match: a red source's damage to a player is raised by the
		// recipient's Tok-Tok, Volcano Born as well as doubled by the source
		// controller's Absorbing Man and Titania.
		// A plus-N addition is collected beside every other kind: beside
		// another addition the orders commute to one sum (replPlusOnly), so
		// the walk poses gorge's no-op ask (trig_oneormore.go); beside a
		// doubling they do not, and the walk applies the picks as made
		// (repl_pluswalk.go).
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.out(p) {
				continue
			}
			if n = s.replOrderCandsSeat(k, fe, p, sc, rcp, n); n < 0 {
				return 0
			}
		}
		if g.anyRepl2&rb2DmgMinus != 0 && s.minusBesidePlus(k, n) {
			s.capFull(StallReplPlus) // a minus line and a plus-N addition do not commute
			return 0
		}
	}
	printed := false
	if fe.dst != NoSlot && s.prevCombatTo(k, fe.dst) {
		// one candidate per granted Effect record covering the To side (two
		// Mazes of Ith on one creature are two Effects in gorge, CR 616.1),
		// then the permanent's own candidate for a printed fact or a
		// pump/static-granted bit without a record (gorge scans its Effect
		// candidates before the cards' face replacements)
		var m int
		if n, m = s.appendPrevGrants(&s.g.ro, n, k, fe.dst, false, true, StallShield); n < 0 {
			return -n - 1
		}
		printed = (m == 0 && s.kwGen(k, fe.dst)&kop.KwPreventCombatDamageTo != 0) ||
			k.gen != nil && k.gen.replBits[s.cardAt(fe.dst)]&(rbPrevCombatTo|rbPrevDmgTo) != 0 && !s.genLost(k, fe.dst)
	}
	if g.anyRepl&rbPrevCol != 0 {
		// Ext PreventNext's shields on the recipient (repl_prevnext.go), the
		// ask's earlier options beside the printed prevent-all: gorge's scan
		// collects the effect records the shields are before the printed
		// lines, so they are collected here ahead of it (the noncombat twin
		// parkNC does the same)
		code := PlayerCode(rcp)
		if fe.dst != NoSlot {
			code = TCode(s.perms[fe.dst].obj)
		}
		ro.pnx = 0
		if n = s.prevNextCands(k, code, fsl, true, &ro.cand, n, &ro.pnx, ro.prev); n < 0 {
			return 0
		}
		ro.prev &^= ro.pnx
	}
	if printed {
		if n == len(ro.cand) {
			s.capFull(StallShield) // loud: never truncate the candidates
			return n
		}
		ro.cand[n] = s.perms[fe.dst].obj
		ro.prev |= 1 << n
		n++
	}
	if g.anyPrevFrom && s.prevFromGrant(k, fsl) {
		// the source's granted prevention (Maze of Ith's Effect, gorge's
		// bodyless Prevent$ ValidSource$ Card.IsRemembered line) is a
		// candidate of its own: an Effect, so after the printed and
		// keyword matches (preventFromGen removes the event once asked) —
		// one candidate per live Effect record; a pump-granted bit without
		// a record is one candidate naming the protected source
		var m int
		if n, m = s.appendPrevGrants(ro, n, k, fsl, true, false, StallReplCands); n < 0 {
			return -n - 1
		}
		if m == 0 {
			if n == len(ro.cand) {
				s.capFull(StallReplCands) // loud: never truncate the candidates
				return n
			}
			ro.cand[n] = s.perms[fsl].obj
			ro.prev |= 1 << n
			n++
		}
	}
	if g.anyRepl&rbPrevCol != 0 {
		// Ext PreventColor's effect records (cast_altfree.go)
		ro.pcol = 0
		var cand [4]Obj
		var pm uint8
		if n = s.prevColCands(k, col, fsl, fe.card, prevTgt{rcp, fe.dst}, true, &ro.cand, n, &cand, &pm); n < 0 {
			return 0
		}
		ro.prev |= pm
		ro.pcol = pm
	}
	// CR 122.1c: gorge appends the shield counter's prevention after the
	// printed and granted-keyword matches (replacement.go), so it is the
	// last candidate here too; beside a static prevention or a Phantom's line,
	// which are printed matches in the object scan, it follows them (below).
	if !spShieldLate(g) {
		n = s.replShieldCand(k, fe, n)
	}
	if g.uses.has(useStaticPrev) || selfPh(g) {
		if n = s.spCombatCands(k, fe, sc, rcp, n); n < 0 {
			return 0 // repl_staticprevent.go
		}
	}
	if spShieldLate(g) {
		n = s.replShieldCand(k, fe, n)
	}
	if g.anyDmgMark && s.dmgMarkCompetesEv(rcp, sc, n) {
		return 0 // repl_dmgmark.go
	}
	if s.prevRiderStall(k, n, fsl, fe.card, prevTgt{rcp, fe.dst}, true) {
		return 0
	}
	if bp := s.pnxBareMask(&ro.cand, ro.pnx); barePrevOnly(n, ro.prev|bp, ro.shld, ro.pnx&^bp, ro.plus) {
		return 1 // no order ask (repl_bareprev.go): replCombatGen prevented it
	}
	ro.p = rcp
	return n
}

// replShieldCand appends the SHIELD counter's prevention (CR 122.1c) as the
// last of the n candidates of frozen combat event fe; the new count.
func (s *State) replShieldCand(k *prep, fe *fzEv, n int) int {
	ro := &s.g.ro
	if fe.dst != NoSlot && k.gen.anyRepl&rbShield != 0 && s.shieldN(fe.dst) > 0 && !s.noPrev(k, true) {
		if n == len(ro.cand) {
			s.capFull(StallShield) // loud: never truncate the candidates
			return n
		}
		ro.cand[n] = s.perms[fe.dst].obj
		ro.prev |= 1 << n
		ro.shld |= 1 << n
		n++
	}
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
		c := s.replOrderCandsFz(k, &bd.fz[bd.ord[i]])
		ask = c >= 2 || c == 1 && ro.spv&1 != 0 // seat-ok: ro.spv is the walk's candidate bitmask; the lone Optional$ line's "may" (repl_staticprevent.go)
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
	ro.replHdr = replHdr{on: true, nfz: nfz, cur: objWord(NoSlot)}
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
		if bd.srcDone.has(fe.src) || fe.dst == NoSlot && bd.plDone.has(fe.src) || fe.src == Slot(ro.cur) && fe.dst != NoSlot && bd.dstDone.has(fe.dst) {
			s.preventFrozenEv(k, r, fe) // dealt (or prevented): fe holds what it dealt
			continue
		}
		if k.gen.uses.has(useStatRdr) && s.statRdrFrozen(k, fe) {
			// a static redirect matches this frozen event: the walk's candidates
			// do not carry it, so gorge's ask is unanswerable here — loud (repl_staticredirect.go)
			s.capFull(StallRedirOrder)
			return false
		}
		if c := s.replOrderCandsFz(k, fe); c >= 2 || c == 1 && ro.spv&1 != 0 { // seat-ok: ro.spv is the walk's candidate bitmask
			ro.askS, ro.askD, ro.n, ro.used = fe.src, fe.dst, uint8(c), 0
			ro.an, ro.ac, ro.apl = fe.n, uint8(c), fe.apl
			if ro.plus != 0 && !s.roPlusPend() {
				// the plus additions are commutative and were already applied by
				// replCombatGen (replPlus defers to this walk only for the ask);
				// beside a doubling the picks are applied as made (roPlusPend),
				// so the asked amount stays the event's own
				for i := 0; i < r.nev; i++ {
					if ev := r.ev[i]; ev.src == fe.src && ev.dst == fe.dst {
						ro.an = ev.n
						break
					}
				}
			}
			if c == 1 && ro.spv&1 != 0 { // seat-ok: ro.spv is the walk's candidate bitmask
				// the lone Optional$ line's "may" (CR 616.1g): its controller
				// answers, not the affected player (gorge replacementAskPlayer),
				// and the skip option (spOptions, mode ro.n) deals all of it
				ro.spv |= spOptionalAsk
				ro.p = s.perms[s.objs[ro.cand[0]].slot].ctrl
			}
			skw := s.kw(k, fe.src)
			ro.actl = s.perms[fe.src].ctrl
			ro.akw = 0
			if skw&kop.KwLifelink != 0 {
				ro.akw |= akwLifelink
			}
			if skw&kop.KwDeathtouch != 0 {
				ro.akw |= akwDeathtouch
			}
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
	if src != Slot(ro.cur) {
		if Slot(ro.cur) != NoSlot {
			bd.srcDone.set(Slot(ro.cur))
		}
		ro.cur, bd.dstDone = objWord(src), slotMask{}
	}
	if dst == NoSlot {
		bd.plDone.set(src)
	} else {
		bd.dstDone.set(dst)
	}
}

// replCandLive: asked candidate i of the combat event still matches it (a
// doubler or plus-N line still on the battlefield and still matching with its
// own controller — the recipient's Tok-Tok, Volcano Born beside the source
// controller's doublings, a recipient still preventing its combat damage; the
// recipient a permanent still).
func (s *State) replCandLive(k *prep, i int) bool {
	ro := &s.g.ro
	if ro.pnx&(1<<i) != 0 {
		return s.g.prevCol[ro.cand[i]].col != 0 && (ro.askD == NoSlot || s.occ.has(ro.askD)) && !s.noPrev(k, true)
	}
	if ro.spv&(1<<i) != 0 {
		return s.spCandLive(k, i) // a static prevention: repl_staticprevent.go
	}
	if ro.pcol&(1<<i) != 0 {
		return (ro.askD == NoSlot || s.occ.has(ro.askD)) && !s.noPrev(k, true) // an effect record: it lasts the turn
	}
	// a granted Effect record's candidate names the granting object, not the
	// permanent it protects: it is live while the record is (its protected
	// slot holds it, the grantor may fall), with the recipient still one
	if ro.prev&(1<<i) != 0 && ro.shld&(1<<i) == 0 && ro.plus&(1<<i) == 0 &&
		ro.cand[i] != s.perms[ro.askS].obj && (ro.askD == NoSlot || ro.cand[i] != s.perms[ro.askD].obj) &&
		(s.prevGrantNamed(ro.askS, ro.cand[i]) || ro.askD != NoSlot && s.prevGrantNamed(ro.askD, ro.cand[i])) {
		return !s.noPrev(k, true) && (ro.askD == NoSlot || s.occ.has(ro.askD))
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
		// a plus-N line need not belong to the source's controller (the
		// recipient's Tok-Tok, Volcano Born beside the source controller's
		// doublings), so the host re-matches with its own controller: the
		// same match roPlusAdd applies the pick with (repl_pluswalk.go), the
		// source read as the frozen event's (an LKI a pause removed)
		if k.gen.replBits[s.cardAt(sl)]&rbPlusDmg == 0 {
			// a minus line (the walk marks it with the plus bit too): it is
			// live while it still matches the event (replOrderCandsSeat)
			return k.gen.anyRepl2&rb2DmgMinus != 0 && s.dmgMinusCand(k, sl, ro.askS, ro.p, ro.askD)
		}
		if s.genLost(k, sl) {
			return false
		}
		card := s.cardAt(ro.askS)
		if fe := s.replOrderFz(ro.askS, ro.askD); fe != nil {
			card = fe.card
		}
		_, raise, ok := s.replPlusCand(k, sl, ro.askS, card, ro.actl, ro.p, ro.askD, true, int(ro.an))
		return ok && raise == 0 // the walk collects additions only (replOrderCandsSeat)
	}
	if ro.prev&(1<<i) != 0 {
		if sl == ro.askS {
			return s.prevFromGrant(k, sl) // the source's own granted prevention
		}
		return s.prevCombatTo(k, sl)
	}
	// a doubler re-matches too (dblDmgMatch reads its controller and filter,
	// CR 616.1): a candidate of another seat the widened scan collected that
	// no longer matches is not live either
	return k.gen.replBits[s.cardAt(sl)]&rbDblDmg != 0 && !s.genLost(k, sl) && s.dblDmgLive(k, sl)
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
			if (ro.prev&(1<<j) != 0 && ro.shld&(1<<j) == 0 || s.spFullAsk(k, j)) &&
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
				s.spendShield(k, ro.askD)
				n = 0
			}
			break
		}
	}
	// gorge (replacement_damage.go handleDamageReplacementChoice) applies
	// each picked replacement as it is picked and re-collects the rest
	// after the last pick: a candidate the state-based actions at a pause
	// removed applies only if it was picked. With a plus-N addition beside a
	// doubling (roPlusPend) the picks were applied as made (replOrderPick), so
	// only the last remaining candidate applies here; without one the
	// additions are already in ro.an and every live candidate applies in scan
	// order (they commute).
	pend := s.roPlusPend()
	for i := 0; i < int(ro.ac) && n > 0; i++ {
		if ro.pnx&(1<<i) != 0 {
			continue // the shield was spent as picked (prevNextPick)
		}
		if ro.spv&(1<<i) != 0 {
			continue // a static prevention was applied to ro.an as picked (spPick)
		}
		if pend && ro.prev&(1<<i) == 0 {
			if ro.used&(1<<i) != 0 || !s.replCandLive(k, i) {
				continue
			}
			if ro.plus&(1<<i) != 0 {
				n += s.roPlusAdd(k, i, n) // the last remaining addition
			} else if n < 1<<12 {
				n *= 2
			}
			continue
		}
		if ro.plus&(1<<i) != 0 {
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
	if pend {
		// the picked order dealt n; the recomputed pass applied the additions
		// before the doublings (replPlus): align its copy of the event
		s.syncAskedEv(k, r, n)
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
					code = TCode(s.perms[ro.askD].obj)
				}
				if d := int(e.n) - n; d > 0 {
					s.combatEvLess(r, e, code, d)
				}
				break
			}
		}
	}
	if ro.spv != 0 {
		// the static preventions were applied to ro.an as picked (spPick),
		// while the recomputed pass applied every candidate in scan order
		// (replCombatGen's spApply). The two can differ -- an all-but shield
		// beside a prevent-N shield does not commute -- so align this event
		// with the picked order's amount. The mirror arm grows it back when
		// the scan order fully prevented an event the picked order dealt.
		s.syncAskedEv(k, r, n)
	}
	if ro.askD == NoSlot {
		s.noteCmdDamage(ro.askS, ro.apl, n)
		s.noteDmgTaken(ro.apl, n)  // kw_bloodthirst.go
		s.notePlDamaged(k, ro.apl) // value_oppsdamaged.go
		s.loseLifeDmg(k, ro.apl, n)
		s.plDmgMark(ro.apl, n) // the redirect's damage (gorge events.foldDamage at the new recipient)
	} else if sl := ro.askD; s.occ.has(sl) {
		p := &s.perms[sl]
		s.touchSl(sl)
		s.dmgPerm(k, sl, p, n)
		if ro.akw&akwDeathtouch != 0 {
			p.markDT(1) // a planeswalker too (gorge effects/damage.go, combat.go: any damaged permanent)
		}
	}
	if fe := s.replOrderFz(ro.askS, ro.askD); fe != nil {
		fe.n, fe.ll, fe.ctrl = int16(n), ro.akw&akwLifelink != 0, ro.actl // a gone recipient's event still gains (as asked)
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
	if s.selfPhDeal(k, src.ctrl, ev.src, ev.dst, n) {
		s.preventCombatEv(k, r, &r.ev[i]) // the Phantom's line on an unasked event (repl_selfph.go)
		return 0
	}
	if k.gen.anyRepl&rbPrevCol != 0 {
		// a lone Ext PreventNext shield, spent as the event is dealt (repl_prevnext.go)
		code := src.atk // the player the source attacks
		if ev.dst != NoSlot {
			code = TCode(s.perms[ev.dst].obj)
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
		s.noteDmgTaken(CodeSeat(src.atk), n)  // kw_bloodthirst.go
		s.notePlDamaged(k, CodeSeat(src.atk)) // value_oppsdamaged.go
		s.loseLifeDmg(k, CodeSeat(src.atk), n)
		s.plDmgMark(CodeSeat(src.atk), n) // the redirect's damage (gorge events.foldDamage at the new recipient)
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
// replacement that applies next among those not yet applied. The pend ask's
// options are the ones gorge re-collects at this pick (a raise past its
// host's power is out, ncPlusLive).
func (s *State) replOrderOptions(d *Decision) {
	ro := &s.g.ro
	d.Kind, d.Sub, d.Player = DkChoose, ChGenReplOrder, ro.p
	pend := !ro.on && s.g.nc.n == ncPend
	for i := 0; i < int(ro.n); i++ {
		if ro.used&(1<<i) == 0 && !(pend && !s.ncPlusLive(i)) {
			o := ro.cand[i]
			if ro.pnx&(1<<i) != 0 {
				o = s.g.prevCol[o].obj // a shield's option names its source
			}
			// a granted combat-damage prevention's candidates name the
			// granting object already (replOrderCandsFz: one per Effect
			// record, CR 616.1); the printed or pump-granted one without a
			// record names the permanent it protects, as gorge's does
			d.add(MakeAction(AkChoose, TCode(o), 0, uint8(i))) // mode: gorge's scan index
		}
	}
	s.spOptions(d) // an optional static prevention's skip (repl_staticprevent.go)
}

// replOrderPick records the pick; true when the ask is posed again (a
// doubling picked, two or more replacements left). With a plus-N addition
// beside a doubling (roPlusPend) the picked addition or doubling applies as
// it is picked (gorge handleDamageReplacementChoice, CR 616.1e): the two do
// not commute, so the walk carries the running amount in ro.an. The parked
// noncombat event's mixed competition is walked the same way (ncPend,
// ncPlusPickNC), and its record's amount is written at the ask's end.
func (s *State) replOrderPick(a Action) bool {
	ro := &s.g.ro
	if i := a.Mode(); i < ro.n {
		ro.used |= 1 << i
		if ro.spv&(1<<i) != 0 {
			return s.spPick(int(i)) // repl_staticprevent.go
		}
		if ro.pnx&(1<<i) != 0 {
			return s.prevNextPick(int(i))
		}
		if ro.on {
			if s.roPlusPend() && ro.prev&(1<<i) == 0 {
				if ro.plus&(1<<i) != 0 {
					ro.an = s.sat16(int(ro.an) + s.roPlusAdd(s.k(), int(i), int(ro.an)))
				} else if ro.an < 1<<12 {
					ro.an *= 2
				}
			}
		} else if s.g.nc.n == ncPend && ro.prev&(1<<i) == 0 {
			// the parked noncombat event's mixed competition: the picked
			// addition, raise or doubling applies as it is picked (gorge
			// handleDamageReplacementChoice, CR 616.1e)
			s.ncPlusPickNC(int(i))
		}
		if ro.prev&(1<<i) != 0 {
			if !ro.on {
				if ro.pnx != 0 {
					// the picked printed prevention is terminal: it prevents
					// the whole event, so the shields never apply and are
					// not spent (gorge handleDamageReplacementChoice)
					ro.an = 0
				}
				s.replNCPickShield(s.k(), int(i))
				if s.g.nc.n == ncPend {
					ro.an = 0 // a prevention picked in the pend ask leaves nothing (repl_selfph.go)
				}
			}
			return false
		}
		left := 0
		pend := !ro.on && s.g.nc.n == ncPend
		for j := 0; j < int(ro.n); j++ {
			// gorge re-collects the rest after each pick: in combat a
			// candidate removed at a pause no longer counts; in the pend ask
			// a raise past its host's power no longer matches (ncPlusLive)
			if ro.used&(1<<j) != 0 {
				continue
			}
			if ro.on {
				if s.replCandLive(s.k(), j) {
					left++
				}
			} else if !pend || s.ncPlusLive(j) {
				left++
			}
		}
		if left < 2 && ro.on && ro.spv != 0 {
			s.spApplyLeft(s.k())
		}
		if left < 2 && !ro.on {
			if !pend {
				s.replNCAutoShield(s.k()) // the pend ask's leftover shield is ncPlusLeftovers'
			}
			if s.g.nc.n == ncPend {
				// gorge re-collects the rest after the last pick: the last
				// remaining candidate applies automatically (a raise past
				// its host's power drops out), and the record's amount
				// becomes the picked order's
				s.ncPlusLeftovers()
				s.g.nc.n = s.g.ro.an
			}
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
		s.spNC()
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
	code  TCode // the recipient (an Obj or PlayerTarget*); n == 0: none parked
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
		if s.duel() {
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
	// the sweep's events are one damage batch for the DamageDealtOnce latch
	// (gorge effDamageAll's BeginDamageBatch; trig_dmgbatch.go); a resumed op
	// re-enters with the batch already open, so the parked ask spans it
	s.dmgBatchOpen()
	for {
		if s.g.dmgSeqI < s.g.dmgSeqN {
			sl := s.resArm().dmgSeqOrd[s.g.dmgSeqI]
			s.g.dmgSeqI++
			s.damageTo(k, TCode(s.perms[sl].obj), n)
			if s.g.nc.n != 0 {
				return // parked: resume after the answer
			}
			continue
		}
		if MaxSeats != 2 && s.g.dmgSeqPl != 0 {
			from := s.res.e.ctrl
			if s.g.dmgSeqPl&dmgSeqSeat0 != 0 {
				from = 0 // OppNonTriggeredTarget's seat order (damageOtherOpps)
			}
			q := s.aliveFrom(from)
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
		if pl := s.g.dmgSeqPl & 3; s.duel() && pl != 0 {
			// bit 0 the active player, bit 1 the other; bit 2 deals the
			// other first (the controller first off its turn: dmgSeqCtrlFirst)
			b := uint8(1)
			if pl&1 == 0 || pl&2 != 0 && s.g.dmgSeqPl&4 != 0 { // seat-ok: a damage-order position bit in the duel branch
				b = 2
			}
			s.g.dmgSeqPl &^= b
			p := s.active
			if b == 2 {
				p = s.nextSeat(s.active)
			}
			s.damageTo(k, PlayerCode(p), n)
			if s.g.nc.n != 0 {
				return
			}
			continue
		}
		s.g.dmgSeqOn, s.g.dmgSeqPl = false, 0
		s.dmgBatchClose() // the sweep is over: the batch's queued amounts are patched in (trig_dmgbatch.go)
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
// matches: it is collected in the scan too, so it is a candidate here after
// the PreventNext shields (gorge scans its effect records first) and before
// the counter. Beside the shields it is the one competitor the ask models
// (below); beside anything else the shields still stall. Protection prevents
// the damage before the replacements
// (gorge emit). A pure plus-N competition is parked with its sum (the same
// in every order); a mixed one — a plus-N addition or a raise beside a
// doubling — with the amount unknown (ncPend), the picks applied as made
// (repl_pluswalk.go).
func (s *State) parkNC(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, code TCode, n int, kw kop.KwMask) bool {
	if s.statRdrStall(k, ctrl, srcSl, srcCard, code, false) {
		return false // a static redirect beside another candidate is gorge's ask: stalled (repl_staticredirect.go)
	}
	g := k.gen
	if parked, handled := s.spParkEarly(k, ctrl, srcSl, srcCard, code, n, kw); handled {
		return parked // a static prevention's ask (repl_staticprevent.go)
	}
	ro := &s.g.ro
	plusOnly := replPlusOnly(g)
	// parked: an earlier event of this op is parked on its ask (nc.n -1 is a
	// parked, prevented one), and ro holds that ask
	parked := s.g.nc.n != 0
	if s.parkNCNoKinds(g, plusOnly) || n <= 0 || ctrl >= MaxSeats || ro.on || ro.n > 0 && !parked {
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
	var prev, shld, plus, rs uint8
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
			} else if rb&rbPlusDmg != 0 {
				// every seat's battlefield: a plus-N line need not belong to
				// the source's controller (the recipient's Tok-Tok, Volcano
				// Born, CR 616.1); parkPlusCand re-matches with the host's own
				// controller, so a line that does not match is not collected
				// an addition (Fated Firepower, Hawkeye) or a raise (Ojer
				// Axonil): gorge collects it beside every other match, so two
				// or more are asked as its CR 616.1 order (replPlus); beside a
				// prevention every order gives 0, beside a doubling the picks
				// are applied as made (parkNCPend). A raise is collected as a
				// candidate of its own kind: it does not commute with the
				// additions or the doublings either.
				nc, add := s.parkPlusCand(k, b, srcSl, srcCard, ctrl, rcp, sl, n, &cand, c, &plus, &rs)
				if nc < 0 {
					return false
				}
				c, plusAdd = nc, plusAdd+add
			}
		}
	}
	var pnx uint8
	if g.anyRepl&rbPrevCol != 0 {
		// Ext PreventNext's shields on the recipient (repl_prevnext.go), the
		// ask's earlier options: gorge's scan collects the effect records the
		// shields are (engine/rules applyReplacementsDispatch's active-effect
		// pass) before the printed lines, so the options name the shields
		// first
		if c = s.prevNextCands(k, code, srcSl, false, &cand, c, &pnx, prev); c < 0 {
			return false
		}
		prev &^= pnx
	}
	// the recipient's own printed K:Prevent "all damage" line (replDamage's
	// prevAnyTo arm): a printed match, so it is offered after the shields
	// (gorge scans the effect records before the printed lines)
	var pAny uint8
	if c2 := s.parkPrevAny(k, &cand, c, sl, &prev); c2 < 0 {
		return false
	} else if c2 > c {
		pAny, c = 1<<c, c2
	}
	var pcol uint8
	if g.anyRepl&rbPrevCol != 0 {
		// Ext PreventColor's effect records (cast_altfree.go)
		var unused [4]Obj
		if c = s.prevColCands(k, s.srcColors(k, srcSl, srcCard), srcSl, srcCard, prevTgt{rcp, sl}, false, &cand, c, &unused, &pcol); c < 0 {
			return false
		}
		prev |= pcol
	}
	if c = s.parkShieldAt(k, sl, c, &cand, &prev, &shld, false); c < 0 {
		return false
	}
	if s.pnxAskStalls(k, ctrl, srcSl, srcCard, rcp, sl, pnx, pAny, prev, shld, plus, rs, c) {
		return false
	}
	if g.anyDmgMark && s.dmgMarkCompetesEv(rcp, ctrl, c) {
		return false // repl_dmgmark.go
	}
	c, spv, done := s.parkNCStatic(k, ctrl, srcSl, srcCard, code, n, kw, c, prev|shld|pnx|pcol, &cand, &plus, &rs)
	if done != 0 {
		return done == 2
	}
	if c = s.parkShieldAt(k, sl, c, &cand, &prev, &shld, true); c < 0 {
		return false
	}
	if c < 2 {
		return false
	}
	if s.prevRiderStall(k, c, srcSl, srcCard, prevTgt{rcp, sl}, false) {
		return false
	}
	if bp := s.pnxBareMask(&cand, pnx); !parked && barePrevOnly(c, prev|bp, shld, pnx&^bp, plus) {
		return false // no order ask (repl_bareprev.go): replDamage prevents it
	}
	if parked {
		// gorge asks this event's order after the parked one is answered and
		// dealt; one op parks one event, so a second competing event of the
		// same op is not modelled: loud, never dealt ahead of the first
		s.capFull(StallReplPlus)
		return false
	}
	if s.parkNCPend(k, ctrl, srcSl, srcCard, code, rcp, sl, n, kw, plus, rs, prev, shld, pnx, spv, cand, c) {
		return true
	}
	if s.parkNCStaticStall(spv) {
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

// pnxAskStalls reports that a parked PreventNext shields' competition still
// stalls (parkNC): gorge poses the CR 616.1 ask over the shields and every
// other candidate, but the walk below models the shields beside the
// recipient's own bare printed prevention only (a pick of the printed line
// stops the event with the shields unspent, gorge
// handleDamageReplacementChoice's terminal arm). Every other competitor (a
// doubler, a plus-N line, the SHIELD counter, a colour record) still stalls,
// loudly and before any dealing.
func (s *State) pnxAskStalls(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, pnx, pAny, prev, shld, plus, rs uint8, c int) bool {
	if pnx == 0 || c < 2 || !s.prevNextCompetes(k, ctrl, srcSl, srcCard, rcp, sl, false, false) {
		return false
	}
	if pAny == 0 || prev != pAny || shld != 0 || plus != 0 || rs != 0 || c != bits.OnesCount8(pnx)+1 {
		s.capFull(StallPrevNext)
		return true
	}
	return false
}

// parkPlusCand appends the battlefield plus-N candidate at b (an addition or
// a raise, replPlusCand) to parkNC's list: the new count and the candidate's
// live addition, or -1 after a loud stall (the candidates overflow).
func (s *State) parkPlusCand(k *prep, b Slot, srcSl Slot, srcCard CardID, ctrl, rcp uint8, sl Slot, n int, cand *[4]Obj, c int, plus, rs *uint8) (int, int) {
	add, raise, ok := s.replPlusCand(k, b, srcSl, srcCard, ctrl, rcp, sl, false, n)
	if !ok {
		return c, 0
	}
	if c == len(cand) {
		s.capFull(StallReplCands) // loud: never truncate the candidates
		return -1, 0
	}
	cand[c] = s.perms[b].obj
	if raise != 0 {
		*rs |= 1 << c
	} else {
		*plus |= 1 << c
	}
	return c + 1, add
}

// parkPrevAny appends the recipient's own printed K:Prevent "all damage" line
// to parkNC's candidates when it has one (replDamage's prevAnyTo arm): the new
// count, or -1 after a loud stall (the candidates overflow).
func (s *State) parkPrevAny(k *prep, cand *[4]Obj, c int, sl Slot, prev *uint8) int {
	if sl == NoSlot || !s.prevAnyTo(k, sl, false) {
		return c
	}
	if c == len(cand) {
		s.capFull(StallReplCands) // loud: never truncate the candidates
		return -1
	}
	cand[c] = s.perms[sl].obj
	*prev |= 1 << c
	return c + 1
}

// dealNC deals the parked noncombat damage after the ask is answered: the
// damage (loyalty for a planeswalker), deathtouch, a player's life loss, the
// source's lifelink gain, the red noncombat damage tally (the parked event
// never reached replDamage, the funnel that takes it) and its damage triggers.
func (s *State) dealNC(k *prep) {
	nc := s.g.nc
	s.g.nc = ncDmg{}
	n := int(nc.n)
	if n <= 0 {
		return
	}
	code := nc.code
	if isPlayerOrNone(code) {
		s.noteDmgTaken(CodeSeat(code), n) // kw_bloodthirst.go
		s.notePlDamagedNC(k, code)        // value_oppsdamaged.go
		s.loseLifeDmg(k, CodeSeat(code), n)
		s.plDmgMark(CodeSeat(code), n) // wasDealtDamageThisTurn's seat arm (gen.go)
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
	if k.anyRedNC {
		// the tally reads the source as the hit lands (value_damagetally.go)
		var tsl Slot = NoSlot
		tc := CardID(0)
		if ob := &s.objs[nc.src]; nc.spell || ob.zone != kop.ZoneBattlefield {
			tc = ob.card
		} else {
			tsl = ob.slot
		}
		s.tallyRedNC(k, nc.ctrl, tsl, tc, n)
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

// plProtPrevent is replDamage's player-protection pair: protection from each
// opponent (static_playerstats.go), then protection from everything (the
// player's only: a walker or battle they control still takes it,
// static_plprot.go). Protection prevents before the replacements (CR 702.16e).
func (s *State) plProtPrevent(k *prep, srcCtrl, rcp uint8, combat bool) bool {
	g := k.gen
	return g.anyRepl&rbPlProt != 0 && s.plProtected(k, srcCtrl, rcp, combat) ||
		s.plProtAll(k, rcp) && !s.noPrev(k, combat)
}
