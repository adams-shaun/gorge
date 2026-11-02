package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Entry riders on a ChangeZoneAll sweep onto the battlefield
// (kop.MvEntryRiders, Ghost Vacuum).
//
// gorge's effChangeZoneAll folds WithCountersType$ into each entry event
// (CR 122.6) and applies StaticEffect$ per moved card inside the move loop
// (CR 603.6b), both before the held enters checks fire. The counter payload
// genState.enterCtr is consumed and cleared by each entrant (enterGen), so it
// is armed again before every enterHeld; the override record goes on the
// entrant's slot straight after it enters.

// sweepRiders is one sweep's decoded riders.
type sweepRiders struct {
	n    int // counters per entrant (0 = none)
	kind kop.CounterKind
	ov   int   // override index, -1 = none
	p, t int16 // the override's evaluated P/T
	// AtEOT$ (trig_ateotmovers.go): a next-end-step promise per entrant
	eot, eotExile bool
}

// sweepRidersOf decodes an OpChangeZoneAllEx with MvEntryRiders: A is the
// constant count | counter kind<<8, B the override index+1 (a pool-wide index,
// so B holds all 16 bits; 0 = none).
func (s *State) sweepRidersOf(k *prep, in *kop.Instr) sweepRiders {
	word, eot, exile := decodeEntryAtEOT(in.A.N)
	r := sweepRiders{n: int(uint8(word)), kind: kop.CounterKind(word >> 8), ov: int(uint16(in.B.N)) - 1, eot: eot, eotExile: exile}
	if r.ov >= 0 {
		r.p, r.t = s.overridePT(k, r.ov)
	}
	return r
}

// armEntryCounters sets the counter payload the next entrant consumes.
func (s *State) armEntryCounters(r *sweepRiders) {
	if r.n > 0 {
		s.g.enterCtr.n, s.g.enterCtr.kind = uint8(r.n), r.kind
	}
}

// entryOverride gives the entrant in sl its override record.
func (s *State) entryOverride(k *prep, r *sweepRiders, sl Slot) {
	if r.ov >= 0 && s.occ.has(sl) {
		s.applyOverridePT(k, sl, r.ov, s.res.e.card, 0, r.p, r.t)
	}
}
