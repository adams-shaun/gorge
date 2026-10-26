package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// tapParams is api:Tap's parameter record, compiled once per resolution
// (Forge TapEffect). CardChoices$ switches the primitive from the Defined$
// walk to a pick over the battlefield cards matching it: ChoiceAmount$ is how
// many (default 1, a value expression), AnyNumber$ True lowers the minimum to
// zero, ChoicePrompt$ is the chooser's prompt. RememberTapped$ True replaces
// the resolution's Remembered set with the permanents this call tapped.
type tapParams struct {
	Tapper          string
	ETB             bool
	CardChoices     string
	AnyNumber       bool
	ChoicePrompt    string
	RememberTapped  bool
	hasChoiceAmount bool
}

func compileTapParams(sa *cards.SA) tapParams {
	_, hasAmount := sa.Params["ChoiceAmount"]
	return tapParams{
		Tapper:          strings.TrimSpace(sa.ParamStr(cards.PKTapper)),
		ETB:             strings.EqualFold(sa.ParamStr(cards.PKETB), "True"),
		CardChoices:     strings.TrimSpace(sa.Params["CardChoices"]),
		AnyNumber:       strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKAnyNumber)), "True"),
		ChoicePrompt:    strings.TrimSpace(sa.ParamStr(cards.PKChoicePrompt)),
		RememberTapped:  strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKRememberTapped)), "True"),
		hasChoiceAmount: hasAmount,
	}
}
