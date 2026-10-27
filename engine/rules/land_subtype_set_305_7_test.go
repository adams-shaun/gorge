package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/chars"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 305.7: "If an effect sets a land's subtype to one or more of the basic
// land types, the land no longer has its old land type. It loses all
// abilities generated from its rules text, its old land types, and any
// copiable effects affecting that land, and it gains the appropriate mana
// ability for each new basic land type. Note that this doesn't remove any
// abilities that were granted to the land by other effects. ... If a land
// gains one or more land types in addition to its own, it keeps its land
// types and rules text, and it gains the new land types and mana abilities."
//
// Every fixture is a real corpus card; Blood Moon sits on seat 1 so seat 0
// holds priority over its own lands throughout.

// animateOffered reports whether the pending priority decision offers id's
// printed AB$ Animate ability (Mutavault's "becomes a 2/2").
func animateOffered(t *testing.T, e *Engine, id state.ObjID) bool {
	t.Helper()
	for i, sa := range e.G.Obj(id).Face().Abilities {
		if sa.Kind == "AB" && sa.API == "Animate" {
			_, ok := findAbilityOption(e, id, i)
			return ok
		}
	}
	t.Fatalf("%s carries no AB$ Animate ability", e.G.Obj(id).Face().Name)
	return false
}

// TestBloodMoonStripsMutavaultRulesTextAbilities: under Blood Moon a
// Mutavault is a Mountain with ONLY "{T}: Add {R}" -- its printed animate
// ability and its printed "{T}: Add {C}" do not exist.
func TestBloodMoonStripsMutavaultRulesTextAbilities(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Mutavault")},
		[]*cards.Card{lookup(t, reg, "Blood Moon")})
	mv := moveByName(t, e, 0, "Mutavault", state.ZBattlefield)
	addMana(t, e, 0, "C")

	// Precondition: without Blood Moon both printed abilities are live.
	if !animateOffered(t, e, mv) {
		t.Fatal("precondition: Mutavault's animate ability is not offered before Blood Moon")
	}
	if mas := e.availableManaAbilities(0, mv); !pay.ManaAbilitiesProduce(mas, "C") {
		t.Fatal("precondition: Mutavault cannot tap for {C} before Blood Moon")
	}

	moveByName(t, e, 1, "Blood Moon", state.ZBattlefield)
	e.priorityRound() // re-pose priority over the new board

	if ty := e.Derived(mv).Types; !slices.Contains(ty, "Mountain") {
		t.Fatalf("Mutavault under Blood Moon types = %v, want Mountain", ty)
	}
	mas := e.availableManaAbilities(0, mv)
	if !pay.ManaAbilitiesProduce(mas, "R") {
		t.Fatal("Mutavault under Blood Moon has no {T}: Add {R} (CR 305.6 intrinsic of its new type)")
	}
	if pay.ManaAbilitiesProduce(mas, "C") {
		t.Fatal("CR 305.7: Mutavault under Blood Moon still has its printed {T}: Add {C}")
	}
	if animateOffered(t, e, mv) {
		t.Fatal("CR 305.7: Mutavault under Blood Moon still offers its printed animate ability")
	}
	replayCheck(t, e, cfg)
}

