package chars

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// refAppendAllCreatureTypes is appendAllCreatureTypes as it was before the
// vocabulary bitset: every vocabulary word compared case-insensitively
// against every creature subtype already present. It is the reference the
// bitset form must reproduce word for word.
func refAppendAllCreatureTypes(types []string) []string {
	var had []string
	for _, t := range types {
		if effects.CreatureTypeWords(t) {
			had = append(had, t)
		}
	}
	for _, w := range effects.CreatureTypeWordList() {
		if slices.ContainsFunc(had, func(t string) bool { return strings.EqualFold(t, w) }) {
			continue
		}
		types = append(types, w)
	}
	return types
}

func TestAppendAllCreatureTypesMatchesReference(t *testing.T) {
	vocab := effects.CreatureTypeWordList()
	materialised := append([]string{"Land", "Creature"}, vocab...)
	stripped := slices.DeleteFunc(slices.Clone(materialised), func(w string) bool {
		return w == "Elf" || w == "Zombie" || w == "Advisor"
	})
	reversed := slices.Clone(vocab)
	slices.Reverse(reversed)
	cases := [][]string{
		nil,
		{},
		{"Land"},
		{"Artifact", "Creature", "Thopter"},
		{"Creature", "elf", "ELF", "Elf", "Elf"},
		{"Tribal", "Instant", "Arcane"},
		materialised,
		stripped,
		append(slices.Clone(materialised), vocab...),
		append([]string{"Legendary", "Creature"}, reversed...),
		append([]string{"Enchantment", "Creature", "Goblin"}, vocab[len(vocab)/2:]...),
	}
	for i, c := range cases {
		got := appendAllCreatureTypes(slices.Clone(c))
		want := refAppendAllCreatureTypes(slices.Clone(c))
		if !slices.Equal(got, want) || (got == nil) != (want == nil) {
			t.Fatalf("case %d (%d words): bitset form %v, reference %v", i, len(c), got, want)
		}
	}
}

// stackedAllTypesBoard is Soulstone Sanctuary ("{4}: becomes a 3/3 creature
// with all creature types until end of turn") activated four times in one
// turn -- what an MCTS seat does when the land is its only mana sink: four
// AddAllCreatureTypes effects on one land, which the layer-4 walk orders by
// CR 613.8 dependency simulation.
func stackedAllTypesBoard() (allTypesTestBoard, state.ObjID) {
	card := &cards.Card{Faces: []*cards.Face{{Name: "Stacked Manland", Types: []string{"Land"}}}}
	game := state.NewGame([]string{"A"})
	obj := game.AddObject(card, 0)
	var act []state.ContinuousEffect
	for ts := uint32(1); ts <= 4; ts++ {
		act = append(act, state.ContinuousEffect{Layer: state.LType, Timestamp: ts, AddTypes: []string{"Creature"}, AddAllCreatureTypes: true})
	}
	return allTypesTestBoard{game: game, active: act}, obj.ID
}

// TestStackedAllCreatureTypesGrantsStayCheap pins the 2026-10-13 census
// regression: with the paid-cost mana census on, the mana-ability gate
// re-derives every permanent's types at every priority point, and the walk
// for a land carrying several all-creature-types grants re-expanded the
// ~330-word vocabulary against itself (an EqualFold per word pair) and
// copied it into a fresh match list on every dependency probe -- a 100-sim
// az game went from 3 s to 800 s. The answer must stay the vocabulary once,
// and the walk must stay within a small allocation budget.
func TestStackedAllCreatureTypesGrantsStayCheap(t *testing.T) {
	board, id := stackedAllTypesBoard()
	types, all := TypesAndAllCreatureTypes(board, board.active, id, 0)
	want := append([]string{"Land", "Creature"}, effects.CreatureTypeWordList()...)
	if !all || !slices.Equal(types, want) {
		t.Fatalf("stacked grants derived all=%v %d words, want all=true and Land Creature + the vocabulary once", all, len(types))
	}
	// The first grant materialises the vocabulary into the owned list (one
	// copy and its growth); every later grant and every dependency probe
	// finds it already there. Measured 7
	// (BenchmarkStackedAllCreatureTypesWalk).
	allocs := testing.AllocsPerRun(20, func() {
		TypesAndAllCreatureTypes(board, board.active, id, 0)
	})
	if allocs > 12 {
		t.Fatalf("stacked all-creature-types walk allocates %.0f times, budget 12", allocs)
	}
}

func BenchmarkStackedAllCreatureTypesWalk(b *testing.B) {
	board, id := stackedAllTypesBoard()
	b.ReportAllocs()
	for b.Loop() {
		TypesAndAllCreatureTypes(board, board.active, id, 0)
	}
}
