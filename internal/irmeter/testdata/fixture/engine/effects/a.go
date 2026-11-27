package effects

import "strings"

// SA and cards stand in for the engine's types so this fixture type-checks
// on its own: pr-precheck vets every directory holding a changed .go file.
type SA struct{ Params map[string]string }

func (*SA) Param(string) string    { return "" }
func (*SA) ParamStr(string) string { return "" }
func (*SA) ParamCode(string) int   { return 0 }

var cards struct {
	PKValidTgts, PKAffected, PKDefined, PKNumDmg, PKMode, PKOrigin string
}

func read(sa *SA, x string) {
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
