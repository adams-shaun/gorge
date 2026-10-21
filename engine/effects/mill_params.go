package effects

import "github.com/adams-shaun/gorge/engine/cards"

// millParams compiles the Mill-specific riders before the zone walk.
type millParams struct {
	Imprint  bool
	Optional bool
}

func compileMillParams(sa *cards.SA) millParams {
	return millParams{
		Imprint:  isTrue(sa.ParamStr(cards.PKImprint)),
		Optional: isTrue(sa.ParamStr(cards.PKOptional)),
	}
}
