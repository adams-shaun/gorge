package neutral

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestNeutralRepeatableActivationMatchesNative pins seed 2834120232294243438,
// decision 807 (run seed 53, twin, autopay all): p1's Staff of Domination has
// been activated four times this turn, which is the bot's A5 repeatability
// budget, so native passes rather than activating "{1}: untap CARDNAME" a
// fifth time. The neutral world counted each activation only while its ability
// was on the stack and folded it back when the ability resolved, because the
// source was tapped when the activation was first seen and untapped
// afterwards: exactly the signature a self-untap ability leaves by resolving,
// not by being reversed (CR 733.1). The census fell to zero, the budget
// reopened and the neutral seat activated a mana sink native had declined.
//
// The fixture is Staff's shape: a repeatable {1} self-untap ability. Four
// activations are observed on the stack while the artifact is tapped, then
// all four resolve (empty stack, source untapped) and the same priority
// decision is posed. The rebuilt census must still be 4, and the bot must
// answer it exactly as native's: pass.
func TestNeutralRepeatableActivationMatchesNative(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Staff\nTypes:Artifact\nA:AB$ Untap | Cost$ 1 | SpellDescription$ Untap CARDNAME.\n")
	staff := obsRef("o-staff", "Fixture Staff", "p1")
	obsWith := func(activations int, tapped bool) protocol.Observation {
		staffRec := obsPerm("o-staff", "Fixture Staff", "p1", &protocol.Permanent{Tapped: tapped}, "artifact")
		obs := protocol.Observation{Viewer: "p1", Turn: 32, PhaseStep: "precombat_main",
			ActiveSeat: ptr("p1"), PrioritySeat: ptr("p1"),
			Players: [2]protocol.PlayerObs{
				{Seat: "p0", Life: 20},
				{Seat: "p1", Life: 20, Battlefield: []protocol.ObjectRecord{staffRec}},
			}}
		for i := 0; i < activations; i++ {
			obj := staff
			obj.ObjectID = fmt.Sprintf("o-ability-%d", i)
			obs.Stack = append(obs.Stack, protocol.StackEntry{ObjectRef: obj, StackKind: "activated_ability", Source: &staff})
		}
		return obs
	}
	decision := func(obs protocol.Observation) protocol.SeatDecision {
		return protocol.SeatDecision{ActingSeat: "p1", Context: protocol.Context{Kind: "priority"}, Observation: obs,
			Candidates: []protocol.Candidate{
				{Semantic: protocol.Pass()},
				{Semantic: protocol.ActivateAbility(staff, 0)},
			}}
	}
	// Four activations observed on the stack while the source is tapped: the
	// bot's census counts each one.
	for i := 1; i <= 4; i++ {
		p := translated(t, s, decision(obsWith(i, true)))
		if len(p.View.Players[1].Battlefield) != 1 {
			t.Fatalf("precondition: the fixture artifact is not on p1's battlefield")
		}
		s.Picked(1) // the neutral seat answered the activation
	}
	// They all resolve: the stack is empty and the self-untap ability has left
	// the source untapped. That is a normal resolution, so the census stays 4.
	p := translated(t, s, decision(obsWith(0, false)))
	staffID := s.B.IDs.Of(staff.ObjectID)
	var activated int32
	for _, cv := range p.View.Players[1].Battlefield {
		if cv.ID == staffID {
			activated = cv.ActivatedThisTurn
		}
	}
	if activated != 4 {
		t.Fatalf("precondition: rebuilt ActivatedThisTurn = %d, want 4 after four resolved activations", activated)
	}
	board := heuristic.BoardFromView(p.View)
	in := botpolicy.Decide(board, &p.Decision, rand.New(rand.NewPCG(1, 1)))
	for _, ch := range in.Choices {
		if ch >= 0 && ch < len(p.Decision.Options) && p.Decision.Options[ch].Kind == "ability" {
			t.Fatalf("the neutral bot activated a fifth time (option %d: %q); native passed at the A5 budget",
				ch, p.Decision.Options[ch].Label)
		}
	}
	// Control: the same decision with the census zeroed is worth activating,
	// so the pass above really is the census and not an unranked option.
	if len(p.View.Players[1].Battlefield) == 1 {
		p.View.Players[1].Battlefield[0].ActivatedThisTurn = 0
	}
	control := heuristic.BoardFromView(p.View)
	cin := botpolicy.Decide(control, &p.Decision, rand.New(rand.NewPCG(1, 1)))
	activatedControl := false
	for _, ch := range cin.Choices {
		if ch >= 0 && ch < len(p.Decision.Options) && p.Decision.Options[ch].Kind == "ability" {
			activatedControl = true
		}
	}
	if !activatedControl {
		t.Fatalf("control: with ActivatedThisTurn 0 the bot should take the {1} ability, got %v", cin.Choices)
	}
}
