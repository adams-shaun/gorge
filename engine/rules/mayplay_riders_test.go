package rules

// mayplay_riders_test.go pins the printed may-play riders cr-g129 consumes:
// MayPlayPlayer$ Player / ActivePlayer / Player.Active (Aluren, Weftwalking:
// the beneficiary is every player / the active player, not the static's
// controller) and MayPlayWithFlash$ True (Azula, Cunning Usurper; Aluren:
// "as though it had flash" lifts the sorcery-timing restriction for the cards
// the grant names, CR 601.3 / CR 702.8a).

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// mpRiderGame builds a two-seat game whose decks hold the named corpus cards
// (hand0 / bf0 for seat 0, hand1 / bf1 for seat 1) over Mountains, so every
// card is a logged draw and replayCheck can rebuild the game from the log. The
// cards are moved to their seat's hand (logged), and the bf lists then onto
// its battlefield. It returns the engine, its Config and the ids by seat and
// name.
func mpRiderGame(t *testing.T, hand0, hand1, bf0, bf1 []string) (*Engine, Config, [2]map[string][]state.ObjID) {
	t.Helper()
	want := [2][2][]string{{hand0, bf0}, {hand1, bf1}}
	decks := make([][]*cards.Card, 2)
	for p := range decks {
		for _, names := range want[p] {
			for _, n := range names {
				decks[p] = append(decks[p], corpusAlternativeCard(t, n))
			}
		}
		decks[p] = append(decks[p], mountainDeck(t, 40-len(decks[p]))...)
	}
	cfg := seatZeroStart(Config{Seed: 1, Names: []string{"a", "b"}, Decks: decks,
		Tokens: testutil.CorpusRegistry(t).Tokens})
	e := New(cfg)
	e.Advance()
	var ids [2]map[string][]state.ObjID
	var onBF []state.ObjID
	for p := state.PlayerID(0); p < 2; p++ {
		ids[p] = map[string][]state.ObjID{}

		for li, names := range want[p] {
			for _, n := range names {
				var got state.ObjID
				for _, id := range e.G.Zone(state.ZHand, p) {
					if e.G.Obj(id).Face().Name == n && !slices.Contains(ids[p][n], id) {
						got = id
						break
					}
				}
				if got == 0 {
					for _, id := range e.G.Zone(state.ZLibrary, p) {
						if e.G.Obj(id).Face().Name == n && !slices.Contains(ids[p][n], id) {
							got = id
							e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
							break
						}
					}
				}
				if got == 0 {
					t.Fatalf("%s not found for seat %d", n, p)
				}
				ids[p][n] = append(ids[p][n], got)
				if li == 1 {
					onBF = append(onBF, got)
				}
			}
		}
	}
	// Seat 1's permanents enter first, so an ETB that looks at the
	// opponent's board (Azula) sees them.
	for _, id := range slices.Backward(onBF) {
		moveToBattlefield(t, e, id)
	}
	return e, cfg, ids
}

// mpRiderDrive passes priority to turn/active/step, answering a cleanup
// discard naively and a declare-attackers / blockers decision with none.
func mpRiderDrive(t *testing.T, e *Engine, turn int32, active state.PlayerID, step state.Step) {
	t.Helper()
	for i := 0; i < 4000; i++ {
		if e.G.Turn == turn && e.G.Active == active && e.G.Step == step {
			return
		}
		if answerIfDiscard(t, e) {
			continue
		}
		d := e.Pending()
		if d == nil || e.G.Over {
			t.Fatalf("no decision while driving to turn %d seat %d step %s (at turn %d step %s)", turn, active, step, e.G.Turn, e.G.Step)
		}
		if d.Kind == decision.KAttackers || d.Kind == decision.KBlockers {
			submitChoices(t, e)
			continue
		}
		castFirst(t, e, "pass")
	}
	t.Fatalf("did not reach turn %d seat %d step %s", turn, active, step)
}

// mpRiderOffer returns the priority option casting id, or false.
func mpRiderOffer(e *Engine, id state.ObjID) (decision.Option, bool) {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return decision.Option{}, false
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			return o, true
		}
	}
	return decision.Option{}, false
}

