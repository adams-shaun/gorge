package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() {
	registerStep(ir.VerbDealDamage, printDealDamage)
	registerStep(ir.VerbDestroy, printDestroy)
	registerStep(ir.VerbCounter, printCounter)
	registerStep(ir.VerbPutCounters, printPutCounters)
	registerStep(ir.VerbRegenerate, printRegenerate)
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
	if err := p.operand(n, "Defined", st.Obj); err != nil {
		return nil, 0, err
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
	if st.Obj != nil && st.Obj.Kind == ir.RefAll {
		if st.Obj.Filter == nil || st.Obj.Filter.Subject == ir.SubjPlayer {
			return nil, 0, unprintable("verb:destroy", st.Src)
		}
		v, err := p.valid(st.Obj.Filter)
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
