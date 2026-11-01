package event

// The event modes. ModeDraft is the zero Mode: a booster draft.
// ModeSealed: every seat opens PacksPerSeat packs straight into its pool,
// then builds.
const (
	ModeDraft  = "draft"
	ModeSealed = "sealed"
)

// The series structures. StructureRoundRobin is the zero Structure (circle
// method, n-1 rounds by default). StructureSwiss pairs by SwissPairings;
// each round opens when the driver logs a RoundPaired entry. StructureNone
// plays no series: the event ends after BUILD.
const (
	StructureRoundRobin = "roundrobin"
	StructureSwiss      = "swiss"
	StructureNone       = "none"
)

// Pack counts per seat.
const (
	DraftPacks         = 3  // draft.Rounds
	DefaultSealedPacks = 6  // a sealed event's default supply
	MaxSealedPacks     = 12 // the largest sealed supply an event may carry
)

// NumSeats reports the event's seat count (4 or 8), 0 before its
// EventCreated.
func (e *Event) NumSeats() int { return len(e.seats) }

// Mode reports the event's supply mode: ModeDraft or ModeSealed (normalized
// from its EventCreated entry, "" meaning ModeDraft). "" before EventCreated.
func (e *Event) Mode() string { return e.mode }

// Structure reports the event's series structure: StructureRoundRobin,
// StructureSwiss or StructureNone (normalized from its EventCreated entry,
// "" meaning StructureRoundRobin). "" before EventCreated.
func (e *Event) Structure() string { return e.structure }

// Rounds reports the event's round count: for StructureRoundRobin the
// normalized round count (n-1 unless the EventCreated entry named one), for
// StructureSwiss the normalized round count (ceil(log2 n) unless the
// EventCreated entry named one), and 0 for StructureNone, which plays no
// rounds.
func (e *Event) Rounds() int { return e.rounds }

// PacksPerSeat reports the event's pack count per seat: 3 in draft mode,
// the sealed supply (6 by default) in sealed mode.
func (e *Event) PacksPerSeat() int { return e.packsPer }

// Collation reports the booster collation the EventCreated entry named, ""
// when absent. It is informational: the packs are logged in full.
func (e *Event) Collation() string { return e.collation }

// CurrentPairings returns a copy of the current round's pairs in pairing
// order, or nil outside a round phase.
func (e *Event) CurrentPairings() [][2]int {
	return append([][2]int(nil), e.pairings...)
}

// defaultPacks is the pack count per seat the mode implies when the
// EventCreated entry names none.
func (e *Event) defaultPacks() int {
	if e.mode == ModeSealed {
		return DefaultSealedPacks
	}
	return DraftPacks
}
