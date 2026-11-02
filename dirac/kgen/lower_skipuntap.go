package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// skipNextUntapKW is the exact sentence gorge reads as CR 611.2b's
// next-untap-step restriction (cards.HiddenUntapNextStepKeyword), with or
// without its HIDDEN marker.
const skipNextUntapKW = "This card doesn't untap during your next untap step."

// skipNextUntap lowers a Pump whose KW$ is exactly that sentence (gorge
// registerPumpEffects stamps Object.CantUntapNextStep on the battlefield
// object at grant, whatever the Duration$: the granted keyword text is
// otherwise inert) as OpSkipNextUntap. Beside a P/T change or another
// keyword it refuses.
func (lc *lowerCtx) skipNextUntap(ps *params, obj lRef, zone int) (bool, error) {
	kw, ok := ps.m["KW"]
	if !ok || strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(kw), "HIDDEN ")) != skipNextUntapKW {
		return false, nil
	}
	ps.consume("KW")
	_, na := ps.m["NumAtt"]
	_, nd := ps.m["NumDef"]
	if na || nd || zone != zBattlefield {
		return true, refuse("next-untap-step restriction beside a P/T change")
	}
	if d, ok := ps.get("Duration"); ok && d != "Permanent" {
		return true, refuse("next-untap-step restriction Duration %q", d)
	}
	lc.emit(lInstr{Op: "SkipNextUntap", Obj: obj})
	return true, nil
}

// skipUntapEffect lowers gorge's "doesn't untap during your next untap
// step" Effect (Duration$ UntilTheEndOfYourNextUntap, IsUntapStepDuration;
// rules/turn.go expireUntapStepEffects; replacement_match.go's
// ValidStepTurnToController$ Player.Activator): one ReplacementEffects$
// line Event$ Untap | Layer$ CantHappen | ValidStepTurnToController$
// Player.Activator over ValidCard$ Card.IsRemembered (RememberObjects$
// Self or Targeted, ExileOnMoved$ Battlefield: OpSkipUntap C=0 of that
// permanent) or ValidCard$ Land.YouCtrl (no remembered object: C=1).
// Refused in a mana ability, whose program the kernel never runs.
func (lc *lowerCtx) skipUntapEffect(ps *params) error {
	if lc.ab.Kind == akMana {
		return refuse("Effect Duration$ UntilTheEndOfYourNextUntap in a mana ability")
	}
	name := strings.TrimSpace(ps.str("ReplacementEffects"))
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "Untap" {
		return refuse("Effect next-untap replacement %q", name)
	}
	rp := newParams(r.Params)
	rp.consume("Event")
	if !rp.is("Layer", "CantHappen") || !rp.is("ValidStepTurnToController", "Player.Activator") {
		return refuse("Effect next-untap replacement shape")
	}
	in := lInstr{Op: "SkipUntap"}
	switch rp.str("ValidCard") {
	case "Card.IsRemembered":
		if !ps.is("ExileOnMoved", "Battlefield") {
			return refuse("Effect next-untap replacement ExileOnMoved$ %q", ps.str("ExileOnMoved"))
		}
		switch ro := ps.str("RememberObjects"); ro {
		case "Self", "Targeted":
			obj, err := lc.defined(ro)
			if err != nil {
				return err
			}
			if obj.Kind == rTarget {
				t := lc.ab.Targets[obj.N]
				if t.F == nil || len(t.F.Clauses) == 0 || t.Zone != zBattlefield || !t.Max.isConst() || t.Max.N > 1 {
					return refuse("Effect next-untap RememberObjects %q: target slot %d", ro, obj.N)
				}
			}
			in.Obj = obj
		default:
			return refuse("Effect next-untap RememberObjects %q", ro)
		}
	case "Land.YouCtrl":
		if _, ok := ps.get("RememberObjects"); ok {
			return refuse("Effect next-untap of lands with RememberObjects$")
		}
		if _, ok := ps.get("ExileOnMoved"); ok {
			return refuse("Effect next-untap of lands with ExileOnMoved$")
		}
		in.C = 1
	default:
		return refuse("Effect next-untap replacement ValidCard %q", rp.str("ValidCard"))
	}
	if left := rp.unread(); len(left) > 0 {
		return refuse("Effect next-untap replacement unread %v", left)
	}
	lc.emit(in)
	return nil
}
