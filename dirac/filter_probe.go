package dirac

import (
	"errors"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The atom probes evaluate one filter or one amount, given as literal kernel
// forms, against a live State: the oracle's atom harness (oracle atomdiff)
// reads them beside gorge's own matcher and count evaluator at every decision
// of a lockstep game. The kernel never calls them, and they allocate nothing
// once the ProbeSet exists.

// ProbeSet is a table overlay: the game's own table with its filters and
// expressions replaced by a superset the probe's compiler built over them (the
// game's first, then the probe's, so every index the game's own statics and
// CDAs hold still names what it did), prepared as a matchup of that table
// would be. Everything a filter or an amount reads off the cards (types,
// colours, mana values, counters) is the game's.
type ProbeSet struct {
	k *prep
	t kop.Table
}

// NewProbeSet builds the overlay over s's table. It fails when the overlay's
// prep lays out more or less state than the game's (an atom that turns on a
// state bank the game's table lacks reads storage the State never allocated).
func (s *State) NewProbeSet(filters []kop.Filter, exprs []kop.Expr) (*ProbeSet, error) {
	return s.NewProbeSetProgs(filters, exprs, nil)
}

// NewProbeSetProgs is NewProbeSet for filters that carry filter programs
// (kop.PredProg): progs replaces Table.FilterProgs, the program table the
// overlay filters index (nil keeps the game's own, for filters with none).
func (s *State) NewProbeSetProgs(filters []kop.Filter, exprs []kop.Expr, progs []kop.FilterProg) (*ProbeSet, error) {
	base := s.k()
	ps := &ProbeSet{t: *base.t}
	ps.t.Filters, ps.t.Exprs = filters, exprs
	if progs != nil {
		ps.t.FilterProgs = progs
	}
	ps.k = prepare(&ps.t)
	if ps.k.cold != base.cold || ps.k.coldCmd != base.coldCmd {
		return nil, errors.New("the probe table's state layout differs from the game's")
	}
	return ps, nil
}

// ProbeBanks is a set of state the kernel keeps only when the game's own
// table asks for it: an atom that reads one is comparable only in a game whose
// table maintains it (the probe overlay cannot turn it on in a running State).
type ProbeBanks uint8

// The banks an atom may need.
const (
	BankEntered   ProbeBanks = 1 << iota // the entered ledger (ThisTurnEntered reads, the entered-from words)
	BankYourTurns                        // each player's count of turns begun (Count$YourTurns)
)

// ProbeBanks is the banks s's table maintains.
func (s *State) ProbeBanks() ProbeBanks {
	var b ProbeBanks
	if s.g.enteredOn && s.turn != 0 { // before the first turn the ledger records nothing (noteEnter)
		b |= BankEntered
	}
	if s.k().anyYourTurns {
		b |= BankYourTurns
	}
	return b
}

// ProbeFilter reports whether object o matches the overlay filter f (its
// whole Or chain) for a source src controlled by ctrl; src is NoObj for none.
// A battlefield object reads the permanent matcher at the current layer, any
// other the card matcher over its card and owner.
func (s *State) ProbeFilter(ps *ProbeSet, f kop.FilterID, o, src Obj, ctrl uint8) bool {
	ob := &s.objs[o]
	if ob.zone == kop.ZoneBattlefield {
		return s.matchPermGen(ps.k, f, ob.slot, ctrl, s.probeSlot(src), layerFull)
	}
	return s.matchCardGen(ps.k, f, ob.card, ob.owner(), ctrl, o)
}

// ProbeValue evaluates the overlay amount v for a source src controlled by
// ctrl, with no stack entry: what a static or a cost reads outside a
// resolution.
func (s *State) ProbeValue(ps *ProbeSet, v kop.ValueRef, src Obj, ctrl uint8) int {
	return s.evalV(ps.k, v, evalCtx{src: src, srcSl: s.probeSlot(src), ctrl: ctrl})
}

// probeSlot is src's battlefield slot, NoSlot for no source or one elsewhere.
func (s *State) probeSlot(src Obj) Slot {
	if src == NoObj || s.objs[src].zone != kop.ZoneBattlefield {
		return NoSlot
	}
	return s.objs[src].slot
}

// ProbeObjZone is object o's zone, and ZoneNone for an index past the
// objects the game has made.
func (s *State) ProbeObjZone(o Obj) kop.Zone {
	if o >= s.nobj {
		return kop.ZoneNone
	}
	return s.objs[o].zone
}

// ProbePT is battlefield object o's power and toughness as the filter
// matcher reads them (a debugging aid for a mismatch's detail).
func (s *State) ProbePT(o Obj) (int, int) {
	if s.objs[o].zone != kop.ZoneBattlefield {
		return 0, 0
	}
	return s.ptGen(s.k(), s.objs[o].slot)
}
