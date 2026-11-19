package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edh_trigC_test.go plays the printed text of the Mode$ Destroyed carriers of
// the EDH trigger-modes class (ticket edh-trigC): Karmic Justice and Cobra
// Trap. Seat 0 is the active player and the "opponent" casting the removal;
// seat 1 owns the carrier.

// trigCSetup seats the fixtures: seat 0 holds the named spells in hand and
// the named battlefield permanents; seat 1 holds the named battlefield
// permanents and the named hand cards. Every printed card is asserted
// primitive-complete first so a fixture can never pass vacuously.
func trigCSetup(t *testing.T, seed uint64, spells, bf0, bf1, hand1 []string) (e *Engine, bf0IDs, bf1IDs, hand1IDs []state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, _, _, _ = effectGrantEngine(t, seed, nil, spells)
	for _, names := range [][]string{spells, bf0, bf1, hand1} {
		for _, n := range names {
			if m := reg.Unsupported(searchCorpusCard(t, reg, n), effects.Supported()); len(m) != 0 {
				t.Fatalf("precondition: %s unsupported: %v", n, m)
			}
		}
	}
	for _, n := range bf0 {
		bf0IDs = append(bf0IDs, putOnBattlefield(t, e, 0, searchCorpusCard(t, reg, n)))
	}
	for _, n := range bf1 {
		bf1IDs = append(bf1IDs, putOnBattlefield(t, e, 1, searchCorpusCard(t, reg, n)))
	}
	for _, n := range hand1 {
		o := e.G.AddObject(searchCorpusCard(t, reg, n), 1)
		o.Zone = state.ZHand
		e.G.SetZone(state.ZHand, 1, append(e.G.Zone(state.ZHand, 1), o.ID))
		hand1IDs = append(hand1IDs, o.ID)
	}
	return e, bf0IDs, bf1IDs, hand1IDs
}

// trigCCast casts spell (seat 0) at target and drives its resolution, answering
// every ask via answer (nil: take the first option), stopping at the first
// priority ask with an empty stack.
func trigCCast(t *testing.T, e *Engine, spell string, target state.ObjID, answer func(d *decision.Decision) (int, bool)) {
	t.Helper()
	castNamed(t, e, spell)
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil || (d.Kind == decision.KPriority && len(e.G.Stack) == 0) {
			return
		}
		if answer != nil {
			if idx, ok := answer(d); ok {
				submitChoices(t, e, idx)
				continue
			}
		}
		switch d.Kind {
		case decision.KTarget:
			if target != 0 {
				submitChoices(t, e, targetOptionIdx(t, e, target))
				target = 0
				continue
			}
			submitChoices(t, e, d.Options[0].Index)
		case decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("no pass option: %+v", d)
			}
			submitChoices(t, e, idx)
		default:
			t.Logf("decision %v %+v", d.Kind, d)
			submitChoices(t, e, d.Options[0].Index)
		}
	}
	t.Fatal("trigCCast exceeded its decision budget")
}

func trigCInZone(e *Engine, id state.ObjID, z state.Zone) bool {
	o := e.G.Obj(id)
	return o != nil && o.Zone == z
}

// trigCCastOptions lists the cast option labels of the pending priority ask.
func trigCCastOptions(e *Engine) []string {
	var out []string
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return out
	}
	for _, o := range d.Options {
		if o.Kind == "cast" {
			out = append(out, o.Label)
		}
	}
	return out
}

