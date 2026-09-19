package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"

	"github.com/adams-shaun/gorge/state"
)

// Alania, Divergent Storm's cast trigger (task castprov2), pinned end to end
// on the real corpus card:
//
//	T:Mode$ SpellCast | ValidCard$ Instant,Sorcery,Otter.!CastSaSource |
//	     ActivatorThisTurnCastEach$ EQ1 | ValidActivatingPlayer$ You | ...
//
// Two independent defects kept it dead: the bare !CastSaSource token inside
// the trigger ValidCard$ was unknown to the filter (the whole comma-set
// matched nobody), and ActivatorThisTurnCastEach$ was unimplemented (only
// the singular ActivatorThisTurnCast was read). The pins:
//
//   - the first instant of the turn fires (the target opponent draws, the
//     copy resolves); a second instant the same turn does not;
//   - the first SORCERY after an instant DOES — the Each semantics is a
//     disjunction of per-alternative firsts;
//   - a second Alania cast (an Otter) does not — the !CastSaSource name
//     exclusion drops the Otter alternative and the others cannot match a
//     creature cast.
//
// OptionalDecider$ You is not a trigger ask this build poses (the draw is
// mandatory once the trigger fires); only the Draw's own target ask is
// answered here. The cast probes are target-less, ask-less synthetic spells
// so a cast never poses a decision of its own; Alania itself is the real
// corpus card.

const alaniaProbeSrc = "Name:Probe\nManaCost:U\nTypes:%s\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:Draw a card.\n"

// alaniaEngine puts Alania on the battlefield (un-cast — it is the trigger
// SOURCE, not one of the pinned casts) and the given cards in seat 0's hand,
// then hands seat 0 a full mana pool.
func alaniaEngine(t *testing.T, hand ...*cards.Card) *Engine {
	t.Helper()
	e := handEngine(t, append([]*cards.Card{corpusAlternativeCard(t, "Alania, Divergent Storm")}, hand...)...)
	alania := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: alania, From: state.ZHand, To: state.ZBattlefield})
	drainQueuedTriggers(t, e)
	for _, r := range "UUUUUUURRRRRRRRGGGGGGGG" {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: string(r), Amount: 1})
	}
	return e
}

// castProbe casts the named card in seat 0's hand (found by name, never by
// index — the casts reshuffle the hand's front) and drains the resolution
// and any queued trigger, answering the trigger draws' target ask when one
// surfaces. Returns the number of decisions it answered.
func castAlaniaProbe(t *testing.T, e *Engine, name string) {
	t.Helper()
	id := state.ObjID(0)
	for _, hid := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(hid); o != nil && o.Face() != nil && o.Face().Name == name {
			id = hid
		}
	}
	if id == 0 {
		t.Fatalf("%s not in hand", name)
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KPriority {
		idx := -1
		for i, o := range d.Options {
			if o.Kind == "cast" && o.Obj == id {
				idx = i
			}
		}
		if idx < 0 {
			// A stale priority round (its options predate this cast's mana):
			// re-ask so the option set reflects the current board.
			e.priorityRound()
			d = e.Pending()
			for i, o := range d.Options {
				if o.Kind == "cast" && o.Obj == id {
					idx = i
				}
			}
		}
		if idx < 0 {
			t.Fatalf("no cast option for %s: %+v", name, d.Options)
		}
		submitChoices(t, e, idx)
	} else {
		castMode(t, e, id, "")
	}
	// Drain the resolution and every trigger it queued, answering the
	// trigger draws' target ask when it surfaces.
	for i := 0; i < 50; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		d := e.Pending()
		if d != nil && d.Kind == decision.KTriggerOptional {
			// The optional-trigger asks (R2's placement ask and CR 603.5's
			// resolution ask, OptionalDecider$ You): answer yes.
			submitChoices(t, e, 0)
			continue
		}
		if d != nil && d.Kind == decision.KTarget {
			// Player target offers include every living player (askTarget's
			// documented coarse offer); pick the actual opponent seat.
			idx := -1
			for i, o := range d.Options {
				if o.Kind == "player" && o.Player == 1 {
					idx = i
				}
			}
			submitChoices(t, e, idx)
			continue
		}
		if len(e.G.Stack) > 0 {
			e.resolveTop()
			continue
		}
		return
	}
	t.Fatalf("the drain did not settle after casting %s", name)
}

