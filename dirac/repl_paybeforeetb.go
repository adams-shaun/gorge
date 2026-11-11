package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplPayBeforeETB (Mox Diamond): "If this would enter, you may discard a land
// card instead. If you do, put it onto the battlefield. If you don't, put it
// into its owner's graveyard." gorge runs the discard, then the entry or the
// graveyard move, inside the replaced move while the spell is still on the
// stack, so no enters watcher ever sees a card that did not enter. The kernel
// runs the replacement's Prog (the optional, remembering land discard) as the
// permanent spell starts resolving (payETBStart); when the program ends,
// payETBGate re-enters the resolution with the discard paid and the card
// enters as any permanent spell does, or the spell leaves the stack for its
// owner's graveyard through the ordinary finish when nothing was discarded.
// The answered marker rides s.g.pre (akPayETB, state 2) until the entry
// completes, so no State byte is added. Entering by any other route (put onto
// the battlefield by an effect, a token copy) stalls: gorge asks there with
// the move still pending, and the kernel's entry is already made.

// StallPayETB: a card whose entry is replaced by ReplPayBeforeETB entered the
// battlefield other than by its cast spell resolving.
const StallPayETB = 170

func init() { stallNames[StallPayETB] = "paybeforeetb" }

// akPayETB marks s.g.pre as the gate's answered record (never posed).
const akPayETB uint8 = 30

// PayETBStats counts the gate's outcomes for the lockstep test's reach check
// (dirac/oracle/moxdiamond_test.go): spells that discarded a land and entered,
// and spells that discarded nothing and went to the graveyard.
type PayETBStats struct{ Entered, Graveyard int }

// DebugPayETBStats, when set, is filled as pay-before-entering programs end.
var DebugPayETBStats *PayETBStats

// payETBAb is the index of card c's ReplPayBeforeETB ability, or -1.
func payETBAb(k *prep, c CardID) int {
	if k.gen.replBits[c]&rbPayETB == 0 {
		return -1
	}
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplPayBeforeETB {
			return i
		}
	}
	return -1
}

// payETBPaid reports whether the gate has answered for object o.
func (s *State) payETBPaid(o Obj) bool {
	return s.g.pre.state == 2 && s.g.pre.kind == akPayETB && s.g.pre.obj == o
}

// payETBStart begins the permanent spell e's pay-before-entering program;
// false when its card has none or the gate already answered.
func (s *State) payETBStart(k *prep, e *stackEntry) bool {
	if k.gen.anyRepl&rbPayETB == 0 || s.payETBPaid(e.obj) {
		return false
	}
	ab := payETBAb(k, e.card)
	if ab < 0 {
		return false
	}
	s.res.e.ab = uint8(ab) // prog(k) reads the replacement's Prog
	s.runProg(k)
	return true
}

// payETBGate runs when a resolution's program ends: the pay-before-entering
// program of a spell that remembered a discard re-enters the resolution to
// enter the card; false (the ordinary finish) when it discarded nothing.
func (s *State) payETBGate(k *prep) bool {
	if k.gen.anyRepl&rbPayETB == 0 {
		return false
	}
	e := &s.res.e
	if e.kind != seSpell || payETBAb(k, e.card) != int(e.ab) {
		return false
	}
	if st := DebugPayETBStats; st != nil {
		if s.g.nrem == 0 {
			st.Graveyard++
		} else {
			st.Entered++
		}
	}
	if s.g.nrem == 0 {
		return false
	}
	s.g.pre = preAsk{obj: e.obj, kind: akPayETB, state: 2}
	s.resolveTop(k)
	return true
}

// payETBStray stalls the game when o, a card carrying the replacement, enters
// without the gate's answer (enterGen).
func (s *State) payETBStray(k *prep, o Obj) {
	if k.gen.anyRepl&rbPayETB != 0 && k.gen.replBits[s.objs[o].card]&rbPayETB != 0 && !s.payETBPaid(o) {
		s.capFull(StallPayETB)
	}
}
