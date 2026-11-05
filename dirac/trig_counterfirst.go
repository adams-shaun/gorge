package dirac

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
// Known approximation: the used record's object version means a source that
// leaves and returns in the same turn fires again here, where gorge's log
// scan keys on the object id alone and refuses.

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

// counterFirstUsed reports whether a's first-time CounterAddedOnce line
// already queued a trigger this turn. The caller has checked the event and
// Tf2FirstTime.
//
//go:noinline
func (s *State) counterFirstUsed(k *prep, l trigListener) bool {
	if !DebugCounterFirstOff && s.usedThisTurn(l.obj, l.ab) {
		if CounterFirstLog != nil {
			*CounterFirstLog = append(*CounterFirstLog, CounterFirstRec{Card: k.t.Cards[l.card].Name, Obj: l.obj, Turn: s.turn})
		}
		return true
	}
	return false
}

// markCounterFirst spends the line's per-turn latch when the trigger queues.
//
//go:noinline
func (s *State) markCounterFirst(k *prep, l trigListener) {
	s.markUsed(l.obj, l.ab)
	if CounterFirstLog != nil {
		*CounterFirstLog = append(*CounterFirstLog, CounterFirstRec{Card: k.t.Cards[l.card].Name, Obj: l.obj, Turn: s.turn, Fired: true})
	}
}
