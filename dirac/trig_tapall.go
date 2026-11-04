package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Mode$ TapAll, "whenever one or more <permanents> you control become
// tapped" (CR 603.2c; gorge trigmatch/tap.go tapAllMatches and the action
// batch latch in trigger_match.go): a TrigTaps line with Tf2Batch over a
// filter of the tapped permanent. gorge opens one action bracket around each
// tapping action -- a Tap or TapAll resolution (effects combatfx.go), the
// declare-attackers taps (rules combat.go, CR 508.1f), one payment's
// tapXType parts (pay paidcost.go, a crew's included), one cast's convoke,
// improvise, harmonize or waterbend taps (EmitConvokeTaps) and one mana
// ability's elected taps (mana_activation.go) -- and queues the line once per
// bracket, at its first matching tap. A tap outside a bracket (a mana
// ability's own {T}, an activated ability's {T}, a single cost tap, a
// permanent put onto the battlefield tapped and attacking) is its own batch.
//
// The kernel's tap batch is genState.tapBatch: 0 none, else the pending
// queue's length + 1 when the outermost bracket opened (brackets nest, the
// outer one counts). A Tf2Batch Taps line already queued since then is not
// queued again. It is a separate latch from batchLatch (gorge's action
// bracket is a separate bracket from its zone and sacrifice batches). The
// mana a permanent makes by tapping fires the line too (manaTapped: genPrep
// manaTaps is set by a TapAll line).

// DebugTapAllPerTap queues a TapAll line once per matching tap, batches
// ignored (mutation testing only: the lockstep must catch it).
var DebugTapAllPerTap bool

// DebugTapAllJoins counts the matching taps that joined an instance already
// queued in their batch (test evidence only; never read by the kernel).
var DebugTapAllJoins int

// tapBatchOpen opens a tap batch when the table has a TapAll line and
// returns whether this call opened it (the outermost one).
func (s *State) tapBatchOpen(k *prep) bool {
	if k.gen == nil || !k.gen.anyTapAll || s.g.tapBatch != 0 {
		return false
	}
	s.g.tapBatch = s.npend + 1
	return true
}

// tapBatchClose closes the tap batch tapBatchOpen opened.
func (s *State) tapBatchClose(opened bool) {
	if opened {
		s.g.tapBatch = 0
	}
}

// tapBatchOK is the latch of a TapAll line: outside a tap batch every tap
// queues it; inside one only the first matching tap does.
//
//go:noinline
func (s *State) tapBatchOK(l trigListener, a *kop.Ability) bool {
	if s.g.tapBatch == 0 || DebugTapAllPerTap {
		return true
	}
	for i := int(s.g.tapBatch) - 1; i < int(s.npend); i++ {
		if p := &s.pend[i]; p.obj == l.obj && p.card == l.card && p.ab == l.ab {
			DebugTapAllJoins++
			return false
		}
	}
	return true
}
