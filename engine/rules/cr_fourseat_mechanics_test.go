package rules

// The multiplayer mechanics with no four-seat scenario test: goad (CR
// 701.15), the council vote (CR 701.38), myriad (CR 702.116), the monarch
// (CR 725) and the initiative (CR 726). Each test is a real-card
// four-seat constructed scenario played through the oracle runner and read
// back from the final game state.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// mechanicAttackPlayers returns the sorted defending seats the pending
// attackers decision offers for obj, and whether any of them is Required.
func mechanicAttackPlayers(d *decision.Decision, obj state.ObjID) ([]state.PlayerID, bool) {
	var players []state.PlayerID
	required := false
	for _, o := range d.Options {
		if o.Obj != obj || o.Battle != 0 {
			continue
		}
		if !slices.Contains(players, o.Player) {
			players = append(players, o.Player)
		}
		if o.Required {
			required = true
		}
	}
	slices.Sort(players)
	return players, required
}

// TestCR701_15bGoadedCreatureAttacksANonGoader: CR 701.15b -- a goaded
// creature must attack a player other than the one who goaded it if one is
// available, so p1's goaded Bears may be declared only against p2 or p3 and
// the declaration is required.
func TestCR701_15bGoadedCreatureAttacksANonGoader(t *testing.T) {
	t.Parallel()
	run := runFourSeat(t, `{"name":"cr701-15b","format":"constructed","players": 4,"cr":["701.15b"],"why":"a goaded creature attacks a non-goader",
"setup":{"p0":{"hand":["Disrupt Decorum"]},"p1":{"battlefield":["Grizzly Bears"]}},
"steps":[{"op":"cast","seat":0,"card":"p0:Disrupt Decorum","mana":"RRRR"},
{"op":"resolve","seat":0},
{"op":"pass_to","seat":0,"step":"declare-attackers","active":"p1"}]}`)
	d := run.e.Pending()
	if d == nil || d.Kind != decision.KAttackers || d.Player != 1 {
		t.Fatalf("pending = %v, want p1's declare-attackers decision", d)
	}
	bears := fourSeatRef(t, run, "p1:Grizzly Bears")
	players, required := mechanicAttackPlayers(d, bears)
	if want := []state.PlayerID{2, 3}; !slices.Equal(players, want) {
		t.Errorf("Bears' offered defenders = %v, want exactly %v (the goader p0 is not offered)", players, want)
	}
	if !required {
		t.Errorf("no Bears attacker option is Required; a goaded creature must attack")
	}
}

// TestCR701_15cTwoGoadersLeaveTheThirdOpponent: CR 701.15c-d -- each goad is
// a separate requirement, so a creature goaded by both p0 and p2 must attack
// the only remaining non-goader, p3.
func TestCR701_15cTwoGoadersLeaveTheThirdOpponent(t *testing.T) {
	t.Parallel()
	run := runFourSeat(t, `{"name":"cr701-15c","format":"constructed","players": 4,"cr":["701.15c","701.15d"],"why":"two goaders leave the third opponent",
"setup":{"p0":{"hand":["Disrupt Decorum"]},"p1":{"battlefield":["Grizzly Bears"]},"p2":{"hand":["Take the Bait"]}},
"steps":[{"op":"cast","seat":0,"card":"p0:Disrupt Decorum","mana":"RRRR"},{"op":"resolve","seat":0},
{"op":"attack","seat":1,"attackers":["p1:Grizzly Bears"],"defender":"p2"},
{"op":"pass","seat":1},
{"op":"cast","seat":2,"card":"p2:Take the Bait","mana":"RRWW"},{"op":"resolve","seat":2},
{"op":"pass_to","seat":2,"step":"end-combat","active":"p1"},
{"op":"pass_to","seat":2,"step":"declare-attackers","active":"p1"}]}`)
	if got := run.e.G.Players[2].Life; got != 20 {
		t.Errorf("p2 life = %d, want 20 (Take the Bait prevents the first combat's damage)", got)
	}
	d := run.e.Pending()
	if d == nil || d.Kind != decision.KAttackers || d.Player != 1 {
		t.Fatalf("pending = %v, want p1's added-combat declare-attackers decision", d)
	}
	bears := fourSeatRef(t, run, "p1:Grizzly Bears")
	players, _ := mechanicAttackPlayers(d, bears)
	if want := []state.PlayerID{3}; !slices.Equal(players, want) {
		t.Errorf("Bears' offered defenders with two goaders = %v, want exactly %v", players, want)
	}
}

