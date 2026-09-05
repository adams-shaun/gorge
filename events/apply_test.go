package events

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func bearCard() *cards.Card {
	c, _ := cards.ParseBytes("b.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	c.Link()
	return c
}

// sailorCard is a permanent with one activated ability -- the same fixture
// TestAbilityPushMintsAnActivatedAbilityObject uses -- for exercising
// AbilityPush's actual mint path (as opposed to only its guard) in
// TestApplyIsPure, where a no-abilities bear cannot.
func sailorCard() *cards.Card {
	c, _ := cards.ParseBytes("s.txt", []byte("Name:Sailor\nManaCost:U\nTypes:Creature Spirit\nPT:1/1\nA:AB$ Draw | Cost$ 3 U | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\nOracle:x\n"))
	c.Link()
	return c
}

// gameWithOneCardSrc builds a two-seat game (Ann/Bob) with one card, parsed
// from src, sitting in seat 0's hand. Task 4's own object-field tests need a
// single known object to apply CastInfo/Choose/... events to, rather than
// twoPlayer's five-bears-per-library shape.
func gameWithOneCardSrc(t *testing.T, src string) (*state.Game, state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"Ann", "Bob"})
	c, d := cards.ParseBytes("one.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	o := g.AddObject(c, 0)
	o.Zone = state.ZHand
	g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})
	return g, o.ID
}

// gameWithOneCard is gameWithOneCardSrc with the default fixture: a 2/2
// Bear, the same card twoPlayer/bearCard already use elsewhere in this file.
func gameWithOneCard(t *testing.T) (*state.Game, state.ObjID) {
	t.Helper()
	return gameWithOneCardSrc(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
}

func twoPlayer(t *testing.T) (*state.Game, *Log) {
	t.Helper()
	g := state.NewGame([]string{"a", "b"})
	bear := bearCard()
	for p := state.PlayerID(0); p < 2; p++ {
		var lib []state.ObjID
		for i := 0; i < 5; i++ {
			lib = append(lib, g.AddObject(bear, p).ID)
		}
		g.SetZone(state.ZLibrary, p, lib)
	}
	return g, NewLog(1)
}

// Every object must be in exactly one zone at all times. This is the invariant
// that a naive "move plus push" pair of events silently breaks.
func zoneCount(g *state.Game, id state.ObjID) int {
	n := 0
	for p := state.PlayerID(0); p < state.PlayerID(len(g.Players)); p++ {
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand, state.ZBattlefield, state.ZGraveyard, state.ZExile} {
			for _, x := range g.Zone(z, p) {
				if x == id {
					n++
				}
			}
		}
	}
	for _, x := range g.Stack {
		if x == id {
			n++
		}
	}
	return n
}

func TestMoveKeepsExactlyOneZone(t *testing.T) {
	g, l := twoPlayer(t)
	id := g.Zone(state.ZLibrary, 0)[0]
	for _, step := range []struct{ from, to state.Zone }{
		{state.ZLibrary, state.ZHand},
		{state.ZHand, state.ZStack},
		{state.ZStack, state.ZBattlefield},
		{state.ZBattlefield, state.ZGraveyard},
		{state.ZGraveyard, state.ZExile},
	} {
		Emit(g, l, Event{Kind: MoveZone, Obj: id, From: step.from, To: step.to})
		if got := zoneCount(g, id); got != 1 {
			t.Fatalf("after %s->%s object is in %d zones", step.from, step.to, got)
		}
		if g.Obj(id).Zone != step.to {
			t.Fatalf("Obj.Zone = %s, want %s", g.Obj(id).Zone, step.to)
		}
	}
	if len(g.Zone(state.ZLibrary, 0)) != 4 {
		t.Fatalf("library = %d, want 4", len(g.Zone(state.ZLibrary, 0)))
	}
}

func TestPutOnStackDoesNotDoublePush(t *testing.T) {
	g, l := twoPlayer(t)
	id := g.Zone(state.ZLibrary, 0)[0]
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
	Emit(g, l, Event{Kind: PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
	if len(g.Stack) != 1 {
		t.Fatalf("stack = %v, want one entry", g.Stack)
	}
	if zoneCount(g, id) != 1 {
		t.Fatal("PutOnStack duplicated the object")
	}
	// Resolve is a marker: the object leaves via its own move event.
	Emit(g, l, Event{Kind: Resolve, Obj: id})
	if len(g.Stack) != 1 {
		t.Fatal("Resolve must not pop the stack")
	}
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZStack, To: state.ZGraveyard})
	if len(g.Stack) != 0 || zoneCount(g, id) != 1 {
		t.Fatal("resolution move did not clear the stack cleanly")
	}
}

func TestScalarEventsFold(t *testing.T) {
	g, l := twoPlayer(t)
	id := g.Zone(state.ZLibrary, 0)[0]
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})

	Emit(g, l, Event{Kind: LifeChange, Player: 1, Amount: -3})
	if g.Players[1].Life != 17 {
		t.Errorf("life = %d", g.Players[1].Life)
	}
	Emit(g, l, Event{Kind: Damage, Player: 1, Amount: 2})
	if g.Players[1].Life != 15 {
		t.Errorf("damage to a player must reduce life: %d", g.Players[1].Life)
	}
	Emit(g, l, Event{Kind: Damage, Obj: id, Amount: 1})
	if g.Obj(id).Damage != 1 {
		t.Errorf("object damage = %d", g.Obj(id).Damage)
	}
	Emit(g, l, Event{Kind: Tap, Obj: id})
	if !g.Obj(id).Tapped {
		t.Error("tap did not apply")
	}
	Emit(g, l, Event{Kind: Untap, Obj: id})
	if g.Obj(id).Tapped {
		t.Error("untap did not apply")
	}
	Emit(g, l, Event{Kind: CounterChange, Obj: id, Counter: "P1P1", Amount: 2})
	Emit(g, l, Event{Kind: CounterChange, Obj: id, Counter: "P1P1", Amount: -5})
	if got := g.Obj(id).Counter("P1P1"); got != 0 {
		t.Errorf("counters clamp at zero, got %d", got)
	}
	Emit(g, l, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 2})
	if g.Players[0].Pool[state.MR] != 2 {
		t.Errorf("pool = %v", g.Players[0].Pool)
	}
	Emit(g, l, Event{Kind: ManaClear, Player: 0})
	if g.Players[0].Pool.Total() != 0 {
		t.Error("mana clear did not empty the pool")
	}
}

