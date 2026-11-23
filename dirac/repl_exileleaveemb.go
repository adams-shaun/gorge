package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A dies-to-exile replacement an emblem carries (Cry of the Carnarium's
// DeathRep Effect, kgen's lower_effectreplemblem.go: "If a creature would die
// this turn, exile it instead", no Duration$). The emblem is the runtime
// emblem of OpCreateEmblem (State.g.nemb) with the UntilEOT flag, so
// cleanupGen drops it at the turn's end; State does not grow. exileOnLeaveN
// (dieZone) walks every live emblem after the battlefield and matches an
// emblem ability with "you" the emblem's seat and no host slot: the emblem's
// Repl.A is 0, so a match counts one death with no counter and a NoSlot
// source (no rider to run after the move).
//
// A candidate beside another matching dies-to-exile source (a card, or a
// second Effect's emblem) is gorge's CR 616.1 order; the kernel never orders
// an emblem, so dieZone's two-or-more count stalls StallReplLeave (the
// repl_staticpreventemblem.go precedent). The parked-move sites keep that
// rule: graveMoveCompetes never parks a move an emblem matches (the ask's
// source list names only cards, so a parked ask beside an emblem would
// silently skip it; the unparked death reaches dieZone and stalls by name).

// useEmbExileLeave: the table has an emblem carrying a dies-to-exile
// replacement.
var useEmbExileLeave = newGenUse(tableEmbHasExileLeave)

// tableEmbHasExileLeave: some emblem of t carries a battlefield-origin
// dies-to-exile replacement (Repl.C 1 is the any-zone arm, exileAnyN's, which
// kgen refuses for this shape).
func tableEmbHasExileLeave(t *kop.Table) bool {
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			a := &t.Emblems[i].Abilities[j]
			if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplExileOnLeave && a.Repl.C != 1 {
				return true
			}
		}
	}
	return false
}

// prepEmbExileLeave opens every death site when some emblem carries a
// dies-to-exile replacement (prep.embExileLeave), the way a printed
// dies-to-exile host does (no card carries the bit: the emblem is created at
// resolution).
func prepEmbExileLeave(p *prep, g *genPrep) {
	if p.embExileLeave {
		g.anyRepl |= rbExileOnLeave
	}
}

// prepEmbRepls folds the emblem-carried replacements' need bits: the static
// damage prevention's (repl_staticpreventemblem.go) and this family's.
func prepEmbRepls(p *prep, g *genPrep) {
	prepEmbPrev(p, g)
	prepEmbExileLeave(p, g)
}

// embExileLeaveN counts the matching dies-to-exile replacements of every live
// emblem of every seat (eachEmbPrev's walk: a departing seat's emblem no
// longer applies, CR 800.4a).
func (s *State) embExileLeaveN(k *prep, sl Slot) int {
	if !k.embExileLeave {
		return 0
	}
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		for i := 0; i < int(s.g.nemb[p]); i++ {
			e := &k.t.Emblems[s.embIdx(p, i)-1]
			for j := range e.Abilities {
				a := &e.Abilities[j]
				if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplExileOnLeave && a.Repl.C != 1 &&
					s.matchPerm(k, a.Repl.Filter, sl, p, NoSlot) {
					n++
				}
			}
		}
	}
	return n
}
