package effects

import (
	"strings"
	"sync/atomic"

	"github.com/adams-shaun/gorge/engine/cards"
)

// SetStateParams is api:SetState's Choices$ pick shape, compiled once
// (CR 701.28 / 708.9: "transform an Incubator token you control" chooses, it
// does not target). The Defined$/Mode$ reads stay with effSetState.
type SetStateParams struct {
	paramBinding
	// Choices$, trimmed; "" selects the Defined$ path.
	Choices string
	// ChoiceTitle$ is the pick's prompt.
	ChoiceTitle string
	// Amount$ (default 1) and MinAmount$ (default Amount$).
	Amount    ParamText
	MinAmount ParamText
	// Mandatory$ True: the pick may not be declined (Forge's chooseCardsForEffect
	// optional flag is !Mandatory).
	Mandatory bool
}

var zeroSetState SetStateParams
var setStateFront [1 << 10]atomic.Pointer[SetStateParams]

// SetStateOf returns the compiled parameters for sa.
func SetStateOf(sa *cards.SA) *SetStateParams {
	if sa == nil {
		return &zeroSetState
	}
	slot := &setStateFront[paramMapSlot(sa.Params)]
	if p := slot.Load(); p != nil && p.boundTo(sa.Params) {
		return p
	}
	p := compileSetState(sa)
	if sa.Params != nil {
		slot.Store(p)
	}
	return p
}

// compileSetState is the only reader of SetState's Choices$ pick parameters.
func compileSetState(sa *cards.SA) *SetStateParams {
	amount, amountOK := sa.Param(cards.PKAmount)
	minAmount, minOK := sa.Param(cards.PKMinAmount)
	return &SetStateParams{
		paramBinding: bindParams(sa),
		Choices:      strings.TrimSpace(sa.ParamStr(cards.PKChoices)),
		ChoiceTitle:  sa.ParamStr(cards.PKChoiceTitle),
		Amount:       paramText(amount, amountOK),
		MinAmount:    paramText(minAmount, minOK),
		Mandatory:    strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKMandatory)), "True"),
	}
}
