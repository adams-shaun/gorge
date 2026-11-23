package kgen

import (
	"maps"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestExpandedKeywordsGrantedLower pins Kodama of the Center Tree's granted
// `Soulshift:N` (CalcKeywordN$ X): gorge's grant is inert, so it lowers to
// nothing, but only when the grant is the static's whole effect.
func TestExpandedKeywordsGrantedLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Kodama of the Center Tree")
	if !ok || card == nil {
		t.Fatal("Kodama of the Center Tree not in the corpus")
	}
	face := card.Faces[0]
	grant := -1
	for i, st := range face.Statics {
		if strings.HasPrefix(st.Params["AddKeyword"], "Soulshift:") && st.Params["CalcKeywordN"] != "" {
			grant = i
		}
	}
	if grant < 0 {
		t.Fatal("precondition: no static with AddKeyword Soulshift:N and CalcKeywordN")
	}
	for _, tr := range face.Triggers {
		t.Fatalf("precondition: Kodama has a trigger %v", tr)
	}
	lower := func(c *cards.Card) *lCard {
		p := &pool{byName: map[string]*cards.Card{c.Faces[0].Name: c}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, c.Faces[0].Name, c, false)
	}

	lc := lower(card)
	if len(lc.Refusals) != 0 {
		t.Fatalf("refusals = %v, want none", lc.Refusals)
	}
	if len(lc.Statics) != 1 || lc.Statics[0].Kind != "CDAPT" {
		t.Errorf("statics = %+v, want only the CDAPT static", lc.Statics)
	}
	if len(lc.Abilities) != 0 {
		t.Errorf("abilities = %v, want none", lc.Abilities)
	}

	// a grant carrying another effect key is not the inert shape: it refuses
	fc := *face
	fc.Statics = append([]cards.Static(nil), face.Statics...)
	ps := maps.Clone(fc.Statics[grant].Params)
	ps["AddPower"] = "1"
	fc.Statics[grant].Params = ps
	card2 := *card
	card2.Faces = []*cards.Face{&fc}
	if lc2 := lower(&card2); len(lc2.Refusals) == 0 {
		t.Errorf("grant with AddPower lowered, want a refusal")
	}
}