// mpRiderLabels lists the pending priority option labels (failure output).
func mpRiderLabels(e *Engine) []string {
	var out []string
	if d := e.Pending(); d != nil {
		for _, o := range d.Options {
			out = append(out, o.Label)
		}
	}
	return out
}

// mpRiderCastResolve casts id from the pending priority decision and resolves it.
func mpRiderCastResolve(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	o, ok := mpRiderOffer(e, id)
	if !ok {
		t.Fatalf("no cast offer for %d: %+v", id, mpRiderLabels(e))
	}
	submitChoices(t, e, o.Index)
	if got := e.G.Obj(id); got.Zone != state.ZStack {
		t.Fatalf("cast object zone %s, want stack (pending %+v)", got.Zone, e.Pending())
	}
	e.resolveTop()
	e.priorityRound()
}

func TestMayPlayPlayerActiveWeftwalking(t *testing.T) {
	t.Run("first spell of each player's own turn is free", func(t *testing.T) {
		e, cfg, ids := mpRiderGame(t,
			[]string{"Grizzly Bears", "Grizzly Bears"}, []string{"Grizzly Bears"},
			[]string{"Weftwalking"}, nil)
		driveToTurn3Main(t, e)
		if len(e.G.Players[0].Pool) != 0 && e.G.Players[0].Pool.Total() != 0 {
			t.Fatalf("precondition: pool not empty: %v", e.G.Players[0].Pool)
		}
		b0, b1 := ids[0]["Grizzly Bears"][0], ids[0]["Grizzly Bears"][1]
		if _, ok := mpRiderOffer(e, b0); !ok {
			t.Fatalf("Bears not offered free on seat 0's turn: %+v", mpRiderLabels(e))
		}
		mpRiderCastResolve(t, e, b0)
		if e.G.Obj(b0).Zone != state.ZBattlefield {
			t.Fatalf("free Bears zone %s, want battlefield", e.G.Obj(b0).Zone)
		}
		if _, ok := mpRiderOffer(e, b1); ok {
			t.Fatalf("second spell offered free: %+v", mpRiderLabels(e))
		}
		// Seat 1's turn: seat 1's first spell is free too (the beneficiary is
		// the active player, not Weftwalking's controller).
		mpRiderDrive(t, e, 4, 1, state.StepMain1)
		if e.G.Players[1].Pool.Total() != 0 {
			t.Fatalf("precondition: seat 1's pool not empty")
		}
		if _, ok := mpRiderOffer(e, ids[1]["Grizzly Bears"][0]); !ok {
			t.Fatalf("seat 1's first spell not offered free on its own turn: %+v", mpRiderLabels(e))
		}
		replayCheck(t, e, cfg)
	})
	t.Run("a graveyard card is never offered", func(t *testing.T) {
		e, cfg, ids := mpRiderGame(t,
			[]string{"Grizzly Bears", "Grizzly Bears"}, nil,
			[]string{"Weftwalking"}, nil)
		driveToTurn3Main(t, e)
		inHand, inYard := ids[0]["Grizzly Bears"][0], ids[0]["Grizzly Bears"][1]
		moveToGraveyard(t, e, inYard)
		e.priorityRound()
		if e.G.Obj(inYard).Zone != state.ZGraveyard {
			t.Fatalf("precondition: Bears in %s, want graveyard", e.G.Obj(inYard).Zone)
		}
		if _, ok := mpRiderOffer(e, inHand); !ok {
			t.Fatalf("precondition: hand Bears not offered: %+v", mpRiderLabels(e))
		}
		if _, ok := mpRiderOffer(e, inYard); ok {
			t.Fatalf("graveyard Bears offered (MayPlayDontGrantZonePermissions$ widened)")
		}
		replayCheck(t, e, cfg)
	})
}

