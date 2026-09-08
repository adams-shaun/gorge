package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Seed two real copies, with the decoy FIRST in battlefield order. Every
// setup mutation is logged, so the paid activation can be replayed too.
func selfSacrificeBoard(t *testing.T, c *cards.Card) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	cfg := Config{Seed: 31, Names: []string{"a", "b"}, Decks: [][]*cards.Card{
		append([]*cards.Card{c, c}, mountainDeck(t, 38)...), mountainDeck(t, 40),
	}}
	e := New(cfg)
	var ids []state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Owner == 0 && o.Card == c {
			ids = append(ids, o.ID)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("copies = %v", ids)
	}
	for _, id := range ids {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZBattlefield})
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	for _, colour := range "WUBRGC" {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: string(colour), Amount: 8})
	}
	return e, cfg, ids[1], ids[0]
}

func TestCardnameSacrificeAbilityOfferedAndPaid(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Flooded Strand", "Mogg Fanatic", "Wasteland", "Expedition Map", "Executioner's Capsule", "Moth Herb Elixir"} {
		t.Run(name, func(t *testing.T) {
			c := mustCorpusCard(t, reg, name)
			e, cfg, source, decoy := selfSacrificeBoard(t, c)
			idx := -1
			for i, sa := range c.Faces[0].Abilities {
				for _, part := range ParseCost(sa.Params["Cost"]).Sac {
					if part.Spec == "CARDNAME" && sa.Kind == "AB" && sa.API != "Mana" {
						idx = i
					}
				}
			}
			if idx < 0 {
				t.Fatal("missing compiled self-sacrifice ability")
			}
			found := false
			for _, opt := range e.legalActions(0) {
				if opt.Kind == "ability" && opt.Obj == source && opt.Ability == idx {
					found = true
				}
			}
			if !found {
				t.Fatal("legalActions did not offer the CARDNAME sacrifice ability")
			}
			e.Advance()
			opt := abilityOption(t, e, source, idx)
			submitChoices(t, e, opt.Index)
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 || len(d.Options) != 1 || d.Options[0].Kind != "sacrifice" || d.Options[0].Obj != source {
				t.Fatalf("payment must offer only source %d, never same-name decoy %d: %+v", source, decoy, d)
			}
			submitChoices(t, e, d.Options[0].Index)
			if e.G.Obj(source).Zone != state.ZGraveyard || e.G.Obj(decoy).Zone != state.ZBattlefield {
				t.Fatal("did not sacrifice exactly the source")
			}
			if countMoves(e.L.Events, source, state.ZGraveyard) != 1 || !hasEvent(e, events.AbilityPush, source) {
				t.Fatal("cost must be charged once before pushing the ability")
			}
			if name == "Mogg Fanatic" {
				d = e.Pending()
				if d == nil || d.Kind != decision.KTarget {
					t.Fatalf("no damage target: %+v", d)
				}
				life := e.G.Players[1].Life
				submitChoices(t, e, indexOfPlayerOption(d, 1))
				passUntilStackEmpty(t, e, 20)
				if e.G.Players[1].Life != life-1 {
					t.Fatal("sacrificed Fanatic's ability did not deal one damage")
				}
			}
			// Flooded Strand's PayLife still parses to {1}, not life; that separate
			// cost-grammar defect is deliberately not fixed or claimed here.
			replayCheck(t, e, cfg)
		})
	}
}

func TestCardnameSacrificeCostCannotUseAnotherCopy(t *testing.T) {
	c := card(t, "Name:Self payer\nTypes:Artifact\nA:AB$ GainLife | Cost$ Sac<1/CARDNAME/this artifact> | ActivationZone$ Graveyard | Defined$ You | LifeAmount$ 1\nOracle:x\n")
	e, _, source, _ := selfSacrificeBoard(t, c)
	e.emit(events.Event{Kind: events.MoveZone, Obj: source, From: state.ZBattlefield, To: state.ZGraveyard})
	for _, opt := range e.legalActions(0) {
		if opt.Kind == "ability" && opt.Obj == source {
			t.Fatal("graveyard source offered by counting another copy")
		}
	}
	// Payment must independently reject the same impossible cost, even if a
	// stale/manual activation reaches it without the offer gate.
	start := len(e.L.Events)
	e.beginActivation(0, decision.Option{Obj: source, Ability: 0})
	if e.cast != nil || e.Pending() != nil || len(e.G.Stack) != 0 {
		t.Fatal("payment accepted another copy")
	}
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.MoveZone || ev.Kind == events.AbilityPush {
			t.Fatal("aborted payment mutated the board")
		}
	}
}
