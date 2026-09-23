// Tests for the ATTRIBUTION of the ManaReflected producer tag (review round
// r2 on the ctms sub-shape of the frozen castfilter1/2 row): which
// permanent's printed Treasure/Cave/Desert/Snow types a reflected mana unit
// carries. The rule the three tests pin: "mana from a <Type>" is mana
// PRODUCED BY a permanent of that type (Marut's official ruling; CR 107.4h's
// snow mana is the same producer-side definition), and the producer of a
// ManaReflected body's mana is the permanent whose ability resolved the body
// -- never the object the body reflects FROM, and never changed by the
// recipient the body pays TO. Kept in its own file so the ticket cannot
// conflict on a shared test file.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// activateManaReflected submits the ManaReflected activation of obj from the
// pending priority decision (a mana ability is offered with Kind "activate",
// one option per object -- rules/legal.go's mana walk). A source with more
// than one mana ability (Pit of Offerings' plain Add {C} beside its reflect)
// then poses the stage-1 "choose a mana ability" ask, so the helper maps the
// reflect ability's index through the same availableManaAbilities walk the
// option built from and answers it. Every board here reflects a
// single-candidate colour set, so an ask AFTER that stage means the fixture
// drifted.
func activateManaReflected(t *testing.T, e *Engine, obj state.ObjID) {
	t.Helper()
	mas := e.availableManaAbilities(0, obj)
	refIdx := -1
	for i, ma := range mas {
		if ma.API == "ManaReflected" {
			refIdx = i
		}
	}
	if refIdx < 0 {
		t.Fatalf("%s carries no ManaReflected ability to activate", e.G.Obj(obj).Face().Name)
	}
	d := e.Pending()
	if d == nil {
		t.Fatal("no priority decision")
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == obj {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("%s's mana activation not offered: %+v", e.G.Obj(obj).Face().Name, d.Options)
	}
	submitChoices(t, e, idx)
	if d2 := e.Pending(); d2 != nil && d2.Kind == decision.KChoose {
		pick := -1
		for _, o := range d2.Options {
			if o.Kind == "mana" && o.Ability == refIdx {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("stage-1 mana-ability ask lacks the ManaReflected option: %+v", d2.Options)
		}
		submitChoices(t, e, pick)
	}
	if cd := e.Pending(); cd != nil && cd.Kind == decision.KChoose {
		t.Fatalf("unexpected colour ask on a one-candidate reflection: %+v", cd.Options)
	}
}

// typedTally reads one seat's typed tally for a tag word.
func typedTally(e *Engine, p state.PlayerID, tagWord string) int32 {
	for i, w := range state.TypedManaTags {
		if w == tagWord {
			return e.G.Players[p].TypedMana[i].Total()
		}
	}
	panic("unknown tag word " + tagWord)
}

// TestManaReflectedCaveSourceTagsItsReflectedMana is the CAVE half of the
// attribution (review r2): Pit of Offerings -- a real corpus Land Cave whose
// reflect ability reads the colours of the cards it exiled -- produces its
// reflected unit as CAVE mana, because the producer is the Cave permanent
// resolving the body. The exiled card's own provenance (a red creature) only
// picks the colour.
func TestManaReflectedCaveSourceTagsItsReflectedMana(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	pit := onBoardCard(t, e, 0, corpusAlternativeCard(t, "Pit of Offerings"))
	// Preconditions: the producer is where the rule reads it, prints the Cave
	// type the head filters on, and the pool starts empty.
	po := e.G.Obj(pit)
	if po == nil || po.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Pit of Offerings not on the battlefield: %+v", po)
	}
	isCave := false
	for _, ty := range po.Face().Types {
		if ty == "Cave" {
			isCave = true
		}
	}
	if !isCave {
		t.Fatal("precondition: Pit of Offerings does not print the Cave type")
	}
	if got := e.G.Players[0].TypedMana[state.TypedCave].Total(); got != 0 {
		t.Fatalf("precondition: Cave tally = %d, want 0 before the activation", got)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: pool = %d, want 0 before the activation", got)
	}

	// A red creature card in seat 0's graveyard, exiled BY the Pit. The
	// MoveZone event's IDs payload records the exiling source exactly the way
	// a real ChangeZone body does (events/apply.go's default ExiledWith
	// branch), which is the record the ability's Valid$ Defined.ExiledWith
	// reads.
	red := e.G.AddObject(card(t, "Name:Red Remnant\nManaCost:2 R\nTypes:Creature Goblin\nPT:2/2\nOracle:x\n"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: red.ID, From: state.ZLibrary, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.MoveZone, Obj: red.ID, From: state.ZGraveyard, To: state.ZExile, IDs: []state.ObjID{pit}})
	if o := e.G.Obj(red.ID); o.Zone != state.ZExile || o.ExiledWith != pit {
		t.Fatalf("precondition: exiled card zone=%s ExiledWith=%d, want Exile/%d", o.Zone, o.ExiledWith, pit)
	}

	e.pending = nil
	e.priorityRound()
	activateManaReflected(t, e, pit)

	// The reflected red unit is in the pool AND carries the CAVE tag -- the
	// source's type, not the exiled card's colour name.
	if got := e.G.Players[0].Pool[state.MR]; got != 1 {
		t.Fatalf("pool R = %d, want 1 (the exiled card's colour)", got)
	}
	if got := e.G.Players[0].TypedMana[state.TypedCave][state.MR]; got != 1 {
		t.Fatalf("Cave tally R = %d, want 1 (a Cave source's reflected unit is Cave mana)", got)
	}
	for _, tagWord := range state.TypedManaTags {
		if tagWord == "Cave" {
			continue
		}
		if got := typedTally(e, 0, tagWord); got != 0 {
			t.Fatalf("%s tally = %d, want 0 (only the Cave tag is earned here)", tagWord, got)
		}
	}
	if got := e.G.Players[0].Snow[state.MR]; got != 0 {
		t.Fatalf("snow tally R = %d, want 0 (a Cave is not snow)", got)
	}
}

// TestManaReflectedAttributionIsTheAbilitySourceNotTheReflectedSet pins the
// attribution rule in BOTH directions on real corpus carriers:
//
//   - source-side: Cactus Preserve (a Desert) reflecting a unit a SNOW
//     mountain could produce tags the unit DESERT -- the producer's type --
//     and never snow, whatever the reflected object's types are;
//   - set-side: Exotic Orchard (a plain Land) reflecting from an opponent's
//     DESERT produces PLAIN red -- the reflected set's types never tag a
//     unit the Orchard's own ability produced.
func TestManaReflectedAttributionIsTheAbilitySourceNotTheReflectedSet(t *testing.T) {
	t.Parallel()

	// Direction 1: the SOURCE's type tags, even when the reflected object is
	// a different producer type.
	e := handEngine(t)
	preserve := onBoardCard(t, e, 0, corpusAlternativeCard(t, "Cactus Preserve"))
	snowy := onBoard(t, e, 0, "Name:Snowy Slope\nManaCost:no cost\nTypes:Basic Snow Land Mountain\n"+
		"A:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n")
	isSnow := false
	for _, ty := range e.G.Obj(snowy).Face().Types {
		if ty == "Snow" {
			isSnow = true
		}
	}
	if !isSnow {
		t.Fatal("precondition: Snowy Slope does not print the Snow supertype")
	}
	if got := e.G.Players[0].TypedMana[state.TypedDesert].Total(); got != 0 {
		t.Fatalf("precondition: Desert tally = %d, want 0 before the activation", got)
	}
	if got := e.G.Players[0].Snow[state.MR]; got != 0 {
		t.Fatalf("precondition: snow tally R = %d, want 0 before the activation", got)
	}

	e.pending = nil
	e.priorityRound()
	activateManaReflected(t, e, preserve)

	if got := e.G.Players[0].Pool[state.MR]; got != 1 {
		t.Fatalf("pool R = %d, want 1 (the snow mountain's producible red)", got)
	}
	if got := e.G.Players[0].TypedMana[state.TypedDesert][state.MR]; got != 1 {
		t.Fatalf("Desert tally R = %d, want 1 (the producer is the Desert source, not the snow set)", got)
	}
	if got := e.G.Players[0].Snow[state.MR]; got != 0 {
		t.Fatalf("snow tally R = %d, want 0 (the reflected set's Snow supertype must not tag)", got)
	}

	// Direction 2: the REFLECTED SET's type does not tag -- Exotic Orchard's
	// reflected red stays plain.
	e2 := handEngine(t)
	orchard := onBoardCard(t, e2, 0, corpusAlternativeCard(t, "Exotic Orchard"))
	onBoard(t, e2, 1, "Name:Guest Dune\nManaCost:no cost\nTypes:Land Desert\n"+
		"A:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n")
	if got := e2.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: pool = %d, want 0 before the activation", got)
	}

	e2.pending = nil
	e2.priorityRound()
	activateManaReflected(t, e2, orchard)

	if got := e2.G.Players[0].Pool[state.MR]; got != 1 {
		t.Fatalf("pool R = %d, want 1 (the guest Desert's producible red)", got)
	}
	for _, tagWord := range state.TypedManaTags {
		if got := typedTally(e2, 0, tagWord); got != 0 {
			t.Fatalf("%s tally = %d, want 0 (Exotic Orchard is not a %s; the reflected set's types never tag)", tagWord, got, tagWord)
		}
	}
	if got := e2.G.Players[0].Snow[state.MR]; got != 0 {
		t.Fatalf("snow tally R = %d, want 0 (the reflected set's types never tag)", got)
	}
}

