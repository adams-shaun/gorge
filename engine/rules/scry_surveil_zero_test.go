package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Scry 0 / surveil 0 is no scry / surveil event (CR 701.22b, 701.25c): no
// "whenever you scry/surveil" trigger fires. An N > 0 instruction over an
// empty library is still a scry (TestScryZeroLookRecordsZeroBottomCompletion).

const scryZeroSrc = "Name:ScryNone\nManaCost:U\nTypes:Sorcery\n" +
	"A:SP$ Scry | Defined$ You | ScryNum$ 0\nOracle:x\n"

const scryOneSrc = "Name:ScryOnce\nManaCost:U\nTypes:Sorcery\n" +
	"A:SP$ Scry | Defined$ You | ScryNum$ 1\nOracle:x\n"

const surveilZeroSrc = "Name:SurveilNone\nManaCost:U\nTypes:Sorcery\n" +
	"A:SP$ Surveil | Defined$ You | Amount$ 0\nOracle:x\n"

// surveilMarkers returns every events.Surveil marker in the log.
func surveilMarkers(e *Engine) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.Surveil {
			out = append(out, ev)
		}
	}
	return out
}

func zeroCountKind(e *Engine, k events.Kind) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == k {
			n++
		}
	}
	return n
}

const matoyaFile = "m/matoya_archon_elder.txt"
const anchorFile = "t/the_temporal_anchor.txt"

// matoyaOnBoard enters the deck's Matoya (Mode$ Scry and Mode$ Surveil
// triggers) and asserts the triggers are on the battlefield.
func matoyaOnBoard(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	id := enterFromLibrary(t, e, 0, "Matoya, Archon Elder")
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("setup: Matoya did not enter: %+v", o)
	}
	o := e.G.Obj(id)
	if len(o.Face().Triggers) < 2 || o.Face().Triggers[0].Mode != "Scry" || o.Face().Triggers[1].Mode != "Surveil" {
		t.Fatalf("setup: Matoya's Scry/Surveil triggers are not on the battlefield: %+v", o)
	}
	return id
}

func castDirect(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	d0 := e.Pending()
	for _, o := range d0.Options {
		if o.Kind == "cast" && o.Obj == id {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no cast option for %d: %+v", id, d0.Options)
}

func TestScryZeroIsNoScryEvent(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 301, scryZeroSrc, corpusCardText(t, matoyaFile), corpusCardText(t, anchorFile), corpusCardText(t, "k/kenessos_priest_of_thassa.txt"))
	addMana(t, e, 0, "U")
	matoya := matoyaOnBoard(t, e)
	anchor := enterFromLibrary(t, e, 0, "The Temporal Anchor")
	if o := e.G.Obj(anchor); o == nil || o.Zone != state.ZBattlefield || len(o.Face().Triggers) < 2 || o.Face().Triggers[1].Params["ToBottom"] != "True" {
		t.Fatalf("setup: The Temporal Anchor is not on the battlefield with its scry trigger: %+v", o)
	}
	// Kenessos rewrites a scry to "that many plus one": a scry 0 that still
	// reached the replacement pass would become a scry 1 and pose a KArrange.
	kenessos := enterFromLibrary(t, e, 0, "Kenessos, Priest of Thassa")
	if o := e.G.Obj(kenessos); o == nil || o.Zone != state.ZBattlefield || len(o.Face().Repls) == 0 {
		t.Fatalf("setup: Kenessos's scry replacement is not on the battlefield: %+v", o)
	}
	if len(e.G.Zone(state.ZLibrary, 0)) < 3 {
		t.Fatal("setup: library too small to show an order change")
	}
	before := slices.Clone(e.G.Zone(state.ZLibrary, 0))
	orders := zeroCountKind(e, events.LibraryOrder)
	castDirect(t, e, id)
	// drainScryResolution fails the test on any KArrange/KChoose ask.
	drainScryResolution(t, e, 60)
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: the scry spell never resolved: stack %v", e.G.Stack)
	}
	if o := e.G.Obj(id); o == nil || o.Zone == state.ZHand {
		t.Fatalf("precondition: the spell was never cast: %+v", o)
	}
	if m := scryMarkers(e); len(m) != 0 {
		t.Fatalf("scry 0 recorded %+v, want no scry event", m)
	}
	if n := anchorTriggerPushes(e, matoya); n != 0 {
		t.Fatalf("Matoya pushed %d triggers on scry 0, want 0", n)
	}
	if n := anchorTriggerPushes(e, anchor); n != 0 {
		t.Fatalf("The Temporal Anchor pushed %d triggers on scry 0, want 0", n)
	}
	if n := zeroCountKind(e, events.LibraryOrder); n != orders {
		t.Fatalf("scry 0 emitted %d LibraryOrder event(s), want none", n-orders)
	}
	if after := e.G.Zone(state.ZLibrary, 0); !slices.Equal(before, after) {
		t.Fatalf("library order changed on scry 0:\n%v\n%v", before, after)
	}
	replayCheck(t, e, cfg)
}

