package neutral

import (
	"encoding/json"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNeutralUpToNTargetsSeesNativeFacts pins seed 7577800171980809590
// decision 840 (run seed 41): Unexplained Absence asks "for each player,
// exile up to one target nonland permanent that player controls"
// (TargetsForEachPlayer$ True), so the two picks must come from different
// controllers. The wire's choose_target carries no Group, so the neutral
// world used to rebuild the ask without the per-controller grouping; the bot
// picked a second target of the controller it had already chosen (Ancient
// Brontodon beside The Watcher in the Water), that option was no longer
// offered at the next substep, and the neutral seat finished where native
// took the second controller's target (Breaker of Armies).
func TestNeutralUpToNTargetsSeesNativeFacts(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	name := func(n string) *string { return &n }
	src := protocol.ObjectRef{ObjectID: "o-abs", CardName: name("Unexplained Absence"),
		OwnerSeat: "p0", ControllerSeat: "p0", Zone: "stack"}
	permanent := func(id, card, seat string, power, toughness int32) protocol.ObjectRecord {
		return protocol.ObjectRecord{
			ObjectRef: protocol.ObjectRef{ObjectID: id, CardName: name(card), OwnerSeat: seat,
				ControllerSeat: seat, Zone: "battlefield"},
			Characteristics: &protocol.Characteristics{Types: []string{"creature"},
				Power: &power, Toughness: &toughness},
			Permanent: &protocol.Permanent{},
		}
	}
	// The opponent's two 9/9s tie for the bot's threat score; own Breaker of
	// Armies (10/8) is the strongest own option. The first pick must be an
	// opponent creature, and the per-controller cap must then send the second
	// pick to our own board instead of the opponent's other 9/9.
	watcher := permanent("o-watcher", "The Watcher in the Water", "p1", 9, 9)
	brontodon := permanent("o-brontodon", "Ancient Brontodon", "p1", 9, 9)
	breaker := permanent("o-breaker", "Breaker of Armies", "p0", 10, 8)
	servitor := permanent("o-servitor", "Blood Servitor", "p0", 2, 2)
	stack := []protocol.StackEntry{{ObjectRef: src, StackKind: "spell",
		Characteristics: &protocol.Characteristics{Types: []string{"instant"}}}}
	obs := &protocol.Observation{Viewer: "p0", Turn: 5, PhaseStep: "precombat_main",
		Players: [2]protocol.PlayerObs{
			{Seat: "p0", Life: 20, Battlefield: []protocol.ObjectRecord{breaker, servitor}},
			{Seat: "p1", Life: 20, Battlefield: []protocol.ObjectRecord{watcher, brontodon}},
		}, Stack: stack}
	ctx := protocol.Context{Kind: "choice", Source: &src}
	targetCand := func(t protocol.TargetRef, sel uint32) protocol.Candidate {
		return protocol.Candidate{Semantic: protocol.ChooseTarget(src, 0, t, sel, 0, 2)}
	}
	objectTarget := func(r protocol.ObjectRecord) protocol.TargetRef {
		return protocol.ObjectTarget(r.ObjectRef)
	}
	first := protocol.SeatDecision{ActingSeat: "p0", Context: ctx, Observation: *obs,
		Candidates: []protocol.Candidate{
			targetCand(objectTarget(watcher), 0), targetCand(objectTarget(brontodon), 0),
			targetCand(objectTarget(breaker), 0), targetCand(objectTarget(servitor), 0),
			// The finish candidate the pose offers once Min is met.
			{Semantic: protocol.FinishTargetSelection(src, 0, 0)},
		}}
	// The continuation after a first pick from the opponent's controller: the
	// host no longer offers the already-capped controller's other permanent,
	// so a plan whose second target is Brontodon can only finish.
	next := protocol.SeatDecision{ActingSeat: "p0", Context: ctx, Observation: *obs,
		Candidates: []protocol.Candidate{
			targetCand(objectTarget(breaker), 1), targetCand(objectTarget(servitor), 1),
			{Semantic: protocol.FinishTargetSelection(src, 0, 1)},
		}}

	// The host sends the decision as JSON; decode it the same way so every
	// semantic field is the raw object the session's readers expect.
	for _, sd := range []*protocol.SeatDecision{&first, &next} {
		raw, err := json.Marshal(sd)
		if err != nil {
			t.Fatalf("marshal decision: %v", err)
		}
		if err := json.Unmarshal(raw, sd); err != nil {
			t.Fatalf("decode decision: %v", err)
		}
	}
	s := NewSession(reg)
	p1, err := s.Payload(&first)
	if err != nil {
		t.Fatalf("first payload: %v", err)
	}
	// Precondition: the neutral ask is up-to-two with one target per
	// controller. Both 9/9s are offered, so the cap is what stops a second
	// pick from their controller -- a plain up-to-two ask would take both.
	if p1.Decision.Max != 2 || len(p1.Decision.Options) != 4 {
		t.Fatalf("rebuilt ask: max=%d options=%d, want max 2 over 4 targets",
			p1.Decision.Max, len(p1.Decision.Options))
	}
	in := botpolicy.Decide(heuristic.BoardFromView(p1.View), &p1.Decision, rand.New(rand.NewPCG(1, 1)))
	if len(in.Choices) != 2 {
		t.Fatalf("native-width plan chose %v, want two targets", in.Choices)
	}

	p2, err := s.Payload(&next)
	if err != nil {
		t.Fatalf("continuation payload: %v", err)
	}
	// The agent's Pick walks the plan's choices against the candidates' ops
	// in order, then falls back to finish when the next choice is not
	// offered. That fallback is the disagreement this test removes.
	second := in.Choices[1]
	picked := -1
	for i, op := range p2.Ops {
		if op.Op == "choose" && op.Option == second {
			picked = i
			break
		}
	}
	if picked < 0 {
		for i, op := range p2.Ops {
			if op.Op == "finish" {
				picked = i
				break
			}
		}
	}
	if picked < 0 {
		t.Fatal("no candidate for the plan's second target and no finish candidate")
	}
	tgt, err := target(next.Candidates[picked].Semantic, "target")
	if err != nil || tgt == nil || tgt.Object == nil {
		t.Fatalf("picked candidate %d names no object target: %v", picked, err)
	}
	if *tgt.Object.CardName != "Breaker of Armies" {
		label := ""
		if second >= 0 && second < len(p2.Decision.Options) {
			label = p2.Decision.Options[second].Label
		}
		t.Fatalf("neutral second target = %s, want Breaker of Armies (the plan's option %d was %q)",
			*tgt.Object.CardName, second, label)
	}
	// The mechanism the choice depends on: the rebuilt options carry the
	// engine's per-controller Group, which is what paced the second pick to
	// the other controller.
	seen := map[string]bool{}
	for i := range p1.Decision.Options {
		if g := p1.Decision.Options[i].Group; g != "" {
			seen[g] = true
		}
	}
	if len(seen) != 2 {
		t.Fatalf("rebuilt options carry %d target-controller groups, want 2 (one per controller)", len(seen))
	}
}
