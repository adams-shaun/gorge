package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// The api: effects for choices (lower_api.go).
func init() {
	apiPS("ChooseCard", (*lowerCtx).chooseCard)
	apiOp("ChoosePlayer", (*lowerCtx).apiChoosePlayer)
	apiPS("ChooseColor", (*lowerCtx).chooseColorPrevent) // Prismatic Strands
	apiPS("ChooseType", (*lowerCtx).chooseType)
	apiPS("ChooseNumber", (*lowerCtx).chooseNumberEffect) // lower_choosenumber.go
}

func (lc *lowerCtx) apiChoosePlayer(sa *cards.SA, ps *params) error {
	return lc.choosePlayer(ps, chainReadsChosenPlayer(sa.Sub)) // lower_definedheads.go, lower_chooseplayermid.go
}
