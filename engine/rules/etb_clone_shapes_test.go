package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cr-g119: the as-enters copy election (CR 614.1c, 614.12a, 707.9b) for the
// bodies that name the template's zone (ChoiceZone$), a new name (NewName$),
// the copy origin (RememberCloneOrigin$) and a reflexive chain (Superior
// Spider-Man), plus Mockingbird's non-cast entry.

// runETBCloneScenario plays raw and fails the test on any mismatch; it returns
// the run so a test can read the engine's final state.
func runETBCloneScenario(t *testing.T, raw string) *oracleRun {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	fails, tr, run := runOracleScenario(reg, sc)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(tr, "\n"))
	}
	if hasNote(run.e, "unimplemented API Clone") {
		t.Fatal("the loud unimplemented-API Clone fallback ran")
	}
	return run
}

// findOn returns the id of the named object in seat's zone.
func findOn(t *testing.T, e *Engine, zone state.Zone, seat state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(zone, seat) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("%s not in seat %d %s", name, seat, zone)
	return 0
}

// TestEchoingDeepsPlayedEntersAsGraveyardLandCopyTapped: "You may have
// Echoing Deeps enter tapped as a copy of any land card in a graveyard, except
// it's a Cave in addition to its other types." The template is in the
// OPPONENT's graveyard ("a graveyard", CR 707.2).
func TestEchoingDeepsPlayedEntersAsGraveyardLandCopyTapped(t *testing.T) {
	t.Parallel()
	raw := func(pick string) string {
		return `{"name":"echoing-deeps","cr":["614.12a","707.9b"],"why":"cr-g119","setup":{` +
			`"p0":{"hand":["Echoing Deeps"]},"p1":{"graveyard":["Forest"]}},` +
			`"steps":[{"op":"play","seat":0,"card":"p0:Echoing Deeps","answers":[` +
			`{"kind":"choose","pick":["` + pick + `"]}]}],"expect":[` +
			`{"card":"p0:Echoing Deeps","zone":"battlefield"}]}`
	}
	t.Run("copy", func(t *testing.T) {
		run := runETBCloneScenario(t, raw("p1:Forest"))
		e := run.e
		id := findOn(t, e, state.ZBattlefield, 0, "Forest")
		der := e.Derived(id)
		for _, ty := range []string{"Land", "Forest", "Cave"} {
			if !slices.Contains(der.Types, ty) {
				t.Fatalf("copy lacks type %q: %v", ty, der.Types)
			}
		}
		if !e.G.Obj(id).Tapped {
			t.Fatal("copy entered untapped; Echoing Deeps enters tapped")
		}
		// The copy has the Forest's mana ability.
		makesG := false
		if f := e.G.Obj(id).Face(); f != nil {
			for _, a := range f.Abilities {
				if cards.IsManaAbilityAPI(a.API) && a.ParamStr(cards.PKProduced) == "G" {
					makesG = true
				}
			}
		}
		if !makesG {
			t.Fatal("copy lacks the Forest's {T}: add {G} mana ability")
		}
		if e.G.Obj(findOn(t, e, state.ZGraveyard, 1, "Forest")) == nil {
			t.Fatal("the copied card left the graveyard")
		}
	})
	t.Run("decline", func(t *testing.T) {
		run := runETBCloneScenario(t, raw("Enter as itself"))
		e := run.e
		id := findOn(t, e, state.ZBattlefield, 0, "Echoing Deeps")
		der := e.Derived(id)
		if slices.Contains(der.Types, "Forest") || !slices.Contains(der.Types, "Cave") {
			t.Fatalf("declined election must leave a plain Land Cave: %v", der.Types)
		}
		if e.G.Obj(id).Tapped {
			t.Fatal("declined election: Echoing Deeps should enter untapped")
		}
	})
}

