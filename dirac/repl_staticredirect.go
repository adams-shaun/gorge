package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ReplDamageRedirect (kop/staticredirect.go): a permanent's printed static
// "all damage that would be dealt to <recipient> is dealt to <X> instead"
// (Pariah, Empyrial Archangel, Veteran Bodyguard, Palisade Giant, Martyrs of
// Korlis, Weathered Bodyguards, Pariah's Shield, Protector of the Crown,
// Treacherous Link, Harsh Judgment). gorge (replacement_replace.go
// ReplaceEvent, VarName$ Affected) rewrites the held damage event's recipient
// in place while the original source stays the damage source: the held event
// is collected once, the candidates are applied to it, and it lands on the
// rewritten recipient as one event (lifelink, deathtouch and the damage
// triggers from the original source). No new damage event is emitted, so the
// replacements do not run again on the rewritten recipient: what matched the
// original recipient applies beside the rewrite (and that pair is gorge's
// CR 616.1 order ask), what matches only the rewritten recipient does not.
//
// The kernel keeps that shape: the arm sits before every other damage
// replacement in replDamage, parkNC and replCombatGen; a redirect that matches
// an event beside any other damage replacement (another redirect of the same
// kind, an Ext DmgRedirect record, a doubler, a prevention, a shield) is
// gorge's CR 616.1 order ask, so the kernel stalls StallRedirOrder rather than
// answer (the same policy the Ext records keep, repl_dmgredirect.go). A lone
// redirect applies without an ask (gorge asks only for two or more
// candidates): the damage is dealt to the rewritten recipient as its landing,
// no replacements re-run. A redirect to a player in a combat pass has no
// plumbed recipient there: loud stall.
//
// The redirected damage does not meet the rewritten recipient's protection:
// protection prevents before the replacements (CR 702.16e), on the original
// recipient.

// rdrMax bounds the matching redirects of one event; more stall.
const rdrMax = 4

// useStatRdr: the table has a printed static damage redirect. Like
// useStaticPrev it keeps replPlusOnly from reading the rbPlusDmg opener as a
// plus-N table (the opener opens the damage sites only).
var useStatRdr = newGenUse(tableHasStatRdr)

// tableHasStatRdr: some card carries a ReplDamageRedirect replacement.
func tableHasStatRdr(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if t.Cards[i].Abilities[j].Repl.Kind == kop.ReplDamageRedirect {
				return true
			}
		}
	}
	return false
}

// prepStatRdrCard is the gen scan's ReplDamageRedirect case (gen.go): the
// printed static damage redirects (useStatRdr gates the arms). rbPlusDmg
// opens every damage site as a damage replacement does, and no card carries
// the bit (selfDamageBit's opener, repl_preventtoself.go), so replPlus and
// the static preventions read the ability kinds and find nothing.
func prepStatRdrCard(g *genPrep, pc *pcard) {
	g.anyRepl |= rbPlusDmg
	pc.hasStat = true
}

// rdrCand is one matching ability: the host permanent and the ability.
type rdrCand struct {
	b  Slot
	ab *kop.Ability
}

// rdrEv is one damage event for the redirect match.
type rdrEv struct {
	srcCtrl uint8
	srcSl   Slot
	srcCard CardID
	code    TCode // the recipient: a permanent's object or a player target
	rcp     uint8
	sl      Slot
	combat  bool
}

// rdrEvOf is the event of damage n from the source to the recipient code; ok
// false when no seat controls the source or the recipient is neither a live
// battlefield object nor a player.
func (s *State) rdrEvOf(srcCtrl uint8, srcSl Slot, srcCard CardID, code TCode, combat bool) (rdrEv, bool) {
	if srcCtrl >= MaxSeats {
		return rdrEv{}, false
	}
	e := rdrEv{srcCtrl: srcCtrl, srcSl: srcSl, srcCard: srcCard, code: code, combat: combat}
	if isPlayerTarget(code) {
		e.rcp = CodeSeat(code)
		e.sl = NoSlot
	} else if int(code) < MaxObj && s.objs[code].zone == kop.ZoneBattlefield {
		e.sl = s.objs[code].slot
		e.rcp = s.perms[e.sl].ctrl
	} else {
		return rdrEv{}, false
	}
	return e, true
}

