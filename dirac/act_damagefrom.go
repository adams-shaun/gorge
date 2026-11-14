package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugDmgSrcPlNoLifelink drops the lifelink gain of OpDealDamageFrom's
// player mode (a mutation: the dmgsrcpl lockstep must
// then diverge). Test-only; never set outside the oracle tests.
var DebugDmgSrcPlNoLifelink bool

// DebugDmgSrcPlLKI corrupts the last-known-information read of the player
// mode: 1 drops the snapshot's keywords, 2 flips its controller. Test-only.
var DebugDmgSrcPlLKI uint8

// dmgSrcPlayers is OpDealDamageFrom with B.N = 1:
// gorge DealDamage `Defined$ Opponent | DamageSource$ TriggeredCard`, the
// trigger object dealing in.A damage to each player in in.C. The source is
// the entry's trigger object: live while it is on the battlefield as the
// same object, else the entry's last known information (CR 113.7a, 608.2h;
// gorge newDamageRider). Lifelink, infect and the damage's controller are
// read once, a departed source's infect from the snapshot's lkiInfect bit
// (gorge lki.Infect); replacements, poison or life loss, lifelink and the
// damage triggers follow damageTo's order for the resolving source.
//
// damageTo's CR 616.1 replacement-order ask (parkNC) is posed only for a
// pure plus-N competition (replPlusSrcPass); any other event two or more
// replacements match stalls loudly (dmgSrcOrderStalls).
func (s *State) dmgSrcPlayers(k *prep, in *kop.Instr) {
	e := &s.res.e
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		s.g.plusBatchAsked = false // a re-executed pass whose source is gone still ends its exemption
		return
	}
	n := s.value(k, in.A)
	if DebugDmgAmountPlusOne {
		n++
	}
	if n <= 0 {
		s.g.plusBatchAsked = false
		return
	}
	ver := s.objs[o].ver
	if e.li != 0 {
		ver = s.lk(e.li).ver
	}
	sl := s.slotIfSame(o, ver)
	var (
		kw     kop.KwMask
		ctrl   uint8
		infect bool
	)
	switch {
	case sl != NoSlot:
		kw, ctrl = s.kw(k, sl), s.perms[sl].ctrl
		infect = s.infectAt(k, sl)
	case e.li != 0:
		l := s.lk(e.li)
		kw, ctrl, infect = l.kw, l.ctrl, l.bits&lkiInfect != 0
		switch DebugDmgSrcPlLKI {
		case 1:
			kw = 0
		case 2:
			ctrl = 1 - ctrl // seat-ok: a test-only mutation (DebugDmgSrcPlLKI)
		}
	default:
		kw, ctrl = k.cards[s.objs[o].card].kw, s.ctrlOfObj(o)
		infect = k.t.Cards[s.objs[o].card].Infect
	}
	card := s.objs[o].card
	if sli, ok := tgtSlot(kop.Ref(in.C)); ok {
		// the any-target form (Scourge of Valkas): the slot names a player,
		// handled below as each-opponent is, or a permanent
		code := s.tgtCode(sli)
		if code == NoTarget {
			s.g.plusBatchAsked = false
			return
		}
		if isObjTarget(code) {
			s.dmgSrcObject(k, Obj(code), o, sl, kw, ctrl, infect, int(n))
			return
		}
	}
	q := s.eachPlayers(k, kop.Ref(in.C))
	// A pure plus-N competition poses gorge's no-op CR 616.1 order ask before
	// the pass deals (repl_plus.go): the op parks and the answer
	// re-executes it, which deals the summed amount; two competing player
	// events (RefEachPlayer) and any noncommuting candidate keep the loud
	// stall. The flag clears with the pass's events, below and at the early
	// returns, so a later op of the same resolution never inherits it.
	var (
		plCodes [MaxSeats]TCode
		plAmts  [MaxSeats]int
	)
	// the first seat is coded even when the set is empty, as before (its
	// code then names no player and the pass sees it as such)
	plCodes[0], plAmts[0] = PlayerCode(q.at(0)), int(n)
	ne := 1
	for i := uint8(1); i < q.n; i++ {
		plCodes[i], plAmts[i] = PlayerCode(q.s[i]), int(n)
		ne++
	}
	if s.replPlusSrcPass(k, ctrl, sl, card, kw, plCodes[:ne], plAmts[:ne]) {
		return // parked: runProg pauses on nc.n; a multi-event pass stalls loud
	}
	if s.dmgSrcOrderStalls(k, ctrl, sl, card, kw, plCodes[:ne], plAmts[:ne]) {
		s.g.plusBatchAsked = false
		return
	}
	for i := uint8(0); i < q.n; i++ {
		p := q.s[i]
		code := PlayerCode(p)
		m := n
		if k.gen != nil && k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevDmgTo|rbProtAll|rbProtCol|rbPlusDmg|rbShield|rbPrevCol|rbPrevMill) != 0 {
			if m = s.replDamage(k, ctrl, sl, card, code, false, m); m <= 0 {
				continue
			}
		}
		s.notePlDamagedNC(k, code) // value_oppsdamaged.go
		if k.gen != nil && k.gen.anyInfect && infect {
			s.addPoison(p, m)
		} else {
			s.loseLife(k, p, m)
		}
		s.res.lastDmg = int16(m)
		if kw&kop.KwLifelink != 0 && !DebugDmgSrcPlNoLifelink {
			s.gainLife(k, ctrl, m)
		}
		if k.gen != nil && k.gen.anyPermMark && sl != NoSlot {
			s.markDealt(k, sl, false)
		}
		if s.subs[kop.TrigDamageDone].any() || k.gen != nil && (s.subs[kop.TrigDamageDealtOnce].any() || k.gen.graveDmg) {
			if sl != NoSlot {
				s.fireDamage(k, sl, int16(m), true, false, code)
			} else if k.gen != nil {
				s.fireDamageSrc(k, o, ctrl, int16(m), dmgPl(code), code, nil)
			}
		}
	}
	s.g.plusBatchAsked = false // the pass's re-execution exemption ends with its events
}

