//go:build manabrew

package manabrew

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// divisionView is a seat-scoped view mid combat: Alice's 3-power Bear (o2)
// is attacking Bob, blocked by Bob's Giant (o7) and Sentry (o8) — the shape
// the engine's askNextDivision ask rides on (engine/rules/combat.go's
// divisionOptions). The blockers sit on Bob's battlefield in the order the
// attacker's view BlockedBy lists them, which is the engine's recipient
// order (liveBlockers walks o.BlockedBy too).
func divisionView() view.View {
	atk := state.PlayerID(1)
	return view.View{Viewer: 0, Turn: 5, Active: 0, Players: []view.PlayerView{
		{ID: 0, Name: "Alice", Battlefield: []view.CardView{
			{ID: 2, Name: "Bear", Printing: view.Printing{Name: "Bear"}, Types: "Creature — Bear",
				Power: 3, Toughness: 3, Attacking: true, AttackingPlayer: &atk, Blocked: true,
				BlockedBy: []state.ObjID{7, 8}},
		}},
		{ID: 1, Name: "Bob", Battlefield: []view.CardView{
			{ID: 7, Name: "Giant", Printing: view.Printing{Name: "Giant"}, Types: "Creature — Giant", Controller: 1},
			{ID: 8, Name: "Sentry", Printing: view.Printing{Name: "Sentry"}, Types: "Creature — Sentry", Controller: 1},
		}},
	}}
}

// divisionDec is the KChoose ask engine/rules/combat.go's askNextDivision
// poses for that 3-power attacker over two blockers: every nonnegative
// composition of 3 into 2 parts, in lexicographic order, each option's
// Label listing the per-recipient amounts in recipient order and its Amount
// carrying the first recipient's share.
func divisionDec() *decision.Decision {
	d := newDec(21, 0, decision.KChoose,
		decision.Option{Index: 0, Kind: "division", Label: "0 to Giant,3 to Sentry", Obj: 2, Player: 0, Amount: 0},
		decision.Option{Index: 1, Kind: "division", Label: "1 to Giant,2 to Sentry", Obj: 2, Player: 0, Amount: 1},
		decision.Option{Index: 2, Kind: "division", Label: "2 to Giant,1 to Sentry", Obj: 2, Player: 0, Amount: 2},
		decision.Option{Index: 3, Kind: "division", Label: "3 to Giant,0 to Sentry", Obj: 2, Player: 0, Amount: 3})
	d.Prompt = "turn 5 — divide Bear's combat damage among its blockers"
	d.Source = 2
	return d
}

// TestDamageAssignPromptAndAnswer covers the chooseCombatDamageAssignment
// mapping of the combat damage-division ask (CR 510.1c): the double-blocked
// attacker's ask poses the protocol's dedicated prompt, and the
// combatDamageAssignmentDecision answer maps back to exactly the option
// index the native decision path would consume (Decision.Validate stays the
// one legality fence).
func TestDamageAssignPromptAndAnswer(t *testing.T) {
	d := divisionDec()
	p := pendingFor(d, divisionView())
	in, ok := p.Prompt.Input.Value.(mb.ChooseCombatDamageAssignmentInput)
	if !ok {
		t.Fatalf("prompt type = %s, want chooseCombatDamageAssignment", p.Prompt.Input.Value.PromptType())
	}
	if in.AttackerID != "o2" {
		t.Fatalf("attackerId = %q, want o2", in.AttackerID)
	}
	if len(in.BlockerIDs) != 2 || in.BlockerIDs[0] != "o7" || in.BlockerIDs[1] != "o8" {
		t.Fatalf("blockerIds = %v, want [o7 o8] in engine recipient order", in.BlockerIDs)
	}
	if in.TotalDamage != 3 {
		t.Fatalf("totalDamage = %d, want 3", in.TotalDamage)
	}
	if in.DefenderID != "" {
		t.Fatalf("defenderId = %q, want empty: gorge's CR 510.1c decision never assigns to the defender", in.DefenderID)
	}
	if in.AttackerHasDeathtouch {
		t.Fatal("attackerHasDeathtouch = true, want false")
	}

	tr := New("table", 2, nil)
	// The mock client's shape: everything to the first blocker — the option
	// "3 to Giant,0 to Sentry" (the engine's lexicographically last split;
	// its Amount is the attacker's whole power).
	o := tr.TranslateResponse(respFor(p, mb.DamageAssignmentDecision{Assignments: []mb.DamageAssignment{
		{AssigneeID: "o7", Damage: 3}}}), p, 0)
	intent := mustIntent(t, o)
	if err := d.Validate(*intent); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(intent.Choices) != 1 || intent.Choices[0] != 3 {
		t.Fatalf("choices = %v, want [3] (3 to Giant, 0 to Sentry)", intent.Choices)
	}

	// A mid split, answered in reply order independent of the prompt's
	// blocker order: 1 to Sentry, 2 to Giant is the option at index 2.
	o = tr.TranslateResponse(respFor(p, mb.DamageAssignmentDecision{Assignments: []mb.DamageAssignment{
		{AssigneeID: "o8", Damage: 1}, {AssigneeID: "o7", Damage: 2}}}), p, 0)
	intent = mustIntent(t, o)
	if err := d.Validate(*intent); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(intent.Choices) != 1 || intent.Choices[0] != 2 {
		t.Fatalf("choices = %v, want [2]", intent.Choices)
	}

	// Everything to the second blocker is the zero-first split (index 0).
	o = tr.TranslateResponse(respFor(p, mb.DamageAssignmentDecision{Assignments: []mb.DamageAssignment{
		{AssigneeID: "o8", Damage: 3}}}), p, 0)
	if got := mustIntent(t, o).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("choices = %v, want [0]", got)
	}

	// An assignee the prompt never offered is invalidShape.
	o = tr.TranslateResponse(respFor(p, mb.DamageAssignmentDecision{Assignments: []mb.DamageAssignment{
		{AssigneeID: "o9", Damage: 3}}}), p, 0)
	wantErrCode(t, o, mb.CodeInvalidShape)
	// A split no option offers (4 total against a 3-power attacker) is too.
	o = tr.TranslateResponse(respFor(p, mb.DamageAssignmentDecision{Assignments: []mb.DamageAssignment{
		{AssigneeID: "o7", Damage: 2}, {AssigneeID: "o8", Damage: 2}}}), p, 0)
	wantErrCode(t, o, mb.CodeInvalidShape)
	// An empty answer assigns nothing, which is not one of the offered splits.
	o = tr.TranslateResponse(respFor(p, mb.DamageAssignmentDecision{}), p, 0)
	wantErrCode(t, o, mb.CodeInvalidShape)
	// A chooseFromSelection-style answer to this prompt type is the
	// translateResponse routing rejecting the wrong shape outright.
	o = tr.TranslateResponse(respFor(p, mb.SelectionDecision{ChosenIndices: []int{2}}), p, 0)
	wantErrCode(t, o, mb.CodeWrongPromptType)
}

