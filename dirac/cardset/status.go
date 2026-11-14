package cardset

import "fmt"

// Status is what a pool entry is allowed to do. kgen computes it at table build:
//
//	supported = lowered && not refused by name && evidenced
//
// The zero value is StatusSupported, so a table without statuses must not load
// (DecodeTable refuses one).
type Status uint8

const (
	StatusSupported  Status = iota // lowered, evidenced, not refused
	StatusNotLowered               // kgen cannot represent it; Pool holds an inert stand-in
	StatusRefused                  // named in dirac/support/refused
	StatusUnverified               // lowered, in no dirac/support/evidence list
)

func (s Status) String() string {
	switch s {
	case StatusSupported:
		return "supported"
	case StatusNotLowered:
		return "not lowered"
	case StatusRefused:
		return "refused"
	case StatusUnverified:
		return "unverified"
	}
	return fmt.Sprintf("Status(%d)", uint8(s))
}

// EnforceEvidence makes an unverified card unplayable through ByName, as a refused
// one is. A caller that wants unverified cards anyway, such as self-play on a pool
// with little evidence, reads the raw table or asks AllowUnverified.
const EnforceEvidence = true

// RefusedError is Matchup's answer for a deck holding a card its set will not play.
type RefusedError struct {
	Card   string
	Status Status
	Reason string
}

func (e *RefusedError) Error() string {
	return fmt.Sprintf("card %q is not supported: %s", e.Card, e.Reason)
}

// playable is which statuses a view of a table plays.
type playable func(Status) bool

func gated(enforce bool) playable {
	return func(s Status) bool {
		return s == StatusSupported || s == StatusUnverified && !enforce
	}
}

func notRefused(s Status) bool { return s != StatusNotLowered && s != StatusRefused }

func lowered(s Status) bool { return s != StatusNotLowered }

// viewOf is the Set that plays d's entries whose status plays accepts.
func viewOf(d *PoolData, plays playable) Set {
	sup := make([]bool, len(d.Status))
	for i, st := range d.Status {
		sup[i] = plays(st)
	}
	return &dataSet{d: d, plays: sup, nameInert: nameInertCards(d)}
}

// RawByName returns the set named name with every lowered card playable, refused
// and unverified alike. It is how a card earns evidence or proves it still diverges,
// so only the lockstep oracle and kgen may call it (internal/boundary holds them to that).
func RawByName(name string) (Set, error) {
	s, err := load(name)
	if err != nil {
		return nil, err
	}
	return viewOf(s.(*dataSet).d, lowered), nil
}

// Raw returns s with every lowered card playable, refused and unverified alike:
// RawByName for a set built outside the named pools (a table of one repro's decks).
func Raw(s Set) Set {
	return viewOf(s.(*dataSet).d, lowered)
}

// AllowUnverified returns s playing unverified cards too. A refused card stays
// refused: it is known to play wrongly, where an unverified one is only unproven.
func AllowUnverified(s Set) Set {
	return viewOf(s.(*dataSet).d, notRefused)
}

// AllowNamed returns s playing its supported cards and the unverified cards names
// holds; tokens play as under AllowUnverified. It is how a targeted evidence run plays
// the cards it owes evidence among cards that already have it, the way a real game
// plays supported cards only.
func AllowNamed(s Set, names map[string]bool) Set {
	d := s.(*dataSet).d
	sup := make([]bool, len(d.Status))
	for i, st := range d.Status {
		if i >= d.NumCards {
			sup[i] = notRefused(st)
			continue
		}
		sup[i] = st == StatusSupported || st == StatusUnverified && names[d.Names[i]]
	}
	return &dataSet{d: d, plays: sup, nameInert: nameInertCards(d)}
}
