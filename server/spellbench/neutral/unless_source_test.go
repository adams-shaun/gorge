package neutral

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNeutralUnlessCostUsesSourcePermanent pins that the neutral world's
// unless-cost options name the ability's source permanent (the native
// decision's Obj), not the trigger wrapper on the stack: the bot's sacrifice
// offer prices that object, and a stack wrapper has no card facts.
func TestNeutralUnlessCostUsesSourcePermanent(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup("Vexing Devil")
	if !ok || len(c.Faces) == 0 {
		t.Fatal("precondition: Vexing Devil is not in the corpus")
	}
	var sa *cards.SA
	for _, tr := range c.Faces[0].Triggers {
		if tr.Effect != nil && tr.Effect.Params["UnlessCost"] != "" {
			sa = tr.Effect
		}
	}
	if sa == nil || sa.API != "Sacrifice" || sa.Params["UnlessCost"] != "DamageYou<4>" {
		t.Fatalf("precondition: Vexing Devil trigger effect = %+v, want Sacrifice/DamageYou<4>", sa)
	}
	name := "Vexing Devil"
	perm := protocol.ObjectRef{ObjectID: "o-vex-perm", CardName: &name, OwnerSeat: "p0", ControllerSeat: "p0", Zone: "battlefield"}
	trig := protocol.ObjectRef{ObjectID: "o-vex-trig", CardName: &name, OwnerSeat: "p0", ControllerSeat: "p0", Zone: "stack"}
	power, toughness := int32(4), int32(3)
	chars := &protocol.Characteristics{Types: []string{"creature"}, Subtypes: []string{"devil"}, Colors: []string{"red"},
		ManaValue: 1, Power: &power, Toughness: &toughness}
	p0, p1 := "p0", "p1"
	sd := protocol.SeatDecision{
		ActingSeat: "p1",
		Group:      protocol.Group{SubstepIndex: 0, SubstepCount: 1},
		Context:    protocol.Context{Kind: "choice", Source: &trig},
		Observation: protocol.Observation{Viewer: "p1", Turn: 5, PhaseStep: "precombat_main", ActiveSeat: &p0, PrioritySeat: &p1,
			Players: [2]protocol.PlayerObs{
				{Seat: "p0", Life: 20, Battlefield: []protocol.ObjectRecord{{ObjectRef: perm, Permanent: &protocol.Permanent{}, Characteristics: chars}}},
				{Seat: "p1", Life: 11},
			},
			Stack: []protocol.StackEntry{{ObjectRef: trig, StackKind: "triggered_ability", Source: &perm}}},
		Candidates: []protocol.Candidate{
			{CandidateID: 0, Semantic: protocol.Semantic{Kind: "optional_cost", Fields: map[string]any{"cost": "other", "pay": true, "source": trig}}},
			{CandidateID: 1, Semantic: protocol.Semantic{Kind: "optional_cost", Fields: map[string]any{"cost": "other", "pay": false, "source": trig}}},
		},
	}
	// Round-trip through the wire the way the host poses it: the semantic
	// fields become json.RawMessage, which the translation's field readers
	// require.
	raw, err := json.Marshal(sd)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &sd); err != nil {
		t.Fatal(err)
	}
	s := NewSession(reg)
	p, err := s.Payload(&sd)
	if err != nil {
		t.Fatal(err)
	}
	if p.Decision.Kind != decision.KModes || p.Decision.ResumeKind != "unless_pay" || p.Decision.ResumeSA == nil {
		t.Fatalf("translated decision = %+v, want the unless_pay KModes ask", p.Decision)
	}
	if len(p.Decision.Options) != 2 || p.Decision.Options[0].Mode != decision.ModeUnlessPay || p.Decision.Options[1].Mode != decision.ModeUnlessDecline {
		t.Fatalf("translated options = %+v, want pay then decline", p.Decision.Options)
	}
	permID := s.B.IDs.Of(perm.ObjectID)
	for _, o := range p.Decision.Options {
		if o.Obj != permID {
			t.Fatalf("option %d (%s) names object %d, want the source permanent %d (the stack trigger is %d)",
				o.Index, o.Label, o.Obj, permID, s.B.IDs.Of(trig.ObjectID))
		}
	}
	brd := heuristic.BoardFromView(p.View)
	if card, ok := brd.Cards.Lookup(permID); !ok || !card.Creature || card.Power != 4 {
		t.Fatalf("precondition: board card for the source permanent = %+v (found %v), want a 4-power creature", card, ok)
	}
	bot := heuristic.NewBot(16462000697783136304)
	got, err := bot.Decide(context.Background(), p.View, p.Decision)
	if err != nil {
		t.Fatal(err)
	}
	native := decision.Decision{Kind: decision.KModes, Min: 1, Max: 1, Player: 1, Source: permID,
		ResumeKind: "unless_pay", ResumeSA: sa,
		Options: []decision.Option{
			{Index: 0, Kind: "mode", Label: "Take 4 damage", Obj: permID, Player: 1, Mode: decision.ModeUnlessPay},
			{Index: 1, Kind: "mode", Label: "Refuse — it stays", Obj: permID, Player: 1, Mode: decision.ModeUnlessDecline},
		}}
	want, err := heuristic.NewBot(16462000697783136304).Decide(context.Background(), p.View, native)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Choices) != 1 || len(want.Choices) != 1 || got.Choices[0] != want.Choices[0] {
		t.Fatalf("neutral answer %v, native-shaped answer %v: the same bot must pick the same pay value", got.Choices, want.Choices)
	}
	if got.Choices[0] != p.Decision.Options[0].Index {
		t.Fatalf("neutral answer %v does not pick the pay option (%d)", got.Choices, p.Decision.Options[0].Index)
	}
}
