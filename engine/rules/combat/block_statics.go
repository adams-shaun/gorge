package combat

import (
	"fmt"
	"math"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// The declare-blockers statics read here: BlockTapped, "tapped creatures you
// control can block as though they were untapped" (Masako the Humorless),
// and BlockRestrict, the defending player's blocker ceiling ("each opponent
// can't block with more than one creature", Mirri, Weatherlight Duelist;
// Dueling Grounds; Silent Arbiter).

// blockTappedAllowed reports whether a BlockTapped static lets the tapped
// creature blocker block as though it were untapped (CR 509.1a names an
// untapped creature; the static lifts exactly that requirement). ValidCard$
// is resolved against the blocker with the static's host as the spec source,
// so Creature.tapped+YouCtrl reaches the static controller's own creatures
// only. A gated static whose condition does not hold lifts nothing.
func blockTappedAllowed(b Board, blocker state.ObjID) bool {
	for _, sv := range b.Statics("BlockTapped") {
		if !b.StaticGateHolds(sv) {
			continue
		}
		if b.MatchesStaticSpec(sv.ParamStr(cards.PKValidCard), blocker, sv) {
			return true
		}
	}
	return false
}

// BlockRestrictLimit is the CR 509.1b ceiling on how many creatures defender
// may declare as blockers this combat, and whether any applies: the smallest
// MaxBlockers$ among the BlockRestrict statics whose ValidDefender$ names the
// defender (an absent ValidDefender$ names every defending player). Forge's
// default MaxBlockers$ is 1. Both delivery routes are read: a printed S:
// static and the continuous restriction an Effect registers (Mirri's attack
// trigger), whose ValidDefender$ resolves against the Effect's controller.
// The ceiling counts distinct blocking creatures, so a creature blocking two
// attackers is still one.
func BlockRestrictLimit(b Board, defender state.PlayerID) (int, bool) {
	limit, found := 0, false
	take := func(n int) {
		if !found || n < limit {
			limit, found = n, true
		}
	}
	g := b.Game()
	for _, sv := range b.Statics("BlockRestrict") {
		if !b.StaticGateHolds(sv) {
			continue
		}
		if spec := sv.Params["ValidDefender"]; spec != "" &&
			!effects.MatchesPlayerSpecCtx(g, spec, defender, sv.Controller, b.PlayerSpecCtx(sv.Source)) {
			continue
		}
		if n, ok := blockerCeiling(sv.Params["MaxBlockers"]); ok {
			take(n)
		}
	}
	for i, active := 0, b.Active(); i < len(active); i++ {
		ce := &active[i]
		if ce.Restriction != effects.ModeBlockRestrict {
			continue
		}
		if spec := ce.RestrictParams["ValidDefender"]; spec != "" &&
			!effects.MatchesPlayerSpecCtx(g, spec, defender, ce.Controller, b.PlayerSpecCtx(ce.Source)) {
			continue
		}
		if n, ok := blockerCeiling(ce.RestrictParams["MaxBlockers"]); ok {
			take(n)
		}
	}
	return limit, found
}

// blockerCeiling parses a MaxBlockers$ amount: absent is Forge's default 1,
// a non-negative literal is itself, and anything else (an SVar the build
// does not evaluate here) is no ceiling, never a mis-enforced one.
func blockerCeiling(raw string) (int, bool) {
	if raw == "" {
		return 1, true
	}
	n := 0
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c < '0' || c > '9' || n > (math.MaxInt32-9)/10 {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// ValidateBlockRestrict rejects a declaration naming more distinct blockers
// than defender's BlockRestrictLimit allows.
func ValidateBlockRestrict(b Board, defender state.PlayerID, blockers []state.ObjID) error {
	limit, ok := BlockRestrictLimit(b, defender)
	if !ok {
		return nil
	}
	n := 0
	for i, id := range blockers {
		dup := false
		for _, prev := range blockers[:i] {
			if prev == id {
				dup = true
				break
			}
		}
		if !dup {
			n++
		}
	}
	if n > limit {
		return fmt.Errorf("declaration names %d blockers; no more than %d can block this combat", n, limit)
	}
	return nil
}
