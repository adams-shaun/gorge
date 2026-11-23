package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// statGrantsLure reports whether a Mode$ Continuous static's AddHiddenKeyword$
// is the Lure sentence "All creatures able to block CARDNAME do so." (CR
// 509.1c; Lure, Gift of the Deity, Seton's Desire, Stone-Tongue Basilisk,
// Tempting Licid). It is a layer-6 keyword grant like AddKeyword$, kept in the
// sentence spelling a runtime KW$ grant has, which combat.ParseHiddenKeyword
// reads per attacker. Only this sentence is read here: the other sentences a
// static's AddHiddenKeyword$ carries stay unmodelled.
func statGrantsLure(st cards.Static) bool {
	v, ok := st.Params["AddHiddenKeyword"]
	return ok && cards.HiddenKeywordOf(v) == cards.HiddenKeywordAllMustBlock
}

// withLureGrant is a static's keyword grant list with the lure sentence added
// when the static grants it.
func withLureGrant(st cards.Static, kws []string) []string {
	if statGrantsLure(st) {
		kws = append(kws, st.Params["AddHiddenKeyword"])
	}
	return kws
}

// setStaticKeywordGrant fills a layer-6 keyword effect from a Continuous
// static: its granted keywords (protection resolved against the source o) and
// the QuotedAbilityGrant mark a lure grant carries, so a later loss ends it
// (CR 613.1f) where a spell's unquoted lure requirement survives.
func setStaticKeywordGrant(kw *state.ContinuousEffect, st cards.Static, o *state.Object) {
	kw.AddKeywords = effects.ResolveGrantedProtection(withLureGrant(st, statKeywords(st)), o)
	kw.QuotedAbilityGrant = statGrantsLure(st)
}
