package rules

// Regressions for the effect-side "enters tapped" riders: the Tap that sets a
// permanent's entry state is emitted after the move (or token mint), so the
// entry's CR 603.6b trigger check must wait for it. Without the hold the check
// runs at the move's own emit against an untapped board and "whenever a
// permanent enters tapped" (Amulet of Vigor) never matches, leaving the
// permanent tapped.
//
// One test per fixed mover site. Each asserts its own precondition: the
// permanent actually entered and the entry-state Tap was logged, then that
// Amulet of Vigor's trigger resolved and untapped it.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// enteredTapped reports whether the log recorded id's effect-side "entered
// tapped" Tap, the entry state the trigger check must see.
func enteredTapped(e *Engine, id state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.Tap && ev.Obj == id && ev.Text == "entered tapped" {
			return true
		}
	}
	return false
}

// untapped reports whether the log recorded an Untap of id (Amulet of Vigor's
// trigger resolving).
func untapped(e *Engine, id state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.Untap && ev.Obj == id {
			return true
		}
	}
	return false
}

// answerPassingAsks resolves the non-target asks a passing board reaches:
// trigger-order choices, look acknowledgements and the bare yes/no
// confirmations an optional fetch poses. It returns the first decision it does
// not recognise (the caller's assertion point).
func answerPassingAsks(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 12; i++ {
		d := passUntilNonPriority(t, e, 40)
		if d == nil {
			return nil
		}
		switch {
		case d.Kind == decision.KTriggerOrder:
			var order []int
			for _, o := range d.Options {
				order = append(order, o.Index)
			}
			submitChoices(t, e, order...)
		case d.Kind == decision.KChoose && d.ResumeKind == "look_ack":
			submitChoices(t, e, d.Options[0].Index)
		case d.Kind == decision.KChoose && len(d.Options) == 2 && d.Options[0].Kind == "yes":
			submitChoices(t, e, d.Options[0].Index)
		default:
			return d
		}
	}
	t.Fatal("passing asks did not settle")
	return nil
}

// TestChangeZoneObjectTappedEntryFiresTappedTrigger: Zuko's Conviction's
// kicked branch returns a target creature card from a graveyard to the
// battlefield tapped through the inlined object-target mover
// (engine/effects/zone_change.go). Amulet of Vigor must see it enter tapped.
func TestChangeZoneObjectTappedEntryFiresTappedTrigger(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Zuko's Conviction", "Amulet of Vigor", "Grizzly Bears")
	onBoardCard(t, e, 0, searchCorpusCard(t, reg, "Amulet of Vigor"))
	spell := searchMoveByName(t, e, "Zuko's Conviction", state.ZHand)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZGraveyard)
	addMana(t, e, 0, "BCCCC")
	submitChoices(t, e, castModeOption(t, e, spell, "kicked"))
	driveToTargetAsk(t, e, bear)
	// Precondition: the object-target mover put the creature onto the
	// battlefield and applied its Tapped$ entry state.
	if o := e.G.Obj(bear); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Zuko return zone = %s, want battlefield", o.Zone)
	}
	if !enteredTapped(e, bear) {
		t.Fatal("precondition: no \"entered tapped\" Tap logged for the returned creature")
	}
	if !untapped(e, bear) {
		t.Fatalf("Amulet of Vigor did not trigger on the enters-tapped return; creature tapped=%v", e.G.Obj(bear).Tapped)
	}
	if e.G.Obj(bear).Tapped {
		t.Fatal("Amulet of Vigor's trigger resolved but the returned creature is still tapped")
	}
}

// TestLibraryFetchTappedEntryFiresTappedTrigger: Risen Reef's enters trigger
// looks at the top card and, when it is a land, puts that remembered card onto
// the battlefield tapped through the Defined$ library fetch in
// engine/effects/zone_search.go's moveDefinedLibraryObjects.
func TestLibraryFetchTappedEntryFiresTappedTrigger(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Risen Reef", "Amulet of Vigor")
	onBoardCard(t, e, 0, searchCorpusCard(t, reg, "Amulet of Vigor"))
	reef := searchMoveByName(t, e, "Risen Reef", state.ZHand)
	// The look's top card is a land, so the optional fetch takes the
	// battlefield-tapped branch.
	plains := putTopOfLibrary(t, e, searchCorpusCard(t, reg, "Plains"), 0)
	addMana(t, e, 0, "GU C")
	submitChoices(t, e, plainCastOption(t, e, reef).Index)
	// Acknowledge the look, accept the "you may put it onto the battlefield
	// tapped" confirmation, and let the Defined$ fetch and Amulet's trigger
	// resolve.
	answerPassingAsks(t, e)
	if o := e.G.Obj(plains); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: fetched land zone = %s, want battlefield (pending %+v)", o.Zone, e.Pending())
	}
	if !enteredTapped(e, plains) {
		t.Fatal("precondition: no \"entered tapped\" Tap logged for the fetched land")
	}
	if !untapped(e, plains) {
		t.Fatalf("Amulet of Vigor did not trigger on the enters-tapped fetch; land tapped=%v", e.G.Obj(plains).Tapped)
	}
	if e.G.Obj(plains).Tapped {
		t.Fatal("Amulet of Vigor's trigger resolved but the fetched land is still tapped")
	}
}

