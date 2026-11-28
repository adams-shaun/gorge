package rules

// CR 800.4m with CR 500.7: a departed player's next turn is a slot of the
// ordinary turn rotation, so an extra turn another seat takes does not move
// the advance past it.

import (
	"strings"
	"testing"
)

// TestCR800_4mExtraTurnBeforeTheDepartedSlot: p2 gives p0 an extra turn on
// turn 3, so the order is p2(3), p0 extra(4), p3(5), p0(6), then p1's turn
// would begin, then p2(7). Leaving p0's extra turn for p3 does not cross p1's
// slot: the goad is still required on p0's turn 6 and gone on turn 9.
func TestCR800_4mExtraTurnBeforeTheDepartedSlot(t *testing.T) {
	const required = `,{"op":"pass_to","seat":0,"step":"declare-attackers","active":"p0","expect":[{"attack_required":{"attacker":"p0:Grizzly Bears"}}]}`
	const attack = `,{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p2"},{"op":"pass_to","seat":0,"step":"end"}`
	steps := `,{"op":"pass_to","seat":2,"step":"main1","active":"p2"},` +
		`{"op":"cast","seat":2,"card":"p2:Time Warp","mana":"UUUUU","targets":["p0"]},{"op":"resolve","seat":2}` +
		required
	raw := strings.Replace(fourSeatDecorum, `"p1":{"life":2,`, `"p2":{"hand":["Time Warp"]},"p1":{"life":2,`, 1)

	extra := runFourSeat(t, strings.Replace(raw, "%s", steps, 1))
	if extra.e.G.Turn != 4 || extra.e.G.Active != 0 || !extra.e.G.Players[1].Lost {
		t.Fatalf("precondition: p%d active on turn %d, want p0's extra turn 4 with p1 gone", extra.e.G.Active, extra.e.G.Turn)
	}

	before := runFourSeat(t, strings.Replace(raw, "%s", steps+attack+required, 1))
	if before.e.G.Turn != 6 || before.e.G.Active != 0 {
		t.Fatalf("precondition: p%d active on turn %d, want p0 on turn 6", before.e.G.Active, before.e.G.Turn)
	}

	after := runFourSeat(t, strings.Replace(raw, "%s", steps+attack+required+attack+
		`,{"op":"pass_to","seat":0,"step":"declare-attackers","active":"p0","expect":[{"attack_required":{"attacker":"p0:Grizzly Bears"},"want":false}]}`, 1))
	if after.e.G.Turn != 9 || after.e.G.Active != 0 {
		t.Fatalf("precondition: p%d active on turn %d, want p0 on turn 9", after.e.G.Active, after.e.G.Turn)
	}
}

// TestCR800_4mExtraTurnAfterTheDepartedSlot: p0 gives p2 an extra turn on
// turn 5. Going p0 -> p2 for that extra turn (6) does not reach p1's slot, so
// the goad survives it; the ordinary rotation then resumes after p0, crosses
// p1's slot on the way to p2's turn 7, and the goad ends there.
func TestCR800_4mExtraTurnAfterTheDepartedSlot(t *testing.T) {
	steps := `,{"op":"pass_to","seat":0,"step":"main1","active":"p0"},` +
		`{"op":"cast","seat":0,"card":"p0:Time Warp","mana":"UUUUU","targets":["p2"]},{"op":"resolve","seat":0},` +
		`{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p2"},` +
		`{"op":"pass_to","seat":0,"step":"upkeep","active":"p2"}`
	raw := strings.Replace(fourSeatDecorum, `"hand":["Shock"]`, `"hand":["Shock","Time Warp"]`, 1)

	extra := runFourSeat(t, strings.Replace(raw, "%s", steps, 1))
	bears := fourSeatRef(t, extra, "p0:Grizzly Bears")
	if extra.e.G.Turn != 6 || extra.e.G.Active != 2 || !extra.e.G.Players[1].Lost {
		t.Fatalf("precondition: p%d active on turn %d, want p2's extra turn 6 with p1 gone", extra.e.G.Active, extra.e.G.Turn)
	}
	if len(extra.e.G.Obj(bears).Goads) != 1 {
		t.Errorf("turn 6 (p2's extra turn): Bears goads = %v, want p1's still there", extra.e.G.Obj(bears).Goads)
	}

	next := runFourSeat(t, strings.Replace(raw, "%s", steps+
		`,{"op":"pass_to","seat":0,"step":"end","active":"p2"},{"op":"pass_to","seat":0,"step":"upkeep","active":"p2"}`, 1))
	bears = fourSeatRef(t, next, "p0:Grizzly Bears")
	if next.e.G.Turn != 7 || next.e.G.Active != 2 {
		t.Fatalf("precondition: p%d active on turn %d, want p2's ordinary turn 7", next.e.G.Active, next.e.G.Turn)
	}
	if got := next.e.G.Obj(bears).Goads; len(got) != 0 {
		t.Errorf("turn 7 (past p1's slot): Bears goads = %v, want none", got)
	}
}