// TestChameleonCopyKeepsItsName: "enter as a copy of a creature you control,
// except his name is Chameleon, Master of Disguise" (NewName$).
func TestChameleonCopyKeepsItsName(t *testing.T) {
	t.Parallel()
	raw := `{"name":"chameleon","cr":["707.9b"],"why":"cr-g119","setup":{` +
		`"p0":{"hand":["Chameleon, Master of Disguise"],"battlefield":["Grizzly Bears"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Chameleon, Master of Disguise","mana":"CCCU"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p0:Grizzly Bears"]}]}],"expect":[]}`
	run := runETBCloneScenario(t, raw)
	e := run.e
	id := findOn(t, e, state.ZBattlefield, 0, "Chameleon, Master of Disguise")
	der := e.Derived(id)
	if der.Power != 2 || der.Toughness != 2 || !slices.Contains(der.Types, "Bear") {
		t.Fatalf("Chameleon should be a 2/2 Bear copy: %+v", der)
	}
	if !hasEvent(e, events.ClonePermanent, id) {
		t.Fatal("precondition: no copy was made")
	}
}

// TestSuperiorSpiderManCopiesGraveyardCreatureAndExilesIt: Mind Swap copies a
// creature card in a graveyard (4/4 Spider Human Hero in addition to its
// types, named Superior Spider-Man) and "When you do, exile that card" (CR
// 603.12, a reflexive trigger the copy election creates).
func TestSuperiorSpiderManCopiesGraveyardCreatureAndExilesIt(t *testing.T) {
	t.Parallel()
	raw := `{"name":"superior-spider-man","cr":["707.9b","603.12"],"why":"cr-g119","setup":{` +
		`"p0":{"hand":["Superior Spider-Man"]},"p1":{"graveyard":["Craw Wurm"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Superior Spider-Man","mana":"CCUB"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Craw Wurm"]}]}],"expect":[]}`
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	fails, tr, run := runOracleScenario(reg, sc)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(tr, "\n"))
	}
	e := run.e
	id := findOn(t, e, state.ZBattlefield, 0, "Superior Spider-Man")
	der := e.Derived(id)
	if der.Power != 4 || der.Toughness != 4 {
		t.Fatalf("want 4/4: %+v", der)
	}
	for _, ty := range []string{"Wurm", "Spider", "Human", "Hero"} {
		if !slices.Contains(der.Types, ty) {
			t.Fatalf("copy lacks subtype %q: %v", ty, der.Types)
		}
	}
	// CR 603.12: the exile is a reflexive triggered ability put on the stack
	// (a DelayedPush of the Execute$ SVar), not an inline part of the copy.
	pushed := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedPush && ev.Counter == "TrigExile" {
			pushed = true
		}
	}
	if !pushed {
		t.Fatal("the exile ran inline: no reflexive trigger was put on the stack")
	}
	for i := 0; i < 8 && len(e.G.Stack) > 0; i++ {
		kr6ResolveTop(e)
		kr6Settle(e)
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("unexpected pending decision: %+v", d)
	}
	if got := e.G.Zone(state.ZExile, 1); len(got) != 1 || e.G.Obj(got[0]).Face().Name != "Craw Wurm" {
		t.Fatalf("Craw Wurm should be exiled, graveyard=%v exile=%v", e.G.Zone(state.ZGraveyard, 1), got)
	}
	if o := e.G.Obj(findOn(t, e, state.ZBattlefield, 0, "Superior Spider-Man")); o == nil {
		t.Fatal("Spider-Man left the battlefield")
	}
}

// TestMockingbirdNonCastEntryCopiesOnlyManaValueZero: with no cast, the mana
// spent to cast Mockingbird is 0 (Forge AbilityUtils Count$CastTotalManaSpent
// is 0 for an uncast card; XMage Mockingbird.java ManaSpentToCastCount reads
// ManaPaidSourceWatcher.getTotalPaid, 0 when nothing was paid), so a
// reanimated Mockingbird may copy only a mana-value-0 creature.
func TestMockingbirdNonCastEntryCopiesOnlyManaValueZero(t *testing.T) {
	t.Parallel()
	raw := func(pick string) string {
		return `{"name":"mockingbird-reanimated","cr":["614.12a","707.9b"],"why":"cr-g119","setup":{` +
			`"p0":{"hand":["Reanimate"],"graveyard":["Mockingbird"]},` +
			`"p1":{"battlefield":["Grizzly Bears","Memnite"]}},` +
			`"steps":[{"op":"cast","seat":0,"card":"p0:Reanimate","mana":"B","targets":["p0:Mockingbird"]},` +
			`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["` + pick + `"]}]}],"expect":[]}`
	}
	// The mana-value-0 creature is offered and copied (1/1 Bird with flying).
	run := runETBCloneScenario(t, raw("p1:Memnite"))
	e := run.e
	id := findOn(t, e, state.ZBattlefield, 0, "Memnite")
	der := e.Derived(id)
	if der.Power != 1 || der.Toughness != 1 || !slices.Contains(der.Types, "Bird") || !slices.Contains(der.Keywords, "Flying") {
		t.Fatalf("reanimated Mockingbird should be a Memnite copy that is a Bird with flying: %+v", der)
	}
	if !hasEvent(e, events.ClonePermanent, id) {
		t.Fatal("precondition: no copy was made")
	}
	// Grizzly Bears (mana value 2) is not a legal template with 0 mana spent.
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(raw("p1:Grizzly Bears")))
	if err != nil {
		t.Fatal(err)
	}
	fails, _, _ := runOracleScenario(reg, sc)
	if len(fails) == 0 {
		t.Fatal("a mana-value-2 creature was offered to a non-cast Mockingbird")
	}
}

