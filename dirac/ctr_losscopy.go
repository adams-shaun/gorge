package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Miss Highwater's loss copy: "for as long as they have a contract counter,
// when they lose the game, for each artifact and creature they controlled,
// create a token that's a copy of it", the Effect its damage trigger makes for
// the player it contracts. The Effect is per seat here: owed[p] is the seat
// whose Effect owes p's loss copy (that seat plus one, 0 for none), set when
// the trigger's put gives p a contract counter with the ext op's C set
// (putContract; kgen's contractLossEffect arms the put when the same chain
// carries the LosesGame Effect).
//
// Gorge's LosesGame trigger goes on the stack at the loss and resolves after
// the CR 800.4a sweep, so the kernel stages the copy the same way: leaveGame
// snapshots p's artifact/creature cards and enqueues an abDelayed trigger
// controlled by the Effect's controller, and resolveDelayed mints the copies
// from that snapshot when it resolves. Copying directly in leaveGame put the
// tokens on the board before gorge's trigger resolved and diverged at the
// first priority of the pod lockstep.
//
// The Effect's Static$ Always self-exile once p has no contract counter is
// not modelled: a contract counter is never removed in the corpus
// (/usr/bin/grep -rl Contract .cards/cardsfolder), so the record is never
// cleared by a count that falls to zero. leaveGame clears it as it fires.
//
// The records are cold arrays of one element per seat, kept only when
// genPrep.anyContract (the contract counts' own condition).

// contractLossTrigAmt is the abDelayed stack entry's trigAmt marking Miss
// Highwater's staged loss copy. Negative markers are the source-less delayed
// triggers' own (speed -1, venture -2, monarch -3).
const contractLossTrigAmt int16 = -4

type contractOwed struct {
	ctrl uint8
	src  Obj // the Effect's source (the LosesGame trigger's source)
}

var coldContractOwed = newCold[contractOwed]("contractOwed", func(g *genPrep) int {
	if g.anyContract {
		return MaxSeats
	}
	return 0
})

// contractPending is p's staged loss copy: the Effect's controller and, in the
// copy body's selector order, the cards of the artifacts and creatures p
// controlled when it left. The CR 800.4a sweep takes those permanents before
// the trigger resolves, so the snapshot is the copy's last-known basis.
type contractPending struct {
	ctrl  uint8
	n     uint8
	cards [MaxPerm]CardID
}

var coldContractPending = newCold[contractPending]("contractPending", func(g *genPrep) int {
	if g.anyContract {
		return MaxSeats
	}
	return 0
})

// contractOwedBy is the seat whose Effect owes seat p's loss copy and the
// Effect's source, ok false when none does.
func (s *State) contractOwedBy(p uint8) (c uint8, src Obj, ok bool) {
	if o := coldContractOwed.of(s); o != nil && p < MaxSeats && o[SeatIx(p)].ctrl != 0 {
		return o[SeatIx(p)].ctrl - 1, o[SeatIx(p)].src, true
	}
	return 0, NoObj, false
}

// setContractOwed records that controller c's Effect, sourced at src, owes
// seat p's loss copy.
func (s *State) setContractOwed(p, c uint8, src Obj) {
	if o := coldContractOwed.of(s); o != nil && p < MaxSeats && c < MaxSeats {
		o[SeatIx(p)] = contractOwed{ctrl: c + 1, src: src}
	}
}

// contractSourceLeft drops every loss-copy debt whose Effect source just left
// the battlefield. The Effect's Duration$ Permanent lifetime is bounded by its
// source's battlefield incarnation (gorge delayedRegistrationLive's
// SourceBattlefield case), so after Miss Highwater leaves, a player's lingering
// contract counter owes no copy; a later loss must not enqueue a trigger gorge
// no longer has. A staged trigger already on the stack is unaffected (CR 603.7:
// the registration is independent once it fired).
func (s *State) contractSourceLeft(src Obj) {
	owed := coldContractOwed.of(s)
	if owed == nil {
		return
	}
	for i := range owed {
		if owed[i].ctrl != 0 && owed[i].src == src {
			owed[i] = contractOwed{}
		}
	}
}

