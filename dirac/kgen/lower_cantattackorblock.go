package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// K:CantAttackOrBlock ("CARDNAME can't attack or block.", Grakk the
// Pacifist). gorge reads it as the hidden keyword whose two combat
// restrictions both fire (rules/combat ParseHiddenKeyword:
// HiddenKeywordFlags{CantAttack, CantBlock}), so kgen lowers it as the
// kernel's two equivalents: the KwCantBlock bit (the one K:Decayed sets) and a
// self CantAttack static (the printed S:Mode$ CantAttack | ValidCard$
// Card.Self line, no Target$ or UnlessDefender$).
func (lc *lowerCtx) lowerCantAttackOrBlock() {
	lc.out.Kw = append(lc.out.Kw, "CantBlock")
	s := cards.Static{Mode: "CantAttack", Params: map[string]string{"Mode": "CantAttack", "ValidCard": "Card.Self"}}
	lc.guard("K:CantAttackOrBlock", func() error { return lc.lowerStatic(s) })
}