// TestDamageAssignDeathtouchFlag covers the prompt's deathtouch field, read
// off the attacker's view keywords (CR 702.2d halves the lethal amount a
// client rendering the prompt must reason about).
func TestDamageAssignDeathtouchFlag(t *testing.T) {
	v := divisionView()
	v.Players[0].Battlefield[0].Keywords = []string{"Deathtouch"}
	p := pendingFor(divisionDec(), v)
	in, ok := p.Prompt.Input.Value.(mb.ChooseCombatDamageAssignmentInput)
	if !ok {
		t.Fatalf("prompt type = %s, want chooseCombatDamageAssignment", p.Prompt.Input.Value.PromptType())
	}
	if !in.AttackerHasDeathtouch {
		t.Fatal("attackerHasDeathtouch = false, want true")
	}
}

// TestDamageAssignUnresolvableFallsBack pins the never-unanswerable
// contract: whenever the seat's view cannot resolve the ask's recipients —
// here an attacker the view shows as unblocked — the ask poses the generic
// chooseFromSelection fallback (answered by native option index) instead of
// the dedicated prompt, and that fallback's answer still maps.
func TestDamageAssignUnresolvableFallsBack(t *testing.T) {
	v := divisionView()
	v.Players[0].Battlefield[0].BlockedBy = nil
	d := divisionDec()
	p := pendingFor(d, v)
	if got := p.Prompt.Input.Value.PromptType(); got != "chooseFromSelection" {
		t.Fatalf("unblocked-attacker ask: prompt type = %s, want the chooseFromSelection fallback", got)
	}
	tr := New("table", 2, nil)
	o := tr.TranslateResponse(respFor(p, mb.SelectionDecision{ChosenIndices: []int{2}}), p, 0)
	intent := mustIntent(t, o)
	if err := d.Validate(*intent); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(intent.Choices) != 1 || intent.Choices[0] != 2 {
		t.Fatalf("choices = %v, want [2]", intent.Choices)
	}
}

// TestDamageAssignRecipientCountMismatchFallsBack covers the other
// unresolvable shape: the option labels and the view disagree about how
// many recipients the division has (a blocker the view still lists but the
// engine's liveBlockers dropped). The mapping must not invent a position
// mapping it cannot verify: it falls back.
func TestDamageAssignRecipientCountMismatchFallsBack(t *testing.T) {
	v := divisionView()
	v.Players[0].Battlefield[0].BlockedBy = []state.ObjID{7, 8, 12}
	p := pendingFor(divisionDec(), v)
	if got := p.Prompt.Input.Value.PromptType(); got != "chooseFromSelection" {
		t.Fatalf("count-mismatched ask: prompt type = %s, want the chooseFromSelection fallback", got)
	}
}