// mpRiderSettle answers any non-priority decision with its first option and
// passes priority until the stack is empty and the seat is at priority.
func mpRiderSettle(t *testing.T, e *Engine) {
	t.Helper()
	e.priorityRound()
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no pending decision while settling")
		}
		if d.Kind != decision.KPriority {
			submitChoices(t, e, 0)
			continue
		}
		if len(e.G.Stack) == 0 {
			return
		}
		castFirst(t, e, "pass")
	}
	t.Fatalf("stack did not settle: %v", e.G.Stack)
}

// mpRiderAzula puts Azula on seat 0's battlefield with seat 1's Grizzly Bears
// exiled by its real ETB (so ExiledWith is Azula's), seat 0 holding a hand
// Grizzly Bears, and returns the exiled Bears.
func mpRiderAzula(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	e, cfg, ids := mpRiderGame(t,
		[]string{"Grizzly Bears"}, nil,
		[]string{"Azula, Cunning Usurper"}, []string{"Grizzly Bears"})
	mpRiderSettle(t, e)
	exiled := ids[1]["Grizzly Bears"][0]
	o := e.G.Obj(exiled)
	if o.Zone != state.ZExile || o.ExiledWith != ids[0]["Azula, Cunning Usurper"][0] {
		t.Fatalf("precondition: Azula's ETB did not exile seat 1's Bears (zone %s, ExiledWith %d)", o.Zone, o.ExiledWith)
	}
	return e, cfg, exiled, ids[0]["Grizzly Bears"][0]
}

