package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A RepeatEach over players with ChangeZoneTable$ True (Adeline, Resplendent
// Cathar: "for each opponent, create a 1/1 ... token that's tapped and
// attacking that player") is one zone batch: gorge's effRepeatEach opens its
// zone bracket around the whole loop and the held enters checks fire once the
// last iteration is done, as one batch (CR 603.6a; a "whenever one or more
// tokens you control enter" trigger fires once, counting them all). The loop
// may stop between iterations on an ask (a token's defender among a player
// and their planeswalkers, CR 508.4), so the frame lives in genState: the
// OpRepeatEachPlayer op opens it in s.g.each.hold, free while no each-target
// loop is parked (the loop's body holds no target: kgen repeatEachPlayers),
// and loopBackGen closes it when the loop's last player is done.

// DebugZoneLoopPerIteration is the mutation switch: no frame is opened, each
// iteration's entries fire their own enters checks, the old two-seat reading.
// Tests only.
var DebugZoneLoopPerIteration bool

// zoneLoopOpen opens the loop's frame for OpRepeatEachPlayer in.
func (s *State) zoneLoopOpen(in *kop.Instr) {
	if in.C&kop.RepeatEachZoneBatch == 0 || DebugZoneLoopPerIteration || s.g.each.j != 0 {
		return
	}
	s.g.each = eachSusp{hold: s.holdEntries()}
}

// zoneLoopHeld reports whether a RepeatEach zone batch is open: entries hold
// their enters checks for it.
func (s *State) zoneLoopHeld() bool { return s.g.each.j == 0 && s.g.each.hold.on }

// zoneLoopClose ends the loop's frame: the held enters checks fire, in entry
// order, as one batch.
func (s *State) zoneLoopClose(k *prep) {
	hr := s.g.each.hold
	s.g.each = eachSusp{}
	s.holdClose(k, hr)
}

// enterTok is enter for a minted token: inside a RepeatEach zone batch its
// enters check waits for the loop's end.
func (s *State) enterTok(k *prep, o Obj, ctrl uint8, tapped bool) Slot {
	if !s.zoneLoopHeld() {
		return s.enter(k, o, ctrl, tapped, kop.ZoneLibrary)
	}
	sl := s.enterNF(k, o, ctrl, tapped, kop.ZoneLibrary, 0)
	s.g.held |= 1 << (s.perms[sl].ts & 7)
	return sl
}

// DebugTokAtkNextSeat is a mutation switch: Adeline's token attacks the next
// living seat after its controller whichever opponent the loop binds, the old
// two-seat reading. Tests only.
var DebugTokAtkNextSeat bool