// desertFlareCarrier is the Produced-trigger shape with a typed producer: the
// Mana Flare script (the real corpus Produced shape -- Mana Flare itself is
// an untagged Enchantment) on a synthetic Desert carrier. No corpus card
// pairs a typed permanent with a ReflectProperty$ Produced reflection
// (measured at the pin: Cactus Preserve and Pit of Offerings are the only
// typed ManaReflected sources, both activated AB$ bodies), so the fixture is
// synthetic; the script shape is Mana Flare's real one.
const desertFlareCarrier = "Name:Ctms Dune\nManaCost:2 R\nTypes:Land Desert\n" +
	"T:Mode$ TapsForMana | ValidCard$ Land | Execute$ TrigMana | TriggerZones$ Battlefield | Static$ True | TriggerDescription$ Whenever a player taps a land for mana, that player adds one mana of any type that land produced.\n" +
	"SVar:TrigMana:DB$ ManaReflected | ColorOrType$ Type | ReflectProperty$ Produced | Defined$ TriggeredActivator\nOracle:x\n"

// TestProducedShapeTagsItsSourceNotTheReflectedLand covers the redirect arm
// the reflect body offers: ReflectProperty$ Produced reads the types the
// TAPPED land produced and Defined$ TriggeredActivator pays the mana to the
// OTHER seat -- and the unit is still tagged from the CARRIER's Desert types.
// The tapped Mountain's types never reach the Counter, and the recipient owns
// the unit at spend time without becoming its producer.
func TestProducedShapeTagsItsSourceNotTheReflectedLand(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	dune := onBoard(t, e, 0, desertFlareCarrier)
	mount := onBoard(t, e, 1, mountainScript())
	// Preconditions: the carrier prints the Desert type the head filters on;
	// the recipient seat starts empty in pool and tallies.
	do := e.G.Obj(dune)
	if do == nil || do.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Ctms Dune not on the battlefield: %+v", do)
	}
	isDesert := false
	for _, ty := range do.Face().Types {
		if ty == "Desert" {
			isDesert = true
		}
	}
	if !isDesert {
		t.Fatal("precondition: Ctms Dune does not print the Desert type")
	}
	if got := e.G.Players[1].Pool.Total(); got != 0 {
		t.Fatalf("precondition: seat 1 pool = %d, want 0 before the tap", got)
	}
	if got := e.G.Players[1].TypedMana[state.TypedDesert].Total(); got != 0 {
		t.Fatalf("precondition: seat 1 Desert tally = %d, want 0 before the tap", got)
	}

	sa := cards.ResolveSVar(do.Face().SVars, "TrigMana")
	if sa == nil || sa.Params["ReflectProperty"] != "Produced" {
		t.Fatalf("the carrier's real Produced-reflection SVar changed: %+v", sa)
	}
	tc := e.triggerReferents(do.Face().Triggers[0], dune,
		events.Event{Kind: events.ManaAdd, Player: 1, Obj: mount, Counter: "R", Amount: 1}, nil)
	effects.Resolve(e, &effects.Ctx{Source: dune, Controller: 0, TriggerContext: tc}, sa)

	// The recipient is the tapping seat, the unit is tagged DESERT from the
	// carrier -- the tapped Mountain's own types are nowhere in the tag.
	if got := e.G.Players[1].Pool[state.MR]; got != 1 {
		t.Fatalf("seat 1 pool R = %d, want 1 (TriggeredActivator receives the mana)", got)
	}
	if got := e.G.Players[1].TypedMana[state.TypedDesert][state.MR]; got != 1 {
		t.Fatalf("seat 1 Desert tally R = %d, want 1 (the CARRIER's Desert type tags, not the tapped Mountain's)", got)
	}
	if got := e.G.Players[1].Snow[state.MR]; got != 0 {
		t.Fatalf("seat 1 snow tally R = %d, want 0", got)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("seat 0 pool = %d, want 0 (the redirect does not pay the source's controller)", got)
	}
}
