package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// One CounterPlayerAddedAll trigger per put action per recipient (gorge
// cr-pc-triggered-counter-map). A CounterTypes$ list, or a PutCounterAll's
// second sweep, is several kernel instructions with one action between them:
// g.ctrCell holds the recipients the action has already placed on, and a
// Tf4CounterCell listener does not fire again for one of them
// (kop.PutCtrCellFirst, kop.PutCtrCellNext). A recipient on which nothing
// landed yet still fires: gorge queues at the first matching event. Every
// other counter-added line keeps its per-event fire.

// DebugCtrCellOff turns the merge off: the pre-cr-pc-triggered-counter-map
// gorge. Test-only (TestCtrcellMutation).
var DebugCtrCellOff bool

// ctrCellHitBit is the counter-cell merge's bit in genState.ctrCellHit (a flag
// mask, not seat arithmetic); the bits above it are the CounterAddedAll batch
// marker.
const ctrCellHitBit uint8 = 1

// ctrCellHit reports the counter-cell merge's bit (genState.ctrCellHit bit 0).
func (s *State) ctrCellHit() bool { return s.g.ctrCellHit&ctrCellHitBit != 0 }

// setCtrCellHit sets the counter-cell merge's bit, leaving the packed
// CounterAddedAll batch marker (bits 1-7) alone.
func (s *State) setCtrCellHit(on bool) {
	if on {
		s.g.ctrCellHit |= ctrCellHitBit
	} else {
		s.g.ctrCellHit &^= ctrCellHitBit
	}
}

// fireCounterCell fires CounterAddedOnce for the n counters a cell-carrying
// instruction just placed on sl, and enters sl in the action's cell.
func (s *State) fireCounterCell(k *prep, sl Slot, n int) {
	if n <= 0 || !s.evLive(kop.TrigCounterAddedOnce) {
		return
	}
	s.setCtrCellHit(s.g.ctrCell.has(sl) && !DebugCtrCellOff)
	s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(n))
	s.setCtrCellHit(false)
	s.g.ctrCell.set(sl)
}
