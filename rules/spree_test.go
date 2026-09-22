package rules

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// tapEveryManaSource taps each untapped mana source seat p controls, so
// PotentialMana (the colour-aware affordability bound the Spree mode gate
// prices against) sees only the mana the test explicitly floats. Without it a
// fresh deck's untapped lands make every mode "affordable" and the cost tests
// would measure nothing.
func tapEveryManaSource(t *testing.T, e *Engine, p state.PlayerID) {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o == nil || o.Tapped || o.Face() == nil {
			continue
		}
		if len(o.Face().ManaAbilities()) == 0 {
			continue
		}
		e.emit(events.Event{Kind: events.Tap, Obj: id})
	}
}

// spreeModeCosts proves the compiled face really carries what the test claims
// to exercise: a K:Spree keyword and a per-mode ModeCost$ the engine can parse.
// A test whose whole point is reading ModeCost$ must fail loudly if the
// parameter stops reaching the SVar, not pass because both sides quietly read
// zero.
func spreeModeCosts(t *testing.T, e *Engine, id state.ObjID, want map[string]Cost) {
	t.Helper()
	f := e.G.Obj(id).Face()
	found := false
	for _, k := range f.Keywords {
		if strings.EqualFold(cards.KeywordHead(k), "Spree") {
			found = true
		}
	}
	if !found {
		t.Fatalf("fixture %s carries no K:Spree keyword", f.Name)
	}
	for name, wantCost := range want {
		got, ok := modeCost(f, name)
		if !ok {
			t.Fatalf("fixture %s mode %s carries no parseable ModeCost$", f.Name, name)
		}
		if !reflect.DeepEqual(got, wantCost) {
			t.Fatalf("fixture %s mode %s ModeCost$ = %+v, want %+v", f.Name, name, got, wantCost)
		}
	}
}

// targetOptionFor returns the target decision's option selecting obj, failing
// if the target ask is not pending for it.
func targetOptionFor(t *testing.T, e *Engine, obj state.ObjID) int {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision, got %+v", d)
	}
	for _, opt := range d.Options {
		if opt.Obj == obj {
			return opt.Index
		}
	}
	t.Fatalf("target option for %d absent from %+v", obj, d.Options)
	return 0
}

// TestRequisitionRaidSpreeChargesEachChosenMode pins the whole Spree family end
// to end on the brief's own card: the total price is the printed {W} PLUS the
// ModeCost$ of every chosen mode. One mode costs {W}{1} (printed {W} + its
// {1}), two modes cost {W}{2}. Before this the modes resolved free for the
// printed {W}.
func TestRequisitionRaidSpreeChargesEachChosenMode(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name      string
		modeTexts []string
		floated   int
		wantPool  int32
		wantModes int
	}{
		{name: "one_mode", modeTexts: []string{"Destroy target artifact."}, floated: 3, wantPool: 1, wantModes: 1},
		{name: "two_modes", modeTexts: []string{"Destroy target artifact.", "Destroy target enchantment."}, floated: 4, wantPool: 1, wantModes: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := crAbortEngine(t, reg, "uw-control", "Requisition Raid", "Sol Ring", "Ghostly Prison")
			id := crAbortMove(t, e, 0, "Requisition Raid", state.ZHand)
			artifact := crAbortMove(t, e, 0, "Sol Ring", state.ZBattlefield)
			enchantment := crAbortMove(t, e, 0, "Ghostly Prison", state.ZBattlefield)
			// Preconditions: the fixture's modes really carry ModeCost$ 1, and
			// both target-bearing modes have a legal target (so a mode's
			// presence or absence below is about cost, never target legality).
			spreeModeCosts(t, e, id, map[string]Cost{
				"DBArtifact":    {Generic: 1},
				"DBEnchantment": {Generic: 1},
			})
			if e.G.Obj(artifact).Zone != state.ZBattlefield || e.G.Obj(enchantment).Zone != state.ZBattlefield {
				t.Fatalf("fixture targets not on the battlefield: artifact=%s enchantment=%s",
					e.G.Obj(artifact).Zone, e.G.Obj(enchantment).Zone)
			}
			tapEveryManaSource(t, e, 0)
			for range tc.floated {
				e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
			}
			e.askPriority(0)
			crAbortAnswer(t, e, "Requisition Raid", crAbortOption(t, e, "Requisition Raid", "cast", id))

			d := e.Pending()
			choices := make([]int, 0, len(tc.modeTexts))
			for _, text := range tc.modeTexts {
				choices = append(choices, modeOptionContaining(t, d, text))
			}
			crAbortAnswer(t, e, "Requisition Raid", choices...)
			// The chosen target-bearing mode's CR 601.2c declaration is answered
			// once (the modal first-target-bearing-mode shape).
			crAbortAnswer(t, e, "Requisition Raid", targetOptionFor(t, e, artifact))

			if got := e.G.Obj(id).Zone; got != state.ZStack {
				t.Fatalf("Requisition Raid zone after the cast = %s, want stack", got)
			}
			if got := e.G.Obj(id).ChosenModes; len(got) != tc.wantModes {
				t.Fatalf("Requisition Raid chosen modes = %v, want %d", got, tc.wantModes)
			}
			// printed {W} + one {1} per chosen mode, paid from the floated W;
			// the pool left is what pins the charged total.
			if got := e.G.Players[0].Pool.Total(); got != tc.wantPool {
				t.Fatalf("Requisition Raid %s left pool %d, want %d (charged %d of %d floated)",
					tc.name, got, tc.wantPool, int32(tc.floated)-tc.wantPool, tc.floated)
			}
		})
	}
}

