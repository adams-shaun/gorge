package decision

import "fmt"

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

// AloneLegal reports whether the choices satisfy CR 506.5: an option marked
// AloneRestricted (its creature can't attack alone / can't block alone) may be
// chosen only together with at least one other option in the same answer.
// Out-of-range indices are ignored (Validate rejects them separately).
func (d *Decision) AloneLegal(choices []int) bool {
	if !d.aloneKind() {
		return true
	}
	for _, c := range choices {
		if c < 0 || c >= len(d.Options) {
			continue
		}
		if !d.Options[c].AloneRestricted {
			continue
		}
		others := 0
		for _, c2 := range choices {
			if c2 != c {
				others++
			}
		}
		if others == 0 {
			return false
		}
	}
	return true
}

// validateAlone is the wire half: Decision.Validate refuses an answer that
// declares a restricted creature alone.
func (d *Decision) validateAlone(choices []int) error {
	if d.AloneLegal(choices) {
		return nil
	}
	for _, c := range choices {
		if c >= 0 && c < len(d.Options) && d.Options[c].AloneRestricted {
			return fmt.Errorf("creature %d can't %s alone", d.Options[c].Obj, d.aloneVerb())
		}
	}
	return nil
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
// creature that can be declared beside attacker option c, or -1.
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
			d.ChargeOptionsFit(pair) && !d.groupCapExceeded(pair) {
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
// Only an alone-restricted creature is ever dropped: dropping an unrestricted
// partner can never make a legal answer legal, and dropping the restricted one
// is safe because the empty declaration is legal (KAttackers/KBlockers are Min
// 0, and FitRequired has already partnered a creature the quota requires). The
// answer handed back is therefore the empty set whenever the drop happens,
// which is exactly the shape AloneLegal re-checks.
func (d *Decision) FitAlone(choices []int) []int {
	if d.AloneLegal(choices) {
		return choices
	}
	out := make([]int, 0, len(choices))
	for _, c := range choices {
		restricted := c >= 0 && c < len(d.Options) && d.Options[c].AloneRestricted
		if restricted && len(choices) == 1 {
			continue
		}
		out = append(out, c)
	}
	if out == nil {
		out = []int{}
	}
	return out
}
