package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// prevCombatTo reports whether the permanent at sl prevents every combat
// damage event dealt to it: the granted/derived KwPreventCombatDamageTo bit
// (Fleeting Flight), or a printed K:Prevent sentence's card-def fact
// (PreventCombatTo, or PreventDmgTo which covers combat too). The printed
// fact is the card's face replacement, so it is gone when the permanent has
// lost all abilities or is face down (gorge replacementFace's
// printedAbilitiesLost, CR 708.2), like every other printed per-card
// replacement bit (replCandLive, parkNC, kw_wither).
func (s *State) prevCombatTo(k *prep, sl Slot) bool {
	if s.kwGen(k, sl)&kop.KwPreventCombatDamageTo != 0 {
		return !s.noPrev(k, true)
	}
	return k.gen != nil && k.gen.replBits[s.cardAt(sl)]&(rbPrevCombatTo|rbPrevDmgTo) != 0 && !s.genLost(k, sl) && !s.noPrev(k, true)
}

// prevCombatFrom reports whether the permanent at sl prevents every combat
// damage event it would deal: the granted/derived KwPreventCombatDamageFrom
// bit (Maze of Ith) or the printed "dealt by" sentence (Fog Bank), the
// latter gone with lost abilities or face down (prevCombatTo).
func (s *State) prevCombatFrom(k *prep, sl Slot) bool {
	if s.kwGen(k, sl)&kop.KwPreventCombatDamageFrom != 0 {
		return !s.noPrev(k, true)
	}
	return k.gen != nil && k.gen.replBits[s.cardAt(sl)]&rbPrevCombatBy != 0 && !s.genLost(k, sl) && !s.noPrev(k, true)
}

// prevFromGrant reports whether the permanent at sl carries the granted
// KwPreventCombatDamageFrom (an Effect's prevention in gorge, so a CR 616.1
// candidate of the events it deals), not the printed "dealt by" sentence.
func (s *State) prevFromGrant(k *prep, sl Slot) bool {
	return s.kwGen(k, sl)&kop.KwPreventCombatDamageFrom != 0 && !s.noPrev(k, true)
}

// recordPrevGrant remembers the source of an until-end-of-turn
// KwPreventCombatDamageFrom / KwPreventCombatDamageTo grant on sl: gorge's
// Effect names itself in the CR 616.1 option, not the creature it protects,
// so the order ask needs the granting object. A full effect pool drops the
// record (the grant still applies; only the option's name falls back).
func (s *State) recordPrevGrant(sl Slot, m kop.KwMask) {
	if m&(kop.KwPreventCombatDamageFrom|kop.KwPreventCombatDamageTo) == 0 {
		return
	}
	src := s.res.e.obj
	if src == NoObj {
		return
	}
	if s.fxM.has(sl) {
		for i := 0; i < int(s.g.nfx); i++ {
			if r := &s.g.fx[i]; r.sl == sl && r.kind == fxPrevGrant {
				r.r.p = int16(src)
				return
			}
		}
	}
	if int(s.g.nfx) >= maxFx {
		return
	}
	s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxPrevGrant, r: ovRec{p: int16(src)}}
	s.g.nfx++
	s.fxM.set(sl)
}

// prevGrantSrc is the object that granted sl its until-end-of-turn combat
// damage prevention, or NoObj (a printed prevention, a directly set keyword,
// or a dropped record: the ask names the protected permanent then).
func (s *State) prevGrantSrc(sl Slot) Obj {
	if !s.fxM.has(sl) {
		return NoObj
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxPrevGrant {
			return Obj(r.r.p)
		}
	}
	return NoObj
}

// prevAnyTo reports whether the permanent at sl prevents every damage event
// dealt to it, combat and noncombat ("Prevent all damage that would be dealt
// to CARDNAME."), the printed fact gone with lost abilities or face down
// (prevCombatTo); combat: the damage is combat damage (noPrev).
func (s *State) prevAnyTo(k *prep, sl Slot, combat bool) bool {
	return k.gen != nil && k.gen.replBits[s.cardAt(sl)]&rbPrevDmgTo != 0 && !s.genLost(k, sl) && !s.noPrev(k, combat)
}

// preventCombatEv takes combat damage event ev out of r's accounting: the
// event is prevented whole (a shield counter, protection), so it is not
// damage dealt — no DamageDone/DamageDealtOnce trigger, no lifelink, no
// deathtouch mark. The source's dealt totals lose it, the recipient's
// assigned damage loses it, and its deathtouch mark is recomputed from the
// events that remain. Every combat prevention goes through here
// (replCombatGen's protection, shieldCombat, dealCombatEvent and
// dealAsked's shield), so a new prevention cannot leave r thinking the
// damage was dealt and fire its triggers.
func (s *State) preventCombatEv(k *prep, r *dmgRec, ev *dmgEv) {
	if ev.n <= 0 {
		return
	}
	r.dealt[ev.src] -= ev.n
	if ev.dst == NoSlot {
		r.dealtPl[ev.src] -= ev.n
		if a := s.perms[ev.src].atk; isPlayerOrNone(a) {
			r.toPl[CodeSeat(a)] -= ev.n
		}
	} else {
		r.toPerm[ev.dst] -= ev.n
		r.dtPerm.clr(ev.dst)
		for i := 0; i < r.nev; i++ {
			if e := &r.ev[i]; e.dst == ev.dst && e.n > 0 && s.kw(k, e.src)&kop.KwDeathtouch != 0 {
				r.dtPerm.set(ev.dst)
			}
		}
	}
	ev.n = 0
}

// preventFrozenEv syncs the recomputed pass's accounting for a frozen event
// already dealt in the paused pass. The resumed pass rebuilds r from the
// board, so a shield that prevented the event before the ask is not in it;
// replOrderCombat skips the already-dealt event, and without this the
// rebuilt event would fire the DamageDone trigger at the end of the pass.
func (s *State) preventFrozenEv(k *prep, r *dmgRec, fe *fzEv) {
	if fe.n > 0 {
		return
	}
	for i := 0; i < r.nev; i++ {
		if e := &r.ev[i]; e.src == fe.src && e.dst == fe.dst {
			s.preventCombatEv(k, r, e)
			return
		}
	}
}
