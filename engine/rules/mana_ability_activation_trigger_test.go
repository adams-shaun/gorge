package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A mana ability is an activated ability (CR 605.1a), so Mode$ AbilityCast /
// SpellAbilityCast must see its activation even though it mints no stack
// object. These tests drive a real "activate" option for a mana rock owned by
// seat 0 while seat 1 controls the watcher.

const manaWatchRockSrc = "Name:Test Rock\nManaCost:1\nTypes:Artifact\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C | Amount$ 2 | SpellDescription$ Rock mana.\nOracle:x\n"

// manaWatchCarrier is an opposing creature watching opponents' activations,
// narrowed by validSA ("" = the un-narrowed carrier).
func manaWatchCarrier(name, mode, validSA string) string {
	narrow := ""
	if validSA != "" {
		narrow = " | ValidSA$ " + validSA
	}
	return "Name:" + name + "\nManaCost:1\nTypes:Creature Test\nPT:1/1\n" +
		"T:Mode$ " + mode + " | ValidActivatingPlayer$ Opponent" + narrow +
		" | TriggerZones$ Battlefield | Execute$ TrigLose | TriggerDescription$ x\n" +
		"SVar:TrigLose:DB$ LoseLife | Defined$ TriggeredActivator | LifeAmount$ 3\nOracle:x\n"
}

func countMarkerKind(e *Engine, k events.Kind) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == k {
			n++
		}
	}
	return n
}

// activateManaSource taps src for mana through its offered activate option,
// then lets every resulting trigger resolve. It returns seat 0's life lost.
func activateManaSource(t *testing.T, e *Engine, src state.ObjID) int32 {
	t.Helper()
	addMana(t, e, 0, "") // re-ask priority so the offer reflects the moved permanents
	opt, ok := gainedManaOption(e, src)
	if !ok {
		t.Fatalf("precondition: no activate option for %d: %+v", src, e.Pending())
	}
	before := e.G.Players[0].Life
	submitChoices(t, e, opt.Index)
	if got := e.G.Players[0].Pool[state.MC]; got < 2 {
		t.Fatalf("precondition: the activation produced %d colourless, want >= 2", got)
	}
	resolveStack(t, e)
	return before - e.G.Players[0].Life
}

func manaWatchGame(t *testing.T, seed uint64, carrierSrc string) (*Engine, Config, state.ObjID) {
	t.Helper()
	rock := card(t, manaWatchRockSrc)
	seat1 := []*cards.Card(nil)
	if carrierSrc != "" {
		seat1 = []*cards.Card{card(t, carrierSrc)}
	}
	e, cfg := tokenReplGameSeats(t, seed, []*cards.Card{rock}, seat1)
	id := moveSeededCard(t, e, 0, rock, state.ZBattlefield)
	if carrierSrc != "" {
		moveSeededCard(t, e, 1, card(t, carrierSrc), state.ZBattlefield)
	}
	return e, cfg, id
}

// TestManaAbilityActivationTrigger: the un-narrowed carriers of both modes see
// the activation; the marker carries the source, the activator and the flat
// pile index of the ability, and the log replays.
func TestManaAbilityActivationTrigger(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"AbilityCast", "SpellAbilityCast"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			e, cfg, rock := manaWatchGame(t, 9301, manaWatchCarrier("Watcher", mode, ""))
			if n := countMarkerKind(e, events.ManaAbilityActivation); n != 0 {
				t.Fatalf("precondition: %d activation markers before any activation", n)
			}
			if lost := activateManaSource(t, e, rock); lost != 3 {
				t.Fatalf("life lost to the %s watcher = %d, want 3", mode, lost)
			}
			var marks []events.Event
			for _, ev := range e.L.Events {
				if ev.Kind == events.ManaAbilityActivation {
					marks = append(marks, ev)
				}
			}
			if len(marks) != 1 || marks[0].Obj != rock || marks[0].Player != 0 || marks[0].Amount != 0 || len(marks[0].IDs) != 0 {
				t.Fatalf("activation markers = %+v, want one {Obj %d Player 0 Amount 0}", marks, rock)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestAbilityCastManaAbilityNarrowing: ValidSA$ narrowing applies to the mana
// ability's own body. Each case is a fresh game with one opposing carrier.
func TestAbilityCastManaAbilityNarrowing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, validSA string
		lose          int32
	}{
		{"unnarrowed", "", 3},
		{"activated", "Activated", 3},
		{"mana_ability_only", "Activated.ManaAbility", 3},
		{"not_mana_ability", "Activated.!ManaAbility", 0},
		{"not_tap_cost", "Activated.!hasTapCost", 0},
		{"spell_only", "Spell", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, rock := manaWatchGame(t, 9302, manaWatchCarrier("Watcher", "AbilityCast", tc.validSA))
			if lost := activateManaSource(t, e, rock); lost != tc.lose {
				t.Fatalf("ValidSA$ %q: life lost = %d, want %d", tc.validSA, lost, tc.lose)
			}
		})
	}
}

