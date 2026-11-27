package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "Whenever one or more <kind> counters are put on this permanent for the
// first time each turn" (Mode$ CounterAddedOnce with FirstTime$ True, U.S.S.
// Enterprise-D): the first matching CounterChange batch of the turn queues
// the trigger, every later one queues nothing. kgen lowers only the
// ValidCard$ Card.Self reading (lower_trigger.go); a per-recipient line
// (Botanical Brawler's "the first time ... on that permanent") refuses,
// because this latch is keyed on the source object.
//
// The per-source latch is the existing usedThisTurn record pool, the same one
// TfOncePerTurn uses (statics_gen.go; cleared by newTurnGen), keyed on the
// trigger line, so the first-time cadence needs no State bytes and a counter
// of another kind never spends it: trigOK only reaches the gate after
// objEventMatch and the CounterType$ / crossing checks accepted the batch.
//
// The spend happens in queueTrig beside the once-per-turn mark, so a line
// whose intervening-if condition (or any later gate) refused the batch does
// not spend the turn. The Panharmonicon echo is NOT suppressed: the additional
// times are copies of the one triggering (CR 603.2d), not new counter events,
// and gorge's panharmoniconEchoes echoes them the same way.
//
// A source that leaves and returns in the same turn is a new object (CR
// 400.7): the used record's object version gives it a fresh latch, and
// gorge's log scan stops at the object's departure from the battlefield.

// DebugCounterFirstOff is the mutation switch of TestEnterpriseDMutation:
// the latch never refuses, so every matching counter batch fires.
var DebugCounterFirstOff bool

// CounterFirstRec is one first-time CounterAddedOnce decision: Fired is a
// trigger the line queued, false a later matching batch refused by the latch.
type CounterFirstRec struct {
	Card  string
	Obj   Obj
	Turn  uint16
	Fired bool
}

// CounterFirstLog, when non-nil, records every first-time decision (the
// enterprise test counts the fired and refused batches; nothing else reads
// it).
var CounterFirstLog *[]CounterFirstRec

// firstTimeLatchEvent reports the events whose Tf2FirstTime is the per-source
// once-a-turn latch of this file: CounterAddedOnce (FirstTime$ True on a
// Card.Self counter line), BecomesTarget (FirstTime$ True on the ungated
// Card.Self self line; kgen lowers only that shape, lower_trigger.go) and
// DamageDoneOnce (FirstTime$ True on the Card.Self line, Hercules, Olympian
// Hero; kgen lowers only that shape, lower_damagedoneonce.go, where the
// damaged referent is the listener the latch is keyed on). The other FirstTime
// events read their own per-turn facts (taps: mechTappedTurn; life
// lost/gained: the loss and gain amounts).
func firstTimeLatchEvent(a *kop.Ability) bool {
	return a.Trig.Event == kop.TrigCounterAddedOnce || a.Trig.Event == kop.TrigBecomesTarget ||
		a.Trig.Event == kop.TrigDamageDone && a.Trig.Flags2&kop.Tf2PerReferent != 0
}

// counterFirstUsed reports whether a's first-time line already had its turn
// this turn. A CounterAddedOnce line reads the per-line mark. A BecomesTarget
// line reads the object's targeted-this-turn bit (mechBtTurn) instead: gorge's
// gate is the log scan over the turn's targetings, so an earlier targeting
// counts even when no such line was live to queue then -- the Lightning Bolt
// that hit a creature before Kira, Great Glass-Spinner entered and granted it
// the line -- and a queue-time mark would also make two lines of one object
// share one latch (the granted-on permanent's own printed line and the
// granted line are both ab 0 in their own cards). The caller has checked the
// event and Tf2FirstTime.
//
//go:noinline
func (s *State) counterFirstUsed(k *prep, l trigListener, a *kop.Ability) bool {
	used := false
	if a.Trig.Event == kop.TrigBecomesTarget {
		// the bit only: a per-line mark would share another line's record (the
		// granted-on permanent's own printed line and the granted line are both
		// ab 0 in their own cards, and a printed line's once-per-turn mark
		// would block the granted line's first time)
		used = s.mSl(k, mechBtTurn, l.sl)
	} else {
		used = s.usedThisTurn(l.obj, l.ab)
	}
	if !DebugCounterFirstOff && used {
		if CounterFirstLog != nil {
			*CounterFirstLog = append(*CounterFirstLog, CounterFirstRec{Card: k.t.Cards[l.card].Name, Obj: l.obj, Turn: s.turn})
		}
		return true
	}
	return false
}

// markCounterFirst spends the line's per-turn latch when the trigger queues.
// A BecomesTarget line writes none: its gate is the object bit, which this
// targeting set after the walk (targetedOne), and a mark would spend pool
// space no read ever consults.
//
//go:noinline
func (s *State) markCounterFirst(k *prep, l trigListener, a *kop.Ability) {
	if a.Trig.Event != kop.TrigBecomesTarget {
		s.markUsed(l.obj, l.ab)
	}
	if CounterFirstLog != nil {
		*CounterFirstLog = append(*CounterFirstLog, CounterFirstRec{Card: k.t.Cards[l.card].Name, Obj: l.obj, Turn: s.turn, Fired: true})
	}
}
