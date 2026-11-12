package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGraveyardActivatedKeywordGrantsRefuseShaped pins the shaped refusal for
// the class of battlefield permanents that grant an activated keyword to cards
// in a graveyard: Varolz, the Scar-Striped and every other corpus card whose
// AffectedZone$ Graveyard static grants Scavenge, Encore, Unearth or Embalm.
// dirac has no lowering for those activations, so each card must keep being
// refused by name at its keyword instead of the generic AffectedZone reason.
func TestGraveyardActivatedKeywordGrantsRefuseShaped(t *testing.T) {
	cases := []struct {
		name string
		head string
	}{
		{"Varolz, the Scar-Striped", "Scavenge"},
		{"Young Deathclaws", "Scavenge"},
		{"The Cave of Skulls", "Scavenge"},
		{"Wire Surgeons", "Encore"},
		{"Graywater's Fixer", "Encore"},
		{"Sliver Gravemother", "Encore"},
		{"Mishra, Tamer of Mak Fawa", "Unearth"},
		{"Dregscape Sliver", "Unearth"},
		{"Grixis", "Unearth"},
		{"Solemn Doomguide", "Unearth"},
		{"Sedris, the Traitor King", "Unearth"},
		{"Naktamun", "Embalm"},
	}
	reg := testutil.CorpusRegistry(t)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			card, ok := reg.Lookup(tc.name)
			if !ok || card == nil {
				t.Fatalf("%s not in the corpus", tc.name)
			}
			// Precondition: the printed static really is a graveyard grant of
			// this keyword, so a pass cannot come from a card whose text moved.
			grant := false
			for _, st := range card.Faces[0].Statics {
				if st.Params["AffectedZone"] == "Graveyard" && strings.HasPrefix(st.Params["AddKeyword"], tc.head) {
					grant = true
				}
			}
			if !grant {
				t.Fatalf("%s has no AffectedZone$ Graveyard AddKeyword$ %s static", tc.name, tc.head)
			}
			p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
			lc := lowerCard(p, card.Faces[0].Name, card, false)
			want := "(" + tc.head + " grant not modelled)"
			shaped := 0
			const bare = `AffectedZone "Graveyard" without MayPlay`
			for _, r := range lc.Refusals {
				if strings.Contains(r.Reason, want) {
					shaped++
				}
				if r.Reason == bare {
					t.Errorf("%s refusal %q is the bare umbrella reason, want the keyword named", tc.name, r.Reason)
				}
			}
			if shaped != 1 {
				t.Fatalf("%s refusals = %v, want exactly one containing %q", tc.name, lc.Refusals, want)
			}
		})
	}
}
