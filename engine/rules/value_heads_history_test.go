package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// value_heads_history_test.go pins the zone, last-known-information and
// player-count Count$ heads of effects/count_history_heads.go on the corpus
// bodies that carry them. Each assertion first proves its precondition (the
// live read differs from the answer, the group has the seats the read must
// span) so a head that reads the wrong thing cannot pass by accident.

const historyBearSrc = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// historyEval evaluates one Count$ expression with src as the source object
// and seat 0 as the resolving controller.
func historyEval(t *testing.T, e *Engine, src state.ObjID, svars map[string]string, expr string) int32 {
	t.Helper()
	ctx := &effects.Ctx{Source: src, Controller: 0, SVars: svars}
	n, ok := effects.EvalCountOK(e, ctx, expr)
	if !ok {
		t.Fatalf("EvalCountOK(%q) did not resolve", expr)
	}
	return n
}

func historyCorpusToZone(t *testing.T, e *Engine, p state.PlayerID, name string, to state.Zone) state.ObjID {
	t.Helper()
	o := e.G.AddObject(mustCorpusCard(t, testutil.CorpusRegistry(t), name), p)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: to})
	return o.ID
}

// TestLastStateGraveyardReadsLKI: Voracious Brood's "creature card in your
// graveyard" count is the graveyard as it stood when the spell was cast
// (CR 608.2h), not the one a response has since emptied.
func TestLastStateGraveyardReadsLKI(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	brood := historyCorpusToZone(t, e, 0, "Voracious Brood", state.ZStack)
	f := e.G.Obj(brood).Face()
	body := f.SVars["X"]
	if body != "Count$LastStateGraveyard Creature.YouCtrl" {
		t.Fatalf("corpus body = %q", body)
	}
	a := historyCorpusToZone(t, e, 0, "Grizzly Bears", state.ZGraveyard)
	historyCorpusToZone(t, e, 0, "Grizzly Bears", state.ZGraveyard)
	// The cast freezes the graveyards (castBattlefieldEvent, taken as the
	// spell reaches the stack), THEN a response exiles one creature card.
	fz, ok := castBattlefieldEvent(e.G, e, brood, 0)
	if !ok {
		t.Fatal("a face reading Count$LastStateGraveyard takes no as-cast freeze")
	}
	e.emit(fz)
	e.emit(events.Event{Kind: events.MoveZone, Obj: a, From: state.ZGraveyard, To: state.ZExile})

	if live := historyEval(t, e, brood, f.SVars, "Count$ValidGraveyard Creature.YouCtrl"); live != 1 {
		t.Fatalf("precondition: live graveyard holds %d creature cards, want 1", live)
	}
	if got := historyEval(t, e, brood, f.SVars, body); got != 2 {
		t.Fatalf("Count$LastStateGraveyard = %d, want 2 (the graveyard as cast)", got)
	}
	if got := historyEval(t, e, brood, f.SVars, "Count$LastStateGraveyardWithFallback Creature.YouCtrl"); got != 2 {
		t.Fatalf("WithFallback = %d, want 2", got)
	}
}

// TestPresenceHeadRevealedDragon: Draconic Roar's Count$Presence_Dragon.1.0
// holds when the optional reveal cost was paid, or when a Dragon was
// controlled as the spell was cast, and not otherwise.
func TestPresenceHeadRevealedDragon(t *testing.T) {
	t.Parallel()
	expr := "Count$Presence_Dragon.1.0"
	cast := func(e *Engine) state.ObjID {
		return historyCorpusToZone(t, e, 0, "Draconic Roar", state.ZStack)
	}
	// Neither a reveal nor a Dragon: the no branch.
	e := layerEngine(t)
	roar := cast(e)
	if o := e.G.Obj(roar).Face(); !strings.Contains(o.SVars["DBDamageController"], "Count$Presence_Dragon.1.0") {
		t.Fatalf("corpus gate moved: %q", o.SVars["DBDamageController"])
	}
	if got := historyEval(t, e, roar, nil, expr); got != 0 {
		t.Fatalf("no Dragon, no reveal: Presence = %d, want 0", got)
	}
	// The pay-time CastInfo of a paid optional (reveal) cost.
	e.emit(events.Event{Kind: events.CastInfo, Obj: roar, Counter: events.FlagsString(state.FlagOptionalCostPaid)})
	if got := historyEval(t, e, roar, nil, expr); got != 1 {
		t.Fatalf("Dragon revealed: Presence = %d, want 1", got)
	}
	// A Dragon on the battlefield as the spell was cast, nothing revealed.
	e2 := layerEngine(t)
	onBoard(t, e2, 0, "Name:Drake\nManaCost:4 R\nTypes:Creature Dragon\nPT:4/4\nOracle:x\n")
	roar2 := cast(e2)
	fz, ok := castBattlefieldEvent(e2.G, e2, roar2, 0)
	if !ok {
		t.Fatal("a face reading Count$Presence_ takes no as-cast freeze")
	}
	e2.emit(fz)
	if got := historyEval(t, e2, roar2, nil, expr); got != 1 {
		t.Fatalf("Dragon controlled as cast: Presence = %d, want 1", got)
	}
}

