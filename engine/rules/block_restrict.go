package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/combat"
)

func init() {
	// BlockRestrict: CR 509.1b's ceiling on how many creatures a defending
	// player may block with (combat.BlockRestrictLimit, both the printed S:
	// static and the Effect-registered restriction). BlockTapped: the
	// as-though-untapped blocking exception read by combat.CanBlock.
	effects.RegisterNonAPI("stat:BlockRestrict", "stat:BlockTapped")
	// kw:MustBeBlockedByTwo -- CR 509.1c's "CARDNAME must be blocked by two
	// or more creatures if able." (Gorm the Great), the printed sentence
	// canonicalised by engine/cards/hiddenkeyword.go: askBlockers publishes
	// it as Option.AttackMustTwo and decision.blockRequiredCore counts the
	// unit a second blocker obeys.
	effects.RegisterNonAPI("kw:MustBeBlockedByTwo")
}

// capBlockRestrict publishes a BlockRestrict ceiling on the declare-blockers
// decision d, reporting false when the ceiling is zero (the defender can
// declare nothing, so it is not asked). Each ordinary option names a
// distinct-blocker pair exclusive per blocker, so the ceiling is exactly the
// decision's Max; when a blocker may block several attackers (a raised group
// cap or a BlockAllDefined pair) one creature can fill several options, so
// Max is left alone and validateBlockers' distinct-blocker count is the
// whole rule. The requirement solver reads Max, so a team obeying more
// requirements than the ceiling allows is never demanded.
func (e *Engine) capBlockRestrict(d *decision.Decision) bool {
	limit, ok := combat.BlockRestrictLimit(asBoard(e), d.Player)
	if !ok {
		return true
	}
	if limit == 0 {
		return false
	}
	if len(d.GroupLimits) != 0 {
		return true
	}
	for i := range d.Options {
		if d.Options[i].BlockMustAll {
			return true
		}
	}
	d.Max = min(d.Max, limit)
	return true
}
