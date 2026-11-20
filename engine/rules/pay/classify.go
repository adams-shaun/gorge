package pay

import (
	"maps"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// classify.go holds the auto-pay planner's pure classification reads:
// which cost and pool shapes a V1 witness admits, the
// last-resort consequence shapes, and the plan/step comparisons the cast flow
// and the offer builder share. None reads the engine.

func ActionFor(d *decision.Decision, id string) (decision.PaymentAction, bool) {
	if d == nil {
		return decision.PaymentAction{}, false
	}
	for _, action := range d.PaymentActions {
		if action.ID == id {
			return decision.ClonePaymentAction(action), true
		}
	}
	return decision.PaymentAction{}, false
}

func FaceReadsManaSpent(f *cards.Face) bool {
	for _, needle := range []string{"ConditionManaSpent$", "ConditionManaNotSpent$", "Count$Adamant", "Count$EachSpentToCast", "Count$TotalManaSpent", "Count$ManaColorsPaid", "Count$xColorPaid", "ManaSpentBy"} {
		if f.Mentions(needle) {
			return true
		}
	}
	// The parsed top-level "| ConditionManaSpent$ G" clause is a param key,
	// invisible to the value search above.
	return f.MentionsParam("ConditionManaSpent") || f.MentionsParam("ConditionManaNotSpent")
}

// PlanNonManaAdmissible reports whether c is a cost a mana-only V1
// witness can describe: its mana half must classify cleanly and its only
// permitted non-mana parts are a fixed-count mandatory Sac<N/Spec> and
// Discard<N/Spec>. A
// variable-count Sac<X/Spec> (cost.CostPart.Announced, the announced count binds
// the cast's X) and Sac<All/...> (which never reaches Cost.Sac -- it parses as
// Unknown) are not admissible, and every other non-mana field declines. The
// mana half is checked by removing the Sac parts and running the same
// classifier the gate has always used, so X/hybrid/snow and the other
// unsupported mana-class shapes still decline here. Returns the decline detail, or
// "" when the cost is admissible.
//
// A fixed-count mandatory Discard<N/Spec> (Thrill of Possibility's "discard a
// card") is admitted on the same terms as the sacrifice: the discard is the
// cast's own CR 601.2h cost ask, answered from the hand after the card has
// moved to the stack, and it never touches a mana source the witness spends,
// so the witness's mana half is unchanged by it. paymentPlanCheck re-reads
// the cast's cost after its announcements (discards included) before the
// first activation, and ValidateCastPayment re-runs the offer gate's
// feasibility check at submit, exactly as for a sacrifice.
func PlanNonManaAdmissible(c Cost) string {
	for _, part := range c.Sac {
		if part.Announced || part.N <= 0 {
			return "cost:sacrifice"
		}
	}
	for _, part := range c.Discard {
		if part.Announced || part.Dyn != "" || part.N <= 0 {
			return "cost:discard"
		}
	}
	rest := c
	rest.Sac, rest.Discard = nil, nil
	// A twobrid pip is mana the witness funds: the plan resolves each pip to
	// its colour face or its two-generic face before the search
	// (planTwobridCost, CR 107.4e), so it is no non-mana admissibility
	// question and the shape gate must not report "cost:twobrid".
	rest.Twobrid = nil
	return PlanCostDetail(rest)
}

// PlanTargetPaidManaHalf is c without the additional-cost parts that are paid
// in the cast's own CR 601.2h flow and never touch a permanent or a mana
// source: life, energy, a reveal, a draw, damage, life gain, a mill and an
// exile from the top of the library. The planned cast of a spell whose only
// target its payment consumes funds the mana half; paymentPlanCheck re-reads
// the cast's real cost once its announcements are made.
func PlanTargetPaidManaHalf(c Cost) Cost {
	c.Life, c.LifeX, c.LifeHalfUp = 0, nil, false
	c.Reveal, c.RevealOrChoose, c.RevealChosen = nil, nil, nil
	c.Draw, c.Energy, c.DamageYou, c.GainLife = nil, nil, nil, nil
	c.Mill, c.ExileFromTop = nil, nil
	return c
}

// PlanManaHalf is the part of an admitted cast cost the V1 witness funds: c
// without its Phyrexian pips. A pip's colour face and its two-life face are
// both answered afterwards by the CR 601.2b pip ask, so the witness neither
// spends pool mana on a pip nor counts a pip's mana in its after-pool; the
// plan search and ValidateCastPayment settle this cost, and paymentPlanCheck
// re-reads the cast's own cost once the pips are announced (a colour-face
// answer changes that cost and falls back to the manual window).
func PlanManaHalf(c Cost) Cost {
	c.Phyrexian = nil
	return c
}

// ConvRider is the pip rider a plan funds under a ManaConvert conversion set
// (rules' Conv for the payment): AnyType when every mana type may be spent as
// mana of any type (ManaConversion$ AnyType->AnyType: Vizier of the
// Menagerie, CR 609.4b), so the planner prices the pips as generic exactly as
// under a may-play MayPlayIgnoreType$ grant; the zero rider for nil or any
// narrower conversion, which the planner leaves as it was.
func ConvRider(c *Conv) PipRider {
	if c == nil || !c.WildC {
		return PipRider{}
	}
	for i := range c.Wild {
		if !c.Wild[i] || c.OnlyC[i] {
			return PipRider{}
		}
	}
	return PipRider{AnyType: true}
}

// Or is the rider that relaxes every pip either r or o relaxes.
func (r PipRider) Or(o PipRider) PipRider {
	return PipRider{AnyColor: r.AnyColor || o.AnyColor, AnyType: r.AnyType || o.AnyType}
}

// PlanManaUnderRider is the mana half a plan funds when the cast's may-play
// grant relaxes its pips (MayPlayIgnoreColor$, MayPlayIgnoreType$): a pip the
// rider lets any mana pay (a coloured or hybrid one) is generic to the plan. The real cast still spends
// under the rider, so the witness only has to bring enough mana.
func PlanManaUnderRider(c Cost, r PipRider) Cost {
	slots := 0
	switch {
	case r.AnyType:
		slots = len(c.Colored)
	case r.AnyColor:
		slots = len(c.Colored) - 1 // {C} stays: "any color" isn't colourless.
	}
	for i := 0; i < slots; i++ {
		c.Generic += c.Colored[i]
		c.Colored[i] = 0
	}
	if (r.AnyType || r.AnyColor) && len(c.Hybrid) != 0 {
		// a hybrid pip is two colours, never {C}: mana of any colour or any
		// type pays it, so it is one generic to the plan too
		c.Generic += int32(len(c.Hybrid))
		c.Hybrid = nil
	}
	return c
}

// PlanManaUnderRiderPinned is the mana half a PINNED re-plan funds after the
// folded plan's pool could not prove a coloured pip's payment: the fold
// prices every pip generic, so the plan may tap a colourless-only source,
// but "any color" never includes colourless (CR 107.4c) and the offer gate
// rejects a pool with no coloured unit for a coloured pip. The pin holds the
// plain coloured pips coloured -- merged on the one concrete colour pin --
// so the plan must fund them from that colour's units, a coloured unit the
// grant then lets pay the printed pip; the rider's other work (the
// twobrid/Phyrexian pips it folds, the {C} it keeps) is unchanged. A HYBRID
// pip is pinned like a coloured one: "any color" pays it with a coloured unit
// of any colour (never {C}), so it too must be funded coloured. The caller
// pins under an AnyColor rider only: an AnyType rider lets even colourless
// mana pay, so there the fold needs no re-plan.
func PlanManaUnderRiderPinned(c Cost, r PipRider, pin int) Cost {
	n := pipUnitsUnderRider(c, r)
	c = PlanManaUnderRider(c, r)
	c.Generic -= n
	c.Colored[pin] = n
	return c
}

// pipUnitsUnderRider is the number of pips an AnyColor pin must fund coloured:
// every plain coloured pip (WUBRG; the trailing {C} slot is colourless and
// stays colourless) plus, under the colour rider alone, every hybrid pip --
// mana of any colour pays it, mana of no colour never does (CR 107.4c). An
// AnyType rider lets colourless mana pay everything, so it pins nothing.
func pipUnitsUnderRider(c Cost, r PipRider) int32 {
	n := int32(0)
	for i := 0; i+1 < len(c.Colored); i++ {
		n += c.Colored[i]
	}
	if r.AnyColor && !r.AnyType {
		n += int32(len(c.Hybrid))
	}
	return n
}

// PlanManaUnderRiderAlternatives is the ordered cost alternatives a plan may
// fund under rider r: the folded cost first (PlanManaUnderRider), then, under
// an AnyColor rider alone, one pinned per WUBRG colour
// (PlanManaUnderRiderPinned) when plain coloured pips or hybrid pips moved,
// then the unfolded cost c when its wire form differs from the fold's. A pin
// holds the coloured and hybrid pips coloured on one concrete colour, so the
// plan must fund them from that colour's units and the rider then lets those
// units pay the printed pip. An AnyType rider lets even colourless mana pay,
// so it has no pins. n is the number of alternatives.
func PlanManaUnderRiderAlternatives(c Cost, r PipRider) (alts [7]Cost, n int) {
	folded := PlanManaUnderRider(c, r)
	alts[0], n = folded, 1
	if r.AnyColor && !r.AnyType {
		moved := pipUnitsUnderRider(c, r)
		if moved != 0 {
			for pin := state.MW; pin <= state.MG; pin++ {
				alts[n] = PlanManaUnderRiderPinned(c, r, pin)
				n++
			}
		}
	}
	if WireCost(c) != WireCost(folded) {
		alts[n] = c
		n++
	}
	return alts, n
}

// PlanManaUnderRiderMatching returns the alternative of
// PlanManaUnderRiderAlternatives whose wire form is wire, scanning the same
// order. A plan built for one alternative (the folded cost or a pinned one) is
// settled against exactly that cost.
//
// A plan built for the board-spread pin (PlanManaUnderRiderSpread: the folded
// cost's pinned units re-coloured across the WUBRG slots) matches by SHAPE,
// not by a reconstructed alternative — the distribution the census funded at
// plan time is in the wire itself, and re-deriving it from a later census
// (an executor step has tapped sources) could name a different one. The shape
// check admits exactly that family and nothing else: the AnyColor pin's pip
// count n is pinned (the fold's generic minus n), the pinned units sit in the
// WUBRG slots summing to n, and the {C} slot is the fold's own. The returned
// cost is the re-coloured fold, and every settle of it (ValidateCastPayment,
// each executor step) resolves it strictly per colour, so a wire the plan does
// not fund fails the settle, never the payment.
func PlanManaUnderRiderMatching(c Cost, r PipRider, wire decision.PaymentCost) (Cost, bool) {
	alts, n := PlanManaUnderRiderAlternatives(c, r)
	for i := 0; i < n; i++ {
		if got, ok := planManaWireMatch(alts[i], wire); ok {
			return got, true
		}
	}
	pins := pipUnitsUnderRider(c, r)
	folded := alts[0]
	if !r.AnyColor || r.AnyType || pins == 0 || folded.Generic < pins ||
		wire.Generic != uint32(folded.Generic-pins) {
		return Cost{}, false
	}
	sum := int32(0)
	for i := state.MW; i <= state.MG; i++ {
		sum += int32(wire.Mana[i])
	}
	if sum != pins || int32(wire.Mana[state.MC]) != folded.Colored[state.MC] {
		return Cost{}, false
	}
	out := folded
	out.Generic -= pins
	for i := range out.Colored {
		out.Colored[i] = int32(wire.Mana[i])
	}
	return planManaWireMatch(out, wire)
}

// planManaWireMatch returns c when its wire form is wire, or c with its
// twobrid pips resolved face by face to the resolution that wires to it (CR
// 107.4e: the plan resolved the pips before the search, so the wire carries
// the announced face). The pip order is the planner's: every colour face
// first, then the generic one (AnnouncePip).
func planManaWireMatch(c Cost, wire decision.PaymentCost) (Cost, bool) {
	if WireCost(c) == wire {
		return c, true
	}
	if len(c.Twobrid) == 0 || len(c.Twobrid) > planTwobridLimit {
		return Cost{}, false
	}
	pips := c.Twobrid
	for mask := 0; mask < 1<<len(pips); mask++ {
		r := c
		r.Twobrid = nil
		bad := false
		for i, t := range pips {
			if mask>>i&1 == 1 {
				r.Generic = cost.AddClampedGeneric(r.Generic, int64(t.Generic))
				continue
			}
			idx := state.ManaIndex(t.Col)
			if idx < 0 || idx >= len(r.Colored) {
				bad = true
				break
			}
			r.Colored[idx]++
		}
		if !bad && WireCost(r) == wire {
			return r, true
		}
	}
	return Cost{}, false
}

func PlanCostDetail(c Cost) string {
	switch {
	case c.X != 0 || c.XMin != 0:
		return "cost:x"
	case c.Snow != 0:
		return "cost:snow"
	case len(c.Hybrid) != 0:
		return "cost:hybrid"
	case len(c.Phyrexian) != 0:
		// A Phyrexian pip's colour face is a mana unit the V1 witness can
		// fund; its life face rides the CR 601.2b pip announce, which the
		// witness's mana half does not bind. Clear the pips and re-run the
		// classifier, so a cost carrying Phyrexian AND another unsupported
		// part still declines with that part's detail.
		return PlanCostDetail(PlanManaHalf(c))
	case len(c.Twobrid) != 0:
		return "cost:twobrid"
	case len(c.HybridPhyrexian) != 0:
		return "cost:hybrid_phyrexian"
	case c.Life != 0 || len(c.LifeX) != 0 || c.LifeHalfUp:
		return "cost:life"
	case c.Tap:
		return "cost:tap"
	case len(c.Sac) != 0:
		return "cost:sacrifice"
	case len(c.Discard) != 0:
		return "cost:discard"
	case len(c.SubCounter) != 0:
		return "cost:sub_counter"
	case len(c.AddCounter) != 0:
		return "cost:add_counter"
	case len(c.Exile) != 0 || len(c.ExileFromTop) != 0:
		return "cost:exile"
	case len(c.Reveal) != 0 || len(c.RevealOrChoose) != 0 || len(c.RevealChosen) != 0:
		return "cost:reveal"
	case len(c.Behold) != 0:
		return "cost:behold"
	case len(c.TapPermanent) != 0:
		return "cost:tap_permanent"
	case len(c.Blight) != 0:
		return "cost:blight"
	case c.Forage:
		return "cost:forage"
	case len(c.Draw) != 0:
		return "cost:draw"
	case len(c.Energy) != 0:
		return "cost:energy"
	case len(c.DamageYou) != 0:
		return "cost:damage"
	case len(c.GainLife) != 0:
		return "cost:gain_life"
	case len(c.Return) != 0:
		return "cost:return"
	case len(c.PutToLib) != 0:
		return "cost:put_to_library"
	case len(c.MoveToGrave) != 0:
		return "cost:move_to_grave"
	case len(c.Mill) != 0:
		return "cost:mill"
	case len(c.Evidence) != 0:
		return "cost:evidence"
	case len(c.RollDice) != 0:
		return "cost:roll_dice"
	case len(c.Exert) != 0:
		return "cost:exert"
	case c.ChooseType != 0:
		return "cost:choose_type"
	case len(c.Unknown) != 0:
		return "cost:unknown"
	case len(c.Withheld) != 0:
		return "cost:withheld"
	case !PlanCostOK(c):
		return "cost:other"
	default:
		return ""
	}
}

// MentionsTarget reports whether s names a target reference in any case.
func MentionsTarget(s string) bool {
	for i := 0; i+6 <= len(s); i++ {
		if strings.EqualFold(s[i:i+6], "target") {
			return true
		}
	}
	return false
}

// PlanUsesLastResort reports whether any step of plan discloses a
// consequence (exactly the last-resort steps).
func PlanUsesLastResort(plan decision.PaymentPlan) bool {
	for _, a := range plan.Activations {
		if a.Consequence != nil {
			return true
		}
	}
	return false
}

// PlanRemainingPain sums the disclosed life + damage of plan's steps
// from index from on (the executor's lethal revalidation).
func PlanRemainingPain(plan decision.PaymentPlan, from int) int64 {
	var pain int64
	for i := from; i < len(plan.Activations); i++ {
		if c := plan.Activations[i].Consequence; c != nil {
			pain += int64(c.Life) + int64(c.Damage)
		}
	}
	return pain
}

func PlanCostOK(c Cost) bool {
	return c.X == 0 && c.XMin == 0 && c.Snow == 0 && c.Life == 0 &&
		len(c.Hybrid) == 0 && len(c.Phyrexian) == 0 && len(c.Twobrid) == 0 && len(c.HybridPhyrexian) == 0 &&
		!c.Tap && len(c.Sac) == 0 && len(c.Discard) == 0 && len(c.SubCounter) == 0 && len(c.AddCounter) == 0 &&
		len(c.Exile) == 0 && len(c.Reveal) == 0 && len(c.RevealOrChoose) == 0 && len(c.RevealChosen) == 0 && len(c.Behold) == 0 &&
		len(c.TapPermanent) == 0 && len(c.Blight) == 0 && !c.Forage && len(c.Draw) == 0 && len(c.Energy) == 0 &&
		len(c.LifeX) == 0 && !c.LifeHalfUp && len(c.DamageYou) == 0 && len(c.GainLife) == 0 && len(c.Return) == 0 &&
		len(c.PutToLib) == 0 && len(c.MoveToGrave) == 0 && len(c.Mill) == 0 && len(c.Evidence) == 0 && len(c.RollDice) == 0 && len(c.Unknown) == 0 && len(c.Exert) == 0 &&
		c.ChooseType == 0
}

// PlanLastResortCostOK reports whether a last-resort activation cost
// is exactly {T} (optional), Sac<1/...>, PayLife<N> and Return<1/...>: every
// other part -- mana, X, counters, discard, exile, mill, reveal, energy, a tap
// of another permanent, an unparsed token -- is outside what a witness step
// can disclose, so the ability stays deferred.
func PlanLastResortCostOK(c Cost) bool {
	rest := c
	rest.Tap, rest.Sac, rest.Life, rest.Return = false, nil, 0, nil
	return rest.Generic == 0 && rest.Colored == (state.Mana{}) && len(rest.ExileFromTop) == 0 && PlanCostOK(rest)
}

// PlanHasSpecialProductionParam reports whether the ability's own head
// carries a production special-effect parameter (spec 3.2: AddsCounters$,
// AddsNoCounter$, PersistentMana$, UnlessCost$, Defined$). Such production
// does something beyond adding plain mana to the pool, so the ability is
// deferred. TriggersWhenSpent$ is NOT in this list: its rider is decided by
// the source's own SVar body (PlanSpentRiderIsScry), so
// PaymentPlanShapeTierOf hands it to the rider arm.
func PlanHasSpecialProductionParam(ma *cards.SA) bool {
	// An any-key test: key order cannot change the answer, so the keys are
	// walked unsorted.
	for key := range maps.Keys(ma.Params) {
		if key == "AddsCounters" || key == "AddsNoCounter" ||
			key == "PersistentMana" || key == "UnlessCost" || key == "Defined" {
			return true
		}
	}
	return false
}

// ContainsTargetFold is strings.Contains(strings.ToLower(key), "target")
// without the lowered copy for an ASCII key (a non-ASCII key takes the
// original expression, so the answer is identical for every input).
func ContainsTargetFold(key string) bool {
	for i := 0; i < len(key); i++ {
		if key[i] >= 0x80 {
			return strings.Contains(strings.ToLower(key), "target")
		}
	}
	const w = "target"
	for i := 0; i+len(w) <= len(key); i++ {
		j := 0
		for ; j < len(w); j++ {
			c := key[i+j]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != w[j] {
				break
			}
		}
		if j == len(w) {
			return true
		}
	}
	return false
}

// PlanSelfCost reports whether a cost part names the play's source itself
// once with a fixed count, the self-reference the planner's reservation walk
// (potential_plan.go) keeps out of its plans and its last-resort consequence
// table reads as the source's own sacrifice. SacrificeMatchSpec normalizes
// Forge's NICKNAME spelling first, exactly as the offer and payment stages
// read it (SacrificeCostCandidates), so the planner cannot disagree with the
// payment about whether a self-reference is payable: measured at this pin,
// Syr Ginger's Sac<1/NICKNAME> {T} activation was otherwise planned as a
// generic sacrifice part, planLeavesCostPayable's tap-owning-source exclusion
// left it no candidates, and the play went unpriced ("cost:reserved_sources")
// although gorge's payment accepts it (engine/rules nickname_sac_test.go).
func PlanSelfCost(part cost.CostPart, id state.ObjID) bool {
	s := strings.ToLower(strings.TrimSpace(SacrificeMatchSpec(part.Spec)))
	return part.N == 1 && (s == "cardname" || s == "this token" || s == "cardname/self" || s == "self")
}

// PlanLastResortChoices is phase 2's alternative table (spec 5): every
// unit's normal AND last-resort alternatives, each carrying the source's
// phase-2 flexibility (flexAll, key 5 over every eligible alternative), minus
// any alternative whose own life + damage would by itself be lethal at the
// caster's current life (the lethal guard, applied to one step here and to
// the whole plan by the search). Unit positions are preserved. It returns
// nil when no unit has a last-resort alternative left, so phase 2 cannot
// differ from phase 1 and is skipped.
func PlanLastResortChoices(choices [][]Alt, life int32) [][]Alt {
	return PlanLastResortChoicesInto(nil, choices, life)
}

// PlanLastResortChoicesInto is PlanLastResortChoices with the copied rows
// built into ps's scratch (nil allocates, the plain behaviour). The copies
// point into ps.arena, which the next plan call resets -- so a table built
// here serves exactly one call, like everything else in the scratch.
func PlanLastResortChoicesInto(ps *PlanScratch, choices [][]Alt, life int32) [][]Alt {
	if ps == nil {
		out := make([][]Alt, len(choices))
		any := false
		for i, alts := range choices {
			for _, a := range alts {
				if a.Tier < TierLastResort {
					continue
				}
				if pain := ConsequencePain(a.Consequence); pain > 0 && pain >= int64(life) {
					continue
				}
				if a.Tier == TierLastResort {
					any = true
				}
				a.Flex = a.FlexAll
				out[i] = append(out[i], a)
			}
		}
		if !any {
			return nil
		}
		return out
	}
	out := resizeCleared(ps.phaseOuter, len(choices))
	ps.phaseOuter = out
	any := false
	for i, alts := range choices {
		at := len(ps.arena)
		for _, a := range alts {
			if a.Tier < TierLastResort {
				continue
			}
			if pain := ConsequencePain(a.Consequence); pain > 0 && pain >= int64(life) {
				continue
			}
			if a.Tier == TierLastResort {
				any = true
			}
			a.Flex = a.FlexAll
			ps.arena = append(ps.arena, a)
		}
		out[i] = ps.arena[at:len(ps.arena):len(ps.arena)]
	}
	if !any {
		return nil
	}
	return out
}

// consequence is exactly c.
func ConsequenceEqual(c Consequence, w *decision.PaymentConsequence) bool {
	got := c.Wire()
	if got == nil || w == nil {
		return got == nil && w == nil
	}
	return *got == *w
}

// AppendColourOnce appends col unless it is already present, preserving the
// first-seen order so an alternative list stays deterministic.
func AppendColourOnce(cols []string, col string) []string {
	for _, c := range cols {
		if c == col {
			return cols
		}
	}
	return append(cols, col)
}

// CostPips counts a parsed printed cost's coloured pips per WUBRG colour:
// each plain pip for its colour, each hybrid pip for both its colours, a
// Phyrexian pip for its colour, a twobrid for its coloured face (its generic
// face is not a pip) and a hybrid-Phyrexian pip for both its colours.
// Generic, {X} and {C} pips count for no colour.
func CostPips(c Cost) [5]int {
	var d [5]int
	for i, n := range c.Colored {
		if i < len(d) {
			d[i] += int(n)
		}
	}
	add := func(sym byte) {
		if i := state.ManaIndex(sym); i < len(d) {
			d[i]++
		}
	}
	for _, h := range c.Hybrid {
		add(h.A)
		add(h.B)
	}
	for _, p := range c.Phyrexian {
		add(p)
	}
	for _, t := range c.Twobrid {
		add(t.Col)
	}
	for _, h := range c.HybridPhyrexian {
		add(h.A)
		add(h.B)
	}
	return d
}

// PlanClassMembers is each class's member list (the verify-mode view
// of a class grouping).
func PlanClassMembers(classes []Class) [][]int {
	out := make([][]int, len(classes))
	for k := range classes {
		out[k] = classes[k].Members
	}
	return out
}

// PlanSameAlternative compares two computations of one alternative.
func PlanSameAlternative(a, b Alt) bool {
	return a == b
}

// PlanSameStep compares two resolutions of one witness step at one
// state: every field, the ability up to the identity of an ability built
// per call (a CR 305.6 intrinsic).
func PlanSameStep(a, b Alt) bool {
	if !SameManaAbility(a.Ma, b.Ma) {
		return false
	}
	a.Ma, b.Ma = nil, nil
	return a == b
}
