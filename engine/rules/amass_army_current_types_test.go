package rules

// cr-g212: Amass must find an Army creature by its CURRENT characteristics.
// CR 701.47a asks for "an Army creature you control", and CR 702.73a makes a
// changeling every creature type, so a changeling is an Army; a printed-type
// read misses it (and any layer-4 Army grant) and mints a second Army token.
// The same read must also ignore a noncreature permanent whose printed types
// merely include Army.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// amassFixtureSrc is the brief's probe: a one-mana sorcery that amasses Orcs 2.
const amassFixtureSrc = "Name:AmassMe\nManaCost:U\nTypes:Sorcery\nA:SP$ Amass | Num$ 2 | Type$ Orc\nOracle:x\n"

// hasPrintedType reports whether o's printed face lists typ. It is the
// precondition for the class these tests cover: the OLD lookup read exactly
// this list.
func hasPrintedType(o *state.Object, typ string) bool {
	if o == nil || o.Face() == nil {
		return false
	}
	for _, t := range o.Face().Types {
		if t == typ {
			return true
		}
	}
	return false
}

// TestAmassUsesChangelingAsArmy: with a changeling creature and no Army
// token, amass must put its counters on the changeling (it is every creature
// type) and create no token.
func TestAmassUsesChangelingAsArmy(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	amass := card(t, amassFixtureSrc)
	changeling := lookup(t, reg, "Changeling Outcast")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{amass, changeling}, nil)
	army := moveByName(t, e, 0, "Changeling Outcast", state.ZBattlefield)
	moveByName(t, e, 0, "AmassMe", state.ZHand)

	// Preconditions: the changeling is on the battlefield where the lookup
	// looks, it is currently an Army creature, and its PRINTED types do not
	// say Army -- so only a current-characteristics read can find it.
	o := e.G.Obj(army)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Changeling Outcast zone = %v, want battlefield", o)
	}
	if !e.matchesSpecFrom("Creature.Army", army, 0, 0) {
		t.Fatal("precondition: Changeling Outcast does not match Creature.Army")
	}
	if hasPrintedType(o, "Army") {
		t.Fatal("precondition: Changeling Outcast's printed types already say Army")
	}
	before := len(e.G.Zone(state.ZBattlefield, 0))

	addMana(t, e, 0, "U")
	castNamed(t, e, "AmassMe")
	passUntilStackEmpty(t, e, 60)

	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != before {
		t.Fatalf("battlefield went %d -> %d objects: amass created a token instead of using the changeling", before, got)
	}
	if got := e.G.Obj(army).Counter("P1P1"); got != 2 {
		t.Fatalf("Changeling Outcast has %d +1/+1 counters, want 2", got)
	}
	if !e.matchesSpecFrom("Orc", army, 0, 0) {
		t.Fatal("Changeling Outcast did not become an Orc")
	}
	replayCheck(t, e, cfg)
}

// TestAmassRealCarrierWithChangeling drives a real Standard amass carrier
// (Tidings of War, "Amass Goblins 1") with Changeling Outcast on the board.
func TestAmassRealCarrierWithChangeling(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	changeling := lookup(t, reg, "Changeling Outcast")
	tidings := lookup(t, reg, "Tidings of War")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{tidings, changeling}, nil)
	army := moveByName(t, e, 0, "Changeling Outcast", state.ZBattlefield)
	moveByName(t, e, 0, "Tidings of War", state.ZHand)

	o := e.G.Obj(army)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Changeling Outcast zone = %v, want battlefield", o)
	}
	if !e.matchesSpecFrom("Creature.Army", army, 0, 0) {
		t.Fatal("precondition: Changeling Outcast does not match Creature.Army")
	}
	if hasPrintedType(o, "Army") {
		t.Fatal("precondition: Changeling Outcast's printed types already say Army")
	}
	before := len(e.G.Zone(state.ZBattlefield, 0))

	addMana(t, e, 0, "R")
	castNamed(t, e, "Tidings of War")
	passUntilStackEmpty(t, e, 60)

	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != before {
		t.Fatalf("battlefield went %d -> %d objects: Tidings of War created a token instead of amassing the changeling", before, got)
	}
	if got := e.G.Obj(army).Counter("P1P1"); got != 1 {
		t.Fatalf("Changeling Outcast has %d +1/+1 counters, want 1 (Tidings of War, amass 1)", got)
	}
	if !e.matchesSpecFrom("Goblin", army, 0, 0) {
		t.Fatal("Changeling Outcast did not also become a Goblin")
	}
	replayCheck(t, e, cfg)
}

// TestAmassIgnoresNoncreatureArmy: a noncreature permanent with the Army
// subtype is not an "Army creature", so amass creates its token instead.
func TestAmassIgnoresNoncreatureArmy(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	amass := card(t, amassFixtureSrc)
	idol := card(t, "Name:Idol\nTypes:Artifact Army\nOracle:x\n")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{amass, idol}, nil)
	idolID := moveByName(t, e, 0, "Idol", state.ZBattlefield)
	moveByName(t, e, 0, "AmassMe", state.ZHand)

	// Preconditions: the idol is on the battlefield, its PRINTED types include
	// Army (the old lookup would have chosen it), and it is not a creature.
	o := e.G.Obj(idolID)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Idol zone = %v, want battlefield", o)
	}
	if !hasPrintedType(o, "Army") || e.matchesSpecFrom("Creature", idolID, 0, 0) {
		t.Fatalf("precondition: Idol printed Army=%v creature=%v, want Army/not creature",
			hasPrintedType(o, "Army"), e.matchesSpecFrom("Creature", idolID, 0, 0))
	}
	before := len(e.G.Zone(state.ZBattlefield, 0))

	addMana(t, e, 0, "U")
	castNamed(t, e, "AmassMe")
	passUntilStackEmpty(t, e, 60)

	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != before+1 {
		t.Fatalf("battlefield went %d -> %d objects, want one new Army token", before, got)
	}
	if got := e.G.Obj(idolID).Counter("P1P1"); got != 0 {
		t.Fatalf("the noncreature Army has %d +1/+1 counters, want 0", got)
	}
	var tokens int
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if tok := e.G.Obj(id); tok != nil && tok.IsToken {
			tokens++
		}
	}
	if tokens != 1 {
		t.Fatalf("amass created %d tokens, want 1", tokens)
	}
	replayCheck(t, e, cfg)
}
