package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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
// Every seat has one: the code space holds MaxSeats player codes.
func PlayerCode(p uint8) uint8 { return PlayerTarget0 + p }

// CodeSeat is the seat a player target code names; c must satisfy
// isPlayerTarget.
func CodeSeat(c uint8) uint8 { return c - PlayerTarget0 }

// nSeats is the number of seats in this game. The default build plays two
// and never reads the byte; the multiplayer build reads what InitN set.
func (s *State) nSeats() uint8 {
	if MaxSeats == 2 {
		return 2
	}
	return s.g.nseat
}

// mullOrderFrom is the mulligan order of n seats: turn order from the
// starting player p (CR 103.5: the starting player decides first, then each
// other player in turn order).
func mullOrderFrom(p, n uint8) [MaxSeats]uint8 {
	if MaxSeats == 2 {
		return [MaxSeats]uint8{p, 1 - p}
	}
	var o [MaxSeats]uint8
	for i := uint8(0); i < n; i++ {
		o[i] = (p + i) % n
	}
	return o
}

// out reports whether seat p has left the game (CR 800.4a). At two seats a
// loss ends the game, so no live-seat walk ever meets a departed seat.
func (s *State) out(p uint8) bool {
	if MaxSeats == 2 {
		return false
	}
	return s.pl[SeatIx(p)].lost
}

// nextAlive is the first seat after p in turn order that is still in the
// game, or p when no other is (gorge state.Game.NextAlive).
func (s *State) nextAlive(p uint8) uint8 {
	if MaxSeats == 2 {
		return 1 - p
	}
	n := s.nSeats()
	q := p
	for i := uint8(1); i < n; i++ {
		q++
		if q >= n {
			q = 0
		}
		if !s.out(q) {
			return q
		}
	}
	return p
}

// aliveCount is the number of seats still in the game (gorge
// state.Game.AliveCount): a round of priority passes by each of them
// resolves or advances (CR 117.4).
func (s *State) aliveCount() uint8 {
	if MaxSeats == 2 {
		return 2
	}
	c := uint8(0)
	for p := uint8(0); p < s.nSeats(); p++ {
		if !s.out(p) {
			c++
		}
	}
	return c
}

// blockDef is the defending player declaring blockers now (CR 509.1: each
// defending player declares; gorge asks them one at a time, askBlockers).
// At two seats it is the active player's one opponent.
func (s *State) blockDef() uint8 {
	if MaxSeats == 2 {
		return 1 - s.active
	}
	return s.g.blkDef
}

// oppsM is the permanents p's opponents control: at two seats the other
// seat's, at more the union of every other seat's.
func (s *State) oppsM(p uint8) slotMask {
	if MaxSeats == 2 {
		return s.ctrlM[1-p]
	}
	var m slotMask
	for q := uint8(0); q < s.nSeats(); q++ {
		if q != p {
			m = m.or(s.ctrlM[q])
		}
	}
	return m
}

// atkSeat is the player attacker a's attack is against (CR 506.2): the
// attacked player, or the controller of the attacked planeswalker, or the
// protector of the attacked battle (CR 310.8). A walker that has left
// leaves its attacker with no defending player here, NoTarget.
func (s *State) atkSeat(k *prep, a Slot) uint8 {
	if MaxSeats == 2 {
		return 1 - s.active
	}
	t := s.perms[a].atk
	if isPlayerTarget(t) {
		return CodeSeat(t)
	}
	q := s.attackedPerm(k, a)
	if q == NoSlot {
		return NoTarget
	}
	if s.typesAt(k, q)&kop.TBattle != 0 && s.perms[q].chosen&protChosen == protChosen {
		return s.perms[q].chosen &^ protChosen
	}
	return s.perms[q].ctrl
}

// blockAtkM is the active player's permanents an attacker among which is
// attacking the defender declaring blockers now; at two seats every one.
func (s *State) blockAtkM(k *prep) slotMask {
	m := s.ctrlM[s.active]
	if MaxSeats == 2 {
		return m
	}
	for x := m; x.any(); x = x.rest() {
		if a := x.first(); s.perms[a].flags&pfAttacking != 0 && s.atkSeat(k, a) != s.g.blkDef {
			m.clr(a)
		}
	}
	return m
}
