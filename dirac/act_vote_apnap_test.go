package dirac

import "testing"

// TestVoteAPNAP: with four seats and the spell controlled by seat 2, the votes
// are asked of seat 2 first and then in seat order wrapping past the last seat
// (2, 3, 0, 1): gorge Vote's APNAP order from the controller. A seat that has
// lost the game is skipped without shifting the seats after it.
func TestVoteAPNAP(t *testing.T) {
	run := func(t *testing.T, lost int, want []uint8) {
		t.Helper()
		sc, angel, _, _ := voteScen(t)
		sc.s.g.nseat = 4
		sc.s.res.e.ctrl = 2
		if lost >= 0 {
			sc.s.pl[lost].lost = true
		}
		sc.s.runProg(sc.k)
		var got []uint8
		for range want {
			d := sc.s.Step()
			if d.Kind != DkChoose || d.Sub != ChVoteCard {
				t.Fatalf("after asking %v: decision kind %d sub %d, want a card vote", got, d.Kind, d.Sub)
			}
			got = append(got, d.Player)
			sc.do(t, AkChoose, TCode(angel), scAny)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("ask order %v, want %v", got, want)
			}
		}
		if d := sc.s.Step(); d.Kind == DkChoose && d.Sub == ChVoteCard {
			t.Fatalf("a further vote was asked of seat %d after %v", d.Player, want)
		}
	}
	t.Run("all seats", func(t *testing.T) { run(t, -1, []uint8{2, 3, 0, 1}) })
	t.Run("lost seat skipped", func(t *testing.T) { run(t, 3, []uint8{2, 0, 1}) })
}
