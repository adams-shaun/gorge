package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// kickedRootFake is the kickedRootCensus a unit test scripts: the root's kicked
// minimum and no candidates anywhere.
type kickedRootFake struct {
	kickedMin int
	subs      []*cards.SA
}

func (f kickedRootFake) kickedTargetMin(_ state.PlayerID, _ state.ObjID, sa *cards.SA) int {
	if sa.Sub != nil || len(f.subs) == 0 {
		return f.kickedMin
	}
	return 1
}
func (f kickedRootFake) collectSubTargetPreAsks(*cards.SA) []*cards.SA { return f.subs }
func (f kickedRootFake) legalTargetCandidates(state.PlayerID, state.ObjID, state.ObjID, *cards.SA) []targetCandidate {
	return nil
}

// CR 601.2b, 601.2c: a root whose target minimum reads the kicked bit and which has no
// Kicked-conditioned link (Divine Resilience: "any number of target creatures you control"
// when kicked) announces nothing kicked, so the kicked cast needs no candidate for it. The
// kernel's census (dirac kickedWaivedFeasible) offers it; gorge withheld it
// (dplay pair 53, decision 16).
func TestKickedWaivedRootWithoutLinkIsAvailable(t *testing.T) {
	t.Parallel()
	root := parseFixtureSA(t,
		"SP$ Pump | ValidTgts$ Creature.YouCtrl | TargetMin$ X | TargetMax$ Y | KW$ Indestructible",
		"SVar:X:Count$Kicked.0.1", "SVar:Y:Count$Kicked.Z.1", "SVar:Z:Count$Valid Creature.YouCtrl")
	if root.Sub != nil {
		t.Fatalf("fixture has a Sub; the case under test is a root with none")
	}
	if !kickedWaivedRootTargetsAvailable(kickedRootFake{}, 0, 1, root) {
		t.Errorf("a kicked-waived root with no link must be available")
	}
	if kickedWaivedRootTargetsAvailable(kickedRootFake{kickedMin: 1}, 0, 1, root) {
		t.Errorf("a root the kicked cast still owes a target must stay the plain census's")
	}
}
