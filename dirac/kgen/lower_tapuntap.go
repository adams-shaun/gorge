package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// untapAll lowers api:UntapAll (gorge effUntapAll,
// engine/effects/combatfx.go:134): every tapped battlefield permanent
// matching ValidCards$ untaps. ValidTgts$ Player scopes the walk to the
// chosen player targets (allPlayersFor, :179).
func (lc *lowerCtx) untapAll(ps *params) error {
	for _, k := range []string{"RememberUntapped", "ControllerUntaps", "Defined"} {
		if _, ok := ps.get(k); ok {
			return refuse("UntapAll %s$", k)
		}
	}
	spec := ps.str("ValidCards")
	if spec == "" {
		return refuse("UntapAll without ValidCards$")
	}
	spec, err := lc.sweepTgtPlayer(ps, spec)
	if err != nil {
		return err
	}
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	note := ""
	if _, ok := ps.get("ValidTgts"); ok {
		note = sweepTgtPlayerNote
	}
	lc.emit(lInstr{Op: "UntapAll", F: f, Note: note})
	return nil
}

// tapAll lowers api:TapAll (gorge effTapAll: every untapped battlefield
// permanent matching ValidCards$ -- "Permanent" when absent -- becomes
// tapped, seat 0's battlefield first). A player ValidTgts$ or Defined$
// selector scopes the walk to that player's battlefield (gorge
// allPlayersFor); TapperController$ True gives each card's own controller
// the tap. RememberTapped$ stays unread.
func (lc *lowerCtx) tapAll(ps *params) error {
	spec := ps.str("ValidCards")
	if spec == "" {
		spec = "Permanent"
	}
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	in := lInstr{Op: "TapAll", F: f}
	if d, ok := ps.get("Defined"); ok {
		r, err := lc.defined(d)
		if err != nil {
			return err
		}
		switch r.Kind {
		case rYou, rEachOpp, rTarget, rTargetController, rTriggeredPlayer, rDefending:
			in.Obj = r
		default:
			return refuse("TapAll Defined$ %q is not a player selector", d)
		}
		in.Note = "only the defined player's permanents"
	}
	if vt, ok := ps.get("ValidTgts"); ok {
		pf, err := lc.filter(vt)
		if err != nil {
			return err
		}
		if len(pf.Clauses) != 0 {
			return refuse("TapAll object target")
		}
		t := lc.addTarget(lTarget{F: pf})
		in.Obj = lRef{Kind: rTarget, N: t}
		in.Note = "only the target player's permanents"
		ps.consume("TgtPrompt")
	}
	if v, ok := ps.get("TapperController"); ok {
		if !strings.EqualFold(strings.TrimSpace(v), "True") {
			return refuse("TapAll TapperController$ %q", v)
		}
		in.C = 1
	}
	lc.emit(in)
	return nil
}

// tapOrUntap lowers api:TapOrUntap (gorge effTapOrUntap): for its one
// object (a target or a Defined$ object) on the battlefield, the
// controller chooses tap or untap. Tapper$ and several-object slots refuse.
func (lc *lowerCtx) tapOrUntap(ps *params) error {
	if _, ok := ps.get("Tapper"); ok {
		return refuse("TapOrUntap Tapper$")
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	switch obj.Kind {
	case rSelf, rTriggered, rEquipped:
	case rTarget:
		t := lc.ab.Targets[obj.N]
		if t.F == nil || len(t.F.Clauses) == 0 || t.Zone != zBattlefield {
			return refuse("TapOrUntap of a non-permanent target")
		}
		if !t.Max.isConst() || t.Max.N > 1 {
			return refuse("TapOrUntap of several targets")
		}
	default:
		return refuse("TapOrUntap of %s", obj)
	}
	lc.emit(lInstr{Op: "TapOrUntap", Obj: obj})
	return nil
}

// untapChoose lowers Untap's UntapType$ + Amount$ chooser (gorge
// effUntap; Snap's "untap up to two lands") to OpUntapChoose. Defined$
// beside it (gorge narrows the candidates to the named players' permanents)
// refuses, as do UntapExactly$ and a Tap.
func (lc *lowerCtx) untapChoose(sa *cards.SA, ps *params) (bool, error) {
	ut, ok := ps.get("UntapType")
	if !ok {
		return false, nil
	}
	if sa.API != "Untap" {
		return true, refuse("Tap UntapType$")
	}
	for _, k := range []string{"Defined", "UntapExactly", "ValidTgts"} {
		if _, ok := ps.get(k); ok {
			return true, refuse("UntapType$ with %s$", k)
		}
	}
	f, err := lc.filter(strings.TrimSpace(ut))
	if err != nil {
		return true, err
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return true, err
	}
	in := lInstr{Op: "UntapChoose", A: n, F: f}
	if v, ok := ps.get("UntapUpTo"); ok {
		if strings.TrimSpace(v) != "True" {
			return true, refuse("UntapUpTo$ %q", v)
		}
		in.C = 1
	}
	lc.emit(in)
	return true, nil
}
