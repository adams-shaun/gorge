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
				switch in.Ext() {
				case kop.ExtPreventColor, kop.ExtPreventDamageScope, kop.ExtPreventNext:
					g.anyRepl |= rbPrevCol
				case kop.ExtDmgRedirect:
					g.anyRepl |= rbPrevCol // opens every damage site; the records are genState.redir's
					g.anyRedir = true
				}
			}
		}
	}
}

// prevColAt is the index (registration order) of the first record that
// prevents the damage from the source (slot sl, else card c) to t this turn
// (CR 615.1a: the source's colour as the damage would be dealt), -1 for none.
// It does not read CR 615.12 (noPrev).
func (s *State) prevColAt(k *prep, sl Slot, c CardID, t prevTgt, combat bool) int {
	for i := range s.g.prevCol {
		if s.prevRecHit(k, i, sl, c, t, combat) {
			return i
		}
	}
	return -1
}

// prevRecHit reports whether record i prevents the damage: an Ext
// PreventColor record by the source's colour, a scoped one by its scope (a
// scoped record's col filters its sources, a depleting shield's is its pool:
// repl_prevnext.go).
func (s *State) prevRecHit(k *prep, i int, sl Slot, c CardID, t prevTgt, combat bool) bool {
	r := &s.g.prevCol[i]
	if r.sc == 0 {
		return r.col != 0 && !DebugNoPrevCol && s.srcColors(k, sl, c)&r.col != 0
	}
	var sc kop.ColorMask
	if r.col != 0 {
		sc = s.srcColors(k, sl, c) // only a colour-filtered record reads it
	}
	return s.prevScopeHit(k, r, sl, c, sc, t, combat)
}

// prevColHit reports whether a record prevents the damage (prevColAt) and the
// damage can be prevented (CR 615.12). Two or more such records on one event
// are the CR 616.1 order ask (prevColCands).
func (s *State) prevColHit(k *prep, sl Slot, c CardID, t prevTgt, combat bool) bool {
	return s.prevColAt(k, sl, c, t, combat) >= 0 && !s.noPrev(k, combat)
}

// prevColRiderHit reports whether a matching colour or scope record carries a
// rider body (pxRider: the gain or token a prevention runs, repl_prevx.go).
// Such a record is not a bare prevention however the event is ordered
// (gorge's allBarePreventions requires With == nil), so a bare unbounded
// shield beside it does pose the CR 616.1 ask. The shield's own riders are
// read in prevNextTake; under its bare flag every matching shield is
// riderless (pnxBare).
func (s *State) prevColRiderHit(k *prep, sl Slot, c CardID, t prevTgt, combat bool) bool {
	for i := range s.g.prevCol {
		if s.g.prevX[i]&pxRider != 0 && s.prevRecHit(k, i, sl, c, t, combat) {
			return true
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
func (s *State) prevColCands(k *prep, col kop.ColorMask, srcSl Slot, srcCard CardID, t prevTgt, combat bool, cand *[4]Obj, n int, _ *[4]Obj, pm *uint8) int {
	for i := range s.g.prevCol {
		r := &s.g.prevCol[i]
		if r.sc != 0 {
			if !s.prevScopeHit(k, r, srcSl, srcCard, col, t, combat) {
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
