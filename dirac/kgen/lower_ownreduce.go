package kgen

import "strings"

// lower_ownreduce.go: an activated ability's own ReduceCost$
// (Channel, battlefield). gorge legal_activation.go
// ownReduceCost evaluates the value through the source face's SVars and
// subtracts it from the cost's generic part, floored at zero (CR 601.2f);
// the offer gate (legal_walk_battlefield.go), the planner witness
// (potential_plan.go planAbilityPayment) and the charge (activate.go
// beginActivation) all fold the same amount, so the kernel reads one
// reduced cost at its offer and at buildCost (dirac/cost_ownreduce.go,
// dirac/cost_ownreduce.go).

// ownReduce lowers ReduceCost$ v of activated ability ab with cost cost.
// A printed hand (Channel) or battlefield ability is read; any value the
// value lowerer returns is kept, but one whose SVar chain reads a target
// refuses: gorge's offer reads such a body at the best legal target
// (ownReduceCostOffer) and re-prices on the chosen one, a different read
// from the kernel's single source-relative evaluation. A mana-cost
// ReduceCost$ (Kami of Jealous Thirst's ReduceCost$ 4 B) is
// ownManaReduction's shape and refuses.
func (lc *lowerCtx) ownReduce(ab *lAbility, cost lCost, v string) error {
	if ab.Kind != akActivated || ab.Latent || ab.Zone != zHand && ab.Zone != zNone && ab.Zone != zBattlefield {
		return refuse("ReduceCost$ outside a printed hand or battlefield ability")
	}
	if cost.Mana.X || cost.Mana.XX || len(cost.Mana.Hybrid) != 0 {
		return refuse("ReduceCost$ beside an {X} or hybrid symbol")
	}
	if lc.svarReadsTarget(strings.TrimLeft(strings.TrimSpace(v), "+-"), 0) {
		return refuse("ReduceCost$ value reads a target")
	}
	val, err := lc.value(v)
	if err != nil {
		return err
	}
	if val.Kind == vConst && val.N <= 0 {
		return nil // gorge: a non-positive literal reduces nothing
	}
	ab.Reduce = &val
	return nil
}

// svarReadsTarget reports whether the SVar chain rooted at name (an SVar
// name or an inline body) mentions a target-reading Defined/Valid word.
func (lc *lowerCtx) svarReadsTarget(name string, depth int) bool {
	body := name
	if b, ok := lc.face.SVars[name]; ok {
		body = b
	}
	for _, w := range [...]string{"Targeted", "ParentTarget", "TriggeredTarget"} {
		if strings.Contains(body, w) {
			return true
		}
	}
	if depth > 4 {
		return true
	}
	for _, tok := range strings.FieldsFunc(body, func(r rune) bool {
		return r == ' ' || r == '$' || r == '/' || r == '.' || r == '+' || r == '|' || r == ','
	}) {
		if tok == name {
			continue
		}
		if _, ok := lc.face.SVars[tok]; ok && lc.svarReadsTarget(tok, depth+1) {
			return true
		}
	}
	return false
}

// expandBasicLandType rewrites Forge's hasABasicLandType property (gorge
// filter_word.go wordHasBasicLandType: a land with at least one of CR
// 205.3i's five basic land types, never the Basic supertype) into five
// alternatives, one per basic land type subtype: the filter's OR of
// clauses then matches exactly those lands (Boseiju, Who Endures).
func expandBasicLandType(expr string) string {
	const w = "hasABasicLandType"
	if !strings.Contains(expr, w) {
		return expr
	}
	var out []string
	for _, alt := range strings.Split(expr, ",") {
		if !strings.Contains(alt, w) {
			out = append(out, alt)
			continue
		}
		for _, t := range [...]string{"Plains", "Island", "Swamp", "Mountain", "Forest"} {
			out = append(out, strings.Replace(alt, w, t, 1))
		}
	}
	return strings.Join(out, ",")
}