// TestGameOverFirstWinsAndSubsequentAreNoOps is the Ruling T22-g regression
// test (fix round 1): before this fix, GameOver had no guard against a
// second event on an already-finished game, so a log carrying a win
// followed by a draw (a duplicate event, a replay quirk, a tampered log)
// produced a game that was simultaneously Winner=1 and Draw=true. The
// first GameOver must win; anything after it is a no-op.
func TestGameOverFirstWinsAndSubsequentAreNoOps(t *testing.T) {
	g, l := twoPlayer(t)
	Emit(g, l, Event{Kind: GameOver, Player: 1, Amount: 0})
	if !g.Over || g.Winner != 1 || g.Draw {
		t.Fatalf("after the first GameOver: Over=%v Winner=%d Draw=%v, want Over=true Winner=1 Draw=false",
			g.Over, g.Winner, g.Draw)
	}
	Emit(g, l, Event{Kind: GameOver, Amount: 1})
	if !g.Over || g.Winner != 1 || g.Draw {
		t.Fatalf("after a second GameOver (Amount 1, a draw shape): Over=%v Winner=%d Draw=%v, "+
			"want unchanged (Over=true Winner=1 Draw=false) -- a game already won must not become drawn too",
			g.Over, g.Winner, g.Draw)
	}
}

// TestGameOverWithUnrecognizedAmountChangesNothing is the second half of the
// Ruling T22-g regression test: GameOver defines exactly two shapes (Amount
// 0 = win, Amount 1 = draw). Before this fix, Over was set unconditionally
// regardless of Amount, so a tampered or malformed event with some third
// Amount and an out-of-range Player -- naming no real seat at all -- still
// ended the game, with Winner left at its zero value: indistinguishable
// from a real seat-0 win, the exact ambiguity the Amount discriminator
// exists to remove.
func TestGameOverWithUnrecognizedAmountChangesNothing(t *testing.T) {
	g, l := twoPlayer(t)
	Emit(g, l, Event{Kind: GameOver, Amount: 7, Player: 250})
	if g.Over {
		t.Fatal("a GameOver whose Amount is neither 0 nor 1 is not a shape this build defines " +
			"and must not end the game")
	}
	if g.Winner != 0 || g.Draw {
		t.Fatalf("Winner=%d Draw=%v, want both untouched by an unrecognized GameOver shape", g.Winner, g.Draw)
	}
}

// TestGameOverWithAmountZeroAndInvalidPlayerChangesNothing is the Ruling
// T22-l regression test (fix round 2): the win shape (Amount == 0) must
// require a validated Player, not merely use one when present. The
// fix-round-1 version of this case set Over unconditionally on Amount 0 and
// only guarded Winner -- so {Amount: 0, Player: <out of range>} produced
// Over=true Winner=0 (its untouched zero value): an invalid seat silently
// reading as a real seat-0 win, the same defect class the Amount
// discriminator exists to remove, just for the win shape instead of some
// third Amount. An untrusted log naming no real winner under the win shape
// must change nothing at all -- refusing to end the game is safe and
// detectable; manufacturing a draw would fabricate a result the log never
// carried.
func TestGameOverWithAmountZeroAndInvalidPlayerChangesNothing(t *testing.T) {
	g, l := twoPlayer(t)
	Emit(g, l, Event{Kind: GameOver, Amount: 0, Player: 250})
	if g.Over {
		t.Fatal("GameOver{Amount: 0} with an invalid Player names no real winner and must not end the game")
	}
	if g.Winner != 0 || g.Draw {
		t.Fatalf("Winner=%d Draw=%v, want both untouched", g.Winner, g.Draw)
	}
}

func TestTurnChangeResetsPerTurnState(t *testing.T) {
	g, l := twoPlayer(t)
	id := g.Zone(state.ZLibrary, 1)[0]
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	if !g.Obj(id).SummonSick {
		t.Fatal("entering the battlefield must set summoning sickness")
	}
	g.Players[1].LandsPlayed = 1
	Emit(g, l, Event{Kind: TurnChange, Player: 1, Amount: 4})
	if g.Turn != 4 || g.Active != 1 {
		t.Fatalf("turn = %d active = %d", g.Turn, g.Active)
	}
	if g.Players[1].LandsPlayed != 0 {
		t.Error("land drop not reset")
	}
	if g.Obj(id).SummonSick {
		t.Error("summoning sickness not cleared for the active player")
	}
}

func TestShuffleReplacesLibraryOrder(t *testing.T) {
	g, l := twoPlayer(t)
	want := []state.ObjID{5, 4, 3, 2, 1}
	Emit(g, l, Event{Kind: Shuffle, Player: 0, IDs: want, Secret: true})
	got := g.Zone(state.ZLibrary, 0)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("library = %v, want %v", got, want)
		}
	}
	// The log must own its copy: mutating the caller's slice cannot rewrite
	// history.
	want[0] = 99
	if g.Zone(state.ZLibrary, 0)[0] == 99 {
		t.Fatal("Shuffle aliased the caller's slice")
	}
}

// zoneLocations enumerates every zone of every player (plus the stack) and
// returns a label for each place id currently appears. Unlike zoneCount, it
// names the locations, so a broken invariant is diagnosable from the
// failure message alone.
func zoneLocations(g *state.Game, id state.ObjID) []string {
	var out []string
	for p := state.PlayerID(0); p < state.PlayerID(len(g.Players)); p++ {
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand, state.ZBattlefield, state.ZGraveyard, state.ZExile} {
			for _, x := range g.Zone(z, p) {
				if x == id {
					out = append(out, fmt.Sprintf("p%d:%s", p, z))
				}
			}
		}
	}
	for _, x := range g.Stack {
		if x == id {
			out = append(out, "stack")
		}
	}
	return out
}

// TestMoveZoneInvariantTableDriven exercises Move against exactly the
// scenarios that a naive "trust the caller's From" implementation gets
// wrong: a wrong From, a same-zone move, a repeated move, and a move that
// lands after the object already moved elsewhere for real. In every case
// the object must end up in exactly one zone.
func TestMoveZoneInvariantTableDriven(t *testing.T) {
	cases := []struct {
		name string
		run  func(g *state.Game, l *Log, id state.ObjID)
	}{
		{
			name: "normal move",
			run: func(g *state.Game, l *Log, id state.ObjID) {
				Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
			},
		},
		{
			name: "move with a wrong From",
			run: func(g *state.Game, l *Log, id state.ObjID) {
				// id really lives in the library; the event lies and claims
				// graveyard. The real removal zone must still be found and
				// used, not the claimed one.
				Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZHand})
			},
		},
		{
			name: "move to the zone already occupied",
			run: func(g *state.Game, l *Log, id state.ObjID) {
				Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZLibrary})
			},
		},
		{
			name: "two identical moves in a row",
			run: func(g *state.Game, l *Log, id state.ObjID) {
				Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
				Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
			},
		},
		{
			name: "stale move after the object already moved elsewhere",
			run: func(g *state.Game, l *Log, id state.ObjID) {
				// The object genuinely moves to hand...
				Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
				// ...then a second, stale event still claims it is coming
				// from the library (its pre-move location) and sends it to
				// the battlefield. The removal must come from hand (its
				// real zone), not library.
				Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, l := twoPlayer(t)
			id := g.Zone(state.ZLibrary, 0)[0]
			tc.run(g, l, id)
			if locs := zoneLocations(g, id); len(locs) != 1 {
				t.Fatalf("object in %d zones after %q: %v", len(locs), tc.name, locs)
			}
		})
	}
}

