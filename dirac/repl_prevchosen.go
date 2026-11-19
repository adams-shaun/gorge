package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The chosen-source shield (Ext PreventNext with kop.PrevNextChosen; gorge
// effEffect's ChooseSource prevention family): "the next time a source of your
// choice would deal damage to you this turn, prevent that damage" (Circle of
// Protection: Red, Rune of Protection, Invulnerability). It is the
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
//	sc   prevNextSc
//	prevX  pxFrom|pxPlayers with the controller's seat in the rider's player
//	     bits; no rider kind, which no other record carries
//
// The source is matched by object, as gorge's ChosenCardStrict is by ObjID: a
// permanent that left the battlefield and came back, or the chosen spell
// resolved to a graveyard, is still the chosen object. It matches a source
// dealing damage as a permanent (its slot) or as the resolving spell or
// ability (a source that left the battlefield first).

// prevChosenLive is the live bit of a chosen-source record's col.
const prevChosenLive kop.ColorMask = 0x80

// DebugNoPrevChosen (the chosen-source lockstep mutation): the shield is
// installed but prevents nothing.
var DebugNoPrevChosen bool

// prevChosenAdd installs the shield on the chosen source; no source chosen (an
// empty pool) is no Effect.
func (s *State) prevChosenAdd(k *prep, in *kop.Instr) {
	if s.g.nchosen == 0 {
		return // no source chosen (an empty pool, or a stack ability: act_choosesource.go) is no Effect
	}
	mask := s.value(k, in.B)
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.col == 0 && r.sc == 0 {
			r.obj, r.col, r.sc, r.ctl = s.res.e.obj, prevChosenLive|kop.ColorMask(mask), prevNextSc, TCode(s.g.chosen[0])
			s.g.prevX[i] = pxFrom | pxPlayers | s.res.e.ctrl<<pxSeatShift
			return
		}
	}
	s.capFull(StallPrevCol) // loud: never drop an effect
}

// pnxChosenForm reports whether rider byte x is a chosen-source record's.
func pnxChosenForm(x uint8) bool { return x&(pxFrom|pxPlayers|pxRider) == pxFrom|pxPlayers }

// pnxChosenMatches reports whether chosen-source record i matches damage to
// the recipient code from the permanent at srcSl (NoSlot: the resolving spell
// or ability's source object).
func (s *State) pnxChosenMatches(i int, code TCode, srcSl Slot) bool {
	r := &s.g.prevCol[i]
	if DebugNoPrevChosen || !isPlayerTarget(code) || CodeSeat(code) != s.g.prevX[i]>>pxSeatShift {
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
	return TCode(src) == r.ctl && chosenSrcMatches(int(r.col&^prevChosenLive), cols, ty)
}
