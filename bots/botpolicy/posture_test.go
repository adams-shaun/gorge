package botpolicy

import (
	"fmt"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/state"
)

// postureManifest builds the manifest ShapeOf reads: one land row at curve
// bucket 0 plus one nonland row and matching curve row per {cmc, count}.
func postureManifest(lands int, rows ...[2]int) *deck.Manifest {
	m := &deck.Manifest{
		Main:  []deck.ManifestRow{{Name: "Land", Count: lands, Land: true}},
		Curve: []deck.CurveRow{{CMC: 0, Count: lands}},
	}
	for i, row := range rows {
		m.Main = append(m.Main, deck.ManifestRow{Name: fmt.Sprintf("Card%d", i), Count: row[1]})
		m.Curve = append(m.Curve, deck.CurveRow{CMC: int32(row[0]), Count: row[1]})
	}
	return m
}

var (
	aggroDeck    = postureManifest(24, [2]int{1, 18}, [2]int{2, 18})
	midrangeDeck = postureManifest(24, [2]int{2, 18}, [2]int{3, 18})
	controlDeck  = postureManifest(26, [2]int{2, 10}, [2]int{4, 12}, [2]int{5, 12})
)

func TestShapeOfThresholds(t *testing.T) {
	mainOnly := &deck.Manifest{Main: []deck.ManifestRow{{Name: "Land", Count: 60, Land: true}}}
	inconsistent := postureManifest(24, [2]int{2, 36})
	inconsistent.Curve[1].Count = 35
	for _, tc := range []struct {
		name string
		m    *deck.Manifest
		want DeckShape
	}{
		{"nil", nil, ShapeMidrange},
		{"main only", mainOnly, ShapeMidrange},
		{"lands only", postureManifest(60), ShapeMidrange},
		{"curve total mismatch", inconsistent, ShapeMidrange},
		{"aggro", aggroDeck, ShapeAggro},
		{"aggro boundary 225", postureManifest(24, [2]int{2, 27}, [2]int{3, 9}), ShapeAggro},
		{"midrange", midrangeDeck, ShapeMidrange},
		{"midrange 300", postureManifest(24, [2]int{3, 36}), ShapeMidrange},
		{"control boundary 302", postureManifest(26, [2]int{3, 33}, [2]int{4, 1}), ShapeControl},
		{"control", controlDeck, ShapeControl},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShapeOf(tc.m); got != tc.want {
				t.Fatalf("ShapeOf = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPostureAggroReleasesHeldBlocker(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    *deck.Manifest
		post PostureRules
		want []int
	}{
		{"aggro unarmed", aggroDeck, PostureRules{}, []int{}},
		{"aggro armed", aggroDeck, PostureRules{Attack: true}, []int{0}},
		{"midrange armed", midrangeDeck, PostureRules{Attack: true}, []int{}},
		{"control armed", controlDeck, PostureRules{Attack: true}, []int{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := boardOf(atk(1, 3, 3), def(1, 2, 2))
			b.Life.Set(0, 20)
			b.Life.Set(1, 20)
			b.OwnDeck = tc.m
			b.Posture = tc.post
			if got := attackDecision(b, 1); !slices.Equal(got, tc.want) {
				t.Fatalf("attackers = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPostureAggroHoldsAtLethalCrackBack(t *testing.T) {
	for _, tc := range []struct {
		life int32
		want []int
	}{
		{2, []int{}},
		{3, []int{0}},
	} {
		t.Run(fmt.Sprintf("life %d", tc.life), func(t *testing.T) {
			b := boardOf(atk(1, 3, 3), def(1, 2, 2))
			b.Life.Set(0, tc.life)
			b.Life.Set(1, 20)
			b.OwnDeck = aggroDeck
			b.Posture = PostureRules{Attack: true}
			if got := attackDecision(b, 1); !slices.Equal(got, tc.want) {
				t.Fatalf("attackers = %v, want %v", got, tc.want)
			}
		})
	}
}

func postureSim(t *testing.T, m *deck.Manifest, ourLife int32, attack bool, want []int) {
	t.Helper()
	b := boardOf(atk(1, 1, 1), atk(2, 2, 2), def(1, 2, 1), def(2, 2, 1))
	b.Life.Set(0, ourLife)
	b.Life.Set(1, 10)
	b.OwnDeck = m
	b.Posture = PostureRules{Attack: attack}
	d, s := simAttack(b, 1, 2)
	if !slices.Equal(d, []int{0}) {
		t.Fatalf("default attacker = %v, want [0] (the premise)", d)
	}
	if !slices.Equal(s, want) {
		t.Fatalf("attack-sim = %v, want %v", s, want)
	}
}

func TestPostureSimAggroRaces(t *testing.T) {
	postureSim(t, aggroDeck, 11, false, []int{})
	postureSim(t, aggroDeck, 11, true, []int{0})
}

func TestPostureSimControlHoldsBack(t *testing.T) {
	postureSim(t, controlDeck, 13, false, []int{0})
	postureSim(t, controlDeck, 13, true, []int{})
}

func TestPostureSimMidrangeInert(t *testing.T) {
	postureSim(t, midrangeDeck, 11, false, []int{})
	postureSim(t, midrangeDeck, 11, true, []int{})
	postureSim(t, midrangeDeck, 13, false, []int{0})
	postureSim(t, midrangeDeck, 13, true, []int{0})
}

func postureReserve(t *testing.T, power int32, m *deck.Manifest, reserve bool, want int) {
	t.Helper()
	b := priorityCards(map[state.ObjID]Card{
		1: {Creature: true, Power: power, CMC: 3, Castable: true},
		2: {Creature: true, Power: 1, CMC: 1, Castable: true},
		3: {CMC: 2, Castable: true, InstantSpeed: true},
	})
	b.Pool = state.Mana{state.MC: 3}
	b.OwnDeck = m
	b.Posture = PostureRules{Reserve: reserve}
	opts := []decision.Option{
		{Index: 0, Kind: "cast", Obj: 1},
		{Index: 1, Kind: "cast", Obj: 2},
		{Index: 2, Kind: "pass"},
	}
	if got, _ := castDecision(b, opts); got != want {
		t.Fatalf("cast = %d, want %d", got, want)
	}
}

func TestPostureReserveAggroSpends(t *testing.T) {
	postureReserve(t, 3, aggroDeck, false, 1)
	postureReserve(t, 3, aggroDeck, true, 0)
}

func TestPostureReserveControlKeeps(t *testing.T) {
	postureReserve(t, 4, controlDeck, false, 0)
	postureReserve(t, 4, controlDeck, true, 1)
}

func TestPostureReserveMidrangeInert(t *testing.T) {
	postureReserve(t, 3, midrangeDeck, false, 1)
	postureReserve(t, 3, midrangeDeck, true, 1)
	postureReserve(t, 4, midrangeDeck, false, 0)
	postureReserve(t, 4, midrangeDeck, true, 0)
}
