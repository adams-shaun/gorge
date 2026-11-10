package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// StallReplPlusBatch: a dealHits batch whose plus-N competition the
// one-parked-event op-resume shape cannot pose: two or more events match two
// or more pure plus-N replacements, or the competing event follows an earlier
// hit that would land first. Both are a declared capacity of the shape, not a
// lost event: the game stops before any of the batch lands.
const StallReplPlusBatch = 139

func init() { stallNames[StallReplPlusBatch] = "replplusbatch" }

// simultaneous-event batches on the stack frame —
// collect, snapshot, apply, fire. Cold, out of line: every arm here is
// reached from a site that already left the hot path (a paused combat
// pass, a multi-entrant resolution, a fight).

// replOrderFz is the frozen event of the paused pass from src to dst (nil:
// none).
func (s *State) replOrderFz(src, dst Slot) *fzEv {
	bd := s.roBody()
	for i := 0; i < int(s.g.ro.nfz); i++ {
		if fe := &bd.fz[i]; fe.src == src && fe.dst == dst {
			return fe
		}
	}
	return nil
}

// replOrderEnd closes a paused combat damage pass: the damage triggers
// fire (combatEventsGen), then each lifelink source gains once for all the
// damage it dealt in the pass, in the order each first landed damage (CR
// 702.15e with 119.9; gorge flushLifelinkBatch, which keeps the
// accumulation open across a parked replacement-order ask). Every frozen
// event holds what it dealt by now (replOrderCombat's walk writes it back),
// so the gains need no per-pass storage beyond the frozen events.
//
//go:noinline
func (s *State) replOrderEnd(k *prep, r *dmgRec) {
	ro := *s.roBody() // the batch's snapshot: the record is cleared before the triggers fire
	nfz := int(s.g.ro.nfz)
	s.g.ro.replHdr = replHdr{}
	*s.roBody() = replBody{}
	s.combatEventsGen(k, r, true)
	var src [MaxPerm]Slot
	var ctl [MaxPerm]uint8
	var amt [MaxPerm]int
	n := 0
	for j := 0; j < nfz; j++ {
		fe := &ro.fz[ro.ord[j]]
		if fe.n <= 0 || !fe.ll {
			continue
		}
		i := 0
		for i < n && (src[i] != fe.src || ctl[i] != fe.ctrl) {
			i++
		}
		if i == n {
			src[n], ctl[n], amt[n] = fe.src, fe.ctrl, 0
			n++
		}
		amt[i] += int(fe.n)
	}
	for i := 0; i < n; i++ {
		s.gainLife(k, ctl[i], amt[i])
	}
	if k.anyInit {
		s.initCombat(k, r) // CR 726.2 (act_initiative.go)
	}
}

// entBatch is one resolution's battlefield entrants (CR 603.6a, gorge
// the zone batch): enterHeld puts each onto the battlefield without
// its enters check, and fire runs the held checks in move order against the
// board after the last entrant, so each entrant's "whenever another ...
// enters" sees the whole batch.
type entBatch struct {
	sl [MaxPerm]Slot
	o  [MaxPerm]Obj
	n  int
}

// enterHeld is enter with the enters check held in b.
func (s *State) enterHeld(k *prep, b *entBatch, o Obj, ctrl uint8, tapped bool, from kop.Zone) Slot {
	sl := s.enterNF(k, o, ctrl, tapped, from, 0)
	if b.n < len(b.sl) {
		b.sl[b.n], b.o[b.n] = sl, o
		b.n++
	}
	return sl
}

// holdBatch hands b's entrants to genState.held across a suspension: fireHeld runs their
// checks with the rest once the last has entered.
func (s *State) holdBatch(b *entBatch) {
	for _, sl := range b.sl[:b.n] {
		s.g.held |= 1 << (s.perms[sl].ts & 7)
	}
}

// fireEntries runs b's held enters checks, in move order.
//
//go:noinline
func (s *State) fireEntries(k *prep, b *entBatch) {
	if !s.anyETB(k) {
		b.n = 0
		return
	}
	if k.gen != nil && k.gen.anyBatch {
		defer s.batchClose(s.batchOpen())
	}
	for i := 0; i < b.n; i++ {
		sl, o := b.sl[i], b.o[i]
		if s.occ.has(sl) && s.perms[sl].obj == o {
			s.fireObj(k, kop.TrigETB, sl, o, s.perms[sl].ctrl, 0)
		}
	}
	b.n = 0
}

