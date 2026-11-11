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
// countered: its own "this spell can't be countered" (noCtr), a
// battlefield ReplSpellsCantBeCountered source its controller controls,
// still with its abilities, whose Filter matches the spell card, or a fired
// uncounter record naming it.
//
//go:noinline
func (s *State) uncounterable(k *prep, i int) bool {
	e := &s.stack[i]
	self := k.cards[e.card].noCtr
	if c := k.cards[e.card].noCtrCond; c != 0 && !self {
		// "if X is 5 or more, this spell can't be countered" (Banefire): the gate reads the entry's X
		self = s.condEval(k, c, evalCtx{e: e, src: e.obj, ctrl: e.ctrl})
	}
	n := 0
	if k.gen != nil && k.gen.anyDelaySpell && s.g.ndp != 0 && !DebugUncounterDelayOff {
		n = s.uncounterDelayed(e)
	}
	scan := k.anyUncounter && !DebugUncounterOff
	if !scan && n == 0 {
		return self
	}
	if scan {
		n += s.uncounterScan(k, e)
	}
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

// The delayed "the next spell you cast this turn can't be countered"
// (Mistrise Village, Theorist's Proxy; kop.DelaySpellUncounter): gorge's
// Static$ True SpellCast registration resolves its body at the cast with no
// stack object, an Effect whose Counter replacement (Layer$ CantHappen)
// stops a Counter on the remembered cast spell until that spell leaves the
// stack (ForgetOnMoved$ Stack; the Effect has no turn bound). The kernel
// carries the registration as a dpSpell record with dprog.ext
// dpExtUncounter; at the cast, fireDelayedSpell rewrites it in place into
// the fired form (dpSpell clear, objs[0]/vers[0] the cast spell), one more
// match here per record. A record whose spell has left the stack is
// dropped (dropUncounterGone), so it holds its delayed-program slot only
// while the spell is on the stack.

// dpExtUncounter marks a dprog record kop.DelaySpellUncounter's.
const dpExtUncounter uint8 = 3

// DebugUncounterDelayOff ignores the fired uncounter records (the mutation:
// the kernel counters a spell gorge refuses to). Never set in production.
var DebugUncounterDelayOff bool

// dpUncounterFired reports whether d is a fired uncounter record.
func dpUncounterFired(d *dprog) bool {
	return d.ext == dpExtUncounter && d.n&dpSpell == 0
}

// uncounterDelayed counts the fired uncounter records naming the spell of
// stack entry e as it is now.
func (s *State) uncounterDelayed(e *stackEntry) (n int) {
	for i := 0; i < int(s.g.ndp); i++ {
		if d := &s.g.dp[i]; dpUncounterFired(d) && d.objs[0] == e.obj && d.vers[0] == s.objs[e.obj].ver {
			n++
		}
	}
	return n
}

// dropUncounterGone drops every fired uncounter record whose spell has left
// the stack (a new object, CR 400.7), keeping the others in order.
//
//go:noinline
func (s *State) dropUncounterGone() {
	n := int(s.g.ndp)
	keep := 0
	for i := 0; i < n; i++ {
		d := &s.g.dp[i]
		if dpUncounterFired(d) && (s.objs[d.objs[0]].ver != d.vers[0] || s.objs[d.objs[0]].zone != kop.ZoneStack) {
			continue
		}
		s.g.dp[keep], s.g.dpLKI[keep] = *d, s.g.dpLKI[i]
		keep++
	}
	for i := keep; i < n; i++ {
		s.g.dp[i], s.g.dpLKI[i] = dprog{}, 0
	}
	s.g.ndp = uint8(keep)
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