// TestBloodMoonStripsPrintedKeywordKeepsGrantedOne: Darksteel Citadel's
// printed Indestructible is rules text and goes; a keyword an OTHER effect
// granted it -- registered before Blood Moon arrived -- stays (CR 305.7's
// "doesn't remove any abilities that were granted to the land by other
// effects", which a Humility-style timestamped removal would get wrong).
func TestBloodMoonStripsPrintedKeywordKeepsGrantedOne(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Darksteel Citadel")},
		[]*cards.Card{lookup(t, reg, "Blood Moon")})
	cit := moveByName(t, e, 0, "Darksteel Citadel", state.ZBattlefield)
	granter := moveByName(t, e, 0, "Mountain", state.ZBattlefield)
	e.AddContinuous(state.ContinuousEffect{Source: granter, Affects: "Land.nonBasic", Layer: state.LAbilities,
		AddKeywords: []string{"Hexproof"}})
	if !e.HasKeyword(cit, "Indestructible") || !e.HasKeyword(cit, "Hexproof") {
		t.Fatal("precondition: Darksteel Citadel lacks printed Indestructible or granted Hexproof before Blood Moon")
	}

	moveByName(t, e, 1, "Blood Moon", state.ZBattlefield)

	if !e.HasKeyword(cit, "Hexproof") {
		t.Fatal("CR 305.7: Blood Moon removed a keyword another effect granted the land")
	}
	if e.HasKeyword(cit, "Indestructible") {
		t.Fatal("CR 305.7: Darksteel Citadel under Blood Moon keeps its printed Indestructible")
	}
	if ty := e.Derived(cit).Types; !slices.Contains(ty, "Artifact") {
		t.Fatalf("Darksteel Citadel under Blood Moon types = %v, want Artifact kept (305.7 changes no card type)", ty)
	}
}

// TestBloodMoonEndsReliquaryTowerStatic: Reliquary Tower's printed static
// ("You have no maximum hand size") is rules text, so under Blood Moon the
// maximum is back to seven.
func TestBloodMoonEndsReliquaryTowerStatic(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Reliquary Tower")},
		[]*cards.Card{lookup(t, reg, "Blood Moon")})
	moveByName(t, e, 0, "Reliquary Tower", state.ZBattlefield)
	if n := e.maxHandSizeFor(0); n != unlimitedHandSize {
		t.Fatalf("precondition: max hand size with Reliquary Tower = %d, want unlimited", n)
	}

	moveByName(t, e, 1, "Blood Moon", state.ZBattlefield)

	if n := e.maxHandSizeFor(0); n != maxHandSize {
		t.Fatalf("CR 305.7: max hand size with Reliquary Tower under Blood Moon = %d, want %d", n, maxHandSize)
	}
}

// TestSpreadingSeasStripsEnchantedLandAbilities: the Aura setter (an
// enchanted-land static that strips the land types and adds Island) strips
// the enchanted Mutavault's printed abilities and leaves {T}: Add {U}.
func TestSpreadingSeasStripsEnchantedLandAbilities(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Mutavault")},
		[]*cards.Card{lookup(t, reg, "Spreading Seas")})
	mv := moveByName(t, e, 0, "Mutavault", state.ZBattlefield)
	seas := moveByName(t, e, 1, "Spreading Seas", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: seas, IDs: []state.ObjID{mv}})
	addMana(t, e, 0, "C")

	if ty := e.Derived(mv).Types; !slices.Contains(ty, "Island") {
		t.Fatalf("enchanted Mutavault types = %v, want Island", ty)
	}
	mas := e.availableManaAbilities(0, mv)
	if !pay.ManaAbilitiesProduce(mas, "U") || pay.ManaAbilitiesProduce(mas, "C") {
		t.Fatal("CR 305.7: Spreading Seas'd Mutavault must tap for {U} only")
	}
	if animateOffered(t, e, mv) {
		t.Fatal("CR 305.7: Spreading Seas'd Mutavault still offers its printed animate ability")
	}
}

