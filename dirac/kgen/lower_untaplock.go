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
// the battlefield under that player. Duration$ AsLongAsInPlay and
// UntilHostLeavesPlay (Somnophore, Tidebinder Mage, Origin of Iron Man) lock
// for as long as the source is on the battlefield under any controller: the
// untap step's form only, OpSkipUntap C kop.SkipUntapLockHostStep.
// Duration$ UntilUntaps (Amber Prison, Ice Floe, Rust Tick, Whip Vine, Sand
// Squid and six more) is that form lasting only while the source remains
// tapped (kop.SkipUntapLockUntapStep); its Effect may carry ExileOnMoved$
// Battlefield in place of ForgetOnMoved$. It reports false, with nothing consumed,
// when the replacement is no Untap line, and refuses an Untap line out of
// that shape by name. Only a triggered or an activated ability can lower it:
// a spell's source is gone when it resolves.
func (lc *lowerCtx) untapLockEffect(ps *params) (bool, error) {
	dur := ps.str("Duration")
	host := dur != "AsLongAsControl"
	untap := dur == "UntilUntaps"
	name := strings.TrimSpace(ps.str("ReplacementEffects"))
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || strings.Contains(name, ",") || r.Event != "Untap" {
		return false, nil
	}
	if lc.ab.Kind != akTriggered && lc.ab.Kind != akActivated {
		return true, refuse("Effect Duration$ %s untap lock outside a triggered or activated ability", dur)
	}
	rp := newParams(r.Params)
	rp.consume("Event")
	if !rp.is("Layer", "CantHappen") || !rp.is("ValidCard", "Card.IsRemembered") {
		return true, refuse("Effect %s untap replacement shape", dur)
	}
	in := lInstr{Op: "SkipUntap", C: kop.SkipUntapLockAny}
	if v, ok := rp.get("ValidStepTurnToController"); ok {
		if strings.TrimSpace(v) != "You" {
			return true, refuse("Effect %s untap replacement ValidStepTurnToController$ %q", dur, v)
		}
		in.C = kop.SkipUntapLockStep
		if untap {
			in.C = kop.SkipUntapLockUntapStep
		} else if host {
			in.C = kop.SkipUntapLockHostStep
		}
	} else if host {
		return true, refuse("Effect untap lock Duration$ %s without ValidStepTurnToController$ You", dur)
	}
	if left := rp.unread(); len(left) > 0 {
		return true, refuse("Effect %s untap replacement unread %v", dur, left)
	}
	// ExileOnMoved$ Battlefield (Amber Prison's family) exiles the Effect when
	// its one remembered target leaves: the same lifetime as the record's drop.
	_, forget := ps.get("ForgetOnMoved")
	if _, exile := ps.get("ExileOnMoved"); exile {
		if forget || !ps.is("ExileOnMoved", "Battlefield") {
			return true, refuse("Effect %s untap ExileOnMoved$ %q", dur, ps.str("ExileOnMoved"))
		}
	} else if !ps.is("ForgetOnMoved", "Battlefield") {
		return true, refuse("Effect %s untap ForgetOnMoved$ %q", dur, ps.str("ForgetOnMoved"))
	}
	ps.consume("IsCurse") // AI-only
	if ro := ps.str("RememberObjects"); ro != "Targeted" {
		return true, refuse("Effect %s untap RememberObjects %q", dur, ro)
	}
	obj, err := lc.defined("Targeted")
	if err != nil {
		return true, err
	}
	if obj.Kind != rTarget {
		return true, refuse("Effect %s untap RememberObjects$ Targeted without a target", dur)
	}
	t := lc.ab.Targets[obj.N]
	if t.F == nil || len(t.F.Clauses) == 0 || t.Zone != zBattlefield || !t.Max.isConst() || t.Max.N > 1 {
		return true, refuse("Effect %s untap target slot %d", dur, obj.N)
	}
	in.Obj = obj
	lc.emit(in)
	return true, nil
}