// TestHandPutTappedEntryFiresTappedTrigger: Zimone, Quandrix Prodigy's {1},
// {T} ability puts one land card from hand onto the battlefield tapped through
// engine/effects/zone_hand.go's hand mover (settleChangeZoneMoveAs's
// hand-origin Tapped$ arm).
func TestHandPutTappedEntryFiresTappedTrigger(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Zimone, Quandrix Prodigy", "Amulet of Vigor")
	onBoardCard(t, e, 0, searchCorpusCard(t, reg, "Amulet of Vigor"))
	zimone := onBoardCard(t, e, 0, searchCorpusCard(t, reg, "Zimone, Quandrix Prodigy"))
	e.G.Obj(zimone).SummonSick = false
	land := searchMoveByName(t, e, "Forest", state.ZHand)
	ability := -1
	for i, sa := range e.G.Obj(zimone).Face().Abilities {
		if sa.Kind == "AB" && sa.API == "ChangeZone" {
			ability = i
		}
	}
	if ability < 0 {
		t.Fatal("Zimone has no ChangeZone ability")
	}
	addMana(t, e, 0, "C")
	submitChoices(t, e, abilityOption(t, e, zimone, ability).Index)
	// The ability's "you may" confirmation, then the one-card hand pick.
	d := passUntilNonPriority(t, e, 30)
	if d != nil && d.Kind == decision.KChoose && len(d.Options) == 2 && d.Options[0].Kind == "yes" {
		submitChoices(t, e, d.Options[0].Index)
		d = passUntilNonPriority(t, e, 30)
	}
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("pending = %+v, want the hand-land pick", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == land {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("the seeded Forest is not among %+v", d.Options)
	}
	submitChoices(t, e, pick)
	// Let the hand move and Amulet's trigger resolve.
	answerPassingAsks(t, e)
	if o := e.G.Obj(land); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: hand land zone = %s, want battlefield", o.Zone)
	}
	if !enteredTapped(e, land) {
		t.Fatal("precondition: no \"entered tapped\" Tap logged for the hand land")
	}
	if !untapped(e, land) || e.G.Obj(land).Tapped {
		t.Fatalf("Amulet of Vigor did not untap the enters-tapped hand land; tapped=%v", e.G.Obj(land).Tapped)
	}
}

// TestHiddenPickTappedEntryFiresTappedTrigger: Blossoming Tortoise's enters
// trigger mills three and returns a land card from the graveyard to the
// battlefield tapped through engine/effects/zone_hidden.go's hidden pick.
func TestHiddenPickTappedEntryFiresTappedTrigger(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Blossoming Tortoise", "Amulet of Vigor")
	onBoardCard(t, e, 0, searchCorpusCard(t, reg, "Amulet of Vigor"))
	tortoise := searchMoveByName(t, e, "Blossoming Tortoise", state.ZHand)
	// Exactly one land in the graveyard: seed it, and put three nonlands on
	// top so the mill adds none.
	land := searchMoveByName(t, e, "Forest", state.ZGraveyard)
	for _, name := range []string{"Grizzly Bears", "Lightning Bolt", "Counterspell"} {
		putTopOfLibrary(t, e, searchCorpusCard(t, reg, name), 0)
	}
	addMana(t, e, 0, "GGCC")
	submitChoices(t, e, plainCastOption(t, e, tortoise).Index)
	if d := answerPassingAsks(t, e); d != nil && d.Kind == decision.KChoose {
		pick := -1
		for _, o := range d.Options {
			if o.Obj == land {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("the seeded graveyard Forest is not among %+v", d.Options)
		}
		submitChoices(t, e, pick)
		answerPassingAsks(t, e)
	}
	if o := e.G.Obj(land); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: hidden-pick land zone = %s, want battlefield (pending %+v)", o.Zone, e.Pending())
	}
	if !enteredTapped(e, land) {
		t.Fatal("precondition: no \"entered tapped\" Tap logged for the hidden-pick land")
	}
	if !untapped(e, land) || e.G.Obj(land).Tapped {
		t.Fatalf("Amulet of Vigor did not untap the enters-tapped hidden-pick land; tapped=%v", e.G.Obj(land).Tapped)
	}
}

