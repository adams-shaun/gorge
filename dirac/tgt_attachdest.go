package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the destination-side Attach pick (gorge effAttach with
// Object$ and Choices$: Fumble's "attach them to another creature"). The
// objects are the resolution's remembered set (the Auras and Equipment an
// earlier ExtRememberAll gathered); the resolving controller picks ONE
// battlefield permanent the filter admits and the AttachObj right after
// (Obj = RefChosen, C = RefRemembered) fastens every object still on the
// battlefield to it. gorge keeps a destination only when attachableBy admits
// it: some object other than the destination itself (Attachable is always
// true, the attachment SBAs own the rest). No destination: no ask, nothing
// chosen (gorge's "cannot attach: no legal target" Note); one: taken unasked
// (the mandatory auto-take); otherwise one KChoose Min 1 Max 1 over the pool
// in battlefield order. The ChooseCard carries C = chooseAttachDest.
const chooseAttachDest = 10

// attachDestScan walks the battlefield for the pick's destinations: the
// permanents f admits for chooser p that some remembered object still on the
// battlefield can attach to. It lists them into d (when non-nil) and returns
// their count and the first.
func (s *State) attachDestScan(k *prep, pc uint8, p uint8, d *Decision) (n int, first Obj) {
	live, only := 0, NoSlot
	for i := 0; i < int(s.g.nrem); i++ {
		if o := s.g.rem[i]; o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
			live++
			only = s.objs[o].slot
		}
	}
	if live == 0 {
		return 0, NoObj // gorge's objs are empty after the attachObjectGone filter
	}
	f := s.prog(k)[pc].F
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if live == 1 && sl == only || !s.matchPermGen(k, f, sl, p, s.resSrcSlot(k), layerFull) {
			continue
		}
		o := s.perms[sl].obj
		if n == 0 {
			first = o
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
	return n, first
}

// attachDest is OpChooseCard for chooseAttachDest.
func (s *State) attachDest(k *prep, p uint8) (ok, done bool) {
	n, first := s.attachDestScan(k, s.res.pc, p, nil)
	switch {
	case n == 0:
		return true, true
	case n == 1:
		s.g.chosen[0], s.g.nchosen = first, 1
		return true, true
	}
	s.res.ask, s.res.askP = raGenChooseCard, p
	return false, true
}
