package rules

// The oracle runner's seat count (oracleScenario.Players): absent seats the
// two-player game every scenario but a multiplayer one plays; 3 seats p0 and
// two opponents, where a player who loses the game leaves it (CR 800.4a) and
// the game goes on (CR 104.2a only ends it when one player remains).

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// oracleShockLoss is p0's Shock at p1, who starts at 2 life. %s is the
// scenario's optional "players" member.
const oracleShockLoss = `{"name":"shock-loss",%s"setup":{"p0":{"hand":["Shock"]},"p1":{"life":2}},` +
	`"steps":[{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p1"]},{"op":"resolve","seat":0}]}`

func runShockLoss(t *testing.T, players string) OracleResult {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(strings.Replace(oracleShockLoss, "%s", players, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	return res
}

// TestOracleThreeSeatLossLeavesTheGame: with three seats p1's loss to Shock
// is a state-based action (CR 704.5a) that removes p1 (CR 800.4a) and the
// game goes on for p0 and p2; the checkpoint carries every seat and marks
// p1 lost.
func TestOracleThreeSeatLossLeavesTheGame(t *testing.T) {
	res := runShockLoss(t, `"players":3,`)
	last := res.Snapshots[len(res.Snapshots)-1]
	if len(last.Players) != 3 {
		t.Fatalf("players = %d, want 3", len(last.Players))
	}
	if last.Over {
		t.Fatalf("game over after one of two opponents lost; CR 800.4a: it goes on")
	}
	for i, want := range []bool{false, true, false} {
		if last.Players[i].Lost != want {
			t.Errorf("p%d lost = %v, want %v", i, last.Players[i].Lost, want)
		}
	}
}

// TestOracleTwoSeatLossEndsTheGame: the default two-seat game, with the
// field absent or explicit, ends at the same loss (CR 104.2a).
func TestOracleTwoSeatLossEndsTheGame(t *testing.T) {
	for _, players := range []string{"", `"players":2,`} {
		res := runShockLoss(t, players)
		last := res.Snapshots[len(res.Snapshots)-1]
		if len(last.Players) != 2 || !last.Over || last.Winner == nil || *last.Winner != 0 {
			t.Fatalf("players %q: %d seats, over %v, winner %v; want 2 seats, p0 wins", players, len(last.Players), last.Over, last.Winner)
		}
	}
}

// TestOracleSeatCountRefused: a seat count outside the engine's scenario
// schema (2 to 4) is a harness error, never a silent clamp.
func TestOracleSeatCountRefused(t *testing.T) {
	for _, n := range []int{1, 5, -1} {
		if _, err := oracleSeatCount(n); err == nil {
			t.Errorf("oracleSeatCount(%d) accepted", n)
		}
	}
	for n, want := range map[int]int{0: 2, 2: 2, 3: 3, 4: 4} {
		if got, err := oracleSeatCount(n); err != nil || got != want {
			t.Errorf("oracleSeatCount(%d) = %d, %v; want %d", n, got, err, want)
		}
	}
}
