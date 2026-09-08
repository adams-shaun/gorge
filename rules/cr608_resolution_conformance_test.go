package rules

// Reference: Magic: The Gathering Comprehensive Rules, 2026-08-07.
// CR 608.2b (5485-5498): recheck legality, not merely the target's zone.
// CR 608.2n (5599-5601): the final graveyard move (NOT 608.2m).
// Scenario guards below assert real fixtures and completed operations; there
// are deliberately no unconditional "checked++" vacuity counters.

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Each seat starts with a real repo deck, plus only compiled corpus supplements.
func crResolutionEngine(t *testing.T, extras ...[]string) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 42, Tokens: reg.Tokens}
	for p, ns := range extras {
		name := "ur-delver"
		if p == 1 {
			name = "death-n-taxes"
		}
		cfg.Names = append(cfg.Names, name)
		deck := append([]*cards.Card{}, testutil.RepoDeck(t, reg, name)...)
		for _, n := range ns {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("CR 608 fixture %s seq 0: missing corpus card", n)
			}
			deck = append(deck, c)
		}
		cfg.Decks = append(cfg.Decks, deck)
	}
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e
}

func crResolutionRound(t *testing.T, e *Engine) {
	t.Helper()
	n := e.G.AliveCount()
	for i := 0; i < n; i++ {
		if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
			t.Fatalf("CR 608 fixture seq %d: expected priority, got %+v", len(e.L.Events), d)
		}
		crAbortAnswer(t, e, "resolution", crAbortOption(t, e, "resolution", "pass", 0))
	}
}

func crResolutionPlayerTarget(t *testing.T, e *Engine, p state.PlayerID) {
	t.Helper()
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		for _, o := range d.Options {
			if o.Kind == "player" && o.Player == p {
				crAbortAnswer(t, e, "resolution", o.Index)
				return
			}
		}
	}
	t.Fatalf("CR 608 fixture seq %d: player %d not offered", len(e.L.Events), p)
}

func TestCR608CompletedSpellLeavesStackAfterDepartedPayer(t *testing.T) {
	requireCR601Audit(t, "CR 608.2n: an earlier departed payer's continuation strands a completed spell (I-1)")
	e := crResolutionEngine(t, nil, nil, nil)
	chain := crAbortMove(t, e, 0, "Chain Lightning", state.ZHand)
	bolt := crAbortMove(t, e, 0, "Lightning Bolt", state.ZHand)
	sa := e.G.Obj(chain).Face().SpellAbility()
	if sa == nil || sa.API != "DealDamage" || sa.Sub == nil || sa.Sub.API != "CopySpellAbility" {
		t.Fatal("CR 608.2n Chain Lightning seq 0: missing real pay-copy chain")
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -17})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 2})
	e.askPriority(0)
	crAbortAnswer(t, e, "Chain Lightning", crAbortOption(t, e, "Chain Lightning", "cast", chain))
	crResolutionPlayerTarget(t, e, 1)
	crResolutionRound(t, e)
	if !e.G.Players[1].Lost {
		t.Fatalf("CR 608.2n Chain Lightning seq %d: lethal damage did not eliminate payer", len(e.L.Events))
	}
	// Allow the existing fizzle exit to clear the first spell. A corrected
	// engine may already have completed its resolution in the same burst.
	if len(e.G.Stack) > 0 {
		crResolutionRound(t, e)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("CR 608.2n Chain Lightning seq %d: first spell still on stack %v", len(e.L.Events), e.G.Stack)
	}
	start, life := len(e.L.Events), e.G.Players[2].Life
	crAbortAnswer(t, e, "Lightning Bolt", crAbortOption(t, e, "Lightning Bolt", "cast", bolt))
	crResolutionPlayerTarget(t, e, 2)
	crResolutionRound(t, e)
	if e.G.Players[2].Life != life-3 {
		t.Fatalf("CR 608.2n Lightning Bolt seq %d: live-target damage fixture did not run once", start)
	}
	if e.G.Obj(bolt).Zone != state.ZGraveyard || len(e.G.Stack) != 0 {
		t.Errorf("CR 608.2n Lightning Bolt seq %d: completed damage but zone=%s stack=%v; want graveyard and empty stack", start, e.G.Obj(bolt).Zone, e.G.Stack)
	}
}

