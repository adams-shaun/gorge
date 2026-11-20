package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The gap-4 expanded heads (Soulshift, Reinforce, Embalm, Eternalize) lower
// from gorge's expansions (cards/kw_soulshift.go, kw_reinforce.go,
// kw_embalm.go): the handler only checks the face carries the expansion and
// refuses by name when it does not, so the K: line's own lowering is the
// check this test pins.

// expandedkw4Carriers is one carrier per head the corpus prints.
var expandedkw4Carriers = map[string][]string{
	"Soulshift":  {"Deathknell Kami", "Kami of Empty Graves", "Moonlit Strider", "Vine Kami"},
	"Reinforce":  {"Break Ties", "Swell of Courage", "Wren's Run Hydra", "Rustic Clachan"},
	"Embalm":     {"Labyrinth Guardian", "Aven Initiate"},
	"Eternalize": {"Adorned Pouncer", "Steadfast Sentinel", "Fanatic of Rhonas", "Resilient Khenra"},
}

// TestExpandedKeywordsLower lowers the four heads on their carriers (no
// refusal: the expansion's own lines lower), and the same face without its
// expansion refuses by the new reason -- the printed K: line stays, the
// trigger or ability it expanded into is gone.
func TestExpandedKeywordsLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, head := range []string{"Soulshift", "Reinforce", "Embalm", "Eternalize"} {
		for _, name := range expandedkw4Carriers[head] {
			card, ok := reg.Lookup(name)
			if !ok || card == nil {
				t.Fatalf("%s not in the corpus", name)
			}
			p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
			lc := lowerCard(p, card.Faces[0].Name, card, false)
			if len(lc.Refusals) != 0 {
				t.Errorf("%s (%s) refusals = %v, want none", name, head, lc.Refusals)
				continue
			}
			// the same face without its expansion: the head refuses
			fc := *card.Faces[0]
			switch head {
			case "Soulshift":
				fc.Triggers = nil
			default:
				abs := make([]*cards.SA, 0, len(fc.Abilities))
				for _, a := range fc.Abilities {
					if !strings.HasPrefix(a.Params["KeywordLine"], head) {
						abs = append(abs, a)
					}
				}
				fc.Abilities = abs
			}
			card2 := *card
			card2.Faces = []*cards.Face{&fc}
			p2 := &pool{byName: map[string]*cards.Card{fc.Name: &card2}, nDecks: map[string]int{}, reg: reg}
			lc2 := lowerCard(p2, fc.Name, &card2, false)
			want := `keyword "` + head + `" without its expansion`
			found := false
			for _, r := range lc2.Refusals {
				found = found || strings.Contains(r.Reason, want)
			}
			if !found {
				t.Errorf("%s without its %s expansion refusals = %v, want %q", fc.Name, head, lc2.Refusals, want)
			}
		}
	}
}
