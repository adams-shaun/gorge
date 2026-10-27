package effects

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/engine/cards"
)

// CopySpellAbilityParams is api:CopySpellAbility's compiled parameter set.
// AddTypes$ is the only key this compiler owns; the primitive's other
// parameter reads stay where they are (this ticket does not migrate them).
type CopySpellAbilityParams struct {
	paramBinding
	// AddTypes is AddTypes$ split into type words by copyTypeList (comma and
	// " & " separators), in script order. It is the type addition every copy
	// the ability mints receives as a CopyExcept (CR 707.9a/b).
	AddTypes []string
}

var zeroCopySpellAbility CopySpellAbilityParams
var copySpellAbilityFront [1 << 10]atomic.Pointer[CopySpellAbilityParams]

// CopySpellAbilityOf returns the compiled parameters for sa.
func CopySpellAbilityOf(sa *cards.SA) *CopySpellAbilityParams {
	if sa == nil {
		return &zeroCopySpellAbility
	}
	slot := &copySpellAbilityFront[paramMapSlot(sa.Params)]
	if p := slot.Load(); p != nil && p.boundTo(sa.Params) {
		return p
	}
	p := compileCopySpellAbility(sa)
	if sa.Params != nil {
		slot.Store(p)
	}
	return p
}

// compileCopySpellAbility is the only reader of CopySpellAbility's AddTypes$.
func compileCopySpellAbility(sa *cards.SA) *CopySpellAbilityParams {
	return &CopySpellAbilityParams{
		paramBinding: bindParams(sa),
		AddTypes:     copyTypeList(sa.ParamStr(cards.PKAddTypes)),
	}
}
