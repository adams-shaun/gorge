package rules

// CR 727 / CR 104.6: Karn Liberated's [-14] -- "Restart the game, leaving in
// exile all non-Aura permanent cards exiled with Karn Liberated. Then put
// those cards onto the battlefield under your control." A restarted game
// immediately ends with no winner, loser or draw (CR 727.1); its players
// start a new game (rule 103) with every card they own except the exempted
// ones (CR 727.2, 727.5), and the restarting player takes the first turn
// (CR 727.1a).

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// karnRestartBoard seats Karn Liberated on seat 0's battlefield with 20
// loyalty, exiles seat 1's top Mountain with Karn's [-3] (so it is "exiled
// with Karn"), moves to seat 0's next turn, damages seat 1 to 13 life, and
// activates and resolves Karn's [-14]. It returns the engine, the config,
// Karn's old id and the exiled Mountain's id.
func karnRestartBoard(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	e, cfg, ids := edrBoard(t, testutil.CorpusRegistry(t), 727, map[string]state.Zone{
		"Karn Liberated": state.ZBattlefield})
	karn := ids["Karn Liberated"]
	e.emit(events.Event{Kind: events.CounterChange, Obj: karn, Counter: "LOYALTY", Amount: 20 - e.G.Obj(karn).Counter("LOYALTY")})
	mountain := moveByName(t, e, 1, "Mountain", state.ZBattlefield)
	e.priorityRound()
	submitChoices(t, e, abilityOption(t, e, karn, 1).Index) // [-3]: exile target permanent
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("CR 727 setup seq %d: decision %+v, want Karn's [-3] target ask", len(e.L.Events), d)
	}
	pick := -1
	for i, o := range d.Options {
		if o.Obj == mountain {
			pick = i
		}
	}
	if pick < 0 {
		t.Fatalf("CR 727 setup seq %d: seat 1's Mountain is not a [-3] target", len(e.L.Events))
	}
	submitChoices(t, e, d.Options[pick].Index)
	passToResolution(t, e)
	if z := e.G.Obj(mountain).Zone; z != state.ZExile {
		t.Fatalf("CR 727 setup seq %d: Mountain zone = %s after [-3], want Exile", len(e.L.Events), z)
	}
	// Seat 0's next turn, so Karn may activate a loyalty ability again.
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -7})
	e.priorityRound()
	submitChoices(t, e, abilityOption(t, e, karn, 2).Index) // [-14]: restart
	passToResolution(t, e)
	return e, cfg, karn, mountain
}

// TestCR727KarnRestartsTheGame: the [-14] restarts the game instead of
// ending it. The new game has no GameOver, starts on turn 1 with seat 0
// (Karn's controller) active, both players at 20 life with seven-card hands,
// every card seat 0 owns -- Karn included, as a new object -- back in its
// library or hand, and the Mountain exiled with Karn still in exile.
func TestCR727KarnRestartsTheGame(t *testing.T) {
	t.Parallel()
	e, cfg, karn, mountain := karnRestartBoard(t)
	for _, ev := range e.L.Events {
		if ev.Kind == events.GameOver {
			t.Fatalf("CR 727.1 seq %d: the restart emitted GameOver (%q); a restarted game has no winner, loser or draw", len(e.L.Events), ev.Text)
		}
	}
	if e.G.Over {
		t.Fatalf("CR 727.1 seq %d: game over after the restart", len(e.L.Events))
	}
	if e.G.Turn != 1 || e.G.Active != 0 || e.G.StartingPlayer != 0 {
		t.Errorf("CR 727.1a seq %d: turn %d active %d starting %d, want turn 1 with seat 0 (Karn's controller) active and starting", len(e.L.Events), e.G.Turn, e.G.Active, e.G.StartingPlayer)
	}
	for p := state.PlayerID(0); p < 2; p++ {
		if got := e.G.Players[p].Life; got != 20 {
			t.Errorf("CR 727.1 seq %d: seat %d life = %d, want 20 (rule 103's starting life)", len(e.L.Events), p, got)
		}
		if got := len(e.G.Zone(state.ZHand, p)); got != 7 {
			t.Errorf("CR 727.1 seq %d: seat %d hand = %d, want 7", len(e.L.Events), p, got)
		}
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if id != mountain {
				t.Errorf("CR 727.1 seq %d: seat %d battlefield holds %d, want nothing but the exempted Mountain before any play", len(e.L.Events), p, id)
			}
		}
	}
	// Seat 0 owns 40 cards (Karn and 39 Mountains); all 40 are in the new
	// game's library and hand. Seat 1 owns 40 Mountains; the exiled one is
	// exempt (CR 727.5), so 39.
	if got := len(e.G.Zone(state.ZLibrary, 0)) + len(e.G.Zone(state.ZHand, 0)); got != 40 {
		t.Errorf("CR 727.2 seq %d: seat 0 library+hand = %d, want 40", len(e.L.Events), got)
	}
	if got := len(e.G.Zone(state.ZLibrary, 1)) + len(e.G.Zone(state.ZHand, 1)); got != 39 {
		t.Errorf("CR 727.2 seq %d: seat 1 library+hand = %d, want 39 (the Mountain exiled with Karn is exempt)", len(e.L.Events), got)
	}
	if z := e.G.Obj(karn).Zone; z != state.ZCeased {
		t.Errorf("CR 400.7 seq %d: Karn's old object zone = %s, want Ceased (the card is a new object in the new game)", len(e.L.Events), z)
	}
	// The exempted Mountain keeps its object (it is not dealt into the new
	// game): in exile, or on the battlefield once Karn's remaining
	// instruction has run (CR 727.4).
	if o := e.G.Obj(mountain); (o.Zone != state.ZExile && o.Zone != state.ZBattlefield) || o.Owner != 1 {
		t.Errorf("CR 727.5 seq %d: exempted Mountain zone %s owner %d, want Exile or Battlefield, owned by seat 1", len(e.L.Events), o.Zone, o.Owner)
	}
	replayCheck(t, e, cfg)
}

