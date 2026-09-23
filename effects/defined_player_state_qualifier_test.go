package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// Issue agent-20260918T234402Z-b031583b: `Defined$ Player.<state-qualifier>`
// at resolution time resolved to nobody, and the controlsCreature./
// controlsPermanent. qualifier grammar did not exist. These are the pure-
// grammar leaves; the end-to-end corpus-card pins live in
// rules/defined_player_state_qualifier_test.go.

func qualifierObject(t *testing.T, g *state.Game, p state.PlayerID, src string) *state.Object {
	t.Helper()
	c, diags := cards.ParseBytes("t.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("diags: %v", diags)
	}
	c.Link()
	id := g.AddObject(c, p).ID
	// Place immediately: SetZone updates the zone LIST (g.Zone's read path);
	// writing only o.Zone would leave the battlefield scan empty.
	g.Obj(id).Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, p, append(g.Zone(state.ZBattlefield, p), id))
	return g.Obj(id)
}

// TestDefinedPlayerStateQualifierControlsGrammar is the filter-grammar leaf
// for Forge's Player.controlsCreature.<spec> / controlsPermanent.<spec>
// qualifiers: existential by default, a trailing _GE<n>-style token turns the
// match into a count comparison, a trailing token that is NOT a count
// comparison stays part of the object spec (and an unmodelled predicate there
// fails closed), and unknown object specs never match.
func TestDefinedPlayerStateQualifierControlsGrammar(t *testing.T) {
	g := state.NewGame([]string{"a", "b", "c"})
	// Seat 0: a 5/5, a 2/2, and the named artifact. Seat 1: a 2/2 and the
	// named artifact. Seat 2: a multicoloured 2/2.
	qualifierObject(t, g, 0, "Name:Big Brute\nManaCost:3 G\nTypes:Creature Beast\nPT:5/5\nOracle:x\n")
	qualifierObject(t, g, 0, "Name:Small Fry\nManaCost:G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	qualifierObject(t, g, 0, "Name:Bonder's Ornament\nManaCost:3\nTypes:Artifact\nOracle:x\n")
	qualifierObject(t, g, 1, "Name:Small Fry\nManaCost:G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	qualifierObject(t, g, 1, "Name:Bonder's Ornament\nManaCost:3\nTypes:Artifact\nOracle:x\n")
	qualifierObject(t, g, 2, "Name:Guildmage\nManaCost:W B\nTypes:Creature Human Wizard\nPT:2/2\nOracle:x\n")

	cases := []struct {
		spec string
		want map[state.PlayerID]bool
	}{
		{"Player.controlsCreature.powerGE4_GE1", map[state.PlayerID]bool{0: true, 1: false, 2: false}},
		{"Player.controlsCreature.powerGE4_GE2", map[state.PlayerID]bool{0: false, 1: false, 2: false}},
		{"Player.controlsCreature.powerGE4_EQ1", map[state.PlayerID]bool{0: true, 1: false, 2: false}},
		{"Player.controlsCreature.powerGE4_LT1", map[state.PlayerID]bool{0: false, 1: true, 2: true}},
		{"Player.controlsCreature.powerGE3", map[state.PlayerID]bool{0: true, 1: false, 2: false}},
		// The underscore guard: "GEx" is not a count comparison, so the whole
		// remainder stays the object spec -- an unmodelled predicate there
		// fails closed for every seat.
		{"Player.controlsCreature.powerGE4_GEx", map[state.PlayerID]bool{0: false, 1: false, 2: false}},
		{"Player.controlsCreature.MultiColor", map[state.PlayerID]bool{0: false, 1: false, 2: true}},
		{"Player.controlsPermanent.namedBonder's Ornament", map[state.PlayerID]bool{0: true, 1: true, 2: false}},
		{"Player.controlsPermanent.EnchantedBy", map[state.PlayerID]bool{0: false, 1: false, 2: false}},
	}
	for _, tc := range cases {
		for p, want := range tc.want {
			if got := MatchesPlayerSpec(g, tc.spec, p, 0); got != want {
				t.Errorf("MatchesPlayerSpec(%q, seat %d) = %v, want %v", tc.spec, p, got, want)
			}
		}
	}
}

// TestDefinedPlayerStateQualifierUnmodelledSpellingsActOnNobody pins the
// routing's empty-set semantics: definedSpec CLAIMS every Player.<...>
// spelling (ok=true) but the qualifiers the grammar cannot evaluate -- Curse
// attachment, the NotedFor* machinery this build lacks, per-turn mana-tap
// history, combat history per player-vs-source, and an X right-hand side --
// match NOBODY, so the resolving effect acts on nobody instead of falling
// back to the spell's (object) targets.
func TestDefinedPlayerStateQualifierUnmodelledSpellingsActOnNobody(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	for _, spec := range []string{
		"Player.attackedBySourceThisTurn",
		"Player.attackedBySourceThisCombat",
		"Player.EnchantedBy",
		"Player.NotedForDiscard",
		"Player.TappedLandForManaThisTurn",
	} {
		ts, ok := definedSpec(h, c, spec)
		if !ok {
			t.Errorf("definedSpec(%q): ok=false -- the base-Player routing must claim it and act on nobody", spec)
			continue
		}
		if len(ts) != 0 {
			t.Errorf("definedSpec(%q) = %v; want the empty set (act on nobody)", spec, ts)
		}
	}
}

// TestDefinedPlayerStateQualifierRoutingLifeEQ pins the routing itself on the
// cheapest supported spelling: definedSpec resolves Player.lifeEQ13 to the
// seat whose life is exactly 13, walking AliveFrom order, and the supported
// withMost spelling likewise.
func TestDefinedPlayerStateQualifierRoutingLifeEQ(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	g.Players[1].Life = 13
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	ts, ok := definedSpec(h, c, "Player.lifeEQ13")
	if !ok || len(ts) != 1 || !ts[0].IsPlayer || ts[0].Player != 1 {
		t.Fatalf("definedSpec(Player.lifeEQ13) = %v, %v; want exactly seat 1", ts, ok)
	}
}