func opponentHand(t *testing.T, e *Engine) int {
	t.Helper()
	return len(e.G.Zone(state.ZHand, 1))
}

func TestAlaniaFirstInstantTriggersAndCopies(t *testing.T) {
	t.Parallel()
	probe := card(t, "Name:Probe\nManaCost:U\nTypes:Instant\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:Draw a card.\n")
	e := alaniaEngine(t, probe)
	before := opponentHand(t, e)
	castAlaniaProbe(t, e, "Probe")
	if got := opponentHand(t, e); got != before+1 {
		t.Fatalf("the target opponent drew %d cards, want 1 (the first instant of the turn must fire)", got-before)
	}
}

func TestAlaniaSecondInstantDoesNotTrigger(t *testing.T) {
	t.Parallel()
	probe := card(t, "Name:Probe\nManaCost:U\nTypes:Instant\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:Draw a card.\n")
	probe2 := card(t, "Name:Probe II\nManaCost:U\nTypes:Instant\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:Draw a card.\n")
	e := alaniaEngine(t, probe, probe2)
	castAlaniaProbe(t, e, "Probe") // the first instant: fires
	before := opponentHand(t, e)
	castAlaniaProbe(t, e, "Probe II") // the second instant: must not fire
	if got := opponentHand(t, e); got != before {
		t.Fatalf("the second instant drew %d cards for the opponent, want 0 (EQ1 fails at 2)", got-before)
	}
}

func TestAlaniaFirstSorceryAfterAnInstantTriggers(t *testing.T) {
	t.Parallel()
	probe := card(t, "Name:Probe\nManaCost:U\nTypes:Instant\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:Draw a card.\n")
	verse := card(t, "Name:Verse\nManaCost:1 G\nTypes:Sorcery\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:Draw a card.\n")
	e := alaniaEngine(t, probe, verse)
	castAlaniaProbe(t, e, "Probe") // the first instant: fires
	before := opponentHand(t, e)
	castAlaniaProbe(t, e, "Verse") // the first SORCERY: the Each disjunction fires on it
	if got := opponentHand(t, e); got != before+1 {
		t.Fatalf("the first sorcery after an instant drew %d cards for the opponent, want 1 (Each semantics)", got-before)
	}
}

func TestAlaniaSecondInstantAfterASorceryDoesNotTrigger(t *testing.T) {
	t.Parallel()
	// The round-2 review edge the original pins missed: instant → sorcery →
	// second instant. The sorcery alternative's tally legitimately held EQ1
	// on the sorcery cast, but the SECOND INSTANT matches only the instant
	// alternative, whose tally is 2 — the Each loop must skip alternatives
	// the current cast does not match, or the trigger false-fires here.
	probe := card(t, "Name:Probe\nManaCost:U\nTypes:Instant\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:Draw a card.\n")
	verse := card(t, "Name:Verse\nManaCost:1 G\nTypes:Sorcery\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:Draw a card.\n")
	probe2 := card(t, "Name:Probe II\nManaCost:U\nTypes:Instant\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:Draw a card.\n")
	e := alaniaEngine(t, probe, verse, probe2)
	castAlaniaProbe(t, e, "Probe") // the first instant: fires
	castAlaniaProbe(t, e, "Verse") // the first sorcery: fires
	before := opponentHand(t, e)
	castAlaniaProbe(t, e, "Probe II") // the second instant: must not fire
	if got := opponentHand(t, e); got != before {
		t.Fatalf("the second instant after a sorcery drew %d cards for the opponent, want 0 (it matches no alternative whose first it is)", got-before)
	}
}

func TestAlaniaSecondAlaniaCastDoesNotTrigger(t *testing.T) {
	t.Parallel()
	second := corpusAlternativeCard(t, "Alania, Divergent Storm")
	e := alaniaEngine(t, second)
	before := opponentHand(t, e)
	castAlaniaProbe(t, e, "Alania, Divergent Storm")
	if got := opponentHand(t, e); got != before {
		t.Fatalf("the second Alania cast drew %d cards for the opponent, want 0 (the Otter alternative drops on the name exclusion)", got-before)
	}
}
