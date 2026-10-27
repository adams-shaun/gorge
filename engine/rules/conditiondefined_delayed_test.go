package rules

// ConditionDefined$ groups that share the Defined$ resolver (cr-g245): Forge
// resolves every ConditionDefined$ value with the same resolver Defined$
// uses, so the gate reads exactly the set the guarded effect acts on
// (CR 603.7c / 608.2c). Before the fix these groups were classified
// unmodelled and every gated sub-ability failed CLOSED.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// nettlingImpGame puts Silvercoat Lion on seat 0's battlefield and Nettling
// Imp on seat 1's, drives to seat 0's turn-3 first main phase (the Lion can
// attack), and resolves the Imp's ability on the Lion the way the stack would
// for an activation during seat 0's turn. It returns the Lion's id.
func nettlingImpGame(t *testing.T, seed uint64) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	imp := mustCorpusCard(t, reg, "Nettling Imp")
	lion := mustCorpusCard(t, reg, "Silvercoat Lion")
	e, cfg := tokenReplGameSeats(t, seed, []*cards.Card{lion}, []*cards.Card{imp})
	lionID := moveSeededCard(t, e, 0, lion, state.ZBattlefield)
	impID := moveSeededCard(t, e, 1, imp, state.ZBattlefield)
	e.Advance()
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	sa := e.G.Obj(impID).Face().Abilities[0]
	if sa.API != "Animate" {
		t.Fatalf("Nettling Imp's ability API = %q, want Animate", sa.API)
	}
	ctx := effects.NewCtx(impID, 1, effects.CtxInit{
		SVars:   e.G.Obj(impID).Face().SVars,
		Targets: []state.Target{{Obj: lionID}},
	})
	effects.Resolve(e, &ctx, sa)
	e.pending = nil
	if len(e.G.Delayed) != 1 {
		t.Fatalf("delayed registrations = %d, want 1", len(e.G.Delayed))
	}
	if o := e.G.Obj(lionID); o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Lion zone %v controller %d, want seat 0's battlefield", o.Zone, o.Controller)
	}
	return e, cfg, lionID
}

// TestNettlingImpDestroysCreatureThatDidNotAttack: the Lion is tapped before
// combat (Pressure Point), so it cannot attack; the end-step delayed trigger
// destroys it.
func TestNettlingImpDestroysCreatureThatDidNotAttack(t *testing.T) {
	t.Parallel()
	e, cfg, lion := nettlingImpGame(t, 451)
	e.emit(events.Event{Kind: events.Tap, Obj: lion})
	if !e.G.Obj(lion).Tapped {
		t.Fatal("precondition: the Lion is tapped")
	}
	driveToStepAll(t, e, 4, 1, state.StepUpkeep)
	if effects.MatchesObjectCtx(e.G, "Card.attackedThisTurn", e.G.Obj(lion), effects.SpecContext{}) {
		t.Fatal("precondition: the Lion did not attack")
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("delayed trigger not consumed: %+v", e.G.Delayed)
	}
	if z := e.G.Obj(lion).Zone; z != state.ZGraveyard {
		t.Fatalf("Silvercoat Lion in %v, want graveyard (it did not attack)", z)
	}
	replayCheck(t, e, cfg)
}

// TestNettlingImpSparesCreatureThatAttacked: the Lion attacks, so at the end
// step the gate is false and it survives.
func TestNettlingImpSparesCreatureThatAttacked(t *testing.T) {
	t.Parallel()
	e, cfg, lion := nettlingImpGame(t, 452)
	attackWith(t, e, lion)
	drainCombatPriority(t, e)
	if !effects.MatchesObjectCtx(e.G, "Card.attackedThisTurn", e.G.Obj(lion), effects.SpecContext{}) {
		t.Fatal("precondition: the Lion attacked")
	}
	driveToStepAll(t, e, 4, 1, state.StepUpkeep)
	if len(e.G.Delayed) != 0 {
		t.Fatalf("delayed trigger not consumed: %+v", e.G.Delayed)
	}
	if z := e.G.Obj(lion).Zone; z != state.ZBattlefield {
		t.Fatalf("Silvercoat Lion in %v, want battlefield (it attacked)", z)
	}
	replayCheck(t, e, cfg)
}

// gateProbeSrc is a seat-0 permanent whose Gate SVar gains 1 life only when
// the ConditionDefined$ group holds a member matching present.
func gateProbeSrc(defined, present string) string {
	return "Name:Gate Probe\nTypes:Artifact\n" +
		"SVar:Gate:DB$ GainLife | Defined$ You | LifeAmount$ 1 | ConditionDefined$ " + defined +
		" | ConditionPresent$ " + present + " | ConditionCompare$ GE1\nOracle:x\n"
}