// contractLossCopy stages seat p's owed loss copy as p leaves the game: it
// snapshots the artifacts and creatures p controls in the copy body's selector
// order and enqueues the abDelayed trigger that will mint the copies. The
// effect's controller (a seat that left too gets none) is the trigger's
// controller.
func (s *State) contractLossCopy(k *prep, p uint8) {
	c, src, ok := s.contractOwedBy(p)
	if !ok {
		return
	}
	coldContractOwed.of(s)[SeatIx(p)] = contractOwed{}
	if s.out(c) {
		return
	}
	pending := coldContractPending.of(s)
	if pending == nil {
		return
	}
	rec := &pending[SeatIx(p)]
	rec.ctrl, rec.n = c, 0
	// Gorge resolves Miss Highwater's
	// `Defined$ TriggeredLastStateBattlefieldLKICopy.Creature+...,Artifact+...`
	// group by group, each in battlefield-zone order (effects/lost_board.go:
	// the union is built in script order, and a permanent matching two groups
	// is listed once). The kernel's battlefield order is perm.ts (zones.go
	// joined: entry order, a control change moves it to the end), not
	// slot-mask or object-index order. Sort the snapshot with the selector's
	// order: creatures first, then artifacts that are not creatures, each in
	// ts order, so the copies bind the same object indices as gorge's.
	var ts [MaxPerm]uint16
	var creature [MaxPerm]bool
	n := 0
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		ty := s.typesAt(k, sl)
		if ty&(kop.TCreature|kop.TArtifact) == 0 {
			continue
		}
		if n >= len(rec.cards) {
			s.stall(StallBoard)
			return
		}
		crt := ty&kop.TCreature != 0
		t := s.perms[sl].ts
		j := n
		for j > 0 {
			prevCrt := creature[j-1]
			if !crt && prevCrt {
				break // a non-creature sorts after every creature
			}
			if prevCrt == crt && ts[j-1] <= t {
				break
			}
			ts[j], creature[j], rec.cards[j] = ts[j-1], creature[j-1], rec.cards[j-1]
			j--
		}
		ts[j], creature[j], rec.cards[j] = t, crt, s.cardAt(sl)
		n++
	}
	rec.n = uint8(n)
	// A contracted loser who controls no artifact or creature still puts the
	// LosesGame trigger on the stack (it resolves doing nothing), so enqueue
	// even with an empty snapshot.
	ver, card := uint8(0), CardID(0)
	if src != NoObj && int(src) < int(s.nobj) {
		ver, card = s.objs[src].ver, s.objs[src].card
	}
	s.enqueue(src, ver, card, abDelayed, c, NoObj, p, contractLossTrigAmt)
}

// contractLossCopyResolve mints the copies staged for seat p when the
// LosesGame trigger resolves. A different controller (a stale staged record)
// or an Effect controller that has left the game makes none.
func (s *State) contractLossCopyResolve(k *prep, ctrl, p uint8) {
	pending := coldContractPending.of(s)
	if pending == nil || p >= MaxSeats || ctrl >= MaxSeats {
		return
	}
	rec := &pending[SeatIx(p)]
	if rec.ctrl != ctrl || rec.n == 0 {
		return
	}
	n := rec.n
	rec.n = 0
	if s.out(ctrl) {
		return
	}
	for i := uint8(0); i < n; i++ {
		if s.mintTokenCopyCard(k, ctrl, rec.cards[i]) {
			return // the board is full (StallBoard)
		}
	}
}

// mintTokenCopyCard mints a token that is a copy of card c, controlled by p.
// The staged copy's host permanent has already left the battlefield (the CR
// 800.4a sweep ran before the trigger resolved), so the token is based on the
// snapshotted card rather than a live template slot.
func (s *State) mintTokenCopyCard(k *prep, p uint8, c CardID) bool {
	if s.nearCapacity(k) {
		s.stall(StallBoard)
		return true
	}
	t := s.newToken(c, p)
	s.objs[t].of |= ofCopy
	var in kop.Instr
	s.copyTokenEnterFrom(k, &in, t, NoSlot, NoSlot, nil)
	return false
}
