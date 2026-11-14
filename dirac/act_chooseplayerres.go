package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The mid-resolution chosen player (kop.ExtChoosePlayerRes; gorge
// effChoosePlayer, engine/effects/choose_control.go; Gluntch, the
// Bestower). The resolving controller is asked once over the living players
// in AliveFrom order (a "player" option each, ChGenPlayer, mode = the seat);
// the answer becomes the resolution's chosen player, which kop.RefChosenPlayerRes
// reads, and joins the remembered players that a later Choices$
// Player.!IsRemembered (kop.ChoosePlayerFresh) leaves out. A pool of one is
// taken unasked, as the oracle answers a single option in place.
//
// resCtx.cpl holds both: the low four bits are the remembered players (one
// bit per seat), cplHas says a player is chosen and cplLast is its seat.

const (
	cplRemMask   = 0x0f // the players this resolution's choices remembered
	cplLast      = 0x30 // the chosen seat
	cplLastShift = 4
	cplHas       = 0x40 // a player is chosen
)

// raChoosePlayerRes is the ask. It sits past the ask families that offset
// from raGenManifestDread (the highest is +82).
const raChoosePlayerRes = raGenManifestDread + 83

// choosePlayerResPool is the players the next choice may name: the living
// players from the controller, less the remembered ones with ChoosePlayerFresh.
func (s *State) choosePlayerResPool(in *kop.Instr) seatSeq {
	q := s.aliveFrom(s.res.e.ctrl)
	if in.C&kop.ChoosePlayerFresh == 0 {
		return q
	}
	var r seatSeq
	for i := uint8(0); i < q.n; i++ {
		if s.res.cpl&(1<<q.s[i]) == 0 {
			r.add(q.s[i])
		}
	}
	return r
}

// choosePlayerResRecord makes p the chosen player and remembers it.
func (s *State) choosePlayerResRecord(p uint8) {
	s.res.cpl = s.res.cpl&cplRemMask | 1<<p | p<<cplLastShift | cplHas
}

// chosenSeatRes is kop.RefChosenPlayerRes: the chosen seat, 255 when none.
func (s *State) chosenSeatRes() uint8 {
	if s.res.cpl&cplHas == 0 {
		return 255
	}
	return (s.res.cpl & cplLast) >> cplLastShift
}

// choosePlayerResStart is OpExt ExtChoosePlayerRes: false with the ask posed.
// An empty pool leaves no chosen player (gorge's choiceRecord of no pick drops
// the old player entry).
func (s *State) choosePlayerResStart(in *kop.Instr) bool {
	q := s.choosePlayerResPool(in)
	switch q.n {
	case 0:
		s.res.cpl &= cplRemMask
	case 1:
		s.choosePlayerResRecord(q.s[0])
	default:
		s.res.ask, s.res.askP = raChoosePlayerRes, s.res.e.ctrl
		return false
	}
	return true
}

// choosePlayerResOptions poses the ask: one option per player in the pool.
func (s *State) choosePlayerResOptions(k *prep, d *Decision) {
	d.Sub = ChGenPlayer
	q := s.choosePlayerResPool(&s.prog(k)[s.res.pc])
	for i := uint8(0); i < q.n; i++ {
		d.add(MakeAction(AkChoose, 0, 0, q.s[i]))
	}
}

// choosePlayerResAnswer records the answer and goes on.
func (s *State) choosePlayerResAnswer(k *prep, a Action) {
	s.choosePlayerResRecord(a.Mode())
	s.continueRes(k, 0)
}

// execPutCounterBy runs OpPutCounter with the Placer$ player W as the counters'
// adder (gorge emitPutCounterChange: SetCounterAdder around the CounterChange),
// which the counter replacements and the added-this-turn ledger read. A W that
// names no player leaves the default adder.
func (s *State) execPutCounterBy(k *prep, in *kop.Instr) bool {
	saved := s.g.ctrAdder
	if p := s.playerRef(k, in.W); p != 255 {
		s.g.ctrAdder = p + 1
	}
	ok := s.execPutCounterAt(k, in)
	s.g.ctrAdder = saved
	return ok
}
