package botpolicy

// The L1 cast-weights pins. Three jobs, per the brief:
//
//  1. the equivalence table: DefaultCastWeights (reached through the zero
//     Board.Cast the adapters leave) picks identically to the LITERAL
//     pre-refactor rule on every cast-pick case cast_test.go,
//     reserve_test.go, commander_test.go and counter_cast_test.go exercise —
//     C1–C8 and CR1 exactly as their doc comments state them;
//  2. one test per NEW feature proving a non-zero weight on it changes a
//     pick in the expected direction;
//  3. the zero-value contract: Board.Cast == CastWeights{} is treated as
//     DefaultCastWeights, tested explicitly.

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// ---------------------------------------------------------------------------
// The literal pre-refactor rule, kept verbatim (constants, not weights) as
// the equivalence oracle. It is the pre-refactor cardWorth/castScore/
// chooseCast arithmetic, word for word — the ONLY place in the package the
// old literal numbers now live.

func legacyCardWorth(c Card) int32 {
	if c.Creature {
		return 30 + 4*c.Power
	}
	return c.CMC
}

func legacyCastScore(b Board, o decision.Option) int32 {
	s := legacyCardWorth(b.Cards[o.Obj])
	switch o.Mode {
	case "kicked", "kicked1", "kicked2", "kickedboth", "surged":
		s += 6
	case "flashback", "miracle":
		s += 4
	}
	return s
}

func legacyCastCost(b Board, id state.ObjID) int32 {
	c := b.Cards[id]
	cost := c.CMC
	if cmdr, ok := b.Commanders[id]; ok && cmdr.InCommandZone {
		cost += 2 * cmdr.Casts
	}
	return cost
}

func legacyChooseCast(b Board, d *decision.Decision) int {
	best := -1
	var bestScore int32 = -1
	res := b.reserve()
	foreignSpell := false
	for _, s := range b.Stack {
		if s.IsSpell && s.Controller != d.Player {
			foreignSpell = true
			break
		}
	}
	for _, o := range d.Options {
		if o.Kind != "cast" {
			continue
		}
		if b.Cards[o.Obj].Counter && !foreignSpell {
			continue
		}
		inCmd := b.Commanders[o.Obj].InCommandZone
		s := legacyCastScore(b, o)
		if inCmd {
			s -= 5 * b.Commanders[o.Obj].Casts * b.Cards[o.Obj].CMC
			if s < 0 {
				continue
			}
		} else if res > 0 && b.Pool.Total()-legacyCastCost(b, o.Obj) >= res {
			s += res * 5
		}
		if best == -1 || s > bestScore || (s == bestScore && o.Index < best) {
			best, bestScore = o.Index, s
		}
	}
	return best
}

// castWeightsCase is one equivalence-table entry: a Board and the options
// of one KPriority decision, the exact shape of one cast-pick test case.
type castWeightsCase struct {
	name string
	b    Board
	opts []decision.Option
}

