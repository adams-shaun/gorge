package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Preparation cards (CR 722). A card with AlternateMode
// Prepare has an inset instant or sorcery as face 1 (kop.FacePrepare). An
// AlterAttribute Attributes$ Prepared on the card's own permanent grants the
// prepared designation and mints the exile copy (gorge effAlterAttribute,
// events apply_player.go + grantPreparedCopy): OpPrepare. Every other shape
// of the grant on the card itself refuses; a targeted grant, Defined$
// Enchanted and Activate$ False (unprepare) lower on any permanent

// alterPrepared lowers AlterAttribute Attributes$ Prepared acting on obj,
// the source (gorge's Defined fallback when there is neither Defined$ nor
// ValidTgts$). Two shapes:
//
//   - the SELF path (no ValidTgts$, no Activate$, Defined$ absent or Self):
//     obj is the preparation card's own permanent (or the replaced object of
//     an ETB replacement); the card must be a preparation front;
//   - the REFERENT path (a ValidTgts$ target, Defined$ Enchanted, or an
//     Activate$ False unprepare): any battlefield permanent. The kernel
//     decides at resolution, as gorge's effAlterAttribute does: a permanent
//     with no prepare spell, or one already prepared, is a no-op, and an
//     unprepare (Activate$ False, C=1) always applies..
func (lc *lowerCtx) alterPrepared(ps *params, obj lRef) error {
	_, tgt := ps.m["ValidTgts"]
	act, hasAct := ps.m["Activate"]
	d := ps.str("Defined")
	if !tgt && !hasAct && (d == "" || d == "Self") {
		if lc.out.FaceMode != kop.FacePrepare || lc.out.Face != 0 {
			return refuse("AlterAttribute Prepared on a non-preparation card")
		}
		lc.emit(lInstr{Op: "Prepare", Obj: obj})
		return nil
	}
	if obj.Kind != rSelf { // the ETB replacement's replaced object
		return refuse("AlterAttribute Prepared target/Defined$/Activate$ in an ETB replacement")
	}
	for _, k := range []string{"Optional", "RememberAltered"} {
		if _, ok := ps.m[k]; ok {
			return refuse("AlterAttribute Prepared %s$", k)
		}
	}
	c := 0
	if hasAct {
		if strings.TrimSpace(ps.str("Activate")) != "False" {
			return refuse("AlterAttribute Prepared Activate$ %q", act)
		}
		c = 1
	}
	var ref lRef
	var err error
	switch {
	case tgt:
		if d != "" && d != "Targeted" {
			return refuse("AlterAttribute Prepared ValidTgts$ with Defined$ %q", d)
		}
		ref, err = lc.objOrTarget(ps, "", zBattlefield)
	case d == "Enchanted":
		ref, err = lc.defined(d)
	case d == "" || d == "Self":
		ref = obj
	default:
		return refuse("AlterAttribute Prepared Defined$ %q", d)
	}
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Prepare", Obj: ref, C: c})
	return nil
}

// validSelfValue lowers Count$ValidSelf <spec>: only the two exact
// IsPrepared spellings (gorge evalCountValidSelf over the source object);
// every other spec refuses.
func (lc *lowerCtx) validSelfValue(spec string) (lValue, error) {
	switch strings.TrimSpace(spec) {
	case "Card.IsPrepared":
		return lValue{Kind: vSelfPrepared}, nil
	case "Card.!IsPrepared":
		return lValue{Kind: vSelfPrepared, N: 1}, nil
	}
	return lValue{}, refuse("Count$ValidSelf spec %q", spec)
}
