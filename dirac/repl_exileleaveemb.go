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

// useEmbExileAny: the table has an emblem carrying an any-zone
// dies-to-exile replacement (an Effect's "if a card would be put into a
// graveyard from anywhere, exile it instead"; kgen's
// lower_effectgravetoexile.go).
var useEmbExileAny = newGenUse(tableEmbHasExileAny)

// tableEmbHasExileLeave: some emblem of t carries a battlefield-origin
// dies-to-exile replacement (Repl.C 1 is the any-zone arm).
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

// tableEmbHasExileAny: some emblem of t carries an any-zone dies-to-exile
// replacement (Repl.C 1).
func tableEmbHasExileAny(t *kop.Table) bool {
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			a := &t.Emblems[i].Abilities[j]
			if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplExileOnLeave && a.Repl.C == 1 {
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

// prepEmbExileAny opens every death site and every placeFrom move when some
// emblem carries an any-zone dies-to-exile replacement (prep.embExileAny):
// the death sites through g.anyRepl, the placeFrom gate through
// prep.embExileAny itself (a printed any-zone host carries its bits on
// genPrep.exileAnyC; the emblem is created at resolution, no card does).
func prepEmbExileAny(p *prep, g *genPrep) {
	if p.embExileAny {
		g.anyRepl |= rbExileOnLeave
	}
}

// prepEmbRepls folds the emblem-carried replacements' need bits: the static
// damage prevention's (repl_staticpreventemblem.go) and this family's.
func prepEmbRepls(p *prep, g *genPrep) {
	prepEmbPrev(p, g)
	prepEmbExileLeave(p, g)
	prepEmbExileAny(p, g)
}

// embExileLeaveN counts the matching dies-to-exile replacements of every live
// emblem of every seat (eachEmbPrev's walk: a departing seat's emblem no
// longer applies, CR 800.4a). Both forms count at a battlefield death: a
// battlefield-origin replacement (Repl.C != 1) and the any-zone arm
// (Repl.C == 1) alike.
func (s *State) embExileLeaveN(k *prep, sl Slot) int {
	if !k.embExileLeave && !k.embExileAny {
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
				if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplExileOnLeave &&
					s.matchPerm(k, a.Repl.Filter, sl, p, NoSlot) {
					n++
				}
			}
		}
	}
	return n
}

// embExileAnyN counts the any-zone dies-to-exile replacements (Repl.C 1) of
// every live emblem of every seat over card c (object o, owned by owner)
// moving to a graveyard from a non-battlefield zone (exileAnyN's walk over
// emblems). An emblem record carries no counter and no rider (Repl.A 0), so
// only the count is read.
func (s *State) embExileAnyN(k *prep, o Obj, c CardID, owner uint8) int {
	if !k.embExileAny {
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
				if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplExileOnLeave && a.Repl.C == 1 &&
					s.matchCardGen(k, a.Repl.Filter, c, owner, p, o) {
					n++
				}
			}
		}
	}
	return n
}
