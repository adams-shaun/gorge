package rules

// UnlessPayer$ asks several players in APNAP order (CR 101.4): the active
// player first, then each other player in turn order. Vexing Devil's ruling
// of 2018-12-07: "the next opponent in turn order (or, if it's an opponent's
// turn, that opponent) chooses whether to be dealt 4 damage, then each other
// opponent in turn order does the same."

import (
	"slices"
	"testing"
)

// unlessAskSeats is the seats asked a pay/decline mode decision at scenario
// step, in order.
func unlessAskSeats(run *oracleRun, step int) []int {
	var seats []int
	for _, od := range run.decisions {
		if od.Step == step && od.Kind == "mode" {
			seats = append(seats, od.Seat)
		}
	}
	return seats
}

// TestCR101_4UnlessOpponentPayersOnControllersTurn: p2 casts Vexing Devil on
// their own turn, so p3 (the next opponent in turn order) is offered the 4
// damage first, then p0, then p1.
func TestCR101_4UnlessOpponentPayersOnControllersTurn(t *testing.T) {
	run := runFourSeat(t, `{"name":"cr101-4-devil","format":"constructed","players":4,"turn":3,"cr":["101.4"],"why":"unless payers in APNAP order",
"setup":{"p2":{"hand":["Vexing Devil"]}},
"steps":[{"op":"cast","seat":2,"card":"p2:Vexing Devil","mana":"R"},
{"op":"resolve","seat":2,"answers":[{"kind":"modes","pick":["Refuse — it stays"]},{"kind":"modes","pick":["Refuse — it stays"]},{"kind":"modes","pick":["Refuse — it stays"]}]}]}`)
	if run.e.G.Active != 2 {
		t.Fatalf("precondition: active seat %d, want 2", run.e.G.Active)
	}
	if got, want := unlessAskSeats(run, 1), []int{3, 0, 1}; !slices.Equal(got, want) {
		t.Fatalf("payers asked %v, want %v", got, want)
	}
	fourSeatLife(t, run, 20, 20, 20, 20)
}

// TestCR101_4UnlessOpponentPayersOnOpponentsTurn: p2 activates Scarwood
// Bandits on p1's turn, so the active opponent p1 is asked first, then p3,
// then p0 -- neither seat order from 0 nor from the controller.
func TestCR101_4UnlessOpponentPayersOnOpponentsTurn(t *testing.T) {
	run := runFourSeat(t, `{"name":"cr101-4-bandits","format":"constructed","players":4,"turn":2,"cr":["101.4"],"why":"unless payers in APNAP order",
"setup":{"p2":{"battlefield":["Scarwood Bandits"]},"p0":{"battlefield":["Ornithopter"]}},
"steps":[{"op":"pass","seat":1},
{"op":"activate","seat":2,"card":"p2:Scarwood Bandits","mana":"CCG","targets":["p0:Ornithopter"]},
{"op":"resolve","seat":2,"answers":[{"kind":"modes","pick":["Don't pay"]},{"kind":"modes","pick":["Don't pay"]},{"kind":"modes","pick":["Don't pay"]}]}]}`)
	if run.e.G.Active != 1 {
		t.Fatalf("precondition: active seat %d, want 1", run.e.G.Active)
	}
	if got, want := unlessAskSeats(run, 2), []int{1, 3, 0}; !slices.Equal(got, want) {
		t.Fatalf("payers asked %v, want %v", got, want)
	}
}
