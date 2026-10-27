package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

const dnswBearSrc = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// dnswAdd puts a second object (same fixture as board's Bear) in zone for
// owner; token marks it a token.
func dnswAdd(t *testing.T, g *state.Game, owner state.PlayerID, zone state.Zone, token bool) *state.Object {
	t.Helper()
	c, d := cards.ParseBytes("t.txt", []byte(dnswBearSrc))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	o := g.AddObject(c, owner)
	o.Zone = zone
	o.IsToken = token
	g.SetZone(zone, owner, append(g.Zone(zone, owner), o.ID))
	return o
}

// TestDoesNotShareNameWithValidSpec pins Forge's valid-spec branch of
// doesNotShareNameWith (The Apprentice's Folly): the candidate passes when no
// battlefield permanent matching the argument shares its name.
func TestDoesNotShareNameWithValidSpec(t *testing.T) {
	g, id := board(t)
	const spec = "Creature.!token+doesNotShareNameWith Card.token+YouCtrl"
	bear := g.Obj(id["myBear"])
	if bear == nil || bear.IsToken {
		t.Fatal("precondition: the board's Bear is missing or a token")
	}
	if !MatchesSpecFrom(g, spec, bear.ID, 0, bear.ID) {
		t.Fatal("a nontoken creature with no same-named token did not match")
	}
	tok := dnswAdd(t, g, 0, state.ZBattlefield, true)
	if tok.Face().Name != bear.Face().Name {
		t.Fatalf("precondition: token %q and Bear %q differ in name", tok.Face().Name, bear.Face().Name)
	}
	if MatchesSpecFrom(g, spec, bear.ID, 0, bear.ID) {
		t.Fatal("a nontoken creature matched with a same-named token on the battlefield")
	}
	if !MatchesSpecFrom(g, spec, id["myFlier"], 0, bear.ID) {
		t.Fatal("a differently named creature was refused because of the Bear token")
	}
	for _, s := range []string{
		spec,
		"Aura.CanEnchantSource+cmcLEX+doesNotShareNameWith Aura.YouCtrl",
		"Room.doesNotShareNameWith Room.YouCtrl",
	} {
		for _, u := range UnknownPredicates(s) {
			if strings.Contains(u, "doesNotShareNameWith") {
				t.Errorf("UnknownPredicates(%q) reports %q", s, u)
			}
		}
	}
}

// TestDoesNotShareNameWithYourGraveyard pins the YourGraveyard spelling
// (Guardian Project): a same-named card in the source controller's graveyard
// fails the candidate.
func TestDoesNotShareNameWithYourGraveyard(t *testing.T) {
	g, id := board(t)
	const spec = "Creature.doesNotShareNameWith YourGraveyard"
	bear := g.Obj(id["myBear"])
	if !MatchesSpecFrom(g, spec, bear.ID, 0, bear.ID) {
		t.Fatal("precondition: no Bear in the graveyard yet, the creature must match")
	}
	gy := dnswAdd(t, g, 0, state.ZGraveyard, false)
	if gy.Face().Name != bear.Face().Name {
		t.Fatalf("precondition: graveyard card %q and Bear %q differ", gy.Face().Name, bear.Face().Name)
	}
	if MatchesSpecFrom(g, spec, bear.ID, 0, bear.ID) {
		t.Fatal("a creature matched with a same-named card in its controller's graveyard")
	}
	gy.Zone = state.ZExile
	g.SetZone(state.ZGraveyard, 0, nil)
	if !MatchesSpecFrom(g, spec, bear.ID, 0, bear.ID) {
		t.Fatal("the creature still failed once the graveyard card was gone")
	}
	if u := UnknownPredicates(spec); len(u) != 0 {
		t.Fatalf("UnknownPredicates(%q) = %v", spec, u)
	}
}

// TestDoesNotShareNameWithRememberedStaysUnknown: a Remembered referent needs
// a defined-card set this filter cannot bind, so it still fails closed.
func TestDoesNotShareNameWithRememberedStaysUnknown(t *testing.T) {
	u := UnknownPredicates("Card.nonLand+doesNotShareNameWith Remembered")
	if len(u) == 0 {
		t.Fatal("doesNotShareNameWith Remembered was recognised")
	}
}