func TestMayPlayWithFlashTimingAzula(t *testing.T) {
	t.Run("offered and resolves at beginning of combat", func(t *testing.T) {
		e, cfg, exiled, _ := mpRiderAzula(t)
		mpRiderDrive(t, e, 3, 0, state.StepBeginCombat)
		addManaQuiet(t, e, 0, "GG")
		e.priorityRound()
		if e.G.Step != state.StepBeginCombat || e.G.Priority != 0 {
			t.Fatalf("precondition: step %s priority %d", e.G.Step, e.G.Priority)
		}
		mpRiderCastResolve(t, e, exiled)
		if o := e.G.Obj(exiled); o.Zone != state.ZBattlefield || o.Controller != 0 {
			t.Fatalf("cast Bears zone %s controller %d, want battlefield under seat 0", o.Zone, o.Controller)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("not offered on the opponent's turn", func(t *testing.T) {
		e, cfg, exiled, _ := mpRiderAzula(t)
		mpRiderDrive(t, e, 4, 1, state.StepBeginCombat)
		addManaQuiet(t, e, 0, "GG")
		addManaQuiet(t, e, 1, "GG")
		e.priorityRound()
		if e.G.Active != 1 {
			t.Fatalf("precondition: active seat %d, want 1", e.G.Active)
		}
		for p := state.PlayerID(0); p < 2; p++ {
			if e.G.Priority != p {
				continue
			}
			if _, ok := mpRiderOffer(e, exiled); ok {
				t.Fatalf("exiled Bears offered to seat %d on seat 1's turn", p)
			}
		}
		replayCheck(t, e, cfg)
	})
}

func TestMayPlayWithFlashOnlyThroughGrant(t *testing.T) {
	e, cfg, exiled, hand := mpRiderAzula(t)
	mpRiderDrive(t, e, 3, 0, state.StepBeginCombat)
	addManaQuiet(t, e, 0, "GG")
	e.priorityRound()
	if _, ok := mpRiderOffer(e, exiled); !ok {
		t.Fatalf("precondition: the granted exiled Bears is not offered: %+v", mpRiderLabels(e))
	}
	if o := e.G.Obj(hand); o.Zone != state.ZHand {
		t.Fatalf("precondition: hand Bears in %s", o.Zone)
	}
	if _, ok := mpRiderOffer(e, hand); ok {
		t.Fatalf("hand Bears offered at instant timing: flash leaked past the grant")
	}
	replayCheck(t, e, cfg)
}

func TestMayPlayPlayerAnyAluren(t *testing.T) {
	e, cfg, ids := mpRiderGame(t, nil, []string{"Centaur Courser"}, []string{"Aluren"}, nil)
	mpRiderDrive(t, e, 4, 1, state.StepBeginCombat)
	e.priorityRound()
	if e.G.Active != 1 || e.G.Priority != 1 || e.G.Players[1].Pool.Total() != 0 {
		t.Fatalf("precondition: active %d priority %d pool %d", e.G.Active, e.G.Priority, e.G.Players[1].Pool.Total())
	}
	c := ids[1]["Centaur Courser"][0]
	if o := e.G.Obj(c); o.Zone != state.ZHand {
		t.Fatalf("precondition: Courser in %s", o.Zone)
	}
	mpRiderCastResolve(t, e, c)
	if o := e.G.Obj(c); o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("free Courser zone %s controller %d", o.Zone, o.Controller)
	}
	replayCheck(t, e, cfg)
}

// TestMayPlayWithFlashAlternativeCostOnly: Aluren's flash covers the free cast,
// not a paid cast of the same card. With the mana to pay for it, seat 1 at
// instant timing is offered the alternative-cost cast alone.
func TestMayPlayWithFlashAlternativeCostOnly(t *testing.T) {
	e, cfg, ids := mpRiderGame(t, nil, []string{"Centaur Courser"}, []string{"Aluren"}, nil)
	mpRiderDrive(t, e, 4, 1, state.StepBeginCombat)
	addManaQuiet(t, e, 1, "GGG")
	e.priorityRound()
	c := ids[1]["Centaur Courser"][0]
	if e.G.Active != 1 || e.G.Priority != 1 || e.G.Players[1].Pool.Total() != 3 {
		t.Fatalf("precondition: active %d priority %d pool %d", e.G.Active, e.G.Priority, e.G.Players[1].Pool.Total())
	}
	var offers []decision.Option
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == c {
			offers = append(offers, o)
		}
	}
	if len(offers) != 1 || offers[0].AltCostIndex == 0 {
		t.Fatalf("want exactly the alternative-cost cast, got %+v (%v)", offers, mpRiderLabels(e))
	}
	submitChoices(t, e, offers[0].Index)
	e.resolveTop()
	e.priorityRound()
	if o := e.G.Obj(c); o.Zone != state.ZBattlefield || e.G.Players[1].Pool.Total() != 3 {
		t.Fatalf("free cast: Courser in %s, pool %d (want battlefield, 3)", o.Zone, e.G.Players[1].Pool.Total())
	}
	replayCheck(t, e, cfg)
}

func TestMayPlayRiderCorpusCarriers(t *testing.T) {
	players := map[string]bool{}
	flash := map[string]bool{}
	for _, c := range testutil.CorpusRegistry(t).AllCards() {
		for _, f := range c.Faces {
			for i := range f.Statics {
				st := &f.Statics[i]
				switch st.Params["MayPlayPlayer"] {
				case "Player", "ActivePlayer", "Player.Active":
					players[f.Name] = true
				}
				if st.Params["MayPlayWithFlash"] == "True" {
					flash[f.Name] = true
				}
			}
		}
	}
	keys := func(m map[string]bool) []string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	wantPlayers := []string{"Aluren", "Ian Malcolm, Chaotician", "Share the Spoils", "Weftwalking"}
	wantFlash := []string{"Aluren", "Azula, Cunning Usurper", "Elsha of the Infinite", "Primal Prayers", "Qasali Ambusher"}
	if got := keys(players); !slices.Equal(got, wantPlayers) {
		t.Errorf("MayPlayPlayer$ Player/ActivePlayer/Player.Active carriers = %v, want %v", got, wantPlayers)
	}
	if got := keys(flash); !slices.Equal(got, wantFlash) {
		t.Errorf("MayPlayWithFlash$ True carriers = %v, want %v", got, wantFlash)
	}
	union := map[string]bool{}
	for _, m := range []map[string]bool{players, flash} {
		for k := range m {
			union[k] = true
		}
	}
	if len(union) != 8 {
		t.Errorf("distinct carrier files = %d (%v), want 8", len(union), keys(union))
	}
}
