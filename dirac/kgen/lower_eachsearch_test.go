package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// hasDefinedPlayerPlayer reports whether an ability chain or SVar of the
// card carries DefinedPlayer$ Player.
func hasDefinedPlayerPlayer(c *cards.Card) bool {
	f := c.Faces[0]
	for _, a := range f.Abilities {
		for sa := a; sa != nil; sa = sa.Sub {
			if strings.TrimSpace(sa.Params["DefinedPlayer"]) == "Player" {
				return true
			}
		}
	}
	for _, s := range f.SVars {
		for _, part := range strings.Split(s, "|") {
			if strings.TrimSpace(part) == "DefinedPlayer$ Player" {
				return true
			}
		}
	}
	return false
}

// TestLowerEachPlayerSearch pins the each-player library search (ChangeZone
// DefinedPlayer$ Player, lower_eachsearch.go): Field of Ruin and Veteran
// Explorer lower to a one-instruction controller-first RepeatEach over a
// Search of the loop's player, and Jace, Architect of Thought's -8 (one
// player searching every library with Chooser$) keeps a named refusal.
func TestLowerEachPlayerSearch(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lower := func(t *testing.T, name string) *lCard {
		t.Helper()
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		// Precondition: the script really says DefinedPlayer$ Player.
		if !hasDefinedPlayerPlayer(card) {
			t.Fatalf("%s carries no DefinedPlayer$ Player", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}
	for _, name := range []string{"Field of Ruin", "Veteran Explorer"} {
		t.Run(name, func(t *testing.T) {
			lc := lower(t, name)
			if len(lc.Refusals) != 0 {
				t.Fatalf("%s refused: %v", name, lc.Refusals)
			}
			loops := 0
			for _, ab := range lc.Abilities {
				for i, in := range ab.Prog {
					if in.Op != "RepeatEach" {
						continue
					}
					loops++
					if in.Obj.Kind != rEachPlayer || in.C != 1 || in.Skip != 1 {
						t.Errorf("%s: RepeatEach %v, want EachPlayer C=1 skip=1", name, in)
					}
					if i+1 >= len(ab.Prog) || ab.Prog[i+1].Op != "Search" || ab.Prog[i+1].Obj.Kind != rEach {
						t.Errorf("%s: the RepeatEach is not directly before a Search of Each: %v", name, ab.Prog)
					}
				}
			}
			if loops != 1 {
				t.Fatalf("%s: %d RepeatEach, want 1", name, loops)
			}
		})
	}
	t.Run("Jace, Architect of Thought", func(t *testing.T) {
		lc := lower(t, "Jace, Architect of Thought")
		const want = "DefinedPlayer$ Player with Chooser$"
		n := 0
		for _, r := range lc.Refusals {
			if strings.Contains(r.Reason, want) {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("refusals = %v, want exactly one containing %q", lc.Refusals, want)
		}
	})
}
