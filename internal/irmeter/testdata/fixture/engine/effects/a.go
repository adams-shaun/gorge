package effects

import (
	"strings"

	"example/cards"
)

func read(sa *cards.SA, x string) {
	_ = sa.ParamStr(cards.PKValidTgts)
	_ = sa.Param(cards.PKAffected)
	_ = sa.ParamStr(cards.PKDefined)
	_ = sa.ParamStr(cards.PKNumDmg)
	_ = sa.ParamCode(cards.PKMode)
	_ = sa.ParamCode(cards.PKOrigin)
	_ = sa.Param(x)
	_ = sa.Params["SubAbility"]
	_ = sa.Params["Mode"]
	_ = strings.TrimSpace("Creature.YouCtrl") + strings.ToLower("Creature.YouCtrl")
	_ = "Card.Other"
}
