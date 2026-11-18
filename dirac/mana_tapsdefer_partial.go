package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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
// prefixes, deepest first, each with every gap's first answer.

// scriptPartialResolve is scriptDFS's last resort for a board with Taps-deferred
// gaps: after every line failed, the prefixes order[:d] (d < len(order)) whose
// taps queued triggers are resolved, and the first that leaves the game
// running with the play offered is the script. It reports the prefix's depth,
// and sets *dead as scriptDFS does for a found line.
//
//go:noinline
func (s *State) scriptPartialResolve(k *prep, a Action, p uint8, order []Slot, ctr, restr, anyR, tapsDef, ca slotMask, dead *bool) (int, bool) {
	if !tapsDef.any() {
		return 0, false
	}
	noGrant := a.Kind() == AkActivate && s.restrictedGrantLive(k, p)
	for depth := len(order) - 1; depth >= 1; depth-- {
		t := s.fork()
		t.g.noGrant = noGrant
		var spill [kop.NumManaKinds]uint8
		ok := true
		for j := 0; j < depth && ok; j++ {
			ok = t.scriptGapTap(k, p, a, order[j], ctr, restr, anyR, tapsDef, ca, 0, &spill)
		}
		found := false
		if ok && t.npend != 0 {
			t.hideDeferred(k, p, a, tapsDef)
			var d Decision
			t.restrOptions(k, &d, p)
			resolve := t.playNeedsEmptyStack(k, a)
			if resolve && hasOption(&d, a) && !t.foreignPendAsks(k, p, resolve) {
				t.resolvePrefixPlay(k, a, p, t.sp, resolve)
				if !t.over {
					t.restrOptions(k, &d, p)
					found = hasOption(&d, a)
				}
			}
		}
		if found {
			*dead = false
			if t.rbAny(SeatIx(p)) || spill != ([kop.NumManaKinds]uint8{}) {
				for c := range spill {
					t.pl[SeatIx(p)].pool[c] -= spill[c]
				}
				*dead = !t.scriptCompletes(k, p, a)
			}
		}
		s.unfork()
		if found {
			return depth, true
		}
	}
	return 0, false
}
