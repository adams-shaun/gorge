package botpolicy_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

type printedChars struct{}

func (printedChars) Power(state.ObjID) int32       { return 0 }
func (printedChars) Toughness(state.ObjID) int32   { return 0 }
func (printedChars) Keywords(state.ObjID) []string { return nil }

// TestRemovalKindAdaptersAgree pins the A1 fact both halves must fill: the
// game-shaped BoardFromGame and the view-shaped BoardFromView classify the
// same printed spell as the same one-shot removal kind. A missing fill on
// either half makes the rule read a different board depending on the host.
func TestRemovalKindAdaptersAgree(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "destroy",
			src:  "Name:Doom Blade\nManaCost:1 B\nTypes:Instant\nOracle:Destroy target creature.\nA:SP$ Destroy | ValidTgts$ Creature | SpellDescription$ Destroy target creature.\n",
			want: "destroy",
		},
		{
			name: "damage",
			src:  "Name:Shock\nManaCost:R\nTypes:Instant\nOracle:Deal 2 damage to any target.\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 2 | SpellDescription$ Deal 2 damage to any target.\n",
			want: "damage",
		},
		{
			name: "bounce",
			src:  "Name:Unsummon\nManaCost:U\nTypes:Instant\nOracle:Return target creature to its owner's hand.\nA:SP$ ChangeZone | ValidTgts$ Creature | Origin$ Battlefield | Destination$ Hand | SpellDescription$ Return target creature to its owner's hand.\n",
			want: "bounce",
		},
		{
			name: "graveyard return is not bounce",
			src:  "Name:Raise Dead\nManaCost:B\nTypes:Sorcery\nOracle:Return target creature card from your graveyard to your hand.\nA:SP$ ChangeZone | ValidTgts$ Creature.YouOwn | Origin$ Graveyard | Destination$ Hand | SpellDescription$ Return target creature card from your graveyard to your hand.\n",
			want: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card, diags := cards.ParseBytes("removalnow_adapter_test.go", []byte(tc.src))
			if len(diags) != 0 {
				t.Fatalf("parse: %v", diags)
			}
			card.Link()
			g := state.NewGame([]string{"a", "b"})
			o := g.AddObject(card, 0)
			g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})

			face := card.Faces[0]
			v := view.View{Viewer: 0, Players: []view.PlayerView{{
				ID: 0,
				Hand: []view.CardView{{
					ID:             o.ID,
					Name:           face.Name,
					Types:          strings.Join(face.Types, " "),
					ManaCost:       face.ManaCost,
					SpellAPI:       face.SpellAbility().API,
					RemovalKind:    tc.want,
					ColourIdentity: cards.ColourMask(face.ColourIdentity()),
				}},
			}}}

			gameKind := botpolicy.BoardFromGame(g, printedChars{}, 0).Cards.Get(o.ID).RemovalKind
			viewKind := heuristic.BoardFromView(v).Cards.Get(o.ID).RemovalKind
			if gameKind != tc.want || viewKind != tc.want {
				t.Fatalf("RemovalKind game=%q view=%q, want %q", gameKind, viewKind, tc.want)
			}
		})
	}
}
