package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The as-enters opponent choice (CR 614.12): an enters program opening with
// OpExt ExtChoosePlayer (kgen lower_chooseplayerenter.go; The Rack, Black
// Vise). gorge runs the body through effChoosePlayer as the permanent enters:
// a KChoose of one "player" option per opponent of the controller, in
// AliveFrom(controller) order, with the permanent already on the battlefield
// (so the kernel queues the ask, never a pre-entry one on the stack). With a
// single opponent the oracle answers that ask in place (forcedPlayerChoice),
// so the kernel records it unasked. The answer is kept in the permanent's chosen byte (the seat; 255 = none),
// which RefChosenPlayer and WhoseChosen read.

// playerOptions fills the as-enters opponent ask: one option per opponent of
// the chooser (d.Player) in turn order after them, mode = the seat.
//
//go:noinline
func (s *State) playerOptions(d *Decision) {
	d.Sub = ChGenPlayer
	q := s.oppsFrom(uint8(d.Player))
	for i := uint8(0); i < q.n; i++ {
		d.add(MakeAction(AkChoose, 0, 0, q.s[i]))
	}
}

// soleOpponent is p's one living opponent when p has exactly one (gorge's
// single-option ChoosePlayer, answered without an ask), else 255.
func (s *State) soleOpponent(p uint8) uint8 {
	if q := s.oppsFrom(p); q.n == 1 {
		return q.s[0]
	}
	return 255
}

// chosenPlayerOf is the seat the permanent at sl chose as it entered (255 =
// none).
func (s *State) chosenPlayerOf(sl Slot) uint8 {
	if sl == NoSlot || !s.occ.has(sl) {
		return 255
	}
	if ch := s.perms[sl].chosen; ch < s.nSeats() {
		return ch
	}
	return 255
}

// chosenPlayerSeat is kop.RefChosenPlayer for entry e: the seat its source
// permanent chose, while the source is still that permanent on the
// battlefield (gorge reads the source's live Object.Chosen, which it clears
// as the permanent leaves, CR 400.7).
func (s *State) chosenPlayerSeat(e *stackEntry) uint8 {
	if e == nil || e.obj == NoObj {
		return 255
	}
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer {
		return 255
	}
	return s.chosenPlayerOf(ob.slot)
}

// chosenIs is kop.WhoseChosen: the listener at sl chose player p as it
// entered.
func (s *State) chosenIs(sl Slot, p uint8) bool {
	ch := s.chosenPlayerOf(sl)
	return ch != 255 && ch == p
}
