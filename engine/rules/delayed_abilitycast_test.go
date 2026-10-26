package rules

// cr-g139b: a DelayedTrigger registered at Mode$ AbilityCast / SpellAbilityCast
// must fire on the next matching activation (Pit Automaton, Repeated
// Reverberation). The registration, firing and one-shot removal all ride the
// ordinary DelayedRegister/DelayedPush path; only the event gate, the mode
// admission and the matcher dispatch were missing.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	dacExhaustA = "Name:Exhaust Alpha\nManaCost:1\nTypes:Creature Construct\nPT:1/1\nA:AB$ GainLife | Cost$ 1 | Exhaust$ True | Defined$ You | LifeAmount$ 2 | SpellDescription$ Gain 2.\nOracle:x\n"
	dacExhaustB = "Name:Exhaust Beta\nManaCost:1\nTypes:Creature Construct\nPT:1/1\nA:AB$ GainLife | Cost$ 1 | Exhaust$ True | Defined$ You | LifeAmount$ 2 | SpellDescription$ Gain 2.\nOracle:x\n"
	dacPlain    = "Name:Plain Gainer\nManaCost:1\nTypes:Creature Construct\nPT:1/1\nA:AB$ GainLife | Cost$ 1 | Defined$ You | LifeAmount$ 3 | SpellDescription$ Gain 3.\nOracle:x\n"
	dacManaDork = "Name:Dac Bloom\nManaCost:G\nTypes:Creature Plant\nPT:0/1\nA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Add {G}.\nOracle:x\n"
)

func dacSetup(t *testing.T, names ...string) (*Engine, Config, map[string]state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	front := []*cards.Card{searchCorpusCard(t, reg, "Pit Automaton")}
	for _, src := range []string{dacExhaustA, dacExhaustB, dacPlain, dacManaDork} {
		front = append(front, card(t, src))
	}
	all := append([]string{"Pit Automaton"}, names...)
	e, cfg, ids := plannedOfferSetup(t, reg, front, all...)
	return e, cfg, ids
}

// dacActivate activates src's ability whose label contains frag and resolves
// the stack. Returns the stack depth right after the activation.
func dacActivate(t *testing.T, e *Engine, src state.ObjID, frag, fund string) int {
	t.Helper()
	addMana(t, e, 0, fund)
	d := e.Pending()
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == src && strings.Contains(o.Label, frag) {
			submitChoices(t, e, o.Index)
			return len(e.G.Stack)
		}
	}
	t.Fatalf("no ability option on %d containing %q: %+v", src, frag, d.Options)
	return 0
}

// dacActivateMana activates src's mana ability. A mana ability never reaches
// the stack, so there is no depth to return and no copy to observe there.
func dacActivateMana(t *testing.T, e *Engine, src state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "")
	d := e.Pending()
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == src {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no mana activation on %d: %+v", src, d.Options)
}

func dacDelayed(e *Engine, mode string) int {
	n := 0
	for _, dt := range e.G.Delayed {
		if dt.EventMode == mode {
			n++
		}
	}
	return n
}

func dacNotImplemented(e *Engine) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "not implemented") {
			return true
		}
	}
	return false
}

func dacRegister(t *testing.T, e *Engine, pit state.ObjID) {
	t.Helper()
	dacActivate(t, e, pit, "copy it", "CC")
	passUntilStackEmpty(t, e, 30)
	if got := dacDelayed(e, "AbilityCast"); got != 1 {
		t.Fatalf("AbilityCast registrations = %d, want 1 (delayed %+v)", got, e.G.Delayed)
	}
	if dacNotImplemented(e) {
		t.Fatal("a not implemented Note was emitted")
	}
}

func TestPitAutomatonCopiesNextExhaust(t *testing.T) {
	t.Parallel()
	e, cfg, ids := dacSetup(t, "Exhaust Alpha")
	dacRegister(t, e, ids["Pit Automaton"])
	life := e.G.Players[0].Life
	if depth := dacActivate(t, e, ids["Exhaust Alpha"], "Gain 2", "C"); depth == 0 {
		t.Fatal("exhaust activation not on the stack")
	}
	passUntilStackEmpty(t, e, 30)
	// The original plus exactly one copy of the activated ability.
	if got := e.G.Players[0].Life; got != life+4 {
		t.Fatalf("life %d, want %d (ability plus its copy)", got, life+4)
	}
	if !hasEventKind(e, events.StackCopy) {
		t.Fatal("no StackCopy was put on the stack for the activated ability")
	}
	if got := dacDelayed(e, "AbilityCast"); got != 0 {
		t.Fatalf("registration still live after firing: %d", got)
	}
	replayCheck(t, e, cfg)
}

