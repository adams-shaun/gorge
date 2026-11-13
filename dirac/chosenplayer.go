package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The as-enters chosen player (The Rack, Black Vise; kop/chosenplayer.go).
// gorge's ChoosePlayer records the answer on the entering object
// (choose_control.go choiceRecord: Object.Chosen) and clears it when the
// object leaves the battlefield (events apply_zone.go); the kernel keeps the
// chosen seat in perm.chosen, which enterGen resets on every entry.

// enterChosenOpp is kop.ExtChosenOpp inside an enters program: the entering
// permanent's chosen player is its controller's one opponent (a two-player
// game; kgen marks the card two-seat).
func (s *State) enterChosenOpp() {
	o := s.g.replaced
	if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
		return
	}
	sl := s.objs[o].slot
	p := &s.perms[sl]
	p.chosen = s.nextAlive(p.ctrl)
	s.touchSl(sl)
}

// srcChosenPl is kop.RefSrcChosenPl: the chosen player of entry e's source
// permanent, read live (gorge reads g.Obj(c.Source).Chosen, the object's
// current incarnation); 255 when the source is not on the battlefield or
// chose no player.
func (s *State) srcChosenPl(e *stackEntry) uint8 {
	if e == nil || e.kind == seSpell || e.obj == NoObj {
		return 255
	}
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneBattlefield {
		return 255
	}
	if ch := s.perms[ob.slot].chosen; ch < s.nSeats() {
		return ch
	}
	return 255
}

// ctxChosenPl is kop.RefSrcChosenPl read in an evaluation context: the
// static's host or the entry's source (Entropic Specter's P/T, The Rack's
// damage), the same live read as srcChosenPl.
func (s *State) ctxChosenPl(ctx evalCtx) uint8 {
	o := ctx.src
	if o == NoObj && ctx.e != nil && ctx.e.kind != seSpell {
		o = ctx.e.obj
	}
	if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
		return 255
	}
	if ch := s.perms[s.objs[o].slot].chosen; ch < s.nSeats() {
		return ch
	}
	return 255
}

// chosenPlIs is kop.WhoseChosenPl: the listener at sl chose player p as it
// entered (gorge Player.Chosen over the trigger host's Object.Chosen).
//
//go:noinline
func (s *State) chosenPlIs(sl Slot, p uint8) bool {
	return sl != NoSlot && s.perms[sl].chosen == p
}