// TestPlayerCountOtherHighestValidFourSeats: at a four-seat table the "other
// players" group is all three other seats, so the highest creature count is
// the best of three, not the one opponent a two-seat read would see.
func TestPlayerCountOtherHighestValidFourSeats(t *testing.T) {
	t.Parallel()
	e := New(seatZeroStart(fourSeatConfig(t, 40, 0)))
	if len(e.G.Players) != 4 {
		t.Fatalf("precondition: %d seats, want 4", len(e.G.Players))
	}
	onBoard(t, e, 0, historyBearSrc)
	onBoard(t, e, 1, historyBearSrc)
	for i := 0; i < 3; i++ {
		onBoard(t, e, 2, historyBearSrc)
	}
	for i := 0; i < 2; i++ {
		onBoard(t, e, 3, historyBearSrc)
	}
	src := onBoard(t, e, 0, historyBearSrc)
	other := "PlayerCountOther$HighestValid Creature.YouCtrl"
	if got := historyEval(t, e, src, nil, other); got != 3 {
		t.Fatalf("%s = %d, want 3 (seat 2 of the three other seats)", other, got)
	}
	// Seat 1 resolving: the others include the active seat 0 (2 creatures + src).
	ctx := &effects.Ctx{Source: src, Controller: 1}
	if n, ok := effects.EvalCountOK(e, ctx, other); !ok || n != 3 {
		t.Fatalf("seat 1: %s = %d (ok %v), want 3", other, n, ok)
	}
	// Rivalry/Greener Pastures: every player but the ACTIVE one, from the
	// resolving seat's perspective.
	if e.G.Active != 0 {
		t.Fatalf("precondition: active seat %d, want 0", e.G.Active)
	}
	for i := 0; i < 4; i++ {
		onBoard(t, e, 1, "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n")
	}
	nonActive := "PlayerCountNonActive$HighestValid Land.YouCtrl"
	if got := historyEval(t, e, src, nil, nonActive); got != 4 {
		t.Fatalf("%s = %d, want 4", nonActive, got)
	}
}

// TestRandomHeadUsesSeededStream: Count$Random.<lo>.<hi> draws only from the
// engine's seeded generator, inclusive of both ends.
func TestRandomHeadUsesSeededStream(t *testing.T) {
	t.Parallel()
	draw := func(seed uint64) []int32 {
		e := New(seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
			Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}))
		src := onBoard(t, e, 0, historyBearSrc)
		out := make([]int32, 24)
		for i := range out {
			out[i] = historyEval(t, e, src, nil, "Count$Random.0.1000")
		}
		return out
	}
	a, b := draw(7), draw(7)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at draw %d: %d vs %d", i, a[i], b[i])
		}
		if a[i] < 0 || a[i] > 1000 {
			t.Fatalf("draw %d = %d outside 0..1000", i, a[i])
		}
	}
	c := draw(8)
	differs := false
	for i := range a {
		if a[i] != c[i] {
			differs = true
		}
	}
	if !differs {
		t.Fatalf("seeds 7 and 8 produced the identical 24-draw stream %v", a)
	}
	// Both ends of a tight range are reachable and nothing escapes it.
	e := New(seatZeroStart(Config{Seed: 3, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}))
	src := onBoard(t, e, 0, historyBearSrc)
	seen := map[int32]bool{}
	for i := 0; i < 200; i++ {
		seen[historyEval(t, e, src, nil, "Count$Random.1.3")] = true
	}
	if len(seen) != 3 || !seen[1] || !seen[3] {
		t.Fatalf("Random.1.3 produced %v, want exactly {1,2,3}", seen)
	}
}

