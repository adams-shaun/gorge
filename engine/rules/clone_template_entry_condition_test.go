package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCloneTemplateEntryConditionReadsTemplateSVars: Mockingbird (X paid, so
// its own X SVar reads non-zero) copying a creature that "enters with a +1/+1
// counter if an opponent lost life this turn" gates the counter on the
// TEMPLATE's CheckSVar$ table (CR 707.2), not the entrant's face, which would
// let the gate pass whenever X was paid.
func TestCloneTemplateEntryConditionReadsTemplateSVars(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		tpl  string
		x    int
		lost bool
		want int32
	}{
		{"Cindering Cutthroat/no opponent lost life", "Cindering Cutthroat", 2, false, 0},
		{"Cindering Cutthroat/an opponent lost life", "Cindering Cutthroat", 2, true, 1},
		{"Frilled Sparkshooter/no opponent lost life", "Frilled Sparkshooter", 4, false, 0},
		{"Frilled Sparkshooter/an opponent lost life", "Frilled Sparkshooter", 4, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := handEngine(t, corpusAlternativeCard(t, "Mockingbird"))
			tpl := e.G.AddObject(corpusAlternativeCard(t, tc.tpl), 1)
			tpl.Zone = state.ZBattlefield
			e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{tpl.ID})
			if tc.lost {
				e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
			}
			if lost := e.G.Players[1].LifeLostThisTurn; (lost > 0) != tc.lost {
				t.Fatalf("precondition: opponent LifeLostThisTurn = %d, lost = %v", lost, tc.lost)
			}
			id := e.G.Zone(state.ZHand, 0)[0]
			e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU] = int32(tc.x), 1
			castMode(t, e, id, "")
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose || len(d.Options) < 3 || d.Options[0].Kind != "x" {
				t.Fatalf("expected X payment choice, got %+v", d)
			}
			submitChoices(t, e, tc.x)
			resolveTopProbe(e)
			d = e.Pending()
			if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
				t.Fatalf("expected ETB election, got %+v", d)
			}
			choice := -1
			for _, opt := range d.Options {
				if opt.Obj == tpl.ID {
					choice = opt.Index
				}
			}
			if choice < 0 {
				t.Fatalf("precondition: %s not offered: %+v", tc.tpl, d.Options)
			}
			submitChoices(t, e, choice)
			settleDrain(e)
			if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Face().Name != tc.tpl {
				t.Fatalf("precondition: Mockingbird did not enter as %s: %+v", tc.tpl, o)
			}
			if got := e.G.Obj(id).Counter("P1P1"); got != tc.want {
				t.Fatalf("copy P1P1 counters = %d, want %d", got, tc.want)
			}
		})
	}
}
