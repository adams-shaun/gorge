package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// manaSpentCondition lowers a lone ConditionManaSpent$ / ConditionManaNotSpent$
// <colour> read in the body of a card's own enters trigger (Azorius Herald:
// "sacrifice it unless {U} was spent to cast it"). gorge's
// conditionManaSpentMet (effects/condition_manaspent.go) reads the source's
// recorded per-colour spend (Object.ManaColorSpent) at resolution: the cast
// that put the permanent onto the battlefield, zero when it entered any other
// way. The kernel's ExAdamant reads the same count, bound into the trigger as
// it is queued (kernel kw_evoke.go evokeEnter, which binds a body SkipUnless
// gate's read too). Unlike Adamant's intervening-if (the departure snapshot),
// gorge reads the body gate live, so the read carries kop.AdamantLiveRead: 0
// once the source has left the battlefield, its spend wiped (Steamcore Weird
// bounced in response deals nothing). ConditionManaSpent$ is "at least one mana of the colour",
// ConditionManaNotSpent$ none. A spell's own gate, several colours, or another
// Condition*$ key beside it refuses.
func (lc *lowerCtx) manaSpentCondition(ps *params) (lCond, bool, error) {
	spent, okS := ps.m["ConditionManaSpent"]
	notSpent, okN := ps.m["ConditionManaNotSpent"]
	if !okS && !okN {
		return lCond{}, false, nil
	}
	key, val, want := "ConditionManaSpent", spent, 1
	if okN {
		key, val, want = "ConditionManaNotSpent", notSpent, 0
	}
	if beside := conditionKeysBesides(ps, key); len(beside) > 0 {
		return lCond{}, true, refuse("%s$ beside %s$", key, beside[0])
	}
	ps.consume(key)
	if tr := lc.selfETBBody(); tr == nil {
		return lCond{}, true, refuse("%s$ outside its own enters trigger's body", key)
	}
	toks := strings.Fields(val)
	if len(toks) != 1 {
		return lCond{}, true, refuse("%s$ %q: several colours", key, val)
	}
	ci := strings.Index("WUBRG", adamantColourLetter(toks[0]))
	if ci < 0 || adamantColourLetter(toks[0]) == "" {
		return lCond{}, true, refuse("%s$ colour %q", key, toks[0])
	}
	v := lValue{Kind: vAdamant, N: ci | 1<<3 | kop.AdamantLiveRead, Sub: []lValue{cst(1), cst(0)}}
	return lCond{Kind: cValue, A: v, Op: opEQ, B: cst(want)}, true, nil
}

// selfETBBody is the trigger whose body is being lowered, if it is a printed
// enters trigger of the card itself (nil otherwise): adamantTrigger's shape
// read from the body.
func (lc *lowerCtx) selfETBBody() *lTrigger {
	if lc.ab == nil || lc.ab.Trigger == nil || !lc.trigBody || lc.ab.Latent || lc.granted > 0 {
		return nil
	}
	tr := lc.ab.Trigger
	if tr.Event != "ETB" || tr.Batch || tr.F == nil || tr.F.Players != 0 || len(tr.F.Clauses) != 1 || tr.F.Clauses[0] != (lClause{Flags: fSelf}) {
		return nil
	}
	return tr
}

// selfETBInterveningManaSpentCond mirrors gorge's
// effects.SelfETBInterveningManaSpent (engine/effects/condition_manaspent.go):
// a printed self-ETB trigger whose own description reads "if {C} was spent
// to cast it" carries the gate on its Execute body, but CR 603.4 makes it an intervening-if, so it joins the
// trigger header's conditions as a queue-time check (the same vAdamant read
// trigManaSpendCond lowers a header ManaSpent$, with the departure snapshot,
// not AdamantLiveRead). The body's live resolution read (manaSpentCondition
// above) stays. "sacrifice it unless {C} was spent" bodies have no "if {"
// description and are deliberately not matched.
func (lc *lowerCtx) selfETBInterveningManaSpentCond(t cards.Trigger) (lCond, bool, error) {
	spent, notSpent, ok := effects.SelfETBInterveningManaSpent(t)
	switch {
	case !ok:
		return lCond{}, false, nil
	case lc.adamantTrigger() == nil:
		// gorge gates this trigger at queue time; a shape the header read
		// cannot carry would silently drop the gate, so refuse it.
		return lCond{}, true, refuse("self-ETB intervening-if mana-spent gate outside the adamant trigger shape")
	case spent != "" && notSpent != "":
		return lCond{}, true, refuse("self-ETB intervening-if with both ConditionManaSpent$ and ConditionManaNotSpent$")
	case spent != "":
		c, err := lc.trigManaSpendCond(spent, false)
		return c, true, err
	default:
		c, err := lc.trigManaSpendCond(notSpent, true)
		return c, true, err
	}
}