// TestKaiWinnNamedManaSource: with the chosen name on Sol Ring, an opponent
// tapping Sol Ring for mana loses 3 life; a different chosen name does not.
func TestKaiWinnNamedManaSource(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	kai, ok := reg.Lookup("Kai Winn, Faithless Leader")
	if !ok {
		t.Fatal("corpus fixture: Kai Winn, Faithless Leader missing")
	}
	sol, ok := reg.Lookup("Sol Ring")
	if !ok {
		t.Fatal("corpus fixture: Sol Ring missing")
	}
	for _, tc := range []struct {
		chosen string
		lose   int32
	}{{"Sol Ring", 3}, {"Mox Pearl", 0}} {
		t.Run(tc.chosen, func(t *testing.T) {
			t.Parallel()
			e, cfg := tokenReplGameSeats(t, 9303, []*cards.Card{sol}, []*cards.Card{kai})
			ring := moveSeededCard(t, e, 0, sol, state.ZBattlefield)
			// Kai Winn is moved by hand, keeping the as-enters name ask that
			// moveSeededCard would discard.
			toMain1(t, e)
			var k state.ObjID
			for _, id := range e.G.Zone(state.ZLibrary, 1) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == kai.Faces[0].Name {
					k = id
					break
				}
			}
			if k == 0 {
				t.Fatal("precondition: Kai Winn not in seat 1's library")
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: k, From: state.ZLibrary, To: state.ZBattlefield})
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
				t.Fatalf("precondition: Kai Winn's name ask not pending: %+v", d)
			}
			idx := -1
			for _, o := range d.Options {
				if o.Label == "Sol Ring" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("precondition: Sol Ring not offered as a name: %+v", d.Options)
			}
			submitChoices(t, e, idx)
			if tc.chosen != "Sol Ring" {
				e.emit(events.Event{Kind: events.Choose, Obj: k, Counter: "name", Text: tc.chosen})
			}
			resolveStack(t, e)
			if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
				t.Fatalf("precondition: not back at priority: %+v", d)
			}
			if got := e.G.Obj(k).ChosenName; got != tc.chosen {
				t.Fatalf("precondition: Kai Winn's chosen name = %q, want %q", got, tc.chosen)
			}
			if lost := activateManaSource(t, e, ring); lost != tc.lose {
				t.Fatalf("chosen %q: Sol Ring tap cost the opponent %d life, want %d", tc.chosen, lost, tc.lose)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestManaAbilityActivationNoCarrier: with no watcher in play the activation
// is byte-for-byte what it always was -- no marker in the log. A trigger of
// another mode is not a watcher either.
func TestManaAbilityActivationNoCarrier(t *testing.T) {
	t.Parallel()
	other := "Name:Other\nManaCost:1\nTypes:Creature Test\nPT:1/1\n" +
		"T:Mode$ SpellCast | ValidActivatingPlayer$ Opponent | TriggerZones$ Battlefield | Execute$ TrigLose | TriggerDescription$ x\n" +
		"SVar:TrigLose:DB$ LoseLife | Defined$ TriggeredActivator | LifeAmount$ 3\nOracle:x\n"
	for name, src := range map[string]string{"empty": "", "other_mode": other} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, _, rock := manaWatchGame(t, 9304, src)
			if lost := activateManaSource(t, e, rock); lost != 0 {
				t.Fatalf("life lost = %d, want 0", lost)
			}
			if n := countMarkerKind(e, events.ManaAbilityActivation); n != 0 {
				t.Fatalf("%d activation markers in a game with no activation watcher", n)
			}
		})
	}
}

// TestManaAbilityActivationGainedShape: a GAINED mana ability (Forge's
// GainsAbilitiesOf$) announces itself in the GainedAbilityPush shape -- the
// source in Obj, the foreign card in IDs[0], its face-local index in Amount --
// and the body resolves through that foreign card.
func TestManaAbilityActivationGainedShape(t *testing.T) {
	t.Parallel()
	warden := gainsMairsilFixtureSrc(t)
	caged := gainsCagedManaSrc(t)
	e, cfg := tokenReplGameSeats(t, 9114, []*cards.Card{warden, caged},
		[]*cards.Card{card(t, manaWatchCarrier("Watcher", "AbilityCast", "Activated.ManaAbility"))})
	wardenID := moveSeededCard(t, e, 0, warden, state.ZBattlefield)
	cagedID := moveSeededCard(t, e, 0, caged, state.ZBattlefield)
	moveSeededCard(t, e, 1, card(t, manaWatchCarrier("Watcher", "AbilityCast", "Activated.ManaAbility")), state.ZBattlefield)
	e.emit(events.Event{Kind: events.MoveZone, Obj: cagedID, From: state.ZBattlefield,
		To: state.ZExile, IDs: []state.ObjID{wardenID}})
	e.emit(events.Event{Kind: events.CounterChange, Obj: cagedID, Counter: "CAGE", Amount: 1})
	e.pending = nil
	addMana(t, e, 0, "C")
	e.priorityRound()
	opt, ok := gainedManaOption(e, wardenID)
	if !ok {
		t.Fatalf("precondition: Warden offers no gained mana ability: %+v", e.Pending())
	}
	before := e.G.Players[0].Life
	submitChoices(t, e, opt.Index)
	resolveStack(t, e)
	var marks []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.ManaAbilityActivation {
			marks = append(marks, ev)
		}
	}
	if len(marks) != 1 || marks[0].Obj != wardenID || len(marks[0].IDs) != 1 || marks[0].IDs[0] != cagedID || marks[0].Amount != 0 {
		t.Fatalf("gained activation markers = %+v, want one {Obj %d IDs [%d] Amount 0}", marks, wardenID, cagedID)
	}
	if lost := before - e.G.Players[0].Life; lost != 3 {
		t.Fatalf("life lost to the ManaAbility-narrowed watcher = %d, want 3 (body resolved through the foreign card)", lost)
	}
	replayCheck(t, e, cfg)
}
