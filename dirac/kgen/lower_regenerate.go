package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// regenerate lowers api:Regenerate (gorge effRegenerate: one this-turn
// shield on each Defined$ object, default Self, or the target). The
// shields stack and each destruction spends one (CR 701.19a; kernel
// addRegenShield / regenerate).
func (lc *lowerCtx) regenerate(ps *params) error {
	if d := strings.TrimSpace(ps.str("Defined")); strings.HasPrefix(d, "Valid ") && !strings.Contains(d, " & ") {
		// Defined$ Valid <filter> (Wrap in Vigor's "regenerate each
		// creature you control"): gorge's Defined set is every battlefield
		// permanent the filter admits, read once, each given a shield. A
		// plain per-card loop over the same set gives each its shield;
		// a shield asks nothing, so the order is not observable.
		ps.get("Defined")
		f, err := lc.filter(strings.TrimSpace(strings.TrimPrefix(d, "Valid ")))
		if err != nil {
			return err
		}
		if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
			return refuse("Regenerate Defined$ %q: not a card filter", d)
		}
		lc.emit(lInstr{Op: "RepeatEachCard", Obj: lRef{Kind: rNone}, F: f, C: kop.RepeatPlain, Skip: 1})
		lc.emit(lInstr{Op: "Regenerate", Obj: lRef{Kind: rEach}})
		return nil
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Regenerate", Obj: obj})
	return nil
}

// noRegen reads Destroy/DestroyAll's NoRegen$: gorge compares it against
// "True" (effDestroy / effDestroyAll skip ReplaceDestruction), so an
// explicit False permits regeneration. Anything else refuses.
func noRegen(ps *params) (int, error) {
	v, ok := ps.get("NoRegen")
	if !ok {
		return 0, nil
	}
	switch v {
	case "True":
		return 1, nil
	case "False":
		return 0, nil
	}
	return 0, refuse("NoRegen %q", v)
}
