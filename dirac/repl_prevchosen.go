package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The chosen-source shield (Ext PreventNext with kop.PrevNextChosen; gorge
// effEffect's ChooseSource prevention family): "the next time a source of your
// choice would deal damage to you this turn, prevent that damage" (Circle of
// Protection: Red, Rune of Protection, Invulnerability), and its recipient and
// lifetime variants (kop.PrevNextChosenTo/Any/Turn: Martyr's Cause, Pilgrim of
// Justice, Pay No Heed). It is the
// source-keyed shield of repl_prevnext.go (pxFrom: spent by the first event it
// prevents, the Effect exiling itself) with the recipient fixed to its
// controller as a player and the source read from the chosen set the Ext
// ChooseSource ask filled (act_choosesource.go). The record:
//
//	obj  the Effect's source (the option an order ask names)
//	ctl  the chosen source's object (a permanent, or a spell on the stack)
//	col  prevChosenLive | the ChosenSrc* mask the source must still meet when
//	     the damage is dealt (ValidSource$ Card.ChosenCardStrict+RedSource: a
//	     source that changed colour since the choice is not prevented); 0 is spent
//	sc   prevNextSc | the pcTo/pcAny/pcTurn bits
//	prevX  pxFrom|pxPlayers with the controller's seat in the rider's player
//	     bits; no rider kind or pxRidGain (pnxChosenForm)
//	prevRidRec  t the recipient's code and n its version (pcTo)
//
// The source is matched by object, as gorge's ChosenCardStrict is by ObjID: a
// permanent that left the battlefield and came back, or the chosen spell
// resolved to a graveyard, is still the chosen object. It matches a source
// dealing damage as a permanent (its slot) or as the resolving spell or
// ability (a source that left the battlefield first).

// prevChosenLive is the live bit of a chosen-source record's col.
const prevChosenLive kop.ColorMask = 0x80

// A chosen-source record's sc bits beside prevNextSc (a PreventNext record
// reads no scope bits; bit 5, PrevScopeFiltered, is left alone): the
// recipient is the record's prevRidRec target at its version (pcTo), any
// recipient (pcAny), else the controller as a player; pcTurn keeps the record
// for every event this turn instead of spending it on the first.
const (
	pcTo   uint8 = 1 << 0
	pcAny  uint8 = 1 << 1
	pcTurn uint8 = 1 << 2
	// pcSpell: the source is a stack spell at its cast version (kop.PrevNextChosenSpell,
	// Dromoka's Command): the record's ctl is the spell's object, prevRidRec.n its
	// version, so a later cast of the same card is another source.
	pcSpell uint8 = 1 << 3
)

// DebugNoPrevSpell (the Dromoka's Command lockstep mutation): a spell-keyed
// shield is never installed.
var DebugNoPrevSpell bool

// DebugNoPrevChosen (the chosen-source lockstep mutation): the shield is
// installed but prevents nothing.
var DebugNoPrevChosen bool

// prevChosenAdd installs the shield on the chosen source; no source chosen (an
// empty pool) is no Effect.
func (s *State) prevChosenAdd(k *prep, in *kop.Instr) {
	spell := in.C&kop.PrevNextChosenSpell != 0 // seat-ok: an instruction flag bit
	var src TCode
	if spell {
		if src = s.prevSpellSource(k, in); src == NoTarget {
			return
		}
	} else if s.g.nchosen == 0 {
		return // no source chosen (an empty pool, or a stack ability: act_choosesource.go) is no Effect
	}
	mask := s.value(k, in.B)
	sc, rt, ver := prevNextSc, TCode(NoTarget), uint8(0)
	switch {
	case spell:
		sc |= pcAny | pcSpell
		ver = s.objs[src].ver
	case in.C&kop.PrevNextChosenTo != 0:
		rt = s.refCode(k, in.Obj)
		if rt == NoTarget {
			return // gorge: no live referent, the Effect prevents nothing
		}
		if !isPlayerOrNone(rt) {
			if int(rt) >= MaxObj || s.objs[rt].zone != kop.ZoneBattlefield {
				return
			}
			ver = s.objs[rt].ver
		}
		sc |= pcTo
	case in.C&kop.PrevNextChosenAny != 0:
		sc |= pcAny
	}
	if in.C&kop.PrevNextChosenTurn != 0 {
		sc |= pcTurn
	}
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.col == 0 && r.sc == 0 {
			rr := s.prevRidAt(i)
			if sc&(pcTo|pcSpell) != 0 && rr == nil {
				s.capFull(StallPrevCol) // the recipient (or the spell's version) has nowhere to live
				return
			}
			if spell {
				r.obj, r.col, r.sc, r.ctl = s.res.e.obj, prevChosenLive|kop.ColorMask(mask), sc, src
			} else {
				r.obj, r.col, r.sc, r.ctl = s.res.e.obj, prevChosenLive|kop.ColorMask(mask), sc, TCode(s.g.chosen[0])
			}
			s.g.prevX[i] = pxFrom | pxPlayers | s.res.e.ctrl<<pxSeatShift
			if in.C&kop.PrevNextGain != 0 {
				s.g.prevX[i] |= pxRidGain // prevNextTake gains the prevented damage for the seat
			}
			if rr != nil {
				rr.t, rr.n, rr.gate = rt, ver, false
				if spell {
					rr.t = src
				}
			}
			return
		}
	}
	s.capFull(StallPrevCol) // loud: never drop an effect
}

