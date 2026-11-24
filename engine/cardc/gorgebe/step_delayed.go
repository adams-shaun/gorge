package gorgebe

import (
	"strconv"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerStep(ir.VerbDelayedTrigger, printDelayedTrigger) }

// printDelayedTrigger prints a one-ability delayed trigger. A next-end-step
// trigger is DelayedTrigger$ Mode$ Phase; a reflexive "when you do" is
// ImmediateTrigger$. Both name the trigger's body by Execute$. A body that
// returns what the previous step exiled remembers it there
// (RememberChanged$) and reads it back as DelayTriggerRememberedLKI.
func printDelayedTrigger(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:delayed-trigger", st.Src)
	if len(st.Abilities) != 1 || st.Optional || st.Who != nil {
		return nil, 0, bad
	}
	a := st.Abilities[0]
	if a.Kind != ir.AbTriggered || a.Trigger == nil || a.Body == nil || a.Cond != nil || a.Cost != nil {
		return nil, 0, bad
	}
	t := a.Trigger
	if t.Subject != nil || t.Ref != nil || t.Object != nil || t.Cond != nil || t.Cmp != nil {
		return nil, 0, bad
	}
	n := &node{}
	switch t.Event {
	case ir.EvNextEndStep:
		n.API = "DelayedTrigger"
		n.Params = []kv{{"Mode", "Phase"}, {"Phase", "End of Turn"}}
	case ir.EvReflexive:
		n.API = "ImmediateTrigger"
	default:
		return nil, 0, bad
	}
	if rc, ok := receiptMove(a.Body); ok {
		// The body returns what the previous step exiled.
		p.onPrev = func(prev *node) error {
			dest, ok := prev.get("Destination")
			if !ok || dest != "Exile" || rc.Obj.Receipt != ir.RcExiled {
				return bad
			}
			prev.set("RememberChanged", "True")
			body := &node{API: "ChangeZone", Params: []kv{{"Origin", dest}, {"Destination", zoneWord(rc.To)}, {"Defined", "DelayTriggerRememberedLKI"}}}
			n.set("Execute", p.sub("TrigReturn", body))
			n.set("RememberObjects", "RememberedLKI")
			n.Sub = &node{API: "Cleanup", Params: []kv{{"ClearRemembered", "True"}}}
			return nil
		}
		return n, 1, nil
	}
	if t.Event == ir.EvNextEndStep {
		return nil, 0, bad
	}
	body, err := p.program(a.Body)
	if err != nil {
		return nil, 0, err
	}
	n.set("Execute", p.sub("Trig"+body.API, body))
	// "Create two tokens. When you do": the trigger fires once per full set
	// of tokens remembered.
	p.onPrev = func(prev *node) error {
		if prev.API != "Token" {
			return nil
		}
		amt, _ := prev.get("TokenAmount")
		k, err := strconv.Atoi(amt)
		if err != nil || k < 2 {
			return nil
		}
		prev.set("RememberOriginalTokens", "True")
		n.set("TriggerAmount", "Remembered$Amount/DivideEvenlyDown."+amt)
		n.Sub = &node{API: "Cleanup", Params: []kv{{"ClearRemembered", "True"}}}
		return nil
	}
	return n, 1, nil
}

// receiptMove returns the single step of body that moves a receipt.
func receiptMove(body *ir.Program) (*ir.Step, bool) {
	if len(body.Steps) != 1 || len(body.Targets) > 0 {
		return nil, false
	}
	s := body.Steps[0]
	if s.Verb != ir.VerbMove || s.Obj == nil || s.Obj.Kind != ir.RefReceipt || s.To == ir.ZoneNone || s.Optional || s.Who != nil ||
		s.Word != "" || (s.Enter != nil && (s.Enter.Tapped || s.Enter.Controller != nil || s.Enter.Counter != "" || len(s.Enter.Effects) > 0)) {
		return nil, false
	}
	return s, true
}
