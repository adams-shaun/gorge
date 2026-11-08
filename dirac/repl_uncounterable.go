package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplSpellsCantBeCountered (Chimil, the Inner Sun,
// Sphinx of the Final Word): a battlefield "spells you control can't be
// countered" replacement. gorge stops the Counter effect's event
// (counterReplacementMatchesAll, effCounter -> CounterAllowed). The Counter
// effect and ward (effWard; both through effects.counterStopped) are such
// events: a bounce or a move of a spell off the stack (changeZone) and a
// fizzle on illegal targets (CR 608.2b) never ask it, so those kernel sites
// stay as they were.

// StallUncounterOrder: two or more "can't be countered" replacements match
// one Counter event (this spell's own plus a battlefield source, or two
// sources). gorge CounterAllowed parks every multi-match on the CR 616.1
// order ask (replacement_choice.go replChoiceCounter), though all candidates
// only stop the event; the kernel does not pose that ask, so the game
// stalls loudly rather than diverge (GORGE-FINDINGS.md).
const StallUncounterOrder = 119

func init() { stallNames[StallUncounterOrder] = "uncounterorder" }

// DebugUncounterOff skips the battlefield scan (the mutation: the kernel
// counters a spell gorge refuses to).
var DebugUncounterOff bool

// DebugUncounterStats counts, for the tests (a table with the kind only):
// UncounterHeld the spells the battlefield scan protected from a Counter,
// UncounterPassed those it let through (the Counter went on),
// UncounterStalls the multi-match events that stalled.
var (
	DebugUncounterStats bool
	UncounterHeld       int
	UncounterPassed     int
	UncounterStalls     int
)

// uncounterable reports whether the spell at stack index i can't be
// countered: its own "this spell can't be countered" (noCtr), or a
// battlefield ReplSpellsCantBeCountered source its controller controls,
// still with its abilities, whose Filter matches the spell card.
//
//go:noinline
func (s *State) uncounterable(k *prep, i int) bool {
	e := &s.stack[i]
	self := k.cards[e.card].noCtr
	if !k.anyUncounter || DebugUncounterOff {
		return self
	}
	n := s.uncounterScan(k, e)
	if self {
		n++
	}
	if DebugUncounterStats && !self {
		if n > 0 {
			UncounterHeld++
		} else {
			UncounterPassed++
		}
	}
	if n > 1 {
		if DebugUncounterStats {
			UncounterStalls++
		}
		s.capFull(StallUncounterOrder)
	}
	return n > 0
}

// uncounterScan counts the battlefield replacements that stop a Counter on
// the spell of stack entry e: each ReplSpellsCantBeCountered ability of a
// permanent the spell's controller controls, still with its abilities,
// whose Filter matches the spell card.
func (s *State) uncounterScan(k *prep, e *stackEntry) (n int) {
	for m := s.statM.and(s.ctrlM[e.ctrl]); m.any(); m = m.rest() {
		src := m.first()
		if s.perms[src].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || k.gen != nil && s.ovrLoses(k, src) {
			continue // the source lost its abilities
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplSpellsCantBeCountered && s.uncounterMatch(k, a.Repl.Filter, e) {
				n++
			}
		}
	}
	return n
}

// uncounterMatch is f read against the spell of stack entry e.
func (s *State) uncounterMatch(k *prep, f kop.FilterID, e *stackEntry) bool {
	if k.gen != nil {
		return s.matchSpellCard(k, f, e.card, e.ctrl, e.ctrl, e.obj, NoSlot)
	}
	return s.matchCard(k, f, e.card, e.ctrl, e.ctrl)
}

// wardCounterAt is every ward resolution's counter of its warded entry idx:
// a spell that can't be countered stays on the stack (Out Cold, Long
// Goodbye: "This includes by the ward ability"; CR 101.2, 702.21a), as gorge
// effWard asks counterStopped before its MoveZone.
func (s *State) wardCounterAt(k *prep, idx int) {
	if s.stack[idx].kind == seSpell && s.uncounterable(k, idx) {
		return
	}
	s.counterAt(k, idx)
}