func TestPitAutomatonIgnoresNonExhaustAndMana(t *testing.T) {
	t.Parallel()
	e, cfg, ids := dacSetup(t, "Plain Gainer", "Dac Bloom")
	dacRegister(t, e, ids["Pit Automaton"])
	life := e.G.Players[0].Life
	dacActivate(t, e, ids["Plain Gainer"], "Gain 3", "C")
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Players[0].Life; got != life+3 {
		t.Fatalf("life %d, want %d (non-exhaust ability not copied)", got, life+3)
	}
	if got := dacDelayed(e, "AbilityCast"); got != 1 {
		t.Fatalf("registration consumed by a non-exhaust activation: %d", got)
	}
	dacActivateMana(t, e, ids["Dac Bloom"])
	if got := dacDelayed(e, "AbilityCast"); got != 1 {
		t.Fatalf("registration consumed by a mana ability: %d", got)
	}
	replayCheck(t, e, cfg)
}

func TestPitAutomatonOneShotAndThisTurn(t *testing.T) {
	t.Parallel()
	e, cfg, ids := dacSetup(t, "Exhaust Alpha", "Exhaust Beta")
	dacRegister(t, e, ids["Pit Automaton"])
	life := e.G.Players[0].Life
	dacActivate(t, e, ids["Exhaust Alpha"], "Gain 2", "C")
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Players[0].Life; got != life+4 {
		t.Fatalf("first exhaust: life %d, want %d (ability plus its copy)", got, life+4)
	}
	life = e.G.Players[0].Life
	dacActivate(t, e, ids["Exhaust Beta"], "Gain 2", "C")
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Players[0].Life; got != life+2 {
		t.Fatalf("second exhaust: life %d, want %d (one-shot, not copied)", got, life+2)
	}
	replayCheck(t, e, cfg)

	// A registration that never fires expires at the turn's end (ThisTurn$).
	e2, cfg2, ids2 := dacSetup(t)
	dacRegister(t, e2, ids2["Pit Automaton"])
	e2.emit(events.Event{Kind: events.TurnChange, Player: 1, Amount: e2.G.Turn + 1})
	if got := dacDelayed(e2, "AbilityCast"); got != 0 {
		t.Fatalf("unfired registration survived the turn end: %d", got)
	}
	replayCheck(t, e2, cfg2)
}

const dacInstant = "Name:Dac Sip\nManaCost:G\nTypes:Instant\nA:SP$ GainLife | LifeAmount$ 2 | SpellDescription$ Gain 2.\nOracle:x\n"

func TestDelayedSpellAbilityCastCopiesNextInstant(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := dotlessEngine(t, reg, searchCorpusCard(t, reg, "Repeated Reverberation"), card(t, dacInstant))
	rr := searchMoveByName(t, e, "Repeated Reverberation", state.ZHand)
	sip := searchMoveByName(t, e, "Dac Sip", state.ZHand)
	castReviewed(t, e, 0, rr, "RRCC")
	if got := dacDelayed(e, "SpellAbilityCast"); got != 1 {
		t.Fatalf("SpellAbilityCast registrations = %d, want 1", got)
	}
	if dacNotImplemented(e) {
		t.Fatal("a not implemented Note was emitted")
	}
	life := e.G.Players[0].Life
	castReviewed(t, e, 0, sip, "G")
	passUntilStackEmpty(t, e, 30)
	// Amount$ 2: the spell plus two copies.
	if got := e.G.Players[0].Life; got != life+6 {
		t.Fatalf("life %d, want %d (instant and two copies)", got, life+6)
	}
	if got := dacDelayed(e, "SpellAbilityCast"); got != 0 {
		t.Fatalf("registration still live after firing: %d", got)
	}
	replayCheck(t, e, cfg)
}
