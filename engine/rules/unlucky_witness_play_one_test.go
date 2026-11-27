package rules

// The "you may play ONE of those cards" grant: an Effect (Electrostatic
// Blast's boon shape, Unlucky Witness' dies shape) exiles cards, remembers
// them, delivers a MayPlay$ True static over the remembered exile, and arms
// two Static$ True Triggers$ bodies -- SpellCast ValidCard$ Card.IsRemembered
// and LandPlayed ValidCard$ Land.IsRemembered -- whose ExileSelf body ends
// every registration of the Effect. Exactly one play is allowed: the first
// play must fire its watcher and end the grant.
//
// The Effect's ForgetOnMoved$ Exile must NOT ride those two registrations:
// sweepEffectDelayed would forget the played card from their Remembered on
// the play move itself (exile->stack, exile->battlefield), BEFORE the
// SpellCast/LandPlayed trigger evaluates ValidCard$ Card.IsRemembered -- so
// the watcher never matched, the grant never ended, and the second exiled
// card stayed offered.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// unluckyWitnessSrc is the Electrostatic Blast Effect shape (corpus
// e/electrostatic_blast.txt: DB$ Effect | StaticAbilities$ STPlay |
// Triggers$ Play1,Play2 | RememberObjects$ Remembered | ForgetOnMoved$ Exile)
// hung on a dies trigger.
const unluckyWitnessSrc = "Name:Unlucky Witness\n" +
	"ManaCost:R\n" +
	"Types:Creature Human Wizard\n" +
	"PT:3/1\n" +
	"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ DiesDig\n" +
	"SVar:DiesDig:DB$ Dig | DigNum$ 2 | Defined$ You | ChangeNum$ All | DestinationZone$ Exile | RememberChanged$ True | SubAbility$ DiesGrant\n" +
	"SVar:DiesGrant:DB$ Effect | RememberObjects$ Remembered | StaticAbilities$ GrantPlay | Triggers$ WatchCast,WatchLand | ForgetOnMoved$ Exile | SubAbility$ DiesTidy\n" +
	"SVar:GrantPlay:Mode$ Continuous | Affected$ Card.IsRemembered | AffectedZone$ Exile | MayPlay$ True | Description$ You may play one of those cards until end of turn.\n" +
	"SVar:WatchCast:Mode$ SpellCast | TriggerZones$ Command | ValidCard$ Card.IsRemembered | ValidActivatingPlayer$ You | Static$ True | Execute$ EndGrant\n" +
	"SVar:WatchLand:Mode$ LandPlayed | TriggerZones$ Command | ValidCard$ Land.IsRemembered | Static$ True | Execute$ EndGrant\n" +
	"SVar:EndGrant:DB$ ChangeZone | Destination$ Exile | Origin$ Command\n" +
	"SVar:DiesTidy:DB$ Cleanup | ClearRemembered$ True\n" +
	"Oracle:When Unlucky Witness dies, exile the top two cards of your library. Until end of turn, you may play one of those cards.\n"

func dugCreature(t *testing.T, name string) string {
	t.Helper()
	return "Name:" + name + "\nManaCost:R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
}

const dugLandSrc = "Name:Dug Land\nTypes:Land\nOracle:x\n"

// witnessToDeath casts the Witness from hand, kills it, and drains the dies
// trigger: the two seeded library-top cards end up exiled and remembered.
func witnessToDeath(t *testing.T, e *Engine, witness state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "R")
	submitChoices(t, e, castOptionFor(t, e, witness).Index)
	passUntilStackEmpty(t, e, 50)
	e.emit(events.Event{Kind: events.MoveZone, Obj: witness, From: state.ZBattlefield, To: state.ZGraveyard})
	e.priorityRound()
	passUntilStackEmpty(t, e, 50)
}

// playLandOptionFor is the may-play land's play option (the printed land
// drop route, Kind play_land, per legal_walk's addLandPlay).
func playLandOptionFor(d *decision.Decision, id state.ObjID) *decision.Option {
	for i := range d.Options {
		o := &d.Options[i]
		if o.Kind == "play_land" && o.Obj == id {
			return o
		}
	}
	return nil
}

// assertBothExiledAndOffered is the test's precondition: both Dug cards are
// in the zone the grant reads (exile) and both are offered, so the
// "Dug B is no longer offered" assertion below compares a real change.
func assertBothExiledAndOffered(t *testing.T, e *Engine, a, b state.ObjID) *decision.Decision {
	t.Helper()
	for _, id := range []state.ObjID{a, b} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZExile {
			t.Fatalf("Dug card %d zone=%v, want exile", id, o)
		}
	}
	addMana(t, e, 0, "RR")
	d := e.Pending()
	if d == nil {
		t.Fatalf("no pending decision after the grant landed")
	}
	return d
}

