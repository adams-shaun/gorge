package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A per-player damage-doubling mark (kop.ExtDmgMark: Lightning, Army of One's
// Stagger). The resolving Effect marks the player its trigger captured: until
// the Effect's controller's next turn, every damage event to that player or a
// permanent they control is doubled (gorge registers an Effect DamageDone
// replacement, Duration$ UntilYourNextTurn, ValidSource$ any).
//
// genState.dmk holds one bit per (controller, marked player) pair: bit
// c*MaxSeats+p; then a 2-bit expiry per controller (dmkQ), taken from rsv
// (one byte for two seats, three for four). The controller's pairs end at the
// cleanup before their next turn begins, as dropNextTurnEmblems and
// dropNextTurnOv end their records. A controller still in the game has that
// cleanup re-derived at every cleanup (gorge's rescheduleNextTurnBoundaries);
// one that has left keeps the boundary frozen at the last cleanup it was in
// (nextTurnFor returns 0 for it), and the mark ends at that cleanup all the
// same: dmkQ counts the cleanups left after the one in progress. Two
// controllers' marks on one player are two replacements (gorge asks their
// CR 616.1 order), so every path that would double twice stalls
// (StallDmgMarkOrder); the same controller marking the same player again
// before it ends would be a second replacement of one pair, which one bit
// cannot hold, so it stalls (StallDmgMark).
//
// The mark competes with other damage replacements only by commuting
// doublings, which gorge orders by a CR 616.1 ask over object candidates; the
// mark is no object, so an event the mark and another replacement both match
// stalls (StallDmgMarkOrder) rather than answer without that ask.

const dmkBytes = (MaxSeats*MaxSeats + 3*MaxSeats + 7) / 8 // genState.dmk: taken from rsv

// StallDmgMark is a second mark of one controller on one player while the
// first is live (genState.dmk).
const StallDmgMark = 157

// StallDmgMarkOrder is a damage event the mark and another replacement both
// match: gorge asks the CR 616.1 order, which the mark has no candidate for.
const StallDmgMarkOrder = 158

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

// dmkQ is controller c's expiry counter's byte and shift in genState.dmk: the
// cleanups left after the one in progress before the cleanup the marks end at
// (at most 3: four seats).
func dmkQ(c uint8) (idx int, sh uint) {
	b := MaxSeats*MaxSeats + 2*int(c)
	return b >> 3, uint(b & 7)
}

// DebugNoDmgSrcMark (the source-mark lockstep mutation): the source marks are
// asked for and never set.
var DebugNoDmgSrcMark bool

// dmkSrc is controller c's source-mark bit in genState.dmk (kop.DmgMarkBySource):
// after the pair bits and the expiry counters; it ends at the cleanup.
func dmkSrc(c uint8) (idx int, mask uint8) {
	b := MaxSeats*MaxSeats + 2*MaxSeats + int(c)
	return b >> 3, 1 << (b & 7)
}

// srcMarkN is the number of live source marks of controller c: the doublings
// every damage event dealt by a source c controls takes (0 or 1).
func (s *State) srcMarkN(c uint8) int {
	if c >= MaxSeats {
		return 0
	}
	if i, m := dmkSrc(c); s.g.dmk[i]&m != 0 {
		return 1
	}
	return 0
}

// dmgMarkEv is the marks an event dealt by a source controlled by src to
// recipient controller rcp takes: the recipient's pair marks and the source
// controller's source mark.
func (s *State) dmgMarkEv(rcp, src uint8) int { return s.dmgMarkN(rcp) + s.srcMarkN(src) }

// dmkLive reports whether controller c has a live mark.
func (s *State) dmkLive(c uint8) bool {
	for p := uint8(0); p < MaxSeats; p++ {
		if i, m := dmkBit(c, p); s.g.dmk[i]&m != 0 {
			return true
		}
	}
	return false
}

// turnsTo is how many turns from the one in progress controller c's next turn
// is (gorge nextTurnFor - the turn now): 1 when it is the next turn (an extra
// turn of the active player first), 0 when c is out of the rotation.
func (s *State) turnsTo(c uint8) uint8 {
	ex := s.pl[s.active].extraTurns
	if c == s.active && ex > 0 {
		return 1
	}
	d := ex
	q := s.active
	for i := 0; i < MaxSeats; i++ {
		q = s.nextAlive(q)
		d++
		if q == c {
			return d
		}
	}
	return 0
}

// dmgMarkAdd is Ext DmgMark: mark the player in.Obj names for the resolving
// controller.
//
//go:noinline
func (s *State) dmgMarkAdd(k *prep, in *kop.Instr) {
	c := s.res.e.ctrl
	if in.C&kop.DmgMarkBySource != 0 {
		if c >= MaxSeats {
			return
		}
		i, m := dmkSrc(c)
		if DebugNoDmgSrcMark {
			return
		}
		if s.g.dmk[i]&m != 0 {
			s.capFull(StallDmgMark) // a second doubling of the same sources: gorge asks their order
			return
		}
		s.g.dmk[i] |= m
		return
	}
	p := s.playerRef(k, in.Obj)
	if p >= MaxSeats || c >= MaxSeats {
		return
	}
	i, m := dmkBit(c, p)
	if s.g.dmk[i]&m != 0 {
		s.capFull(StallDmgMark)
		return
	}
	s.g.dmk[i] |= m
	// The boundary is the turn before c's next turn: that many cleanups from
	// this turn's, this turn's the first (gorge UntilTurn = nextTurnFor - 1).
	qi, sh := dmkQ(c)
	q := uint8(0)
	if d := s.turnsTo(c); d > 1 {
		q = min(d-1, 3)
	}
	s.g.dmk[qi] = s.g.dmk[qi]&^(3<<sh) | q<<sh
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

// dmgMarkTotal is the number of live marks on any player: the sweep guard's
// conservative count of the replacements the marks bring to some hit.
func (s *State) dmgMarkTotal() int {
	n := 0
	for c := uint8(0); c < MaxSeats; c++ {
		n += s.srcMarkN(c)
	}
	for p := uint8(0); p < MaxSeats; p++ {
		n += s.dmgMarkN(p)
	}
	return n
}

// dmgMarkEnd is the cleanup's turn over the marks: a controller's end at the
// cleanup before their next turn begins. One still in the game ends them when
// that turn is the next, else re-derives the count; one that has left ends
// them when the count it froze runs out.
//
//go:noinline
func (s *State) dmgMarkEnd() {
	for c := uint8(0); c < MaxSeats; c++ {
		i, m := dmkSrc(c)
		s.g.dmk[i] &^= m // the source marks last the turn
	}
	for c := uint8(0); c < MaxSeats; c++ {
		if !s.dmkLive(c) {
			continue
		}
		qi, sh := dmkQ(c)
		q := s.g.dmk[qi] >> sh & 3
		if s.out(c) {
			if q > 0 {
				s.g.dmk[qi] = s.g.dmk[qi]&^(3<<sh) | (q-1)<<sh
				continue
			}
		} else if d := s.turnsTo(c); d > 1 {
			s.g.dmk[qi] = s.g.dmk[qi]&^(3<<sh) | min(d-2, 3)<<sh
			continue
		}
		for p := uint8(0); p < MaxSeats; p++ {
			i, m := dmkBit(c, p)
			s.g.dmk[i] &^= m
		}
		s.g.dmk[qi] &^= 3 << sh
	}
}

// dmgMarkCompetes reports (and stalls) an event to player rcp that the mark
// and cands other replacements match: two or more matches are gorge's CR 616.1
// order ask.
func (s *State) dmgMarkCompetes(rcp uint8, cands int) bool {
	return s.dmgMarkCompetesEv(rcp, MaxSeats, cands)
}

// dmgMarkCompetesEv is dmgMarkCompetes for an event dealt by a source
// controlled by src (MaxSeats: none), which the source mark also matches.
func (s *State) dmgMarkCompetesEv(rcp, src uint8, cands int) bool {
	if n := s.dmgMarkEv(rcp, src); n > 0 && n+cands >= 2 {
		s.capFull(StallDmgMarkOrder)
		return true
	}
	return false
}

// dmgMarkDirect is dmgMarkCompetes for replDamage, the direct damage sites'
// resolver: the other matches are the doublers dblDmgN2 counts beside the
// marks and a recipient creature's controller's other noncombat-preventing
// permanent (its own replacement walk below).
func (s *State) dmgMarkDirect(k *prep, m, others int, rcp uint8, sl Slot, combat bool) bool {
	c := others
	if k.gen.anyRepl&rbPrevNC != 0 && !combat && sl != NoSlot && s.isCreature(k, sl) {
		for q := s.statM.and(s.ctrlM[rcp]); q.any(); q = q.rest() {
			b := q.first()
			if b != sl && k.gen.replBits[s.cardAt(b)]&rbPrevNC != 0 && !s.genLost(k, b) {
				c++
				break
			}
		}
	}
	if m+c >= 2 {
		s.capFull(StallDmgMarkOrder)
		return true
	}
	return false
}
