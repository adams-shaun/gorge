package searchprobe

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

func syntheticCard(t *testing.T, text string) *cards.Card {
	t.Helper()
	c, ds := cards.ParseBytes("search-fixture", []byte(text))
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

func TestGenesisNameConstraintsAccountForDuplicateCopies(t *testing.T) {
	a := syntheticCard(t, "Name:A\nTypes:Land\nOracle:Fixture.\n")
	b := syntheticCard(t, "Name:B\nTypes:Land\nOracle:Fixture.\n")
	setup := PublicGame{Names: []string{"a", "b"}, Decks: [][]*cards.Card{{a, a, b}, {b, b, b}}}
	h := History{Actor: 0, Frames: []Frame{{Identities: []Identity{{ID: 1, Name: "A"}}, Events: []ObservedEvent{{Kind: events.Note, Text: "a won the toss"}, {Kind: events.Shuffle, Player: 0}, {Kind: events.Draw, Obj: 1}}}}}
	epochs, err := compileEpochs(h)
	if err != nil {
		t.Fatal(err)
	}
	tape, tossWeight, err := publicToss(setup, h)
	if err != nil {
		t.Fatal(err)
	}
	if len(tape) != 1 || tape[0] != (rules.ChanceDraw{Bound: 2, Value: 0}) {
		t.Fatal("public toss not forced")
	}
	seen := make(map[state.ObjID]bool)
	for seed := uint64(0); seed < 100; seed++ {
		p := &proposalState{epochs: epochs, base: seed, observer: NewCollector(0), result: &SampleResult{}, logWeight: tossWeight}
		order, err := p.plan(rules.ShuffleContext{Player: 0, Library: []rules.ShuffleCard{{ID: 1, Name: "A"}, {ID: 2, Name: "A"}, {ID: 3, Name: "B"}}})
		if err != nil {
			t.Fatal(err)
		}
		// P(toss=0)*P(first named A) = 1/2 * 2/3 = 1/3.
		if math.Abs(math.Exp(p.logWeight)-1.0/3) > 1e-12 {
			t.Fatalf("weight %g", math.Exp(p.logWeight))
		}
		if order[0] == 3 {
			t.Fatal("violated A constraint")
		}
		seen[order[0]] = true
	}
	if len(seen) != 2 {
		t.Fatal("duplicate copy lost proposal support")
	}
}

func TestSamplerExecutesLibraryAndRevealHistory(t *testing.T) {
	for _, tc := range []struct {
		name, sa string
		kind     events.Kind
		to       state.Zone
	}{
		{"shuffle", "Shuffle | Defined$ You", events.Shuffle, state.ZLibrary},
		{"look_reorder", "RearrangeTopOfLibrary | Defined$ You | NumCards$ 3", events.LibraryOrder, state.ZLibrary},
		{"search", "ChangeZone | Origin$ Library | Destination$ Hand | ChangeType$ Basic | ChangeNum$ 1 | Shuffle$ True", events.Shuffle, state.ZLibrary},
		{"reveal", "RevealHand | Defined$ Opponent", events.Note, state.ZLibrary},
		{"bounce", "ChangeZoneAll | Origin$ Battlefield | Destination$ Hand | ChangeType$ Land", events.MoveZone, state.ZHand},
		{"return", "ChangeZoneAll | Origin$ Battlefield | Destination$ Library | ChangeType$ Land | LibraryPosition$ 0", events.MoveZone, state.ZLibrary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			land := syntheticCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n")
			spell := syntheticCard(t, "Name:History Experiment\nManaCost:0\nTypes:Sorcery\nA:SP$ "+tc.sa+"\nOracle:Fixture.\n")
			decks := make([][]*cards.Card, 2)
			tape := []rules.ChanceDraw{{Bound: 2, Value: 0}}
			for p := range decks {
				decks[p] = make([]*cards.Card, 20)
				for i := range decks[p] {
					decks[p][i] = land
				}
				for n := 20; n > 1; n-- {
					tape = append(tape, rules.ChanceDraw{Bound: n, Value: n - 1})
				}
			}
			decks[0][0] = spell
			setup := PublicGame{Names: []string{"a", "b"}, Decks: decks}
			e, err := rules.NewHypothetical(rules.Config{Seed: 1, Names: setup.Names, Decks: decks}, tape)
			if err != nil {
				t.Fatal(err)
			}
			if err = e.AdvanceHypothetical(); err != nil {
				t.Fatal(err)
			}
			collector := NewCollector(0)
			h := History{Actor: 0, Answers: make(map[int][]Action)}
			r := rand.New(rand.NewPCG(4, 8))
			pos := 0
			seen := false
			for i := 0; i < 300; i++ {
				f, err := collector.Capture(e, e.L.Events[pos:])
				if err != nil {
					t.Fatal(err)
				}
				h.Frames = append(h.Frames, f)
				if i > 0 {
					for _, ev := range f.Events {
						if ev.Kind == tc.kind && (tc.kind != events.MoveZone || ev.To == tc.to) {
							seen = true
						}
					}
				}
				d := e.Pending()
				if d == nil {
					t.Fatal("ended before history root")
				}
				if seen && len(e.G.Stack) == 0 && d.Player == 0 {
					break
				}
				in := botpolicy.Decide(botpolicy.BoardFromGame(e.G, e, d.Player), d, r)
				if d.Player == 0 {
					h.Answers[i], err = collector.Actions(d, in)
					if err != nil {
						t.Fatal(err)
					}
				}
				pos = len(e.L.Events)
				if err = e.SubmitHypothetical(in); err != nil {
					t.Fatal(err)
				}
			}
			if !seen {
				t.Fatal("fixture never executed requested effect")
			}
			result, err := Sample(setup, h, SampleOptions{Seed: 818, Attempts: 16, Worlds: 4, MaxSubmits: 5000})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Worlds) != 4 {
				t.Fatalf("lost valid effect history: %+v", result)
			}
			for _, w := range result.Worlds {
				if err := VerifyWorld(w); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
