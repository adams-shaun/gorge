package rules

import (
	"fmt"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	startDFC = "Name:Front Half\nManaCost:1\nTypes:Creature Human\nPT:1/1\nAlternateMode:DoubleFaced\nOracle:x\n\n" +
		"ALTERNATE\n\nName:Back Half\nTypes:Creature Werewolf\nPT:3/3\nOracle:x\n"
	startAura = "Name:Start Aura\nManaCost:W\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"A:SP$ Attach | Cost$ W | ValidTgts$ Creature | AILogic$ Pump\nOracle:x\n"
	// startETB draws a card when it enters: the trigger a scripted start drops.
	startETB = "Name:Start ETB\nManaCost:1\nTypes:Creature Elf\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDraw | TriggerDescription$ x\n" +
		"SVar:TrigDraw:DB$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n"
)

// startDeck is 40 distinct cards "C0".."C39" so a deck index names one object;
// special swaps a slot for a scripted card.
func startDeck(t *testing.T, tag string, special map[int]string) []*cards.Card {
	t.Helper()
	deck := make([]*cards.Card, 40)
	for i := range deck {
		src := fmt.Sprintf("Name:%s%d\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", tag, i)
		if i%3 == 0 {
			src = fmt.Sprintf("Name:%s%d\nTypes:Basic Land Forest\nOracle:x\n", tag, i)
		}
		if s, ok := special[i]; ok {
			src = s
		}
		deck[i] = card(t, src)
	}
	return deck
}

func startConfig(t *testing.T, seed uint64, start *StartSpec, special [2]map[int]string) Config {
	t.Helper()
	return Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{startDeck(t, "A", special[0]), startDeck(t, "B", special[1])},
		Start: start}
}

// deckIndex maps an object to its seat-deck index (objects are minted in deck
// order, seat by seat).
func deckIndex(e *Engine, id state.ObjID) int {
	o := e.G.Obj(id)
	return int(id-1) - 40*int(o.Owner)
}

func zoneIdx(e *Engine, z state.Zone, p int) []int {
	var out []int
	for _, id := range e.G.Zone(z, state.PlayerID(p)) {
		out = append(out, deckIndex(e, id))
	}
	return out
}

func TestScriptedStartNilAndEmptyAreByteIdentical(t *testing.T) {
	base := New(startConfig(t, 7, nil, [2]map[int]string{}))
	empty := New(startConfig(t, 7, &StartSpec{}, [2]map[int]string{}))
	if base.L.Head() != empty.L.Head() || len(base.L.Events) != len(empty.L.Events) {
		t.Fatalf("an empty StartSpec moved the chain: %d vs %d events", len(base.L.Events), len(empty.L.Events))
	}
}

func TestScriptedStartPlacement(t *testing.T) {
	life := int32(13)
	start := &StartSpec{Seats: []StartSeat{{
		Hand:        []int{5, 6, 7},
		Battlefield: []int{1, 2, 4},
		Graveyard:   []int{10, 11},
		Exile:       []int{12},
		Library:     []int{20, 21, 22},
		Tapped:      []int{2},
		Counters:    []StartCounter{{Card: 1, Kind: "P1P1", N: 2}, {Card: 4, Kind: "CHARGE", N: 5}},
		Life:        &life,
	}, {Battlefield: []int{3}}}}
	cfg := startConfig(t, 11, start, [2]map[int]string{})
	e := New(cfg)

	if got := zoneIdx(e, state.ZHand, 0); !slices.Equal(got, []int{5, 6, 7}) {
		t.Errorf("hand = %v", got)
	}
	if got := zoneIdx(e, state.ZBattlefield, 0); !slices.Equal(got, []int{1, 2, 4}) {
		t.Errorf("battlefield = %v", got)
	}
	if got := zoneIdx(e, state.ZGraveyard, 0); !slices.Equal(got, []int{10, 11}) {
		t.Errorf("graveyard = %v", got)
	}
	if got := zoneIdx(e, state.ZExile, 0); !slices.Equal(got, []int{12}) {
		t.Errorf("exile = %v", got)
	}
	lib := zoneIdx(e, state.ZLibrary, 0)
	if len(lib) != 40-3-3-2-1 || !slices.Equal(lib[:3], []int{20, 21, 22}) {
		t.Errorf("library = %v", lib)
	}
	if e.G.Players[0].Life != 13 || e.G.Players[1].Life != 20 {
		t.Errorf("life = %d/%d", e.G.Players[0].Life, e.G.Players[1].Life)
	}
	byIdx := func(p, idx int) *state.Object { return e.G.Obj(state.ObjID(40*p + idx + 1)) }
	if !byIdx(0, 2).Tapped || byIdx(0, 1).Tapped || byIdx(0, 4).Tapped {
		t.Errorf("tapped = %v %v %v", byIdx(0, 1).Tapped, byIdx(0, 2).Tapped, byIdx(0, 4).Tapped)
	}
	if byIdx(0, 1).Counter("P1P1") != 2 || byIdx(0, 4).Counter("CHARGE") != 5 {
		t.Errorf("counters = %v %v", byIdx(0, 1).Counters, byIdx(0, 4).Counters)
	}
	for _, o := range []*state.Object{byIdx(0, 1), byIdx(0, 2), byIdx(0, 4), byIdx(1, 3)} {
		if o.Zone != state.ZBattlefield || o.SummonSick {
			t.Errorf("obj %d: zone %v summon-sick %v", o.ID, o.Zone, o.SummonSick)
		}
	}
	if len(e.G.Zone(state.ZHand, 1)) != 0 && e.G.Players[1].Life != 20 {
		t.Errorf("seat 1 untouched fields moved")
	}
	// The same events are in the hash chain, and the game plays on.
	if !slices.ContainsFunc(e.L.Events, func(ev events.Event) bool { return ev.Kind == events.SetupSettled }) {
		t.Error("no SetupSettled in the log")
	}
	e.Advance()
	if e.Pending() == nil || e.G.Turn != 1 {
		t.Fatalf("game did not reach a turn-1 decision: turn %d", e.G.Turn)
	}
	if e.G.StartingPlayer != e.G.Active {
		t.Errorf("starting player %d, active %d", e.G.StartingPlayer, e.G.Active)
	}
}

