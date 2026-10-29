package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestTwinAttackProjectionNamesDefender pins the seed-41 attack projection:
// a face-down land is a 2/2 creature for the rules (CR 708.2), so its native
// declare_attack intent must project an attacker option that names the
// defending player. The neutral world's VIEW must still print the visible
// face's type line, because that is the fact native's board adapter reads and
// the reason native's policy declines the attack; rebuilding the observation's
// masked "creature" line as the view's type line would make the two bots
// answer different intents for the same native decision.
func TestTwinAttackProjectionNamesDefender(t *testing.T) {
	s := fixtureSession(t, "Name:Plains\nTypes:Basic Land Plains\n")
	att := obsRef("o-plains", "Plains", "p0")
	faceDown := obsPerm("o-plains", "Plains", "p0", untapped(), "creature")
	faceDown.FaceDown = true
	faceDown.Characteristics = &protocol.Characteristics{
		Types: []string{"creature"}, Power: ptr(int32(2)), Toughness: ptr(int32(2)),
	}
	obs := testObservation("p0", "declare_attackers",
		[]protocol.ObjectRecord{faceDown}, nil)
	def := protocol.PlayerTarget("p1")
	sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 1},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{
			{CandidateID: 0, Semantic: protocol.DeclareAttack(att, &def)},
			{CandidateID: 1, Semantic: protocol.DeclareAttack(att, nil)},
		}}
	p := translated(t, s, sd)
	if p.Decision.Kind != decision.KAttackers {
		t.Fatalf("decision kind %q, want attackers", p.Decision.Kind)
	}
	id := s.B.IDs.Of("o-plains")
	var opt *decision.Option
	for i := range p.Decision.Options {
		if p.Decision.Options[i].Kind == "attacker" && p.Decision.Options[i].Obj == id {
			opt = &p.Decision.Options[i]
			break
		}
	}
	if opt == nil {
		t.Fatalf("the face-down land got no attacker option: %+v", p.Decision.Options)
	}
	// The precondition the defender assertion depends on: the projected view
	// carries the permanent, and the rules still see it as a creature.
	cv, ok := findCard(battlefield(&p.View, 0), id)
	if !ok {
		t.Fatal("precondition: the face-down land is not in the projected view")
	}
	if !cv.FaceDown || !isCreature(cv) {
		t.Fatalf("precondition: projected view face_down=%v creature=%v, want true/true", cv.FaceDown, isCreature(cv))
	}
	// The defender is a player, not the zero-value "no defender": the option's
	// Player is the v2 project's only source for a player attack.
	if opt.Player != 1 {
		t.Fatalf("attacker option defender = %d, want p1 (a null defender is a decline, not this intent)", opt.Player)
	}
	// The view half: native prints the visible face ("Basic Land Plains") for
	// a face-down permanent; the masked observation type is not the view fact.
	if cv.Types != "Basic Land Plains" {
		t.Fatalf("projected view type line = %q, want the visible face %q", cv.Types, "Basic Land Plains")
	}
}