// TestMetamorphicBlastSpreeUnaffordableModeNotOffered pins the cost-aware mode
// offer: with only {U}{U} floated, Metamorphic Blast's {3} "target player
// draws two cards" mode cannot be paid on top of the printed {U}, so it is not
// offered, while its {1} Rabbit mode (same target legality: a creature and a
// player are both present) is. A mode declined for cost is not selected.
func TestMetamorphicBlastSpreeUnaffordableModeNotOffered(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := crAbortEngine(t, reg, "uw-control", "Metamorphic Blast", "Grizzly Bears")
	id := crAbortMove(t, e, 0, "Metamorphic Blast", state.ZHand)
	creature := crAbortMove(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	spreeModeCosts(t, e, id, map[string]Cost{
		"DBAnimate": {Generic: 1},
		"DBDraw":    {Generic: 3},
	})
	if e.G.Obj(creature).Zone != state.ZBattlefield {
		t.Fatalf("fixture creature not on the battlefield")
	}
	// Prove the {3} mode is TARGET-legal here (a player exists), so its absence
	// from the options is the cost gate and not the target filter.
	drawSA := cards.ResolveSVar(e.G.Obj(id).Face().SVars, "DBDraw")
	if drawSA == nil || len(e.legalTargetCandidates(0, id, id, drawSA)) < 1 {
		t.Fatalf("fixture: DB$ Draw has no legal player target, cannot attribute its absence to cost")
	}
	tapEveryManaSource(t, e, 0)
	for range 2 {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 1})
	}
	e.askPriority(0)
	crAbortAnswer(t, e, "Metamorphic Blast", crAbortOption(t, e, "Metamorphic Blast", "cast", id))

	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("after cast, next decision = %+v, want modes", d)
	}
	hasRabbit, hasDraw := false, false
	for _, opt := range d.Options {
		hasRabbit = hasRabbit || strings.Contains(opt.Label, "Rabbit")
		hasDraw = hasDraw || strings.Contains(opt.Label, "draws two cards")
	}
	if !hasRabbit {
		t.Fatalf("the {1} Rabbit mode was not offered: %+v", d.Options)
	}
	if hasDraw {
		t.Fatalf("the unaffordable {3} draw mode was offered with only {U}{U} floated: %+v", d.Options)
	}
	crAbortAnswer(t, e, "Metamorphic Blast", modeOptionContaining(t, d, "Rabbit"))
	crAbortAnswer(t, e, "Metamorphic Blast", targetOptionFor(t, e, creature))
	if got := e.G.Obj(id).Zone; got != state.ZStack {
		t.Fatalf("Metamorphic Blast zone = %s, want stack", got)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("Metamorphic Blast left pool %d, want 0 (printed {U} + the {1} mode)", got)
	}
}

