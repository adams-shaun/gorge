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
// Neither card can reach that rider in kgen today, so a kernel exile-origin
// DestAlt arm has no carrier it could lower or a lockstep game could play.
// Ketria is a Plane: its rider sits under the ChaosEnsues trigger. Illuna's
// rider sits under the Mutates trigger, and the card also prints K:Mutate.
// lower_trigger.go's mode switch refuses both trigger modes in its default
// arm, and lower.go refuses the unknown Mutate keyword, all before the
// ChangeZone rider's parameters are read. These refusals are the upstream
// shape, not a DestinationAlternative$ refusal.
//
// Whoever lowers Mutate, Mutates or ChaosEnsues must update this test: the
// exile-origin DestinationAlternative$ arm then becomes reachable and must be
// lowered as a prelude ask before the move, the way gorge's
// askUnconditionalDestination does, or refused by name. Do not simply delete
// these expectations.
func TestExileOriginDestAltCarriersRefuseUpstream(t *testing.T) {
	cases := []struct {
		name string
		svar string
		want []string
	}{
		{
			name: "Ketria",
			svar: "DBChoose",
			want: []string{"trigger mode ChaosEnsues"},
		},
		{
			name: "Illuna, Apex of Wishes",
			svar: "DBChange",
			want: []string{`keyword "Mutate"`, "trigger mode Mutates"},
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
			for _, r := range lc.Refusals {
				if strings.Contains(r.Reason, "DestinationAlternative") {
					t.Errorf("%s refuses the rider's DestinationAlternative$ (%q), want an upstream refusal", tc.name, r.Reason)
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
