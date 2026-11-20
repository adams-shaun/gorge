package decision

import (
	"fmt"
	"slices"
)

// CR 506.5's attack-alone / block-alone restricted family, in the decision
// layer. The RULE lives here once and three callers derive from it: the wire
// validator (Decision.Validate), the shared bot repair (Clamp's FitAlone), and
// the engine's own declaration setters (askAttackersLoadout, which cannot pose
// a lone answer the validator would reject). The fact it binds is
// Option.AloneRestricted, published by rules from combat.DeclaresAlone, so the
// published fact, the validator and the repair cannot drift apart -- the
// "a decision's legal-answer rule has ONE home" invariant.

// aloneKind reports whether the decision is one whose answer is a SET of
// creatures declared together, so that an AloneRestricted option is illegal
// only when it is the whole set. Every other decision kind ignores the flag.
func (d *Decision) aloneKind() bool {
	return d.Kind == KAttackers || d.Kind == KBlockers
}

// AloneLegal reports whether the choices satisfy CR 506.5 and CR 508.1c: an
// option marked AloneRestricted (its creature can't attack alone / can't block
// alone) may be chosen only together with at least one other option in the
// same answer, an AloneOnly option only when no other creature is declared,
// and an option naming a CompanionNeed only beside that many distinct
// creatures from its CompanionFrom set. Out-of-range indices are ignored
// (Validate rejects them separately).
func (d *Decision) AloneLegal(choices []int) bool {
	return d.aloneViolator(choices) < 0
}

// aloneBound reports that the option's creature carries a declaration rule
// AloneLegal checks: it can't be declared alone, can only be declared alone,
// or needs companions.
func (o *Option) aloneBound() bool {
	return o.AloneRestricted || o.AloneOnly || o.CompanionNeed > 0
}

// aloneViolator is the position in choices of the first option whose
// declaration rule the answer breaks, or -1 when none does (and on every
// decision that is not a KAttackers/KBlockers set).
func (d *Decision) aloneViolator(choices []int) int {
	if !d.aloneKind() {
		return -1
	}
	for pos, c := range choices {
		if c < 0 || c >= len(d.Options) {
			continue
		}
		o := &d.Options[c]
		if !o.aloneBound() {
			continue
		}
		others, partners := 0, 0
		var seen []int
		for _, c2 := range choices {
			if c2 == c || c2 < 0 || c2 >= len(d.Options) {
				continue
			}
			others++
			obj := d.Options[c2].Obj
			if o.CompanionNeed > 0 && slices.Contains(o.CompanionFrom, c2) && obj != o.Obj && !slices.Contains(seen, int(obj)) {
				seen = append(seen, int(obj))
				partners++
			}
		}
		if (o.AloneRestricted && others == 0) || (o.AloneOnly && others > 0) || partners < o.CompanionNeed {
			return pos
		}
	}
	return -1
}

// validateAlone is the wire half: Decision.Validate refuses an answer that
// declares a restricted creature alone.
func (d *Decision) validateAlone(choices []int) error {
	if d.AloneLegal(choices) {
		return nil
	}
	o := &d.Options[choices[d.aloneViolator(choices)]]
	if o.AloneRestricted {
		return fmt.Errorf("creature %d can't %s alone", o.Obj, d.aloneVerb())
	}
	if o.AloneOnly {
		return fmt.Errorf("creature %d can only %s alone", o.Obj, d.aloneVerb())
	}
	return fmt.Errorf("creature %d can't %s without %d other qualifying creature(s) also doing so", o.Obj, d.aloneVerb(), o.CompanionNeed)
}

// aloneVerb is the declaration verb the diagnostic names, from the decision
// kind (the same split aloneKind makes).
func (d *Decision) aloneVerb() string {
	if d.Kind == KBlockers {
		return "block"
	}
	return "attack"
}

// alonePartner returns the cheapest option (ties: lowest index) naming another
// creature that can be declared beside attacker option c, or -1. The pair must
// satisfy AloneLegal, so an only-alone creature never has a partner and a
// companion-bound one only the creatures it names.
func (d *Decision) alonePartner(c int) int {
	best := -1
	if d.maxChoices() < 2 {
		return best
	}
	for i := range d.Options {
		o := &d.Options[i]
		if o.Obj == d.Options[c].Obj || (best >= 0 && o.Value >= d.Options[best].Value) {
			continue
		}
		pair := []int{c, i}
		if d.BudgetsFit(d.Options[c].Value+o.Value, d.Options[c].Value2+o.Value2) &&
			d.ChargeOptionsFit(pair) && !d.groupCapExceeded(pair) && d.AloneLegal(pair) {
			best = i
		}
	}
	return best
}

// FitAlone repairs a chosen set that declares an AloneRestricted creature
// alone (CR 506.5) by dropping that creature from the answer -- to declare
// nothing at all is always legal (the restriction constrains HOW the creature
// may be declared, never that it must be). It is a no-op on every decision
// that is not a KAttackers/KBlockers set and on every answer AloneLegal
// already admits, so every ordinary answer is returned unchanged. botpolicy's
// Clamp runs it, so a bot arm that ignores Option.AloneRestricted still cannot
// submit an answer Submit rejects (the deterministic-bot livelock the
// "one legal-answer home" invariant exists to prevent).
//
// The companion restrictions (AloneOnly, CompanionNeed) repair the same way:
// each option whose rule the answer breaks is dropped, one at a time, until
// AloneLegal holds -- a dropped creature can only shrink the others' partner
// sets, so the loop ends, and the empty declaration is always legal
// (KAttackers/KBlockers are Min 0, and FitRequired has already partnered a
// creature the quota requires). Only a restricted creature is ever dropped.
func (d *Decision) FitAlone(choices []int) []int {
	if d.AloneLegal(choices) {
		return choices
	}
	out := slices.Clone(choices)
	for pos := d.aloneViolator(out); pos >= 0; pos = d.aloneViolator(out) {
		out = slices.Delete(out, pos, pos+1)
	}
	if out == nil {
		out = []int{}
	}
	return out
}
