package decision

import (
	"sort"

	"github.com/adams-shaun/gorge/state"
)

// The Required-quota contract. Option.Required marks an option whose Obj the
// answer MUST include (CR 508.1d's "attacks if able" on a KAttackers pair
// list), and Decision.MaxSum caps the chosen options' Value total (the
// CantAttackUnless attack-prop price). The two constraints interact: a
// required creature is "able" to attack only while the declaration stays
// within budget, so the number of required Objs an answer must cover is not
// "all of them" but the most the budget and Max can carry.
//
// That number is ONE rule, stated here and read by both sides: the engine's
// declaration check (rules' validateAttackDeclaration) demands exactly
// RequiredQuota distinct required Objs, and every client repair (botpolicy's
// Clamp) builds its answer from FitRequired, which starts from the very set
// RequiredQuota counts. So no answer a client assembles by this rule can be
// rejected for a requirement, and no requirement can outrun what the budget
// lets a client choose. A second, independent derivation on either side is
// how an engine-satisfiable decision became a bot livelock (attackprop1
// review: the engine priced the requirement from each creature's CHEAPEST
// pair while the bot's trim dropped required picks priced on DEARER ones).

// requiredCore returns, for each distinct Obj carrying a Required option, its
// cheapest Required option (ties: lowest option index), ordered by ascending
// Value and then by the Obj's first-seen option position; then it keeps the
// longest prefix whose Values fit MaxSum (when > 0) and whose length fits Max.
// Ascending-price greedy is optimal for the count: no other choice of one
// option per Obj covers more Objs within the same budget.
func (d *Decision) requiredCore() []int {
	type pick struct{ idx, pos int }
	best := make(map[state.ObjID]*pick) // membership/lookup only -- never ranged.
	var order []*pick
	for i := range d.Options {
		o := &d.Options[i]
		if !o.Required {
			continue
		}
		p, ok := best[o.Obj]
		if !ok {
			p = &pick{idx: i, pos: i}
			best[o.Obj] = p
			order = append(order, p)
			continue
		}
		if o.Value < d.Options[p.idx].Value {
			p.idx = i
		}
	}
	sort.SliceStable(order, func(a, b int) bool {
		va, vb := d.Options[order[a].idx].Value, d.Options[order[b].idx].Value
		if va != vb {
			return va < vb
		}
		return order[a].pos < order[b].pos
	})
	var core []int
	sum := 0
	for _, p := range order {
		if len(core) >= d.maxChoices() {
			break
		}
		v := d.Options[p.idx].Value
		if d.MaxSum > 0 && sum+v > d.MaxSum {
			break // ascending: nothing later fits either
		}
		sum += v
		core = append(core, p.idx)
	}
	return core
}

// RequiredQuota is how many distinct Required Objs a valid answer must
// include: the most the decision's MaxSum budget and Max ceiling can carry,
// cheapest Required option per Obj first. 0 when no option is Required.
func (d *Decision) RequiredQuota() int {
	return len(d.requiredCore())
}

// RequiredChosen counts the distinct Objs among the chosen options that
// carry a Required option. Out-of-range indices are ignored.
func (d *Decision) RequiredChosen(choices []int) int {
	seen := make(map[state.ObjID]bool, len(choices)) // membership only.
	n := 0
	for _, c := range choices {
		if c < 0 || c >= len(d.Options) || !d.Options[c].Required {
			continue
		}
		if obj := d.Options[c].Obj; !seen[obj] {
			seen[obj] = true
			n++
		}
	}
	return n
}

// FitRequired returns an answer that satisfies the Max ceiling, the MaxSum
// budget and the RequiredQuota at once, keeping as much of choices (a
// client's preferred answer, in its order) as those allow. When choices
// already satisfies all three it is returned unchanged, so a decision with no
// budget and no requirement pressure is byte-identical.
//
// Otherwise the answer is rebuilt from the requiredCore -- exactly the set
// RequiredQuota counts, so the quota is met by construction -- and then each
// preferred choice, in order, is folded in while it fits: a choice whose Obj
// is already covered by a Required pick REPLACES that pick when the swap
// stays within budget (one option per Obj: an attacker attacks one defender),
// any other choice is appended while the budget, Max and option Groups allow.
// Neither step can lower the count of Required Objs, so the rebuilt answer
// keeps the quota.
func (d *Decision) FitRequired(choices []int) []int {
	sum := 0
	for _, c := range choices {
		if c >= 0 && c < len(d.Options) {
			sum += d.Options[c].Value
		}
	}
	if len(choices) <= d.maxChoices() &&
		(d.MaxSum <= 0 || sum <= d.MaxSum) &&
		d.RequiredChosen(choices) >= d.RequiredQuota() {
		return choices
	}

	out := d.requiredCore()
	sum = 0
	slotOf := make(map[state.ObjID]int, len(out)) // Obj -> position in out.
	have := make(map[int]bool, len(out)+len(choices))
	groups := make(map[string]bool)
	objTaken := make(map[state.ObjID]bool, len(out)) // membership only.
	for i, c := range out {
		objTaken[d.Options[c].Obj] = true
		sum += d.Options[c].Value
		slotOf[d.Options[c].Obj] = i
		have[c] = true
		if g := d.Options[c].Group; g != "" {
			groups[g] = true
		}
	}
	requiredObj := make(map[state.ObjID]bool)
	for i := range d.Options {
		if d.Options[i].Required {
			requiredObj[d.Options[i].Obj] = true
		}
	}
	fits := func(delta int) bool { return d.MaxSum <= 0 || sum+delta <= d.MaxSum }
	for _, c := range choices {
		if c < 0 || c >= len(d.Options) || (have[c] && !d.Repeatable) {
			continue
		}
		o := &d.Options[c]
		if requiredObj[o.Obj] {
			if slot, ok := slotOf[o.Obj]; ok {
				old := &d.Options[out[slot]]
				if (o.Group != "" && o.Group != old.Group && groups[o.Group]) || !fits(o.Value-old.Value) {
					continue
				}
				sum += o.Value - old.Value
				delete(have, out[slot])
				if old.Group != "" {
					delete(groups, old.Group)
				}
				out[slot] = c
				have[c] = true
				if o.Group != "" {
					groups[o.Group] = true
				}
				continue
			}
		}
		if len(out) >= d.maxChoices() {
			continue
		}
		// KAttackers: one pair per creature (CR 506.2; the engine rejects a
		// creature declared twice), so a second pair of an Obj already in
		// the answer is skipped like a second member of an option Group.
		if d.Kind == KAttackers && objTaken[o.Obj] {
			continue
		}
		if (o.Group != "" && groups[o.Group]) || !fits(o.Value) {
			continue
		}
		sum += o.Value
		out = append(out, c)
		have[c] = true
		objTaken[o.Obj] = true
		if o.Group != "" {
			groups[o.Group] = true
		}
		if requiredObj[o.Obj] {
			slotOf[o.Obj] = len(out) - 1
		}
	}
	return out
}

// maxChoices is Max floored at 0: Validate rejects any answer longer than a
// negative Max, so the repair treats it as "choose nothing".
func (d *Decision) maxChoices() int {
	if d.Max < 0 {
		return 0
	}
	return d.Max
}
