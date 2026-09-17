package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestTargetsForEachPlayerUsesDecisionGroups pins Forge TargetRestrictions'
// setForEachPlayer contract on Blatant Thievery's real target shape: at most
// one permanent per opposing controller, with one required from each.
func TestTargetsForEachPlayerUsesDecisionGroups(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Blatant Thievery")
	if !ok {
		t.Fatal("Blatant Thievery missing from corpus")
	}
	sa := card.Faces[0].SpellAbility()
	e := newSeats(t, 3)
	for _, p := range []state.PlayerID{1, 2} {
		for i := 0; i < 2; i++ {
			o := e.G.AddObject(&cards.Card{Faces: []*cards.Face{{Name: "Relic", Types: []string{"Artifact"}}}}, p)
			e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		}
	}
	e.askTarget(0, 0, sa)
	d := e.Pending()
	if d == nil || d.Min != 2 || d.Max != 2 || len(d.Options) != 4 {
		t.Fatalf("target decision = %+v, want two required choices over four options", d)
	}
	if d.Options[0].Group == "" || d.Options[0].Group != d.Options[1].Group || d.Options[0].Group == d.Options[2].Group {
		t.Fatalf("target groups = [%q %q %q], want one group per controller", d.Options[0].Group, d.Options[1].Group, d.Options[2].Group)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0, 1}}); err == nil {
		t.Fatal("two targets controlled by one player were accepted")
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0, 2}}); err != nil {
		t.Fatalf("one target per controller rejected: %v", err)
	}
}
