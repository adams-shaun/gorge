package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The api: effects for draw, discard, mill, scry and mana (lower_api.go).
func init() {
	apiOp("Draw", (*lowerCtx).apiDraw)
	apiPS("Discard", (*lowerCtx).discard)
	apiOp("Recruit", (*lowerCtx).recruit) // lower_recruit.go
	apiOp("Mill", (*lowerCtx).apiMill)
	apiOp("Surveil", (*lowerCtx).apiSurveil)
	apiPS("Scry", (*lowerCtx).scry)
	apiPS("DrainMana", (*lowerCtx).drainMana)
	apiPS("Mana", (*lowerCtx).manaEffect)
}

func (lc *lowerCtx) apiDraw(sa *cards.SA, ps *params) error {
	if ps.is("Defined", "TriggeredCardOpponent") {
		return lc.drawTriggerOpps(ps) // lower_triggeropps.go
	}
	if _, ok := ps.get("Reveal"); ok {
		// gorge's Draw never reads Reveal$ (draw_params.go's
		// drawKnownKeys omits it): effDraw emits no reveal decision, only
		// the unread-parameter Note (draw.go noteUnreadParams), which is
		// information only on a draw. Any value -- the corpus's Reveal$
		// All and Reveal$ True -- reads the same, so the kernel reads the
		// value and lowers nothing (Fa'adiyah Seer, Sindbad, Jace, Mirror
		// Mage).
	}
	remDrawn, remAll, err := rememberDrawn(ps)
	if err != nil {
		return err
	}
	// Reveal$ (Fa'adiyah Seer's "draw a card and reveal it"): gorge's
	// compileDraw reads no Reveal key (it is an unread Note) and its draw
	// emits no reveal event, so the drawn card changes nothing a later
	// node reads; the remembered card (RememberDrawn$) is the whole effect.
	ps.consume("Reveal")
	if who, ok, err := lc.manaSpenderYouAnd(ps); err != nil { // lower_manaspender.go
		return err
	} else if ok {
		if remDrawn || remAll {
			return refuse("RememberDrawn$ on a two-player draw")
		}
		if _, ok := ps.get("OptionalDecider"); ok {
			return refuse("OptionalDecider$ on a two-player draw")
		}
		n, err := lc.amount(ps, "NumCards", 1)
		if err != nil {
			return err
		}
		lc.emit(lInstr{Op: "Draw", Obj: who, A: n})
		return nil
	}
	if who, ok, err := lc.targetedAndYou(ps, "Defined"); err != nil {
		return err
	} else if ok {
		if remDrawn {
			return refuse("RememberDrawn$ on a two-player draw")
		}
		if _, ok := ps.get("OptionalDecider"); ok {
			return refuse("OptionalDecider$ on a two-player draw")
		}
		n, err := lc.amount(ps, "NumCards", 1)
		if err != nil {
			return err
		}
		lc.emit(lInstr{Op: "Draw", Obj: who, A: n})
		lc.emit(lInstr{Op: "Draw", Obj: lRef{Kind: rYou}, A: n})
		return nil
	}
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "NumCards", 1)
	if err != nil {
		return err
	}
	if err := lc.drawDecider(ps, who); err != nil { // lower_optional.go
		return err
	}
	if ps.is("Upto", "True") {
		return lc.drawUpto(ps, who, n, remDrawn || remAll) // lower_drawupto.go
	}
	in := lInstr{Op: "Draw", Obj: who, A: n}
	switch {
	case remAll:
		in.CS = "RememberAll"
	case remDrawn:
		in.CS = "Remember"
	}
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) apiMill(sa *cards.SA, ps *params) error {
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "NumCards", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "Mill", Obj: who, A: n}
	if ps.is("RememberMilled", "True") {
		in.CS = "Remember"
	}
	if err := lc.millImprint(ps, &in); err != nil { // lower_imprintzone.go
		return err
	}
	if err := lc.mayGate(ps, kop.MayGuardMill, who); err != nil {
		return err
	}
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) apiSurveil(sa *cards.SA, ps *params) error {
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "Surveil", Obj: who, A: n}
	if rem, err := rememberParam(ps, "RememberMoved"); err != nil {
		return err
	} else if rem {
		in.CS = "Remember" // the cards put into the graveyard
	}
	if kept, err := rememberParam(ps, "RememberKept"); err != nil {
		return err
	} else if kept {
		if in.CS != "" {
			return refuse("Surveil RememberKept$ beside RememberMoved$")
		}
		in.CS = "RememberKept" // the cards left on top
	}
	lc.emit(in)
	return nil
}