// TestScriptedStartLibraryOrderIsTheClosedRule: the listed cards top first,
// then the post-deal shuffled order minus every placed card, then the unplaced
// opening-hand cards in hand order.
func TestScriptedStartLibraryOrderIsTheClosedRule(t *testing.T) {
	plain := New(startConfig(t, 21, nil, [2]map[int]string{}))
	rest := zoneIdx(plain, state.ZLibrary, 0)
	hand := zoneIdx(plain, state.ZHand, 0)
	if len(hand) != 7 || len(rest) != 33 {
		t.Fatalf("plain deal: hand %d library %d", len(hand), len(rest))
	}
	// Place two cards from the library and one from the dealt hand; script
	// one more of the library on top.
	top := []int{rest[5], hand[2]}
	start := &StartSpec{Seats: []StartSeat{{Battlefield: []int{rest[9]}, Hand: []int{rest[20]}, Library: top}}}
	e := New(startConfig(t, 21, start, [2]map[int]string{}))

	want := slices.Clone(top)
	placed := map[int]bool{rest[5]: true, hand[2]: true, rest[9]: true, rest[20]: true}
	for _, idx := range rest {
		if !placed[idx] {
			want = append(want, idx)
		}
	}
	for _, idx := range hand {
		if !placed[idx] {
			want = append(want, idx)
		}
	}
	if got := zoneIdx(e, state.ZLibrary, 0); !slices.Equal(got, want) {
		t.Errorf("library order\n got %v\nwant %v", got, want)
	}
	// A seat the spec does not list is left exactly as dealt.
	if got, want := zoneIdx(e, state.ZLibrary, 1), zoneIdx(plain, state.ZLibrary, 1); !slices.Equal(got, want) {
		t.Errorf("unlisted seat 1 library moved: %v vs %v", got, want)
	}
}

// TestScriptedStartRngStreamUnchanged: the setup consumes no rng, so the
// stream after genesis is the unscripted game's.
func TestScriptedStartRngStreamUnchanged(t *testing.T) {
	plain := New(startConfig(t, 33, nil, [2]map[int]string{}))
	scripted := New(startConfig(t, 33, &StartSpec{Seats: []StartSeat{{Hand: []int{1, 2}, Battlefield: []int{3}}}}, [2]map[int]string{}))
	if plain.rng.Draws != scripted.rng.Draws {
		t.Fatalf("rng draws %d vs %d", plain.rng.Draws, scripted.rng.Draws)
	}
	if plain.G.StartingPlayer != scripted.G.StartingPlayer {
		t.Fatalf("starting player moved: %d vs %d", plain.G.StartingPlayer, scripted.G.StartingPlayer)
	}
}

func TestScriptedStartDeterministic(t *testing.T) {
	start := &StartSpec{Seats: []StartSeat{{Hand: []int{1}, Battlefield: []int{2, 3}, Graveyard: []int{4}, Library: []int{5}}}}
	a := New(startConfig(t, 5, start, [2]map[int]string{}))
	b := New(startConfig(t, 5, start, [2]map[int]string{}))
	if a.L.Head() != b.L.Head() {
		t.Fatal("two builds of one scripted Config differ")
	}
}

