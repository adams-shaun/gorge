package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"cost:SacHalf", "Cost.SacHalf (with CostSac, N 0)",
			"Sac<X/Spec> whose SVar X is Count$Valid Spec.YouCtrl/HalfUp as a card's own RaiseCost additional cost (CR 118.3, 601.2b; Tectonic Split's \"sacrifice half the lands you control, rounded up\"): nothing is announced. The part sacrifices half the payer's permanents matching Cost.Filter, rounded up, counted when the pick is posed (gorge folds SVar X into a fixed Sac<N/Spec> where the cost is composed, raise_cost_extra.go resolveRaiseCostText, counting every matching permanent); one pick per permanent (ChCostSacrifice). Cost.N stays 0 and dirac.sacXCost reads the part as no announced X. A table byte, not a CostFlags bit (the words are full). A spell's own additional cost."})
}

// usageSacHalf records the half-sacrifice cost row of a card's own cost.
func usageSacHalf(c kop.Cost, name string, add func(label, name string)) {
	if c.SacHalf {
		add("cost:SacHalf", name)
	}
}