// TestReanimatedPhantasmalImageGetsCopyElection: Phantasmal Image put onto the
// battlefield without being cast, by Zombify's ChangeZone, still gets its
// "you may have this enter as a copy" election. The ask is posed at the
// MoveZone boundary for every entry, so the copy is made, keeps the printed
// AddTypes$ Illusion exception, and the loud fallback never runs.
func TestReanimatedPhantasmalImageGetsCopyElection(t *testing.T) {
	t.Parallel()
	raw := `{"name":"phantasmal-image-reanimated","cr":["614.12a","707.9b"],"why":"cr-g119","setup":{` +
		`"p0":{"hand":["Zombify"],"graveyard":["Phantasmal Image"]},` +
		`"p1":{"battlefield":["Grizzly Bears"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Zombify","mana":"BBBC","targets":["p0:Phantasmal Image"]},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Grizzly Bears"]}]}]}`
	run := runETBCloneScenario(t, raw)
	e := run.e
	id := findOn(t, e, state.ZBattlefield, 0, "Grizzly Bears")
	der := e.Derived(id)
	if !slices.Contains(der.Types, "Illusion") {
		t.Fatalf("reanimated Phantasmal Image copy lacks the printed AddTypes$ Illusion exception: %v", der.Types)
	}
	if !hasEvent(e, events.ClonePermanent, id) {
		t.Fatal("precondition: no copy was made")
	}
	// The copied Grizzly Bears is the opponent's; the reanimated card is a
	// separate object that copied it (CR 707.2), not the same permanent.
	orig := findOn(t, e, state.ZBattlefield, 1, "Grizzly Bears")
	if orig == id {
		t.Fatal("the copy is the opponent's own Grizzly Bears, not a new object")
	}
}

// TestBlinkedCloneGetsCopyElection: Clone exiled and returned by Ephemerate is
// a non-cast entry too. The +1/+1 counter keeps the 0/0 Clone alive until the
// blink, and the re-entry election makes the returning object a Grizzly Bears
// copy; the loud fallback never runs.
func TestBlinkedCloneGetsCopyElection(t *testing.T) {
	t.Parallel()
	raw := `{"name":"clone-blinked","cr":["614.12a","707.9b"],"why":"cr-g119","setup":{` +
		`"p0":{"hand":["Ephemerate"],"battlefield":["Clone"],"counters":{"Clone":{"P1P1":1}}},` +
		`"p1":{"battlefield":["Grizzly Bears"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Ephemerate","mana":"W","targets":["p0:Clone"]},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Grizzly Bears"]}]}]}`
	run := runETBCloneScenario(t, raw)
	e := run.e
	id := findOn(t, e, state.ZBattlefield, 0, "Grizzly Bears")
	if !hasEvent(e, events.ClonePermanent, id) {
		t.Fatal("precondition: no copy was made")
	}
	// The copied Grizzly Bears is the opponent's; the returning card is a
	// separate object that copied it.
	orig := findOn(t, e, state.ZBattlefield, 1, "Grizzly Bears")
	if orig == id {
		t.Fatal("the copy is the opponent's own Grizzly Bears, not a new object")
	}
}
