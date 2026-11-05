package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestMayFlashBeholdLowering: a K:MayFlashCost extra that is exactly one
// Behold<1/Type> lowers with no mana (lCard.MayFlash empty, MayFlashBehold
// the candidates' filter; Molten Exhale); a behold count above 1, a behold
// beside mana and a non-creature type keep refusing it.
func TestMayFlashBeholdLowering(t *testing.T) {
	face := func(kw string) *cards.Card {
		return &cards.Card{Faces: []*cards.Face{{Name: "Synthetic", ManaCost: "1 R", Types: []string{"Sorcery"}, Keywords: []string{kw}}}}
	}
	lower := func(c *cards.Card) (*lCard, string) {
		p := &pool{byName: map[string]*cards.Card{"Synthetic": c}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		lc := lowerCard(p, "Synthetic", c, false)
		var got []string
		for _, r := range lc.Refusals {
			got = append(got, r.Reason)
		}
		return lc, strings.Join(got, " | ")
	}
	t.Run("behold a Dragon", func(t *testing.T) {
		lc, refused := lower(face("MayFlashCost:Behold<1/Dragon>"))
		if refused != "" {
			t.Fatalf("refused: %s", refused)
		}
		if lc.MayFlash == nil || lc.MayFlashBehold == nil {
			t.Fatalf("may-flash %v, behold filter %v: want both", lc.MayFlash, lc.MayFlashBehold)
		}
		if got := lc.MayFlash.String(); got != "{0}" && got != "" {
			t.Errorf("may-flash extra mana %q, want none", got)
		}
		var b strings.Builder
		dumpLIR(&b, lc)
		if !strings.Contains(b.String(), "+behold1") {
			t.Errorf("listing %q has no +behold1", b.String())
		}
	})
	for _, c := range []struct{ name, kw, want string }{
		{"two Dragons", "MayFlashCost:Behold<2/Dragon>", `may-flash cost "Behold<2/Dragon>"`},
		{"beside mana", "MayFlashCost:Behold<1/Dragon> 1", `may-flash cost "Behold<1/Dragon> 1"`},
		{"not a creature type", "MayFlashCost:Behold<1/Artifact>", `may-flash cost "Behold<1/Artifact>"`},
	} {
		lc, refused := lower(face(c.kw))
		if !strings.Contains(refused, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, refused, c.want)
		}
		if lc.MayFlashBehold != nil {
			t.Errorf("%s: a refused extra kept its behold filter", c.name)
		}
	}
}
