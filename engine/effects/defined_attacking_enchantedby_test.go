package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The Commander curses' second leg. Curse of Opulence's sub-ability names its
// token owner
// `TriggeredAttackingPlayer.Opponent+controlsCreature.attacking
// Player.EnchantedBy`, and Curse of Disturbance, Verbosity, Bounty and
// Vitality carry the same Defined$ spelling on their Draw / UntapAll /
// GainLife legs. Before the dotted-TriggeredAttackingPlayer bridge existed,
// definedSpec did not know the spelling and the resolving primitive fell back
// to the controller -- the curse's OWN controller took the second token, a
// second draw, more life, while the text names "each opponent attacking that
// player": only the seats ATTACKING the enchanted player other than the
// declaration's one attacker, which is the empty set in every game this
// engine can hold (only the active player ever declares attackers, so no
// opponent of the attacker controls an attacking creature).

// testEnchantedByCurse puts a curse on seat enchanted, controlled by ctrl,
// and asserts the precondition the empty-set read depends on: the enchanted
// seat differs from the curse's controller, so the old controller fallback
// would answer a non-empty set and the test cannot pass vacuously.
func testEnchantedByCurse(t *testing.T, reg *cards.Registry, g *state.Game, ctrl, enchanted state.PlayerID) *state.Object {
	t.Helper()
	curse := corpusObject(t, reg, g, "Curse of Opulence")
	curse.HasAttachedPlayer = true
	curse.AttachedPlayer = enchanted
	if !curse.HasAttachedPlayer || curse.AttachedPlayer != enchanted || curse.Controller != ctrl ||
		enchanted == ctrl {
		t.Fatalf("precondition failed: curse must enchant seat %d under controller %d (got %+v)", enchanted, ctrl, curse)
	}
	return curse
}

func TestTriggeredAttackingPlayerEnchantedByEmpty(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	// The curse's controller ATTACKS the player it enchants: the declaring
	// player is seat 0, the curse's controller is seat 0, the enchanted seat
	// is seat 1.
	curse := testEnchantedByCurse(t, reg, g, 0, 1)
	c := &Ctx{Source: curse.ID, Controller: 0,
		TriggerContext: TriggerContext{AttackingPlayer: state.Target{Player: 0, IsPlayer: true}}}

	spec := "TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy"
	h := &fakeHost{g: g}
	if ts := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": spec}}); len(ts) != 0 {
		t.Fatalf("the curses' second leg %q = %+v, want an empty set: the declaring player is never its own second recipient", spec, ts)
	}

	// Fail closed: an attacking player outside the game names nobody, the
	// bare referent's absent-role read.
	bad := &Ctx{Source: curse.ID, Controller: 0,
		TriggerContext: TriggerContext{AttackingPlayer: state.Target{Player: state.PlayerID(len(g.Players)), IsPlayer: true}}}
	if ts := Defined(h, bad, &cards.SA{Params: map[string]string{"Defined": spec}}); len(ts) != 0 {
		t.Fatalf("out-of-range TriggeredAttackingPlayer = %+v, want an empty set (the fail-closed absent-role read)", ts)
	}

	// The bridge reads the DECLARING player, not the resolving controller:
	// re-anchor the attack on seat 1 (the enchanted seat, controller of the
	// creatureless side) and the set is still empty -- it is not the
	// controller fallback's shape either way.
	c1 := &Ctx{Source: curse.ID, Controller: 0,
		TriggerContext: TriggerContext{AttackingPlayer: state.Target{Player: 1, IsPlayer: true}}}
	if ts := Defined(h, c1, &cards.SA{Params: map[string]string{"Defined": spec}}); len(ts) != 0 {
		t.Fatalf("second leg re-anchored on seat 1 = %+v, want an empty set (the read is the declaring player, not the controller)", ts)
	}
}

// TestAttackingEnchantedByRider pins the filter word the curse grammar rides:
// `attacking Player.EnchantedBy` is the same `attacking <PlayerSpec>` rider
// every dotted player spec arg uses -- the candidate is attacking a seat the
// shared player grammar names, here "enchanted by any enchantment" (the
// corpus's Player.EnchantedBy read, player_filter.go). Before the widening the
// dotted arg stayed wordUnknown: the matcher failed it closed seat by seat, so
// a creature attacking an enchanted seat matched nothing and the census kept
// reporting the word.
func TestAttackingEnchantedByRider(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them", "third"})

	// Seat 0 is enchanted (a curse attached to it, cast by seat 1); seat 1 is
	// not, and no other enchantment is attached anywhere else.
	curse := corpusObject(t, reg, g, "Curse of Opulence")
	curse.Controller, curse.HasAttachedPlayer, curse.AttachedPlayer = 1, true, 0
	bears := corpusObject(t, reg, g, "Grizzly Bears")
	bears.Controller, bears.IsAttacking, bears.Attacking = 1, true, 0
	notEnch := corpusObject(t, reg, g, "Grizzly Bears")
	notEnch.Controller, notEnch.IsAttacking, notEnch.Attacking = 1, true, 1
	if !curse.HasAttachedPlayer || curse.AttachedPlayer != 0 || curse.Controller != 1 {
		t.Fatal("precondition failed: the curse must enchant seat 0")
	}
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.ID != curse.ID && o.HasAttachedPlayer {
			t.Fatalf("precondition failed: a second object (%s) is attached to a player", o.Face().Name)
		}
	}
	if !bears.IsAttacking || bears.Attacking != 0 || bears.Controller != 1 {
		t.Fatalf("precondition failed: the first creature must attack seat 0 (got %+v)", bears)
	}

	sc := SpecContext{You: 1}
	if !MatchesObjectCtx(g, "Creature.attacking Player.EnchantedBy", bears, sc) {
		t.Error("attacking Player.EnchantedBy must match a creature attacking the enchanted seat 0")
	}
	if MatchesObjectCtx(g, "Creature.attacking Player.EnchantedBy", notEnch, sc) {
		t.Error("attacking Player.EnchantedBy must not match a creature attacking the unenchanted seat 1")
	}

	// The planeswalker trap, the same discriminator the base rider reads: the
	// defender is the permanent, not a player, so it never matches.
	jace := corpusObject(t, reg, g, "Jace Beleren")
	jace.Controller = 0
	pwAtk := corpusObject(t, reg, g, "Grizzly Bears")
	pwAtk.Controller, pwAtk.IsAttacking, pwAtk.Attacking, pwAtk.AttackingBattle = 1, true, 0, jace.ID
	if pwAtk.AttackingBattle == 0 || jace.ID == 0 {
		t.Fatal("precondition failed: the planeswalker attack carries no AttackingBattle")
	}
	if MatchesObjectCtx(g, "Creature.attacking Player.EnchantedBy", pwAtk, sc) {
		t.Error("attacking Player.EnchantedBy must not match a creature attacking a planeswalker")
	}

	// The census and the matcher share the one recogniser: the dotted
	// player-spec arg is known, an unqualified player word it cannot resolve
	// stays loud.
	if got := UnknownPredicates("Creature.attacking Player.EnchantedBy"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Creature.attacking Player.EnchantedBy) = %v, want empty", got)
	}
	if got := UnknownPredicates("Creature.attacking Player.Gork"); len(got) == 0 {
		t.Error("UnknownPredicates(Creature.attacking Player.Gork) must stay non-empty: an unknown qualifier stays loud")
	}
}
