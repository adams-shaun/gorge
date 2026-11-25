package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// damageAllOpp is OpDamageAll with its B.N bits 2 (each opponent of the
// controller is dealt the damage too, after the permanents) and 4 (the
// filter's own types choose the permanents: "each creature and
// planeswalker they control"), End the Festivities (gorge effDamageAll:
// ValidCards$ over both battlefields, then ValidPlayers$, one batch).
//
// Bit 8 deals the damage to the player the instruction's Obj names (a target
// slot, RefTarget0..4; gorge validPlayers' ValidPlayers$ Targeted) after the
// permanents, and bit 16 scopes the permanents to that player's own (gorge
// targetPlayerKindScope: a player-kind ValidTgts$ on the sweep). A target
// that names no player sweeps no permanent under bit 16 and damages no one.
//
// Bit 32 is ValidPlayers$ OppNonTriggeredTarget alone: damageOtherOpps.
//
//go:noinline
func (s *State) damageAllOpp(k *prep, in *kop.Instr, ctrl uint8, n int) {
	if in.B.N&32 != 0 {
		s.damageOtherOpps(k, ctrl, n)
		return
	}
	src := s.objRef(k, kop.RefSelf)
	tseat := uint8(255)
	if in.B.N&24 != 0 {
		tseat = s.playerRef(k, in.Obj)
	}
	var hit slotMask
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if in.B.N&16 != 0 && (tseat == 255 || !s.ctrlM[tseat].has(sl)) {
			continue
		}
		if (in.B.N&4 != 0 || s.isCreature(k, sl)) && s.matchPerm(k, in.F, sl, ctrl, src) {
			hit = hit.with(sl)
		}
	}
	var pl uint8 // dmgSeqPl's bits: 1 the active player, 2 the other; the seat mask at more seats
	if in.B.N&2 != 0 && MaxSeats != 2 {
		pl = seatBits(s.oppsFrom(ctrl))
	} else if in.B.N&2 != 0 {
		if s.nextSeat(ctrl) == s.active {
			pl = 1
		} else {
			pl = 2
		}
	} else if in.B.N&8 != 0 && tseat != 255 {
		switch {
		case MaxSeats != 2:
			pl = 1 << tseat
		case tseat == s.active:
			pl = 1
		default:
			pl = 2
		}
	}
	if k.gen != nil && (k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevDmgTo|rbPrevCol) != 0 || shieldBesidePlus(k.gen) ||
		k.gen.anyRepl2&rb2DmgAny != 0) {
		s.damageSeqStart(k, hit, false)
		s.g.dmgSeqPl = pl
		s.damageSeqNext(k, n)
		return
	}
	// Same sweep order as OpDamageAll's plain path (gorge effDamageAll:
	// each seat's battlefield in entry order, seat 0 first, then each
	// opponent from the controller, players last). The whole sweep is one
	// damage batch for the DamageDealtOnce latch (gorge effDamageAll's
	// BeginDamageBatch; trig_dmgbatch.go).
	s.dmgBatchOpen()
	var ord [MaxPerm]Slot
	hi := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		hi = s.battlefieldOrder(p, hit.and(s.ctrlM[p]), ord[:], hi)
	}
	for _, sl := range ord[:hi] {
		s.damageTo(k, TCode(s.perms[sl].obj), n)
	}
	if pl != 0 {
		q := s.oppsFrom(ctrl)
		if in.B.N&2 == 0 {
			q = seatOne(tseat)
		}
		for i := uint8(0); i < q.n; i++ {
			s.damageTo(k, PlayerCode(q.s[i]), n)
		}
	}
	s.dmgBatchClose()
}

// damageOtherOpps is ValidPlayers$ OppNonTriggeredTarget (Hydra Omnivore,
// U.S.S. Prometheus: "it deals that much damage to each other opponent"):
// no permanent, and each living opponent of ctrl but the trigger's player
// (the player the trigger's damage was dealt to), in seat order from seat 0
// (gorge validPlayers: AliveFrom(0) less the controller and
// c.TriggerTarget's player; CR 102.2). At two seats that is nobody. One
// damage batch, dealt one event at a time when replacements may park
// (damageSeqNext, dmgSeqSeat0 keeping the seat-0 order).
func (s *State) damageOtherOpps(k *prep, ctrl uint8, n int) {
	ex := noSeat
	if e := s.curEntry(); e != nil && e.kind == seTrigger && !DebugOtherOppsAll {
		ex = e.trigPl
	}
	q := s.aliveFrom(0)
	var m uint8
	for i := uint8(0); i < q.n; i++ {
		if p := q.s[i]; p != ctrl && p != ex {
			m |= 1 << p
		}
	}
	if m == 0 {
		return
	}
	if MaxSeats != 2 && k.gen != nil && (k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevDmgTo|rbPrevCol) != 0 || shieldBesidePlus(k.gen) ||
		k.gen.anyRepl2&rb2DmgAny != 0) {
		s.damageSeqStart(k, slotMask{}, false)
		s.g.dmgSeqPl = m | dmgSeqSeat0
		s.damageSeqNext(k, n)
		return
	}
	s.dmgBatchOpen()
	for i := uint8(0); i < q.n; i++ {
		if p := q.s[i]; m&(1<<p) != 0 {
			s.damageTo(k, PlayerCode(p), n)
		}
	}
	s.dmgBatchClose()
}

// dmgSeqSeat0 is the dmgSeqPl bit (above the seat mask) that deals the
// sequence's players in seat order from seat 0 instead of from the
// resolving controller (damageOtherOpps).
const dmgSeqSeat0 = 0x80

// DebugOtherOppsAll is the lockstep mutation of damageOtherOpps: it deals
// the damaged opponent too (TestFourSeatOtherOpponentsLockstep must then
// diverge).
var DebugOtherOppsAll bool
