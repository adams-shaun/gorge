package rules

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// ChooseNumber Random$ True (Tibalt's Trickery, Murgish Cemetery, Hapatos
// Might) is the card's own "choose at random": the chooser is never asked,
// and exactly one Choose event records the drawn number inside the card's
// Min$/Max$ range. Before this arm the ask was posed, letting a player pick.
func TestChooseNumberRandomSkipsTheAskKernel(t *testing.T) {
	t.Parallel()
	e, src, mk := chooseSourceBoard(t)
	start := len(e.L.Events)
	d := runProbeSA(t, e, parseFixtureSA(t, "SP$ ChooseNumber | Defined$ You | Min$ 1 | Max$ 3 | Random$ True"), mk, nil)
	if d != nil {
		t.Fatalf("Random$ True posed an ask: %+v", d)
	}
	evs := chooseEventsFor(eventsSince(e, start), "number")
	if len(evs) != 1 {
		t.Fatalf("Choose events = %+v, want exactly one", evs)
	}
	if evs[0].Obj != src || evs[0].Amount < 1 || evs[0].Amount > 3 {
		t.Fatalf("Choose event = %+v, want source %d and a number in 1..3", evs[0], src)
	}
	if got := e.G.Obj(src).ChosenNumber; got != evs[0].Amount {
		t.Fatalf("ChosenNumber = %d, want the drawn %d", got, evs[0].Amount)
	}
}

// chooseNumberRandomCorpusSA finds the card's ChooseNumber SA: on the
// ability chains first, then over the sorted SVars.
func chooseNumberRandomCorpusSA(t *testing.T, cardName string) *cards.SA {
	t.Helper()
	c := corpusCardNamed(t, cardName)
	for _, face := range c.Faces {
		for _, ab := range face.Abilities {
			for sub := ab; sub != nil; sub = sub.Sub {
				if sub.API == "ChooseNumber" {
					return sub
				}
			}
		}
		for _, name := range sortedKeysOf(face.SVars) {
			if sa := cards.ResolveSVar(face.SVars, name); sa != nil && sa.API == "ChooseNumber" {
				return sa
			}
		}
	}
	t.Fatalf("corpus pin moved: %q has no ChooseNumber", cardName)
	return nil
}

// TestChooseNumberRandomCorpusCarriersKernel: every corpus card that spells
// a resolution-time "chosen at random" number draws it from the seeded rng
// inside its own Min$/Max$ and never asks. The non-random fallback records
// Min every time, so two distinct draws across eight proves the rng is read.
func TestChooseNumberRandomCorpusCarriersKernel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		card     string
		min, max int
	}{
		{"Murgish Cemetery", 2, 6},
		{"Hapato's Might", 0, 6},
		{"Tibalt's Trickery", 1, 3},
		{"Saji's Torrent", 0, 5},
	} {
		t.Run(tc.card, func(t *testing.T) {
			t.Parallel()
			sa := chooseNumberRandomCorpusSA(t, tc.card)
			if sa.Params["Random"] != "True" || sa.Params["Min"] != strconv.Itoa(tc.min) || sa.Params["Max"] != strconv.Itoa(tc.max) {
				t.Fatalf("corpus pin moved: %s ChooseNumber params = %v, want Random True Min %d Max %d", tc.card, sa.Params, tc.min, tc.max)
			}
			sa = isolateSA(sa)
			e, src, mk := chooseSourceBoard(t)
			seen := map[int]bool{}
			for i := 0; i < 8; i++ {
				start := len(e.L.Events)
				if d := runProbeSA(t, e, sa, mk, nil); d != nil {
					t.Fatalf("draw %d: Random$ True posed an ask: %+v", i, d)
				}
				evs := chooseEventsFor(eventsSince(e, start), "number")
				if len(evs) != 1 {
					t.Fatalf("draw %d: Choose events = %+v, want exactly one", i, evs)
				}
				n := int(evs[0].Amount)
				if evs[0].Obj != src || n < tc.min || n > tc.max {
					t.Fatalf("draw %d: Choose event = %+v, want source %d and a number in %d..%d", i, evs[0], src, tc.min, tc.max)
				}
				if got := e.G.Obj(src).ChosenNumber; got != int32(n) {
					t.Fatalf("draw %d: ChosenNumber = %d, want the drawn %d", i, got, n)
				}
				seen[n] = true
			}
			if len(seen) < 2 {
				t.Fatalf("8 draws gave only %v; the seeded rng is not read", seen)
			}
		})
	}
}