// TestApplyNeverPanics feeds every Kind a battery of hostile field values —
// zero/garbage ObjIDs, out-of-range players, out-of-range zones, extreme
// amounts, and IDs/Pairs naming nothing real. Apply runs on the match's only
// goroutine and Event reaches it from network input, so any panic here is a
// remote kill of the whole match: every case must be a no-op, never a crash.
//
// The kind list is Kind(0)..len(kindNames)-1, not a hand-typed slice, on
// purpose: a hand-typed list is exactly how this test used to miss coverage
// -- the previous version of this loop enumerated every Kind up to ClockTick
// and simply never mentioned TriggerPush, whose own Player-handling bug
// (Ruling T20-e) this test would otherwise exist to catch. kindNames is the
// one place already required to stay in lockstep with the Kind const block
// (Kind.String() reads it, and every new Kind ships with its own
// Test<Kind>KindString regression test asserting that mapping), so bounding
// the loop by its length means a future appended Kind is covered here with
// no edit to this test required -- exactly what "append-only, never
// renumbered" should buy a totality test like this one.
func TestApplyNeverPanics(t *testing.T) {
	const badZone = state.Zone(200)
	const badObj = state.ObjID(999999)
	const badPlayer = state.PlayerID(250)

	for k := Kind(0); int(k) < len(kindNames); k++ {
		t.Run(k.String(), func(t *testing.T) {
			variants := []struct {
				name string
				e    Event
			}{
				{"obj zero", Event{Kind: k, Obj: 0, Player: 0}},
				{"obj out of range", Event{Kind: k, Obj: badObj, Player: 0}},
				{"player out of range", Event{Kind: k, Player: badPlayer}},
				{"player out of range with a valid obj", Event{Kind: k, Player: badPlayer}},
				{"from zone out of range", Event{Kind: k, Player: 0, From: badZone, To: state.ZHand}},
				{"to zone out of range", Event{Kind: k, Player: 0, From: state.ZLibrary, To: badZone}},
				{"amount min int32", Event{Kind: k, Player: 0, Amount: math.MinInt32}},
				{"amount max int32", Event{Kind: k, Player: 0, Amount: math.MaxInt32}},
				{"ids nonexistent", Event{Kind: k, Player: 0, IDs: []state.ObjID{badObj}}},
				{"pairs nonexistent", Event{Kind: k, Player: 0, Pairs: [][2]state.ObjID{{badObj, badObj}}}},
			}
			for _, v := range variants {
				t.Run(v.name, func(t *testing.T) {
					g, _ := twoPlayer(t)
					id := g.Zone(state.ZLibrary, 0)[0]
					e := v.e
					// Route the case at a real object/zone so the guard under
					// test is actually exercised, not short-circuited by a
					// zero Obj. "obj *", "player out of range" and
					// "ids/pairs" variants deliberately keep Obj at its
					// zero/bad value: for Damage in particular, Obj==0
					// combined with a bad Player is what reaches the
					// unguarded g.Players[e.Player] fallback branch.
					switch v.name {
					case "from zone out of range", "to zone out of range",
						"amount min int32", "amount max int32":
						e.Obj = id
					case "player out of range with a valid obj":
						// A bear (twoPlayer's fixture card) has no T: line,
						// so it has zero triggers -- TriggerPush would break
						// on its trigger-index guard before ever reaching
						// the Player-consuming AddObject call this variant
						// exists to reach, whether or not Ruling T20-e's fix
						// is present, which would make this variant vacuous
						// for the one kind that motivated it. Give
						// TriggerPush a source that actually has a trigger
						// so an out-of-range Player has to be stopped by
						// validPlayer specifically, not by an unrelated
						// earlier guard.
						if k == TriggerPush {
							scribe := triggerCard(t)
							src := g.AddObject(scribe, 0)
							src.Zone = state.ZBattlefield
							g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})
							e.Obj = src.ID
						} else {
							e.Obj = id
						}
					}

					before := len(g.Objs)
					Apply(g, e) // must not panic

					switch v.name {
					case "from zone out of range":
						// From is untrusted and unused for the mutation (see
						// Move / FIX1): a garbage From with a valid To still
						// succeeds, landing the object in exactly one zone.
						if (k == MoveZone || k == Draw || k == PutOnStack) && len(zoneLocations(g, id)) != 1 {
							t.Fatalf("object must still land in exactly one zone, got %v", zoneLocations(g, id))
						}
					case "to zone out of range":
						// To is the real destination; an invalid one must
						// no-op the whole move, leaving the object exactly
						// where it started.
						if k == MoveZone || k == Draw || k == PutOnStack {
							if locs := zoneLocations(g, id); len(locs) != 1 || locs[0] != "p0:library" {
								t.Fatalf("out-of-range To must no-op the move, got %v", locs)
							}
						}
					case "player out of range":
						if k == PlayerLost && g.Players[0].Lost {
							t.Fatal("out-of-range player must not affect player 0")
						}
					case "player out of range with a valid obj":
						// Ruling T20-e's regression: a real trigger source
						// plus an out-of-range Player must still mint no
						// ability object, not merely avoid a panic.
						if k == TriggerPush && len(g.Objs) != before {
							t.Fatal("TriggerPush with an out-of-range Player must not create the ability object")
						}
					}
				})
			}
		})
	}
}

// TestLandPlayedIncrementsLandsPlayed is the carrier for a land drop: nothing
// but this event may ever change Players[p].LandsPlayed outside a
// TurnChange reset, so a game reconstructed from the log matches the live
// game on that field.
func TestLandPlayedIncrementsLandsPlayed(t *testing.T) {
	g, l := twoPlayer(t)
	if g.Players[0].LandsPlayed != 0 {
		t.Fatalf("LandsPlayed = %d before any event, want 0", g.Players[0].LandsPlayed)
	}
	Emit(g, l, Event{Kind: LandPlayed, Player: 0})
	if g.Players[0].LandsPlayed != 1 {
		t.Fatalf("LandsPlayed = %d, want 1", g.Players[0].LandsPlayed)
	}
	Emit(g, l, Event{Kind: LandPlayed, Player: 0})
	if g.Players[0].LandsPlayed != 2 {
		t.Fatalf("LandsPlayed = %d, want 2", g.Players[0].LandsPlayed)
	}
	if g.Players[1].LandsPlayed != 0 {
		t.Fatal("LandPlayed for player 0 must not affect player 1")
	}
}