// restartKeptFields are the Engine fields resetForRestart carries into the
// new game (engine/rules/restart.go); every other field, embedded structs
// walked, must be its zero value right after the reset.
var restartKeptFields = map[string]bool{
	"G": true, "L": true, "rng": true, "deckManifests": true, "compiledText": true, "landTypeWords": true,
	"ManaAbilityHook": true, "format": true, "mulligans": true, "windowDiagnostics": true, "paidManaCensus": true,
	"startingLife": true, "setNameInPool": true, "layer4InPool": true, "controlStaticInPool": true,
	"skipPass": true, "loop": true, "tapeHeldHook": true, "tapeHeldStats": true, "PaymentStats": true,
	"decArena": true, "turnsTaken": true, "manaExpended": true, "manaExpendedTurn": true, "trigGrant": true,
	"staticEpoch": true, "activeEpoch": true,
	// Seeded by resetForRestart at the current log length, so the new game's
	// per-turn ledgers ignore the pre-restart log.
	"turnLedgerBase": true, "turnsTakenEpoch": true, "turnStartTurns": true, "turnStartEpoch": true,
}

// TestCR727RestartResetsTurnLedgers: the log is carried across the restart,
// but the per-turn ledgers must count only the new game. The old game reached
// turn 4; the new game is on turn 1, seat 0 active. A ledger that folds the
// whole log reports the old turns too, and turnStartsFor's unsorted result
// breaks the delayed next-turn binary search.
func TestCR727RestartResetsTurnLedgers(t *testing.T) {
	t.Parallel()
	e, _, _, _ := karnRestartBoard(t)
	if e.G.Turn != 1 || e.G.Active != 0 {
		t.Fatalf("CR 727 ledger precondition: turn %d active %d, want turn 1 seat 0", e.G.Turn, e.G.Active)
	}
	if got := e.TurnsTaken(0); got != 1 {
		t.Errorf("TurnsTaken(0) = %d, want 1 (the new game's turn 1 only)", got)
	}
	if got := e.TurnsTaken(1); got != 0 {
		t.Errorf("TurnsTaken(1) = %d, want 0 (seat 1 has not taken a turn in the new game)", got)
	}
	if got := e.turnStartsFor(0); len(got) != 1 || got[0] != 1 {
		t.Errorf("turnStartsFor(0) = %v, want [1] (the new game's turn 1 only)", got)
	}
	if got := e.turnStartsFor(1); len(got) != 0 {
		t.Errorf("turnStartsFor(1) = %v, want [] (seat 1 has not started a turn in the new game)", got)
	}
}

// TestCR727TerminalRestartIsADraw: a restart whose keep set leaves every seat
// with fewer than seven library cards reaches CR 104.4a's draw, exactly as
// genesis's all-undersized opening deal does, rather than crowning the first
// seat to be dealt.
func TestCR727TerminalRestartIsADraw(t *testing.T) {
	t.Parallel()
	e, cfg, _ := edrBoard(t, testutil.CorpusRegistry(t), 7271, nil)
	var keep []state.ObjID
	for p := state.PlayerID(0); p < 2; p++ {
		lib := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, p)...)
		for _, id := range lib[5:] {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZExile})
			keep = append(keep, id)
		}
		// The opening hands are exempted too, so they stay in hand instead of
		// refilling the new game's library.
		keep = append(keep, e.G.Zone(state.ZHand, p)...)
		if got := len(e.G.Zone(state.ZLibrary, p)); got != 5 {
			t.Fatalf("terminal restart precondition: seat %d library = %d, want 5", p, got)
		}
	}
	e.restart = &restartRequest{player: 0, keep: keep}
	e.finishRestart()
	if !e.G.Over || !e.G.Draw || e.G.Winner != 0 {
		t.Fatalf("terminal restart: over %v draw %v winner %d, want CR 104.4a's draw", e.G.Over, e.G.Draw, e.G.Winner)
	}
	replayCheck(t, e, cfg)
}