// This corpus invariant examines single-face, required-target, literal burn
// SAs without sub-abilities. It isolates resolution from casting: normal hand
// origin, no alternative permission, no flashback/copy flags. Thus Eelectrocute's
// graveyard-casting replacement must NOT exile either an ordinary resolution
// or an all-targets-illegal spell. Expectations use zones/order, never the
// engine's target or resting-zone helpers. Counter counts EXAMINED SAs.
func TestCR608CorpusOrdinaryBurnFinishesInGraveyard(t *testing.T) {
	requireCR601Audit(t, "CR 608.2b/n: a conditional graveyard-cast replacement exiles an ordinary hand-origin spell")
	reg := testutil.CorpusRegistry(t)
	checked := 0
	for _, c := range reg.Cards {
		if len(c.Faces) != 1 {
			continue
		}
		f := c.Faces[0]
		sa := f.SpellAbility()
		if f.IsPermanent() || f.IsLand() || sa == nil || sa.Kind != "SP" || sa.API != "DealDamage" || sa.Sub != nil {
			continue
		}
		switch sa.Params["ValidTgts"] {
		case "Any", "Creature", "Creature,Player":
		default:
			continue
		}
		if sa.Params["TargetMin"] != "" && sa.Params["TargetMin"] != "1" {
			continue
		}
		n, err := strconv.Atoi(sa.Params["NumDmg"])
		if err != nil || n < 1 || n > 20 {
			continue
		}
		// Guard movement replacements rather than silently asserting graveyard
		// for a future card with a legitimate self-exile/shuffle rule. Counter
		// and damage replacements do not change this resting-zone oracle.
		for _, r := range f.Repls {
			if r.Event == "Moved" && (f.Name != "Eelectrocute" || r.Params["ValidLKI"] != "Card.CastSa Spell.MayPlaySource") {
				t.Fatalf("CR 608.2n %s seq 0: new movement replacement needs an independent resting-zone oracle", f.Name)
			}
		}
		checked++
		for _, departed := range []bool{false, true} {
			e := crAbortEngine(t, reg, "ur-delver", f.Name)
			id := crAbortMove(t, e, 0, f.Name, state.ZHand)
			target := crAbortMove(t, e, 0, "Delver of Secrets", state.ZBattlefield)
			e.emit(events.Event{Kind: events.PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
			e.emit(events.Event{Kind: events.TargetsChosen, Obj: id, IDs: []state.ObjID{target}})
			if departed {
				e.emit(events.Event{Kind: events.MoveZone, Obj: target, From: state.ZBattlefield, To: state.ZGraveyard})
			}
			start := len(e.L.Events)
			e.pending = nil
			e.resolveTop()
			resolves, resolved, moved := 0, -1, -1
			for _, ev := range e.L.Events[start:] {
				if ev.Kind == events.Resolve && ev.Obj == id {
					resolves++
					resolved = int(ev.Seq)
				}
				if ev.Kind == events.MoveZone && ev.Obj == id && ev.From == state.ZStack {
					moved = int(ev.Seq)
				}
				if departed && ev.Kind == events.Damage {
					t.Errorf("CR 608.2b %s seq %d: all targets departed but damage ran", f.Name, ev.Seq)
				}
			}
			wantResolves := 1
			if departed {
				wantResolves = 0
			}
			if resolves != wantResolves || moved <= resolved || e.G.Obj(id).Zone != state.ZGraveyard || len(e.G.Stack) != 0 {
				t.Errorf("CR 608.2b/n %s seq %d: departed=%t resolves=%d finalMove=%d zone=%s stack=%v; want resolves=%d then graveyard", f.Name, start, departed, resolves, moved, e.G.Obj(id).Zone, e.G.Stack, wantResolves)
			}
		}
	}
	if checked == 0 {
		t.Fatal("CR 608.2b/n corpus seq 0: no compiled burn SAs examined")
	}
	t.Logf("MEASURED CR 608.2b/n examined=%d compiled single-face literal burn SAs, live/departed target cases each", checked)
}

func TestCR608ResolutionRechecksVinesTargetRestriction(t *testing.T) {
	requireCR601Audit(t, "CR 608.2b: Vines restriction is checked when targeting, not when resolving")
	e := crResolutionEngine(t, nil, []string{"Vines of Vastwood"})
	bolt := crAbortMove(t, e, 0, "Lightning Bolt", state.ZHand)
	vines := crAbortMove(t, e, 1, "Vines of Vastwood", state.ZHand)
	target := crAbortMove(t, e, 1, "Serra Avenger", state.ZBattlefield)
	vsa := e.G.Obj(vines).Face().SpellAbility()
	if vsa == nil || vsa.Sub == nil || vsa.Sub.API != "Effect" || vsa.Sub.Params["StaticAbilities"] != "STCantTarget" {
		t.Fatal("CR 608.2b Vines of Vastwood seq 0: restriction fixture changed")
	}
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 1})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 1, Counter: "G", Amount: 1})
	e.askPriority(0)
	crAbortAnswer(t, e, "Lightning Bolt", crAbortOption(t, e, "Lightning Bolt", "cast", bolt))
	crAbortAnswer(t, e, "Lightning Bolt", crAbortOption(t, e, "Lightning Bolt", "permanent", target))
	crAbortAnswer(t, e, "response", crAbortOption(t, e, "response", "pass", 0))
	crAbortAnswer(t, e, "Vines of Vastwood", crAbortOption(t, e, "Vines of Vastwood", "cast", vines))
	crAbortAnswer(t, e, "Vines of Vastwood", crAbortOption(t, e, "Vines of Vastwood", "permanent", target))
	crResolutionRound(t, e)
	if e.G.Obj(vines).Zone != state.ZGraveyard || e.G.Obj(target).Damage != 0 || len(e.G.Stack) != 1 {
		t.Fatalf("CR 608.2b Vines of Vastwood seq %d: response did not finish cleanly", len(e.L.Events))
	}
	start := len(e.L.Events)
	crResolutionRound(t, e)
	// Oracle: Vines controlled by seat 1 forbids seat 0's Bolt from targeting
	// this creature for the rest of the turn. No engine targeting helper is an oracle.
	if e.G.Obj(target).Zone != state.ZBattlefield || e.G.Obj(target).Damage != 0 {
		t.Errorf("CR 608.2b Lightning Bolt/Vines of Vastwood seq %d: illegal target affected, zone=%s damage=%d", start, e.G.Obj(target).Zone, e.G.Obj(target).Damage)
	}
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Resolve && ev.Obj == bolt {
			t.Errorf("CR 608.2b Lightning Bolt seq %d: all targets illegal but Resolve emitted", ev.Seq)
		}
	}
}
