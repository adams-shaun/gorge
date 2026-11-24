package gorgebe

import (
	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() {
	registerStep(ir.VerbChoose, printChooseCard)
}

// isReceipt reports whether r denotes the objects an earlier step produced.
func isReceipt(r *ir.Ref, k ir.ReceiptKind) bool {
	return r != nil && r.Kind == ir.RefReceipt && r.Receipt == k
}

// chosenFromExiled reports whether st picks one of the cards an earlier step
// exiled: "choose one of them".
func chosenFromExiled(st *ir.Step) bool {
	if st == nil || st.Verb != ir.VerbChoose || st.Optional || st.Who != nil || !chooseOf(st.Obj) {
		return false
	}
	f := st.Obj.Filter
	return f != nil && f.Op == ir.FPred && f.Pred == ir.PIs && isReceipt(f.Ref, ir.RcExiled) && onlyZone(f, ir.ZoneExile) &&
		st.Obj.N != nil && st.Obj.N.Kind == ir.VConst && st.Obj.N.N == 1
}

// mayPlayOf returns st's one may-play or may-cast effect, or nil.
func mayPlayOf(st *ir.Step) *ir.Effect {
	if st == nil || st.Verb != ir.VerbApplyEffect || st.Optional || len(st.Effects) != 1 || !isYou(st.Obj) {
		return nil
	}
	if e := st.Effects[0]; e.Kind == ir.EffMayPlay || e.Kind == ir.EffMayCast {
		return e
	}
	return nil
}

// exileKept reports whether the step after an exile of the library's top
// cards works on the cards exiled: Forge then writes the exile as a Dig that
// remembers them.
func exileKept(next *ir.Step) bool {
	if chosenFromExiled(next) {
		return true
	}
	e := mayPlayOf(next)
	return e != nil && isReceipt(e.Ref, ir.RcExiled)
}

// printExileKept prints "exile the top n cards of your library" whose cards
// the next step uses: Dig$ with RememberChanged$.
func printExileKept(p *pr, st *ir.Step) (*node, error) {
	o := st.Obj
	if st.Optional || o.Of != nil && !isYou(o.Of) || st.Who != nil && !isYou(st.Who) || o.N == nil {
		return nil, unprintable("verb:exile", st.Src)
	}
	k, err := p.amount(o.N)
	if err != nil {
		return nil, err
	}
	return &node{API: "Dig", Params: []kv{{"Defined", "You"}, {"DigNum", k}, {"ChangeNum", "All"},
		{"DestinationZone", "Exile"}, {"RememberChanged", "True"}}}, nil
}

// printChooseCard prints "choose one of the exiled cards": ChooseCard$ over
// what the exile remembered.
func printChooseCard(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if !chosenFromExiled(st) || p.prev == nil || p.prev.Verb != ir.VerbExile {
		return nil, 0, unprintable("verb:choose", st.Src)
	}
	return &node{API: "ChooseCard", Params: []kv{{"Choices", "Card.IsRemembered"}, {"ChoiceZone", "Exile"},
		{"Mandatory", "True"}, {"ForgetOtherRemembered", "True"}}}, 1, nil
}

// printMayPlay prints "you may play (or cast) that card": an Effect$ whose
// static lets you play the remembered card from the zone it is in.
func printMayPlay(p *pr, st *ir.Step, e *ir.Effect) (*node, int, error) {
	n := &node{API: "Effect"}
	var zone ir.Zone
	var aff, clear string
	switch {
	case slotRef(e.Ref) != nil && slotRef(e.Ref).Kind == ir.RefTarget:
		r := slotRef(e.Ref)
		f := p.frame()
		if f == nil || int(r.Slot) >= len(f.targets) || f.targets[r.Slot] == nil {
			return nil, 0, unprintable("effect:may-play", st.Src)
		}
		z, ok := singleZone(f.targets[r.Slot].Filter)
		if !ok {
			return nil, 0, unprintable("effect:may-play-zone", st.Src)
		}
		zone, aff = z, "Card.IsRemembered"
		if err := p.rememberOperand(n, r); err != nil {
			return nil, 0, err
		}
		n.set("ExileOnMoved", zoneWord(zone))
	case isReceipt(e.Ref, ir.RcExiled):
		zone, aff, clear = ir.ZoneExile, "Card.IsRemembered", "ClearRemembered"
		n.set("RememberObjects", "Remembered")
		n.set("ForgetOnMoved", "Exile")
	case isReceipt(e.Ref, ir.RcChosen) && p.prev != nil && p.prev.Verb == ir.VerbChoose:
		zone, aff, clear = ir.ZoneExile, "Card.ChosenCard", "ClearChosenCard"
		n.set("RememberObjects", "ChosenCard")
		n.set("ForgetOnMoved", "Exile")
	default:
		return nil, 0, unprintable("effect:may-play-ref", st.Src)
	}
	switch st.Dur {
	case ir.DurEndOfTurn:
	case ir.DurUntilEndOfYourNextTurn:
		n.set("Duration", "UntilTheEndOfYourNextTurn")
	default:
		return nil, 0, unprintable("effect:may-play-dur-"+st.Dur.String(), st.Src)
	}
	p.effectStatic(n, "Play", "Mode$ Continuous | MayPlay$ True | Affected$ "+aff+" | AffectedZone$ "+zoneWord(zone))
	if clear != "" {
		n.Sub = &node{API: "Cleanup", Params: []kv{{clear, "True"}}}
	}
	return n, 1, nil
}