// statRdrMatch reports whether the static redirect a of the host at b matches
// the event e (gorge replacement_damage.go's match, replacement_match.go's
// gates).
func (s *State) statRdrMatch(k *prep, b Slot, a *kop.Ability, e *rdrEv) bool {
	r := &a.Repl
	c := r.C
	hc := s.perms[b].ctrl
	if c&kop.RdrGateUntapped != 0 && s.perms[b].flags&pfTapped != 0 {
		return false
	}
	if c&kop.RdrGateEquipped != 0 && s.perms[b].attached == NoSlot {
		return false
	}
	if c&kop.RdrCombatOnly != 0 && !e.combat {
		return false
	}
	if e.sl == NoSlot {
		if c&kop.RdrToYou == 0 || e.rcp != hc {
			return false
		}
	} else {
		att := s.perms[b].attached
		enchanted := att != NoSlot && s.occ.has(att) && e.sl == att
		if !(c&kop.RdrToEnchanted != 0 && enchanted) &&
			!(c&kop.RdrToYouPerms != 0 && e.rcp == hc && e.sl != b) {
			return false
		}
	}
	if c&kop.RdrSrcUnblocked != 0 {
		// gorge: o.IsAttacking && len(o.BlockedBy) == 0, read live at the
		// combat assignments (engine/effects/filter.go unblocked); never
		// outside combat
		if !e.combat || e.srcSl == NoSlot {
			return false
		}
		fl := s.perms[e.srcSl].flags
		if fl&pfAttacking == 0 || fl&pfBlocked != 0 {
			return false
		}
	}
	if c&kop.RdrSrcChosen != 0 && e.srcSl != NoSlot {
		return false // the chosen colour reads a spell source only
	}
	if r.Filter2 != 0 {
		if e.srcSl != NoSlot {
			if !s.matchPermGen(k, r.Filter2, e.srcSl, hc, b, layerFull) {
				return false
			}
		} else if c&kop.RdrSrcChosen != 0 {
			// the source's own choice against the spell's colours (Harsh
			// Judgment; Diamond Mare's spell-of-the-chosen-colour read)
			if !s.matchCardSrc(k, r.Filter2, e.srcCard, e.srcCtrl, hc, NoObj, b, s.perms[b].obj) {
				return false
			}
		} else {
			ctx := evalCtx{src: s.perms[b].obj, srcSl: b, ctrl: hc}
			if !s.matchCardGenAt(k, r.Filter2, e.srcCard, e.srcCtrl, hc, NoObj, &ctx) {
				return false
			}
		}
	}
	return true
}

// statRdrCollect fills out with the matching redirects of e in gorge's scan
// order (each seat's battlefield in entry order, the abilities in print
// order) and returns their number; -1 after a loud stall (more than rdrMax).
func (s *State) statRdrCollect(k *prep, e *rdrEv, out *[rdrMax]rdrCand) int {
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var bf [MaxPerm]Slot
		for _, b := range bf[:s.battlefieldOrder(p, s.statM.and(s.ctrlM[p]), bf[:], 0)] {
			if s.genLost(k, b) {
				continue
			}
			ab := k.t.Cards[s.cardAt(b)].Abilities
			for j := range ab {
				if ab[j].Kind != kop.AbReplacement || ab[j].Repl.Kind != kop.ReplDamageRedirect {
					continue
				}
				if !s.statRdrMatch(k, b, &ab[j], e) {
					continue
				}
				if n == rdrMax {
					s.capFull(StallRedirOrder) // loud: never truncate the candidates
					return -1
				}
				out[n] = rdrCand{b: b, ab: &ab[j]}
				n++
			}
		}
	}
	return n
}