// historyHeadCards names every card the zone-history head class unlocks.
var historyHeadCards = []string{
	"Marrow Chomper", "Skullmulcher", "Tar Fiend", "Voracious Dragon", "Polukranos, Unchained",
	"Voracious Brood", "Aether Burst", "Spoils of War", "Undercity Upheaval",
	"Draconic Roar", "Foul-Tongue Invocation", "Silumgar's Scorn", "Olivia's Midnight Ambush",
	"Wolf Strike", "Multani, Maro-Sorcerer", "Sage of Ancient Lore", "Seal of the Guildpact",
	"Tablet of the Guilds", "Archive Trap", "Reverse Polarity", "Blazing Effigy",
	"Odric, Blood-Cursed", "Embodiment of Agonies", "Reset", "Psychic Battle", "Fblthp, the Lost",
	"Takeno, Samurai General", "Call to Arms", "Surge of Acclaim",
	"Advice from the Fae", "Peer Pressure", "Greener Pastures", "Rivalry", "Enemy of Enlightenment",
	"Needlebite Trap", "Celestial Mantle", "Personal Incarnation",
	"Undercellar Sweep", "Ashuza's Breath", "Lydari Elephant", "Murgish Cemetery",
	"Necropolis of Azar", "Rainbow Knights", "Tornellan Protector",
}

// historyHeadsDeferred are the class's cards whose head needs history gorge
// does not record yet (a per-object regeneration count, the emerge sacrifice,
// the per-turn stack high-water mark, the per-player StoreSVar accumulator).
// Each is expected to stay held out by exactly that one count: primitive; the
// test fails when one becomes modelled so the entry is removed with it.
var historyHeadsDeferred = map[string]string{
	"Adipose Offspring": "count:Emerged",
	"Spiny Starfish":    "count:RegeneratedThisTurn",
	"Plague of Vermin":  "count:PlayerCountRemembered$SVarAmount",
}

// TestEveryZoneHistoryHeadCardIsPlayable: no card of the class is held out of
// the playable pool by a count: primitive. A card that still names another
// unsupported primitive (a TwoColors$ choice, say) is a different class; the
// deferred cards are pinned to their one remaining head.
func TestEveryZoneHistoryHeadCardIsPlayable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	countHeads := func(name string) []string {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Errorf("corpus has no %q", name)
			return nil
		}
		var out []string
		for _, p := range reg.Unsupported(c, sup) {
			if strings.HasPrefix(p, cards.ValueHeadPrefix) {
				out = append(out, p)
			}
		}
		return out
	}
	for _, name := range historyHeadCards {
		if heads := countHeads(name); len(heads) > 0 {
			t.Errorf("%s: count head still unmodelled: %v", name, heads)
		}
	}
	for name, want := range historyHeadsDeferred {
		if heads := countHeads(name); len(heads) != 1 || heads[0] != want {
			t.Errorf("%s: unmodelled count heads = %v, want exactly [%s] (a modelled head leaves historyHeadsDeferred)", name, heads, want)
		}
	}
}

// TestDevouredHeadCountsDevouredCreatures: Marrow Chomper (Devour 2) devours
// two creatures and its ETB trigger gains 2 life for each, 4 in all; a decline
// devours nothing and gains nothing.
func TestDevouredHeadCountsDevouredCreatures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		devoured int
		wantLife int32
	}{{"two devoured", 2, 4}, {"declined", 0, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			e := handEngine(t,
				corpusAlternativeCard(t, "Marrow Chomper"),
				corpusAlternativeCard(t, "Grizzly Bears"),
				corpusAlternativeCard(t, "Grizzly Bears"))
			fodder := devourFodder(t, e, "Grizzly Bears", "Grizzly Bears")
			life := e.G.Players[0].Life
			d := enterDevourerAsk(t, e, "Marrow Chomper")
			if d == nil || d.ResumeKind != "sacrifice" || d.Max != 2 {
				t.Fatalf("precondition: sacrifice ask %+v, want sacrifice 0..2", d)
			}
			var picks []int
			for i := 0; i < tc.devoured; i++ {
				picks = append(picks, d.Options[i].Index)
			}
			submitChoices(t, e, picks...)
			for i, id := range fodder {
				if gone := e.G.Obj(id).Zone == state.ZGraveyard; gone != (i < tc.devoured) {
					t.Fatalf("fodder %d in graveyard = %v, want %v", i, gone, i < tc.devoured)
				}
			}
			for e.putTriggersOnStack() {
			}
			if tc.devoured > 0 && len(e.G.Stack) == 0 {
				t.Fatal("precondition: the devourer's ETB trigger is not on the stack")
			}
			for n := 0; n < 4 && len(e.G.Stack) > 0; n++ {
				e.resolveTop()
			}
			if got := e.G.Players[0].Life - life; got != tc.wantLife {
				t.Fatalf("life change = %d, want %d (2 per devoured creature)", got, tc.wantLife)
			}
		})
	}
}

