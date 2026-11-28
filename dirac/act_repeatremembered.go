package dirac

import (
	"slices"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The remembered-set loop of OpRepeatEach (C kop.RepeatRemembered): gorge's
// api:RepeatEach DefinedCards$ DirectRemembered with UseImprinted$ True, the
// loop that follows a RememberDestroyed$ / RememberLKI$ mass effect ("for each
// permanent destroyed this way, its controller creates a token"). The body
// reads the bound card's controller as ImprintedController: the player ref
// RefEach, which in a card loop is eachCtrl.

// coldRemCtl is each remembered object's last battlefield controller, the
// capture a RememberLKI$ move or a RememberSacrificed$ sacrifice took (gorge's
// Snap.ChangeZone entry, lkiControllerFor), seat + 1 at the object's index in
// g.rem; 0 is no capture. g.tli holds the one latest snapshot only, so a
// move of several objects (Curse of the Swine's X creatures, each player's
// sacrifice) needs a slot per object. Only a table with a remembered-set loop,
// a RefOppNonRemCtrl read, a remembering SacrificeAll, or a remembering
// sacrifice beside a reader of the capture (tableReadsRemCtl) has the region
// (genPrep.anyRemEach, anyRemCtl): a capture nothing reads takes no State
// bytes. Resolution memory, in no view: each resolution starts it empty
// (resolveTop), as gorge's per-resolution Ctx.
var coldRemCtl = newCold[uint8]("remCtl", func(g *genPrep) int {
	if g.anyRemEach || g.anyRemCtl {
		return maxRem
	}
	return 0
})

// DebugRemCtlOff is TestRepeatEachRememberedLowers' mutation: eachCtrl ignores
// the RememberLKI$ capture and reads the owner. Tests only.
var DebugRemCtlOff bool

// remCtlNote records ctrl as the last controller of remembered object o (the
// one it had as it moved), when o is in the remembered set.
func (s *State) remCtlNote(o Obj, ctrl uint8) {
	c := coldRemCtl.of(s)
	if len(c) == 0 {
		return
	}
	for i := 0; i < int(s.g.nrem) && i < len(c); i++ {
		if s.g.rem[i] == o {
			c[i] = ctrl + 1
			return
		}
	}
}

// remCtlClear forgets the capture of remembered index i (a fresh object takes
// the slot).
func (s *State) remCtlClear(i int) {
	if c := coldRemCtl.of(s); i < len(c) {
		c[i] = 0
	}
}

// remCtlDrop closes up the captures after remembered index i of the n held was
// forgotten, as forgetRemGen closes up the set.
func (s *State) remCtlDrop(i, n int) {
	if c := coldRemCtl.of(s); i < len(c) {
		copy(c[i:], c[i+1:min(n, len(c))])
	}
}

// remCtl is the captured last controller of remembered object o, ok false when
// none was captured (o on the battlefield, or a move no RememberLKI$ /
// RememberSacrificed$ flag covered).
func (s *State) remCtl(o Obj) (uint8, bool) {
	if s.objs[o].zone == kop.ZoneBattlefield {
		return 0, false
	}
	c := coldRemCtl.of(s)
	for i := 0; i < int(s.g.nrem) && i < len(c); i++ {
		if s.g.rem[i] == o && c[i] != 0 {
			return c[i] - 1, true
		}
	}
	return 0, false
}

// eachCtrl is the controller of o, the card OpRepeatEach binds: its live
// controller on the battlefield, else the capture of its move or sacrifice
// (CR 608.2h), else its owner (the card moved with no capture, as gorge's
// graveyard object's controller reads: Move resets it to the owner, CR 400.7).
func (s *State) eachCtrl(o Obj) uint8 {
	if !DebugRemCtlOff {
		if c, ok := s.remCtl(o); ok {
			return c
		}
	}
	return s.ctrlOfObj(o)
}

// eachRememberedLoop reports whether the innermost loop frame of the
// resolution is a remembered-set card loop (kop.RepeatRemembered), whose
// RefEach, read as a damage recipient, is the bound card's controller. Only the
// plain frame has a loop stack entry; the batched loop's body is a Token.
func (s *State) eachRememberedLoop(k *prep) bool {
	r := &s.res
	if r.loopD == 0 {
		return false
	}
	in := &s.prog(k)[r.loopS[r.loopD-1]-1]
	return in.Op == kop.OpRepeatEach && in.C&kop.RepeatRemembered != 0
}

// tableReadsRemCtl reports whether t reads a remembered object's captured last
// controller other than through RefOppNonRemCtrl (prepProgOps): a condition
// over the remembered set whose filter may read the controller (matchRemGate),
// or a program that does (progReadsRemCtl).
func tableReadsRemCtl(t *kop.Table) bool {
	for i := range t.Conds {
		if c := &t.Conds[i]; c.Kind == kop.CondRefMatches && c.Ref == kop.RefRemembered && !filterCtrlFree(t, c.F) {
			return true
		}
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if progReadsRemCtl(t.Cards[i].Abilities[j].Prog) {
				return true
			}
		}
	}
	return false
}

