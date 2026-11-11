//go:build dirac_mp

package dirac

// The dirac_mp size class: four seat slots. The size knobs that depend on the
// seat count live here and in seats_default.go; every other file derives from
// them.
const MaxSeats = 4

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

// embRemSlots is how many objects one emblem remembers (static_emblemrem.go):
// the frozen-number byte and embR. The four-seat class has no reserved bytes
// left for embX, so it holds two and a larger set stalls (StallEmbRem).
const embRemSlots = 2

// linkPadN pads genState after its sixteen exile-until-leaves links so perms
// keeps its cache-line phase (statelayout_mp_test.go): the four-seat class's
// alignment leaves a smaller gap than the default class's.
const linkPadN = 8
