package effects

import "github.com/adams-shaun/gorge/engine/cards"

// preventDamageParams compiles the PreventDamage riders read after the
// recipients are resolved.
type preventDamageParams struct {
	// CombatOnly: CombatDamage$ True ("prevent the next N combat damage ..."),
	// the shield matches combat damage only.
	CombatOnly bool
}

func compilePreventDamageParams(sa *cards.SA) preventDamageParams {
	return preventDamageParams{
		CombatOnly: isTrue(sa.ParamStr(cards.PKCombatDamage)),
	}
}