// runGate resolves the probe's Gate SVar under a context bind configures and
// reports whether seat 0 gained life.
func runGate(t *testing.T, e *Engine, probe state.ObjID, bind func(*effects.Ctx)) bool {
	t.Helper()
	o := e.G.Obj(probe)
	sa := cards.ResolveSVar(o.Face().SVars, "Gate")
	if sa == nil {
		t.Fatal("probe has no Gate SVar")
	}
	ctx := effects.NewCtx(probe, 0, effects.CtxInit{SVars: o.Face().SVars})
	if bind != nil {
		bind(&ctx)
	}
	before := e.G.Players[0].Life
	effects.Resolve(e, &ctx, sa)
	e.pending = nil
	return e.G.Players[0].Life > before
}

const (
	gateGiantSrc = "Name:Gate Giant\nManaCost:2 R\nTypes:Creature Giant\nPT:3/3\nOracle:x\n"
	gateBearSrc  = "Name:Gate Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	gateBoltSrc  = "Name:Gate Bolt\nManaCost:R\nTypes:Instant\nA:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ x.\nOracle:x\n"
	gateSamSrc   = "Name:Gate Samurai\nManaCost:1 W\nTypes:Creature Human Samurai\nPT:2/2\nOracle:x\n"
)

// gateGame seeds the probe plus the listed cards in seat 0's deck and puts the
// probe on the battlefield.
func gateGame(t *testing.T, seed uint64, defined, present string, extras ...string) (*Engine, Config, state.ObjID, []state.ObjID) {
	t.Helper()
	probe := card(t, gateProbeSrc(defined, present))
	seat0 := []*cards.Card{probe}
	for _, src := range extras {
		seat0 = append(seat0, card(t, src))
	}
	e, cfg := tokenReplGame(t, seed, seat0...)
	probeID := moveSeededCard(t, e, 0, probe, state.ZBattlefield)
	ids := make([]state.ObjID, 0, len(extras))
	for _, c := range seat0[1:] {
		ids = append(ids, moveSeededCard(t, e, 0, c, state.ZBattlefield))
	}
	return e, cfg, probeID, ids
}

// TestConditionDefinedDelayTriggerRememberedLKI: Surtland Flinger's
// `ConditionDefined$ DelayTriggerRememberedLKI | ConditionPresent$ Giant`
// reads the delayed registration's own capture.
func TestConditionDefinedDelayTriggerRememberedLKI(t *testing.T) {
	t.Parallel()
	e, cfg, probe, ids := gateGame(t, 453, "DelayTriggerRememberedLKI", "Giant", gateGiantSrc, gateBearSrc)
	giant, bear := ids[0], ids[1]
	if !runGate(t, e, probe, func(c *effects.Ctx) { c.DelayedRemembered = []state.Target{{Obj: giant}} }) {
		t.Fatal("remembered Giant: gate not met")
	}
	if runGate(t, e, probe, func(c *effects.Ctx) { c.DelayedRemembered = []state.Target{{Obj: bear}} }) {
		t.Fatal("remembered Bear: gate met, want not")
	}
	replayCheck(t, e, cfg)
}

// TestConditionDefinedTriggeredNewCardLKICopy: Oathkeeper, Takeno's Daisho's
// `ConditionDefined$ TriggeredNewCardLKICopy | ConditionPresent$ Card.Samurai`.
func TestConditionDefinedTriggeredNewCardLKICopy(t *testing.T) {
	t.Parallel()
	e, cfg, probe, ids := gateGame(t, 454, "TriggeredNewCardLKICopy", "Card.Samurai", gateSamSrc, gateBearSrc)
	sam, bear := ids[0], ids[1]
	if !runGate(t, e, probe, func(c *effects.Ctx) { c.Remembered = []state.Target{{Obj: sam}} }) {
		t.Fatal("triggered Samurai: gate not met")
	}
	if runGate(t, e, probe, func(c *effects.Ctx) { c.Remembered = []state.Target{{Obj: bear}} }) {
		t.Fatal("triggered Bear: gate met, want not")
	}
	replayCheck(t, e, cfg)
}

// TestConditionDefinedReplacedSource: Comeuppance's `ConditionDefined$
// ReplacedSource | ConditionPresent$ Card.Creature`.
func TestConditionDefinedReplacedSource(t *testing.T) {
	t.Parallel()
	e, cfg, probe, ids := gateGame(t, 455, "ReplacedSource", "Card.Creature", gateBearSrc)
	bear := ids[0]
	if !runGate(t, e, probe, func(c *effects.Ctx) { c.Repl.Source = bear }) {
		t.Fatal("creature damage source: gate not met")
	}
	if runGate(t, e, probe, func(c *effects.Ctx) { c.Repl.Source = probe }) {
		t.Fatal("artifact damage source: gate met, want not")
	}
	replayCheck(t, e, cfg)
}