// TestRequisitionRaidSpreeZeroModeCastNotOffered pins the family's lower
// bound: a Spree cast must choose at least MinCharmNum$ modes, so with only
// the printed {W} affordable no mode is payable and the cast is not offered --
// it reverses to hand (CR 733.1) instead of silently resolving a free mode.
func TestRequisitionRaidSpreeZeroModeCastNotOffered(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := crAbortEngine(t, reg, "uw-control", "Requisition Raid", "Sol Ring", "Ghostly Prison")
	id := crAbortMove(t, e, 0, "Requisition Raid", state.ZHand)
	crAbortMove(t, e, 0, "Sol Ring", state.ZBattlefield)
	crAbortMove(t, e, 0, "Ghostly Prison", state.ZBattlefield)
	spreeModeCosts(t, e, id, map[string]Cost{"DBArtifact": {Generic: 1}})
	tapEveryManaSource(t, e, 0)
	// Only {W}: the printed cost is payable, not one mode's {1} on top.
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	e.askPriority(0)
	crAbortAnswer(t, e, "Requisition Raid", crAbortOption(t, e, "Requisition Raid", "cast", id))

	if got := e.G.Obj(id).Zone; got != state.ZHand {
		t.Fatalf("zero-mode Requisition Raid zone = %s, want hand (the cast must reverse)", got)
	}
	if e.cast != nil {
		t.Fatalf("zero-mode Requisition Raid left a pending cast")
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.ModeChosen && ev.Obj == id {
			t.Fatalf("zero-mode cast recorded a mode announcement at seq %d", ev.Seq)
		}
	}
}

// TestRequisitionRaidSpreeUnaffordableCombinationReverses pins the other side
// of the per-mode gate: each mode is individually affordable ({W}{1}), but the
// SUM ({W}{2}) exceeds {W}{W}, so the whole proposal reverses (CR 733.1) and
// the cast never pays for modes it cannot afford.
func TestRequisitionRaidSpreeUnaffordableCombinationReverses(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := crAbortEngine(t, reg, "uw-control", "Requisition Raid", "Sol Ring", "Ghostly Prison")
	id := crAbortMove(t, e, 0, "Requisition Raid", state.ZHand)
	crAbortMove(t, e, 0, "Sol Ring", state.ZBattlefield)
	crAbortMove(t, e, 0, "Ghostly Prison", state.ZBattlefield)
	spreeModeCosts(t, e, id, map[string]Cost{"DBArtifact": {Generic: 1}, "DBEnchantment": {Generic: 1}})
	tapEveryManaSource(t, e, 0)
	for range 2 {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	}
	e.askPriority(0)
	crAbortAnswer(t, e, "Requisition Raid", crAbortOption(t, e, "Requisition Raid", "cast", id))

	d := e.Pending()
	crAbortAnswer(t, e, "Requisition Raid",
		modeOptionContaining(t, d, "Destroy target artifact."),
		modeOptionContaining(t, d, "Destroy target enchantment."))
	// Each mode is affordable alone, so the proposal advances past the mode
	// answer; the unaffordable {W}{2} total reverses it (CR 733.1) before any
	// target is committed or mana is spent.
	if got := e.G.Obj(id).Zone; got != state.ZHand {
		t.Fatalf("over-budget Requisition Raid zone = %s, want hand", got)
	}
	if got := e.G.Players[0].Pool.Total(); got != 2 {
		t.Fatalf("over-budget Requisition Raid pool = %d, want 2 (nothing charged)", got)
	}
	if e.cast != nil {
		t.Fatalf("over-budget Requisition Raid left a pending cast")
	}
}
