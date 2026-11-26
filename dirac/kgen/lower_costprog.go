package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cost programs (kop.CostAtom, Table.CostProgs): an activated ability whose
// cost has a part the flag-driven kop.Cost cannot carry lowers its cost as a
// sequence of atoms instead, which the kernel's walker (dirac/cost_prog.go)
// pays in gorge's payment order. The ability's kop.Cost keeps its mana and
// its {T} flag (the offer and the mana stage read them); the program pays
// the rest. A pool with no program writes no CostProgs, so its table takes
// none of the walker's path (useCostProg).

// subCtrAllCost lowers SubCounter<All/KIND>[/CARDNAME|NICKNAME]: remove every
// counter of KIND from the source, zero included (gorge cost.parse's
// SubCounterAll part, its count fixed to the pile at payment by
// pay.FixLifeXCost; CR 118.3). Only a printed battlefield activated ability's
// own cost; a filtered target (Hankyu's OriginalHost), a wildcard kind or a
// second counter part stays refused in its census class (cost:SubCounter).
func (lc *lowerCtx) subCtrAllCost(c *lCost, parts []string, tok string) error {
	if len(parts) > 4 {
		return refuse("cost token %q", tok)
	}
	if len(parts) >= 3 && !strings.EqualFold(parts[2], "CARDNAME") && !strings.EqualFold(parts[2], "NICKNAME") {
		return refuse("cost token %q", tok)
	}
	if strings.EqualFold(parts[1], "Any") {
		return refuse("cost token %q", tok)
	}
	if !lc.actCost || lc.ab == nil || lc.ab.Kind != akActivated || lc.ab.Latent {
		return refuse("cost token %q", tok)
	}
	if c.SubCtrAll != "" || c.SubCtr != 0 || c.SubCtrX || c.SubAmongN != 0 || c.Loyalty != 0 || c.LoyaltyX || c.AddCtr != 0 {
		return refuse("cost token %q", tok)
	}
	c.SubCtrAll = parts[1]
	return nil
}

// costCountersRemoved lowers SVar$CostCountersRemoved, the counters the
// ability's SubCounter cost removed (Forge CostRemoveCounter's SVar; gorge
// Ctx.Paid.CountersRemoved). The kernel's SubCounter atom records the count
// it removed as the activation's entry amount (stackEntry.trigAmt), read as
// the trigger amount: only the activated ability whose own cost carries the
// program reads it.
func (lc *lowerCtx) costCountersRemoved() (lValue, error) {
	if lc.ab == nil || lc.ab.Kind != akActivated || lc.ab.Latent || lc.ab.Cost.SubCtrAll == "" {
		return lValue{}, refuse("SVar$CostCountersRemoved outside an activated ability with a SubCounter<All/KIND> cost")
	}
	return lValue{Kind: vTriggerAmount}, nil
}

// lSacPart is one further filtered Sac<N/Spec> part of a cost (lCost.SacMore).
type lSacPart struct {
	F *lFilter
	N int
}

// sacMoreOK: a further filtered sacrifice part may lower here, as a cost
// program's own Sac atom: a printed activated ability's own cost.
func (lc *lowerCtx) sacMoreOK() bool {
	return lc.actCost && lc.ab != nil && lc.ab.Kind == akActivated && !lc.ab.Latent && (lc.ab.Zone == 0 || lc.ab.Zone == zBattlefield)
}

// costProgSacUnits bounds a program's sacrifice units: the kernel's joint
// feasibility walk (cost_progsac.go) assigns at most this many.
const costProgSacUnits = 8

