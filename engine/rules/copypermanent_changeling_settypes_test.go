package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 707.9d: a copy effect that provides a specific set of creature types
// does not copy a characteristic-defining ability that defines creature
// types. God-Pharaoh's Gift's "except it's a 4/4 black Zombie" sets the
// creature types, so the copied Changeling Outcast's Changeling CDA is not
// copied: the token is a Zombie and nothing else, without the keyword.
// (The "in addition to its other types" form is the rule's exception; it
// never reaches this path because AddTypes$ does not remove creature types.)
func TestGodPharaohsGiftCopyOfChangelingDropsCDA(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "God-Pharaoh's Gift"), lookup(t, reg, "Changeling Outcast")},
		[]*cards.Card{})
	moveByName(t, e, 0, "God-Pharaoh's Gift", state.ZBattlefield)
	outcast := moveByName(t, e, 0, "Changeling Outcast", state.ZGraveyard)
	if got := e.G.Obj(outcast).Zone; got != state.ZGraveyard {
		t.Fatalf("precondition: Changeling Outcast zone = %d, want graveyard", got)
	}

	// At the beginning of MY combat the optional trigger exiles the graveyard
	// creature and copies it. Drive there, then answer the trigger's optional
	// yes and hidden graveyard pick.
	driveToCombatAnswering(t, e, 0, 400)
	e.putTriggersOnStack()
	answerOptionalYes(t, e, 80)

	cid := findTokenCopyOf(t, e, e.G.Obj(outcast).Card)
	d := e.Derived(cid)
	if d.Power != 4 || d.Toughness != 4 || d.Colors != "B" {
		t.Fatalf("GPG copy = %d/%d colours %q, want the exception's 4/4 black", d.Power, d.Toughness, d.Colors)
	}
	if !copyHasTypeWord(d.Types, "Zombie") {
		t.Fatalf("GPG copy types = %v, want the exception's Zombie", d.Types)
	}
	// CR 707.9d: the copied Changeling CDA is not copied.
	if d.AllCreatureTypes {
		t.Errorf("GPG copy has every creature type; the copied Changeling CDA must not be copied")
	}
	if e.HasKeyword(cid, "Changeling") {
		t.Errorf("GPG copy kept the copied Changeling keyword: %v", d.Keywords)
	}
	if copyHasTypeWord(d.Types, "Shapeshifter") {
		t.Errorf("GPG copy kept the copied Shapeshifter subtype: %v", d.Types)
	}
	assertChangelingTypePredicate(t, e, cid, "Creature.Zombie", true)
	assertChangelingTypePredicate(t, e, cid, "Creature.Surrakar", false)
	if n := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.CopyExcept && ev.Obj == cid
	}); n != 1 {
		t.Fatalf("precondition: %d CopyExcept events for the token, want 1", n)
	}
	replayCheck(t, e, cfg)
}
