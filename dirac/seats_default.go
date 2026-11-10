//go:build !dirac_mp

package dirac

// The default size class: two seats. The size knobs that depend on the seat
// count live here and in seats_mp.go; every other file derives from them.
const MaxSeats = 2

// The target-code space (types.go): objects 0..MaxObj-1, stack abilities
// StackTarget0..PlayerTarget0-1 (16 codes), players PlayerTarget0.. (one
// code per seat), 0xFE a departed stack target and 0xFF none.
const (
	MaxObj              = 236  // card instances: both decks + tokens (at most StackTarget0)
	StackTarget0  uint8 = 0xEC // the bottom stack ability's target code
	PlayerTarget0 uint8 = 0xFC // seat 0's target code
)

// object.acts packs the owner above this turn's activation count: one owner
// bit for two seats.
const (
	objActsMask   uint8 = 0x7f // object.acts: this turn's non-mana activation count
	objOwnerBits  uint8 = 0x80 // object.acts: the owner (0 or 1)
	objOwnerShift       = 7
)