// pnxChosenTo reports whether chosen-source record i's recipient is the
// damage recipient code.
func (s *State) pnxChosenTo(i int, code TCode) bool {
	r := &s.g.prevCol[i]
	switch {
	case r.sc&pcAny != 0:
		return true
	case r.sc&pcTo != 0:
		rr := s.prevRidAt(i)
		if rr == nil || rr.t != code {
			return false
		}
		// a permanent that left and came back is a new object to gorge's
		// IsRemembered (ExileOnMoved$/ForgetOnMoved$ Battlefield)
		return isPlayerOrNone(code) || int(code) < MaxObj && s.objs[code].ver == rr.n
	}
	return isPlayerTarget(code) && CodeSeat(code) == s.g.prevX[i]>>pxSeatShift
}

// pnxChosenTurn reports whether chosen-source record i lasts the turn
// (pcTurn): it prevents every event and is never spent.
func (s *State) pnxChosenTurn(i int) bool {
	return pnxChosenForm(s.g.prevX[i]) && s.g.prevCol[i].sc&pcTurn != 0
}

// pnxChosenForm reports whether rider byte x is a chosen-source record's: no
// rider, or the gain rider (kop.PrevNextGain: Reverse Damage's "you gain life
// equal to the damage prevented this way").
func pnxChosenForm(x uint8) bool {
	return x&(pxFrom|pxPlayers) == pxFrom|pxPlayers && (x&pxRider == 0 || x&pxRider == pxRidGain)
}

// pnxChosenMatches reports whether chosen-source record i matches damage to
// the recipient code from the permanent at srcSl (NoSlot: the resolving spell
// or ability's source object).
func (s *State) pnxChosenMatches(i int, code TCode, srcSl Slot) bool {
	r := &s.g.prevCol[i]
	if DebugNoPrevChosen || !s.pnxChosenTo(i, code) {
		return false
	}
	k := s.k()
	var src Obj
	var cols kop.ColorMask
	var ty kop.TypeMask
	switch {
	case srcSl != NoSlot:
		src, cols, ty = s.perms[srcSl].obj, s.colorsGen(k, srcSl), s.typesAt(k, srcSl)
	case s.res.active:
		src = s.res.e.obj
		cd := &k.t.Cards[s.objs[src].card]
		cols, ty = cd.Colors, cd.Types
	default:
		return false
	}
	if r.sc&pcSpell != 0 {
		// a spell-keyed shield: the spell at its cast version, on the stack
		rr := s.prevRidAt(i)
		if rr == nil || int(src) >= MaxObj || s.objs[src].ver != rr.n || s.objs[src].zone != kop.ZoneStack {
			return false
		}
	}
	return TCode(src) == r.ctl && chosenSrcMatches(int(r.col&^prevChosenLive), cols, ty)
}

// prevSpellSource is the stack spell a kop.PrevNextChosenSpell shield names
// (the instruction's Obj, a target slot): NoTarget when there is none, it is
// no spell on the stack any longer (gorge: the Effect is exiled as the
// remembered spell leaves it, so it prevents nothing) or the mutation is on.
func (s *State) prevSpellSource(k *prep, in *kop.Instr) TCode {
	c := s.refCode(k, in.Obj)
	if DebugNoPrevSpell || c == NoTarget || isPlayerOrNone(c) || int(c) >= MaxObj || s.objs[c].zone != kop.ZoneStack {
		return NoTarget
	}
	return c
}