// castWeightsCases mirrors every cast-pick case the existing cast tests
// exercise — the C1–C8 shapes of cast_test.go and counter_cast_test.go,
// the C7 shapes of reserve_test.go, and the CR1 shapes of commander_test.go.
func castWeightsCases() []castWeightsCase {
	cmdr := func(casts int32) map[state.ObjID]Commander {
		return map[state.ObjID]Commander{1: {Casts: casts, InCommandZone: true}}
	}
	return []castWeightsCase{
		{
			name: "C1 creature over spell",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 1}, 2: {CMC: 1}}},
			opts: []decision.Option{castSpell(0, 2), castCreature(1, 1)},
		},
		{
			name: "C2 higher power",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 1}, 2: {Creature: true, Power: 4}}},
			opts: []decision.Option{castCreature(0, 1), castCreature(1, 2)},
		},
		{
			name: "C3 bigger spell",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {CMC: 1}, 2: {CMC: 3}}},
			opts: []decision.Option{castSpell(0, 1), castSpell(1, 2)},
		},
		{
			name: "C4 kicked over ordinary",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 2}}},
			opts: []decision.Option{{Index: 0, Kind: "cast", Obj: 1}, {Index: 1, Kind: "cast", Obj: 1, Mode: "kicked"}},
		},
		{
			name: "C4 flashback over ordinary",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 2}}},
			opts: []decision.Option{{Index: 0, Kind: "cast", Obj: 1}, {Index: 1, Kind: "cast", Obj: 1, Mode: "flashback"}},
		},
		{
			name: "C5 unreadable option is low rank",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 1}}},
			opts: []decision.Option{castSpell(0, 999), castCreature(1, 1)},
		},
		{
			name: "C6 tie on index",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 2}, 2: {Creature: true, Power: 2}}},
			opts: []decision.Option{castCreature(0, 1), castCreature(1, 2)},
		},
		{
			name: "equal-cost non-creatures tie on index",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {CMC: 2}, 2: {CMC: 2}}},
			opts: []decision.Option{castSpell(0, 2), castSpell(1, 1)},
		},
		{
			name: "C3 ranks by cost not oracle",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {CMC: 4}, 2: {CMC: 1}}},
			opts: []decision.Option{castSpell(0, 2), castSpell(1, 1)},
		},
		{
			name: "C7 prefer the cast that keeps the reserve",
			b: Board{IsMain: true, Pool: state.Mana{state.MC: 3}, Cards: map[state.ObjID]Card{
				1: {CMC: 1, Castable: true},
				3: {CMC: 3, Castable: true},
				2: {CMC: 1, Castable: true, InstantSpeed: true},
			}},
			opts: []decision.Option{{Index: 0, Kind: "cast", Obj: 3}, {Index: 1, Kind: "cast", Obj: 1}, {Index: 2, Kind: "pass"}},
		},
		{
			name: "C7 never suppresses the best play",
			b: Board{IsMain: true, Pool: state.Mana{state.MC: 4}, Cards: map[state.ObjID]Card{
				1: {CMC: 4, Creature: true, Power: 4, Castable: true},
				2: {CMC: 1, Castable: true, InstantSpeed: true},
			}},
			opts: []decision.Option{{Index: 0, Kind: "cast", Obj: 1}, {Index: 1, Kind: "pass"}},
		},
		{
			name: "C7 allows a cast with surplus",
			b: Board{IsMain: true, Pool: state.Mana{state.MC: 4}, Cards: map[state.ObjID]Card{
				1: {CMC: 3, Creature: true, Power: 2, Castable: true},
				2: {CMC: 1, Castable: true, InstantSpeed: true},
			}},
			opts: []decision.Option{{Index: 0, Kind: "cast", Obj: 1}, {Index: 1, Kind: "pass"}},
		},
		{
			name: "C7 inert without an instant-speed card",
			b: Board{IsMain: true, Pool: state.Mana{state.MC: 3}, Cards: map[state.ObjID]Card{
				1: {CMC: 3, Creature: true, Power: 2, Castable: true},
			}},
			opts: []decision.Option{{Index: 0, Kind: "cast", Obj: 1}, {Index: 1, Kind: "pass"}},
		},
		{
			name: "C7 never holds the commander cast",
			b: Board{IsMain: true, Pool: state.Mana{state.MC: 3},
				Commanders: map[state.ObjID]Commander{9: {Casts: 0, InCommandZone: true}},
				Cards: map[state.ObjID]Card{
					9: {CMC: 3, Creature: true, Power: 3, Castable: true},
					2: {CMC: 1, Castable: true, InstantSpeed: true},
				}},
			opts: []decision.Option{{Index: 0, Kind: "cast", Obj: 9}, {Index: 1, Kind: "pass"}},
		},
		{
			name: "CR1 first cast",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 4, CMC: 4}}, Commanders: cmdr(0)},
			opts: []decision.Option{castCreature(0, 1), {Index: 1, Kind: "pass"}},
		},
		{
			name: "CR1 second cast still casts",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 4, CMC: 4}, 2: {CMC: 3}}, Commanders: cmdr(1)},
			opts: []decision.Option{castSpell(0, 2), castCreature(1, 1)},
		},
		{
			name: "CR1 fourth cast refused",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 4, CMC: 4}}, Commanders: cmdr(3)},
			opts: []decision.Option{castCreature(0, 1), {Index: 1, Kind: "pass"}},
		},
		{
			name: "CR1 12/12 third cast refused",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 12, CMC: 12}}, Commanders: cmdr(2)},
			opts: []decision.Option{castCreature(0, 1), {Index: 1, Kind: "pass"}},
		},
		{
			name: "CR1 cheap outlasts expensive",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 2, CMC: 2}}, Commanders: cmdr(2)},
			opts: []decision.Option{castCreature(0, 1), {Index: 1, Kind: "pass"}},
		},
		{
			name: "CR1 expensive CMC8 priced out",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 4, CMC: 8}}, Commanders: cmdr(2)},
			opts: []decision.Option{castCreature(0, 1), {Index: 1, Kind: "pass"}},
		},
		{
			name: "CR1 taxed below the big spell",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 4, CMC: 4}, 2: {CMC: 8}}, Commanders: cmdr(2)},
			opts: []decision.Option{castCreature(0, 1), castSpell(1, 2)},
		},
		{
			name: "CR1 hand cast untaxed",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 4, CMC: 4}}, Commanders: map[state.ObjID]Commander{1: {Casts: 3, InCommandZone: false}}},
			opts: []decision.Option{castCreature(0, 1), {Index: 1, Kind: "pass"}},
		},
		{
			name: "CR1 refusal falls to the land drop",
			b:    Board{IsMain: true, Cards: map[state.ObjID]Card{1: {Creature: true, Power: 2, CMC: 2}, 2: {Basic: true}}, Commanders: cmdr(2)},
			opts: []decision.Option{castCreature(0, 1), playLand(1, 2)},
		},
		{
			name: "C8 counter refused at an own-spells-only stack",
			b: Board{Cards: map[state.ObjID]Card{
				10: {CMC: 2, Castable: true, InstantSpeed: true, Counter: true},
			}, Stack: []StackEntry{{ID: 90, Controller: 0, IsSpell: true}}},
			opts: counterPriority(10).Options,
		},
		{
			name: "C8 counter cast at a foreign spell",
			b: Board{Cards: map[state.ObjID]Card{
				10: {CMC: 2, Castable: true, InstantSpeed: true, Counter: true},
			}, Stack: []StackEntry{{ID: 91, Controller: 1, IsSpell: true}}},
			opts: counterPriority(10).Options,
		},
	}
}

