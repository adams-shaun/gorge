package rules

// Etched Oracle's Count$UniqueManaColorsProduced is read only by the Forge AI
// hint SVar:NeedsToPlayVar:Z GE4, never by its rules text, so the value-head
// census must not report it (cards' referencedNames skips NeedsToPlayVar).
// The printed ability ({1}, remove four +1/+1 counters: target player draws
// three) is pinned on the REAL corpus card.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// oracleAbilityOption is the priority option activating id's ability, or -1.
func oracleAbilityOption(e *Engine, id state.ObjID) int {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return -1
	}
	for _, op := range d.Options {
		if op.Kind == "ability" && op.Obj == id {
			return op.Index
		}
	}
	return -1
}

func etchedOracleOnBattlefield(t *testing.T, reg *cards.Registry, counters int32) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Etched Oracle")}, nil)
	id := moveByName(t, e, 0, "Etched Oracle", state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: counters})
	if got := e.G.Obj(id).Counter("P1P1"); got != counters {
		t.Fatalf("precondition: Oracle P1P1=%d, want %d", got, counters)
	}
	addMana(t, e, 0, "R")
	return e, cfg, id
}

func TestEtchedOracleIsSupportedAndDrawsThree(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	t.Run("registry", func(t *testing.T) {
		if got := reg.Unsupported(lookup(t, reg, "Etched Oracle"), effects.Supported()); len(got) != 0 {
			t.Fatalf("Etched Oracle unsupported: %v", got)
		}
		for _, name := range []string{"Baton of Courage", "Chamber Sentry", "Clearwater Goblet",
			"Heliophial", "Infused Arrows", "Lunar Avenger", "Opaline Bracers", "Pentad Prism",
			"Sawtooth Thresher", "Skyreach Manta", "Solarion", "Spinal Parasite", "Suncrusher",
			"Suntouched Myr"} {
			for _, r := range reg.Unsupported(lookup(t, reg, name), effects.Supported()) {
				if r == "count:UniqueManaColorsProduced" {
					t.Errorf("%s still reports %s", name, r)
				}
			}
		}
	})
	t.Run("four counters draw three", func(t *testing.T) {
		e, cfg, id := etchedOracleOnBattlefield(t, reg, 4)
		hand := len(e.G.Zone(state.ZHand, 0))
		idx := oracleAbilityOption(e, id)
		if idx < 0 {
			t.Fatalf("precondition: Oracle's ability not offered: %+v", e.Pending().Options)
		}
		submitChoices(t, e, idx)
		answerTargetPlayer(t, e, 0)
		passUntilStackEmpty(t, e, 20)
		if got := len(e.G.Zone(state.ZHand, 0)); got != hand+3 {
			t.Fatalf("seat 0 hand = %d, want %d (drew three)", got, hand+3)
		}
		if got := e.G.Obj(id).Counter("P1P1"); got != 0 {
			t.Fatalf("Oracle P1P1 = %d after paying, want 0", got)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("three counters cannot pay", func(t *testing.T) {
		e, _, id := etchedOracleOnBattlefield(t, reg, 3)
		if idx := oracleAbilityOption(e, id); idx >= 0 {
			t.Fatalf("Oracle's ability offered with three counters (option %d)", idx)
		}
	})
}

// answerTargetPlayer answers pending non-priority asks until priority returns,
// picking the player-target option naming seat p (else the first option).
func answerTargetPlayer(t *testing.T, e *Engine, p state.PlayerID) {
	t.Helper()
	for i := 0; i < 10; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			return
		}
		pick := -1
		for _, op := range d.Options {
			if op.Kind == "player" && op.Player == p {
				pick = op.Index
			}
		}
		if pick < 0 {
			t.Logf("ask %v %q: %+v", d.Kind, d.ResumeKind, d.Options)
			pick = d.Options[0].Index
		}
		submitChoices(t, e, pick)
	}
}
