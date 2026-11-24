package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Miss Highwater's loss copy: "for as long as they have a contract counter,
// when they lose the game, for each artifact and creature they controlled,
// create a token that's a copy of it", the Effect its damage trigger makes for
// the player it contracts. Gorge arms the same Effect as a delayed LosesGame
// registration (effects.effEffect): at the PlayerLost emit it queues a fire,
// and at the batch end the fired ability goes on the stack (NStack) with the
// loser's battlefield read as its referents (TriggerLostBoard); the copies are
// minted when it resolves, controlled by the Effect's controller. The kernel
// does the same with an abDelayed entry (contractLossAmt): the fire is
// markLost's (at every loss site, before the CR 800.4a walk, the board
// captured here), the resolution is resolveContractLoss. A pod, where a loss does not end the
// game, is the only case that fires: in two seats the game ends on the loss
// and gorge drops the queued fire (G.Over), so the kernel fires nothing there.
//
// The Effect is per seat here: owed[p] is the seat whose Effect owes p's loss
// copy (that seat plus one, 0 for none), set when the trigger's put gives p a
// contract counter with the ext op's C set (putContract; kgen arms it from the
// Effect's resolution). The |SB marker on gorge's registration ends it when
// the Effect's source leaves the battlefield (contractSourceLeft), and
// putContract's record carries that source (src).
//
// The captured board is the losers' artifacts and creatures at the fire, in
// zone order, creatures first: gorge's lostBoardDefined union walks its two
// groups in script order (Creature, then Artifact) over TriggerLostBoard, and
// a token copy carries the card alone -- no counters or attachments (CR
// 707.7), entering untapped (copypermanent.go) -- so the card id is all the
// copy needs. The Effect's Static$ Always self-exile once p has no contract
// counter is not modelled on either side: a contract counter is never removed
// in the corpus (/usr/bin/grep -rl Contract .cards/cardsfolder), so the record
// is never cleared by a count that falls to zero. The fire clears it as it
// fires.
//
// Approximation: the copy is minted from the captured card id, so a permanent
// whose copiable values come from a copy effect (a Clone) or a face-down
// permanent is copied as its printed card, not as its copiable values
// (CR 707.2). Gorge copies the copiable values; the kernel does not record
// them.
//
// The record and the capture are cold arrays, kept only when
// genPrep.anyContract (the contract counts' own condition).

var coldContractOwed = newCold[uint8]("contractOwed", func(g *genPrep) int {
	if g.anyContract {
		return MaxSeats
	}
	return 0
})

// coldContractSrc is the Effect's source (the Miss Highwater the put resolved
// on) beside each owed record, so the |SB bound can end the record (the leave
// walk) and the fire can name it as the abDelayed entry's source, the object
// gorge's stack ability names.
var coldContractSrc = newCold[Obj]("contractSrc", func(g *genPrep) int {
	if g.anyContract {
		return MaxSeats
	}
	return 0
})

// coldContractCopy is each seat's captured loss-copy board, MaxPerm card ids
// in mint order, with coldContractCopyN holding its length.
var coldContractCopy = newCold[CardID]("contractCopy", func(g *genPrep) int {
	if g.anyContract {
		return MaxSeats * MaxPerm
	}
	return 0
})

var coldContractCopyN = newCold[uint8]("contractCopyN", func(g *genPrep) int {
	if g.anyContract {
		return MaxSeats
	}
	return 0
})

// contractLossAmt is the abDelayed entry's trigAmt marking the loss copy
// (speed's is -1, the venture's -2, the monarch draw's -3, the monarch steal's
// -4, the rad drain's -5, the Ring emblem's -6..-8, the extra turn's -9);
// trigPl is the loser.
const contractLossAmt = -10

// contractOwedBy is the seat whose Effect owes seat p's loss copy, ok false
// when none does.
func (s *State) contractOwedBy(p uint8) (c uint8, ok bool) {
	if o := coldContractOwed.of(s); o != nil && p < MaxSeats && o[SeatIx(p)] != 0 {
		return o[SeatIx(p)] - 1, true
	}
	return 0, false
}

// setContractOwed records that controller c's Effect (its source src) owes
// seat p's loss copy.
func (s *State) setContractOwed(p, c uint8, src Obj) {
	if o := coldContractOwed.of(s); o != nil && p < MaxSeats && c < MaxSeats {
		o[SeatIx(p)] = c + 1
	}
	if so := coldContractSrc.of(s); so != nil && p < MaxSeats {
		so[SeatIx(p)] = src
	}
}

// contractSourceLeft ends the owed records whose Effect source left the
// battlefield (the leave walk): gorge's registration carries |SB, so the
// source's departure ends the grant (CR 611.2).
func (s *State) contractSourceLeft(o Obj) {
	owed, src := coldContractOwed.of(s), coldContractSrc.of(s)
	if owed == nil || src == nil {
		return
	}
	for p := uint8(0); p < MaxSeats; p++ {
		if owed[SeatIx(p)] != 0 && src[SeatIx(p)] == o {
			owed[SeatIx(p)] = 0
			src[SeatIx(p)] = 0
		}
	}
}

