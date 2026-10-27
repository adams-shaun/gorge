package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// ReplaceSplitDamageParams is a DB$ ReplaceSplitDamage body's parameters
// (Forge ReplaceSplitDamageEffect): the redirect recipient and the shield size.
// The body is applied inline by rules' applyReplaceSplitDamageBody, never
// through Resolve, so this record is its only parameter reader.
type ReplaceSplitDamageParams struct {
	// DamageTarget$ as written (Remembered, EffectSource, Imprinted, You):
	// the Defined$ selector of the redirect recipient.
	DamageTarget string
	// VarName$ names the SVar holding the shield size (absent: 1).
	VarName ParamText
}

// ReplaceSplitDamageOf compiles sa's ReplaceSplitDamage parameters.
func ReplaceSplitDamageOf(sa *cards.SA) ReplaceSplitDamageParams {
	if sa == nil {
		return ReplaceSplitDamageParams{}
	}
	vn, ok := sa.Param(cards.PKVarName)
	return ReplaceSplitDamageParams{
		DamageTarget: strings.TrimSpace(sa.ParamStr(cards.PKDamageTarget)),
		VarName:      ParamText{Text: vn, Present: ok},
	}
}

// ReplaceSplitDamageShield resolves the body's shield size in c's frame: the
// VarName$ SVar's value, or 1 when the body names none. ok is false for a
// value this build cannot price, so the caller leaves the damage untouched.
func ReplaceSplitDamageShield(h Host, c *Ctx, sa *cards.SA) (int32, bool) {
	p := ReplaceSplitDamageOf(sa)
	if !p.VarName.Present {
		return 1, true
	}
	return numResolvedText(h, c, p.VarName, 1)
}
