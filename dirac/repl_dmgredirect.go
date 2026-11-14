package dirac

import (
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Ext DmgRedirect (gorge replacement_splitdamage.go, replacement_replace.go's
// ReplaceEffect Affected rewrite): an Effect's DamageDone redirect, in two
// kinds (kop/dmgredirect.go).
//
//   - DmgRedirectNext (Nomads en-Kor, Warrior en-Kor): the next `pool` (1)
//     damage that would be dealt to the host this turn is dealt to the
//     remembered creature instead. n = min(held amount, pool) is dealt to it
//     as a new damage event from the same source (so it meets protection,
//     prevention and doubling again, and a redirect record of its own); the
//     held event keeps the rest.
//   - DmgRedirectScope (Gideon's Sacrifice): all damage this turn to the
//     controller and the permanents they control is dealt to the chosen
//     permanent instead while it is on the battlefield. The chosen permanent's
//     own damage is the replacement's own recipient already: the rewrite
//     changes nothing, but the record is still a candidate of the event (two
//     of them, or one beside another replacement, are gorge's order ask).
//
// A record ends with the turn, with its pool, and when the host or the
// recipient leaves the battlefield (the Effect's ExileOnMoved$ Battlefield and
// Duration$ UntilHostLeavesPlayOrEOT; redirDrop). The records live in
// genState.redir, apart from the prevention records, because a redirect names
// two objects.
//
// A redirect applies after protection (CR 702.16e prevents first, gorge emit).
// Anything else that would be a candidate for the same event (another
// redirect, a prevention or scope record, a shield, a doubler, a plus-N, a
// mark: prevNextCompetes) is gorge's CR 616.1 order ask, which the records
// are no candidate of: the kernel stalls StallRedirOrder rather than answer
// without it. So does a source whose damage the redirect does not model
// (infect, wither), and a combat pass already in an order ask.

const (
	maxRedir   = 4
	redirBytes = maxRedir * int(unsafe.Sizeof(redirRec{})) // genState.redir: taken from rsv (gen.go rsvN)
)

// redirRec is one live redirect: kind 0 is a free slot; host the damaged
// permanent (DmgRedirectNext), tgt the recipient, ctl the controller whose
// damage is redirected (DmgRedirectScope) and pool the damage left (Next).
type redirRec struct {
	host, tgt       Obj
	ctl, pool, kind uint8
}

func init() {
	stallNames[StallRedirOrder] = "dmgredirectorder"
	stallNames[StallRedirCap] = "dmgredirectcap"
}

// DebugNoDmgRedirect (the redirect lockstep mutation): the records are made
// and spent nowhere; the damage is dealt to its original recipient.
var DebugNoDmgRedirect bool

// redirAdd is Ext DmgRedirect: the resolving Effect registers a redirect to
// the permanent Obj names.
func (s *State) redirAdd(k *prep, in *kop.Instr) {
	code := s.refCode(k, in.Obj)
	if code == NoTarget || isPlayerOrNone(code) || int(code) >= MaxObj || s.objs[code].zone != kop.ZoneBattlefield {
		return // gorge: no live referent, the registration redirects nothing
	}
	e := &s.res.e
	r := redirRec{tgt: Obj(code), kind: uint8(in.C)}
	switch r.kind {
	case kop.DmgRedirectNext:
		if e.obj == NoObj || s.objs[e.obj].zone != kop.ZoneBattlefield {
			return // Duration$ UntilHostLeavesPlayOrEOT: the host has left
		}
		r.host, r.pool = e.obj, 1
	case kop.DmgRedirectScope:
		r.ctl = e.ctrl
	default:
		return
	}
	for i := range s.g.redir {
		if s.g.redir[i].kind == 0 {
			s.g.redir[i] = r
			return
		}
	}
	s.capFull(StallRedirCap) // loud: never drop an effect
}

// redirDrop ends the records that name the permanent o leaving the battlefield.
func (s *State) redirDrop(o Obj) {
	for i := range s.g.redir {
		if r := &s.g.redir[i]; r.kind != 0 && (r.tgt == o || r.host == o && r.kind == kop.DmgRedirectNext) {
			*r = redirRec{}
		}
	}
}

// redirLive is the number of live records: the sweep guard's conservative
// count of the replacements they bring to some hit.
func (s *State) redirLive() int {
	n := 0
	for i := range s.g.redir {
		if s.g.redir[i].kind != 0 {
			n++
		}
	}
	return n
}

// redirFind finds the records that redirect a damage event to the recipient
// code (controlled by rcp): the last matching record's index and how many
// match.
func (s *State) redirFind(code TCode, rcp uint8) (at, hits int) {
	at = -1
	for i := range s.g.redir {
		r := &s.g.redir[i]
		switch r.kind {
		case kop.DmgRedirectNext:
			if r.pool == 0 || code != TCode(r.host) {
				continue
			}
		case kop.DmgRedirectScope:
			if r.ctl != rcp {
				continue
			}
		default:
			continue
		}
		if s.objs[r.tgt].zone != kop.ZoneBattlefield {
			continue // "if it's still on the battlefield": the held event stands
		}
		at = i
		hits++
	}
	return at, hits
}

// redirBeside is the guard of an arm that answers an event before the
// redirect arm is reached (a prevention record, a depleting shield): when a
// redirect record also matches the event, gorge collects both as CR 616.1
// candidates and asks the order, so the kernel stalls StallRedirOrder. It
// reports whether it stalled.
func (s *State) redirBeside(code TCode, rcp uint8) bool {
	if DebugNoDmgRedirect {
		return false
	}
	if _, h := s.redirFind(code, rcp); h > 0 {
		s.capFull(StallRedirOrder)
		return true
	}
	return false
}

// redirCompetes reports whether another damage replacement would also be a
// candidate for the event (a conservative superset, as prevNextCompetes).
func (s *State) redirCompetes(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, code TCode, rcp uint8, sl Slot, combat bool) bool {
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.sc&prevNextSc != 0 && r.col != 0 && r.ctl == code {
			return true // a depleting shield on the recipient
		}
	}
	return s.prevNextCompetes(k, srcCtrl, srcSl, srcCard, rcp, sl, combat)
}

// redirSpend takes record at's share of an event of n: the amount to
// redirect and its recipient's object code. A pool spent ends the record.
func (s *State) redirSpend(at, n int) (int, TCode) {
	r := &s.g.redir[at]
	tgt := TCode(r.tgt)
	if r.kind != kop.DmgRedirectNext {
		return n, tgt
	}
	m := min(n, int(r.pool))
	r.pool -= uint8(m)
	if r.pool == 0 {
		*r = redirRec{}
	}
	return m, tgt
}

// redirMatch is the shared front of redirTake and redirCombat: the record
// that redirects the event, or -1 when none does or the event is stalled.
func (s *State) redirMatch(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, code TCode, rcp uint8, sl Slot, combat bool) int {
	if DebugNoDmgRedirect {
		return -1
	}
	at, hits := s.redirFind(code, rcp)
	if hits == 0 {
		return -1
	}
	if hits > 1 || s.redirCompetes(k, srcCtrl, srcSl, srcCard, code, rcp, sl, combat) {
		s.capFull(StallRedirOrder) // gorge asks the CR 616.1 order; the records are no candidate of it
		return -1
	}
	if r := &s.g.redir[at]; r.kind == kop.DmgRedirectScope && code == TCode(r.tgt) {
		return -1 // the chosen permanent's own damage: a no-op rewrite
	}
	if k.gen.anyInfect {
		infect := false
		if srcSl != NoSlot {
			infect = s.infectAt(k, srcSl) || s.witherKw(k, srcSl, s.kw(k, srcSl))
		} else {
			infect = true // a source with no slot: its infect is not read here
		}
		if infect {
			s.capFull(StallRedirOrder)
			return -1
		}
	}
	return at
}

// redirTake is replDamage's redirect arm for noncombat damage n from the
// source to the recipient code (controlled by rcp, the permanent at sl, else
// a player): it deals the redirected share to the record's recipient and
// reports the damage left for the original one.
func (s *State) redirTake(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, code TCode, rcp uint8, sl Slot, n int) (int, bool) {
	at := s.redirMatch(k, srcCtrl, srcSl, srcCard, code, rcp, sl, false)
	if at < 0 {
		return n, false
	}
	m, tcode := s.redirSpend(at, n)
	if m <= 0 {
		return n, false
	}
	s.redirDeal(k, srcCtrl, srcSl, srcCard, tcode, m)
	return n - m, true
}

// redirDeal deals the redirected damage m to the permanent tcode as a new
// event from the source: replacements again, then the damage with deathtouch,
// lifelink and the damage triggers of damageTo. A source with no slot is read
// only while it is the resolving entry.
func (s *State) redirDeal(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, tcode TCode, m int) {
	if m = s.replDamage(k, srcCtrl, srcSl, srcCard, tcode, false, m); m <= 0 {
		return
	}
	e := &s.res.e
	var kw kop.KwMask
	if srcSl != NoSlot {
		kw = s.kw(k, srcSl)
	} else if s.res.active && e.card == srcCard && e.obj != NoObj {
		kw = s.srcKw(k)
	} else {
		s.capFull(StallRedirOrder) // a source the kernel cannot read here
		return
	}
	if s.objs[tcode].zone != kop.ZoneBattlefield {
		return
	}
	dsl := s.objs[tcode].slot
	s.touchSl(dsl)
	s.dmgPerm(k, dsl, &s.perms[dsl], m)
	if kw&kop.KwDeathtouch != 0 {
		s.perms[dsl].markDT(1)
	}
	if k.gen.dmgBy != nil && srcSl != NoSlot {
		s.noteDmgBy(k, srcSl, dsl)
	}
	s.res.lastDmg = int16(m)
	if kw&kop.KwLifelink != 0 {
		s.gainLife(k, srcCtrl, m)
	}
	if k.gen.anyPermMark && srcSl != NoSlot {
		s.markDealt(k, srcSl, false)
	}
	if s.subs[kop.TrigDamageDone].any() || s.subs[kop.TrigDamageDealtOnce].any() || k.gen.graveDmg {
		if srcSl != NoSlot {
			s.fireDamage(k, srcSl, int16(m), false, false, tcode)
		} else {
			s.fireDamageSrc(k, e.obj, e.ctrl, int16(m), 0, tcode, nil)
		}
	}
}

// redirCombat is replCombatGen's redirect arm for combat event ev of r (the
// recipient code, controlled by rcp): the redirected share leaves the event
// and joins the pass as a new event from the same source to the record's
// recipient, which the loop then walks like any other. It reports whether the
// event lost damage.
func (s *State) redirCombat(k *prep, r *dmgRec, ev *dmgEv, ordered bool, code TCode, rcp uint8) bool {
	at := s.redirMatch(k, s.perms[ev.src].ctrl, ev.src, s.cardAt(ev.src), code, rcp, ev.dst, true)
	if at < 0 {
		return false
	}
	if ordered || s.g.ro.on || r.nev >= len(r.ev) {
		s.capFull(StallRedirOrder) // an event-by-event ordered walk, or no room for the new event
		return false
	}
	m, tcode := s.redirSpend(at, int(ev.n))
	if m <= 0 {
		return false
	}
	tsl := s.objs[tcode].slot
	src := ev.src
	s.combatEvLess(r, ev, code, m)
	r.toPerm[tsl] = s.addSat16(r.toPerm[tsl], m)
	r.dealt[src] = s.addSat16(r.dealt[src], m)
	if s.kw(k, src)&kop.KwDeathtouch != 0 {
		r.dtPerm.set(tsl)
	}
	r.note(src, tsl, s.sat16(m))
	return true
}
