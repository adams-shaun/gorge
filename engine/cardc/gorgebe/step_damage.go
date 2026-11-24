package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() {
	registerStep(ir.VerbDealDamage, printDealDamage)
	registerStep(ir.VerbDestroy, printDestroy)
	registerStep(ir.VerbCounter, printCounter)
	registerStep(ir.VerbPutCounters, printPutCounters)
	registerStep(ir.VerbRegenerate, printRegenerate)
	registerStep(ir.VerbRemoveCounters, printRemoveCounters)
}

func printDealDamage(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if st.Who != nil {
		return nil, 0, unprintable("verb:deal-damage-who", st.Src)
	}
	amt, err := p.amount(st.N)
	if err != nil {
		return nil, 0, err
	}
	n := &node{API: "DealDamage", Params: []kv{{"NumDmg", amt}}}
	if o := slotRef(st.Obj); o != nil && o.Kind == ir.RefAll && o.Filter != nil && o.Filter.Subject != ir.SubjPlayer {
		v, err := p.valid(o.Filter)
		if err != nil {
			return nil, 0, err
		}
		n.API = "DamageAll"
		n.set("ValidCards", v)
	} else if err := p.operand(n, "Defined", st.Obj); err != nil {
		return nil, 0, err
	}
	for _, later := range steps[1:] {
		if later.N != nil && later.N.Kind == ir.VExcessDamage && st.Obj != nil && st.Obj.Kind == ir.RefTarget {
			// "the excess damage dealt this way": Forge names it and the damage to count
			n.set("ExcessSVar", "Excess")
			n.set("ExcessSVarCondition", "Card.targetedBy")
			p.excess = true
			break
		}
	}
	if st.Source != nil && st.Source.Kind != ir.RefSelf {
		d, err := p.defined(st.Source)
		if err != nil {
			return nil, 0, err
		}
		n.set("DamageSource", d)
	}
	if err := optionalDecider(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

func printDestroy(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	var n *node
	if o := slotRef(st.Obj); o != nil && o.Kind == ir.RefAll {
		if o.Filter == nil || o.Filter.Subject == ir.SubjPlayer {
			return nil, 0, unprintable("verb:destroy", st.Src)
		}
		v, err := p.valid(o.Filter)
		if err != nil {
			return nil, 0, err
		}
		n = &node{API: "DestroyAll", Params: []kv{{"ValidCards", v}}}
	} else {
		n = &node{API: "Destroy"}
		if err := p.operand(n, "Defined", st.Obj); err != nil {
			return nil, 0, err
		}
	}
	if err := optionalDecider(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

func printCounter(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	n := &node{API: "Counter"}
	if err := p.operand(n, "Defined", st.Obj); err != nil {
		return nil, 0, err
	}
	if err := optionalDecider(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

func printPutCounters(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if st.Word == "" {
		return nil, 0, unprintable("verb:put-counters", st.Src)
	}
	amt, err := p.amount(st.N)
	if err != nil {
		return nil, 0, err
	}
	n := &node{API: "PutCounter", Params: []kv{{"CounterType", st.Word}, {"CounterNum", amt}}}
	if err := p.operand(n, "Defined", st.Obj); err != nil {
		return nil, 0, err
	}
	if err := optionalDecider(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

func printRegenerate(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	n := &node{API: "Regenerate"}
	if st.Obj == nil || st.Obj.Kind != ir.RefSelf {
		if err := p.operand(n, "Defined", st.Obj); err != nil {
			return nil, 0, err
		}
	}
	if err := optionalDecider(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

// printRemoveCounters removes counters of one kind: every one from each object
// a filter selects (RemoveCounterAll), or a number from one object.
func printRemoveCounters(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:remove-counters", st.Src)
	if st.Word == "" || st.Who != nil {
		return nil, 0, bad
	}
	var n *node
	if o := slotRef(st.Obj); o != nil && o.Kind == ir.RefAll && o.Filter != nil && o.Filter.Subject != ir.SubjPlayer && st.N == nil {
		v, err := p.valid(o.Filter)
		if err != nil {
			return nil, 0, err
		}
		n = &node{API: "RemoveCounterAll", Params: []kv{{"ValidCards", v}, {"CounterType", st.Word}, {"AllCounters", "True"}}}
	} else {
		if st.N == nil {
			return nil, 0, bad
		}
		amt, err := p.amount(st.N)
		if err != nil {
			return nil, 0, err
		}
		n = &node{API: "RemoveCounter", Params: []kv{{"CounterType", st.Word}, {"CounterNum", amt}}}
		if err := p.operand(n, "Defined", st.Obj); err != nil {
			return nil, 0, err
		}
	}
	if err := optionalDecider(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}
