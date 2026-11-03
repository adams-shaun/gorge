package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPlayAllowRepeatsCastsTheSameCardSeveralTimes: "Copy that card three
// times. You may cast the copies" is one exiled card offered Amount$ times,
// and each entry the player takes is a fresh copy cast from it. Without
// AllowRepeats$ the one card is struck from the offer after its first play.
func TestPlayAllowRepeatsCastsTheSameCardSeveralTimes(t *testing.T) {
	t.Parallel()
	const exiled = "Name:Exiled spell\nManaCost:1 R\nTypes:Sorcery\nOracle:Deal 3 damage.\n"
	const fixture = "Name:Play repeat fixture\nManaCost:0\nTypes:Sorcery\nOracle:\n"
	for _, tc := range []struct {
		name        string
		amount      string
		repeats     string
		wantOptions int
		take        int
		wantCopies  int
	}{
		{"three repeats", "3", "True", 3, 2, 2},
		{"a counted amount", "Number$3", "True", 3, 3, 3},
		{"without repeats", "3", "", 1, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, _, _ := newFixtureDeck(t, 44031, fixture, exiled)
			original := moveSeeded(t, e, 0, exiled, state.ZExile)
			source := moveSeeded(t, e, 0, fixture, state.ZStack)
			params := map[string]string{
				"Valid": "Card", "ValidZone": "Exile", "CopyCard": "True", "WithoutManaCost": "True",
				"Amount": tc.amount, "Optional": "True",
			}
			if tc.repeats != "" {
				params["AllowRepeats"] = tc.repeats
			}
			e.G.Obj(source).Ability = &cards.SA{Kind: "DB", API: "Play", Params: params}
			before := len(e.L.Events)
			e.pending = nil
			e.resolveTop()
			ask := e.Pending()
			if ask == nil || ask.Kind != decision.KModes || ask.ResumeKind != "play" {
				t.Fatalf("Play ask = %+v", ask)
			}
			if len(ask.Options) != tc.wantOptions || ask.Max != tc.wantOptions {
				t.Fatalf("Play offers %d entries with Max %d, want %d", len(ask.Options), ask.Max, tc.wantOptions)
			}
			for _, o := range ask.Options {
				if o.Obj != original {
					t.Fatalf("an entry names %d, want the exiled card %d", o.Obj, original)
				}
			}
			var picks []int
			for i := 0; i < tc.take; i++ {
				picks = append(picks, ask.Options[i].Index)
			}
			submitChoices(t, e, picks...)
			copies := map[state.ObjID]bool{}
			for _, ev := range e.L.Events[before:] {
				if ev.Kind == events.PutOnStack && ev.Obj != original && ev.Obj != source {
					copies[ev.Obj] = true
				}
			}
			if len(copies) != tc.wantCopies {
				t.Fatalf("%d copies cast, want %d", len(copies), tc.wantCopies)
			}
			if z := e.G.Obj(original).Zone; z != state.ZExile {
				t.Fatalf("the original left exile for %s", z)
			}
		})
	}
}