// TestLandPlayedIgnoresInvalidPlayer mirrors the invalid-player guard every
// other player-scoped case uses: an out-of-range player must be a no-op,
// never a panic or a stray mutation.
func TestLandPlayedIgnoresInvalidPlayer(t *testing.T) {
	g, l := twoPlayer(t)
	Emit(g, l, Event{Kind: LandPlayed, Player: state.PlayerID(99)})
	for _, p := range g.Players {
		if p.LandsPlayed != 0 {
			t.Fatalf("LandsPlayed = %d after an out-of-range player event, want 0", p.LandsPlayed)
		}
	}
}

func TestLandPlayedKindString(t *testing.T) {
	if got := LandPlayed.String(); got != "land_played" {
		t.Fatalf("LandPlayed.String() = %q, want %q", got, "land_played")
	}
}

// TestDamageClampsAtZero mirrors AddCounter's existing clamp: Object.Damage
// must never go negative, however large a negative Damage event's Amount is.
func TestDamageClampsAtZero(t *testing.T) {
	g, l := twoPlayer(t)
	id := g.Zone(state.ZLibrary, 0)[0]
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})

	Emit(g, l, Event{Kind: Damage, Obj: id, Amount: 3})
	if g.Obj(id).Damage != 3 {
		t.Fatalf("damage = %d, want 3", g.Obj(id).Damage)
	}
	Emit(g, l, Event{Kind: Damage, Obj: id, Amount: -10})
	if g.Obj(id).Damage != 0 {
		t.Fatalf("damage clamps at zero, got %d", g.Obj(id).Damage)
	}
	Emit(g, l, Event{Kind: Damage, Obj: id, Amount: math.MinInt32})
	if g.Obj(id).Damage != 0 {
		t.Fatalf("damage clamps at zero even for extreme negative amounts, got %d", g.Obj(id).Damage)
	}
}

// TestApplyIsPure applies a varied event sequence to two independently built
// but initially identical games and asserts the resulting states are deeply
// equal, including zone contents and order — not just lengths. Apply must
// stay a pure function of (g, e): the same events on the same starting
// state always produce the same result.
func TestApplyIsPure(t *testing.T) {
	build := func(t *testing.T) (*state.Game, *Log) {
		t.Helper()
		g, l := twoPlayer(t)
		// Task 4: a token table, so TokenCreate below actually mints
		// (rather than only exercising its no-op guard) -- reusing
		// bearCard's own source is enough to prove the mutation is
		// deterministic; it need not be a "real" token script.
		g.Tokens = map[string]*cards.Card{"bear": bearCard()}
		// A sixth card in seat 0's library with one activated ability, so
		// AbilityPush below can actually mint an ability object rather than
		// only exercise its no-abilities guard.
		sailor := g.AddObject(sailorCard(), 0)
		g.SetZone(state.ZLibrary, 0, append(g.Zone(state.ZLibrary, 0), sailor.ID))
		return g, l
	}

	sequence := func(g *state.Game, l *Log) {
		lib0 := append([]state.ObjID(nil), g.Zone(state.ZLibrary, 0)...)
		lib1 := append([]state.ObjID(nil), g.Zone(state.ZLibrary, 1)...)

		Emit(g, l, Event{Kind: MoveZone, Obj: lib0[0], From: state.ZLibrary, To: state.ZHand})
		Emit(g, l, Event{Kind: MoveZone, Obj: lib0[1], From: state.ZLibrary, To: state.ZHand})
		Emit(g, l, Event{Kind: MoveZone, Obj: lib0[0], From: state.ZHand, To: state.ZBattlefield})
		Emit(g, l, Event{Kind: Tap, Obj: lib0[0]})
		Emit(g, l, Event{Kind: CounterChange, Obj: lib0[0], Counter: "P1P1", Amount: 2})
		Emit(g, l, Event{Kind: LifeChange, Player: 1, Amount: -5})
		Emit(g, l, Event{Kind: Damage, Player: 1, Amount: 2})
		Emit(g, l, Event{Kind: Damage, Obj: lib0[0], Amount: 1})
		Emit(g, l, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 3})
		Emit(g, l, Event{Kind: MoveZone, Obj: lib1[0], From: state.ZLibrary, To: state.ZBattlefield})

		// Task 4's six new kinds, one apiece, exercised here so any hidden
		// non-determinism in their Apply cases (map-range order, and so on)
		// would show up as a divergence below exactly like every other
		// kind's already does.
		Emit(g, l, Event{Kind: CastInfo, Obj: lib1[0], Amount: 4, Counter: "kicked,miracle"})
		Emit(g, l, Event{Kind: Choose, Obj: lib1[0], Counter: "type", Text: "Bear"})
		Emit(g, l, Event{Kind: Attach, Obj: lib0[0], IDs: []state.ObjID{lib1[0]}})
		Emit(g, l, Event{Kind: TokenCreate, Player: 0, Text: "bear"})
		Emit(g, l, Event{Kind: PutOnStack, Obj: lib1[1], Player: 1, From: state.ZLibrary, To: state.ZStack})
		Emit(g, l, Event{Kind: StackCopy, Obj: lib1[1], Player: 1})
		// lib0[5] is the sailor build added: on the battlefield with one
		// activated ability, so this actually mints an ability object
		// rather than only exercising the no-abilities guard.
		Emit(g, l, Event{Kind: MoveZone, Obj: lib0[5], From: state.ZLibrary, To: state.ZBattlefield})
		Emit(g, l, Event{Kind: AbilityPush, Obj: lib0[5], Player: 0, Amount: 0})

		Emit(g, l, Event{Kind: DeclareAttackers, Player: 0, IDs: []state.ObjID{lib0[0]}})
		Emit(g, l, Event{Kind: DeclareBlockers, Pairs: [][2]state.ObjID{{lib1[0], lib0[0]}}})
		Emit(g, l, Event{Kind: TurnChange, Player: 1, Amount: 2})
		Emit(g, l, Event{Kind: MoveZone, Obj: lib0[0], From: state.ZBattlefield, To: state.ZGraveyard})
		// A stale/incorrect From on the last move: real zone is hand, not
		// library. Purity must hold even through the invariant-guard path.
		Emit(g, l, Event{Kind: MoveZone, Obj: lib0[1], From: state.ZLibrary, To: state.ZExile})
	}

	g1, l1 := build(t)
	sequence(g1, l1)

	g2, l2 := build(t)
	sequence(g2, l2)

	if !reflect.DeepEqual(g1.Players, g2.Players) {
		t.Fatalf("players diverged:\n%+v\n%+v", g1.Players, g2.Players)
	}
	if !reflect.DeepEqual(g1.Objs, g2.Objs) {
		t.Fatalf("objects diverged:\n%+v\n%+v", g1.Objs, g2.Objs)
	}
	if !reflect.DeepEqual(g1.Stack, g2.Stack) {
		t.Fatalf("stack diverged: %v vs %v", g1.Stack, g2.Stack)
	}
	for p := state.PlayerID(0); p < state.PlayerID(len(g1.Players)); p++ {
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand, state.ZBattlefield, state.ZGraveyard, state.ZExile} {
			a, b := g1.Zone(z, p), g2.Zone(z, p)
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("p%d:%s diverged: %v vs %v", p, z, a, b)
			}
		}
	}
	if g1.Turn != g2.Turn || g1.Active != g2.Active || g1.Clock != g2.Clock {
		t.Fatalf("scalar state diverged: turn=%d/%d active=%d/%d clock=%d/%d",
			g1.Turn, g2.Turn, g1.Active, g2.Active, g1.Clock, g2.Clock)
	}
}