// statRdrCompetes reports whether the matched redirect competes with another
// damage replacement for the event (gorge's CR 616.1 order ask): an Ext
// DmgRedirect record, or another replacement the kernel's superset guard
// reads (prevNextCompetes: a depleting shield, a colour or scope record, a
// printed prevention, a doubler, a plus-N, a self-damage kind, a damage
// mark), or a source whose damage the rewrite does not model (infect,
// wither).
func (s *State) statRdrCompetes(k *prep, e *rdrEv) bool {
	if _, h := s.redirFind(e.code, e.rcp); h > 0 {
		return true
	}
	if k.gen.anyInfect {
		infect := true // a source with no slot: its infect is not read here
		if e.srcSl != NoSlot {
			infect = s.infectAt(k, e.srcSl) || s.witherKw(k, e.srcSl, s.kw(k, e.srcSl))
		}
		if infect {
			return true
		}
	}
	return s.redirCompetes(k, e.srcCtrl, e.srcSl, e.srcCard, e.code, e.rcp, e.sl, e.combat)
}

// statRdrStall reports whether a static redirect matches the event of damage
// n from the source to code and another damage replacement would also be a
// candidate of it (gorge's CR 616.1 order ask): it sets StallRedirOrder and
// reports true, so nothing answers before the ask. A redirect that matches
// alone is no stall (gorge asks only for two or more candidates).
func (s *State) statRdrStall(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, code TCode, combat bool) bool {
	if k.gen == nil || !k.gen.uses.has(useStatRdr) {
		return false
	}
	e, ok := s.rdrEvOf(srcCtrl, srcSl, srcCard, code, combat)
	if !ok {
		return false
	}
	var cands [rdrMax]rdrCand
	n := s.statRdrCollect(k, &e, &cands)
	if n <= 0 {
		return false
	}
	if n > 1 || s.statRdrCompetes(k, &e) {
		s.capFull(StallRedirOrder)
		return true
	}
	return false
}

// statRdrTake is replDamage's static-redirect arm for noncombat damage n from
// the source to the recipient code: the lone matching redirect rewrites the
// event's recipient and the damage lands there (no replacements re-run, gorge
// replacement_damage.go's single emit). It reports whether it answered the
// event (applied it, or stalled so nothing else may).
func (s *State) statRdrTake(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, code TCode, rcp uint8, sl Slot, combat bool, n int) bool {
	if k.gen == nil || !k.gen.uses.has(useStatRdr) || combat || n <= 0 {
		return false // the combat events are taken in replCombatGen
	}
	e, ok := s.rdrEvOf(srcCtrl, srcSl, srcCard, code, false)
	if !ok {
		return false
	}
	var cands [rdrMax]rdrCand
	n2 := s.statRdrCollect(k, &e, &cands)
	if n2 <= 0 {
		return false
	}
	if n2 > 1 || s.statRdrCompetes(k, &e) {
		s.capFull(StallRedirOrder)
		return true
	}
	to, ok := s.statRdrTo(k, cands[0].b, cands[0].ab, &e)
	if !ok {
		return false // a no-op or unmodelled rewrite: the event stands
	}
	s.statRdrDeal(k, srcCtrl, srcSl, srcCard, to, n)
	return true
}

// statRdrTo is the recipient the redirect rewrites the event to (a
// permanent's object or a player target code); false when the rewrite is a
// no-op (the event is already dealt to that recipient) or unmodelled.
func (s *State) statRdrTo(k *prep, b Slot, a *kop.Ability, e *rdrEv) (TCode, bool) {
	c := a.Repl.C
	var to TCode
	switch {
	case c&kop.RdrAsSelf != 0:
		to = TCode(s.perms[b].obj)
	case c&kop.RdrAsAttached != 0:
		att := s.perms[b].attached
		if att == NoSlot || !s.occ.has(att) {
			return 0, false // gorge's rewrite names nothing: the event stands
		}
		to = TCode(s.perms[att].obj)
	case c&kop.RdrAsSrcCtrl != 0:
		if e.srcCtrl >= MaxSeats {
			return 0, false
		}
		to = PlayerCode(e.srcCtrl)
	case c&kop.RdrAsTgtCtrl != 0:
		if e.rcp >= MaxSeats {
			return 0, false
		}
		to = PlayerCode(e.rcp)
	default:
		return 0, false
	}
	if to == e.code {
		return 0, false // a no-op rewrite: the event stands
	}
	return to, true
}

