package botpolicy

import "github.com/adams-shaun/gorge/engine/decision"

// forcedConvokeChoices answers a Convoke announcement that must cover the cast
// itself because the spell may not spend mana (stat:ManaRestriction, Hogaak's
// "You can't spend mana to cast this spell"). Such an ask publishes its exact
// size: Min is the fewest taps the credits must make and MaxSum the number of
// coloured taps the pips can take (each coloured option's Value is 1), the
// same two numbers Validate enforces. The answer takes a coloured option from
// as many distinct creatures as the budget allows, then one generic from the
// rest until Min is met; it is nil when the offered creatures cannot reach
// Min, so the caller falls back.
func forcedConvokeChoices(d *decision.Decision) []int {
	if d.Min <= 0 || !d.HasBudget() {
		return nil
	}
	used := make(map[int32]bool, d.Min)
	picks := make([]int, 0, d.Min)
	budget := d.MaxSum
	for i := range d.Options {
		o := &d.Options[i]
		if o.Value > 0 && budget >= o.Value && !used[int32(o.Obj)] {
			used[int32(o.Obj)] = true
			budget -= o.Value
			picks = append(picks, o.Index)
		}
	}
	for i := range d.Options {
		o := &d.Options[i]
		if len(picks) >= d.Min {
			break
		}
		if o.Kind == "convoke_generic" && !used[int32(o.Obj)] {
			used[int32(o.Obj)] = true
			picks = append(picks, o.Index)
		}
	}
	if len(picks) < d.Min {
		return nil
	}
	return picks
}
