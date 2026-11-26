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

// IsEntryTap reports whether sa is a bare "enters tapped" body (api:Tap with
// ETB$ True and no chain, tapper or card choice, the shape every ETBTapped
// land carries), the rules-side gate for a copied replacement line that taps
// the entering permanent.
func IsEntryTap(sa *cards.SA) bool {
	if sa == nil || sa.Sub != nil || (sa.CompiledAPI() != cards.APITap && sa.API != "Tap") {
		return false
	}
	p := compileTapParams(sa)
	return p.ETB && p.Tapper == "" && p.CardChoices == "" && !p.RememberTapped
}
