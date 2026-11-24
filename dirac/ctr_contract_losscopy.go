package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Miss Highwater's loss copy: "for as long as they have a contract counter,
// when they lose the game, for each artifact and creature they controlled,
// create a token that's a copy of it", the Effect its damage trigger makes for
// the player it contracts. The Effect is per seat here: owed[p] is the seat
// whose Effect owes p's loss copy (that seat plus one, 0 for none), set when
// the trigger's put gives p a contract counter (putContract) and read by
// leaveGame as p leaves the game while the game goes on (a pod; two seats end
// the game on the loss, so no copy is made there).
//
// The Effect's Static$ Always self-exile once p has no contract counter is
// not modelled: a contract counter is never removed in the corpus
// (/usr/bin/grep -rl Contract .cards/cardsfolder), so the record is never
// cleared by a count that falls to zero. leaveGame clears it as it fires.
//
// The record is a cold array of one byte per seat, kept only when
// genPrep.anyContract (the contract counts' own condition).

var coldContractOwed = newCold[uint8]("contractOwed", func(g *genPrep) int {
	if g.anyContract {
		return MaxSeats
	}
	return 0
})

// contractOwedBy is the seat whose Effect owes seat p's loss copy, ok false
// when none does.
func (s *State) contractOwedBy(p uint8) (c uint8, ok bool) {
	if o := coldContractOwed.of(s); o != nil && p < MaxSeats && o[SeatIx(p)] != 0 {
		return o[SeatIx(p)] - 1, true
	}
	return 0, false
}

// setContractOwed records that controller c's Effect owes seat p's loss copy.
func (s *State) setContractOwed(p, c uint8) {
	if o := coldContractOwed.of(s); o != nil && p < MaxSeats && c < MaxSeats {
		o[SeatIx(p)] = c + 1
	}
}

// contractLossCopy makes the token copies seat p's owed record names as p
// leaves the game: one for each artifact and creature p controls, in object
// order, controlled by the Effect's controller c (a seat that left too gets
// none). It runs before p's permanents leave (leaveGame) and clears the record.
func (s *State) contractLossCopy(k *prep, p uint8) {
	c, ok := s.contractOwedBy(p)
	if !ok {
		return
	}
	coldContractOwed.of(s)[SeatIx(p)] = 0
	if s.out(c) {
		return
	}
	var host [MaxPerm]Obj
	n := 0
	for i := 0; i < int(s.nobj); i++ {
		ob := &s.objs[Obj(i)]
		if ob.zone != kop.ZoneBattlefield || !s.ctrlM[SeatIx(p)].has(ob.slot) {
			continue
		}
		if s.typesAt(k, ob.slot)&(kop.TCreature|kop.TArtifact) != 0 {
			host[n] = Obj(i)
			n++
		}
	}
	in := kop.Instr{Op: kop.OpCreateToken}
	for _, h := range host[:n] {
		if s.mintTokenCopy(k, &in, c, h) {
			return // the board is full (StallBoard)
		}
	}
}