// fireOrHoldEntries is fireEntries, except inside a "for each of them" loop's
// open frame (execEachTarget): the copies of the loop's other targets enter
// in the same zone batch (gorge effCopyPermanent, CR 603.6a), so these
// entrants' checks wait for the frame's end (genState.held, fireHeld).
func (s *State) fireOrHoldEntries(k *prep, b *entBatch) {
	if s.g.inLoop && s.g.batchLatch&batchHeld != 0 {
		s.holdBatch(b)
		b.n = 0
		return
	}
	s.fireEntries(k, b)
}

// createTokensHeld is OpCreateToken / OpCreateTokenEx for n ≥ 2 tokens:
// one Token resolution's creations enter together (gorge runTokenMints'
// zone batch, CR 603.6a), so their enters checks are held to the end. True
// when the board is full (a stall). ad are the cards of the ReplAddToken
// tokens made beside them, entering after the first token (they take the
// op's riders: gorge plans them as mints of the same Token effect).
//
//go:noinline
func (s *State) createTokensHeld(k *prep, in *kop.Instr, p uint8, n int, ad []CardID) bool {
	var eb entBatch
	for i := 0; i < n+len(ad); i++ {
		if s.nearCapacity(k) {
			s.fireEntries(k, &eb)
			s.stall(StallBoard)
			return true
		}
		c := CardID(in.C)
		if i >= 1 && i <= len(ad) {
			c = ad[i-1] // a ReplAddToken's token follows the first one (repl_tokorder.go)
		}
		o := s.newToken(c, p)
		tapped := in.B.N&kop.TrTapped != 0
		if in.Op == kop.OpCreateTokenEx {
			tapped = in.B.N&kop.TokTapped != 0
			s.g.tokMade++
			if in.B.N&kop.TokRememberObj != 0 {
				s.remember(o)
			}
			if in.B.N&kop.TokImprintObj != 0 {
				s.imprint(o) // zone_imprint.go
			}
		}
		sl := s.enterHeld(k, &eb, o, p, tapped, kop.ZoneLibrary)
		if in.Op == kop.OpCreateTokenEx && in.B.N&(kop.TokSacAtEnd|kop.TokExileAtEnd) != 0 {
			s.tokenAtEOT(in, o)
		}
		if in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokAttacking != 0 {
			s.tokenAttacks(k, in, sl, p) // filter_combatwords.go
		}
		if k.gen.anyTokCr {
			s.fireTokenCreated(k, sl)
		}
		if in.Op == kop.OpCreateToken && in.B.N&kop.TrHasteEOT != 0 {
			s.perms[sl].eotKw |= kop.KwHaste
			s.touchSl(sl)
			s.promoKw(k, sl, kop.KwHaste)
		}
		s.res.lastTok = o
	}
	s.fireEntries(k, &eb)
	return false
}

// hitBatch is one simultaneous damage event of a fight, bite or
// DamageSource$ effect: each hit names its source creature as a field (CR
// 701.14a, gorge fightRider: the fighter, not the resolving spell,
// is the source for replacement, prevention, lifelink and the damage
// triggers), its recipient and its amount, read before any hit lands.
type hitBatch struct {
	h [2]dmgHit
	n int
}

type dmgHit struct {
	src, dst Slot
	n        int16
	ctrl     uint8 // the source's controller as the hit is dealt
	ll       bool  // the source has lifelink as the hit is dealt
}

func (b *hitBatch) add(s *State, src, dst Slot, n int) {
	if b.n < len(b.h) {
		b.h[b.n] = dmgHit{src: src, dst: dst, n: int16(min(max(n, -1<<14), 1<<14))}
		b.n++
	}
}

// dealHits applies the hits hs in order, then fires each dealt hit's damage
// triggers in hit order (gorge emits one Damage event per hit, CR 120.8: a
// hit of 0 is none), then each lifelink source gains once for its hits (CR
// 702.15e: gorge's damage batch flushes the lifelink accumulation when it
// closes).
//
//go:noinline
func (s *State) dealHits(k *prep, hs []dmgHit) bool {
	return s.dealHitsLatched(k, hs, nil)
}

