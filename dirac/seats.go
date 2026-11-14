// MaxSeats, the length of every per-seat array in State and Config, is set
// per size class: seats_default.go (two seats) and seats_mp.go (build tag
// dirac_mp, four seats). The seat count is still two everywhere (seat loops,
// opponent arithmetic and the target codes); the constant only sizes the
// arrays.

package dirac

// noSeat is a seat byte that names no seat: a walker's departed attacker
// has no defending player (atkSeat), and no attacker no first defender
// (firstAtkSeat). It is no target code: a code is wider than a byte under
// dirac_mp, so a seat is never compared against NoTarget.
const noSeat uint8 = 0xFF
