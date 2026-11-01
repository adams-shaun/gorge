package policynet

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/view"
)

// TestPipCounts pins the v2 pip slots against known costs, including the
// generic clamp and the hybrid/twobrid faces dirac's encoder shares.
func TestPipCounts(t *testing.T) {
	cases := []struct {
		cost string
		want [6]int
	}{
		{"{2}{W}", [6]int{1, 0, 0, 0, 0, 2}},
		{"{G}{G}", [6]int{0, 0, 0, 0, 2, 0}},
		{"{X}{R}", [6]int{0, 0, 0, 1, 0, 0}},
		{"{U/B}", [6]int{0, 1, 1, 0, 0, 0}},
		{"{2/W}", [6]int{1, 0, 0, 0, 0, 0}},
		{"{6}{W}", [6]int{1, 0, 0, 0, 0, pipMax}},
		{"no cost", [6]int{}},
		{"", [6]int{}},
	}
	for _, tc := range cases {
		if got := pipCounts(tc.cost); got != tc.want {
			t.Errorf("pipCounts(%q) = %v, want %v", tc.cost, got, tc.want)
		}
	}
}

// TestEntityV2BlockSlots pins the raw slots the v2 block writes: colour at
// entColourOff, pip counts at entPipsOff, and the coarse category at
// entCategory. A porting error in any of the three would otherwise only show
// up as a moved aggregate pin.
func TestEntityV2BlockSlots(t *testing.T) {
	v := view.View{
		Viewer:     0,
		Visibility: "seat",
		Players: []view.PlayerView{{
			ID:   0,
			Name: "me",
			Hand: []view.CardView{
				{ID: 1, Name: "Elite Vanguard", Types: "Creature — Human Soldier", ManaCost: "{2}{W}", Power: 2, Toughness: 1},
				{ID: 2, Name: "Grizzly Bears", Types: "Creature — Bear", ManaCost: "{G}{G}", Power: 2, Toughness: 2},
				{ID: 3, Name: "Fireball", Types: "Sorcery", ManaCost: "{X}{R}"},
				{ID: 4, Name: "Forest", Types: "Basic Land — Forest"},
				{ID: 5, Name: "Shock", Types: "Instant", ManaCost: "{R}", SpellAPI: "DealDamage"},
			},
		}},
	}
	st := EncodeStateWith(FeaturesEntity, v, 0, nil)
	if len(st.Cards) != 5 {
		t.Fatalf("card list has %d cards, want 5", len(st.Cards))
	}
	check := func(k int, want map[int]float32) {
		t.Helper()
		raw := st.Cards[k].Raw
		if len(raw) != EntityRawWidth {
			t.Fatalf("card %d raw width %d, want %d", k, len(raw), EntityRawWidth)
		}
		for i := entColourOff; i < EntityRawWidth; i++ {
			w := want[i]
			if raw[i] != w {
				t.Errorf("card %d slot %d = %v, want %v", k, i, raw[i], w)
			}
		}
	}
	// {2}{W} creature: W colour, W pip 1/5, generic 2/5, creature category.
	check(0, map[int]float32{
		entColourOff: 1, entPipsOff: 1.0 / pipMax, entPipsOff + 5: 2.0 / pipMax, entCategory + 1: 1,
	})
	// {G}{G} creature: G colour and two G pips, creature category.
	check(1, map[int]float32{
		entColourOff + 4: 1, entPipsOff + 4: 2.0 / pipMax, entCategory + 1: 1,
	})
	// {X}{R}: R colour and one R pip; X adds nothing.
	check(2, map[int]float32{
		entColourOff + 3: 1, entPipsOff + 3: 1.0 / pipMax,
	})
	// A land has no mana cost and only the ramp category.
	check(3, map[int]float32{entCategory + 2: 1})
	// A removal instant: R colour/pip, removal category (not trick).
	check(4, map[int]float32{
		entColourOff + 3: 1, entPipsOff + 3: 1.0 / pipMax, entCategory: 1,
	})
}
