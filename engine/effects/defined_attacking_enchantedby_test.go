package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The Commander curses' second leg. Curse of Opulence's sub-ability names its
// token owner `TriggeredAttackingPlayer.Opponent+controlsCreature.attacking
// Player.EnchantedBy`, and Curse of Disturbance, Verbosity, Bounty and
// Vitality carry the same Defined$ spelling on their Draw / UntapAll /
// GainLife legs.
//
// Forge resolves TriggeredAttackingPlayer to the ONE declaring seat and then
// filters that seat through the qualifier list relative to the ability's
// controller (Player.Opponent is isOpponentOf(sourceController)). So the
// ordinary case -- an opponent attacks a player enchanted by the curse's
// controller -- must name that opponent; only the controller attacking its
// own cursed target is excluded.

// testEnchantedByCurse puts a curse on seat enchanted, controlled by ctrl,
// and asserts the precondition its non-empty case depends on: the enchantment
// is the source and names exactly the enchanted seat.
func testEnchantedByCurse(t *testing.T, reg *cards.Registry, g *state.Game, ctrl, enchanted state.PlayerID) *state.Object {
	t.Helper()
	curse := corpusObject(t, reg, g, "Curse of Opulence")
	curse.HasAttachedPlayer = true
	curse.AttachedPlayer = enchanted
	curse.Controller = ctrl
	live := g.Obj(curse.ID)
	if live == nil || !live.HasAttachedPlayer || live.AttachedPlayer != enchanted || live.Controller != ctrl ||
		enchanted == ctrl {
		t.Fatalf("precondition failed: curse must enchant seat %d under controller %d (got %+v)", enchanted, ctrl, live)
	}
	return live
}

// testAttackingCurseBear puts an attacking creature controlled by seat under
// attack on the enchanted seat, on the battlefield.
func testAttackingCurseBear(t *testing.T, reg *cards.Registry, g *state.Game, seat, attacking state.PlayerID) *state.Object {
	t.Helper()
	bear := corpusObject(t, reg, g, "Grizzly Bears")
	bear.Controller, bear.IsAttacking, bear.Attacking = seat, true, attacking
	g.SetZone(state.ZBattlefield, seat, append(g.Zone(state.ZBattlefield, seat), bear.ID))
	live := g.Obj(bear.ID)
	if live == nil || live.Zone != state.ZBattlefield || !live.IsAttacking || live.Attacking != attacking {
		t.Fatalf("precondition failed: creature must attack seat %d (got %+v)", attacking, live)
	}
	return live
}

func TestTriggeredAttackingPlayerEnchantedByNamesTheAttacker(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"attacker", "curse controller", "enchanted"})
	// The curse's controller (1) enchants seat 2; seat 0 attacks that player.
	curse := testEnchantedByCurse(t, reg, g, 1, 2)
	testAttackingCurseBear(t, reg, g, 0, 2)
	spec := "TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy"
	h := &fakeHost{g: g}
	c := &Ctx{Source: curse.ID, Controller: 1,
		TriggerContext: TriggerContext{AttackingPlayer: state.Target{Player: 0, IsPlayer: true}}}
	ts := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": spec}})
	if len(ts) != 1 || !ts[0].IsPlayer || ts[0].Player != 0 {
		t.Fatalf("the curses' second leg %q = %+v, want exactly seat 0 (the opponent attacking the enchanted player)", spec, ts)
	}

	// The controller itself attacks the player it enchants: Player.Opponent
	// excludes it, so nobody gets the second token/draw/life/untap.
	c1 := &Ctx{Source: curse.ID, Controller: 1,
		TriggerContext: TriggerContext{AttackingPlayer: state.Target{Player: 1, IsPlayer: true}}}
	if ts := Defined(h, c1, &cards.SA{Params: map[string]string{"Defined": spec}}); len(ts) != 0 {
		t.Fatalf("the controller attacking its own cursed target = %+v, want the empty set", ts)
	}

	// Fail closed: an attacking player outside the game names nobody, the
	// bare referent's absent-role read.
	bad := &Ctx{Source: curse.ID, Controller: 1,
		TriggerContext: TriggerContext{AttackingPlayer: state.Target{Player: state.PlayerID(len(g.Players)), IsPlayer: true}}}
	if ts := Defined(h, bad, &cards.SA{Params: map[string]string{"Defined": spec}}); len(ts) != 0 {
		t.Fatalf("out-of-range TriggeredAttackingPlayer = %+v, want an empty set (the fail-closed absent-role read)", ts)
	}
}

