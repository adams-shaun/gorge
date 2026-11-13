//go:build dirac_mp

package dirac

// The dirac_mp size class: four seat slots. The size knobs that depend on the
// seat count live here and in seats_default.go; every other file derives from
// them.
const MaxSeats = 4

// turnExhaustShift is the first bit of a player's Exhaust-activation fact in
// genState.turnF: bits 0-3 hold the coin flips, bits 4-7 the Exhaust
// activations (static_turnbits.go).
const turnExhaustShift = 4

// The target-code space (types.go): objects 0..MaxObj-1, stack abilities
// StackTarget0..PlayerTarget0-1 (16 codes, as in the default class), players
// PlayerTarget0..PlayerTarget0+3 (one code per seat), 0xFE a departed stack
// target and 0xFF none. Four player codes cost two object slots.
const (
	MaxObj              = 234  // card instances: the decks + tokens (at most StackTarget0)
	StackTarget0  uint8 = 0xEA // the bottom stack ability's target code
	PlayerTarget0 uint8 = 0xFA // seat 0's target code
)

// object.acts packs the owner above this turn's activation count: two owner
// bits for four seats, so the count saturates at 63 (cast.go stalls with
// StallActs rather than wrapping).
const (
	objActsMask   uint8 = 0x3f // object.acts: this turn's non-mana activation count
	objOwnerBits  uint8 = 0xc0 // object.acts: the owner (0..3)
	objOwnerShift       = 6
)

// ovRec.fl (gen.go) packs the controller an UntilYourNextTurn override
// record waits for (two seat bits) above the merge count, which then caps at
// 16 (addOvRec moves on to a new record once the count saturates).
const (
	ovNTWho      uint8 = 0x30 // ovNT record: the waiting controller (0..3)
	ovNTWhoShift       = 4
	ovReps       uint8 = 0x0f // the merge count's mask
)

// embRemSlots is how many objects one emblem remembers (static_emblemrem.go):
// the frozen-number byte and embR. The four-seat class has no reserved bytes
// left for embX, so it holds two and a larger set stalls (StallEmbRem).
const embRemSlots = 2

// linkPadN pads genState after its sixteen exile-until-leaves links so perms
// keeps its cache-line phase (statelayout_mp_test.go). Four seats of
// player.lastLost moved perms eight bytes past phase 0; fifty-six bytes here
// bring it round to the next line.
const linkPadN = 56