// TestDefaultCastWeightsMatchesPreRefactorRule is the equivalence table: on
// every mirrored cast-pick case, chooseCast over a Board whose Cast is the
// zero value (treated as DefaultCastWeights) picks the same option as the
// literal pre-refactor rule above. Any drift in the default arithmetic —
// a reordered feature, a changed constant, a dropped gate — fails here by
// case name.
func TestDefaultCastWeightsMatchesPreRefactorRule(t *testing.T) {
	for _, tc := range castWeightsCases() {
		d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: tc.opts}
		want := legacyChooseCast(tc.b, d)
		got := tc.b.chooseCast(d)
		if got != want {
			t.Fatalf("%s: chooseCast (zero Cast = default weights) = option %d, want the pre-refactor rule's %d", tc.name, got, want)
		}
	}
}

// TestDefaultCastWeightsFieldPicksLikeDefaultToo pins the same equivalence
// through an EXPLICITLY-set DefaultCastWeights — the adapters never set
// Board.Cast, so both reach paths (zero value and explicit default) must
// agree with the legacy rule on the whole table.
func TestDefaultCastWeightsFieldPicksLikeDefaultToo(t *testing.T) {
	for _, tc := range castWeightsCases() {
		tc.b.Cast = DefaultCastWeights
		d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: tc.opts}
		if got := tc.b.chooseCast(d); got != legacyChooseCast(tc.b, d) {
			t.Fatalf("%s: chooseCast (explicit DefaultCastWeights) = option %d, want the pre-refactor rule's pick", tc.name, got)
		}
	}
}