// TestCR725_2MonarchCrownFollowsCombatDamage: CR 725.2/725.3 -- combat damage
// from a creature to the monarch makes that creature's controller the
// monarch, and only the monarch's own end step draws the crown's card.
func TestCR725_2MonarchCrownFollowsCombatDamage(t *testing.T) {
	t.Parallel()
	run := runFourSeat(t, `{"name":"cr725-2","format":"constructed","players": 4,"cr":["725.2","725.3"],"why":"the crown follows combat damage and draws only for its holder",
"setup":{"p0":{"hand":["Marchesa's Decree"]},"p1":{"battlefield":["Grizzly Bears"]}},
"steps":[{"op":"cast","seat":0,"card":"p0:Marchesa's Decree","mana":"BBBB"},{"op":"resolve","seat":0},
{"op":"attack","seat":1,"attackers":["p1:Grizzly Bears"],"defender":"p0"},
{"op":"pass_to","seat":1,"step":"main2","active":"p1",
 "expect":[{"life":{"p0":18,"p1":19}}]},
{"op":"pass_to","seat":1,"step":"upkeep","active":"p2",
 "expect":[{"hand_size":{"p1":2}}]},
{"op":"pass_to","seat":1,"step":"upkeep","active":"p3",
 "expect":[{"hand_size":{"p1":2}}]}]}`)
	for _, p := range []state.PlayerID{0, 2, 3} {
		if run.e.G.IsMonarch(p) {
			t.Errorf("p%d is the monarch; the crown went to p1 with the combat damage", p)
		}
	}
	if !run.e.G.IsMonarch(1) {
		t.Errorf("p1 is not the monarch after its Bears damaged the monarch")
	}
}

