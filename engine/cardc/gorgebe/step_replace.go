package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() {
	registerStep(ir.VerbModifyEnter, printModifyEnter)
	registerStep(ir.VerbSetAmount, printSetAmount)
}

// wouldEvent reports whether e is a replaced event.
func wouldEvent(e ir.EventKind) bool {
	switch e {
	case ir.EvWouldEnter, ir.EvWouldPutCounters, ir.EvWouldDealDamage, ir.EvWouldCreateTokens, ir.EvWouldDie:
		return true
	}
	return false
}

// enterCounterOnly reports whether e enters with a counter and nothing else.
func enterCounterOnly(e *ir.Enter) bool {
	return e != nil && e.Counter != "" && e.N != nil && !e.Tapped && !e.UnderOwner && e.Controller == nil && len(e.Effects) == 0
}

func printModifyEnter(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if !wouldEvent(p.ev) {
		return nil, 0, unprintable("verb:modify-enter-outside-replacement", st.Src)
	}
	bad := unprintable("verb:modify-enter", st.Src)
	e := st.Enter
	if e == nil || st.Optional {
		return nil, 0, bad
	}
	switch {
	case e.Tapped && e.Counter == "" && e.N == nil && !e.UnderOwner && e.Controller == nil && len(e.Effects) == 0:
		if !p.evSelf {
			// a replacement of other objects' entering taps the object replaced
			return &node{API: "Tap", Params: []kv{{"ETB", "True"}, {"Defined", "ReplacedCard"}}}, 1, nil
		}
		return &node{API: "Tap", Params: []kv{{"Defined", "Self"}, {"ETB", "True"}}}, 1, nil
	case enterCounterOnly(e) && p.evSelf:
		n, err := p.amount(e.N)
		if err != nil {
			return nil, 0, err
		}
		return &node{API: "PutCounter", Params: []kv{{"Defined", "Self"}, {"ETB", "True"}, {"CounterType", e.Counter}, {"CounterNum", n}}}, 1, nil
	}
	return nil, 0, bad
}

func printSetAmount(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if !wouldEvent(p.ev) {
		return nil, 0, unprintable("verb:set-amount-outside-replacement", st.Src)
	}
	if p.ev == ir.EvWouldPutCounters && !st.Optional && st.N != nil && st.N.Kind == ir.VMul && len(st.N.Args) == 2 &&
		isConst(st.N.Args[0]) && st.N.Args[0].N == 2 && st.N.Args[1].Kind == ir.VReceiptAmount && st.N.Args[1].Word == "counters" {
		y := p.countVarAs("Y", "ReplaceCount$CounterNum/Twice")
		return &node{API: "ReplaceCounter", Params: []kv{{"Amount", y}}}, 1, nil
	}
	if p.ev != ir.EvWouldDealDamage || st.Optional || st.N == nil {
		return nil, 0, unprintable("verb:set-amount", st.Src)
	}
	n, err := p.amount(st.N)
	if err != nil {
		return nil, 0, err
	}
	return &node{API: "ReplaceEffect", Params: []kv{{"VarName", "DamageAmount"}, {"VarValue", n}}}, 1, nil
}
