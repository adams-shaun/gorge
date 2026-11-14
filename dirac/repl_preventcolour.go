package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// prevTgt is the recipient of a damage event: the player rcp it hits (dst
// NoSlot) or the permanent at dst whose controller is rcp.
type prevTgt struct {
	rcp uint8
	dst Slot
}

// raPrevColor: Ext PreventColor's colour ask (ChGenColor, mode = W U B R G).
const raPrevColor uint8 = 251

// prepPrevCol turns the damage hook on for a table with an Ext
// PreventColor, PreventDamageScope or PreventNext instruction.
func prepPrevCol(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if e := in.Ext(); e == kop.ExtPreventColor || e == kop.ExtPreventDamageScope || e == kop.ExtPreventNext {
					g.anyRepl |= rbPrevCol
					return
				}
			}
		}
	}
}

// prevColHit reports whether the damage source (slot sl, else card c) has
// a colour an Ext PreventColor record prevents this turn (CR 615.1a: the
// source's colour as the damage would be dealt). Two or more such records
// on one event are the CR 616.1 order ask (prevColCands).
func (s *State) prevColHit(k *prep, sl Slot, c CardID, t prevTgt, combat bool) bool {
	var m kop.ColorMask
	for i := range s.g.prevCol {
		if s.g.prevCol[i].sc&prevNextSc == 0 {
			m |= s.g.prevCol[i].col // a depleting shield's col is its pool (repl_prevnext.go)
		}
	}
	if m != 0 && s.srcColors(k, sl, c)&m != 0 && !DebugNoPrevCol {
		return !s.noPrev(k, combat) // CR 615.12
	}
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.sc != 0 && s.prevScopeHit(k, r.sc, uint8(r.ctl), sl, t) { // code-ok: a scoped record's ctl is a seat
			return !s.noPrev(k, combat)
		}
	}
	return false
}

// DebugPrevColRecolor (TestStrandsLockstepRecolorMutation): a card
// resolving again moves its earlier records to the new colour, gorge's
// reading before 690ed17f0.
var DebugPrevColRecolor bool

// DebugNoPrevCol (TestStrandsLockstepMutation): the chosen colour is asked
// and recorded but nothing is prevented.
var DebugNoPrevCol bool

// prevColCands appends the Ext PreventColor records preventing damage from a
// source of colours col to cand after its first n entries (gorge collects
// each Effect's replacement as its own candidate, in registration order),
// marking each in *pm; -1 when cand overflows (a loud stall).
func (s *State) prevColCands(k *prep, col kop.ColorMask, srcSl Slot, t prevTgt, combat bool, cand *[4]Obj, n int, _ *[4]Obj, pm *uint8) int {
	for i := range s.g.prevCol {
		r := &s.g.prevCol[i]
		if r.sc != 0 {
			if !s.prevScopeHit(k, r.sc, uint8(r.ctl), srcSl, t) { // code-ok: a scoped record's ctl is a seat
				continue
			}
		} else if r.col == 0 || r.col&col == 0 || DebugNoPrevCol {
			continue
		}
		if s.noPrev(k, combat) {
			return n // CR 615.12: no record prevents
		}
		if n == len(cand) {
			s.capFull(StallReplCands) // loud: never truncate the candidates
			return -1
		}
		cand[n] = r.obj
		*pm |= 1 << n
		n++
	}
	return n
}