// TestFlagstonesDiesTriggerSuppressedByCR3057 pins the leaves-the-battlefield
// look-back path on the real corpus card: Flagstones of Trokair's printed
// dies trigger is queued when it leaves the battlefield normally, and is not
// queued once Spreading Seas has set its subtype to Island and CR 305.7 has
// taken its rules-text abilities.
func TestFlagstonesDiesTriggerSuppressedByCR3057(t *testing.T) {
	t.Parallel()
	moveToGraveyard := func(t *testing.T, e *Engine, flag state.ObjID) {
		t.Helper()
		e.pending = nil
		e.emit(events.Event{Kind: events.MoveZone, Obj: flag, From: state.ZBattlefield, To: state.ZGraveyard})
		if z := e.G.Obj(flag).Zone; z != state.ZGraveyard {
			t.Fatalf("Flagstones zone after the move = %v, want graveyard", z)
		}
	}

	t.Run("control_fires_without_setter", func(t *testing.T) {
		t.Parallel()
		reg := testutil.CorpusRegistry(t)
		e, _ := landTypesEngine(t, reg,
			[]*cards.Card{lookup(t, reg, "Flagstones of Trokair")}, nil)
		flag := moveByName(t, e, 0, "Flagstones of Trokair", state.ZBattlefield)
		if z := e.G.Obj(flag).Zone; z != state.ZBattlefield {
			t.Fatalf("precondition: Flagstones zone = %v, want battlefield", z)
		}
		if e.printedAbilitiesLost(e.G.Obj(flag)) {
			t.Fatal("precondition: Flagstones lost its printed abilities without a CR 305.7 setter")
		}

		e.pendingTriggers = nil
		moveToGraveyard(t, e, flag)

		if len(e.pendingTriggers) != 1 {
			t.Fatalf("Flagstones without a setter queued %d dies trigger(s), want 1", len(e.pendingTriggers))
		}
		if got := e.pendingTriggers[0].Source; got != flag {
			t.Fatalf("dies trigger source = %d, want Flagstones %d", got, flag)
		}
	})

	t.Run("suppressed_under_spreading_seas", func(t *testing.T) {
		t.Parallel()
		reg := testutil.CorpusRegistry(t)
		e, _ := landTypesEngine(t, reg,
			[]*cards.Card{lookup(t, reg, "Flagstones of Trokair")},
			[]*cards.Card{lookup(t, reg, "Spreading Seas")})
		flag := moveByName(t, e, 0, "Flagstones of Trokair", state.ZBattlefield)
		seas := moveByName(t, e, 1, "Spreading Seas", state.ZBattlefield)
		e.emit(events.Event{Kind: events.Attach, Obj: seas, IDs: []state.ObjID{flag}})
		e.pending = nil
		e.Advance()
		e.pendingTriggers = nil

		if ty := e.Derived(flag).Types; !slices.Contains(ty, "Island") {
			t.Fatalf("enchanted Flagstones types = %v, want Island", ty)
		}
		if !e.printedAbilitiesLost(e.G.Obj(flag)) {
			t.Fatal("precondition: Spreading Seas did not remove Flagstones' printed abilities")
		}

		moveToGraveyard(t, e, flag)

		if len(e.pendingTriggers) != 0 {
			t.Fatalf("CR 305.7: Flagstones enchanted by Spreading Seas queued %d dies trigger(s), want 0", len(e.pendingTriggers))
		}
	})
}

// TestAddedBasicLandTypeKeepsRulesText is the guard for 305.7's last
// sentence: a basic land type gained IN ADDITION (no RemoveLandTypes$)
// removes nothing -- Mutavault keeps its animate ability and {C}, and gains
// {R}. Correct today; must stay correct.
func TestAddedBasicLandTypeKeepsRulesText(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg, []*cards.Card{lookup(t, reg, "Mutavault")}, nil)
	mv := moveByName(t, e, 0, "Mutavault", state.ZBattlefield)
	e.AddContinuous(state.ContinuousEffect{Source: mv, Affects: "Card.Self", Layer: state.LType,
		AddTypes: []string{"Mountain"}})
	addMana(t, e, 0, "C")

	mas := e.availableManaAbilities(0, mv)
	if !pay.ManaAbilitiesProduce(mas, "C") || !pay.ManaAbilitiesProduce(mas, "R") {
		t.Fatal("Mutavault that gained Mountain in addition must tap for both {C} and {R}")
	}
	if !animateOffered(t, e, mv) {
		t.Fatal("Mutavault that gained Mountain in addition lost its animate ability")
	}
}

