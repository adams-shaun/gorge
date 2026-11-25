package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The remembered-set loop of OpRepeatEach (C kop.RepeatRemembered): gorge's
// api:RepeatEach DefinedCards$ DirectRemembered with UseImprinted$ True, the
// loop that follows a RememberDestroyed$ / RememberLKI$ mass effect ("for each
// permanent destroyed this way, its controller creates a token"). The body
// reads the bound card's controller as ImprintedController: the player ref
// RefEach, which in a card loop is eachCtrl.

// coldRemCtl is each remembered object's last battlefield controller, the
// RememberLKI$ capture a move off the battlefield took (gorge's
// Snap.ChangeZone entry, lkiControllerFor), seat + 1 at the object's index in
// g.rem; 0 is no capture. g.tli holds the one latest snapshot only, so a
// move of several targets (Curse of the Swine's X creatures) needs a slot per
// object. Only a table with a remembered-set loop has the region
// (genPrep.anyRemEach). Not hashed: the capture and its read are one
// resolution and no ask sits between them.
var coldRemCtl = newCold[uint8]("remCtl", func(g *genPrep) int {
	if g.anyRemEach {
		return maxRem
	}
	return 0
})

// DebugRemCtlOff is TestRepeatEachRememberedLowers' mutation: eachCtrl ignores
// the RememberLKI$ capture and reads the owner. Tests only.
var DebugRemCtlOff bool

// remCtlNote records ctrl as the last controller of remembered object o (its
// RememberLKI$ snapshot's), when o is in the remembered set.
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

// eachCtrl is the controller of o, the card OpRepeatEach binds: its live
// controller on the battlefield, else the RememberLKI$ capture of its move
// (CR 608.2h), else its owner (the card moved with no capture, as gorge's
// graveyard object's controller reads: Move resets it to the owner, CR 400.7).
func (s *State) eachCtrl(o Obj) uint8 {
	if s.objs[o].zone != kop.ZoneBattlefield && !DebugRemCtlOff {
		c := coldRemCtl.of(s)
		for i := 0; i < int(s.g.nrem) && i < len(c); i++ {
			if s.g.rem[i] == o && c[i] != 0 {
				return c[i] - 1
			}
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
	}
}