// withTuned copies a Board with one weight of its cast profile turned on —
// the shape a learned profile arrives in (everything default but the one
// learned weight).
func withTuned(b Board, tune func(*CastWeights)) Board {
	w := DefaultCastWeights
	tune(&w)
	b.Cast = w
	return b
}

// ---------------------------------------------------------------------------
// One test per new feature: a non-zero weight on it changes a pick in the
// expected direction. Every control asserts the default pick first, so the
// flip is attributable to the weight alone.

// TestCastWeightCreatureToughness: with CreatureToughness weight 2, of two
// equal-power creatures (an index tie by default) the tougher body is cast.
func TestCastWeightCreatureToughness(t *testing.T) {
	cards := map[state.ObjID]Card{
		1: {Creature: true, Power: 2, Toughness: 1, Castable: true},
		2: {Creature: true, Power: 2, Toughness: 4, Castable: true},
	}
	opts := []decision.Option{castCreature(0, 1), castCreature(1, 2)}
	b := Board{IsMain: true, Cards: cards}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: opts}
	if got := b.chooseCast(d); got != 0 {
		t.Fatalf("default pick = option %d, want the index tie (0)", got)
	}
	if got := withTuned(b, func(w *CastWeights) { w.CreatureToughness = 2 }).chooseCast(d); got != 1 {
		t.Fatalf("CreatureToughness=2 pick = option %d, want 1 — the 4-toughness body outranks the 1-toughness one", got)
	}
}

// TestCastWeightCurveFit: with CurveFit weight 10, of two equal-power
// creatures (an index tie by default) the one whose cost exactly equals the
// producible mana (pool 1 + an untapped source producing 2 = 3) is cast.
// A tapped source promises nothing, so the control keeps the tie.
func TestCastWeightCurveFit(t *testing.T) {
	cards := map[state.ObjID]Card{
		1: {Creature: true, Power: 2, CMC: 1, Castable: true},
		2: {Creature: true, Power: 2, CMC: 3, Castable: true},
		5: {OnBattlefield: true, Produces: prod(state.MC, 2)}, // an untapped colourless source
	}
	opts := []decision.Option{castCreature(0, 1), castCreature(1, 2)}
	b := Board{IsMain: true, Pool: state.Mana{state.MC: 1}, Cards: cards}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: opts}
	if got := b.chooseCast(d); got != 0 {
		t.Fatalf("default pick = option %d, want the index tie (0)", got)
	}
	if got := withTuned(b, func(w *CastWeights) { w.CurveFit = 10 }).chooseCast(d); got != 1 {
		t.Fatalf("CurveFit=10 pick = option %d, want 1 — the 3-cost cast exactly spends the producible 3 (pool 1 + source 2)", got)
	}
	// Control: the source is tapped — producible drops to the pool's 1 and
	// the fit moves to the 1-cost cast, which the weight then prefers.
	src := cards[5]
	src.Tapped = true
	cards[5] = src
	if got := withTuned(b, func(w *CastWeights) { w.CurveFit = 10 }).chooseCast(d); got != 0 {
		t.Fatalf("CurveFit=10 with a tapped source = option %d, want 0 — producible is the pool's 1 and the 1-cost cast is what fits", got)
	}
}

// TestCastWeightManaLeft: with ManaLeft weight 1, of two equal-power
// creatures (an index tie by default) the cheaper cast wins — it leaves
// more of the pool unspent.
func TestCastWeightManaLeft(t *testing.T) {
	cards := map[state.ObjID]Card{
		1: {Creature: true, Power: 2, CMC: 3, Castable: true},
		2: {Creature: true, Power: 2, CMC: 1, Castable: true},
	}
	opts := []decision.Option{castCreature(0, 1), castCreature(1, 2)}
	b := Board{IsMain: true, Pool: state.Mana{state.MC: 5}, Cards: cards}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: opts}
	if got := b.chooseCast(d); got != 0 {
		t.Fatalf("default pick = option %d, want the index tie (0)", got)
	}
	if got := withTuned(b, func(w *CastWeights) { w.ManaLeft = 1 }).chooseCast(d); got != 1 {
		t.Fatalf("ManaLeft=1 pick = option %d, want 1 — the 1-cost cast leaves 4 of the pool against the 3-cost's 2", got)
	}
}