func TestUnluckyWitnessCastOneEndsGrant(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, unluckyWitnessSrc))
	witness := e.G.Zone(state.ZHand, 0)[0]
	seeded := seedLibraryTop(t, e, dugCreature(t, "Dug A"), dugCreature(t, "Dug B"))
	dugA, dugB := seeded[0], seeded[1]
	witnessToDeath(t, e, witness)
	d := assertBothExiledAndOffered(t, e, dugA, dugB)
	if mayPlayOption(d, dugA) == nil || mayPlayOption(d, dugB) == nil {
		t.Fatalf("the grant did not offer both exiled Dugs: %+v", d.Options)
	}
	// Play ONE: cast Dug A. Its SpellCast watcher must end the grant.
	submitChoices(t, e, mayPlayOption(d, dugA).Index)
	passUntilStackEmpty(t, e, 50)
	if o := e.G.Obj(dugA); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Dug A zone=%v, want battlefield", o)
	}
	// Refill the pool so the withholding below is the grant ending, not an
	// unaffordable printed cost.
	addMana(t, e, 0, "R")
	d = e.Pending()
	if d == nil {
		t.Fatalf("no pending decision after the may-play cast")
	}
	if mayPlayOption(d, dugB) != nil {
		t.Fatalf("Dug B still offered after casting Dug A: %+v", d.Options)
	}
}

func TestUnluckyWitnessLandPlayEndsGrant(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, unluckyWitnessSrc))
	witness := e.G.Zone(state.ZHand, 0)[0]
	seeded := seedLibraryTop(t, e, dugLandSrc, dugCreature(t, "Dug B"))
	dugLand, dugB := seeded[0], seeded[1]
	witnessToDeath(t, e, witness)
	d := assertBothExiledAndOffered(t, e, dugLand, dugB)
	if playLandOptionFor(d, dugLand) == nil || mayPlayOption(d, dugB) == nil {
		t.Fatalf("the grant did not offer the land drop and Dug B: %+v", d.Options)
	}
	// Play ONE: the land drop must fire the LandPlayed watcher and end the
	// grant.
	submitChoices(t, e, playLandOptionFor(d, dugLand).Index)
	passUntilStackEmpty(t, e, 50)
	if o := e.G.Obj(dugLand); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Dug Land zone=%v, want battlefield", o)
	}
	// The pool still holds Dug B's {R} (the land drop spent nothing): the
	// withholding below is the grant ending, not an unaffordable cost.
	d = e.Pending()
	if d == nil {
		t.Fatalf("no pending decision after the land drop")
	}
	if mayPlayOption(d, dugB) != nil {
		t.Fatalf("Dug B still offered after playing Dug Land: %+v", d.Options)
	}
}

// TestUnluckyWitnessPlusTokenWatcherEndsGrant: the watcher's ValidCard$ may
// carry IsRemembered after a "+" (the corpus' `Card.IsImprinted+IsRemembered+
// wasCastFromExile` carriers); the memory must survive the play move there too.
func TestUnluckyWitnessPlusTokenWatcherEndsGrant(t *testing.T) {
	t.Parallel()
	src := strings.Replace(unluckyWitnessSrc, "ValidCard$ Card.IsRemembered | ValidActivatingPlayer",
		"ValidCard$ Card.Creature+IsRemembered | ValidActivatingPlayer", 1)
	if src == unluckyWitnessSrc {
		t.Fatalf("fixture rewrite did not apply")
	}
	e := handEngine(t, card(t, src))
	witness := e.G.Zone(state.ZHand, 0)[0]
	seeded := seedLibraryTop(t, e, dugCreature(t, "Dug A"), dugCreature(t, "Dug B"))
	dugA, dugB := seeded[0], seeded[1]
	witnessToDeath(t, e, witness)
	d := assertBothExiledAndOffered(t, e, dugA, dugB)
	if mayPlayOption(d, dugA) == nil || mayPlayOption(d, dugB) == nil {
		t.Fatalf("the grant did not offer both exiled Dugs: %+v", d.Options)
	}
	submitChoices(t, e, mayPlayOption(d, dugA).Index)
	passUntilStackEmpty(t, e, 50)
	addMana(t, e, 0, "R")
	d = e.Pending()
	if d == nil {
		t.Fatalf("no pending decision after the may-play cast")
	}
	if mayPlayOption(d, dugB) != nil {
		t.Fatalf("Dug B still offered after casting Dug A: %+v", d.Options)
	}
}
