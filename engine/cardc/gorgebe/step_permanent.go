package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() {
	registerStep(ir.VerbTap, printTap)
	registerStep(ir.VerbUntap, printTap)
	registerStep(ir.VerbSacrifice, printSacrifice)
}

func printTap(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	api, all := "Tap", "TapAll"
	if st.Verb == ir.VerbUntap {
		api, all = "Untap", "UntapAll"
	}
	o := st.Obj
	if o == nil {
		return nil, 0, unprintable("verb:"+st.Verb.String(), st.Src)
	}
	var n *node
	if o.Kind == ir.RefAll {
		if o.Filter == nil || o.Filter.Subject == ir.SubjPlayer {
			return nil, 0, unprintable("verb:"+st.Verb.String(), st.Src)
		}
		v, err := p.valid(o.Filter)
		if err != nil {
			return nil, 0, err
		}
		n = &node{API: all, Params: []kv{{"ValidCards", v}}}
	} else {
		n = &node{API: api}
		if err := p.operand(n, "Defined", o); err != nil {
			return nil, 0, err
		}
	}
	if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

func printSacrifice(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:sacrifice", st.Src)
	o := st.Obj
	var n *node
	switch {
	case o != nil && o.Kind == ir.RefSelf:
		n = &node{API: "Sacrifice", Params: []kv{{"Defined", "Self"}}}
	case o != nil && o.Kind == ir.RefChoose && o.Of == nil && o.Filter != nil:
		v, err := p.valid(o.Filter)
		if err != nil {
			return nil, 0, err
		}
		k, err := p.amount(o.N)
		if err != nil {
			return nil, 0, err
		}
		n = &node{API: "Sacrifice", Params: []kv{{"SacValid", v}, {"Amount", k}}}
		if st.Who != nil {
			if err := p.operand(n, "Defined", st.Who); err != nil {
				return nil, 0, err
			}
		}
	default:
		return nil, 0, bad
	}
	if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

func init() {
	registerStep(ir.VerbDoubleCounters, printDoubleCounters)
	registerStep(ir.VerbFight, printFight)
}

// printDoubleCounters prints MultiplyCounter$: double the named counters on an object.
func printDoubleCounters(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if st.Obj == nil || st.Word == "" || st.Who != nil {
		return nil, 0, unprintable("verb:double-counters", st.Src)
	}
	n := &node{API: "MultiplyCounter"}
	if err := p.operand(n, "Defined", st.Obj); err != nil {
		return nil, 0, err
	}
	n.set("CounterType", st.Word)
	if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

// printFight prints Fight$: Defined$ is the first fighter, the target slot
// declares the second.
func printFight(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if st.Obj == nil || st.Obj.Kind != ir.RefTarget || st.Source == nil || st.Who != nil {
		return nil, 0, unprintable("verb:fight", st.Src)
	}
	d, err := p.defined(st.Source)
	if err != nil {
		return nil, 0, err
	}
	n := &node{API: "Fight", Params: []kv{{"Defined", d}}}
	ps, err := p.declare(st.Obj.Slot, st.Obj.Src)
	if err != nil {
		return nil, 0, err
	}
	if ps == nil {
		return nil, 0, unprintable("verb:fight", st.Src)
	}
	for _, x := range ps {
		n.set(x.K, x.V)
	}
	if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}
