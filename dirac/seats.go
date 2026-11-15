// MaxSeats, the length of every per-seat array in State and Config, is set
// in seats_default.go: two seats. Seat loops, opponent arithmetic and the
// target codes read it, never a literal 2, so the kernel stays seat-generic.

package dirac

// noSeat is a seat byte that names no seat: a walker's departed attacker
// has no defending player (atkSeat), and no attacker no first defender
// (firstAtkSeat). It is no target code: a seat is never compared against
// NoTarget.
const noSeat uint8 = 0xFF
