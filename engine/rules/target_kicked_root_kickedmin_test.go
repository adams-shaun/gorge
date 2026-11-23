package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// kickedMinFake scripts a chain whose root takes no target when kicked and whose one link
// owes subMin kicked targets from cands candidates.
type kickedMinFake struct {
	subMin, cands int
	sub           *cards.SA
}

func (f kickedMinFake) kickedTargetMin(_ state.PlayerID, _ state.ObjID, sa *cards.SA) int {
	if sa == f.sub {
		return f.subMin
	}
	return 0
}
func (f kickedMinFake) collectSubTargetPreAsks(*cards.SA) []*cards.SA { return []*cards.SA{f.sub} }
func (f kickedMinFake) legalTargetCandidates(state.PlayerID, state.ObjID, state.ObjID, *cards.SA) []targetCandidate {
	return make([]targetCandidate, f.cands)
}

// CR 601.2b, 601.2c: Highly Illogical's kicked cast announces no root target ("counter target
// spell with mana value 2 or less" has TargetMin$ X over Count$Kicked.0.1) and one target for
// the link ("instead counter target spell", TargetMin$ Y over Count$Kicked.1.0), which has no
// Condition$ Kicked. With only a bigger spell on the stack the plain cast is dead and the
// kicked cast is legal; with none the link has nothing to take.
func TestKickedWaivedRootWithUnconditionedKickedMinLink(t *testing.T) {
	t.Parallel()
	root := parseFixtureSA(t,
		"SP$ Counter | TargetType$ Spell | ValidTgts$ Card.cmcLE2 | TargetMin$ X | TargetMax$ X | SubAbility$ DBCounter",
		"SVar:X:Count$Kicked.0.1")
	sub := parseFixtureSA(t,
		"DB$ Counter | TargetType$ Spell | ValidTgts$ Card | TargetMin$ Y | TargetMax$ Y",
		"SVar:Y:Count$Kicked.1.0")
	root.Sub = sub // the chain under test: a root with a link, which the old census left to the plain cast
	if got := sub.ParamStr(cards.PKCondition); got != "" {
		t.Fatalf("fixture link is conditioned (%q); the case under test has no Condition$", got)
	}
	if !kickedWaivedRootTargetsAvailable(kickedMinFake{subMin: 1, cands: 1, sub: sub}, 0, 1, root) {
		t.Errorf("a kicked-waived root whose unconditioned link has its candidate must be available")
	}
	if kickedWaivedRootTargetsAvailable(kickedMinFake{subMin: 1, cands: 0, sub: sub}, 0, 1, root) {
		t.Errorf("a link with no candidate must leave the kicked cast unavailable")
	}
}
