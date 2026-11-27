package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// rememberedCantAttack lowers a permanent Effect that remembers the cards its
// own resolution remembered and carries one CantAttack static over them
// (Promise of Loyalty's "each of those creatures can't attack you or
// planeswalkers you control for as long as it has a vow counter on it"):
//
//	DB$ Effect | RememberObjects$ Remembered | ForgetOnMoved$ Battlefield |
//	  ForgetCounter$ VOW | Duration$ Permanent
//	Mode$ CantAttack | ValidCard$ Card.IsRemembered | Target$ You,Planeswalker.YouCtrl
//
// The emblem keeps the resolution's remembered set (kop.Emblem.CaptureRem),
// drops an object that leaves the battlefield (CaptureLapse) or has no counter
// of the ForgetCounter$ kind left (CaptureCtr), and applies the static to the
// set's members alone: the static's filter keeps the creature check, the
// remembered test being the capture gate (dirac embCaptureOK). The Target$
// scope reads as for any CantAttack static: "You" is the emblem's player, who
// is the Effect's controller. ok is false for any other shape, which keeps
// its own refusal.
func (lc *lowerCtx) rememberedCantAttack(ps *params, s cards.Static) (ok bool, err error) {
	if s.Mode != "CantAttack" || strings.TrimSpace(ps.m["RememberObjects"]) != "Remembered" {
		return false, nil
	}
	why := func(more string) (bool, error) {
		return true, refuse("Effect CantAttack over RememberObjects$ Remembered: %s", more)
	}
	if lc.repeatPl > 0 || !lc.hasRememberWriter() {
		return why("the set is not one this resolution wrote")
	}
	if strings.TrimSpace(ps.str("ForgetOnMoved")) != "Battlefield" {
		return why("no ForgetOnMoved$ Battlefield")
	}
	if strings.TrimSpace(s.Params["ValidCard"]) != "Card.IsRemembered" {
		return why("ValidCard$ " + s.Params["ValidCard"])
	}
	counter := strings.TrimSpace(ps.str("ForgetCounter"))
	if counter != "" {
		if _, cerr := counterKind(counter); cerr != nil {
			return why(cerr.Error())
		}
	}
	cp := make(map[string]string, len(s.Params))
	for k, v := range s.Params {
		cp[k] = v
	}
	cp["ValidCard"] = "Creature"
	s.Params = cp
	sts, err := lc.staticBody(s)
	if err != nil {
		return true, err
	}
	if len(sts) != 1 {
		return why("the static lowers to several records")
	}
	ps.consume("RememberObjects", "ForgetOnMoved", "ForgetCounter", "Duration", "Name", "Image", "Stackable")
	if left := ps.unread(); len(left) > 0 {
		return true, refuse("Effect static unread %v", left)
	}
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{
		Statics: sts, CaptureRem: true, CaptureLapse: true, CaptureCounter: counter,
	})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}, C: len(lc.out.Emblems) - 1})
	return true, nil
}
