package kgen

// lower_stacktgt.go: Ertai Resurrected's chain. A Counter with
// RememberForCounter$ True remembers what it counters, a Destroy with
// RememberLKI$ + AlwaysRemember$ its target's last known information, and
// the following Draw's Defined$ RememberedController is that thing's
// controller (gorge Defined RememberedController over Ctx.Remembered). The
// kernel reads it as the controller of the remembering op's own target slot
// (RefTargetController / RefTgtCtrl1-2: a player is that player, a stack
// ability its controller, an object that left its last known controller).

// rememberCounterNote marks a Counter whose countered object is remembered.
const rememberCounterNote = "remember for counter"

func (lc *lowerCtx) rememberedController() (lRef, error) {
	for i := len(lc.ab.Prog) - 1; i >= 0; i-- {
		in := lc.ab.Prog[i]
		if (in.Op == "Counter" && in.Note == rememberCounterNote || in.Op == "Destroy" && in.Note == rememberLKINote) && in.Obj.Kind == rTarget {
			return lRef{Kind: rTargetController, N: in.Obj.N}, nil
		}
		if in.Op != "Branch" && in.Op != "Charm" && in.Op != "DelayedTrigger" { // a delayed registration earlier in the chain reads the same set
			break
		}
	}
	return lRef{}, refuse("Defined$ RememberedController without a remembering Counter or Destroy")
}
