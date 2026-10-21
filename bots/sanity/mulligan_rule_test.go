package sanity

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/state"
)

// floodBoard is six Plains and a 2-drop: MulliganConstructed mulligans it
// after one London mulligan (six lands is outside its 2..5 window), while
// MulliganForge keeps it because bottoming one Plains leaves 5 of 6.
func floodBoard() botpolicy.Board {
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
	return botpolicy.Board{
		Cards:    botpolicy.TableOf(m),
		HandSize: 7,
		OwnDeck: &deck.Manifest{Main: []deck.ManifestRow{
			{Name: "Plains", Count: 24, Land: true},
			{Name: "Spell", Count: 36},
		}},
	}
}

func floodAsk() decision.Decision {
	return decision.Decision{Kind: decision.KMulligan, Seq: 4, Min: 1, Max: 1,
		Prompt:  "p0 plays first. Keep your hand (put 1 card on the bottom of your library) or take a mulligan?",
		Options: []decision.Option{{Index: 0, Kind: "keep"}, {Index: 1, Kind: "mulligan"}}}
}

func TestWrapMulliganUsesGivenRule(t *testing.T) {
	b := floodBoard()
	d := floodAsk()

	inner := &recBoard{recSeat{pick: "never"}}
	bs, ok := WrapMulligan(inner, botpolicy.MulliganForge).(heuristic.BoardSeat)
	if !ok {
		t.Fatal("a BoardSeat inner must stay a BoardSeat")
	}
	in, err := bs.DecideBoard(context.Background(), b, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("WrapMulligan(forge) = %+v, want keep [0]", in)
	}
	if inner.got.Kind != "" {
		t.Fatal("the wrapped seat was consulted for a mulligan ask")
	}

	inner = &recBoard{recSeat{pick: "never"}}
	bs, ok = Wrap(inner).(heuristic.BoardSeat)
	if !ok {
		t.Fatal("a BoardSeat inner must stay a BoardSeat")
	}
	in, err = bs.DecideBoard(context.Background(), b, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("Wrap (forge) = %+v, want keep [0]", in)
	}
	if inner.got.Kind != "" {
		t.Fatal("the wrapped seat was consulted for a mulligan ask")
	}
}

func TestWrapMulliganUsesGivenRuleWhenItDiffers(t *testing.T) {
	b := floodBoard()
	d := floodAsk()

	inner := &recBoard{recSeat{pick: "never"}}
	bs, ok := WrapMulligan(inner, botpolicy.MulliganConstructed).(heuristic.BoardSeat)
	if !ok {
		t.Fatal("a BoardSeat inner must stay a BoardSeat")
	}
	in, err := bs.DecideBoard(context.Background(), b, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 1 {
		t.Fatalf("WrapMulligan(constructed) = %+v, want mulligan [1]", in)
	}
	if inner.got.Kind != "" {
		t.Fatal("the wrapped seat was consulted for a mulligan ask")
	}
}
