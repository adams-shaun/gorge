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
	// stat:IgnoreLandWalk: combat.LandwalkEvades skips a landwalk line an
	// active static names (Great Wall, Street Savvy's ValidBlocker$ form).
	effects.RegisterNonAPI("stat:IgnoreLandWalk")
	// stat:CanBlockIfShadow / stat:CanBlockIfReach: the as-though blocking
	// exceptions combat.CanBlock reads (Aether Web, Heartwood Dryad, Dragon
	// Hunter; CR 702.28b, 702.9b).
	effects.RegisterNonAPI("stat:CanBlockIfShadow", "stat:CanBlockIfReach", "stat:AttackVigilance")
	// kw:MustBeBlockedByAll (the filtered Lure), kw:MustBeBlockedByOne and the
	// companion restriction sentences (combat.ParseHiddenKeyword; CR 509.1c,
	// 508.1c-d): askBlockers publishes the first two as per-pair / per-attacker
	// requirements, publishCompanions the rest as option facts.
	// kw:MustBeBlockedBy <filter> is the granted filtered requirement (The
	// Masamune, Ace's Baseball Bat, Slayer's Cleaver); the census keys on the
	// whole head, so each carrier's filter is its own key.
	effects.RegisterNonAPI("kw:MustBeBlockedBy Creature", "kw:MustBeBlockedBy Creature.Dalek",
		"kw:MustBeBlockedBy Creature.Eldrazi")
	effects.RegisterNonAPI("kw:MustBeBlockedByAll", "kw:MustBeBlockedByOne", "kw:CanOnlyAttackAlone",
		"kw:CantAttackUnlessGreaterPower", "kw:CantBlockUnlessGreaterPower",
		"kw:CantAttackUnlessTwoOthers", "kw:CantBlockUnlessTwoOthers", "kw:CantAttackUnlessBlackOrGreen")
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