// TestCloakTappedEntryFiresTappedTrigger: Become Anonymous cloaks three cards
// (the exiled target plus the top two) tapped through
// engine/effects/zone_manifest.go's cloak mover. Each cloak entry must fire
// Amulet of Vigor.
func TestCloakTappedEntryFiresTappedTrigger(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Become Anonymous", "Amulet of Vigor")
	onBoardCard(t, e, 0, searchCorpusCard(t, reg, "Amulet of Vigor"))
	bear := onBoardCard(t, e, 0, searchCorpusCard(t, reg, "Grizzly Bears"))
	spell := searchMoveByName(t, e, "Become Anonymous", state.ZHand)
	addMana(t, e, 0, "UUCC")
	submitChoices(t, e, plainCastOption(t, e, spell).Index)
	d := passUntilNonPriority(t, e, 30)
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the spell's target ask", d)
	}
	answerTargetAsk(t, e, []state.ObjID{bear})
	answerPassingAsks(t, e)
	var cloaked []state.ObjID
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.To == state.ZBattlefield && ev.Counter == events.CloakEntryCounter {
			cloaked = append(cloaked, ev.Obj)
		}
	}
	if len(cloaked) != 3 {
		t.Fatalf("precondition: cloak entries = %d (%v), want the target plus the top two", len(cloaked), cloaked)
	}
	for _, id := range cloaked {
		if !enteredTapped(e, id) {
			t.Fatalf("precondition: cloak entry %d has no \"entered tapped\" Tap", id)
		}
		if !untapped(e, id) {
			t.Fatalf("Amulet of Vigor did not trigger on cloak entry %d; tapped=%v", id, e.G.Obj(id).Tapped)
		}
		if e.G.Obj(id).Tapped {
			t.Fatalf("Amulet of Vigor's trigger resolved but cloak entry %d is still tapped", id)
		}
	}
}

// TestDigTappedEntryFiresTappedTrigger: Elvish Rejuvenator's enters trigger
// digs five and may put a land onto the battlefield tapped through
// engine/effects/dig.go's Dig window.
func TestDigTappedEntryFiresTappedTrigger(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Elvish Rejuvenator", "Amulet of Vigor")
	onBoardCard(t, e, 0, searchCorpusCard(t, reg, "Amulet of Vigor"))
	rejuvenator := searchMoveByName(t, e, "Elvish Rejuvenator", state.ZHand)
	// Exactly one land among the looked-at five: the seeded Plains, under
	// four nonlands.
	plains := putTopOfLibrary(t, e, searchCorpusCard(t, reg, "Plains"), 0)
	for _, name := range []string{"Grizzly Bears", "Lightning Bolt", "Counterspell", "Dark Ritual"} {
		putTopOfLibrary(t, e, searchCorpusCard(t, reg, name), 0)
	}
	addMana(t, e, 0, "GCC")
	submitChoices(t, e, plainCastOption(t, e, rejuvenator).Index)
	if d := answerPassingAsks(t, e); d != nil && d.Kind == decision.KChoose && d.ResumeKind == "dig" {
		pick := -1
		for _, o := range d.Options {
			if o.Obj == plains {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("the seeded Plains is not among the dug cards %+v", d.Options)
		}
		submitChoices(t, e, pick)
		answerPassingAsks(t, e)
	}
	if o := e.G.Obj(plains); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: dug land zone = %s, want battlefield (pending %+v)", o.Zone, e.Pending())
	}
	if !enteredTapped(e, plains) {
		t.Fatal("precondition: no \"entered tapped\" Tap logged for the dug land")
	}
	if !untapped(e, plains) || e.G.Obj(plains).Tapped {
		t.Fatalf("Amulet of Vigor did not untap the enters-tapped dug land; tapped=%v", e.G.Obj(plains).Tapped)
	}
}

// TestDigUntilTappedEntryFiresTappedTrigger: Clifftop Lookout reveals until a
// land and puts it onto the battlefield tapped through
// engine/effects/diguntil.go.
func TestDigUntilTappedEntryFiresTappedTrigger(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Clifftop Lookout", "Amulet of Vigor")
	onBoardCard(t, e, 0, searchCorpusCard(t, reg, "Amulet of Vigor"))
	lookout := searchMoveByName(t, e, "Clifftop Lookout", state.ZHand)
	var before []state.ObjID
	before = append(before, e.G.Zone(state.ZBattlefield, 0)...)
	addMana(t, e, 0, "GCC")
	submitChoices(t, e, plainCastOption(t, e, lookout).Index)
	answerPassingAsks(t, e)
	land := state.ObjID(0)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		found := false
		for _, old := range before {
			if old == id {
				found = true
			}
		}
		if found {
			continue
		}
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		for _, typ := range o.Face().Types {
			if typ == "Land" {
				land = id
			}
		}
	}
	if land == 0 {
		t.Fatalf("precondition: no land entered the battlefield; zone=%v", e.G.Zone(state.ZBattlefield, 0))
	}
	if !enteredTapped(e, land) {
		t.Fatal("precondition: no \"entered tapped\" Tap logged for the dig-until land")
	}
	if !untapped(e, land) || e.G.Obj(land).Tapped {
		t.Fatalf("Amulet of Vigor did not untap the enters-tapped dig-until land; tapped=%v", e.G.Obj(land).Tapped)
	}
}