// DebugDmgAmountPlusOne adds 1 to every DealDamageFrom / Bite amount
// (a mutation: the owntgt lockstep must then diverge).
// Test-only; never set outside the oracle tests.
var DebugDmgAmountPlusOne bool

// dmgSrcObject is dmgSrcPlayers' permanent recipient: the trigger object o
// deals n damage to the permanent dst (CR 120.3). A source still on the
// battlefield deals through the ordinary hit batch (replacements, infect,
// deathtouch, lifelink and the damage triggers as any creature's damage).
// A source that left is dealt from its last known information (CR 113.7a,
// 608.2h), kw/ctrl/infect read by the caller, with the same order as
// damageTo for the resolving source.
func (s *State) dmgSrcObject(k *prep, dst, o Obj, sl Slot, kw kop.KwMask, ctrl uint8, infect bool, n int) {
	if s.objs[dst].zone != kop.ZoneBattlefield {
		s.g.plusBatchAsked = false
		return
	}
	dsl := s.objs[dst].slot
	if sl != NoSlot {
		var hb hitBatch
		hb.add(s, sl, dsl, n)
		s.dealHits(k, hb.h[:hb.n]) // clears plusBatchAsked itself unless the batch parked
		return
	}
	card := s.objs[o].card
	code := TCode(dst)
	if s.replPlusSrcPass(k, ctrl, NoSlot, card, kw, []TCode{code}, []int{n}) {
		return
	}
	if s.dmgSrcOrderStalls(k, ctrl, NoSlot, card, kw, []TCode{code}, []int{n}) {
		s.g.plusBatchAsked = false
		return
	}
	m := n
	if k.gen != nil && k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevDmgTo|rbProtAll|rbProtCol|rbPlusDmg|rbShield|rbPrevCol|rbPrevMill|rbSelfDmg) != 0 {
		m = s.replDamage(k, ctrl, NoSlot, card, code, false, m)
	}
	s.g.plusBatchAsked = false
	if m <= 0 {
		return
	}
	s.touchSl(dsl)
	if k.gen != nil && k.gen.anyInfect && infect && s.isCreature(k, dsl) {
		s.dmgCountersIf(k, dsl, m)
		s.infectCounters(k, dsl, m)
	} else {
		s.dmgPerm(k, dsl, &s.perms[dsl], m)
	}
	if kw&kop.KwDeathtouch != 0 {
		s.perms[dsl].markDT(1)
	}
	s.res.lastDmg = int16(m)
	if kw&kop.KwLifelink != 0 {
		s.gainLife(k, ctrl, m)
	}
	if k.gen != nil && (s.subs[kop.TrigDamageDone].any() || s.subs[kop.TrigDamageDealtOnce].any()) {
		s.fireDamageSrc(k, o, ctrl, int16(m), 0, code, nil)
	}
}

// dmgSrcOrderStalls reports, stalling loudly, that an event of this
// DealDamageFrom pass is one gorge parks on its CR 616.1 order ask (gorge
// newDamageRider deals through the ordinary replacement collection; parkNC
// is that collector): the pass deals straight through replDamage, so it
// never poses the ask. A pure plus-N competition is replPlusSrcPass's, which
// parks it. parkNC is probed with the ask state saved and restored; it can
// change nothing else before it reports the park, and a park stops the game.
func (s *State) dmgSrcOrderStalls(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, kw kop.KwMask, codes []TCode, amts []int) bool {
	if k.gen == nil || replPlusOnly(k.gen) {
		return false
	}
	saveRO, saveNC := s.g.ro, s.g.nc
	for i := range codes {
		asks := amts[i] > 0 && s.parkNC(k, ctrl, srcSl, srcCard, codes[i], amts[i], kw)
		s.g.ro, s.g.nc = saveRO, saveNC
		if asks {
			s.capFull(StallReplPlus)
			return true
		}
	}
	return false
}
