package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

const assimilateScript = "Name:Collective Call\nManaCost:2 B\nTypes:Sorcery\n" +
	"A:SP$ Assimilate | TgtZone$ Graveyard | ValidTgts$ Creature\nOracle:x\n"

// assimilated checks the keyword action's result on id: under seat 0, one +1/+1
// counter, a Borg artifact creature with no other creature type left.
func assimilated(t *testing.T, e *Engine, id state.ObjID, wantPower int32, gone string) {
	t.Helper()
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("assimilated card: zone %s controller %d, want the battlefield under seat 0", o.Zone, o.Controller)
	}
	d := e.Derived(id)
	for _, want := range []string{"Artifact", "Creature", "Borg"} {
		if !slices.Contains(d.Types, want) {
			t.Errorf("types %v lack %s", d.Types, want)
		}
	}
	if slices.Contains(d.Types, gone) {
		t.Errorf("types %v keep the creature type %s", d.Types, gone)
	}
	if d.Power != wantPower {
		t.Errorf("power %d, want %d with the +1/+1 counter", d.Power, wantPower)
	}
}

// TestAssimilateFromAGraveyardEntersUnderYourControl: the release notes' reading
// of a card in a graveyard. It enters already a Borg artifact creature with the
// counter, so "whenever an artifact enters" triggers on it, and the types end
// when it leaves.
func TestAssimilateFromAGraveyardEntersUnderYourControl(t *testing.T) {
	t.Parallel()
	watcher := "Name:Artifact Watcher\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Artifact.YouCtrl | TriggerZones$ Battlefield | Execute$ TrigGain | TriggerDescription$ x\n" +
		"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 5\nOracle:x\n"
	e, cfg, _ := newFixtureDeckWithOpponentCard(t, 901, assimilateScript, watcher,
		"Name:Rival Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	bear := findCardObj(t, e, 1, "Rival Bear", state.ZGraveyard)
	putCreature(t, e, 0, watcher)
	life := e.G.Players[0].Life
	effects.Resolve(e, &effects.Ctx{Controller: 0, Targets: []state.Target{{Obj: bear}}, TargetsOffered: true},
		card(t, assimilateScript).Faces[0].SpellAbility())
	assimilated(t, e, bear, 3, "Bear")
	e.Advance()
	resolveStack(t, e)
	if got := e.G.Players[0].Life; got != life+5 {
		t.Errorf("life %d, want %d: the artifact-enters trigger did not see it", got, life+5)
	}
	replayCheck(t, e, cfg)
}

// TestAssimilateAPermanentChangesControlInPlace: a permanent stays where it is
// and keeps its non-creature subtypes (the release notes' Dryad Arbor).
func TestAssimilateAPermanentChangesControlInPlace(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 902, assimilateScript)
	arbor := fixtureInZone(t, e, 1, state.ZBattlefield, "Name:Grove Dryad\nTypes:Creature Land Dryad Forest\nPT:1/1\nOracle:x\n")
	entries := len(e.L.Events)
	effects.Resolve(e, &effects.Ctx{Controller: 0, Targets: []state.Target{{Obj: arbor}}, TargetsOffered: true},
		card(t, assimilateScript).Faces[0].SpellAbility())
	assimilated(t, e, arbor, 2, "Dryad")
	if d := e.Derived(arbor); !slices.Contains(d.Types, "Land") || !slices.Contains(d.Types, "Forest") {
		t.Errorf("types %v lost Land or Forest", d.Types)
	}
	for _, ev := range e.L.Events[entries:] {
		if ev.Obj == arbor && ev.To == state.ZBattlefield {
			t.Fatalf("a permanent re-entered: %+v", ev)
		}
	}
}

// TestAssimilateEndsWhenItLeaves: the types go with the permanent; a later return
// is an ordinary card.
func TestAssimilateEndsWhenItLeaves(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 903, assimilateScript)
	bear := fixtureInZone(t, e, 1, state.ZGraveyard, "Name:Rival Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	sa := card(t, assimilateScript).Faces[0].SpellAbility()
	effects.Resolve(e, &effects.Ctx{Controller: 0, Targets: []state.Target{{Obj: bear}}, TargetsOffered: true}, sa)
	assimilated(t, e, bear, 3, "Bear")
	back := card(t, "Name:Reflow\nManaCost:2\nTypes:Sorcery\nA:SP$ ChangeZone | Origin$ Battlefield | Destination$ Graveyard | ValidTgts$ Creature\nOracle:x\n").Faces[0].SpellAbility()
	effects.Resolve(e, &effects.Ctx{Controller: 0, Targets: []state.Target{{Obj: bear}}, TargetsOffered: true}, back)
	if z := e.G.Obj(bear).Zone; z != state.ZGraveyard {
		t.Fatalf("zone %s, want the graveyard", z)
	}
	again := card(t, "Name:Raise\nManaCost:2\nTypes:Sorcery\nA:SP$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | ValidTgts$ Creature\nOracle:x\n").Faces[0].SpellAbility()
	effects.Resolve(e, &effects.Ctx{Controller: 1, Targets: []state.Target{{Obj: bear}}, TargetsOffered: true}, again)
	d := e.Derived(bear)
	if slices.Contains(d.Types, "Borg") || slices.Contains(d.Types, "Artifact") || !slices.Contains(d.Types, "Bear") {
		t.Fatalf("types after leaving and returning: %v, want an ordinary Bear", d.Types)
	}
}