// dealHitsToOne is dealHits for hits on one recipient (ExtEachDamage): the only
// noncombat batch that lands on a referent twice, so a per-referent line
// latches as in a combat damage step, queued once at the first event with the
// batch's total to it (gorge trigger_match.go damageBatchKey).
//
//go:noinline
func (s *State) dealHitsToOne(k *prep, hs []dmgHit) bool {
	if k.gen == nil || !k.gen.anyDmgRef {
		return s.dealHits(k, hs)
	}
	var r dmgRec
	return s.dealHitsLatched(k, hs, &refLatch{r: &r})
}

// replPlusBatch parks a batch whose first hit's damage event is matched by two
// or more pure plus-N replacements (or, batchOrderAskable, an addition beside
// a prevention): gorge collects every matching damage
// replacement and asks the affected player their CR 616.1 order even though the
// additions commute, so the kernel poses the same no-op ask before the batch
// lands. The parked event's amount is discarded: the answer resumes the op
// (interp_ask.go), which re-executes the batch and deals every hit with the
// summed amount (trig_oneormore.go's exemption). The op-resume shape holds one
// parked event per op, so it parks only a batch whose first hit competes: two
// or more competing hits would need one ask per event, and a later competing
// hit would be parked before the earlier hits land. Both are a declared
// capacity (StallReplPlusBatch): the game stops before any hit lands rather
// than losing an earlier event. handled reports that the batch was consumed
// and must not deal.
func (s *State) replPlusBatch(k *prep, hs []dmgHit) (handled bool) {
	if !batchOrderAskable(k) || s.g.ro.n != 0 || s.g.ro.on || s.g.nc.n != 0 || s.g.plusBatchAsked {
		return false
	}
	// Probe parkNC for each hit with the ask state saved and restored: it is
	// the collector the answer will use, so the count and the parked event
	// cannot disagree with it.
	saveRO, saveNC := s.g.ro, s.g.nc
	n, at := 0, -1
	for i := range hs {
		h := &hs[i]
		if h.n <= 0 || !s.occ.has(h.src) || !s.occ.has(h.dst) {
			continue
		}
		if s.parkNC(k, s.perms[h.src].ctrl, h.src, s.cardAt(h.src), uint8(s.perms[h.dst].obj), int(h.n), s.kw(k, h.src)) {
			n, at = n+1, i
		}
		s.g.ro, s.g.nc = saveRO, saveNC
	}
	switch {
	case n >= 2:
		// gorge asks once per event; the shape parks one event per op.
		s.capFull(StallReplPlusBatch)
		return true
	case n == 1:
		// A hit before the competing one lands before gorge's ask (the batch
		// is dealt in hit order): parking the whole batch would hide it.
		for j := 0; j < at; j++ {
			if h := &hs[j]; h.n > 0 && s.occ.has(h.src) && s.occ.has(h.dst) {
				s.capFull(StallReplPlusBatch)
				return true
			}
		}
		h := &hs[at]
		s.parkNC(k, s.perms[h.src].ctrl, h.src, s.cardAt(h.src), uint8(s.perms[h.dst].obj), int(h.n), s.kw(k, h.src))
		s.g.plusBatchAsked = true
		return true
	}
	return false
}

// batchOrderAskable reports whether replPlusBatch may park a batch: a table
// whose only damage replacements are plus-N additions, or one carrying an
// addition beside other kinds when parkNC's probe (which runs replDamage for
// the parked amount) and the answer's re-execution read the same result
// without a side effect: no shield to spend, no Mindskinner mill, no
// noncombat damage tally. Every order then lands the same amount (a
// prevention beside an addition is 0 in either order, CR 616.1), and a
// non-commuting doubling still stalls loudly in replPlus.
func batchOrderAskable(k *prep) bool {
	g := k.gen
	if replPlusOnly(g) {
		return true
	}
	return g != nil && g.anyRepl&rbPlusDmg != 0 && g.anyRepl&(rbShield|rbPrevMill) == 0 && !k.anyRedNC
}

