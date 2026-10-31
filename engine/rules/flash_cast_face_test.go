package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	flashForCreatures = "Name:Creature Herald\nTypes:Artifact\n" +
		"S:Mode$ CastWithFlash | ValidCard$ Creature | ValidSA$ Spell | Caster$ You | Description$ Creature spells have flash.\nOracle:x\n"
	flashForSorceries = "Name:Sorcery Herald\nTypes:Artifact\n" +
		"S:Mode$ CastWithFlash | ValidCard$ Sorcery | ValidSA$ Spell | Caster$ You | Description$ Sorcery spells have flash.\nOracle:x\n"

	creatureAndErrand = "Name:Wandering Giant\nManaCost:0\nTypes:Creature Giant\nPT:2/2\n" +
		"AlternateMode:Adventure\nOracle:front\nALTERNATE\nName:Long Errand\nManaCost:0\nTypes:Sorcery Adventure\n" +
		"A:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw.\nOracle:back\n"
	flashCreatureAndErrand = "Name:Sudden Giant\nManaCost:0\nTypes:Creature Giant\nPT:2/2\nK:Flash\n" +
		"AlternateMode:Adventure\nOracle:front\nALTERNATE\nName:Slow Errand\nManaCost:0\nTypes:Sorcery Adventure\n" +
		"A:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw.\nOracle:back\n"
	creatureAndOmen = "Name:Wandering Drake\nManaCost:0\nTypes:Creature Drake\nPT:2/2\n" +
		"AlternateMode:Omen\nOracle:front\nALTERNATE\nName:Long Omen\nManaCost:0\nTypes:Sorcery Omen\n" +
		"A:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw.\nOracle:back\n"
	creatureAndModalSorcery = "Name:Wandering Ox\nManaCost:0\nTypes:Creature Ox\nPT:2/2\n" +
		"AlternateMode:Modal\nOracle:front\nALTERNATE\nName:Long Furrow\nManaCost:0\nTypes:Sorcery\n" +
		"A:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw.\nOracle:back\n"
)

// CR 715.3a, CR 720.3a: only the characteristics of the face being cast are
// evaluated. An off-sorcery-timing offer reads flash off that face, not off the
// front face the card shows in hand, in both directions.
func TestFlashIsReadOffTheFaceBeingCast(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, src, herald string
		front, back       string
		wantFront         bool
		wantBack          bool
	}{
		{name: "creature grant, Adventure", src: creatureAndErrand, herald: flashForCreatures,
			front: "Wandering Giant", back: "Long Errand", wantFront: true},
		{name: "sorcery grant, Adventure", src: creatureAndErrand, herald: flashForSorceries,
			front: "Wandering Giant", back: "Long Errand", wantBack: true},
		{name: "printed flash, Adventure", src: flashCreatureAndErrand,
			front: "Sudden Giant", back: "Slow Errand", wantFront: true},
		{name: "creature grant, Omen", src: creatureAndOmen, herald: flashForCreatures,
			front: "Wandering Drake", back: "Long Omen", wantFront: true},
		{name: "creature grant, modal", src: creatureAndModalSorcery, herald: flashForCreatures,
			front: "Wandering Ox", back: "Long Furrow", wantFront: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := card(t, tc.src)
			deck := append([]*cards.Card{c}, mountainDeck(t, 40)...)
			e := New(Config{Seed: 1, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, mountainDeck(t, 40)}})
			o := e.G.AddObject(c, 0)
			o.Zone = state.ZHand
			e.G.SetZone(state.ZHand, 0, []state.ObjID{o.ID})
			e.G.Step = state.StepUpkeep
			e.G.Active, e.G.Priority = 0, 0
			if tc.herald != "" {
				src := e.G.AddObject(card(t, tc.herald), 0)
				src.Zone = state.ZBattlefield
				e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})
			}
			offered := func(name string) bool {
				for _, o := range e.legalActions(0) {
					if o.Kind == "cast" && o.Label == "Cast "+name {
						return true
					}
				}
				return false
			}
			if got := offered(tc.front); got != tc.wantFront {
				t.Errorf("%s offered = %v, want %v", tc.front, got, tc.wantFront)
			}
			if got := offered(tc.back); got != tc.wantBack {
				t.Errorf("%s offered = %v, want %v", tc.back, got, tc.wantBack)
			}
		})
	}
}
