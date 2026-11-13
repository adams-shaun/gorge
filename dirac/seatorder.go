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
func PlayerCode(p uint8) TCode { return PlayerTarget0 + TCode(p) }

// isPlayerOrNone reports whether target byte c is at or above the player
// codes: a player, a departed stack target (0xFE) or NoTarget. It is the raw
// "c >= PlayerTarget0" range test, named so it reads the same at every seat
// count; isPlayerTarget is the test for a player alone.
func isPlayerOrNone(c TCode) bool { return c >= PlayerTarget0 }

// CodeSeat is the seat a player target code names; c must satisfy
// isPlayerTarget.
func CodeSeat(c TCode) uint8 { return uint8(c - PlayerTarget0) } // code-ok: a seat, below MaxSeats

// nSeats is the number of seats in this game. The default build plays two
// and never reads the byte; the multiplayer build reads what InitN set.
func (s *State) nSeats() uint8 {
	if MaxSeats == 2 {
		return 2
	}
	return s.g.nseat
}

// NumSeats is the number of seats in this game, departed seats included
// (gorge's view keeps a seat that has left in its place).
func (s *State) NumSeats() uint8 { return s.nSeats() }

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
	if isPlayerTarget(TCode(t)) {
		return CodeSeat(TCode(t))
	}
	q := s.attackedPerm(k, a)
	if q == NoSlot {
		return uint8(NoTarget)
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

// seatMax is the largest per-seat count in per (max8(per[0], per[1]) at two
// seats).
func seatMax(per *[MaxSeats]uint8) uint8 {
	m := per[0]
	for _, v := range per[1:] {
		m = max8(m, v)
	}
	return m
}

// seatCount is the number of seats seen marks.
func seatCount(seen *[MaxSeats]bool) uint8 {
	d := uint8(0)
	for _, b := range seen {
		if b {
			d++
		}
	}
	return d
}

// seatSeq is an ordered set of seats, at most MaxSeats, returned by value
// (an each-player or each-opponent expansion, in gorge's order).
type seatSeq struct {
	s [MaxSeats]uint8
	n uint8
}

// seatOne is the set holding p alone, or the empty set when p is 255.
func seatOne(p uint8) seatSeq {
	if p == 255 {
		return seatSeq{}
	}
	return seatSeq{s: [MaxSeats]uint8{p}, n: 1}
}

// at is the i-th seat of q, or 255 past its end.
func (q seatSeq) at(i uint8) uint8 {
	if i >= q.n {
		return 255
	}
	return q.s[i]
}

// add appends p to q.
func (q *seatSeq) add(p uint8) {
	q.s[q.n] = p
	q.n++
}

// aliveFrom is every seat still in the game in turn order from p (gorge
// state.Game.AliveFrom): "each player" from the resolving controller.
func (s *State) aliveFrom(p uint8) seatSeq {
	if MaxSeats == 2 {
		return seatSeq{s: [MaxSeats]uint8{p, 1 - p}, n: 2}
	}
	var q seatSeq
	n := s.nSeats()
	for i := uint8(0); i < n; i++ {
		x := p + i
		if x >= n {
			x -= n
		}
		if !s.out(x) {
			q.add(x)
		}
	}
	return q
}

// oppsFrom is p's opponents still in the game, in turn order after p (gorge
// reads "each opponent" as AliveFrom(p) without p; CR 102.3).
func (s *State) oppsFrom(p uint8) seatSeq {
	if MaxSeats == 2 {
		return seatSeq{s: [MaxSeats]uint8{1 - p}, n: 1}
	}
	q := s.aliveFrom(p)
	if q.n > 0 && q.s[0] == p {
		copy(q.s[:], q.s[1:])
		q.n--
	}
	return q
}

// The each-player chain (g.repNext) holds the seats still to come after the
// one acting now. At two seats that is at most one seat, kept as itself
// (255: none). At more seats it packs up to MaxSeats-1 seats, two bits each
// from the low end, under a count in the top two bits; distinct seats never
// spell 255. seatsPack and seatsPop are that byte's encoding, shared with
// other one-byte seat chains (the may-mill asks' g.pickZ).

// seatsPack packs q's seats from index i on.
func seatsPack(q seatSeq, i uint8) uint8 {
	if MaxSeats == 2 {
		return q.at(i)
	}
	if i >= q.n {
		return 255
	}
	b, m := uint8(0), uint8(0)
	for j := i; j < q.n; j++ {
		b |= (q.s[j] & 3) << (2 * m)
		m++
	}
	return m<<6 | b
}

// seatsPop takes a packed chain's next seat (255: none).
func seatsPop(c *uint8) uint8 {
	b := *c
	if MaxSeats == 2 || b == 255 {
		*c = 255
		return b
	}
	m := b >> 6
	p := b & 3
	if m <= 1 {
		*c = 255
	} else {
		*c = (m-1)<<6 | (b&0x3f)>>2
	}
	return p
}

// repRest queues q's seats from index i on as the chain's remainder.
func (s *State) repRest(q seatSeq, i uint8) { s.g.repNext = seatsPack(q, i) }

// repPop takes the chain's next seat (255: none).
func (s *State) repPop() uint8 { return seatsPop(&s.g.repNext) }

// repPending reports whether the chain has a seat to come.
func (s *State) repPending() bool { return s.g.repNext != 255 }

// dmgSeat is the player a damage trigger names as "that player" (its
// trigger player): the damaged seat when the recipient rcp is a player. At
// two seats the old reading stays, the source controller's opponent. At more
// seats a permanent recipient carries its controller (the referent's
// controller, as ddoRecipient queues an object referent and gorge's
// TriggeredTargetController reads it). A batch's aggregate (NoTarget) names
// no single damaged seat; the next seat stands in there, and every line that
// reads a recipient's player skips the aggregate fire (ddoLine).
func (s *State) dmgSeat(rcp, srcCtrl uint8) uint8 {
	if MaxSeats == 2 {
		return 1 - srcCtrl
	}
	if isPlayerTarget(TCode(rcp)) {
		return CodeSeat(TCode(rcp))
	}
	if isObjTarget(TCode(rcp)) && s.objs[rcp].zone == kop.ZoneBattlefield {
		return s.perms[s.objs[rcp].slot].ctrl
	}
	return NextSeat(srcCtrl)
}

// oppDamaged reports whether damage mask plM (bit q: seat q was dealt
// damage) holds an opponent of seat p.
func oppDamaged(plM, p uint8) bool {
	if MaxSeats == 2 {
		return plM&(1<<(1-p)) != 0
	}
	return plM&^(1<<p) != 0
}
