package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestNeutralTwinBlockDecision pins the recorded twin block position: the v2
// host's own candidate pair is authoritative for the blocker under decision,
// and the board-derived pairs for the other untapped blockers must stay inside
// combat.CanBlock's legality. The old rebuild offered a ground blocker against
// a Shadow attacker and an equipped blocker that had lost "can't block" to its
// equipment, so the whole-declaration bot spent its plan on pairs the engine
// never offers and the next substeps fell back to a different answer.
func TestNeutralTwinBlockDecision(t *testing.T) {
	t.Run("shadow pairs only with shadow", func(t *testing.T) {
		s := fixtureSession(t,
			"Name:Fixture Shade\nTypes:Creature Shade\nPT:2/1\nK:Shadow\n",
			"Name:Fixture Sentry\nTypes:Creature Soldier\nPT:1/2\nK:Shadow\n",
			"Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\n",
			"Name:Fixture Ground\nTypes:Creature Human Soldier\nPT:2/2\n")
		cur := obsRef("o-shade", "Fixture Shade", "p1")
		sentry := obsPerm("o-sentry", "Fixture Sentry", "p0", attackingAt("p1"), "creature")
		// The v2 observation carries the live keyword set; a Shadow creature
		// is exactly one whose characteristics list it.
		sentry.Characteristics = &protocol.Characteristics{Types: []string{"creature"}, Keywords: []string{"shadow"}}
		shade := obsPerm("o-shade", "Fixture Shade", "p1", untapped(), "creature")
		shade.Characteristics = &protocol.Characteristics{Types: []string{"creature"}, Keywords: []string{"shadow"}}
		obs := testObservation("p1", "declare_blockers",
			[]protocol.ObjectRecord{
				sentry,
				obsPerm("o-bear", "Fixture Bear", "p0", attackingAt("p1"), "creature"),
			},
			[]protocol.ObjectRecord{
				shade,
				obsPerm("o-ground", "Fixture Ground", "p1", untapped(), "creature"),
			})
		// Precondition: the ground blocker is a real, untapped creature that
		// the derived-pair loop reaches, and the two attackers differ only in
		// Shadow.
		groundID := s.B.IDs.Of("o-ground")
		shadeID := s.B.IDs.Of("o-shade")
		sentryID := s.B.IDs.Of("o-sentry")
		bearID := s.B.IDs.Of("o-bear")
		sd := protocol.SeatDecision{ActingSeat: "p1", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 2},
			Context: protocol.Context{Kind: "choice"}, Observation: obs,
			Candidates: []protocol.Candidate{{Semantic: protocol.DeclareBlock(cur, ptr(obsRef("o-sentry", "Fixture Sentry", "p0")))}}}
		p := translated(t, s, sd)
		if p.Decision.Kind != decision.KBlockers {
			t.Fatalf("decision kind %q, want blockers", p.Decision.Kind)
		}
		pairs := map[[2]uint64]bool{}
		for _, o := range p.Decision.Options {
			if o.Kind == "block" {
				pairs[[2]uint64{uint64(o.Obj), uint64(o.Attacker)}] = true
			}
		}
		// Precondition: the current blocker's authoritative pair is there.
		if !pairs[[2]uint64{uint64(shadeID), uint64(sentryID)}] {
			t.Fatalf("current blocker's authoritative pair is missing: %+v", p.Decision.Options)
		}
		// The ground blocker may block the ordinary attacker...
		if !pairs[[2]uint64{uint64(groundID), uint64(bearID)}] {
			t.Fatalf("legal ground pair against a non-Shadow attacker is missing: %+v", p.Decision.Options)
		}
		// ...but not the Shadow one.
		if pairs[[2]uint64{uint64(groundID), uint64(sentryID)}] {
			t.Fatalf("ground blocker was offered against a Shadow attacker (CR 702.28): %+v", p.Decision.Options)
		}
	})

	t.Run("equipment-granted cant block", func(t *testing.T) {
		s := fixtureSession(t,
			"Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\n",
			"Name:Fixture Squire\nTypes:Creature Human Soldier\nPT:1/1\n",
			"Name:Fixture Wall\nTypes:Artifact Creature Wall\nPT:0/5\n",
			"Name:Fixture Plate\nTypes:Artifact Equipment\nK:Equip:2\n"+
				"S:Mode$ CantBlock | ValidCard$ Creature.EquippedBy | Secondary$ True | Description$ Equipped creature can't block.\n")
		cur := obsRef("o-squire", "Fixture Squire", "p1")
		wall := obsRef("o-wall", "Fixture Wall", "p1")
		plateAttached := obsPerm("o-plate", "Fixture Plate", "p1", untapped(), "artifact")
		plateAttached.Permanent.AttachedTo = &protocol.TargetRef{Object: &wall}
		obs := testObservation("p1", "declare_blockers",
			[]protocol.ObjectRecord{obsPerm("o-bear", "Fixture Bear", "p0", attackingAt("p1"), "creature")},
			[]protocol.ObjectRecord{
				obsPerm("o-squire", "Fixture Squire", "p1", untapped(), "creature"),
				obsPerm("o-wall", "Fixture Wall", "p1", untapped(), "creature"),
				plateAttached,
			})
		sd := protocol.SeatDecision{ActingSeat: "p1", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 2},
			Context: protocol.Context{Kind: "choice"}, Observation: obs,
			Candidates: []protocol.Candidate{{Semantic: protocol.DeclareBlock(cur, ptr(obsRef("o-bear", "Fixture Bear", "p0")))}}}
		p := translated(t, s, sd)
		if p.Decision.Kind != decision.KBlockers {
			t.Fatalf("decision kind %q, want blockers", p.Decision.Kind)
		}
		// Precondition: the equipment reached the blocker through the view's
		// AttachedTo link, which is where cantBlock looks.
		wallID := s.B.IDs.Of("o-wall")
		if !attachedTo(p.View, wallID, s.B.IDs.Of("o-plate")) {
			t.Fatal("precondition: the equipment is not attached to the wall in the rebuilt view")
		}
		var hasWall, hasSquire bool
		for _, o := range p.Decision.Options {
			if o.Kind != "block" {
				continue
			}
			if o.Obj == wallID {
				hasWall = true
			}
			if o.Obj == s.B.IDs.Of("o-squire") {
				hasSquire = true
			}
		}
		if !hasSquire {
			t.Fatalf("current blocker's authoritative pair is missing: %+v", p.Decision.Options)
		}
		if hasWall {
			t.Fatalf("a blocker whose equipment says it can't block got a derived pair: %+v", p.Decision.Options)
		}
	})
}

func ptr[T any](v T) *T { return &v }

// attachedTo reports whether the view has a permanent attached to host.
func attachedTo(v view.View, host, attached state.ObjID) bool {
	for _, p := range v.Players {
		for _, cv := range p.Battlefield {
			if cv.ID == attached && cv.AttachedTo == host {
				return true
			}
		}
	}
	return false
}