// TestConditionDefinedTopOfLibrary: Vesuvan Drifter's `ConditionDefined$
// TopOfLibrary | ConditionPresent$ Creature`, against a library of Mountains:
// the Creature gate is unmet and the same group under a Land gate is met.
func TestConditionDefinedTopOfLibrary(t *testing.T) {
	t.Parallel()
	creatureGate, landGate := card(t, gateProbeSrc("TopOfLibrary", "Creature")), card(t,
		"Name:Land Gate Probe\nTypes:Artifact\n"+
			"SVar:Gate:DB$ GainLife | Defined$ You | LifeAmount$ 1 | ConditionDefined$ TopOfLibrary | ConditionPresent$ Land | ConditionCompare$ GE1\nOracle:x\n")
	e, cfg := tokenReplGame(t, 456, creatureGate, landGate)
	creatureProbe := moveSeededCard(t, e, 0, creatureGate, state.ZBattlefield)
	landProbe := moveSeededCard(t, e, 0, landGate, state.ZBattlefield)
	if lib := e.G.Zone(state.ZLibrary, 0); len(lib) == 0 || e.G.Obj(lib[0]).Face().Name != "Mountain" {
		t.Fatal("precondition: seat 0's library top is a Mountain")
	}
	if runGate(t, e, creatureProbe, nil) {
		t.Fatal("Mountain on top: Creature gate met, want not")
	}
	if !runGate(t, e, landProbe, nil) {
		t.Fatal("Mountain on top: Land gate not met")
	}
	replayCheck(t, e, cfg)
}

// TestConditionDefinedRememberedCard: God-Eternal Kefnet's `ConditionDefined$
// RememberedCard | ConditionPresent$ Instant,Sorcery`.
func TestConditionDefinedRememberedCard(t *testing.T) {
	t.Parallel()
	probeCard, boltCard, bearCard := card(t, gateProbeSrc("RememberedCard", "Instant,Sorcery")), card(t, gateBoltSrc), card(t, gateBearSrc)
	e, cfg := tokenReplGame(t, 458, probeCard, boltCard, bearCard)
	probe := moveSeededCard(t, e, 0, probeCard, state.ZBattlefield)
	bolt := moveSeededCard(t, e, 0, boltCard, state.ZHand)
	bear := moveSeededCard(t, e, 0, bearCard, state.ZHand)
	if !runGate(t, e, probe, func(c *effects.Ctx) { c.Remembered = []state.Target{{Obj: bolt}} }) {
		t.Fatal("remembered instant: gate not met")
	}
	if runGate(t, e, probe, func(c *effects.Ctx) { c.Remembered = []state.Target{{Obj: bear}} }) {
		t.Fatal("remembered creature: gate met, want not")
	}
	replayCheck(t, e, cfg)
}

// TestConditionDefinedExiledWith: The Creation of Avacyn's `ConditionDefined$
// ExiledWith | ConditionPresent$ Creature`, over the cards the source exiled.
func TestConditionDefinedExiledWith(t *testing.T) {
	t.Parallel()
	exiler := "Name:Gate Exiler\nTypes:Artifact\n" +
		"SVar:Gate:DB$ GainLife | Defined$ You | LifeAmount$ 1 | ConditionDefined$ ExiledWith | ConditionPresent$ Creature | ConditionCompare$ GE1\nOracle:x\n"
	src := card(t, exiler)
	bearCard := card(t, gateBearSrc)
	e, cfg := tokenReplGame(t, 459, src, bearCard)
	probe := moveSeededCard(t, e, 0, src, state.ZBattlefield)
	bear := moveSeededCard(t, e, 0, bearCard, state.ZBattlefield)
	if runGate(t, e, probe, nil) {
		t.Fatal("nothing exiled: gate met, want not")
	}
	// The MoveZone an effect's exile emits carries its source in IDs.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZExile, IDs: []state.ObjID{probe}})
	if b := e.G.Obj(bear); b.Zone != state.ZExile || b.ExiledWith != probe {
		t.Fatalf("precondition: Bear zone %v exiled with %d, want exile with %d", b.Zone, b.ExiledWith, probe)
	}
	if !runGate(t, e, probe, nil) {
		t.Fatal("exiled creature: gate not met")
	}
	replayCheck(t, e, cfg)
}

// TestConditionDefinedOriginalHost: Heliod's Punishment's granted ability's
// `ConditionDefined$ OriginalHost | ConditionPresent$ Card.counters_EQ0_TASK`
// reads the GRANTOR when the ability was granted, else the source itself.
func TestConditionDefinedOriginalHost(t *testing.T) {
	t.Parallel()
	e, cfg, probe, ids := gateGame(t, 460, "OriginalHost", "Card.counters_EQ0_TASK", gateBearSrc)
	bear := ids[0]
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "TASK", Amount: 1})
	if e.G.Obj(bear).Counter("TASK") != 1 || e.G.Obj(probe).Counter("TASK") != 0 {
		t.Fatal("precondition: the Bear has a task counter, the probe none")
	}
	if !runGate(t, e, probe, nil) {
		t.Fatal("source without task counters (no grantor): gate not met")
	}
	if runGate(t, e, probe, func(c *effects.Ctx) { c.Grantor = bear }) {
		t.Fatal("grantor with a task counter: gate met, want not")
	}
	replayCheck(t, e, cfg)
}
