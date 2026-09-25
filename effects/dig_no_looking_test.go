package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestDigNoLooking(t *testing.T) {
	cases := []struct {
		name           string
		params         string
		wantSecretLook bool
		wantPublic     bool
		wantNames      bool
	}{
		{name: "blind", params: " | NoLooking$ True"},
		{name: "ordinary Dig", wantSecretLook: true, wantNames: true},
		{name: "Reveal wins over NoLooking", params: " | NoLooking$ True | Reveal$ True", wantPublic: true, wantNames: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &askHost{}
			h.g = state.NewGame(names(2))
			alpha := mkCard(t, "Name:Alpha Identity\nTypes:Creature\nPT:2/2\nOracle:x\n")
			beta := mkCard(t, "Name:Beta Identity\nTypes:Creature\nPT:3/3\nOracle:x\n")
			alphaID := h.g.AddObject(alpha, 0).ID
			betaID := h.g.AddObject(beta, 0).ID
			h.g.SetZone(state.ZLibrary, 0, []state.ObjID{alphaID, betaID})
			if len(h.g.Zone(state.ZLibrary, 0)) != 2 || alphaID == betaID || h.g.Obj(alphaID).Face().Name == h.g.Obj(betaID).Face().Name {
				t.Fatal("precondition: Dig window must contain two distinct cards with different names")
			}
			effect := sa(t, "SP$ Dig | Defined$ You | DigNum$ 2 | ChangeNum$ 1 | ChangeValid$ Card | DestinationZone$ Hand"+tc.params)
			Resolve(h, &Ctx{Controller: 0}, effect)
			if h.asked == nil || h.asked.Kind != decision.KChoose || len(h.asked.Options) != 2 {
				t.Fatalf("decision = %+v, want a usable choice over both window cards", h.asked)
			}
			for _, id := range []state.ObjID{alphaID, betaID} {
				found := false
				for _, opt := range h.asked.Options {
					if opt.Obj == id {
						found = true
					}
				}
				if !found {
					t.Fatalf("choice options %v lost object reference %d", h.asked.Options, id)
				}
			}
			if tc.wantNames {
				if !strings.Contains(h.asked.Prompt, "Look at") || !strings.Contains(h.asked.Options[0].Label, "Identity") || !strings.Contains(h.asked.Options[1].Label, "Identity") {
					t.Fatalf("ordinary/public decision should name the cards: %+v", h.asked)
				}
			} else {
				if strings.Contains(h.asked.Prompt, "Look at") || strings.Contains(h.asked.Prompt, "Alpha Identity") || strings.Contains(h.asked.Prompt, "Beta Identity") {
					t.Fatalf("blind prompt identifies the window: %q", h.asked.Prompt)
				}
				for _, opt := range h.asked.Options {
					if strings.Contains(opt.Label, "Alpha Identity") || strings.Contains(opt.Label, "Beta Identity") {
						t.Fatalf("blind option identifies a card: %+v", opt)
					}
				}
			}
			var secretIDs, publicIDs []state.ObjID
			for _, ev := range h.log {
				if ev.Kind == events.Note && ev.Secret {
					secretIDs = append(secretIDs, ev.IDs...)
				}
				if ev.Kind == events.Note && !ev.Secret {
					publicIDs = append(publicIDs, ev.IDs...)
				}
			}
			if (len(secretIDs) != 0) != tc.wantSecretLook {
				t.Fatalf("private look IDs = %v, want private disclosure %t", secretIDs, tc.wantSecretLook)
			}
			if tc.wantPublic {
				if len(publicIDs) != 2 || publicIDs[0] != alphaID || publicIDs[1] != betaID {
					t.Fatalf("public reveal IDs = %v, want the complete window [%d %d]", publicIDs, alphaID, betaID)
				}
			} else if len(publicIDs) != 0 {
				t.Fatalf("unexpected public disclosure: %v", publicIDs)
			}

			// An answer still carries real object references and moves the chosen
			// card; NoLooking changes disclosure only, not Dig's selection.
			chosen := h.asked.Options[1].Obj
			Resolve(h, &Ctx{Controller: 0, Dig: []state.ObjID{chosen}, DigDone: true}, effect)
			if h.g.Obj(chosen).Zone != state.ZHand || len(h.g.Zone(state.ZHand, 0)) != 1 {
				t.Fatalf("answer did not move chosen object %d to hand", chosen)
			}
		})
	}
}