// TestBloodMoonUrzasSagaKeepsSagaSubtype is the type-line guard: Urza's
// Saga is an Enchantment Land -- Urza's Saga; Blood Moon strips the LAND
// subtype Urza's but the ENCHANTMENT subtype Saga stays (CR 305.7 changes
// only land subtypes). Correct today; must stay correct.
func TestBloodMoonUrzasSagaKeepsSagaSubtype(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Urza's Saga")},
		[]*cards.Card{lookup(t, reg, "Blood Moon")})
	saga := moveByName(t, e, 0, "Urza's Saga", state.ZBattlefield)
	moveByName(t, e, 1, "Blood Moon", state.ZBattlefield)
	ty := e.Derived(saga).Types
	for _, want := range []string{"Enchantment", "Land", "Saga", "Mountain"} {
		if !slices.Contains(ty, want) {
			t.Fatalf("Urza's Saga under Blood Moon types = %v, want %s", ty, want)
		}
	}
	if slices.Contains(ty, "Urza's") {
		t.Fatalf("Urza's Saga under Blood Moon types = %v, want the land subtype Urza's stripped", ty)
	}
}

// TestBloodMoonEndsLandStaticKeywordGrant: Mountain Stronghold's printed
// static ("Red legendary creatures you control have bands with other
// legendary creatures") is rules text of a land Blood Moon sets, so the
// layer-6 grant it generates stops (the source-pruning half of the fix).
func TestBloodMoonEndsLandStaticKeywordGrant(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Mountain Stronghold"), lookup(t, reg, "Krenko, Mob Boss")},
		[]*cards.Card{lookup(t, reg, "Blood Moon")})
	moveByName(t, e, 0, "Mountain Stronghold", state.ZBattlefield)
	krenko := moveByName(t, e, 0, "Krenko, Mob Boss", state.ZBattlefield)
	if !e.HasKeyword(krenko, "Bands with other") {
		t.Fatal("precondition: Krenko lacks Mountain Stronghold's bands-with-other grant")
	}

	moveByName(t, e, 1, "Blood Moon", state.ZBattlefield)

	if e.HasKeyword(krenko, "Bands with other") {
		t.Fatal("CR 305.7: Mountain Stronghold under Blood Moon still grants bands with other")
	}
}

// TestBloodMoonEndsTheWorldTreeGrant: The World Tree's printed static gives
// lands you control "{T}: Add one mana of any color" at six lands; under
// Blood Moon The World Tree is a Mountain with no rules text, so a basic
// Mountain no longer taps for any colour.
func TestBloodMoonEndsTheWorldTreeGrant(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "The World Tree")},
		[]*cards.Card{lookup(t, reg, "Blood Moon")})
	moveByName(t, e, 0, "The World Tree", state.ZBattlefield)
	var mountain state.ObjID
	for i := 0; i < 5; i++ {
		mountain = moveByName(t, e, 0, "Mountain", state.ZBattlefield)
	}
	if !pay.ManaAbilitiesProduce(e.availableManaAbilities(0, mountain), "Any") {
		t.Fatal("precondition: a basic Mountain lacks The World Tree's any-colour grant at six lands")
	}

	moveByName(t, e, 1, "Blood Moon", state.ZBattlefield)

	if pay.ManaAbilitiesProduce(e.availableManaAbilities(0, mountain), "Any") {
		t.Fatal("CR 305.7: The World Tree under Blood Moon still grants {T}: Add one mana of any color")
	}
}

// TestCorpusLandTypeWordsExcludeOtherCardTypesSubtypes: the RemoveLandTypes
// vocabulary is the LAND subtypes (CR 205.3i). A subtype a land face prints
// for another of its card types is not one: Saga (Urza's Saga, an
// Enchantment), Clue (Scene of the Crime, an Artifact), Wrenn (Wrenn and
// One, a Planeswalker). Planet stays: it is a land type that also rides a
// Plane, a card type no land face carries.
func TestCorpusLandTypeWordsExcludeOtherCardTypesSubtypes(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	words := chars.CorpusLandTypeWords(reg.Universe())
	for _, want := range []string{"Forest", "Urza's", "Desert", "Gate", "Cave", "Planet", "Town", "Sphere"} {
		if !slices.Contains(words, want) {
			t.Errorf("land-type vocabulary lacks %q: %v", want, words)
		}
	}
	for _, bad := range []string{"Saga", "Clue", "Vehicle", "Wrenn"} {
		if slices.Contains(words, bad) {
			t.Errorf("land-type vocabulary carries %q, a subtype of another card type: %v", bad, words)
		}
	}
}
