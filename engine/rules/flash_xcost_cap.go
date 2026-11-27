package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// flashXCap is the X-announcement half of a `Spell.XCostLE<N>` CastWithFlash
// grant (Silver Scrutiny: "you may cast this spell as though it had flash if
// X is 3 or less"). A cast made at a time a sorcery could not have been cast
// whose timing rests on that grant may announce only X <= N: CR 601.2e's
// recheck (recheckIllegal, flashGrantCoversTargets) reverses any larger X, so
// xAsk caps its menu at the largest X that recheck still accepts rather than
// offering an announcement that can only be reversed. ok is false when no
// cap applies: a sorcery-timed cast, a face with its own instant timing, no
// XCostLE grant, or another grant that covers the cast at X = N+1.
func (e *Engine) flashXCap(pc *pendingCast) (n int32, ok bool) {
	if pc.isAbility() || !pc.offSorcery || pc.mode == "mayflash" || pc.mode == "defeat_cast" || pc.mode == "fuse" {
		return 0, false
	}
	o := e.G.Obj(pc.card)
	if o == nil || o.Face() == nil {
		return 0, false
	}
	found := false
	for _, sv := range e.withSelfStatics(e.activeStatics("CastWithFlash"), pc.card, "CastWithFlash", pc.player) {
		if !e.actorMatches(sv, "Caster", pc.player) {
			continue
		}
		for alt := range strings.SplitSeq(sv.ParamStr(cards.PKValidSA), ",") {
			kind, constraint, _ := strings.Cut(strings.TrimSpace(alt), ".")
			if v, isX := validSAXCostLE(constraint); isX && kind == "Spell" && (!found || v > n) {
				n, found = v, true
			}
		}
	}
	if !found {
		return 0, false
	}
	saved := pc.x
	pc.x = n + 1
	covered := e.flashGrantCoversTargets(pc.player, pc.card, o.Face(), nil)
	pc.x = saved
	return n, !covered
}
