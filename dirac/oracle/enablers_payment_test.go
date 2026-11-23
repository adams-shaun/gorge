package oracle

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// A gorge-only cast's enablers are the sources of its first payment plan.
func TestGorgeSideEnablersFromPaymentPlan(t *testing.T) {
	const land, rock, token = state.ObjID(5), state.ObjID(6), state.ObjID(7)
	g := &GorgeSide{E: &rules.Engine{G: &state.Game{}}}
	g.id.inst = make([]Ref, 8)
	g.id.inst[land], g.id.inst[rock], g.id.inst[token] = 3, 4, RefNone
	plan := func(src ...state.ObjID) decision.PaymentAction {
		var p decision.PaymentPlan
		for _, s := range src {
			p.Activations = append(p.Activations, decision.PaymentActivation{Source: s})
		}
		return decision.PaymentAction{Plans: []decision.PaymentPlan{p, {Activations: []decision.PaymentActivation{{Source: token}}}}}
	}
	cast := Pack(ActCast, 0, RefNone, 0)
	other := Pack(ActCast, 1, RefNone, 0)
	pass := Pack(ActPass, RefNone, RefNone, 0)
	g.fr = &frame{prio: []prioEntry{
		{a: pass, kind: prOption},
		{a: cast, kind: prPlan, pay: plan(land, token, rock, land)},
		{a: other, kind: prOption},
	}}
	if got, want := g.enablers([]string{cast.String()}), []string{"#3", "#4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("enablers(cast) = %v, want %v: first plan only, token skipped, repeats once", got, want)
	}
	// A kernel-only option (no planner-paid entry for it) has no enablers,
	// and neither has an option offered as it stands.
	if got := g.enablers([]string{"cast(#9,-,0)"}); got != nil {
		t.Errorf("kernel-only option: enablers = %v, want nil", got)
	}
	if got := g.enablers([]string{other.String()}); got != nil {
		t.Errorf("option with no payment plan: enablers = %v, want nil", got)
	}
	if got := (&GorgeSide{}).enablers([]string{cast.String()}); got != nil {
		t.Errorf("no frame: enablers = %v, want nil", got)
	}
}