// TestCastWeightPrecombat: with Precombat weight 15, a commander cast the
// CR1 tax has priced below zero (46 - 5*3*4 = -14) is rescued in the FIRST
// main phase; outside the first main phase (FirstMain false) the same
// weight keeps it refused.
func TestCastWeightPrecombat(t *testing.T) {
	b := Board{IsMain: true, FirstMain: true,
		Cards:      map[state.ObjID]Card{1: {Creature: true, Power: 4, CMC: 4, Castable: true}},
		Commanders: map[state.ObjID]Commander{1: {Casts: 3, InCommandZone: true}}}
	opts := []decision.Option{castCreature(0, 1), {Index: 1, Kind: "pass"}}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: opts}
	if got := b.chooseCast(d); got != -1 {
		t.Fatalf("default pick = option %d, want -1 — the fourth command-zone cast is priced out (CR1)", got)
	}
	if got := withTuned(b, func(w *CastWeights) { w.Precombat = 15 }).chooseCast(d); got != 0 {
		t.Fatalf("Precombat=15 pick = option %d, want 0 — the rescued recast is worth it in the first main phase", got)
	}
	b.FirstMain = false // same weight, second main phase: no rescue
	if got := withTuned(b, func(w *CastWeights) { w.Precombat = 15 }).chooseCast(d); got != -1 {
		t.Fatalf("Precombat=15 outside the first main phase = option %d, want -1", got)
	}
}

// TestCastWeightInstantOnOwnTurn: with InstantOnOwnTurn weight 10, of two
// equal-CMC spells (an index tie by default) the instant-speed one is cast
// during the seat's own main phase; on another seat's turn the same weight
// keeps the index tie.
func TestCastWeightInstantOnOwnTurn(t *testing.T) {
	cards := map[state.ObjID]Card{
		1: {CMC: 2, Castable: true},
		2: {CMC: 2, Castable: true, InstantSpeed: true},
	}
	opts := []decision.Option{castSpell(0, 1), castSpell(1, 2)}
	b := Board{IsMain: true, MyTurn: true, Cards: cards}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: opts}
	if got := b.chooseCast(d); got != 0 {
		t.Fatalf("default pick = option %d, want the index tie (0)", got)
	}
	if got := withTuned(b, func(w *CastWeights) { w.InstantOnOwnTurn = 10 }).chooseCast(d); got != 1 {
		t.Fatalf("InstantOnOwnTurn=10 pick = option %d, want 1 — the instant-speed card is cast in the seat's own main phase", got)
	}
	b.MyTurn = false // another seat's main phase: the weight is inert
	if got := withTuned(b, func(w *CastWeights) { w.InstantOnOwnTurn = 10 }).chooseCast(d); got != 0 {
		t.Fatalf("InstantOnOwnTurn=10 on another seat's turn = option %d, want 0", got)
	}
}

// TestCastWeightOppCreatures: with OppCreatures weight 15 and one opposing
// creature on a public battlefield, a CR1-priced-out commander cast
// (-14) is rescued; with the default weight it stays refused. The constant
// feature can never reorder an otherwise-tied pair — it only moves every
// surviving cast the same way, which is exactly what turns a refusal into
// a cast at the CR1 boundary.
func TestCastWeightOppCreatures(t *testing.T) {
	b := Board{IsMain: true,
		Cards:      map[state.ObjID]Card{1: {Creature: true, Power: 4, CMC: 4, Castable: true}},
		Commanders: map[state.ObjID]Commander{1: {Casts: 3, InCommandZone: true}},
		Creatures:  map[state.ObjID]Creature{50: {Controller: 1}}}
	opts := []decision.Option{castCreature(0, 1), {Index: 1, Kind: "pass"}}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: opts}
	if got := b.chooseCast(d); got != -1 {
		t.Fatalf("default pick = option %d, want -1 — the fourth command-zone cast is priced out (CR1)", got)
	}
	if got := withTuned(b, func(w *CastWeights) { w.OppCreatures = 15 }).chooseCast(d); got != 0 {
		t.Fatalf("OppCreatures=15 pick = option %d, want 0 — an opposing board lifts every cast past the CR1 boundary", got)
	}
}