func TestSurveilZeroIsNoSurveilEvent(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 302, surveilZeroSrc, corpusCardText(t, matoyaFile))
	addMana(t, e, 0, "U")
	matoya := matoyaOnBoard(t, e)
	castDirect(t, e, id)
	drainScryResolution(t, e, 60)
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: the surveil spell never resolved: stack %v", e.G.Stack)
	}
	if o := e.G.Obj(id); o == nil || o.Zone == state.ZHand {
		t.Fatalf("precondition: the spell was never cast: %+v", o)
	}
	if m := surveilMarkers(e); len(m) != 0 {
		t.Fatalf("surveil 0 recorded %+v, want no surveil event", m)
	}
	if n := anchorTriggerPushes(e, matoya); n != 0 {
		t.Fatalf("Matoya pushed %d triggers on surveil 0, want 0", n)
	}
	replayCheck(t, e, cfg)
}

func TestCerebralDownloadWithNoArtifactsSurveilsZero(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 303, corpusCardText(t, "c/cerebral_download.txt"), corpusCardText(t, matoyaFile))
	addMana(t, e, 0, "UUUUU")
	matoya := matoyaOnBoard(t, e)
	for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(oid); o != nil && o.Face() != nil && slices.Contains(o.Face().Types, "Artifact") {
			t.Fatalf("setup: seat 0 controls an artifact: %v", o.Face().Name)
		}
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	castDirect(t, e, id)
	drainScryResolution(t, e, 60)
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: Cerebral Download never resolved: stack %v", e.G.Stack)
	}
	// The spell left the hand; the draw-three follows the surveil 0.
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand-1+3 {
		t.Fatalf("hand = %d, want %d (cast one, drew three)", got, hand-1+3)
	}
	if m := surveilMarkers(e); len(m) != 0 {
		t.Fatalf("surveil X=0 recorded %+v, want no surveil event", m)
	}
	if n := anchorTriggerPushes(e, matoya); n != 0 {
		t.Fatalf("Matoya pushed %d triggers on surveil X=0, want 0", n)
	}
	replayCheck(t, e, cfg)
}

func TestScryOneStillTriggersOnEmptyLibrary(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 304, scryOneSrc, corpusCardText(t, matoyaFile))
	addMana(t, e, 0, "U")
	matoya := matoyaOnBoard(t, e)
	emptyLibrary(t, e, 0)
	castDirect(t, e, id)
	drainScryResolution(t, e, 60)
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: the scry spell never resolved: stack %v", e.G.Stack)
	}
	if m := scryMarkers(e); len(m) != 1 || m[0].Obj != id {
		t.Fatalf("scry 1 over an empty library recorded %+v, want exactly one", m)
	}
	if n := anchorTriggerPushes(e, matoya); n != 1 {
		t.Fatalf("Matoya pushed %d triggers, want 1", n)
	}
	replayCheck(t, e, cfg)
}
