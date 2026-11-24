package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The recorded chosen colour of a source (gorge events.Choose, Obj the source,
// Counter "color"; Object.ChosenColor), the byte an as-enters ChooseColor
// answer lands in (turn_gen.go answerAsk: perm.chosen = the option's colour
// index, 0..4 = WUBRG, 255 none). Count$Devotion.Chosen reads it
// (kop.ExDevotionChosen).

// recordChosenColor records colour col (0..4 = WUBRG) on the permanent at sl
// and on its card object, so a source that later leaves keeps the choice as
// gorge's Object.ChosenColor does.
func (s *State) recordChosenColor(sl Slot, col uint8) {
	if sl == NoSlot || col > 4 {
		return
	}
	s.perms[sl].chosen = objWord(col)
	if o := s.perms[sl].obj; o != NoObj {
		s.objs[o].chosen = col
	}
	s.touchSl(sl)
}

// clearChosenColor forgets the colour recorded on the permanent at sl and on
// its card object (gorge `DB$ Cleanup | ClearChosenColor$ True`).
func (s *State) clearChosenColor(sl Slot) {
	if sl == NoSlot {
		return
	}
	s.perms[sl].chosen = 255
	if o := s.perms[sl].obj; o != NoObj {
		s.objs[o].chosen = 255
	}
	s.touchSl(sl)
}

// chosenColorOf is the colour recorded on the evaluating source (0..4), and
// false with none: the permanent's byte while it is on the battlefield, else
// its object's.
func (s *State) chosenColorOf(ctx evalCtx) (uint8, bool) {
	ch := uint8(255)
	if ctx.srcSl != NoSlot {
		ch = uint8(s.perms[ctx.srcSl].chosen)
	} else if ctx.src != NoObj {
		ch = s.objs[ctx.src].chosen
	}
	return ch, ch < 5
}

// exDevotionChosen is kop.ExDevotionChosen: the controller's devotion to the
// source's recorded colour, 0 with none.
func (s *State) exDevotionChosen(k *prep, ctx evalCtx) int {
	ch, ok := s.chosenColorOf(ctx)
	if !ok {
		return 0
	}
	return s.devotion(k, kop.ColorMask(1)<<ch, ctx.ctrl)
}