// dealHitsLatched is dealHits with rl, whose dmgRec it fills, as the batch's
// per-referent latch (nil: each hit is its own batch). It reports whether the
// batch was parked on the replacement-order ask (or stalled) instead of dealt,
// so a caller's post-batch work is skipped.
func (s *State) dealHitsLatched(k *prep, hs []dmgHit, rl *refLatch) bool {
	if s.replPlusBatch(k, hs) {
		return true
	}
	for i := range hs {
		h := &hs[i]
		h.n = int16(s.damageHit(k, h))
		if rl != nil && h.n > 0 {
			rl.r.ev[rl.r.nev] = dmgEv{src: h.src, dst: h.dst, n: h.n}
			rl.r.nev++
		}
	}
	// a triggered or lifelink follow-up can deal nested damage: the batch's
	// re-execution exemption ends with the hits themselves
	s.g.plusBatchAsked = false
	if k.gen != nil && k.gen.anyPermMark {
		for i := range hs {
			if h := &hs[i]; h.n > 0 {
				s.markDealt(k, h.src, false) // filter_permmarks.go
			}
		}
	}
	if k.gen != nil && s.subs[kop.TrigDamageDone].or(s.subs[kop.TrigDamageDealtOnce]).any() {
		at := uint8(0)
		// One source dealing the whole batch (a creature's sweep) is one
		// damage batch for its DamageDealtOnce lines: they queue once, at
		// the first hit, carrying the batch total (gorge trigger_match.go
		// damageBatchKey), as a combat damage step does.
		var lt dmgLatch
		var latch *dmgLatch
		if s.subs[kop.TrigDamageDealtOnce].any() && rl == nil {
			latch = &lt
			for i := range hs {
				if h := &hs[i]; h.n > 0 {
					if h.src != hs[0].src {
						latch = nil
						break
					}
					lt.total += h.n
				}
			}
		}
		for i := range hs {
			h := &hs[i]
			if h.n <= 0 {
				continue
			}
			at++
			if !s.occ.has(h.src) {
				continue
			}
			if rl != nil {
				rl.at = at
			}
			s.fireDamageEvGen(k, h.src, h.n, false, false, latch, uint8(s.perms[h.dst].obj), rl)
		}
	}
	for i := range hs {
		h := &hs[i]
		if h.n <= 0 || !h.ll {
			continue
		}
		tot := int(h.n)
		for j := i + 1; j < len(hs); j++ {
			if g := &hs[j]; g.src == h.src && g.ctrl == h.ctrl && g.ll {
				tot += int(g.n)
				g.ll = false
			}
		}
		s.gainLife(k, h.ctrl, tot)
	}
	return false
}

// damageHit deals hit h (damage, loyalty for a planeswalker, -1/-1
// counters for infect, deathtouch) and returns the amount dealt; h's
// controller and lifelink are read as it is dealt.
func (s *State) damageHit(k *prep, h *dmgHit) int {
	a, b, n := h.src, h.dst, int(h.n)
	if !s.occ.has(a) || !s.occ.has(b) {
		return 0
	}
	if k.gen != nil && k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevDmgTo|rbProtAll|rbProtCol|rbPlusDmg|rbShield|rbPrevCol|rbPrevMill|rbSelfDmg) != 0 && n > 0 {
		n = s.replDamage(k, s.perms[a].ctrl, a, s.cardAt(a), uint8(s.perms[b].obj), false, n)
	}
	if n <= 0 {
		return 0
	}
	akw := s.kw(k, a)
	h.ctrl, h.ll = s.perms[a].ctrl, akw&kop.KwLifelink != 0
	s.touchSl(b)
	if k.gen != nil && k.gen.dmgBy != nil {
		s.noteDmgBy(k, a, b) // DamagedBy marks (filter_combatwords.go)
	}
	p := &s.perms[b]
	if k.gen != nil && k.gen.anyInfect && (s.infectAt(k, a) || s.witherKw(k, a, akw)) && s.isCreature(k, b) {
		s.infectBetween(k, a, b, n, akw)
		return n
	}
	s.dmgPerm(k, b, p, n) // a planeswalker loses loyalty, a battle defense, a creature marks it
	if akw&kop.KwDeathtouch != 0 {
		p.markDT(1) // gorge marks any permanent deathtouch damages (effects/damage.go)
	}
	s.res.lastDmg = int16(n)
	return n
}
