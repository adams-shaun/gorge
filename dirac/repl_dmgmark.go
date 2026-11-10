package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A per-player damage-doubling mark (kop.ExtDmgMark: Lightning, Army of One's
// Stagger). The resolving Effect marks the player its trigger captured: until
// the Effect's controller's next turn, every damage event to that player or a
// permanent they control is doubled (gorge registers an Effect DamageDone
// replacement, Duration$ UntilYourNextTurn, ValidSource$ any).
//
// genState.dmk holds one bit per (controller, marked player) pair: bit
// c*MaxSeats+p, taken from rsv (one byte for two seats, two for four). The
// controller's pairs end at the cleanup before their next turn begins, as
// dropNextTurnEmblems and dropNextTurnOv end their records. Two controllers'
// marks on one player are two replacements (x4); the same controller marking
// the same player again before it ends would be a second replacement of one
// pair, which one bit cannot hold, so it stalls (StallDmgMark).
//
// The mark competes with other damage replacements only by commuting
// doublings, which gorge orders by a CR 616.1 ask over object candidates; the
// mark is no object, so an event the mark and another replacement both match
// stalls (StallDmgMarkOrder) rather than answer without that ask.

const dmkBytes = (MaxSeats*MaxSeats + 7) / 8 // genState.dmk: taken from rsv

// StallDmgMark is a second mark of one controller on one player while the
// first is live (genState.dmk).
const StallDmgMark = 156

// StallDmgMarkOrder is a damage event the mark and another replacement both
// match: gorge asks the CR 616.1 order, which the mark has no candidate for.
const StallDmgMarkOrder = 157

func init() {
	stallNames[StallDmgMark] = "dmgmark"
	stallNames[StallDmgMarkOrder] = "dmgmarkorder"
}

// prepDmgMark sets genPrep.anyDmgMark and opens every damage site (anyRepl
// carries rbDblDmg; no card has the bit, so the per-permanent walks find none).
func prepDmgMark(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() == kop.ExtDmgMark {
					g.anyDmgMark = true
				}
			}
		}
	}
	if g.anyDmgMark {
		g.anyRepl |= rbDblDmg
	}
}

// dmkBit is the pair (controller c, marked player p)'s position in genState.dmk.
func dmkBit(c, p uint8) (idx int, mask uint8) {
	b := int(c)*MaxSeats + int(p)
	return b >> 3, 1 << (b & 7)
}

// dmgMarkAdd is Ext DmgMark: mark the player in.Obj names for the resolving
// controller.
//
//go:noinline
func (s *State) dmgMarkAdd(k *prep, in *kop.Instr) {
	p := s.playerRef(k, in.Obj)
	c := s.res.e.ctrl
	if p >= MaxSeats || c >= MaxSeats {
		return
	}
	i, m := dmkBit(c, p)
	if s.g.dmk[i]&m != 0 {
		s.capFull(StallDmgMark)
		return
	}
	s.g.dmk[i] |= m
}

// dmgMarkN is the number of live marks on player p: the doublings every damage
// event to p or a permanent p controls takes. Callers gate on anyDmgMark.
func (s *State) dmgMarkN(p uint8) int {
	if p >= MaxSeats {
		return 0
	}
	n := 0
	for c := uint8(0); c < MaxSeats; c++ {
		if i, m := dmkBit(c, p); s.g.dmk[i]&m != 0 {
			n++
		}
	}
	return n
}

// dmgMarkEnd ends the marks of the next active player: this cleanup is the one
// before their next turn begins.
//
//go:noinline
func (s *State) dmgMarkEnd() {
	c := s.nextActive()
	for q := uint8(0); q < MaxSeats; q++ {
		i, m := dmkBit(c, q)
		s.g.dmk[i] &^= m
	}
}

// dmgMarkCompetes reports (and stalls) an event to player rcp that the mark
// and cands other replacements match: two or more matches are gorge's CR 616.1
// order ask.
func (s *State) dmgMarkCompetes(rcp uint8, cands int) bool {
	if n := s.dmgMarkN(rcp); n > 0 && n+cands >= 2 {
		s.capFull(StallDmgMarkOrder)
		return true
	}
	return false
}