// contractLossFire fires seat p's owed loss copy as p is marked lost, before
// p's permanents leave: the board is captured here (the CR 800.4a walk would
// take it), the record is spent, and the abDelayed entry is enqueued for the
// Effect's controller, named on the Effect's source -- gorge's stack ability
// is the registration's source and resolves the copy after the sweep, from
// its TriggerLostBoard.
func (s *State) contractLossFire(k *prep, p uint8) {
	c, ok := s.contractOwedBy(p)
	if !ok {
		return
	}
	src := coldContractSrc.of(s)[SeatIx(p)]
	coldContractOwed.of(s)[SeatIx(p)] = 0
	if s.out(c) {
		return // gorge: the registration ended with its controller (delayedRegistrationLive)
	}
	if int(src) >= MaxObj || s.objs[src].zone != kop.ZoneBattlefield {
		return // |SB: the source-battlefield bound the leave walk has not yet caught
	}
	if s.objs[src].owner() == p || s.ctrlOfObj(src) == p {
		// the source leaves with this very sweep (the loser owns or controls
		// it): gorge's queued fire is dropped with its departed source
		// (dropDepartedTriggers) before it is ever put on the stack
		return
	}
	on := 0
	for q := uint8(0); q < s.nSeats(); q++ {
		if q != p && !s.out(q) {
			on++
		}
	}
	if on < 2 {
		return // the loss ends the game: gorge drops the queued fire (G.Over)
	}
	cap, n := coldContractCopy.of(s), coldContractCopyN.of(s)
	if cap == nil || n == nil {
		return
	}
	base := int(SeatIx(p)) * MaxPerm
	m := 0
	add := func(o Obj) {
		if m < MaxPerm {
			cap[base+m] = s.objs[o].card
			m++
		}
	}
	// the board in zone order (FeatBattlefield's entry order, gorge's
	// TriggerLostBoard), creatures first, then the artifacts that are not
	// creatures: lostBoardDefined's two groups in script order
	var bfArr [MaxPerm]Obj // stack, not a heap scratch: the fire runs mid-walk
	bf := s.FeatBattlefield(p, bfArr[:0])
	for _, o := range bf {
		if s.typesAt(k, s.objs[o].slot)&kop.TCreature != 0 {
			add(o)
		}
	}
	for _, o := range bf {
		if s.typesAt(k, s.objs[o].slot)&(kop.TCreature|kop.TArtifact) == kop.TArtifact {
			add(o)
		}
	}
	n[SeatIx(p)] = uint8(m)
	s.enqueue(src, s.objs[src].ver, s.objs[src].card, abDelayed, c, NoObj, p, contractLossAmt)
}

// resolveContractLoss resolves the fired loss copy: one token copy of each
// captured artifact and creature, controlled by the Effect's controller
// (e.ctrl), in capture order. A copy carries the card alone -- no counters or
// attachments (CR 707.7), entering untapped -- so the capture's card id is
// all of it.
//
//go:noinline
func (s *State) resolveContractLoss(k *prep, e *stackEntry) {
	cap, n := coldContractCopy.of(s), coldContractCopyN.of(s)
	if cap == nil || n == nil {
		return
	}
	p := e.trigPl
	base := int(SeatIx(p)) * MaxPerm
	m := int(n[SeatIx(p)])
	if m > MaxPerm {
		m = MaxPerm
	}
	n[SeatIx(p)] = 0
	if s.out(e.ctrl) {
		return // the Effect's controller left: gorge's registration is gone
	}
	if int(e.obj) < MaxObj && (s.objs[e.obj].zone != kop.ZoneBattlefield || s.objs[e.obj].ver != e.srcVer) {
		return // |SB: the source left (or left and came back, a new object) between the fire and the resolution
	}
	for i := 0; i < m; i++ {
		if s.mintLossCopy(k, e.ctrl, cap[base+i]) {
			return // the board is full (StallBoard)
		}
	}
}

// mintLossCopy mints one token under p that is a copy of the card c (a ceased
// host's card): the copy's token-ness, its enter and its ETB are the copy of
// a live permanent's (mintTokenCopy), from the card alone -- the template
// slot is gone by the time the fired copy resolves.
// True when the board is full (a stall).
func (s *State) mintLossCopy(k *prep, p uint8, c CardID) bool {
	if s.nearCapacity(k) {
		s.stall(StallBoard)
		return true
	}
	t := s.newToken(c, p)
	s.objs[t].of |= ofCopy
	var cp kop.Instr
	e := &s.res.e
	was := e.ctrl
	e.ctrl = p
	s.copyTokenEnterFrom(k, &cp, t, NoSlot, NoSlot, nil)
	e.ctrl = was
	s.res.lastTok = t
	return false
}
