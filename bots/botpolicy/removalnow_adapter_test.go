package botpolicy_test

import (
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

// TestRemovalKindAdaptersAgree pins the A1 facts both halves must fill: the
// game-shaped BoardFromGame and the view-shaped BoardFromView classify the
// same printed spell as the same one-shot removal kind, scope and literal
// damage. The view half is built through view.Project -- the REAL
// projection a client receives, engine/view/card.go's spell-ability
// projection -- so a reverted projection line fails here, not only in the
// whole-game parity tests. A missing fill on either half makes the rule
// read a different board depending on the host.
func TestRemovalKindAdaptersAgree(t *testing.T) {
	for _, tc := range []struct {
		name    string
		src     string
		kind    string
		targets string
		damage  int32
	}{
		{
			name:    "destroy",
			src:     "Name:Doom Blade\nManaCost:1 B\nTypes:Instant\nOracle:Destroy target creature.\nA:SP$ Destroy | ValidTgts$ Creature | SpellDescription$ Destroy target creature.\n",
			kind:    "destroy",
			targets: "creature",
		},
		{
			name:    "damage",
			src:     "Name:Shock\nManaCost:R\nTypes:Instant\nOracle:Deal 2 damage to any target.\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 2 | SpellDescription$ Deal 2 damage to any target.\n",
			kind:    "damage",
			targets: "any",
			damage:  2,
		},
		{
			name:    "artifact destroy has no creature scope",
			src:     "Name:Shatter\nManaCost:1 R\nTypes:Instant\nOracle:Destroy target artifact.\nA:SP$ Destroy | ValidTgts$ Artifact | SpellDescription$ Destroy target artifact.\n",
			kind:    "destroy",
			targets: "",
		},
		{
			name:    "bounce",
			src:     "Name:Unsummon\nManaCost:U\nTypes:Instant\nOracle:Return target creature to its owner's hand.\nA:SP$ ChangeZone | ValidTgts$ Creature | Origin$ Battlefield | Destination$ Hand | SpellDescription$ Return target creature to its owner's hand.\n",
			kind:    "bounce",
			targets: "creature",
		},
		{
			name: "graveyard return is not bounce",
			src:  "Name:Raise Dead\nManaCost:B\nTypes:Sorcery\nOracle:Return target creature card from your graveyard to your hand.\nA:SP$ ChangeZone | ValidTgts$ Creature.YouOwn | Origin$ Graveyard | Destination$ Hand | SpellDescription$ Return target creature card from your graveyard to your hand.\n",
			kind: "",
		},
		{
			name: "X damage is an unknown figure",
			src:  "Name:Fireball\nManaCost:X R\nTypes:Instant\nOracle:Fireball deals X damage divided as you choose.\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ X | SpellDescription$ Fireball deals X damage.\n",
			kind: "damage",
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

			// The view half through the real projection: the spell-ability
			// facts (SpellAPI, RemovalKind, RemovalTargets, RemovalDamage)
			// come off the printed face inside view.Project, not from a
			// hand-fed CardView. The chars read is nil, which Project
			// documents as a degrade (zero power/toughness); the asserted
			// facts are printed-face facts and never read it.
			v := view.Project(g, nil, 0, nil)
			var cv *view.CardView
			for i := range v.Players[0].Hand {
				if v.Players[0].Hand[i].ID == o.ID {
					cv = &v.Players[0].Hand[i]
				}
			}
			if cv == nil {
				t.Fatalf("projection: hand card %d missing", o.ID)
			}
			if cv.RemovalKind != tc.kind || cv.RemovalTargets != tc.targets || cv.RemovalDamage != tc.damage {
				t.Fatalf("projected RemovalKind=%q RemovalTargets=%q RemovalDamage=%d, want %q %q %d",
					cv.RemovalKind, cv.RemovalTargets, cv.RemovalDamage, tc.kind, tc.targets, tc.damage)
			}
			gameKind := botpolicy.BoardFromGame(g, printedChars{}, 0).Cards.Get(o.ID).RemovalKind
			facts := heuristic.BoardFromView(v).Cards.Get(o.ID)
			if gameKind != tc.kind || facts.RemovalKind != tc.kind {
				t.Fatalf("RemovalKind game=%q view=%q, want %q", gameKind, facts.RemovalKind, tc.kind)
			}
			if facts.RemovalTargets != tc.targets || facts.RemovalDamage != tc.damage {
				t.Fatalf("view Board facts RemovalTargets=%q RemovalDamage=%d, want %q %d",
					facts.RemovalTargets, facts.RemovalDamage, tc.targets, tc.damage)
			}
		})
	}
}