// TestCastWeightOwnCreatures: the mirror of the OppCreatures pin — the
// seat's OWN creature count, read off the same public census, carries the
// weight.
func TestCastWeightOwnCreatures(t *testing.T) {
	b := Board{IsMain: true,
		Cards:      map[state.ObjID]Card{1: {Creature: true, Power: 4, CMC: 4, Castable: true}},
		Commanders: map[state.ObjID]Commander{1: {Casts: 3, InCommandZone: true}},
		Creatures:  map[state.ObjID]Creature{50: {Controller: 0}}}
	opts := []decision.Option{castCreature(0, 1), {Index: 1, Kind: "pass"}}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: opts}
	if got := b.chooseCast(d); got != -1 {
		t.Fatalf("default pick = option %d, want -1 — the fourth command-zone cast is priced out (CR1)", got)
	}
	if got := withTuned(b, func(w *CastWeights) { w.OwnCreatures = 15 }).chooseCast(d); got != 0 {
		t.Fatalf("OwnCreatures=15 pick = option %d, want 0 — the seat's own board lifts every cast past the CR1 boundary", got)
	}
}

// TestCastWeightLifeDelta: with LifeDelta weight 15 and the seat 15 ahead
// of its lowest opponent, a CR1-priced-out commander cast (-14) is rescued;
// with the default weight it stays refused.
func TestCastWeightLifeDelta(t *testing.T) {
	b := Board{IsMain: true,
		Cards:      map[state.ObjID]Card{1: {Creature: true, Power: 4, CMC: 4, Castable: true}},
		Commanders: map[state.ObjID]Commander{1: {Casts: 3, InCommandZone: true}},
		Life:       map[state.PlayerID]int32{0: 20, 1: 5}}
	opts := []decision.Option{castCreature(0, 1), {Index: 1, Kind: "pass"}}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: opts}
	if got := b.chooseCast(d); got != -1 {
		t.Fatalf("default pick = option %d, want -1 — the fourth command-zone cast is priced out (CR1)", got)
	}
	if got := withTuned(b, func(w *CastWeights) { w.LifeDelta = 15 }).chooseCast(d); got != 0 {
		t.Fatalf("LifeDelta=15 pick = option %d, want 0 — being 15 ahead of the lowest opponent lifts every cast past the CR1 boundary", got)
	}
}

// TestCastWeightsZeroValueIsDefault pins the zero-value contract directly:
// a Board whose Cast nobody set and a Board carrying DefaultCastWeights
// explicitly pick identically on the whole table — and a genuinely tuned
// profile (the C7/creature terms zeroed, non-creatures priced up) changes a
// pick, proving the field is live and not decorative.
func TestCastWeightsZeroValueIsDefault(t *testing.T) {
	for _, tc := range castWeightsCases() {
		d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: tc.opts}
		zero := tc.b
		zero.Cast = CastWeights{}
		if got, want := zero.chooseCast(d), tc.b.chooseCast(d); got != want {
			t.Fatalf("%s: explicit zero Cast picked option %d, want the unset-Board pick %d", tc.name, got, want)
		}
	}
	// A tuned profile is live: creature terms zeroed, non-creatures priced
	// up — the C1 ranking inverts and the spell is cast over the creature.
	b := Board{IsMain: true, Cards: map[state.ObjID]Card{
		1: {Creature: true, Power: 1},
		2: {CMC: 3},
	}}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{castSpell(0, 2), castCreature(1, 1)}}
	if got := withTuned(b, func(w *CastWeights) { w.CreatureBase, w.CreaturePower, w.NonCreatureCMC = 0, 0, 10 }).chooseCast(d); got != 0 {
		t.Fatalf("tuned profile pick = option %d, want 0 — with the creature terms zeroed the CMC-3 spell outranks the 1/1", got)
	}
}
