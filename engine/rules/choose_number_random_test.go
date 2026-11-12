package rules

import "testing"

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
