package rules

// The oracle runner at four seats (oracleScenario.Players = 4), the Commander
// pod: seats p0..p3 named a..d, p0 starts and draws on turn 1 (CR 103.8a),
// turns pass in seat order (CR 800.1), and a seat that loses leaves the game
// while it goes on (CR 800.4a).

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func runOracleRaw(t *testing.T, raw string) OracleResult {
	t.Helper()
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	if len(res.Snapshots) == 0 {
		t.Fatal("no snapshots")
	}
	return res
}

// oracleFourSeatShock: p0 Shocks p3 (2 life, a Grizzly Bears); p2 holds a
// Memnite that must outlive p3's departure.
const oracleFourSeatShock = `{"name":"four-seat-shock","players":4,"setup":{` +
	`"p0":{"hand":["Shock"]},"p2":{"battlefield":["Memnite"]},"p3":{"life":2,"battlefield":["Grizzly Bears"]}},` +
	`"steps":[{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p3"]},{"op":"resolve","seat":0}]}`

// TestOracleScenarioRunsAtFourSeats: p3's loss to Shock (CR 704.5a) removes
// p3 and everything p3 owns (CR 800.4a); the game goes on for the other three.
func TestOracleScenarioRunsAtFourSeats(t *testing.T) {
	res := runOracleRaw(t, oracleFourSeatShock)
	setup := res.Snapshots[0]
	if !hasSnapPerm(setup, "Grizzly Bears", 3) || !hasSnapPerm(setup, "Memnite", 2) {
		t.Fatalf("setup permanents %+v: want p3's Grizzly Bears and p2's Memnite", setup.Permanents)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	if len(last.Players) != 4 {
		t.Fatalf("players = %d, want 4", len(last.Players))
	}
	if last.Over {
		t.Fatal("game over after one of three opponents lost; CR 800.4a: it goes on")
	}
	for i, want := range []bool{false, false, false, true} {
		if last.Players[i].Lost != want {
			t.Errorf("p%d lost = %v, want %v", i, last.Players[i].Lost, want)
		}
	}
	for _, p := range last.Permanents {
		if p.Name == "Grizzly Bears" {
			t.Errorf("Grizzly Bears still on the battlefield (controller %d) after its owner left", p.Controller)
		}
	}
	if !hasSnapPerm(last, "Memnite", 2) {
		t.Errorf("Memnite under p2 gone: %+v", last.Permanents)
	}
}

func hasSnapPerm(s OracleSnapshot, name string, controller int) bool {
	for _, p := range s.Permanents {
		if p.Name == name && p.Controller == controller {
			return true
		}
	}
	return false
}

// TestOracleFourSeatTurnOrder: turns pass p0, p1, p2, p3 and back to p0
// (CR 800.1, 101.4 seat order), one turn each.
func TestOracleFourSeatTurnOrder(t *testing.T) {
	const upkeep = "upkeep"
	if _, ok := state.ParseStep(upkeep); !ok {
		t.Fatalf("state.ParseStep(%q) unknown", upkeep)
	}
	var steps []string
	for _, seat := range []string{"p1", "p2", "p3", "p0"} {
		steps = append(steps, `{"op":"pass_to","step":"`+upkeep+`","active":"`+seat+`"}`)
	}
	res := runOracleRaw(t, `{"name":"four-seat-turns","players":4,"setup":{},"steps":[`+strings.Join(steps, ",")+`]}`)
	if len(res.Snapshots) != 5 {
		t.Fatalf("snapshots = %d, want setup + 4 steps", len(res.Snapshots))
	}
	if s := res.Snapshots[0]; s.Active != 0 || s.Turn != 1 {
		t.Fatalf("setup: active p%d turn %d, want p0 turn 1", s.Active, s.Turn)
	}
	for i, want := range []struct {
		active int
		turn   int32
	}{{1, 2}, {2, 3}, {3, 4}, {0, 5}} {
		s := res.Snapshots[i+1]
		if s.Active != want.active || s.Turn != want.turn || s.Step != upkeep {
			t.Errorf("step %d: active p%d turn %d step %s, want p%d turn %d %s", i, s.Active, s.Turn, s.Step, want.active, want.turn, upkeep)
		}
	}
}

// TestOracleFourSeatStartingPlayerDraws: in a game of three or more seats the
// starting player skips no draw (CR 103.8a, 800.7), so p0 alone holds a card.
func TestOracleFourSeatStartingPlayerDraws(t *testing.T) {
	res := runOracleRaw(t, `{"name":"four-seat-draw","players":4,"setup":{},"steps":[]}`)
	s := res.Snapshots[0]
	if len(s.Players) != 4 {
		t.Fatalf("players = %d, want 4", len(s.Players))
	}
	for i, want := range []int{1, 0, 0, 0} {
		if got := len(s.Players[i].Hand); got != want {
			t.Errorf("p%d hand = %v, want %d card(s)", i, s.Players[i].Hand, want)
		}
	}
}

// TestOracleTwoSeatBytesUnchanged: the "players" field absent and an explicit
// 2 run the same game to the same result bytes.
func TestOracleTwoSeatBytesUnchanged(t *testing.T) {
	absent, err := json.Marshal(runShockLoss(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	two, err := json.Marshal(runShockLoss(t, `"players":2,`))
	if err != nil {
		t.Fatal(err)
	}
	if len(absent) == 0 || !bytes.Equal(absent, two) {
		t.Fatalf("result bytes differ:\nabsent %s\nplayers 2 %s", absent, two)
	}
}
