package forgeheuristic

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestForgeHeuristicCommonSenseKeepsForgeMulligan pins that the common-sense
// wrap keeps the policy's forge mulligan: the six-Plains-plus-a-spell hand
// after one mulligan is a keep under forge. Since FORGE-M1 the guard's own
// default is forge too, so the unwrapped-default control keeps the same hand.
func TestForgeHeuristicCommonSenseKeepsForgeMulligan(t *testing.T) {
	land := botpolicy.Card{Basic: true, ManaCost: "no cost", Castable: true}
	spell := botpolicy.Card{Creature: true, CMC: 2, ManaCost: "1 W", Castable: true}
	m := map[state.ObjID]botpolicy.Card{}
	for i := 0; i < 7; i++ {
		if i < 6 {
			m[state.ObjID(i+1)] = land
		} else {
			m[state.ObjID(i+1)] = spell
		}
	}
	b := botpolicy.Board{
		Cards:    botpolicy.TableOf(m),
		HandSize: 7,
		OwnDeck: &deck.Manifest{Main: []deck.ManifestRow{
			{Name: "Plains", Count: 24, Land: true},
			{Name: "Spell", Count: 36},
		}},
	}
	d := decision.Decision{Kind: decision.KMulligan, Seq: 4, Min: 1, Max: 1,
		Prompt:  "p0 plays first. Keep your hand (put 1 card on the bottom of your library) or take a mulligan?",
		Options: []decision.Option{{Index: 0, Kind: "keep"}, {Index: 1, Kind: "mulligan"}}}

	forge, err := bots.New(Policy, bots.Options{Seed: 1, CommonSense: true})
	if err != nil {
		t.Fatalf("bots.New(%q): %v", Policy, err)
	}
	fb, ok := forge.(heuristic.BoardSeat)
	if !ok {
		t.Fatalf("forge-heuristic CommonSense seat is %T, want a BoardSeat", forge)
	}
	in, err := fb.DecideBoard(context.Background(), b, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("forge-heuristic CommonSense = %+v, want keep [0]", in)
	}

	base, err := bots.New(bots.Default, bots.Options{Seed: 1, CommonSense: true})
	if err != nil {
		t.Fatalf("bots.New(%q): %v", bots.Default, err)
	}
	bb, ok := base.(heuristic.BoardSeat)
	if !ok {
		t.Fatalf("default CommonSense seat is %T, want a BoardSeat", base)
	}
	in, err = bb.DecideBoard(context.Background(), b, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("default CommonSense = %+v, want keep [0] under forge", in)
	}
}