// TestCR727ResetForRestartKeepsOnlyConfig pins resetForRestart's kept set on
// a mid-game engine: a new Engine field is per-game (reset) by default, and
// one that must survive a restart is added to resetForRestart and to
// restartKeptFields together.
func TestCR727ResetForRestartKeepsOnlyConfig(t *testing.T) {
	t.Parallel()
	e, _, _, _ := karnRestartBoard(t)
	e.resetForRestart()
	var walk func(v reflect.Value, path string)
	walk = func(v reflect.Value, path string) {
		ty := v.Type()
		for i := 0; i < ty.NumField(); i++ {
			f, fv := ty.Field(i), v.Field(i)
			if f.Anonymous && fv.Kind() == reflect.Struct {
				walk(fv, path+f.Name+".")
				continue
			}
			if restartKeptFields[f.Name] {
				continue
			}
			if !fv.IsZero() {
				t.Errorf("CR 727 resetForRestart: Engine field %s%s is not reset", path, f.Name)
			}
		}
	}
	walk(reflect.ValueOf(e).Elem(), "")
}

// TestCR727KarnReturnsTheExiledCardsJustBeforeTheFirstUntap: CR 727.4 --
// the restarting effect finishes resolving just before the new game's first
// untap step, so Karn's "Then put those cards onto the battlefield under your
// control" puts seat 1's Mountain onto the battlefield under seat 0's
// control, owned by seat 1, after turn 1's TurnChange and before its untap.
func TestCR727KarnReturnsTheExiledCardsJustBeforeTheFirstUntap(t *testing.T) {
	t.Parallel()
	e, cfg, _, mountain := karnRestartBoard(t)
	o := e.G.Obj(mountain)
	if o.Zone != state.ZBattlefield || o.Controller != 0 || o.Owner != 1 {
		t.Fatalf("CR 727.4 seq %d: Mountain zone %s controller %d owner %d, want Battlefield under seat 0, owned by seat 1", len(e.L.Events), o.Zone, o.Controller, o.Owner)
	}
	turn, untap, entered := -1, -1, -1
	restart := -1
	for i, ev := range e.L.Events {
		switch {
		case ev.Kind == events.GameRestart:
			restart = i
		case restart >= 0 && ev.Kind == events.TurnChange && turn < 0:
			turn = i
		case turn >= 0 && ev.Kind == events.MoveZone && ev.Obj == mountain && ev.To == state.ZBattlefield && entered < 0:
			entered = i
		case turn >= 0 && ev.Kind == events.StepChange && ev.Step == state.StepUntap && untap < 0:
			untap = i
		}
	}
	if !(restart >= 0 && turn > restart && entered > turn && untap > entered) {
		t.Errorf("CR 727.4 seq %d: GameRestart %d, turn 1 TurnChange %d, Mountain entry %d, untap StepChange %d; want them in that order", len(e.L.Events), restart, turn, entered, untap)
	}
	replayCheck(t, e, cfg)
}

// TestChangeZoneAllPermanentOffTheBattlefieldIsAPermanentCard: a
// ChangeZoneAll whose Origin$ excludes the battlefield reads a leading
// Permanent base as "permanent card" (Forge's CardType reading, the Dig and
// search permanentCardSpec rule), so it moves Grizzly Bears (a creature card)
// from the graveyard and leaves Lightning Bolt (an instant card). Karn's
// "non-Aura permanent cards exiled with Karn" and 12 other corpus lines use it.
func TestChangeZoneAllPermanentOffTheBattlefieldIsAPermanentCard(t *testing.T) {
	t.Parallel()
	e, cfg, ids := edrBoard(t, testutil.CorpusRegistry(t), 7271, map[string]state.Zone{
		"Grizzly Bears": state.ZGraveyard, "Lightning Bolt": state.ZGraveyard})
	bears, bolt := ids["Grizzly Bears"], ids["Lightning Bolt"]
	sweep := card(t, "Name:Permanent Sweep\nManaCost:0\nTypes:Sorcery\nA:SP$ ChangeZoneAll | ChangeType$ Permanent | Origin$ Graveyard | Destination$ Hand | SpellDescription$ x\nOracle:x\n")
	effects.Resolve(e, effects.NewCtxPtr(0, 0, effects.CtxInit{}), sweep.Faces[0].Abilities[0])
	if z := e.G.Obj(bears).Zone; z != state.ZHand {
		t.Errorf("ChangeZoneAll Permanent from Graveyard seq %d: Grizzly Bears zone = %s, want Hand", len(e.L.Events), z)
	}
	if z := e.G.Obj(bolt).Zone; z != state.ZGraveyard {
		t.Errorf("ChangeZoneAll Permanent from Graveyard seq %d: Lightning Bolt zone = %s, want Graveyard (an instant is no permanent card)", len(e.L.Events), z)
	}
	replayCheck(t, e, cfg)
}