// TestScriptedStartDropsEveryQueuedTrigger: a placed permanent with an entry
// trigger leaves nothing queued or on the stack (D4).
func TestScriptedStartDropsEveryQueuedTrigger(t *testing.T) {
	special := [2]map[int]string{{2: startETB}, {}}
	start := &StartSpec{Seats: []StartSeat{{Battlefield: []int{2}, Hand: []int{4}}}}
	// Control: the same card entering outside a scripted start queues its
	// trigger, so the drop below is the setup's, not the card's.
	e := New(startConfig(t, 9, start, special))
	if n := len(e.pendingTriggers); n != 0 || e.orderedTriggers != 0 {
		t.Fatalf("pending triggers after setup: %d (ordered %d)", n, e.orderedTriggers)
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	e.Advance()
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush {
			t.Fatalf("a trigger reached the stack: %+v", ev)
		}
	}
	if handBefore != 1 {
		t.Fatalf("hand = %d, want the one scripted card", handBefore)
	}
	if e.cmdZoneScan == 0 {
		t.Error("cmdZoneScan not set past the setup events")
	}
}

func TestScriptedStartAttachedAuraAndBackFace(t *testing.T) {
	special := [2]map[int]string{{8: startAura, 14: startDFC}, {}}
	start := &StartSpec{Seats: []StartSeat{
		{Battlefield: []int{8, 14}, Attach: []StartAttach{{Card: 8, ToSeat: 1, To: 7}}, BackFace: []int{14}},
		{Battlefield: []int{7}},
	}}
	e := New(startConfig(t, 3, start, special))
	aura, bearer := e.G.Obj(9), e.G.Obj(40+7+1)
	if aura.Zone != state.ZBattlefield || bearer.Zone != state.ZBattlefield {
		t.Fatalf("zones: aura %v bearer %v", aura.Zone, bearer.Zone)
	}
	if aura.AttachedTo != bearer.ID {
		t.Errorf("aura attached to %d, want %d (the bearer in the other seat)", aura.AttachedTo, bearer.ID)
	}
	if dfc := e.G.Obj(15); dfc.Zone != state.ZBattlefield || dfc.FaceIdx != 1 {
		t.Errorf("DFC zone %v face %d, want battlefield on face 1", dfc.Zone, dfc.FaceIdx)
	}
}

// TestScriptedStartIgnoresBadEntries: out-of-range, repeated and commander
// cards degrade, never crash.
func TestScriptedStartIgnoresBadEntries(t *testing.T) {
	cfg := startConfig(t, 4, &StartSpec{Seats: []StartSeat{{
		Hand: []int{1, 1, -1, 99}, Battlefield: []int{1, 2}, Counters: []StartCounter{{Card: 77, Kind: "X", N: 1}},
	}, {}, {Life: new(int32)}}}, [2]map[int]string{})
	e := New(cfg)
	if got := zoneIdx(e, state.ZHand, 0); !slices.Equal(got, []int{1}) {
		t.Errorf("hand = %v, want [1] (the first claim wins)", got)
	}
	if got := zoneIdx(e, state.ZBattlefield, 0); !slices.Equal(got, []int{2}) {
		t.Errorf("battlefield = %v, want [2]", got)
	}
}

func TestScriptedStartMulligansDisableIt(t *testing.T) {
	cfg := startConfig(t, 4, &StartSpec{Seats: []StartSeat{{Battlefield: []int{2}}}}, [2]map[int]string{})
	cfg.Mulligans = 1
	e := New(cfg)
	if len(e.G.Zone(state.ZBattlefield, 0)) != 0 {
		t.Fatal("a start applied under a mulligan round")
	}
}

func TestScriptedStartSeedForStarter(t *testing.T) {
	for _, seats := range []int{2, 3, 4} {
		for want := 0; want < seats; want++ {
			s := SeedForStarter(100, seats, want)
			if s < 100 {
				t.Fatalf("seed %d < 100", s)
			}
			if got := newRNG(s).IntN(seats); got != want {
				t.Fatalf("seats %d want %d: seed %d tosses %d", seats, want, s, got)
			}
			for lower := uint64(100); lower < s; lower++ {
				if newRNG(lower).IntN(seats) == want {
					t.Fatalf("seats %d want %d: %d is lower and also tosses it", seats, want, lower)
				}
			}
		}
	}
	// A seat that cannot start returns the seed rather than looping.
	if SeedForStarter(5, 2, 2) != 5 || SeedForStarter(5, 0, 0) != 5 {
		t.Error("an unreachable seat must return the seed")
	}
	// The toss decides the engine's starting seat.
	for want := 0; want < 2; want++ {
		cfg := startConfig(t, SeedForStarter(40, 2, want), &StartSpec{}, [2]map[int]string{})
		if got := New(cfg).G.StartingPlayer; int(got) != want {
			t.Errorf("want seat %d to start, got %d", want, got)
		}
	}
}
