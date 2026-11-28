package rules

// Four-seat multiplayer conformance (CR 800, 802, 806): each test is a
// "players": 4 constructed scenario driven through player actions, read back
// from the run's snapshots, recorded decisions and final game state.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// runFourSeat runs raw and fails the test on any scenario failure.
func runFourSeat(t *testing.T, raw string) *oracleRun {
	t.Helper()
	run, fails := runOracleCommander(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	return run
}

// fourSeatRef is the bound object id of ref, failing the test when unbound.
func fourSeatRef(t *testing.T, run *oracleRun, ref string) state.ObjID {
	t.Helper()
	id, err := run.resolve(ref)
	if err != nil || id == 0 {
		t.Fatalf("precondition: %s did not bind: %v", ref, err)
	}
	return id
}

// fourSeatLife asserts every seat's life total.
func fourSeatLife(t *testing.T, run *oracleRun, want ...int32) {
	t.Helper()
	if len(run.e.G.Players) != 4 {
		t.Fatalf("precondition: %d seats, want 4", len(run.e.G.Players))
	}
	for i, w := range want {
		if got := run.e.G.Players[i].Life; got != w {
			t.Errorf("p%d life = %d, want %d", i, got, w)
		}
	}
}

// offersAttack reports whether the pending attackers decision offers obj
// attacking player p.
func offersAttack(d *decision.Decision, obj state.ObjID, p state.PlayerID) bool {
	for _, o := range d.Options {
		if o.Obj == obj && o.Battle == 0 && o.Player == p {
			return true
		}
	}
	return false
}

// offersBlock reports whether the pending blockers decision offers blocker
// b on attacker a.
func offersBlock(d *decision.Decision, b, a state.ObjID) bool {
	for _, o := range d.Options {
		if o.Obj == b && o.Attacker == a {
			return true
		}
	}
	return false
}

// TestCR802_3EachAttackerChoosesItsDefender: CR 802.2/802.3 -- with the
// attack-multiple-players option each attacking creature attacks the one
// defending player its controller chose for it; one Bears at each opponent
// deals 2 to each.
func TestCR802_3EachAttackerChoosesItsDefender(t *testing.T) {
	run := runFourSeat(t, `{"name":"cr802-3","format":"constructed","players":4,"cr":["802.3"],"why":"each attacker at its own defender",
"setup":{"p0":{"battlefield":["Grizzly Bears","Grizzly Bears","Grizzly Bears"]}},
"steps":[{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears","p0:Grizzly Bears#2","p0:Grizzly Bears#3"],"defenders":["p1","p2","p3"]},
{"op":"pass_to","seat":0,"step":"main2"}]}`)
	if run.e.G.Step != state.StepMain2 {
		t.Fatalf("precondition: step %v, want main2", run.e.G.Step)
	}
	fourSeatLife(t, run, 20, 18, 18, 18)
}

// TestCR802_4aOnlyTheAttackedPlayerBlocks: CR 802.4a -- only a defending
// player (one being attacked) may declare blockers, and only against
// creatures attacking that player. p0's Bears attack p2: the one blockers
// decision of the combat is p2's, p2's Giant may block, p1's may not.
func TestCR802_4aOnlyTheAttackedPlayerBlocks(t *testing.T) {
	run := runFourSeat(t, `{"name":"cr802-4a","format":"constructed","players":4,"cr":["802.4a"],"why":"only the attacked player blocks",
"setup":{"p0":{"battlefield":["Grizzly Bears"]},"p1":{"battlefield":["Hill Giant"]},"p2":{"battlefield":["Hill Giant"]}},
"steps":[{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p2"},
{"op":"pass_to","seat":0,"decision":"blockers",
 "expect":[{"can_block":{"blocker":"p2:Hill Giant","attacker":"p0:Grizzly Bears"}},
           {"can_block":{"blocker":"p1:Hill Giant","attacker":"p0:Grizzly Bears"},"want":false}]},
{"op":"pass_to","seat":0,"step":"main2"}]}`)
	var seats []int
	for _, od := range run.decisions {
		if od.Kind == string(decision.KBlockers) {
			seats = append(seats, od.Seat)
		}
	}
	if len(seats) != 1 || seats[0] != 2 {
		t.Fatalf("blockers decisions posed to seats %v, want exactly [2]", seats)
	}
}

// TestCR802_4bBlocksAreJudgedPerDefender: CR 802.4b -- a blocking
// restriction is judged per defending player: p2's creatures never count
// toward a block p1 makes. p0's menace Brute attacks p1 (who has one Giant)
// and the Bears attack p2. p1's lone Giant has no legal block (it cannot
// block the Brute alone, CR 702.111b, nor the Bears, which attack p2), so p1
// is never asked; p2's Giant may block the Bears and not the Brute.
func TestCR802_4bBlocksAreJudgedPerDefender(t *testing.T) {
	const base = `{"name":"cr802-4b","format":"constructed","players":4,"cr":["802.4b"],"why":"menace judged per defender",
"setup":{"p0":{"battlefield":["Boggart Brute","Grizzly Bears"]},"p1":{"battlefield":["Hill Giant"]},"p2":{"battlefield":["Hill Giant"]}},
"steps":[{"op":"attack","seat":0,"attackers":["p0:Boggart Brute","p0:Grizzly Bears"],"defenders":["p1","p2"]},
{"op":"pass_to","seat":0,"decision":"blockers"%s}%s]}`
	at := runFourSeat(t, strings.Replace(strings.Replace(base, "%s",
		`,"expect":[{"can_block":{"blocker":"p2:Hill Giant","attacker":"p0:Grizzly Bears"}},`+
			`{"can_block":{"blocker":"p2:Hill Giant","attacker":"p0:Boggart Brute"},"want":false}]`, 1), "%s", "", 1))
	d := at.e.Pending()
	if d == nil || d.Kind != decision.KBlockers || d.Player != 2 {
		t.Fatalf("first blockers decision = %v, want p2's (p1 has no legal block)", d)
	}
	g1 := fourSeatRef(t, at, "p1:Hill Giant")
	brute := fourSeatRef(t, at, "p0:Boggart Brute")
	if offersBlock(d, g1, brute) {
		t.Errorf("p1's Giant offered in p2's blockers decision")
	}

	// The whole combat: p1 is never posed a blockers decision, so the Brute
	// connects for 3; the declaration itself put each attacker at its seat.
	run := runFourSeat(t, strings.Replace(strings.Replace(base, "%s", "", 1), "%s",
		`,{"op":"block","seat":2,"blocks":[]},{"op":"pass_to","seat":0,"step":"main2"}`, 1))
	for _, od := range run.decisions {
		if od.Kind == string(decision.KBlockers) && od.Seat == 1 {
			t.Errorf("p1 posed a blockers decision with only a lone Giant against menace: %+v", od)
		}
	}
	fourSeatLife(t, run, 20, 17, 18, 20)
}

// TestCR806_2AnyOpponentMayBeAttackedAndTargeted: CR 806.2 -- free-for-all
// has no range of influence limit (806.2a) and a player may attack any
// opponent (806.2b): p0 Shocks p2, who is not adjacent to p0 at a four-seat
// table, and attacks p2 with the Bears.
func TestCR806_2AnyOpponentMayBeAttackedAndTargeted(t *testing.T) {
	run := runFourSeat(t, `{"name":"cr806-2","format":"constructed","players":4,"cr":["806.2"],"why":"any opponent targetable and attackable",
"setup":{"p0":{"battlefield":["Grizzly Bears"],"hand":["Shock"]}},
"steps":[{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p2"]},{"op":"resolve","seat":0},
{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p2"},
{"op":"pass_to","seat":0,"step":"main2"}]}`)
	fourSeatLife(t, run, 20, 20, 16, 20)
}

// TestCR800_4fDepartedPlayerPaysNoCost: CR 800.4f -- a player who left the
// game cannot pay a cost an object or effect asks of them. p1 casts Memnite
// into p0's Rhystic Study; p0 Shocks p1 out with the trigger on the stack;
// the trigger resolves and p0 draws, since no one remains to pay {1}.
// p1 is posed no decision: the departed player is no payer
// (effects.unlessPayerTargetsFor), and nobody is charged in their place.
func TestCR800_4fDepartedPlayerPaysNoCost(t *testing.T) {
	run := runFourSeat(t, `{"name":"cr800-4f","format":"constructed","players":4,"turn":2,"cr":["800.4f"],"why":"a departed player pays no cost",
"setup":{"p0":{"battlefield":["Rhystic Study"],"hand":["Shock"]},"p1":{"life":2,"hand":["Memnite"]}},
"steps":[{"op":"cast","seat":1,"card":"p1:Memnite","expect":[{"trigger_on_stack":"p0:Rhystic Study"}]},
{"op":"pass","seat":1},{"op":"pass","seat":2},{"op":"pass","seat":3},
{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p1"]},
{"op":"resolve","seat":0}]}`)
	g := run.e.G
	if !g.Players[1].Lost {
		t.Fatalf("p1 lost = false, want true")
	}
	if g.Over {
		t.Fatalf("game over with three players left (CR 800.4a)")
	}
	var asked []string
	for _, od := range run.decisions {
		// Step 5 resolves the stack: neither p1 nor a stand-in payer may
		// be asked there.
		if od.Step == 5 && od.Kind == "mode" {
			asked = append(asked, fmt.Sprintf("p%d %s %s", od.Seat, od.Kind, od.First))
		}
	}
	if len(asked) != 0 {
		t.Errorf("unless-pay decisions at the trigger's resolution = %v; want none (CR 800.4f)", asked)
	}
	if n := len(g.Stack); n != 0 {
		t.Errorf("precondition: stack depth = %d, want 0 (the Study trigger resolved)", n)
	}
	setup := run.snaps[0]
	before := len(setup.Players[0].Hand)
	if got := len(g.Zone(state.ZHand, 0)); got != before {
		// Shock left the hand, the Study's draw replaced it.
		t.Errorf("p0 hand = %d, want %d (Shock cast, one card drawn from the Study)", got, before)
	}
	if n := len(g.Zone(state.ZGraveyard, 0)); n != 1 {
		t.Errorf("precondition: p0 graveyard = %d, want 1 (the Shock)", n)
	}
}

// fourSeatDecorum is p1's Disrupt Decorum goading p0's Bears on turn 2, then
// p0's Shock removing p1; %s is the pass_to step after it.
const fourSeatDecorum = `{"name":"cr800-4m","format":"constructed","players":4,"turn":2,"cr":["800.4m"],"why":"a duration ends at the departed player's next turn",
"setup":{"p0":{"battlefield":["Grizzly Bears"],"hand":["Shock"]},"p1":{"life":2,"hand":["Disrupt Decorum"]}},
"steps":[{"op":"cast","seat":1,"card":"p1:Disrupt Decorum","mana":"RRRR"},{"op":"resolve","seat":1},
{"op":"pass","seat":1},{"op":"pass","seat":2},{"op":"pass","seat":3},
{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p1"]},{"op":"resolve","seat":0}%s]}`

// TestCR800_4mDurationEndsAtTheDepartedPlayersNextTurn: CR 800.4m -- an
// effect lasting "until [a departed player]'s next turn" lasts until that
// turn would have begun. p1's goad holds through p0's turn 5 (p1 is no
// longer an offered defender) and the rule ends it when p1's turn 6 would
// have begun, so on p0's next turn the Bears need not attack. With p1 gone
// that turn is 8 (p2 takes 6, p3 takes 7), not 9. The turn-5 expect pins
// the other half: the goad is still required before that boundary (a goad
// dropped at p2's turn 3, the first turn "after" the departed seat, would
// fail it), and events.expireDepartedGoads expires it when the ordinary turn
// rotation passes p1's seat.
func TestCR800_4mDurationEndsAtTheDepartedPlayersNextTurn(t *testing.T) {
	run := runFourSeat(t, strings.Replace(fourSeatDecorum, "%s",
		`,{"op":"pass_to","seat":0,"step":"declare-attackers","active":"p0","expect":[{"attack_required":{"attacker":"p0:Grizzly Bears"}}]}`, 1))
	if !run.e.G.Players[1].Lost {
		t.Fatalf("precondition: p1 has not left the game")
	}
	d := run.e.Pending()
	if d == nil || d.Kind != decision.KAttackers || run.e.G.Turn != 5 {
		t.Fatalf("precondition: pending %v on turn %d, want p0's attackers decision on turn 5", d, run.e.G.Turn)
	}
	bears := fourSeatRef(t, run, "p0:Grizzly Bears")
	if offersAttack(d, bears, 1) {
		t.Errorf("turn 5: departed p1 offered as a defender")
	}
	if !offersAttack(d, bears, 2) {
		t.Errorf("precondition: p2 not offered as a defender")
	}

	run8 := runFourSeat(t, strings.Replace(fourSeatDecorum, "%s",
		`,{"op":"pass_to","seat":0,"step":"declare-attackers","active":"p0"},`+
			`{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p2"},`+
			`{"op":"pass_to","seat":0,"step":"end"},`+
			`{"op":"pass_to","seat":0,"step":"declare-attackers","active":"p0","expect":[{"attack_required":{"attacker":"p0:Grizzly Bears"},"want":false}]}`, 1))
	if d := run8.e.G.Active; run8.e.G.Turn != 8 || d != 0 {
		t.Fatalf("precondition: p%d active on turn %d, want p0 on turn 8", d, run8.e.G.Turn)
	}
}

// TestCR800_5SeatOrderIsTurnOrder: CR 800.5 / 101.4 -- turns pass to the
// left in seat order: p1, p2, p3, then p0 again on turns 2 to 5.
func TestCR800_5SeatOrderIsTurnOrder(t *testing.T) {
	steps := []string{}
	for _, p := range []string{"p1", "p2", "p3", "p0"} {
		steps = append(steps, `{"op":"pass_to","seat":0,"step":"upkeep","active":"`+p+`"}`)
	}
	raw := `{"name":"cr800-5","format":"constructed","players":4,"cr":["800.5"],"why":"turn order is seat order","setup":{},"steps":[` +
		strings.Join(steps, ",") + `]}`
	for i := range steps {
		run := runFourSeat(t, strings.Replace(raw, strings.Join(steps, ","), strings.Join(steps[:i+1], ","), 1))
		wantActive, wantTurn := state.PlayerID((i+1)%4), i+2
		if run.e.G.Active != wantActive || int(run.e.G.Turn) != wantTurn {
			t.Errorf("after %d passes: active p%d on turn %d, want p%d on turn %d", i+1, run.e.G.Active, run.e.G.Turn, wantActive, wantTurn)
		}
	}
}

// TestOracleAttackDefendersShape: defenders must carry one seat per
// attacker and never join defender; a scenario using defender plays as
// before.
func TestOracleAttackDefendersShape(t *testing.T) {
	const bad = `{"name":"defenders-shape","format":"constructed","players":4,"setup":{"p0":{"battlefield":["Grizzly Bears","Grizzly Bears"]}},
"steps":[{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears","p0:Grizzly Bears#2"],%s}]}`
	for _, shape := range []string{`"defenders":["p1"]`, `"defenders":["p1","p2"],"defender":"p1"`} {
		run, fails := runOracleCommander(t, strings.Replace(bad, "%s", shape, 1))
		if !strings.Contains(strings.Join(fails, "; "), "attack: defenders needs one seat per attacker") {
			t.Errorf("%s: fails = %v, want the defenders harness error\n%s", shape, fails, strings.Join(run.log, "\n"))
		}
	}
	two := runFourSeat(t, `{"name":"defender-two-seat","setup":{"p0":{"battlefield":["Grizzly Bears"]}},
"steps":[{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p1"},{"op":"pass_to","seat":0,"step":"main2"}]}`)
	if len(two.e.G.Players) != 2 || two.e.G.Players[1].Life != 18 || two.e.G.Players[0].Life != 20 {
		t.Fatalf("two-seat defender attack: %d seats, p1 life %d; want 2 seats, 18", len(two.e.G.Players), two.e.G.Players[1].Life)
	}
}