// TestCR725_4DepartedMonarchGoesToTheActivePlayer: CR 725.4 -- when the
// monarch leaves the game, the active player becomes the monarch, or the next
// player in turn order when the departing monarch was the active player.
func TestCR725_4DepartedMonarchGoesToTheActivePlayer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want state.PlayerID
	}{
		{
			name: "another seat's turn",
			want: 3,
			raw: `{"name":"cr725-4-active","format":"constructed","players": 4,"turn":2,"cr":["725.4"],"why":"a departed monarch's crown goes to the active player",
"setup":{"p1":{"hand":["Marchesa's Decree"],"life":2},"p3":{"hand":["Shock"]}},
"steps":[{"op":"cast","seat":1,"card":"p1:Marchesa's Decree","mana":"BBBB"},{"op":"resolve","seat":1},
{"op":"pass_to","seat":1,"step":"main1","active":"p3"},
{"op":"cast","seat":3,"card":"p3:Shock","mana":"R","targets":["p1"]},{"op":"resolve","seat":3}]}`,
		},
		{
			name: "the active player leaves",
			want: 2,
			raw: `{"name":"cr725-4-active-leaves","format":"constructed","players": 4,"turn":2,"cr":["725.4"],"why":"a departing active monarch's crown goes to the next player in turn order",
"setup":{"p1":{"hand":["Marchesa's Decree"],"life":2},"p3":{"hand":["Shock"]}},
"steps":[{"op":"cast","seat":1,"card":"p1:Marchesa's Decree","mana":"BBBB"},{"op":"resolve","seat":1},
{"op":"pass","seat":1},{"op":"pass","seat":2},
{"op":"cast","seat":3,"card":"p3:Shock","mana":"R","targets":["p1"]},{"op":"resolve","seat":3}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := runFourSeat(t, tc.raw)
			g := run.e.G
			if !g.Players[1].Lost {
				t.Fatalf("p1 has not left the game")
			}
			if g.Over {
				t.Fatalf("game over with three players left (CR 800.4a)")
			}
			if !g.IsMonarch(tc.want) {
				t.Errorf("monarch = %d, want p%d", g.Monarch, tc.want)
			}
			for p := state.PlayerID(0); p < 4; p++ {
				if p != tc.want && g.IsMonarch(p) {
					t.Errorf("p%d also holds the crown; want only p%d", p, tc.want)
				}
			}
		})
	}
}

// TestCR726_2InitiativeMovesToTheAttackersController: CR 726.2/726.3 --
// combat damage from a creature to the initiative holder makes that
// creature's controller take the initiative.
func TestCR726_2InitiativeMovesToTheAttackersController(t *testing.T) {
	t.Parallel()
	run := runFourSeat(t, `{"name":"cr726-2","format":"constructed","players": 4,"cr":["726.2","726.3"],"why":"the initiative moves to the attacker's controller",
"setup":{"p0":{"hand":["Undermountain Adventurer"]},"p2":{"battlefield":["Grizzly Bears"]}},
"steps":[{"op":"cast","seat":0,"card":"p0:Undermountain Adventurer","mana":"GGGG"},{"op":"resolve","seat":0},
{"op":"attack","seat":2,"attackers":["p2:Grizzly Bears"],"defender":"p0"},
{"op":"block","seat":0,"blocks":[]},
{"op":"pass_to","seat":2,"step":"main2","active":"p2",
 "expect":[{"life":{"p0":18}}]}]}`)
	if !run.e.G.IsInitiative(2) {
		t.Errorf("p2 does not have the initiative after its Bears damaged the holder")
	}
	for _, p := range []state.PlayerID{0, 1, 3} {
		if run.e.G.IsInitiative(p) {
			t.Errorf("p%d also has the initiative; the combat damage moved it to p2", p)
		}
	}
}

// TestCR726_4DepartedInitiativeHolderHandsItOn: CR 726.4 -- when the
// initiative holder leaves the game, the active player takes the initiative,
// or the next player in turn order when the departed holder was active.
func TestCR726_4DepartedInitiativeHolderHandsItOn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want state.PlayerID
	}{
		{
			name: "another seat's turn",
			want: 3,
			raw: `{"name":"cr726-4-active","format":"constructed","players": 4,"turn":2,"cr":["726.4"],"why":"a departed holder's initiative goes to the active player",
"setup":{"p1":{"hand":["Undermountain Adventurer"],"life":2},"p3":{"hand":["Shock"]}},
"steps":[{"op":"cast","seat":1,"card":"p1:Undermountain Adventurer","mana":"GGGG"},{"op":"resolve","seat":1},
{"op":"pass_to","seat":1,"step":"main1","active":"p3"},
{"op":"cast","seat":3,"card":"p3:Shock","mana":"R","targets":["p1"]},{"op":"resolve","seat":3}]}`,
		},
		{
			name: "the active player leaves",
			want: 2,
			raw: `{"name":"cr726-4-active-leaves","format":"constructed","players": 4,"turn":2,"cr":["726.4"],"why":"a departing active holder's initiative goes to the next player in turn order",
"setup":{"p1":{"hand":["Undermountain Adventurer"],"life":2},"p3":{"hand":["Shock"]}},
"steps":[{"op":"cast","seat":1,"card":"p1:Undermountain Adventurer","mana":"GGGG"},{"op":"resolve","seat":1},
{"op":"pass","seat":1},{"op":"pass","seat":2},
{"op":"cast","seat":3,"card":"p3:Shock","mana":"R","targets":["p1"]},{"op":"resolve","seat":3}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := runFourSeat(t, tc.raw)
			g := run.e.G
			if !g.Players[1].Lost {
				t.Fatalf("p1 has not left the game")
			}
			if g.Over {
				t.Fatalf("game over with three players left (CR 800.4a)")
			}
			if !g.IsInitiative(tc.want) {
				t.Errorf("initiative = %d, want p%d", g.Initiative, tc.want)
			}
			for p := state.PlayerID(0); p < 4; p++ {
				if p != tc.want && g.IsInitiative(p) {
					t.Errorf("p%d also has the initiative; want only p%d", p, tc.want)
				}
			}
		})
	}
}