// TestTargetsChosenSetsObjectTargets is Ruling T14-b's regression test: a
// spell or ability's chosen targets must flow through an event, the same as
// every other state mutation, so that replaying the log alone reproduces
// what the live game has. Covers both target shapes (Amount discriminates
// them), a nil-object no-op, an invalid-player no-op, and String().
func TestTargetsChosenSetsObjectTargets(t *testing.T) {
	g, l := twoPlayer(t)
	spell := g.Zone(state.ZLibrary, 0)[0]
	Emit(g, l, Event{Kind: MoveZone, Obj: spell, From: state.ZLibrary, To: state.ZStack})

	creature := g.Zone(state.ZLibrary, 0)[1]

	// Object targets: Amount 0, read from IDs.
	Emit(g, l, Event{Kind: TargetsChosen, Obj: spell, Amount: 0, IDs: []state.ObjID{creature}})
	got := g.Obj(spell).Targets
	if len(got) != 1 || got[0].Obj != creature || got[0].IsPlayer {
		t.Fatalf("object target = %+v, want a single object target of %d", got, creature)
	}

	// A player target: Amount 1, read from Player. Overwrites the prior
	// object target rather than accumulating.
	Emit(g, l, Event{Kind: TargetsChosen, Obj: spell, Amount: 1, Player: 0})
	got = g.Obj(spell).Targets
	if len(got) != 1 || !got[0].IsPlayer || got[0].Player != 0 {
		t.Fatalf("player target = %+v, want a single player-0 target", got)
	}
	// PlayerID 0 is both a real seat and the zero value: confirm this is not
	// mistaken for "no target" by checking IsPlayer explicitly, and that
	// seat 1 also works (rules out the discriminator secretly keying off a
	// nonzero Player value instead of Amount).
	Emit(g, l, Event{Kind: TargetsChosen, Obj: spell, Amount: 1, Player: 1})
	got = g.Obj(spell).Targets
	if len(got) != 1 || !got[0].IsPlayer || got[0].Player != 1 {
		t.Fatalf("player-1 target = %+v, want a single player-1 target", got)
	}

	// A nil object (Obj 0, or one that does not exist) must be a no-op, not
	// a panic.
	before := g.Obj(spell).Targets
	Emit(g, l, Event{Kind: TargetsChosen, Obj: 0, Amount: 1, Player: 0})
	Emit(g, l, Event{Kind: TargetsChosen, Obj: state.ObjID(99999), Amount: 1, Player: 0})
	if got := g.Obj(spell).Targets; !reflect.DeepEqual(got, before) {
		t.Fatalf("a nil-object TargetsChosen event mutated something: %+v", got)
	}

	// An invalid player must be a no-op, leaving the object's existing
	// targets (from the last valid write) untouched.
	Emit(g, l, Event{Kind: TargetsChosen, Obj: spell, Amount: 1, Player: state.PlayerID(250)})
	if got := g.Obj(spell).Targets; len(got) != 1 || !got[0].IsPlayer || got[0].Player != 1 {
		t.Fatalf("invalid-player TargetsChosen changed targets to %+v, want unchanged", got)
	}

	if got, want := TargetsChosen.String(), "targets_chosen"; got != want {
		t.Fatalf("TargetsChosen.String() = %q, want %q", got, want)
	}
}

// twoFacedCard builds a two-face card for FlipFace's tests: parseSA/parse.go
// starts a new Face on an "ALTERNATE" line.
func twoFacedCard(t *testing.T) *cards.Card {
	t.Helper()
	src := "Name:Front\nTypes:Creature\nPT:1/1\nOracle:x\n\nALTERNATE\n\nName:Back\nTypes:Creature\nPT:3/3\nOracle:x\n"
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	return c
}

// TestFlipFaceChangesActiveFace is Task 18's regression test for SetState's
// dedicated event (Ruling T18-a): FaceIdx must move through an event like
// every other mutation, and every hostile input (nil object, a single-face
// card, an out-of-range index) must be a no-op rather than a panic or an
// out-of-bounds FaceIdx.
func TestFlipFaceChangesActiveFace(t *testing.T) {
	g, l := twoPlayer(t)
	two := twoFacedCard(t)
	o := g.AddObject(two, 0)
	o.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), o.ID))

	if o.FaceIdx != 0 {
		t.Fatalf("FaceIdx = %d before any event, want 0", o.FaceIdx)
	}
	Emit(g, l, Event{Kind: FlipFace, Obj: o.ID, Amount: 1})
	if o.FaceIdx != 1 {
		t.Fatalf("FaceIdx = %d, want 1", o.FaceIdx)
	}
	if o.Face().Name != "Back" {
		t.Fatalf("Face().Name = %q, want Back", o.Face().Name)
	}
	Emit(g, l, Event{Kind: FlipFace, Obj: o.ID, Amount: 0})
	if o.FaceIdx != 0 || o.Face().Name != "Front" {
		t.Fatalf("flip back failed: FaceIdx=%d name=%q", o.FaceIdx, o.Face().Name)
	}

	// An index at or beyond len(Faces) must not corrupt FaceIdx.
	Emit(g, l, Event{Kind: FlipFace, Obj: o.ID, Amount: 2})
	if o.FaceIdx != 0 {
		t.Fatalf("out-of-range FaceIdx accepted: %d", o.FaceIdx)
	}
	Emit(g, l, Event{Kind: FlipFace, Obj: o.ID, Amount: -1})
	if o.FaceIdx != 0 {
		t.Fatalf("negative FaceIdx accepted: %d", o.FaceIdx)
	}

	// A single-face card (the bear fixture) must never move off face 0.
	single := g.Zone(state.ZLibrary, 1)[0]
	Emit(g, l, Event{Kind: FlipFace, Obj: single, Amount: 1})
	if g.Obj(single).FaceIdx != 0 {
		t.Fatal("FlipFace moved a single-face card off its only face")
	}

	// A token (Card == nil) and a nonexistent object must both be no-ops.
	Emit(g, l, Event{Kind: FlipFace, Obj: 0, Amount: 1})
	Emit(g, l, Event{Kind: FlipFace, Obj: state.ObjID(99999), Amount: 1})

	if got, want := FlipFace.String(), "flip_face"; got != want {
		t.Fatalf("FlipFace.String() = %q, want %q", got, want)
	}
}