// statRdrDeal lands the redirected damage n on the recipient to as the one
// event it is: a permanent's damage, or a player's life loss, from the
// original source (its lifelink, deathtouch, marks and damage triggers).
func (s *State) statRdrDeal(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, to TCode, n int) {
	if !isPlayerOrNone(to) {
		s.redirLand(k, srcCtrl, srcSl, srcCard, to, n) // repl_dmgredirect.go
		return
	}
	kw, ok := s.redirKw(k, srcSl, srcCard)
	if !ok {
		return
	}
	s.noteDmgTaken(CodeSeat(to), n) // kw_bloodthirst.go
	s.notePlDamagedNC(k, to)        // value_oppsdamaged.go
	s.loseLifeDmg(k, CodeSeat(to), n)
	s.res.lastDmg = int16(n)
	if kw&kop.KwLifelink != 0 {
		s.gainLife(k, srcCtrl, n)
	}
	if k.gen.anyPermMark && srcSl != NoSlot {
		s.markDealt(k, srcSl, false) // filter_permmarks.go
	}
	if s.subs[kop.TrigDamageDone].any() || s.subs[kop.TrigDamageDealtOnce].any() || k.gen.graveDmg {
		if srcSl != NoSlot {
			s.fireDamage(k, srcSl, int16(n), true, false, to)
		} else {
			s.fireDamageSrc(k, s.res.e.obj, s.res.e.ctrl, int16(n), dmgPl(to), to, nil)
		}
	}
}

// statRdrCombat is replCombatGen's static-redirect arm for combat event ev of
// r (the recipient code, controlled by rcp): the lone matching redirect's
// damage leaves the event and lands on the rewritten recipient's event. It
// reports whether the event was taken (applied, or stalled so nothing else
// may answer).
func (s *State) statRdrCombat(k *prep, r *dmgRec, ev *dmgEv, byOrd bool, code TCode, rcp uint8) bool {
	e := rdrEv{srcCtrl: s.perms[ev.src].ctrl, srcSl: ev.src, srcCard: s.cardAt(ev.src),
		code: code, rcp: rcp, sl: ev.dst, combat: true}
	var cands [rdrMax]rdrCand
	n := s.statRdrCollect(k, &e, &cands)
	if n <= 0 {
		return false
	}
	if n > 1 || s.statRdrCompetes(k, &e) {
		s.capFull(StallRedirOrder)
		return true
	}
	to, ok := s.statRdrTo(k, cands[0].b, cands[0].ab, &e)
	if !ok {
		return false
	}
	if isPlayerOrNone(to) {
		// a combat pass records a player hit only as the attacker's own
		// defender: a redirected player recipient is unmodelled, loud
		s.capFull(StallRedirOrder)
		return true
	}
	if byOrd || r.nev >= len(r.ev) {
		s.capFull(StallRedirOrder) // an order-sensitive walk, or no room for the new event
		return true
	}
	tsl := s.objs[to].slot
	m := int(ev.n)
	s.combatEvLess(r, ev, code, m)
	r.toPerm[tsl] = s.addSat16(r.toPerm[tsl], m)
	r.dealt[ev.src] = s.addSat16(r.dealt[ev.src], m)
	if s.kw(k, ev.src)&kop.KwDeathtouch != 0 {
		r.dtPerm.set(tsl)
	}
	r.note(ev.src, tsl, s.sat16(m))
	return true
}

// statRdrFrozen reports whether a static redirect matches the frozen combat
// event fe (the paused order walk): the walk cannot pose gorge's ask with the
// redirect among the candidates, so the caller stalls instead of answering.
func (s *State) statRdrFrozen(k *prep, fe *fzEv) bool {
	e := rdrEv{srcCtrl: fe.ctrl, srcSl: fe.src, srcCard: fe.card, combat: true}
	if fe.dst == NoSlot {
		e.code, e.rcp = PlayerCode(fe.apl), fe.apl
	} else {
		e.code = TCode(s.perms[fe.dst].obj)
		e.rcp = s.perms[fe.dst].ctrl
		e.sl = fe.dst
	}
	var cands [rdrMax]rdrCand
	return s.statRdrCollect(k, &e, &cands) > 0
}
