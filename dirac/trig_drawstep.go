package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// drawStepFirst reports whether the Draw just taken by who is the first card
// that player drew in their own draw step (CR 504.1). drawStepN counts the
// active player's draws since entry to the draw step, the same ephemeral fact
// gorge's trigmatch.firstCardInDrawStep derives from the event log. A draw by
// a non-active seat during another player's draw step is never that seat's
// turn-based draw, so it is never "first".
func (s *State) drawStepFirst(who uint8) bool {
	return s.step == kop.StepDraw && who == s.active && s.g.drawStepN == 1
}

// drawStepGateOK applies a Drawn trigger's FirstCardInDrawStep$ qualifier:
// True wants the first card of the drawer's own draw step, False wants every
// other draw (gorge trigmatch.drawnMatches).
func drawStepGateOK(t *kop.Trigger, first bool) bool {
	if t.Flags5&kop.Tf5FirstCardInDrawStep != 0 && !first {
		return false
	}
	if t.Flags5&kop.Tf5NotFirstCardInDrawStep != 0 && first {
		return false
	}
	return true
}