// TestClockTickIncrementsClock is the carrier for Ruling T19-a: Game.Clock
// must only ever advance through this event, never a direct field write, so
// a game reconstructed from the log alone matches a live game's Clock (and
// therefore the Object.Timestamp values Move stamps from it).
func TestClockTickIncrementsClock(t *testing.T) {
	g, l := twoPlayer(t)
	if g.Clock != 0 {
		t.Fatalf("Clock = %d before any event, want 0", g.Clock)
	}
	Emit(g, l, Event{Kind: ClockTick})
	if g.Clock != 1 {
		t.Fatalf("Clock = %d after one tick, want 1", g.Clock)
	}
	Emit(g, l, Event{Kind: ClockTick})
	if g.Clock != 2 {
		t.Fatalf("Clock = %d after two ticks, want 2", g.Clock)
	}
}

func TestClockTickKindString(t *testing.T) {
	if got, want := ClockTick.String(), "clock_tick"; got != want {
		t.Fatalf("ClockTick.String() = %q, want %q", got, want)
	}
}

// triggerCard is a permanent with one T: line, for TriggerPush's own tests.
// Its Execute$ SVar (TrigDraw, a DB$ Draw) is what a resolved ability object
// should end up carrying as its Ability.
func triggerCard(t *testing.T) *cards.Card {
	t.Helper()
	c, d := cards.ParseBytes("t.txt", []byte(`Name:Scribe
ManaCost:1 U
Types:Creature Human
PT:1/1
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDraw | TriggerDescription$ draw
SVar:TrigDraw:DB$ Draw | NumCards$ 1 | Defined$ You
Oracle:x
`))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	return c
}

// TestTriggerPushCreatesTheAbilityObjectOnTheStack is Ruling T20-a's carrier:
// the ability object must be minted inside Apply itself (from Player/Obj/
// Amount/IDs, no new Event field), not by a direct, unlogged Game.AddObject
// call from rules.Engine, or a log-only replay can never recreate it.
func TestTriggerPushCreatesTheAbilityObjectOnTheStack(t *testing.T) {
	g, l := twoPlayer(t)
	scribe := triggerCard(t)
	src := g.AddObject(scribe, 0)
	src.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})

	before := len(g.Objs)
	remembered := state.ObjID(999) // an arbitrary "triggering object" id for this focused test
	Emit(g, l, Event{Kind: TriggerPush, Player: 0, Obj: src.ID, Amount: 0, IDs: []state.ObjID{remembered}})

	if len(g.Objs) != before+1 {
		t.Fatalf("object count = %d, want %d (one ability object created)", len(g.Objs), before+1)
	}
	if len(g.Stack) != 1 {
		t.Fatalf("stack = %v, want the ability object", g.Stack)
	}
	ability := g.Obj(g.Stack[0])
	if ability.Card != nil {
		t.Fatal("ability object must have no Face (Ruling F3)")
	}
	if ability.Ability == nil || ability.Ability.API != "Draw" {
		t.Fatalf("Ability = %+v, want Scribe's Execute$ SVar (a Draw SA)", ability.Ability)
	}
	if ability.Source != src.ID {
		t.Fatalf("Source = %d, want %d (the permanent whose trigger fired)", ability.Source, src.ID)
	}
	if ability.Controller != 0 || ability.Owner != 0 {
		t.Fatalf("Controller/Owner = %d/%d, want 0/0", ability.Controller, ability.Owner)
	}
	if len(ability.Remembered) != 1 || ability.Remembered[0].Obj != remembered {
		t.Fatalf("Remembered = %v, want [{Obj: %d}]", ability.Remembered, remembered)
	}
}

// TestTriggerPushDegradesGracefullyForAMissingSourceOrIndex covers totality:
// a nonexistent source, or a trigger index this permanent's Face doesn't
// have (stale data, a tampered log), must no-op rather than panic.
func TestTriggerPushDegradesGracefullyForAMissingSourceOrIndex(t *testing.T) {
	g, l := twoPlayer(t)
	before := len(g.Objs)
	Emit(g, l, Event{Kind: TriggerPush, Player: 0, Obj: 999999, Amount: 0})
	if len(g.Objs) != before {
		t.Fatalf("object count = %d, want unchanged %d for a nonexistent source", len(g.Objs), before)
	}

	scribe := triggerCard(t)
	src := g.AddObject(scribe, 0)
	src.Zone = state.ZBattlefield
	before = len(g.Objs)
	Emit(g, l, Event{Kind: TriggerPush, Player: 0, Obj: src.ID, Amount: 99})
	if len(g.Objs) != before {
		t.Fatalf("object count = %d, want unchanged %d for an out-of-range trigger index", len(g.Objs), before)
	}
}

func TestTriggerPushKindString(t *testing.T) {
	if got, want := TriggerPush.String(), "trigger_push"; got != want {
		t.Fatalf("TriggerPush.String() = %q, want %q", got, want)
	}
}

// TestEndCombatResetClearsIsAttackingAndBlockedBy is Ruling T21-e's carrier:
// rules.setStep used to clear these two fields with a direct loop over
// e.G.Objs instead of emitting anything, so a log-only reconstruction never
// learned combat had ended. This exercises Apply's own EndCombatReset case in
// isolation (no rules.Engine involved), on two objects: one still marked as
// attacking/blocked, and one already clear, confirming the clear one is left
// alone (not just that the case doesn't panic).
func TestEndCombatResetClearsIsAttackingAndBlockedBy(t *testing.T) {
	g, l := twoPlayer(t)
	attacker := g.Obj(g.Zone(state.ZLibrary, 0)[0])
	attacker.Zone = state.ZBattlefield
	attacker.IsAttacking = true
	attacker.Attacking = 1
	blocker := g.Obj(g.Zone(state.ZLibrary, 1)[0])
	blocker.Zone = state.ZBattlefield
	attacker.BlockedBy = []state.ObjID{blocker.ID}

	untouched := g.Obj(g.Zone(state.ZLibrary, 0)[1])
	untouched.Zone = state.ZBattlefield

	Emit(g, l, Event{Kind: EndCombatReset})

	if attacker.IsAttacking {
		t.Error("IsAttacking should be cleared")
	}
	if attacker.BlockedBy != nil {
		t.Errorf("BlockedBy = %v, want nil", attacker.BlockedBy)
	}
	if untouched.IsAttacking || untouched.BlockedBy != nil {
		t.Fatal("an already-clear object should stay clear, not merely not panic")
	}
}

func TestEndCombatResetKindString(t *testing.T) {
	if got, want := EndCombatReset.String(), "end_combat_reset"; got != want {
		t.Fatalf("EndCombatReset.String() = %q, want %q", got, want)
	}
}

