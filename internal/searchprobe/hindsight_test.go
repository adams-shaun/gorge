package searchprobe

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

func TestHindsightArrangeIntentRoundTripsBothOrderedPiles(t *testing.T) {
	collector := NewCollector(0)
	d := &decision.Decision{Seq: 4, Player: 0, Kind: decision.KArrange, Min: 1, Max: 1, Restable: true, Options: []decision.Option{
		{Index: 0, Kind: "bottom", Label: "A"},
		{Index: 1, Kind: "bottom", Label: "B"},
		{Index: 2, Kind: "bottom", Label: "C"},
	}}
	want := decision.Intent{Seq: 4, Player: 0, Choices: []int{1}, Rest: []int{2, 0}}
	choices, rest, err := collector.IntentActions(d, want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := collector.MatchIntent(d, choices, rest)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got.Choices, got.Rest) != fmt.Sprint(want.Choices, want.Rest) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestHindsightCloneContinuationIsByteIdentical(t *testing.T) {
	e := observationEngine(t, 311)
	clone := e.Clone()
	r := rand.New(rand.NewPCG(91, 27))
	for step := 0; step < 30 && !e.G.Over; step++ {
		d := e.Pending()
		if d == nil || clone.Pending() == nil {
			t.Fatal("missing decision")
		}
		in := botpolicy.Decide(botpolicy.BoardFromGame(e.G, e, d.Player), d, r)
		if err := e.SubmitHypothetical(in); err != nil {
			t.Fatal(err)
		}
		if err := clone.SubmitHypothetical(in); err != nil {
			t.Fatal(err)
		}
		if e.L.Head() != clone.L.Head() || e.RNGDraws() != clone.RNGDraws() || len(e.L.Events) != len(clone.L.Events) {
			t.Fatalf("clone diverged after continuation answer %d", step)
		}
	}
}

func TestHindsightResamplingDoesNotReuseTrueHiddenState(t *testing.T) {
	setup, actual, history := distinctOpeningHistory(t)
	truth := hiddenSignature(actual, history.Actor)
	different := false
	for seed := uint64(1); seed <= 8 && !different; seed++ {
		result, err := Sample(setup, history, SampleOptions{Seed: seed, Attempts: 32, Worlds: 4, MaxSubmits: 5000})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Worlds) == 0 {
			continue
		}
		for _, world := range result.Worlds {
			if got := hiddenSignature(world.Engine, history.Actor); got != truth {
				different = true
				break
			}
		}
	}
	if !different {
		t.Fatal("all sampled worlds reproduced the actual opponent hand and both unseen library orders")
	}
}

func distinctOpeningHistory(t *testing.T) (PublicGame, *rules.Engine, History) {
	t.Helper()
	decks := make([][]*cards.Card, 2)
	for seat := range decks {
		for i := 0; i < 24; i++ {
			name := fmt.Sprintf("Hidden %d-%02d", seat, i)
			card, diagnostics := cards.ParseBytes(name, []byte("Name:"+name+"\nTypes:Basic Land Mountain\nOracle:Fixture.\n"))
			if len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			card.Link()
			for _, face := range card.Faces {
				face.ApplyIntrinsics()
			}
			decks[seat] = append(decks[seat], card)
		}
	}
	setup := PublicGame{Names: []string{"a", "b"}, Decks: decks}
	e := rules.New(rules.Config{Seed: 777, Names: setup.Names, Decks: setup.Decks})
	e.Advance()
	collector := NewCollector(0)
	frame, err := collector.Capture(e, e.L.Events)
	if err != nil {
		t.Fatal(err)
	}
	return setup, e, History{Actor: 0, Frames: []Frame{frame}, Answers: map[int][]Action{}}
}

func hiddenSignature(e *rules.Engine, actor state.PlayerID) string {
	var parts []string
	for p := range e.G.Players {
		player := state.PlayerID(p)
		zones := []state.Zone{state.ZLibrary}
		if player != actor {
			zones = append(zones, state.ZHand)
		}
		for _, zone := range zones {
			for _, id := range e.G.Zone(zone, player) {
				o := e.G.Obj(id)
				if o != nil && o.Card != nil && len(o.Card.Faces) > 0 {
					parts = append(parts, fmt.Sprintf("%d/%d/%s", p, zone, o.Card.Faces[0].Name))
				}
			}
		}
	}
	return strings.Join(parts, "|")
}
