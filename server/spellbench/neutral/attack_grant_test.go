package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestAttackersRebuildsActivatedCanAttackDefender pins the neutral declaration
// pool's reading of Wall of One Thousand Cuts: a printed-Defender creature
// whose own activated Effect grants it CanAttackDefender this turn is a legal
// attacker, rebuilt from the printed ability plus the session's activation
// census rather than a keyword the observation does not carry.
func TestAttackersRebuildsActivatedCanAttackDefender(t *testing.T) {
	script := "Name:Fixture Wall\nTypes:Creature Wall\nPT:3/5\nK:Defender\n" +
		"A:AB$ Effect | Cost$ W | StaticAbilities$ CanAttack | Duration$ UntilHostLeavesPlayOrEOT | SpellDescription$ CARDNAME can attack this turn as though it didn't have defender.\n" +
		"SVar:CanAttack:Mode$ CanAttackDefender | ValidCard$ Card.EffectSource | Description$ EFFECTSOURCE can attack this turn as though it didn't have defender.\n"
	build := func(uses map[string]int32) ([]decision.Option, state.ObjID) {
		s := fixtureSession(t, script, "Name:Fixture Bears\nTypes:Creature Bear\nPT:2/2\n")
		s.uses, s.usesTurn = uses, 3
		s.activated = map[string]int{"o-wall": 0}
		wall := obsPerm("o-wall", "Fixture Wall", "p0", untapped(), "creature")
		wall.Characteristics.Keywords = []string{"defender"} // the observation's printed keyword
		player := protocol.PlayerTarget("p1")
		cur := obsRef("o-bears", "Fixture Bears", "p0")
		obs := testObservation("p0", "declare_attackers", []protocol.ObjectRecord{
			obsPerm("o-bears", "Fixture Bears", "p0", untapped(), "creature"), wall,
		}, nil)
		sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 2},
			Context: protocol.Context{Kind: "choice"}, Observation: obs,
			Candidates: []protocol.Candidate{{Semantic: protocol.DeclareAttack(cur, &player)}}}
		p := translated(t, s, sd)
		if p.Decision.Kind != decision.KAttackers {
			t.Fatalf("decision kind %q, want attackers", p.Decision.Kind)
		}
		return p.Decision.Options, s.B.IDs.Of("o-wall")
	}
	opts, wall := build(map[string]int32{"o-wall": 1})
	found := false
	for _, o := range opts {
		if o.Kind == "attacker" && o.Obj == wall {
			found = true
		}
	}
	if !found {
		t.Fatalf("activated CanAttackDefender grant did not lift the wall: %+v", opts)
	}
	// Negative control: without this turn's activation the wall stays home.
	opts, wall = build(map[string]int32{})
	for _, o := range opts {
		if o.Kind == "attacker" && o.Obj == wall {
			t.Fatalf("no activation this turn, yet the wall was offered: %+v", opts)
		}
	}
}
