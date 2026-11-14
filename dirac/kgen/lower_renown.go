package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// Renown N (CR 702.112). gorge's cards/kw_renown.go expands K:Renown:N into
// one self trigger, Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$
// Player | CombatDamage$ True | IsPresent$ Card.Self+!IsRenowned |
// PresentDefined$ Self (the intervening if, CR 603.4), whose body is DB$
// PutCounter | Defined$ Self | CounterType$ P1P1 | Renown$ N. The trigger
// lowers as any other; its body is OpExt ExtRenown (the kernel's
// act_renown.go).

// renown lowers PutCounter Renown$ N (gorge effPutCounter's RenownSet arm):
// N +1/+1 counters on the source unless it is renowned or has left the
// battlefield, then the renowned designation. Only the source and the
// +1/+1 kind, the expansion's one shape: anything else refuses.
func (lc *lowerCtx) renown(ps *params, obj lRef, raw string) error {
	if obj.Kind != rSelf {
		return refuse("Renown$ recipient %v", obj)
	}
	if _, ok := ps.m["CounterNum"]; ok {
		return refuse("Renown$ with CounterNum$")
	}
	if ct, ok := ps.get("CounterType"); ok && ct != "P1P1" {
		return refuse("Renown$ with CounterType$ %q", ct)
	}
	n, err := lc.value(raw)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Renown", Obj: obj, A: n})
	return nil
}

// faceHasRenownTrigger reports whether the face carries the Renown
// expansion: a trigger whose Execute body is a PutCounter with Renown$.
func faceHasRenownTrigger(f *cards.Face) bool {
	for _, t := range f.Triggers {
		body := cards.ResolveSVar(f.SVars, t.Params["Execute"])
		if body != nil && body.API == "PutCounter" && body.Params["Renown"] != "" {
			return true
		}
	}
	return false
}
