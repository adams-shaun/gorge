package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// spellNumTargetsToken parses Forge's SpellAbility property
// `numTargets <CMP><n>` (Voracious Bibliophile's SpellCast `ValidSA$
// Spell.numTargets GE1`, the EQ1 spellings on the targeting statics): the
// whole two-token form survives the spec splitter as one predicate token, the
// same shape `kicked <n>` takes. A malformed comparison is not the form and
// stays unknown to the matcher and the census alike.
func spellNumTargetsToken(p string) (CmpOp, int, bool) {
	rest, ok := strings.CutPrefix(p, "numTargets ")
	if !ok {
		return CmpNone, 0, false
	}
	op, n, ok := parseCountCompare(strings.TrimSpace(rest))
	if !ok {
		return CmpNone, 0, false
	}
	return CmpOpOf(op), int(n), true
}

// spellNumTargets is the number of distinct targets a spell or ability on the
// stack chose: its root declaration (Object.Targets) and every SubAbility$
// link's (Object.SubTargets), one object or player counted once, the way
// Forge's numTargets folds all of the ability's target choices into one set.
func spellNumTargets(o *state.Object) int {
	n := 0
	all := [2][]state.Target{o.Targets, o.SubTargets}
	for li, list := range all {
		for i, t := range list {
			if !targetSeenBefore(all, li, i, t) {
				n++
			}
		}
	}
	return n
}

// targetSeenBefore reports t appearing earlier in all (lists before li, or
// list li before index i). The lists are tiny, so the scan allocates nothing.
func targetSeenBefore(all [2][]state.Target, li, i int, t state.Target) bool {
	for lj := 0; lj <= li; lj++ {
		end := len(all[lj])
		if lj == li {
			end = i
		}
		for _, u := range all[lj][:end] {
			if u == t {
				return true
			}
		}
	}
	return false
}

// matchSpellAbilityToken evaluates the spell-ability property tokens
// matchPositive reads by shape: `hasAbility <SA spec>` and
// `numTargets <CMP><n>`. ok is false when p is neither.
func matchSpellAbilityToken(p string, o *state.Object) (result, ok bool) {
	if hasAbilityToken(p) {
		return objectHasAbility(o, strings.TrimPrefix(p, "hasAbility ")), true
	}
	if op, n, is := spellNumTargetsToken(p); is {
		return o != nil && op.Apply(spellNumTargets(o), n), true
	}
	return false, false
}

// spellAbilityToken reports a token matchSpellAbilityToken answers, for the
// census (positiveRecognised) and the compiled classifier.
func spellAbilityToken(p string) bool {
	_, _, num := spellNumTargetsToken(p)
	return num || hasAbilityToken(p)
}
