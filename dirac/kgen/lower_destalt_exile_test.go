package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestExileOriginDestAltCarriersRefuseUpstream pins the two corpus cards whose
// ChangeZone carries the unconditional exile-origin DestinationAlternative$
// Hand rider (Origin$ Exile, Destination$ Battlefield,
// DestinationAlternative$ Hand): Ketria and Illuna, Apex of Wishes.
//
// Ketria is a Plane: its rider sits under the ChaosEnsues trigger, which now
// lowers; the card's remaining refusal is the PlaneswalkedTo trigger above it.
// That upstream refusal still hides the rider, so a DestinationAlternative$
// refusal there would mean the guard regressed.
//
// Illuna's trigger and keyword now lower, so the rider is reachable. gorge
// asks the controller to choose the hand or the battlefield before the move
// (askUnconditionalDestination); the kernel has no such prelude ask, so
// lower_destalt.go refuses that shape by name instead.
func TestExileOriginDestAltCarriersRefuseUpstream(t *testing.T) {
	cases := []struct {
		name    string
		svar    string
		want    []string
		destAlt bool // the rider is reachable, so its named refusal is expected
	}{
		{
			name: "Ketria",
			svar: "DBChoose",
			want: []string{"trigger mode PlaneswalkedTo"},
		},
		{
			name:    "Illuna, Apex of Wishes",
			svar:    "DBChange",
			want:    []string{"DB$ ChangeZone: exile-origin DestinationAlternative$ Hand"},
			destAlt: true,
		},
	}
	reg := testutil.CorpusRegistry(t)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			card, ok := reg.Lookup(tc.name)
			if !ok || card == nil {
				t.Fatalf("%s not in the corpus", tc.name)
			}
			// Precondition: the rider really is the unconditional
			// exile-origin battlefield-or-hand alternative at the SVar this
			// case names, so a pass cannot come from a card whose text moved.
			body, ok := card.Faces[0].SVars[tc.svar]
			if !ok {
				t.Fatalf("%s has no SVar %q (have %v)", tc.name, tc.svar, card.Faces[0].SVars)
			}
			for _, want := range []string{"Origin$ Exile", "Destination$ Battlefield", "DestinationAlternative$ Hand"} {
				if !strings.Contains(body, want) {
					t.Fatalf("%s SVar %q does not contain %q: %q", tc.name, tc.svar, want, body)
				}
			}
			for _, unwanted := range []string{"DestAltSVar$", "AlternativeDecider$"} {
				if strings.Contains(body, unwanted) {
					t.Fatalf("%s SVar %q contains %q, so the rider is conditional: %q", tc.name, tc.svar, unwanted, body)
				}
			}
			p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
			lc := lowerCard(p, card.Faces[0].Name, card, false)
			if !tc.destAlt {
				for _, r := range lc.Refusals {
					if strings.Contains(r.Reason, "DestinationAlternative") {
						t.Errorf("%s refuses the rider's DestinationAlternative$ (%q), want an upstream refusal", tc.name, r.Reason)
					}
				}
			}
			for _, want := range tc.want {
				found := false
				for _, r := range lc.Refusals {
					if r.Reason == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("%s refusals = %v, want %q", tc.name, lc.Refusals, want)
				}
			}
		})
	}
}
