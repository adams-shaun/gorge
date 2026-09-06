package view

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// commanderFixture builds a two-seat game with one commander per seat, both
// in their owners' command zones: alice has cast hers once (CR 903.8), and
// bob has taken 5 commander damage from alice's commander (CR 903.10). The
// commander bookkeeping (Commanders/CmdCasts/CmdDamage) is set directly —
// the way a live Commander game holds it — so the projection tests pin
// what Project derives from it without needing an Engine.
func commanderFixture(t *testing.T) *state.Game {
	t.Helper()
	g := state.NewGame([]string{"alice", "bob"})
	src := "Name:Commander\nManaCost:2 G\nTypes:Legendary Creature Bear\nPT:4/4\nOracle:x\n"
	c, diags := cards.ParseBytes("commander_test.go", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("parsing commander: %v", diags)
	}
	c.Link()
	a := g.AddObject(c, 0) // alice's commander, dense index 0
	b := g.AddObject(c, 1) // bob's commander, dense index 1
	g.Players[0].Commanders = []state.ObjID{a.ID}
	g.Players[0].CmdCasts = []int32{1}
	g.Players[0].CmdDamage = []int32{0, 0}
	g.Players[1].Commanders = []state.ObjID{b.ID}
	g.Players[1].CmdCasts = []int32{0}
	g.Players[1].CmdDamage = []int32{5, 0} // bob took 5 from alice's commander
	g.SetZone(state.ZCommand, 0, []state.ObjID{a.ID})
	g.SetZone(state.ZCommand, 1, []state.ObjID{b.ID})
	return g
}

// TestProjectProjectsCommandZoneForEverySeat is the m30 promise kept on the
// wire: the command zone (ZCommand is not Hidden) projects for EVERY seat —
// alice sees bob's command zone as fully as her own — together with the
// roster (each seat's full Commanders list, never shrunk), the parallel CR
// 903.8 cast counts, and the CR 903.10 clock on the player who took it.
func TestProjectProjectsCommandZoneForEverySeat(t *testing.T) {
	g := commanderFixture(t)
	v := Project(g, flatChars{g}, 0, nil)
	alice, bob := v.Players[0], v.Players[1]
	if len(alice.Command) != 1 || alice.Command[0].ID != g.Players[0].Commanders[0] {
		t.Fatalf("alice's command zone = %+v, want her commander", alice.Command)
	}
	if alice.Command[0].Name != "Commander" || alice.Command[0].Types != "Legendary Creature Bear" {
		t.Errorf("command-zone CardView = %q / %q, want the commander's identity", alice.Command[0].Name, alice.Command[0].Types)
	}
	// The roster runs parallel to the cast counts, and the counts survive
	// the commander's location (alice's has been cast once).
	if len(alice.Commanders) != 1 || alice.Commanders[0].ID != g.Players[0].Commanders[0] {
		t.Fatalf("alice's roster = %+v, want her commander", alice.Commanders)
	}
	if len(alice.CommanderCasts) != 1 || alice.CommanderCasts[0] != 1 {
		t.Errorf("alice's cast counts = %v, want [1] (one command-zone cast)", alice.CommanderCasts)
	}
	if len(bob.Commanders) != 1 || bob.Commanders[0].ID != g.Players[1].Commanders[0] || bob.CommanderCasts[0] != 0 {
		t.Errorf("bob's roster = %+v casts %v, want his commander never cast", bob.Commanders, bob.CommanderCasts)
	}
	// The clock: bob has taken 5 from alice's commander, keyed by the
	// commander's object id.
	if bob.CmdDamage[g.Players[0].Commanders[0]] != 5 || len(bob.CmdDamage) != 1 {
		t.Errorf("bob's commander damage = %v, want {alice's commander: 5}", bob.CmdDamage)
	}
	// Alice has taken none: nil map, so the JSON key is absent (omitempty).
	if alice.CmdDamage != nil {
		t.Errorf("alice's commander damage = %v, want nil (nothing taken)", alice.CmdDamage)
	}

	// Public: bob's view of alice's command zone/roster/casts is the same.
	vb := Project(g, flatChars{g}, 1, nil)
	if len(vb.Players[0].Command) != 1 || len(vb.Players[0].Commanders) != 1 || vb.Players[0].CommanderCasts[0] != 1 {
		t.Errorf("bob's view of alice's command zone = %+v / %+v / %v — the command zone must be public", vb.Players[0].Command, vb.Players[0].Commanders, vb.Players[0].CommanderCasts)
	}
	if vb.Players[1].CmdDamage[g.Players[0].Commanders[0]] != 5 {
		t.Errorf("bob's own view omits his commander damage (must be public)")
	}
}

// TestCommandZoneRosterSurvivesTheCast is why the roster exists at all: a
// commander that leaves the command zone (cast — here moved to the
// battlefield) is GONE from the zone list but STILL in the roster with its
// cast count, so a client — and the bot policy's view-shaped half — can
// tell the battlefield creature is a commander even before it has dealt any
// commander damage.
func TestCommandZoneRosterSurvivesTheCast(t *testing.T) {
	g := commanderFixture(t)
	a := g.Players[0].Commanders[0]
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{a})
	g.SetZone(state.ZCommand, 0, nil)
	v := Project(g, flatChars{g}, 0, nil)
	if len(v.Players[0].Command) != 0 {
		t.Errorf("alice's command zone = %+v, want empty after the cast", v.Players[0].Command)
	}
	if len(v.Players[0].Commanders) != 1 || v.Players[0].Commanders[0].ID != a || v.Players[0].CommanderCasts[0] != 1 {
		t.Errorf("alice's roster after the cast = %+v casts %v, want the commander still rostered with 1 cast", v.Players[0].Commanders, v.Players[0].CommanderCasts)
	}
	if len(v.Players[0].Battlefield) != 1 || v.Players[0].Battlefield[0].ID != a {
		t.Errorf("her battlefield = %+v, want the cast commander on it", v.Players[0].Battlefield)
	}
}

// TestCommandZoneMarshalShapes is the wire-shape pin for the new public
// facts: the command zone and roster marshal as explicit empty arrays on a
// game with no commanders, the cast counts as [], and cmd_damage is absent
// when nobody has taken any (omitempty — absence reads as zero, the same
// rule Counters uses), then present as an id-keyed tally once damage lands.
func TestCommandZoneMarshalShapes(t *testing.T) {
	g := state.NewGame([]string{"alice", "bob"})
	blob, err := json.Marshal(Project(g, flatChars{g}, 0, nil))
	if err != nil {
		t.Fatal(err)
	}
	s := string(blob)
	for _, key := range []string{"command", "commanders", "commander_casts"} {
		if !strings.Contains(s, `"`+key+`":[]`) {
			t.Fatalf("%q did not marshal as []: %s", key, s)
		}
	}
	if strings.Contains(s, `"cmd_damage":null`) || strings.Contains(s, `"cmd_damage":{}`) {
		t.Fatalf("empty commander damage should be absent (omitempty), not null or {}: %s", s)
	}

	// With damage on the table, cmd_damage appears keyed by commander id.
	g2 := commanderFixture(t)
	blob2, err := json.Marshal(Project(g2, flatChars{g2}, 1, nil))
	if err != nil {
		t.Fatal(err)
	}
	s2 := string(blob2)
	id := g2.Players[0].Commanders[0]
	if !strings.Contains(s2, `"cmd_damage":{"`+strconv.Itoa(int(id))+`":5}`) {
		t.Fatalf("bob's commander damage did not marshal as {commander id: 5}: %s", s2)
	}
}