// trigCPass submits the pending priority ask's pass option.
func trigCPass(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority ask pending: %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "pass" {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no pass option: %+v", d.Options)
}

// TestEDHTrigCKarmicJusticeDestroyed plays Karmic Justice's printed text: a
// noncreature permanent you control destroyed by a spell an opponent controls
// fires it (its target ask is restricted to that opponent's permanents), while
// your own destroy, a creature, and a lethal-damage death do not.
func TestEDHTrigCKarmicJusticeDestroyed(t *testing.T) {
	t.Parallel()
	t.Run("opponent spell destroys your noncreature permanent", func(t *testing.T) {
		t.Parallel()
		e, bf0, bf1, _ := trigCSetup(t, 9411, []string{"Disenchant"}, []string{"Mountain", "Forest"}, []string{"Karmic Justice", "Sol Ring"}, nil)
		addMana(t, e, 0, "1W")
		sawAsk := false
		trigCCast(t, e, "Disenchant", bf1[1], func(d *decision.Decision) (int, bool) {
			if d.Kind != decision.KTarget || d.Player != 1 {
				return 0, false
			}
			// Karmic's own target ask: only the destroyer's permanents.
			sawAsk = true
			for _, o := range d.Options {
				if ob := e.G.Obj(o.Obj); ob == nil || ob.Controller != 0 {
					t.Errorf("Karmic Justice offered %+v, not a permanent seat 0 controls", o)
				}
			}
			return targetOptionIdx(t, e, bf0[0]), true
		})
		if !sawAsk {
			t.Fatal("Karmic Justice never asked for its target")
		}
		if !trigCInZone(e, bf1[1], state.ZGraveyard) {
			t.Fatalf("precondition: Sol Ring survived Disenchant")
		}
		if !trigCInZone(e, bf0[0], state.ZGraveyard) {
			t.Fatalf("Karmic Justice did not destroy the causer's Mountain")
		}
		if !trigCInZone(e, bf0[1], state.ZBattlefield) {
			t.Fatalf("Karmic Justice destroyed a permanent it did not target")
		}
	})
	t.Run("your own Wrath-style destroy does not", func(t *testing.T) {
		t.Parallel()
		// Seat 0 owns Karmic Justice and casts the Wrath: were the mode to
		// ignore the causer, the trigger would fire for seat 0, "TriggeredCauser"
		// would name seat 0 and its target ask would offer seat 0's own Mountain.
		e, bf0, bf1, _ := trigCSetup(t, 9413, []string{"Akroma's Vengeance"}, []string{"Karmic Justice", "Sol Ring", "Mountain"}, []string{"Mountain"}, nil)
		addMana(t, e, 0, "WWWWWW")
		trigCCast(t, e, "Akroma's Vengeance", 0, func(d *decision.Decision) (int, bool) {
			if d.Kind == decision.KTarget || d.Kind == decision.KChoose {
				t.Errorf("Karmic Justice fired on its own controller's destroy: %+v", d)
			}
			return 0, false
		})
		if !trigCInZone(e, bf0[1], state.ZGraveyard) {
			t.Fatalf("precondition: Akroma's Vengeance did not destroy seat 0's Sol Ring")
		}
		if !trigCInZone(e, bf0[2], state.ZBattlefield) || !trigCInZone(e, bf1[0], state.ZBattlefield) {
			t.Fatalf("a Mountain was destroyed: Karmic Justice fired on its own controller's spell")
		}
	})
	t.Run("a creature is not a noncreature permanent", func(t *testing.T) {
		t.Parallel()
		e, bf0, bf1, _ := trigCSetup(t, 9414, []string{"Terror"}, []string{"Mountain"}, []string{"Karmic Justice", "Grizzly Bears"}, nil)
		addMana(t, e, 0, "1B")
		trigCCast(t, e, "Terror", bf1[1], nil)
		if !trigCInZone(e, bf1[1], state.ZGraveyard) {
			t.Fatalf("precondition: Terror did not destroy the Bears")
		}
		if !trigCInZone(e, bf0[0], state.ZBattlefield) {
			t.Fatalf("Karmic Justice fired on a creature")
		}
	})
}

// TestEDHTrigCCobraTrapDestroyed plays Cobra Trap's printed text: its {G}
// alternative cost lights up only after a spell an opponent controls
// destroyed a noncreature permanent its controller controls this turn, and
// the tracker resets with the next turn.
func TestEDHTrigCCobraTrapDestroyed(t *testing.T) {
	t.Parallel()
	hasAlt := func(e *Engine) bool {
		for _, l := range trigCCastOptions(e) {
			if l == "Cast Cobra Trap (alternative cost)" {
				return true
			}
		}
		return false
	}
	// passToSeat1 passes seat 0's priority so the opponent holds it.
	passToSeat1 := func(t *testing.T, e *Engine) {
		t.Helper()
		trigCPass(t, e)
		if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != 1 {
			t.Fatalf("expected seat 1's priority ask, got %+v", d)
		}
		e.emit(events.Event{Kind: events.ManaAdd, Player: 1, Counter: "G", Amount: 1})
		e.pending = nil
		e.priorityRound()
	}
	t.Run("lights up after an opponent's destroy, cast makes four Snakes", func(t *testing.T) {
		t.Parallel()
		e, _, bf1, hand1 := trigCSetup(t, 9415, []string{"Disenchant"}, nil, []string{"Sol Ring"}, []string{"Cobra Trap"})
		addMana(t, e, 0, "1W")
		trigCCast(t, e, "Disenchant", bf1[0], nil)
		if !trigCInZone(e, bf1[0], state.ZGraveyard) {
			t.Fatalf("precondition: Sol Ring survived Disenchant")
		}
		passToSeat1(t, e)
		if !hasAlt(e) {
			t.Fatalf("Cobra Trap's alternative cost not offered after the destroy: %v", trigCCastOptions(e))
		}
		d := e.Pending()
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Label == "Cast Cobra Trap (alternative cost)" {
				submitChoices(t, e, o.Index)
				break
			}
		}
		for i := 0; i < 12 && len(e.G.Stack) > 0; i++ {
			trigCPass(t, e)
		}
		snakes := 0
		var names []string
		for _, id := range e.G.Zone(state.ZBattlefield, 1) {
			if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil {
				names = append(names, o.Face().Name)
				if strings.Contains(o.Face().Name, "Snake") {
					snakes++
				}
			}
		}
		if snakes != 4 {
			t.Fatalf("Cobra Trap made %d Snakes, want 4 (trap in zone %v, tokens %v)", snakes, e.G.Obj(hand1[0]).Zone, names)
		}
	})
	t.Run("not lit before any destroy", func(t *testing.T) {
		t.Parallel()
		e, _, _, _ := trigCSetup(t, 9416, []string{"Disenchant"}, nil, []string{"Sol Ring"}, []string{"Cobra Trap"})
		addMana(t, e, 0, "1W")
		passToSeat1(t, e)
		if hasAlt(e) {
			t.Fatalf("Cobra Trap's alternative cost offered with nothing destroyed")
		}
	})
	t.Run("not lit by a creature's death", func(t *testing.T) {
		t.Parallel()
		e, _, bf1, _ := trigCSetup(t, 9417, []string{"Terror"}, nil, []string{"Grizzly Bears"}, []string{"Cobra Trap"})
		addMana(t, e, 0, "1B")
		trigCCast(t, e, "Terror", bf1[0], nil)
		if !trigCInZone(e, bf1[0], state.ZGraveyard) {
			t.Fatalf("precondition: Terror did not destroy the Bears")
		}
		passToSeat1(t, e)
		if hasAlt(e) {
			t.Fatalf("Cobra Trap lit by a creature's destruction")
		}
	})
	t.Run("not lit by its own controller's destroy", func(t *testing.T) {
		t.Parallel()
		// Seat 0 holds the Trap and destroys its OWN Sol Ring: "a spell or
		// ability an opponent controlled" excludes it.
		e, bf0, _, _ := trigCSetup(t, 9419, []string{"Disenchant", "Cobra Trap"}, []string{"Sol Ring"}, nil, nil)
		addMana(t, e, 0, "1W")
		trigCCast(t, e, "Disenchant", bf0[0], nil)
		if !trigCInZone(e, bf0[0], state.ZGraveyard) {
			t.Fatalf("precondition: Sol Ring survived Disenchant")
		}
		addMana(t, e, 0, "G")
		if hasAlt(e) {
			t.Fatalf("Cobra Trap lit by its own controller's destroy: %v", trigCCastOptions(e))
		}
	})
	t.Run("the tracker resets with the next turn", func(t *testing.T) {
		t.Parallel()
		e, _, bf1, _ := trigCSetup(t, 9418, []string{"Disenchant"}, nil, []string{"Sol Ring"}, []string{"Cobra Trap"})
		addMana(t, e, 0, "1W")
		trigCCast(t, e, "Disenchant", bf1[0], nil)
		passToSeat1(t, e)
		if !hasAlt(e) {
			t.Fatalf("precondition: Cobra Trap not lit this turn: %v", trigCCastOptions(e))
		}
		driveToStepAll(t, e, e.G.Turn+1, 1, state.StepMain1)
		e.emit(events.Event{Kind: events.ManaAdd, Player: 1, Counter: "G", Amount: 1})
		e.pending = nil
		e.priorityRound()
		if hasAlt(e) {
			t.Fatalf("Cobra Trap still lit on the next turn: %v", trigCCastOptions(e))
		}
	})
}