// TestCastInfoRecordsXAndFlags is Task 4's carrier for CastInfo: X and
// CastFlags must survive a spell resolving onto the battlefield (an ETB
// "if it was kicked" trigger needs to read them off the permanent) and
// reset once that permanent actually leaves the battlefield again.
func TestCastInfoRecordsXAndFlags(t *testing.T) {
	g, id := gameWithOneCard(t)
	Apply(g, Event{Kind: CastInfo, Obj: id, Amount: 3, Counter: "kicked,flashback"})
	o := g.Obj(id)
	if o.X != 3 || o.CastFlags != state.FlagKicked|state.FlagFlashback {
		t.Fatalf("%+v", o)
	}
	if FlagsString(o.CastFlags) != "kicked,flashback" || FlagsFrom("surged,miracle") != state.FlagSurged|state.FlagMiracle {
		t.Fatal("flag round trip")
	}
	Move(g, id, state.ZHand, state.ZBattlefield)
	if g.Obj(id).X != 3 || g.Obj(id).CastFlags == 0 {
		t.Fatal("cast info must survive onto the battlefield (ETB 'if it was kicked')")
	}
	Move(g, id, state.ZBattlefield, state.ZGraveyard)
	if g.Obj(id).X != 0 || g.Obj(id).CastFlags != 0 {
		t.Fatal("cast info must reset when the permanent leaves the battlefield")
	}
}

func TestCastInfoKindString(t *testing.T) {
	if got, want := CastInfo.String(), "cast_info"; got != want {
		t.Fatalf("CastInfo.String() = %q, want %q", got, want)
	}
}

// TestFlagsFromTrimsWhitespaceAndIgnoresUnknownNames pins down the two
// FlagsFrom behaviors TestCastInfoRecordsXAndFlags' clean "kicked,flashback"
// round trip does not exercise: surrounding whitespace around a flag name
// (a log or a hand-typed Counter string might carry it) must not stop that
// name from matching, an unrecognized name must be silently ignored rather
// than panicking or matching something else, and the empty string -- CastInfo
// with no flags at all -- must parse to zero.
func TestFlagsFromTrimsWhitespaceAndIgnoresUnknownNames(t *testing.T) {
	if got, want := FlagsFrom(" kicked , bogus "), state.FlagKicked; got != want {
		t.Fatalf("FlagsFrom(%q) = %d, want %d (kicked only, trimmed, bogus ignored)", " kicked , bogus ", got, want)
	}
	if got := FlagsFrom(""); got != 0 {
		t.Fatalf("FlagsFrom(\"\") = %d, want 0", got)
	}
}

// TestChooseRecordsNameTypeAndNumber is Task 4's carrier for Choose: each of
// its three shapes lands on the right field, an unrecognized shape is a
// no-op (not a panic), and a nil object is a no-op too.
func TestChooseRecordsNameTypeAndNumber(t *testing.T) {
	g, id := gameWithOneCard(t)
	Apply(g, Event{Kind: Choose, Obj: id, Counter: "name", Text: "Lightning Bolt"})
	Apply(g, Event{Kind: Choose, Obj: id, Counter: "type", Text: "Goblin"})
	Apply(g, Event{Kind: Choose, Obj: id, Counter: "number", Amount: 2})
	o := g.Obj(id)
	if o.ChosenName != "Lightning Bolt" || o.ChosenType != "Goblin" || o.ChosenNumber != 2 {
		t.Fatalf("%+v", o)
	}
	Apply(g, Event{Kind: Choose, Obj: id, Counter: "colour", Text: "x"}) // unknown kind: no-op, no panic
	Apply(g, Event{Kind: Choose, Obj: 999, Counter: "name", Text: "x"})
}

func TestChooseKindString(t *testing.T) {
	if got, want := Choose.String(), "choose"; got != want {
		t.Fatalf("Choose.String() = %q, want %q", got, want)
	}
}

// TestTokenCreateMintsFromTheGameTokenTable is Task 4's carrier for
// TokenCreate: the minted object comes from Game.Tokens (never from data
// smuggled through the event itself), lands on the battlefield summoning
// sick, and an unknown key or invalid player is a no-op.
func TestTokenCreateMintsFromTheGameTokenTable(t *testing.T) {
	g, _ := gameWithOneCard(t)
	tok, _ := cards.ParseBytes("tok.txt", []byte("Name:Goblin Token\nManaCost:no cost\nTypes:Creature Goblin\nColors:red\nPT:1/1\nOracle:\n"))
	g.Tokens = map[string]*cards.Card{"r_1_1_goblin": tok}
	before := len(g.Objs)
	Apply(g, Event{Kind: TokenCreate, Player: 1, Text: "r_1_1_goblin"})
	if len(g.Objs) != before+1 {
		t.Fatal("no object minted")
	}
	o := g.Obj(state.ObjID(before + 1))
	if !o.IsToken || o.Owner != 1 || o.Controller != 1 || o.Zone != state.ZBattlefield || o.Face().Name != "Goblin Token" || !o.SummonSick {
		t.Fatalf("token %+v", o)
	}
	if bf := g.Zone(state.ZBattlefield, 1); len(bf) != 1 || bf[0] != o.ID {
		t.Fatal("token not in its controller's battlefield list")
	}
	Apply(g, Event{Kind: TokenCreate, Player: 1, Text: "no_such_token"}) // unknown key: no-op
	Apply(g, Event{Kind: TokenCreate, Player: 9, Text: "r_1_1_goblin"})  // invalid player: no-op
	if len(g.Objs) != before+1 {
		t.Fatal("bad TokenCreate minted something")
	}
}

func TestTokenCreateKindString(t *testing.T) {
	if got, want := TokenCreate.String(), "token_create"; got != want {
		t.Fatalf("TokenCreate.String() = %q, want %q", got, want)
	}
}