func TestTriggeredAttackingPlayerEnchantedByNeedsAttackingCreature(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"attacker", "curse controller", "enchanted"})
	// The curse's controller (1) enchants seat 2, but the declaring player 0
	// controls no attacking creature: `controlsCreature.attacking
	// Player.EnchantedBy` fails, so the second leg names nobody even though
	// the declaring seat is an opponent.
	curse := testEnchantedByCurse(t, reg, g, 1, 2)
	spec := "TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy"
	h := &fakeHost{g: g}
	c := &Ctx{Source: curse.ID, Controller: 1,
		TriggerContext: TriggerContext{AttackingPlayer: state.Target{Player: 0, IsPlayer: true}}}
	if ts := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": spec}}); len(ts) != 0 {
		t.Fatalf("no attacking creature for seat 0 = %+v, want the empty set", ts)
	}
}

// TestAttackingEnchantedByRider pins the filter word the curse grammar rides:
// `attacking Player.EnchantedBy` is the same `attacking <PlayerSpec>` rider
// every dotted player spec arg uses -- the candidate attacks the seat THIS
// source Aura enchants (Forge's source-relative Player.EnchantedBy). Before the
// widening the dotted arg stayed wordUnknown: the matcher failed it closed seat
// by seat, so a creature attacking an enchanted seat matched nothing and the
// census kept reporting the word.
func TestAttackingEnchantedByRider(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them", "third"})

	// Seat 0 is enchanted by this curse (cast by seat 1); a second curse from
	// seat 2 enchants seat 1, and must not make seat 0's qualifier match.
	curse := corpusObject(t, reg, g, "Curse of Opulence")
	curse.Controller, curse.HasAttachedPlayer, curse.AttachedPlayer = 1, true, 0
	other := corpusObject(t, reg, g, "Curse of Opulence")
	other.Controller, other.HasAttachedPlayer, other.AttachedPlayer = 2, true, 1
	bears := corpusObject(t, reg, g, "Grizzly Bears")
	bears.Controller, bears.IsAttacking, bears.Attacking = 1, true, 0
	notEnch := corpusObject(t, reg, g, "Grizzly Bears")
	notEnch.Controller, notEnch.IsAttacking, notEnch.Attacking = 2, true, 1
	sc := SpecContext{You: 1, Source: curse.ID}
	if !g.Obj(curse.ID).HasAttachedPlayer || g.Obj(curse.ID).AttachedPlayer != 0 || g.Obj(curse.ID).Controller != 1 ||
		g.Obj(other.ID).AttachedPlayer != 1 || !g.Obj(bears.ID).IsAttacking || g.Obj(bears.ID).Attacking != 0 {
		t.Fatalf("precondition failed: %+v %+v %+v", g.Obj(curse.ID), g.Obj(other.ID), g.Obj(bears.ID))
	}

	if !MatchesObjectCtx(g, "Creature.attacking Player.EnchantedBy", g.Obj(bears.ID), sc) {
		t.Error("attacking Player.EnchantedBy must match a creature attacking this Aura's enchanted seat 0")
	}
	if MatchesObjectCtx(g, "Creature.attacking Player.EnchantedBy", g.Obj(notEnch.ID), sc) {
		t.Error("attacking Player.EnchantedBy must not match a creature attacking the other Aura's enchanted seat 1")
	}
	// The same creature matches the OTHER curse's qualifier: the read is the
	// evaluating source's attachment, not any enchantment.
	if !MatchesObjectCtx(g, "Creature.attacking Player.EnchantedBy", g.Obj(notEnch.ID), SpecContext{You: 2, Source: other.ID}) {
		t.Error("the second curse's own qualifier must match the creature attacking its enchanted seat 1")
	}
	if MatchesObjectCtx(g, "Creature.attacking Player.EnchantedBy", g.Obj(bears.ID), SpecContext{You: 2, Source: other.ID}) {
		t.Error("the second curse's qualifier must not match the creature attacking seat 0")
	}

	// The planeswalker trap, the same discriminator the base rider reads: the
	// defender is the permanent, not a player, so it never matches.
	jace := corpusObject(t, reg, g, "Jace Beleren")
	jace.Controller = 0
	pwAtk := corpusObject(t, reg, g, "Grizzly Bears")
	pwAtk.Controller, pwAtk.IsAttacking, pwAtk.Attacking, pwAtk.AttackingBattle = 0, true, 0, jace.ID
	if MatchesObjectCtx(g, "Creature.attacking Player.EnchantedBy", g.Obj(pwAtk.ID), sc) {
		t.Error("attacking Player.EnchantedBy must not match a creature attacking a planeswalker")
	}
	if got := UnknownPredicates("Creature.attacking Player.EnchantedBy"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Creature.attacking Player.EnchantedBy) = %v, want empty", got)
	}
}
