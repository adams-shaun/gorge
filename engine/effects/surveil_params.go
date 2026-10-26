package effects

import (
	"strings"
	"sync/atomic"

	"github.com/adams-shaun/gorge/engine/cards"
)

// SurveilParams contains the parameters shared by the Surveil ask and its
// answer record. RememberKept is compiled here so both answered and no-host
// resolution use the same interpretation.
type SurveilParams struct {
	paramBinding
	RememberKept bool
	// RememberMoved is RememberMoved$ True: the cards the surveil put into
	// the graveyard join the remembered set (Forge's toGrave).
	RememberMoved bool
}

func compileSurveil(sa *cards.SA) *SurveilParams {
	return &SurveilParams{
		paramBinding:  bindParams(sa),
		RememberKept:  strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKRememberKept)), "True"),
		RememberMoved: strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKRememberMoved)), "True"),
	}
}

// SurveilOf returns the compiled parameter record for this ability.
func SurveilOf(sa *cards.SA) *SurveilParams {
	if p := surveilFront[paramMapSlot(sa.Params)].Load(); p != nil && p.boundTo(sa.Params) {
		return p
	}
	p := compileSurveil(sa)
	if sa.Params != nil {
		surveilFront[paramMapSlot(sa.Params)].Store(p)
	}
	return p
}

var surveilFront [1 << 10]atomic.Pointer[SurveilParams]
