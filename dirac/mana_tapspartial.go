package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A Taps-deferred gap whose trigger hurts its payer (Scald's "whenever a
// player taps an Island for mana, Scald deals 1 damage to that player") queues
// one trigger per tap, and a sorcery-speed play waits for the stack (CR 307.1,
// 602.5d). gorge's prefix search (rules scriptSearch.visit) works down the gaps
// first, so the lines that tap every gap come first, and a resolution that
// ends the game (CR 704.5a) fails the line; it then backtracks, and a shorter
// prefix whose triggers leave the payer alive, the pool floating, is the one
// that offers the play. scriptDFS's node check resolves the stack only at the
// full depth, so it dropped such a play (Fallowsage at two life beside three
// Islands under Scald): this is the search's fallback over the shorter
// prefixes.
//
// The search is bounded: potentialScriptNodes clones, one per try, over the
// subsets of the gaps the prefix has tapped and the triggers it has let
// resolve. Ten Lands under Manabarbs never reach the shorter prefix inside
// it, and gorge offers nothing (Spiked Corridor at five life, CR 307.1), so
// the fallback walks gorge's tree on counts (gapLattice) and takes the
// prefix gorge's walk ends at, or none when its budget runs out first.

// gapLattice is gorge's prefix search over n identical gaps, one trigger per
// tap, each resolution costing one life: a node is (gaps tapped, triggers
// pending, life left), counted before its dedupe as scriptSearch.visit counts
// it, visited once, and a line is done when the stack is empty and a prefix of
// that many gaps is a play (good, one bit per size).
type gapLattice struct {
	n       int
	good    uint32
	nodes   int
	seen    [potentialScriptNodes]uint64
	limited bool
}

// visit is scriptSearch.visit on counts: it returns the size of the prefix
// that pays, 0 for none. The children are the untapped gaps, then the pass
// that resolves the top trigger (potential_script.go).
func (g *gapLattice) visit(m uint32, pend, life int) int {
	if g.nodes >= len(g.seen) {
		g.limited = true
		return 0
	}
	g.nodes++
	if life <= 0 {
		return 0
	}
	key := uint64(m)<<16 | uint64(pend)<<8 | uint64(life)
	for _, v := range g.seen[:g.nodes-1] {
		if v == key+1 {
			return 0
		}
	}
	g.seen[g.nodes-1] = key + 1
	if sz := bits.OnesCount32(m); pend == 0 && m != 0 && g.good&(1<<sz) != 0 {
		return sz
	}
	for i := 0; i < g.n; i++ {
		if m&(1<<i) == 0 {
			if sz := g.visit(m|1<<i, pend+1, life); sz != 0 {
				return sz
			}
		}
	}
	if pend > 0 {
		return g.visit(m, pend-1, life-1)
	}
	return 0
}

// scriptPartialResolve is scriptDFS's last resort for a board with Taps-deferred
// gaps: after every line failed, the prefixes order[:d] whose taps queued
// triggers (one each) are resolved, and the first prefix gorge's budgeted walk
// reaches, with the play offered and the game running, is the script. It
// reports the prefix's depth, and sets *dead as scriptDFS does for a found
// line. A play that does not wait for the stack, or a gap whose tap queues
// other than one trigger, is outside the walk's counts: no script.
//
//go:noinline
func (s *State) scriptPartialResolve(k *prep, a Action, p uint8, order []Slot, ctr, restr, anyR, tapsDef, ca slotMask, dead *bool) (int, bool) {
	n := len(order)
	if !tapsDef.any() || n < 2 || n > 30 {
		return 0, false
	}
	noGrant := a.Kind() == AkActivate && s.restrictedGrantLive(k, p)
	var lat gapLattice
	lat.n = n
	alive := 0 // gaps whose triggers all resolve with the payer alive
	for d := 1; d <= n; d++ {
		ok, live, good := s.partialProbe(k, a, p, order, d, ctr, restr, anyR, tapsDef, ca, noGrant, new(bool))
		if !ok {
			return 0, false
		}
		if live {
			alive = d
			if good {
				lat.good |= 1 << d
			}
		}
	}
	size := lat.visit(0, 0, alive+1)
	if size == 0 || size >= n {
		return 0, false
	}
	s.partialProbe(k, a, p, order, size, ctr, restr, anyR, tapsDef, ca, noGrant, dead)
	return size, true
}

// partialProbe taps the first d gaps of order on a copy, lets their triggers
// resolve, and reports whether the line is inside the walk's counts (ok), the
// game still runs (live) and the play is offered then (good, with *dead set
// as scriptDFS sets it for a found line).
func (s *State) partialProbe(k *prep, a Action, p uint8, order []Slot, d int, ctr, restr, anyR, tapsDef, ca slotMask, noGrant bool, dead *bool) (ok, live, good bool) {
	t := s.fork()
	defer s.unfork()
	t.g.noGrant = noGrant
	var spill [kop.NumManaKinds]uint8
	for j := 0; j < d; j++ {
		if !t.scriptGapTap(k, p, a, order[j], ctr, restr, anyR, tapsDef, ca, 0, &spill) {
			return false, false, false
		}
	}
	if int(t.npend) != d {
		return false, false, false
	}
	t.hideDeferred(k, p, a, tapsDef)
	var dec Decision
	t.restrOptions(k, &dec, p)
	resolve := t.playNeedsEmptyStack(k, a)
	if !resolve {
		return false, false, false
	}
	offered := hasOption(&dec, a) && !t.foreignPendAsks(k, p, resolve)
	t.resolvePrefixPlay(k, a, p, t.sp, resolve)
	if t.over {
		return true, false, false
	}
	t.restrOptions(k, &dec, p)
	if !offered || !hasOption(&dec, a) {
		return true, true, false
	}
	*dead = false
	if t.rbAny(SeatIx(p)) || spill != ([kop.NumManaKinds]uint8{}) {
		for c := range spill {
			t.pl[SeatIx(p)].pool[c] -= spill[c]
		}
		*dead = !t.scriptCompletes(k, p, a)
	}
	return true, true, true
}
