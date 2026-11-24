package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() { registerStep(ir.VerbPayCost, printPayCost) }

func printPayCost(p *pr, steps []*ir.Step) (*node, int, error) {
	pay := steps[0]
	bad := unprintable("verb:pay-cost", pay.Src)
	if !pay.Optional || len(steps) < 2 {
		return nil, 0, bad
	}
	cs := steps[1]
	if cs.Verb != ir.VerbIf || cs.Else != nil || cs.Cond == nil {
		return nil, 0, bad
	}
	switched := false
	switch c := cs.Cond; {
	case c.Kind == ir.CCostPaid:
		switched = true
	case c.Kind == ir.CNot && len(c.Args) == 1 && c.Args[0] != nil && c.Args[0].Kind == ir.CCostPaid:
	default:
		return nil, 0, bad
	}
	t, err := p.program(cs.Then)
	if err != nil {
		return nil, 0, err
	}
	cost, err := p.costText(pay.Cost)
	if err != nil {
		return nil, 0, err
	}
	if t.Sub != nil {
		// A cost on the head of a chain gates the whole chain, which
		// UnlessCost$ on the head alone would not.
		if !switched || pay.Who != nil || t.Ability {
			return nil, 0, unprintable("verb:pay-cost-chain", pay.Src)
		}
		t.Params = append([]kv{{"Cost", cost}}, t.Params...)
		t.Ability = true
		return t, 2, nil
	}
	payer := "You"
	if pay.Who != nil {
		if payer, err = p.defined(pay.Who); err != nil {
			return nil, 0, err
		}
	}
	t.set("UnlessCost", cost)
	t.set("UnlessPayer", payer)
	if switched {
		t.set("UnlessSwitched", "True")
	}
	return t, 2, nil
}
