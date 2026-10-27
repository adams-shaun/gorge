package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSwatAwayOffersAndPutsAnOpponentsSpellInItsOwnersLibrary: Swat Away's
// ChangeZone has Origin$ Battlefield,Stack, an unmarked Creature alternative,
// Card.inZoneStack, and no TgtZone$ (CR 115.1/115.5). The opponent's creature
// spell on the stack is offered, and on resolution its owner picks the top or
// bottom of their library.
func TestSwatAwayOffersAndPutsAnOpponentsSpellInItsOwnersLibrary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		bottom bool
	}{{"top", false}, {"bottom", true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bearSrc := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
			e := handEngine(t, card(t, bearSrc))
			swat := e.G.AddObject(choiceCorpusCard(t, "Swat Away"), 1)
			swat.Zone = state.ZHand
			e.G.SetZone(state.ZHand, 1, []state.ObjID{swat.ID})
			sa := swat.Face().SpellAbility()
			if sa.Params["TgtZone"] != "" || sa.Params["Origin"] != "Battlefield,Stack" {
				t.Fatalf("Swat Away no longer has the unmarked-origin shape: %v", sa.Params)
			}
			bearID := e.G.Zone(state.ZHand, 0)[0]
			onField := onBoard(t, e, 0, "Name:Ox\nTypes:Creature Ox\nPT:2/2\nOracle:x\n")
			e.G.Players[0].Pool[state.MG] = 5
			e.G.Players[1].Pool[state.MU] = 5
			e.askPriority(0)

			submitChoices(t, e, passToCast(t, e, bearID))
			if len(e.G.Stack) != 1 || e.G.Stack[0] != bearID {
				t.Fatalf("bear not on the stack: %v", e.G.Stack)
			}
			submitChoices(t, e, passToCast(t, e, swat.ID))
			d := e.Pending()
			if d == nil || d.Kind != decision.KTarget {
				t.Fatalf("expected a target decision, got %+v", d)
			}
			idx, fieldOffered := -1, false
			for _, o := range d.Options {
				if o.Obj == bearID {
					idx = o.Index
				}
				if o.Obj == onField {
					fieldOffered = true
				}
			}
			// The unmarked Creature alternative stays a battlefield target
			// beside the stack one (the base census was the stack alone).
			if !fieldOffered {
				t.Fatalf("battlefield creature %d not offered: %+v", onField, d.Options)
			}
			if idx < 0 {
				t.Fatalf("opponent's spell %d not offered: %+v", bearID, d.Options)
			}
			submitChoices(t, e, idx)

			// Resolve Swat Away; the bear's owner (seat 0) answers the
			// top-or-bottom question.
			var asked *decision.Decision
			for i := 0; i < 12 && !e.G.Over; i++ {
				p := e.Pending()
				if p != nil && p.Kind == decision.KChoose && p.ResumeKind == "changezone_alternative" {
					asked = p
					break
				}
				if p == nil || p.Kind != decision.KPriority {
					t.Fatalf("unexpected decision %+v", p)
				}
				submitChoicePass(t, e)
			}
			if asked == nil {
				t.Fatalf("no top-or-bottom ask; stack %v", e.G.Stack)
			}
			if asked.Player != 0 {
				t.Fatalf("top-or-bottom asked of seat %d, want the owner (0)", asked.Player)
			}
			pick := 0
			if tc.bottom {
				pick = 1
			}
			submitChoices(t, e, pick)
			passUntilStackEmpty(t, e, 8)

			lib := e.G.Zone(state.ZLibrary, 0)
			if z := e.G.Obj(bearID).Zone; z != state.ZLibrary {
				t.Fatalf("bear zone = %v, want library", z)
			}
			at := 0
			if tc.bottom {
				at = len(lib) - 1
			}
			if lib[at] != bearID {
				t.Fatalf("bear at wrong library end (bottom=%v): top=%d bottom=%d bear=%d", tc.bottom, lib[0], lib[len(lib)-1], bearID)
			}
		})
	}
}
