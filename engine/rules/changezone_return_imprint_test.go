package rules

// ChangeZone's Imprint$ True records the moved card in the source's persistent
// imprint association whatever the destination is. The object-target loop used
// to record it only for an exile destination (or ImprintLast), so a Return move
// lost the association: Phelia, Exuberant Shepherd's return never saw her
// imprinted card and put no counter, and Fishing Gear always minted its Fish.
//
// Both tests drive the REAL corpus SVar bodies: Phelia's TrigBounce (a Return
// with Imprint$ True) and Fishing Gear's DBChange (the optional battlefield
// move with Imprint$ True) followed by DBToken, whose ConditionDefined$
// Imprinted gate is what decides the Fish.

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestChangeZoneReturnImprintsMovedCard plays Phelia's real exile-then-return
// pair on a Grizzly Bears: the exile records the card in the delayed trigger,
// the return is a ChangeZone with Imprint$ True to the BATTLEFIELD, and the
// chained PutCounter's ConditionDefined$ Imprinted gate must see it. Before the
// fix the return recorded no imprint, so the counter was never put.
func TestChangeZoneReturnImprintsMovedCard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	phelia := lookup(t, reg, "Phelia, Exuberant Shepherd")
	bears := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{phelia, bears}, nil)
	src := moveByName(t, e, 0, "Phelia, Exuberant Shepherd", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	if e.G.Obj(src).Zone != state.ZBattlefield || e.G.Obj(bear).Zone != state.ZBattlefield {
		t.Fatalf("precondition: Phelia zone=%s Bears zone=%s, want both battlefield",
			e.G.Obj(src).Zone, e.G.Obj(bear).Zone)
	}
	if got := e.G.Obj(src).Counter("P1P1"); got != 0 {
		t.Fatalf("precondition: Phelia starts with %d +1/+1 counters, want 0", got)
	}

	// Phelia's real attack trigger body: exile the target and remember it.
	exile := resolveSourceFaceSA(t, e, src, "TrigExile")
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0,
		Targets: []state.Target{{Obj: bear}}, TargetsOffered: true}, exile)
	if z := e.G.Obj(bear).Zone; z != state.ZExile {
		t.Fatalf("precondition: exiled Bears zone=%s, want exile", z)
	}

	// The delayed trigger's real return body, with the registration's captured
	// card bound the way the trigger binds it. Resolve runs the whole chain
	// (TrigBounce -> DBPutCounter -> DBCleanup), so the final counter is what
	// the imprint gate decided.
	bounce := resolveSourceFaceSA(t, e, src, "TrigBounce")
	mark := len(e.L.Events)
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0,
		TriggerContext: effects.TriggerContext{DelayedRemembered: []state.Target{{Obj: bear}}}}, bounce)
	if z := e.G.Obj(bear).Zone; z != state.ZBattlefield {
		t.Fatalf("returned Bears zone=%s, want battlefield", z)
	}
	if got := e.G.Obj(src).Counter("P1P1"); got != 1 {
		t.Fatalf("Phelia counters after the return = %d, want 1 (+1/+1): the Return move's Imprint$ was not recorded", got)
	}
	if countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.Imprint && ev.Obj == src && slices.Contains(ev.IDs, bear)
	}) == 0 {
		t.Fatal("the Return move emitted no Imprint event for the moved card")
	}
	// The counter placement is what the chain changed; assert it happened after
	// the mark so a pre-existing counter cannot stand in for it.
	if countEventsSince(e, mark, func(ev events.Event) bool {
		return ev.Kind == events.CounterChange && ev.Obj == src && ev.Counter == "P1P1" && ev.Amount == 1
	}) != 1 {
		t.Fatal("the return chain did not emit exactly one +1/+1 CounterChange on Phelia")
	}
	replayCheck(t, e, cfg)
}

// TestFishingGearImprintsMovedCard drives Fishing Gear's real DBChange/DBToken
// chain: the optional battlefield move carries Imprint$ True, so the moved
// permanent card is imprinted and DBToken's ConditionDefined$ Imprinted gate
// (ConditionCompare$ EQ0) withholds the Fish. Then the same DBToken body is
// resolved with the (chained Cleanup) empty imprint list and mints the Fish,
// proving the token depends on the association rather than on the card's
// absence.
func TestFishingGearImprintsMovedCard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	gear := lookup(t, reg, "Fishing Gear")
	bears := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{gear, bears}, nil)
	src := moveByName(t, e, 0, "Fishing Gear", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	if e.G.Obj(src).Zone != state.ZBattlefield || e.G.Obj(bear).Zone != state.ZBattlefield {
		t.Fatalf("precondition: Gear zone=%s Bears zone=%s, want both battlefield",
			e.G.Obj(src).Zone, e.G.Obj(bear).Zone)
	}
	// The Dig has already left the permanent card in exile; DBChange is the
	// optional "put it onto the battlefield under your control" move.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZExile, Player: 0})
	if z := e.G.Obj(bear).Zone; z != state.ZExile {
		t.Fatalf("precondition: Bears zone=%s, want exile", z)
	}
	if got := fishingFish(e); got != 0 {
		t.Fatalf("precondition: %d Fish tokens on the battlefield, want 0", got)
	}

	change := resolveSourceFaceSA(t, e, src, "DBChange")
	mark := len(e.L.Events)
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0,
		Remembered: []state.Target{{Obj: bear}}}, change)
	if z := e.G.Obj(bear).Zone; z != state.ZBattlefield {
		t.Fatalf("returned Bears zone=%s, want battlefield", z)
	}
	// The chain ends in DBCleanup (ClearImprinted$), so read the association
	// from the log: the event is what DBToken's condition saw before the clear.
	if countEventsSince(e, mark, func(ev events.Event) bool {
		return ev.Kind == events.Imprint && ev.Obj == src && slices.Contains(ev.IDs, bear)
	}) == 0 {
		t.Fatal("the battlefield move emitted no Imprint event for the moved card")
	}
	if got := fishingFish(e); got != 0 {
		t.Fatalf("%d Fish tokens after the imprinted move, want 0: DBToken did not see the imprint", got)
	}

	// The chain's DBCleanup leaves the association empty. The SAME DBToken body
	// now mints the Fish: the earlier silence was the imprint, not a broken
	// token script.
	if got := e.G.Obj(src).Imprinted; len(got) != 0 {
		t.Fatalf("precondition: imprint after the chain's cleanup = %v, want empty", got)
	}
	token := resolveSourceFaceSA(t, e, src, "DBToken")
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0}, token)
	if got := fishingFish(e); got != 1 {
		t.Fatalf("%d Fish tokens with an empty imprint, want 1", got)
	}
	replayCheck(t, e, cfg)
}

// countEventsSince counts log events after the mark index that pass keep. The
// existing countEventsFrom filters by minted object id, not by log position.
func countEventsSince(e *Engine, mark int, keep func(events.Event) bool) int {
	n := 0
	for _, ev := range e.L.Events[mark:] {
		if keep(ev) {
			n++
		}
	}
	return n
}

// fishingFish counts the Fish tokens on the battlefield. Fishing Gear's only
// token is the 1/1 blue Fish, so a name contains-check is enough and avoids
// depending on the token script's exact printed name.
func fishingFish(e *Engine) int {
	n := 0
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if !o.IsToken || o.Zone != state.ZBattlefield || o.Face() == nil {
			continue
		}
		if strings.Contains(strings.ToLower(o.Face().Name), "fish") {
			n++
		}
	}
	return n
}
