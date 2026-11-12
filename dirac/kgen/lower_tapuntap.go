package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// untapAll lowers api:UntapAll (gorge effUntapAll,
// engine/effects/combatfx.go:134): every tapped battlefield permanent
// matching ValidCards$ untaps. ValidTgts$ Player scopes the walk to the
// chosen player targets (allPlayersFor, :179). Defined$ "You" folds into the
// filter's YouCtrl word: gorge's defined-player walk over one seat and the
// filter's controller clause admit the same permanents (Curse of Bounty's
// "untap up to two lands you control" leg). Any other Defined$ value refuses.
func (lc *lowerCtx) untapAll(ps *params) error {
	if v, ok := ps.get("Defined"); ok {
		if strings.TrimSpace(v) != "You" {
			return refuse("UntapAll Defined$ %q", v)
		}
		ps.consume("Defined")
	}
	for _, k := range []string{"RememberUntapped", "ControllerUntaps"} {
		if _, ok := ps.get(k); ok {
			return refuse("UntapAll %s$", k)
		}
	}
	spec := ps.str("ValidCards")
	if spec == "" {
		if _, was := ps.get("Defined"); was {
			return refuse("UntapAll Defined$ without ValidCards$")
		}
		return refuse("UntapAll without ValidCards$")
	}
	if _, ok := ps.m["Defined"]; ok {
		spec += "+YouCtrl" // Defined$ You folded in (gorge's defined-player walk)
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
// tapped, seat 0's battlefield first). A player ValidTgts$ scopes the walk
// to the target player's battlefield (gorge allPlayersFor); Defined$,
// RememberTapped$ and TapperController$ stay unread.
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
	lc.emit(in)
	return nil
}

// tapOrUntap lowers api:TapOrUntap (gorge effTapOrUntap): for its one
// object (a target or a Defined$ object) on the battlefield, the
// controller chooses tap or untap. Several-object slots refuse. Tapper$ /
// TargetingPlayer$ are read on an AttackersDeclared trigger only (Curse of
// Inertia's "target permanent controlled by enchanted player's controller":
// Tapper$ names the tap's provenance — the kernel's OpTap W read feeds the
// Taps triggers; TargetingPlayer$ moves the TARGET ask's Player to the
// queue-time trigger player, legality still judged from the controller —
// TgfTrigChooser); both refuse everywhere else.
func (lc *lowerCtx) tapOrUntap(ps *params) error {
	inAtkTrig := func() bool {
		tr := lc.ab.Trigger
		return tr != nil && tr.Event == "AttackersDeclared"
	}
	if v, ok := ps.get("Tapper"); ok {
		if strings.TrimSpace(v) != "TriggeredAttackingPlayer" || !inAtkTrig() {
			return refuse("TapOrUntap Tapper$ %q outside an AttackersDeclared trigger", v)
		}
	}
	trigChooser := false
	if v, ok := ps.get("TargetingPlayer"); ok {
		if strings.TrimSpace(v) != "TriggeredAttackingPlayer" || !inAtkTrig() {
			return refuse("TapOrUntap TargetingPlayer$ %q outside an AttackersDeclared trigger", v)
		}
		trigChooser = true
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	switch obj.Kind {
	case rSelf, rTriggered, rEquipped:
	case rTarget:
		t := &lc.ab.Targets[obj.N]
		if t.F == nil || len(t.F.Clauses) == 0 || t.Zone != zBattlefield {
			return refuse("TapOrUntap of a non-permanent target")
		}
		if !t.Max.isConst() || t.Max.N > 1 {
			return refuse("TapOrUntap of several targets")
		}
		if trigChooser {
			t.TrigChooser = true // the target ask goes to the trigger player
		}
	default:
		return refuse("TapOrUntap of %s", obj)
	}
	if trigChooser && obj.Kind != rTarget {
		return refuse("TapOrUntap TargetingPlayer$ without a target ask")
	}
	in := lInstr{Op: "TapOrUntap", Obj: obj}
	if _, ok := ps.m["Tapper"]; ok {
		in.W = lRef{Kind: rTriggeredPlayer} // the tap's provenance (OpTap's W read)
	}
	lc.emit(in)
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
