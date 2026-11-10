package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// untapLockEffect lowers gorge's "that permanent doesn't untap (or can't
// become untapped) for as long as you control CARDNAME" Effect: Duration$
// AsLongAsControl over one Event$ Untap | Layer$ CantHappen |
// ValidCard$ Card.IsRemembered replacement, RememberObjects$ Targeted (the
// parent's target) and ForgetOnMoved$ Battlefield. With
// ValidStepTurnToController$ You the lock is the untap step's only
// (OpSkipUntap C kop.SkipUntapLockStep: Ty Lee, Chi Blocker); with none, every
// untap event is replaced (kop.SkipUntapLockAny: Spider-Woman, Secret Agent).
// The kernel keeps a record of the target, the ability's source and its
// controller (dirac/act_untaplock.go); the lock lives while the source is on
// the battlefield under that player. It reports false, with nothing consumed,
// when the replacement is no Untap line, and refuses an Untap line out of
// that shape by name. Only a triggered or an activated ability can lower it:
// a spell's source is gone when it resolves.
func (lc *lowerCtx) untapLockEffect(ps *params) (bool, error) {
	name := strings.TrimSpace(ps.str("ReplacementEffects"))
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || strings.Contains(name, ",") || r.Event != "Untap" {
		return false, nil
	}
	if lc.ab.Kind != akTriggered && lc.ab.Kind != akActivated {
		return true, refuse("Effect Duration$ AsLongAsControl untap lock outside a triggered or activated ability")
	}
	rp := newParams(r.Params)
	rp.consume("Event")
	if !rp.is("Layer", "CantHappen") || !rp.is("ValidCard", "Card.IsRemembered") {
		return true, refuse("Effect AsLongAsControl untap replacement shape")
	}
	in := lInstr{Op: "SkipUntap", C: kop.SkipUntapLockAny}
	if v, ok := rp.get("ValidStepTurnToController"); ok {
		if strings.TrimSpace(v) != "You" {
			return true, refuse("Effect AsLongAsControl untap replacement ValidStepTurnToController$ %q", v)
		}
		in.C = kop.SkipUntapLockStep
	}
	if left := rp.unread(); len(left) > 0 {
		return true, refuse("Effect AsLongAsControl untap replacement unread %v", left)
	}
	if !ps.is("ForgetOnMoved", "Battlefield") {
		return true, refuse("Effect AsLongAsControl untap ForgetOnMoved$ %q", ps.str("ForgetOnMoved"))
	}
	if _, ok := ps.get("ExileOnMoved"); ok {
		return true, refuse("Effect AsLongAsControl untap ExileOnMoved$")
	}
	if ro := ps.str("RememberObjects"); ro != "Targeted" {
		return true, refuse("Effect AsLongAsControl untap RememberObjects %q", ro)
	}
	obj, err := lc.defined("Targeted")
	if err != nil {
		return true, err
	}
	if obj.Kind != rTarget {
		return true, refuse("Effect AsLongAsControl untap RememberObjects$ Targeted without a target")
	}
	t := lc.ab.Targets[obj.N]
	if t.F == nil || len(t.F.Clauses) == 0 || t.Zone != zBattlefield || !t.Max.isConst() || t.Max.N > 1 {
		return true, refuse("Effect AsLongAsControl untap target slot %d", obj.N)
	}
	in.Obj = obj
	lc.emit(in)
	return true, nil
}
