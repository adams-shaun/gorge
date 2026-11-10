package dirac

// Seat helpers. The kernel plays two seats today; these name what raw seat
// arithmetic (1-p, p&1, PlayerTarget0+p) means, in terms that stay true at
// MaxSeats seats, and compile to the raw form at MaxSeats == 2. Use them in
// place of the raw spellings; TestSeatArithmeticOnlyShrinks counts what is
// left.
//
// "The opponent" has no helper on purpose: at more than two seats the kernel
// must choose among opponents (target opponent, defending player) or visit
// each of them, and those are decisions still to be designed.

// NextSeat is the seat after p in turn order (CR 101.4: APNAP order runs in
// turn order from the active player; CR 102.1: the turn passes in seating
// order).
func NextSeat(p uint8) uint8 {
	if MaxSeats == 2 {
		return 1 - p
	}
	return (p + 1) % MaxSeats
}

// IsOpponent reports whether seat q is an opponent of seat p (CR 102.2: in a
// two-player game the other player is your opponent; CR 102.3: in a
// multiplayer game without teams every other player is).
func IsOpponent(p, q uint8) bool { return p != q }

// SeatIx is seat p as an index into a per-seat array: the modulo is the
// bounds proof, and at two seats it is the p&1 it replaces.
func SeatIx(p uint8) uint8 { return p % MaxSeats }

// PlayerCode is seat p's target code (CR 115.1: a player can be a target).
func PlayerCode(p uint8) uint8 { return PlayerTarget0 + p }

// CodeSeat is the seat a player target code names; c must satisfy
// isPlayerTarget.
func CodeSeat(c uint8) uint8 { return c - PlayerTarget0 }