// TestStackCopyDuplicatesASpellAboveIt is Task 4's carrier for StackCopy:
// the copy shares the original's Card/FaceIdx/X/CastFlags/Targets by value
// (never by aliasing a shared slice), lands directly above the original on
// the stack, and a missing or off-stack source is a no-op.
func TestStackCopyDuplicatesASpellAboveIt(t *testing.T) {
	g, id := gameWithOneCard(t)
	Apply(g, Event{Kind: PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
	Apply(g, Event{Kind: TargetsChosen, Obj: id, Player: 1, Amount: 1})
	Apply(g, Event{Kind: CastInfo, Obj: id, Amount: 2, Counter: "kicked"})
	Apply(g, Event{Kind: StackCopy, Obj: id, Player: 0})
	if len(g.Stack) != 2 || g.Stack[1] == id {
		t.Fatalf("stack %v", g.Stack)
	}
	c := g.Obj(g.Stack[1])
	if !c.IsCopy || c.Card != g.Obj(id).Card || c.FaceIdx != g.Obj(id).FaceIdx || c.Controller != 0 || c.X != 2 || c.CastFlags != state.FlagKicked {
		t.Fatalf("copy %+v", c)
	}
	if len(c.Targets) != 1 || !c.Targets[0].IsPlayer || c.Targets[0].Player != 1 {
		t.Fatalf("copy targets %v", c.Targets)
	}
	c.Targets[0].Player = 0
	if g.Obj(id).Targets[0].Player != 1 {
		t.Fatal("copy shares its Targets array with the original")
	}
	Apply(g, Event{Kind: StackCopy, Obj: 999, Player: 0}) // nothing there: no-op
	Move(g, id, state.ZStack, state.ZGraveyard)
	Apply(g, Event{Kind: StackCopy, Obj: id, Player: 0}) // not on the stack: no-op
	if len(g.Stack) != 1 {
		t.Fatalf("stack %v", g.Stack)
	}
}

func TestStackCopyKindString(t *testing.T) {
	if got, want := StackCopy.String(), "stack_copy"; got != want {
		t.Fatalf("StackCopy.String() = %q, want %q", got, want)
	}
}

// TestAttachAndDetach is Task 4's carrier for Attach: an IDs entry attaches,
// an empty IDs detaches, leaving the battlefield always detaches, and an
// unknown target is a no-op.
func TestAttachAndDetach(t *testing.T) {
	g, eq := gameWithOneCard(t)
	tgt := g.AddObject(g.Obj(eq).Card, 0).ID
	Move(g, eq, state.ZLibrary, state.ZBattlefield)
	Move(g, tgt, state.ZLibrary, state.ZBattlefield)
	Apply(g, Event{Kind: Attach, Obj: eq, IDs: []state.ObjID{tgt}})
	if g.Obj(eq).AttachedTo != tgt {
		t.Fatal("not attached")
	}
	Apply(g, Event{Kind: Attach, Obj: eq})
	if g.Obj(eq).AttachedTo != 0 {
		t.Fatal("not detached")
	}
	Apply(g, Event{Kind: Attach, Obj: eq, IDs: []state.ObjID{tgt}})
	Move(g, eq, state.ZBattlefield, state.ZGraveyard)
	if g.Obj(eq).AttachedTo != 0 {
		t.Fatal("leaving the battlefield must detach")
	}
	Apply(g, Event{Kind: Attach, Obj: eq, IDs: []state.ObjID{999}}) // unknown target: no-op
}

func TestAttachKindString(t *testing.T) {
	if got, want := Attach.String(), "attach"; got != want {
		t.Fatalf("Attach.String() = %q, want %q", got, want)
	}
}

// TestAbilityPushMintsAnActivatedAbilityObject is Task 4's carrier for
// AbilityPush: the ability object is minted inside Apply (no Face, per
// Ruling F3), carries the source's chosen Ability and Source, and an
// out-of-range ability index or invalid player is a no-op.
func TestAbilityPushMintsAnActivatedAbilityObject(t *testing.T) {
	g, id := gameWithOneCardSrc(t, "Name:Sailor\nManaCost:U\nTypes:Creature Spirit\nPT:1/1\nA:AB$ Draw | Cost$ 3 U | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\nOracle:x\n")
	Move(g, id, state.ZHand, state.ZBattlefield)
	Apply(g, Event{Kind: AbilityPush, Obj: id, Player: 0, Amount: 0})
	if len(g.Stack) != 1 {
		t.Fatal("no ability object")
	}
	ab := g.Obj(g.Stack[0])
	if ab.Card != nil || ab.Ability != g.Obj(id).Face().Abilities[0] || ab.Source != id || ab.Controller != 0 {
		t.Fatalf("ability object %+v", ab)
	}
	Apply(g, Event{Kind: AbilityPush, Obj: id, Player: 0, Amount: 7}) // index out of range: no-op
	Apply(g, Event{Kind: AbilityPush, Obj: id, Player: 9, Amount: 0}) // invalid player: no-op
	if len(g.Stack) != 1 {
		t.Fatal("bad AbilityPush minted something")
	}
}

// TestAbilityPushPlayerRefRememberedRoundTrips is FL-41's symmetry check for
// AbilityPush: the same PlayerRef sentinel TriggerPush decodes must decode
// in AbilityPush too, so an activated ability that remembered a player (the
// same shape rules.pushTrigger produces for a DeclareAttackers trigger)
// rebuilds {Player, IsPlayer: true} rather than {Obj: 0}.
func TestAbilityPushPlayerRefRememberedRoundTrips(t *testing.T) {
	g, id := gameWithOneCardSrc(t, "Name:Sailor\nManaCost:U\nTypes:Creature Spirit\nPT:1/1\nA:AB$ Draw | Cost$ 3 U | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\nOracle:x\n")
	Move(g, id, state.ZHand, state.ZBattlefield)
	Apply(g, Event{Kind: AbilityPush, Obj: id, Player: 0, Amount: 0,
		IDs: []state.ObjID{id, state.PlayerRef(1)}})
	if len(g.Stack) != 1 {
		t.Fatal("no ability object")
	}
	ab := g.Obj(g.Stack[0])
	if len(ab.Remembered) != 2 || ab.Remembered[0].Obj != id || ab.Remembered[0].IsPlayer ||
		!ab.Remembered[1].IsPlayer || ab.Remembered[1].Player != 1 {
		t.Fatalf("ability object Remembered %+v", ab.Remembered)
	}
}

func TestAbilityPushKindString(t *testing.T) {
	if got, want := AbilityPush.String(), "ability_push"; got != want {
		t.Fatalf("AbilityPush.String() = %q, want %q", got, want)
	}
}

// TestTargetsChosenAppendShapes is Task 4's carrier for TargetsChosen's two
// new shapes (Amount 2 appends an object target, Amount 3 appends a player
// target); shapes 0 and 1 must keep replacing exactly as before, so every
// pre-Task-4 log still applies unchanged.
func TestTargetsChosenAppendShapes(t *testing.T) {
	g, id := gameWithOneCard(t)
	other := g.AddObject(g.Obj(id).Card, 1).ID
	Apply(g, Event{Kind: PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
	Apply(g, Event{Kind: TargetsChosen, Obj: id, IDs: []state.ObjID{other}})         // replace with objects
	Apply(g, Event{Kind: TargetsChosen, Obj: id, Player: 1, Amount: 3})              // append player
	Apply(g, Event{Kind: TargetsChosen, Obj: id, IDs: []state.ObjID{id}, Amount: 2}) // append object
	tg := g.Obj(id).Targets
	if len(tg) != 3 || tg[0].Obj != other || !tg[1].IsPlayer || tg[1].Player != 1 || tg[2].Obj != id {
		t.Fatalf("targets %+v", tg)
	}
	Apply(g, Event{Kind: TargetsChosen, Obj: id, Player: 0, Amount: 1}) // shape 1 still replaces
	if tg := g.Obj(id).Targets; len(tg) != 1 || !tg[0].IsPlayer {
		t.Fatalf("targets %+v", tg)
	}
}
