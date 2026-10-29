package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestTwinBlockRestoresMustBlockDuty pins Watchdog's "blocks each combat if
// able": the v2 observation carries no static registry, so the neutral world
// reads the printed static and restores Option.Required/BlockMust. Without it
// Decision.FitRequired cannot repair the bot's answer and the two agents choose
// different declarations.
func TestTwinBlockRestoresMustBlockDuty(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\n",
		"Name:Watchdog\nTypes:Artifact Creature Dog\nPT:1/2\n"+
			"S:Mode$ MustBlock | ValidCreature$ Card.Self | Description$ CARDNAME blocks each combat if able.\n")
	bear := obsRef("o-bear", "Fixture Bear", "p0")
	dog := obsRef("o-dog", "Watchdog", "p1")
	obs := testObservation("p1", "declare_blockers",
		[]protocol.ObjectRecord{obsPerm("o-bear", "Fixture Bear", "p0", attackingAt("p1"), "creature")},
		[]protocol.ObjectRecord{obsPerm("o-dog", "Watchdog", "p1", untapped(), "creature")})
	sd := protocol.SeatDecision{ActingSeat: "p1", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 1},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{{Semantic: protocol.DeclareBlock(dog, &bear)}}}
	p := translated(t, s, sd)
	if p.Decision.Kind != decision.KBlockers {
		t.Fatalf("decision kind %q, want blockers", p.Decision.Kind)
	}
	dogID := s.B.IDs.Of("o-dog")
	var opt *decision.Option
	for i := range p.Decision.Options {
		if p.Decision.Options[i].Kind == "block" && p.Decision.Options[i].Obj == dogID {
			opt = &p.Decision.Options[i]
			break
		}
	}
	if opt == nil {
		t.Fatalf("precondition: Watchdog got no block option: %+v", p.Decision.Options)
	}
	if !opt.Required || !opt.BlockMust {
		t.Fatalf("Watchdog's block duty was not restored: Required=%v BlockMust=%v", opt.Required, opt.BlockMust)
	}
}

// TestTwinBlockOmitsHiddenRestrictedAttacker pins the offer-derived reading of
// a block restriction the observation cannot carry: an attacker the current
// blocker can block by every public fact, but whose pair the engine did not
// offer, is restricted for this declaration and must not get a derived pair
// for another blocker.
func TestTwinBlockOmitsHiddenRestrictedAttacker(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Shown\nTypes:Creature Bear\nPT:2/2\n",
		"Name:Fixture Hidden\nTypes:Creature Bear\nPT:2/2\n",
		"Name:Fixture Current\nTypes:Creature Soldier\nPT:2/2\n",
		"Name:Fixture Other\nTypes:Creature Soldier\nPT:2/2\n")
	shown := obsRef("o-shown", "Fixture Shown", "p0")
	current := obsRef("o-current", "Fixture Current", "p1")
	obs := testObservation("p1", "declare_blockers",
		[]protocol.ObjectRecord{
			obsPerm("o-shown", "Fixture Shown", "p0", attackingAt("p1"), "creature"),
			obsPerm("o-hidden", "Fixture Hidden", "p0", attackingAt("p1"), "creature"),
		},
		[]protocol.ObjectRecord{
			obsPerm("o-current", "Fixture Current", "p1", untapped(), "creature"),
			obsPerm("o-other", "Fixture Other", "p1", untapped(), "creature"),
		})
	sd := protocol.SeatDecision{ActingSeat: "p1", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 2},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{{Semantic: protocol.DeclareBlock(current, &shown)}}}
	p := translated(t, s, sd)
	if p.Decision.Kind != decision.KBlockers {
		t.Fatalf("decision kind %q, want blockers", p.Decision.Kind)
	}
	other := s.B.IDs.Of("o-other")
	shownID, hiddenID := s.B.IDs.Of("o-shown"), s.B.IDs.Of("o-hidden")
	var shownPair, hiddenPair bool
	for _, o := range p.Decision.Options {
		if o.Kind != "block" || o.Obj != other {
			continue
		}
		if o.Attacker == shownID {
			shownPair = true
		}
		if o.Attacker == hiddenID {
			hiddenPair = true
		}
	}
	if !shownPair {
		t.Fatalf("precondition: the publicly legal pair for the other blocker is missing: %+v", p.Decision.Options)
	}
	if hiddenPair {
		t.Fatalf("an attacker the engine did not offer got a derived pair: %+v", p.Decision.Options)
	}
}