// TestSearchedLibraryHeadCountsSearchersThisTurn: Archive Trap's "an opponent
// searched their library this turn" reads the searching seat's per-turn
// watcher, which a TurnChange clears.
func TestSearchedLibraryHeadCountsSearchersThisTurn(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	src := onBoard(t, e, 0, historyBearSrc)
	const expr = "Count$SearchedLibrary.Opponent"
	if got := historyEval(t, e, src, nil, expr); got != 0 {
		t.Fatalf("before any search: %d, want 0", got)
	}
	// Seat 0 searching its own library is not an opponent searching.
	e.emit(events.Event{Kind: events.SearchedLibrary, Player: 0})
	if got := historyEval(t, e, src, nil, expr); got != 0 {
		t.Fatalf("own search: %d, want 0", got)
	}
	e.emit(events.Event{Kind: events.SearchedLibrary, Player: 1})
	if got := historyEval(t, e, src, nil, expr); got != 1 {
		t.Fatalf("opponent searched: %d, want 1", got)
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 1})
	if got := historyEval(t, e, src, nil, expr); got != 0 {
		t.Fatalf("next turn: %d, want 0 (the watcher resets)", got)
	}
}

// TestHistoryHeadsReadTheirState walks the remaining heads of the class over
// their corpus spellings, each against a board whose read differs from the
// zero a never-modelled head would answer.
func TestHistoryHeadsReadTheirState(t *testing.T) {
	t.Parallel()
	const (
		redBear = "Name:RBear\nManaCost:1 R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
		grnBear = "Name:GBear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
		bluBear = "Name:UBear\nManaCost:1 U\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	)
	check := func(t *testing.T, got, want int32, what string) {
		t.Helper()
		if got != want {
			t.Fatalf("%s = %d, want %d", what, got, want)
		}
	}
	t.Run("branch predicates", func(t *testing.T) {
		e := layerEngine(t)
		src := onBoard(t, e, 0, historyBearSrc)
		o := e.G.Obj(src)
		check(t, historyEval(t, e, src, nil, "Count$Night.13.2"), 2, "Night by day-less game")
		e.G.DayNight = state.DayNightNight
		check(t, historyEval(t, e, src, nil, "Count$Night.13.2"), 13, "Night")
		check(t, historyEval(t, e, src, nil, "Count$MaxSpeed.2.1"), 1, "MaxSpeed below max")
		e.G.Players[0].Speed = 4
		check(t, historyEval(t, e, src, nil, "Count$MaxSpeed.2.1"), 2, "MaxSpeed at max")
		check(t, historyEval(t, e, src, nil, "Count$Escaped.12.6"), 6, "Escaped, cast normally")
		o.CastFlags |= state.FlagEscaped
		check(t, historyEval(t, e, src, nil, "Count$Escaped.12.6"), 12, "Escaped")
		check(t, historyEval(t, e, src, nil, "Count$wasCastFromYourLibrary.2.1"), 1, "wasCastFromYourLibrary, uncast")
		o.HasLatestCast, o.LatestCastFrom = true, state.ZLibrary
		check(t, historyEval(t, e, src, nil, "Count$wasCastFromYourLibrary.2.1"), 2, "wasCastFromYourLibrary")
	})
	t.Run("hands and upkeeps", func(t *testing.T) {
		e := layerEngine(t)
		src := onBoard(t, e, 0, historyBearSrc)
		want := int32(len(e.G.Zone(state.ZHand, 0)) + len(e.G.Zone(state.ZHand, 1)))
		inZoneCard(t, e, 1, state.ZHand, historyBearSrc)
		inZoneCard(t, e, 1, state.ZHand, historyBearSrc)
		check(t, historyEval(t, e, src, nil, "Count$NumInAllHands"), want+2, "NumInAllHands")
		e.G.UpkeepsThisTurn, e.G.Step = 1, state.StepUpkeep
		check(t, historyEval(t, e, src, nil, "Count$FinishedUpkeepsThisTurn"), 0, "FinishedUpkeeps in the upkeep")
		e.G.Step = state.StepMain1
		check(t, historyEval(t, e, src, nil, "Count$FinishedUpkeepsThisTurn"), 1, "FinishedUpkeeps after it")
	})
	t.Run("graveyard mana costs and imprint", func(t *testing.T) {
		e := layerEngine(t)
		src := onBoard(t, e, 0, historyBearSrc)
		inZoneCard(t, e, 0, state.ZGraveyard, grnBear)
		inZoneCard(t, e, 0, state.ZGraveyard, grnBear)
		inZoneCard(t, e, 0, state.ZGraveyard, "Name:Wolf\nManaCost:2 G\nTypes:Creature Wolf\nPT:3/3\nOracle:x\n")
		inZoneCard(t, e, 0, state.ZGraveyard, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n")
		check(t, historyEval(t, e, src, nil, "Count$ExactManaCostGraveyard Card.nonLand+YouOwn"), 2, "ExactManaCostGraveyard")
		three := inZoneCard(t, e, 0, state.ZLibrary, "Name:Big\nManaCost:2 G\nTypes:Creature Wolf\nPT:3/3\nOracle:x\n")
		e.G.Obj(src).Imprinted = []state.ObjID{three}
		check(t, historyEval(t, e, src, nil, "Count$ImprintedCardManaCost"), 3, "ImprintedCardManaCost")
	})
	t.Run("keywords and colours", func(t *testing.T) {
		e := layerEngine(t)
		onBoard(t, e, 0, "Name:Flyer\nManaCost:1 G\nTypes:Creature Bird\nPT:1/1\nK:Flying\nOracle:x\n")
		onBoard(t, e, 0, "Name:Flyer2\nManaCost:1 G\nTypes:Creature Bird\nPT:1/1\nK:Flying\nK:Haste\nOracle:x\n")
		onBoard(t, e, 1, "Name:Trampler\nManaCost:1 G\nTypes:Creature Beast\nPT:1/1\nK:Trample\nOracle:x\n")
		samurai := onBoard(t, e, 0, "Name:Sam\nManaCost:1 W\nTypes:Creature Human Samurai\nPT:1/1\nK:Bushido:2\nK:Bushido:1\nOracle:x\n")
		check(t, historyEval(t, e, samurai, nil, "Count$AbilityYouCtrl Flying,First Strike,Haste,Trample"), 2, "AbilityYouCtrl")
		check(t, historyEval(t, e, samurai, nil, "Count$TotalValue Bushido"), 3, "TotalValue")
		for _, s := range []string{redBear, redBear, redBear, grnBear, grnBear, bluBear} {
			onBoard(t, e, 0, s)
		}
		check(t, historyEval(t, e, samurai, nil, "Count$SecondMostProminentColor Permanent.YouCtrl"), 3, "SecondMostProminentColor (green 4, red 3)")
		onBoard(t, e, 0, grnBear)
		check(t, historyEval(t, e, samurai, nil, "Count$SecondMostProminentColor Permanent.YouCtrl"), 3, "SecondMostProminentColor (green 5, red 3)")
		// Seal of the Guildpact / Tablet of the Guilds: the card shares one of the chosen colours.
		e.G.Obj(samurai).ChosenColor = "Red,Green"
		spell := inZoneCard(t, e, 0, state.ZStack, "Name:Bolt\nManaCost:R G\nTypes:Instant\nOracle:x\n")
		ctx := &effects.Ctx{Source: samurai, Controller: 0}
		ctx.TriggerCard = spell
		if n, ok := effects.EvalCountOK(e, ctx, "Count$HasNumChosenColors.TriggeredCard"); !ok || n != 2 {
			t.Fatalf("HasNumChosenColors.TriggeredCard = %d (ok %v), want 2", n, ok)
		}
		ctx.AffectedObj = inZoneCard(t, e, 0, state.ZStack, "Name:Spark\nManaCost:R\nTypes:Instant\nOracle:x\n")
		if n, ok := effects.EvalCountOK(e, ctx, "Count$HasNumChosenColors.OriginalHost"); !ok || n != 1 {
			t.Fatalf("HasNumChosenColors.OriginalHost = %d (ok %v), want 1", n, ok)
		}
	})
	t.Run("damage ledger", func(t *testing.T) {
		e := layerEngine(t)
		art := onBoard(t, e, 1, "Name:Gizmo\nManaCost:2\nTypes:Artifact\nOracle:x\n")
		bear := onBoard(t, e, 1, historyBearSrc)
		e.G.Obj(art).DamageDealtThisTurn = []state.DamageDealtRecord{{Recipient: state.PlayerRef(0), Amount: 3}}
		e.G.Obj(bear).DamageDealtThisTurn = []state.DamageDealtRecord{{Recipient: state.PlayerRef(0), Amount: 2}}
		src := onBoard(t, e, 0, historyBearSrc)
		check(t, historyEval(t, e, src, nil, "Count$DamageThisTurn Artifact You/Times.2"), 6, "DamageThisTurn")
		eff := onBoard(t, e, 0, "Name:Blazing Effigy\nManaCost:3 R\nTypes:Creature Elemental\nPT:3/3\nOracle:x\n")
		other := onBoard(t, e, 1, "Name:Blazing Effigy\nManaCost:3 R\nTypes:Creature Elemental\nPT:3/3\nOracle:x\n")
		e.G.Obj(other).DamageDealtThisTurn = []state.DamageDealtRecord{
			{Recipient: eff, RecipientZone: state.ZBattlefield, RecipientControl: 0, Amount: 2},
			{Recipient: bear, RecipientZone: state.ZBattlefield, RecipientControl: 1, Amount: 4},
		}
		check(t, historyEval(t, e, eff, nil, "Count$TotalDamageThisTurn Card.Other+namedBlazing_Effigy/Plus.3"), 5, "TotalDamageThisTurn")
	})
	t.Run("player groups", func(t *testing.T) {
		e := layerEngine(t)
		src := onBoard(t, e, 0, historyBearSrc)
		e.G.Players[1].LifeGainedThisTurn = 3
		check(t, historyEval(t, e, src, nil, "PlayerCountOpponents$LifeGainedThisTurn"), 3, "opponents' life gained")
		inZoneCard(t, e, 1, state.ZHand, historyBearSrc)
		check(t, historyEval(t, e, src, nil, "PlayerCountOpponents$CardsInHand"), int32(len(e.G.Zone(state.ZHand, 1))), "opponents' cards in hand")
		e.G.Players[1].Life = 13
		ctx := &effects.Ctx{Source: src, Controller: 0}
		ctx.TriggerSourceController = state.Target{Player: 1, IsPlayer: true}
		if n, ok := effects.EvalCountOK(e, ctx, "PlayerCountDefinedTriggeredSourceController$LifeTotal/Twice"); !ok || n != 26 {
			t.Fatalf("source controller life x2 = %d (ok %v), want 26", n, ok)
		}
		ctx.TriggerCard = inZoneCard(t, e, 1, state.ZGraveyard, historyBearSrc)
		if n, ok := effects.EvalCountOK(e, ctx, "PlayerCountDefinedTriggeredCardOwner$LifeTotal/HalfUp"); !ok || n != 7 {
			t.Fatalf("card owner life half up = %d (ok %v), want 7", n, ok)
		}
		ctx.AttackedTarget = state.Target{Player: 1, IsPlayer: true}
		const init = "PlayerCountDefinedTriggeredAttackedTargetAndYou$HasPropertyhasInitiative"
		if n, ok := effects.EvalCountOK(e, ctx, init); !ok || n != 0 {
			t.Fatalf("no initiative holder = %d (ok %v), want 0", n, ok)
		}
		e.G.Initiative, e.G.HasInitiative = 1, true
		if n, ok := effects.EvalCountOK(e, ctx, init); !ok || n != 1 {
			t.Fatalf("attacked player holds initiative = %d (ok %v), want 1", n, ok)
		}
	})
}