// TestCR701_38aVotesGoInTurnOrderFromTheCaster: CR 701.38a-c -- the council
// votes starting with the spell's controller and then in turn order, and
// every permanent tied for the most votes is exiled.
func TestCR701_38aVotesGoInTurnOrderFromTheCaster(t *testing.T) {
	t.Parallel()
	run := runFourSeat(t, `{"name":"cr701-38a","format":"constructed","players": 4,"turn":3,"cr":["701.38a","701.38b","701.38c"],"why":"votes go in turn order from the caster",
"setup":{"p2":{"hand":["Council's Judgment"]},"p0":{"battlefield":["Grizzly Bears"]},"p1":{"battlefield":["Hill Giant"]},"p3":{"battlefield":["Memnite"]}},
"steps":[{"op":"cast","seat":2,"card":"p2:Council's Judgment","mana":"WWW"},
{"op":"resolve","seat":2,"answers":[
 {"kind":"choose","pick":["Grizzly Bears"],"seat":2},
 {"kind":"choose","pick":["Hill Giant"],"seat":3},
 {"kind":"choose","pick":["Hill Giant"],"seat":0},
 {"kind":"choose","pick":["Grizzly Bears"],"seat":1}]}]}`)
	for _, ref := range []string{"p0:Grizzly Bears", "p1:Hill Giant"} {
		if o := run.e.G.Obj(fourSeatRef(t, run, ref)); o == nil || o.Zone != state.ZExile {
			t.Errorf("%s zone = %v, want exile (tied for the most votes)", ref, o.Zone)
		}
	}
	if o := run.e.G.Obj(fourSeatRef(t, run, "p3:Memnite")); o == nil || o.Zone != state.ZBattlefield {
		t.Errorf("Memnite zone = %v, want battlefield (no votes)", o.Zone)
	}
}

// TestCR702_116aMyriadCopiesAttackEachOtherOpponent: CR 702.116a -- myriad
// creates a tapped, attacking token copy for each opponent other than the
// defending player, and the copies are exiled at end of combat.
func TestCR702_116aMyriadCopiesAttackEachOtherOpponent(t *testing.T) {
	t.Parallel()
	run := runFourSeat(t, `{"name":"cr702-116a","format":"constructed","players": 4,"cr":["702.116a"],"why":"myriad copies attack each other opponent and are exiled at end of combat",
"setup":{"p0":{"battlefield":["Wyrm's Crossing Patrol"]}},
"steps":[{"op":"attack","seat":0,"attackers":["p0:Wyrm's Crossing Patrol"],"defender":"p2"},
{"op":"resolve","seat":0,"answers":[
 {"kind":"choose","pick":["Create a copy"],"seat":0},
 {"kind":"choose","pick":["Create a copy"],"seat":0}],
 "expect":[{"count":{"seat":0,"zone":"battlefield","name":"Wyrm's Crossing Patrol","token":true},"eq":2}]},
{"op":"pass_to","seat":0,"step":"main2","active":"p0",
 "expect":[{"life":{"p1":19,"p2":19,"p3":19}},
           {"count":{"seat":0,"zone":"battlefield","name":"Wyrm's Crossing Patrol","token":true},"eq":0}]}]}`)
	if run.e.G.Turn != 1 {
		t.Errorf("turn = %d, want 1 (the whole combat is p0's first turn)", run.e.G.Turn)
	}
}