// progReadsRemCtl reports whether one program reads the capture: the
// remembered controller as a player (remController), a copy for each player
// but a remembered permanent's controller (nonRemExcl), or a card loop whose
// bound card may be a sacrificed one (eachCtrl): a loop over a ref's cards, or
// a battlefield loop in a program that remembers a sacrifice.
func progReadsRemCtl(prog []kop.Instr) bool {
	sacs := slices.ContainsFunc(prog, func(in kop.Instr) bool { return in.Op == kop.OpSacrifice && in.B.N&1 != 0 }) // seat-ok: an instruction flag bit
	for _, in := range prog {
		switch {
		case in.Obj == kop.RefRememberedController || in.W == kop.RefRememberedController,
			in.Op == kop.OpCopyAsTokenEx && uint16(in.B.N)&kop.CopyNonRemCtrl != 0,
			in.Op == kop.OpRepeatEach && (sacs || in.Obj != kop.RefNone):
			return true
		}
	}
	return false
}

// filterCtrlFree reports whether every alternative of the filter f tests only
// what a card is wherever it lies (its types, colours, subtypes and the
// commander designation), so the controller it is matched under cannot change
// the answer. Any other field counts as a possible controller read, and so
// does no filter at all (f 0).
func filterCtrlFree(t *kop.Table, f kop.FilterID) bool {
	if f == 0 {
		return false
	}
	for ; f != 0; f = t.Filters[f].Or {
		fl := t.Filters[f]
		if fl.Pred == kop.PredWord7 && fl.PredN == kop.W7IsCommander {
			fl.Pred, fl.PredN = 0, 0
		}
		fl.Types, fl.NotTypes, fl.Colors, fl.NotColors = 0, 0, 0, 0
		fl.Sub, fl.NotSub, fl.NotSubs, fl.Or = 0, 0, kop.Filter{}.NotSubs, 0
		if fl != (kop.Filter{}) {
			return false
		}
	}
	return true
}

// prepProgOps is prepareGen's pass over one ability's program: an emblem
// grant's table entry, the remembered-set loop's cold region
// (genPrep.anyRemEach), which also sizes the remembered-searcher seat mask.
func (g *genPrep) prepProgOps(t *kop.Table, card CardID, a *kop.Ability) {
	for _, in := range a.Prog {
		if in.Op == kop.OpCreateEmblem {
			g.prepEmbGrant(t, card, &t.Emblems[in.C])
		}
		if in.Op == kop.OpRepeatEach && in.C&kop.RepeatRemembered != 0 {
			g.anyRemEach = true
		}
		if in.Obj == kop.RefOppNonRemCtrl || in.Op == kop.OpSacrificeAll && in.C&kop.MvRemember != 0 {
			g.anyRemCtl = true // an OpSacrifice's remember bit: prepareGen, beside anyRemSac
		}
		if in.Op == kop.OpExt && in.A.K == kop.VConst && in.A.N == int16(kop.ExtRememberSearcher) {
			g.anyRemSearcher = true
		}
	}
}