// costProg is ability a's cost program index, 0 when its cost needs none: a
// SubCounter<All/KIND> cost, or two or more filtered sacrifice parts, beside
// at most mana and {T}.
func (cb *cardBuilder) costProg(a *lAbility) (uint16, error) {
	c := a.Cost
	if c.SubCtrAll == "" && len(c.SacMore) == 0 {
		return 0, nil
	}
	what := "SubCounter<All/" + c.SubCtrAll + "> cost"
	if len(c.SacMore) > 0 {
		// the census class the parts had before the program lowered them
		// (other:sacrifice cost "_" beside a second sacrifice part)
		what = fmt.Sprintf("sacrifice cost %q beside a second sacrifice part", fmt.Sprintf("Sac<%d/%s>", c.SacMore[0].N, c.SacMore[0].F))
	}
	// refuseAs keeps a sacrifice program's refusal in that class (no detail)
	refuseAs := func(format, why string) error {
		if len(c.SacMore) > 0 {
			return refuse("%s", what)
		}
		return refuse(format, what, why)
	}
	if a.Kind != akActivated || a.Latent || (a.Zone != 0 && a.Zone != zBattlefield) {
		return 0, refuseAs("%s %s", "outside a printed battlefield activated ability")
	}
	rest := c
	rest.Tap, rest.SubCtrAll = false, "" // pureMana reads no mana
	if len(c.SacMore) > 0 {
		rest.Sac, rest.SacN, rest.SacMore = nil, 0, nil
	}
	if c.SubCtrAll != "" && len(c.SacMore) > 0 || !rest.pureMana() {
		return 0, refuseAs("%s %s", "beside a part the cost program does not carry")
	}
	var prog []kop.CostAtom
	if !c.Mana.None {
		prog = append(prog, kop.CostAtom{K: kop.CostAtomMana})
	}
	if c.Tap {
		prog = append(prog, kop.CostAtom{K: kop.CostAtomTap})
	}
	if c.SubCtrAll != "" {
		ck, err := counterKind(c.SubCtrAll)
		if err != nil {
			return 0, err
		}
		prog = append(prog, kop.CostAtom{K: kop.CostAtomSubCounter, N: kop.ValueRef{K: kop.VCountersSelf, N: int16(ck)}, Ctr: ck})
	}
	if len(c.SacMore) > 0 {
		units := c.SacN
		parts := append([]lSacPart{{F: c.Sac, N: c.SacN}}, c.SacMore...)
		for _, m := range parts[1:] {
			units += m.N
		}
		if !c.Mana.None {
			// the mana planner's sacrifice reservation (gorge
			// planLeavesCostPayable, the kernel's tapForSacKept) claims one
			// part's candidates; a mana cost beside two parts stays refused
			return 0, refuseAs("%s %s", "beside a mana cost")
		}
		if units > costProgSacUnits {
			return 0, refuse("%s: %d sacrifices, over the %d the kernel assigns", what, units, costProgSacUnits)
		}
		for _, m := range parts {
			f, err := cb.sacFilter(m.F)
			if err != nil {
				return 0, err
			}
			prog = append(prog, kop.CostAtom{K: kop.CostAtomSac, N: kop.Const(m.N), F: f})
		}
	}
	return cb.internCostProg(prog)
}

// sacFilter interns a sacrifice part's filter as cost() does: you can only
// sacrifice what you control.
func (cb *cardBuilder) sacFilter(sac *lFilter) (kop.FilterID, error) {
	f := *sac
	f.Clauses = append([]lClause(nil), f.Clauses...)
	for i := range f.Clauses {
		if f.Clauses[i].Ctrl == whoAny {
			f.Clauses[i].Ctrl = whoYou
		}
	}
	return cb.filter(&f, kop.ZoneBattlefield)
}

// internCostProg returns prog's index in the pool's CostProgs, adding it on
// first use; index 0 is unused (no program).
func (cb *cardBuilder) internCostProg(prog []kop.CostAtom) (uint16, error) {
	pt := cb.pt
	key := fmt.Sprintf("%v", prog)
	if i, ok := pt.costProgKey[key]; ok {
		return i, nil
	}
	if len(pt.costProgs) == 0 {
		pt.costProgs = append(pt.costProgs, nil)
	}
	if len(pt.costProgs) > 0xffff {
		return 0, fmt.Errorf("cost programs: more than %d", 0xffff)
	}
	if pt.costProgKey == nil {
		pt.costProgKey = map[string]uint16{}
	}
	i := uint16(len(pt.costProgs))
	pt.costProgs = append(pt.costProgs, append([]kop.CostAtom(nil), prog...))
	pt.costProgKey[key] = i
	return i, nil
}
