package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// madnessAskAfterDiscard seeds cardName over lands, discards it with an empty
// mana pool, and resolves the madness trigger up to its cast-or-not ask.
func madnessAskAfterDiscard(t *testing.T, seed uint64, cardName string, heroFixtures, lands []string) (*Engine, Config, *decision.Decision, state.ObjID) {
	t.Helper()
	e, cfg, _ := altCostEngine(t, seed, []string{cardName}, heroFixtures, nil)
	for _, name := range lands {
		moveToBattlefieldByName(t, e, 0, name)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: pool = %d, want empty", got)
	}
	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != len(lands) {
		t.Fatalf("precondition: %d permanents, want %d untapped sources", got, len(lands))
	}
	id := findCardObj(t, e, 0, cardName, state.ZHand)
	e.pending = nil
	e.emit(events.Discard(id, 0))
	e.priorityRound()
	passOnceP(t, e)
	passOnceP(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional || d.Source != id {
		t.Fatalf("madness resolution choice: %+v", d)
	}
	return e, cfg, d, id
}

func madnessOption(d *decision.Decision, kind string) int {
	for _, o := range d.Options {
		if o.Kind == kind {
			return o.Index
		}
	}
	return -1
}

// TestMadnessOfferedWithUntappedLands: CR 702.35b with CR 601.2g -- the
// madness cast is offered whenever untapped sources can pay for it, not only
// against floating mana; the cast then taps a Mountain for the {R}.
func TestMadnessOfferedWithUntappedLands(t *testing.T) {
	t.Parallel()
	t.Run("two untapped Mountains", func(t *testing.T) {
		e, cfg, d, temper := madnessAskAfterDiscard(t, 935, "Fiery Temper", nil, []string{"Mountain", "Mountain"})
		yes, no := madnessOption(d, "yes"), madnessOption(d, "no")
		if yes < 0 || no < 0 || len(d.Options) != 2 {
			t.Fatalf("madness ask options = %+v, want yes and no", d.Options)
		}
		submitChoices(t, e, yes)
		// Answer the cast's own asks (Temper's target, then the payment
		// window) until priority returns; the payment taps a Mountain.
		for i := 0; i < 8; i++ {
			d := e.Pending()
			if d == nil || d.Kind == decision.KPriority {
				break
			}
			submitChoices(t, e, 0)
		}
		if z := e.G.Obj(temper).Zone; z != state.ZStack {
			t.Fatalf("Fiery Temper in %s after choosing yes, want stack (pending %+v)", z, e.Pending())
		}
		tapped := 0
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if e.G.Obj(id).Tapped {
				tapped++
			}
		}
		if tapped != 1 {
			t.Fatalf("%d Mountains tapped, want exactly 1 for {R}", tapped)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("no untapped red source", func(t *testing.T) {
		_, _, d, _ := madnessAskAfterDiscard(t, 936, "Fiery Temper", nil, nil)
		if len(d.Options) != 1 || d.Options[0].Kind != "no" {
			t.Fatalf("madness ask options = %+v, want only no", d.Options)
		}
	})
	// A source whose RestrictValid$ cannot pay for this spell is not part of
	// the potential bound (CR 106.6), so the offer is withheld and declining
	// puts the card in its owner's graveyard (CR 702.35b).
	t.Run("restricted source cannot pay", func(t *testing.T) {
		restricted := "Name:Picky Rock\nTypes:Land\n" +
			"A:AB$ Mana | Cost$ T | Produced$ R | RestrictValid$ Spell.Creature\nOracle:x\n"
		e, cfg, d, temper := madnessAskAfterDiscard(t, 937, "Fiery Temper", []string{restricted}, []string{"Picky Rock"})
		if len(d.Options) != 1 || d.Options[0].Kind != "no" {
			t.Fatalf("madness ask options = %+v, want only no from a creature-only source", d.Options)
		}
		submitChoices(t, e, madnessOption(d, "no"))
		if z := e.G.Obj(temper).Zone; z != state.ZGraveyard {
			t.Fatalf("Fiery Temper in %s after declining, want graveyard", z)
		}
		replayCheck(t, e, cfg)
	})
	// CR 601.2c: a spell with no legal target cannot be cast, so the madness
	// ask must not offer it (the cast would reverse and strand the card).
	t.Run("targetless board", func(t *testing.T) {
		e, cfg, d, whispers := madnessAskAfterDiscard(t, 938, "Malevolent Whispers", nil,
			[]string{"Mountain", "Mountain", "Mountain", "Mountain"})
		if len(d.Options) != 1 || d.Options[0].Kind != "no" {
			t.Fatalf("madness ask options = %+v, want only no with no creatures to target", d.Options)
		}
		submitChoices(t, e, madnessOption(d, "no"))
		if z := e.G.Obj(whispers).Zone; z != state.ZGraveyard {
			t.Fatalf("Malevolent Whispers in %s after declining, want graveyard", z)
		}
		replayCheck(t, e, cfg)
	})
	// CR 702.35b: a madness cast that begins and then reverses was not cast,
	// so the card goes to the graveyard rather than staying in exile.
	t.Run("aborted cast goes to the graveyard", func(t *testing.T) {
		e, cfg, d, temper := madnessAskAfterDiscard(t, 939, "Fiery Temper", nil, []string{"Mountain", "Mountain"})
		submitChoices(t, e, madnessOption(d, "yes"))
		if e.cast == nil {
			t.Fatalf("precondition: no in-flight madness cast after choosing yes; pending %+v", e.Pending())
		}
		e.abortCast(e.cast, "test abort", false)
		e.pending = nil
		if z := e.G.Obj(temper).Zone; z != state.ZGraveyard {
			t.Fatalf("Fiery Temper in %s after the cast reversed, want graveyard", z)
		}
		replayCheck(t, e, cfg)
	})
}
