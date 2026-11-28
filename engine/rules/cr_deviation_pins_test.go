package rules

// CR deviation pins: each test asserts what the engine does today where it
// departs from the rule its crcoverage row names, so fixing the deviation
// fails here and sends the author back to that row.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCRDeviation103_7 pins the Planechase start.
// CR 103.7: only the starting player turns a plane face up, and a phenomenon
// turned up goes to the bottom until a plane is revealed.
// The engine instead reveals the top card of every seat's planar deck, a
// phenomenon included.
func TestCRDeviation103_7(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	plane := searchCorpusCard(t, reg, "Aretopolis")
	phenomenon := card(t, "Name:Probe Phenomenon\nTypes:Phenomenon\nOracle:x\n")
	cfg := seatZeroStart(Config{Seed: 1903, Names: []string{"A", "B"},
		Decks:       [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)},
		PlanarDecks: [][]*cards.Card{{plane}, {phenomenon}}})
	e := New(cfg)
	if e.G.Active != 0 {
		t.Fatalf("precondition: seat 0 must start, got seat %d", e.G.Active)
	}
	revealed := map[state.PlayerID]state.ObjID{}
	for _, ev := range e.L.Events {
		if ev.Kind == events.PlanarReveal {
			revealed[ev.Player] = ev.Obj
		}
	}
	if len(revealed) != 2 {
		t.Fatalf("planar reveals by seat = %v, want both seats (the deviation)", revealed)
	}
	if o := e.G.Obj(revealed[1]); o == nil || o.Face() == nil || o.Face().Name != "Probe Phenomenon" {
		t.Fatalf("seat 1 revealed %+v, want the phenomenon left face up (the deviation)", o)
	}
}

// TestCRDeviation104_4b pins a mandatory loop.
// CR 104.4b: a loop of mandatory actions with no way to stop is a draw.
// The engine instead aborts with a LivelockError and records no game result.
func TestCRDeviation104_4b(t *testing.T) {
	t.Parallel()
	e := livelockTestEngine(t, &LoopGuard{CycleEvents: 30, MaxPeriod: 8})
	if e.G.Over {
		t.Fatal("precondition: the game is already over")
	}
	lle := drivePattern(t, e, func(int) events.Event {
		return events.Event{Kind: events.Note, Obj: 42, Text: "loop"}
	}, 1000)
	if lle == nil {
		t.Fatal("the mandatory loop did not abort")
	}
	if e.G.Over {
		t.Fatal("the loop ended the game; the deviation (abort, no draw) is gone: update CR 104.4b's row")
	}
}

// TestCRDeviation310_9a pins a battle with no battle type.
// CR 310.9a: only its controller can be the protector of a battle with no
// battle types.
// The engine instead records an opponent as its protector, as for a Siege.
func TestCRDeviation310_9a(t *testing.T) {
	t.Parallel()
	battle := card(t, "Name:Typeless Battle\nManaCost:2 R\nTypes:Battle\nDefense:3\nOracle:x\n")
	if f := battle.Faces[0]; !f.IsBattle() || len(f.Types) != 1 {
		t.Fatalf("precondition: fixture is not a battle with no battle type: %v", f.Types)
	}
	// No Advance: the entry replacement poses no protector ask over a
	// pending decision, so the battle enters before the first priority.
	e := New(seatZeroStart(Config{Seed: 3109, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{append([]*cards.Card{battle}, mountainDeck(t, 39)...), mountainDeck(t, 40)}}))
	id := moveByName(t, e, 0, "Typeless Battle", state.ZBattlefield)
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || !o.ProtectorValid {
		t.Fatalf("battle did not enter with a recorded protector: zone %v valid %v", o.Zone, o.ProtectorValid)
	}
	if o.Protector != 1 {
		t.Fatalf("protector = seat %d, want seat 1 (the deviation; the rule names seat 0)", o.Protector)
	}
}

// TestCRDeviation506_8f pins the after-blockers activation window.
// CR 506.8f (the row's id): "after blockers are declared" needs a
// declare-blockers step, so a combat that skipped it never opens the window.
// The engine instead offers an ActivationAfterBlockers$ ability in the end of
// combat step of a combat with no attackers.
func TestCRDeviation506_8f(t *testing.T) {
	t.Parallel()
	const src = "Name:After Blockers Probe\nManaCost:1 W\nTypes:Creature Human\nPT:2/3\n" +
		"A:AB$ Pump | Cost$ 0 | ValidTgts$ Creature | ActivationPhases$ BeginCombat->EndCombat | ActivationAfterBlockers$ True | NumAtt$ +1 | SpellDescription$ probe\n" +
		"Oracle:x\n"
	e, cfg, id := newFixtureDeck(t, 5068, src)
	moveByName(t, e, 0, "After Blockers Probe", state.ZBattlefield)
	start := len(e.L.Events)
	driveToStepAll(t, e, e.G.Turn, 0, state.StepBeginCombat)
	if abilityFor(t, e, 0, id) != nil {
		t.Fatal("precondition: the rider did not withhold the probe at beginning of combat")
	}
	driveToStepAll(t, e, e.G.Turn, 0, state.StepEndCombat)
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.DeclareBlockers {
			t.Fatal("precondition: blockers were declared this turn")
		}
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the probe is not on the battlefield: %+v", o)
	}
	if abilityFor(t, e, 0, id) == nil {
		t.Fatal("the probe was withheld after a skipped declare-blockers step; the deviation is gone: update CR 506.8f's row")
	}
	replayCheck(t, e, cfg)
}

// crpinAKharnScenario: p1's Threaten takes p0's Khârn the Betrayer, whose
// ChangesController line ("when you lose control of ~, draw two cards")
// triggers.
const crpinAKharnScenario = `{"name":"cr-603-10d-kharn","cr":["603.10d"],"why":"Threaten takes Kharn; who controls the lose-control trigger",
 "setup":{"p0":{"battlefield":["Khârn the Betrayer"]},"p1":{"hand":["Threaten"]}},
 "steps":[{"op":"pass_to","step":"main1","active":"p1"},{"op":"cast","seat":1,"card":"p1:Threaten","mana":"CCR","targets":["p0:Khârn the Betrayer"]},{"op":"pass","seat":1},{"op":"pass","seat":0}]}`

// TestCRDeviation603_10d pins a lose-control trigger.
// CR 603.10d: "when a player loses control of an object" looks back in time,
// so the player who lost control controls the trigger.
// The engine instead gives the trigger to the player who gained control.
func TestCRDeviation603_10d(t *testing.T) {
	t.Parallel()
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(crpinAKharnScenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("precondition: scenario did not play cleanly: %v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	taken := false
	for _, p := range last.Permanents {
		taken = taken || (p.Controller == 1 && p.Owner == 0 && p.Name == "Khârn the Betrayer")
	}
	if !taken {
		t.Fatalf("precondition: p1 never gained Khârn: %+v", last.Permanents)
	}
	var ctrl []int
	for _, s := range res.Snapshots {
		for _, st := range s.Stack {
			if st.Kind == "ability" && st.Trigger == "1" {
				ctrl = append(ctrl, st.Controller)
			}
		}
	}
	if len(ctrl) == 0 {
		t.Fatal("precondition: Khârn's ChangesController trigger never reached the stack")
	}
	if ctrl[0] != 1 {
		t.Fatalf("lose-control trigger controlled by seat %d, want seat 1 (the deviation): update CR 603.10d's row", ctrl[0])
	}
}

// TestCRDeviation607_5a pins a stale choice surviving a later copy effect.
// CR 607.5a: an object that becomes a copy of a "choose a color" permanent
// after it entered made no choice for the copied ability, so the chosen color
// is undefined and "protection from the chosen color" does nothing.
// The engine keeps the object's own earlier ChosenColor across the copy, so
// the copied protection reads it.
func TestCRDeviation607_5a(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	shifter := onBoard(t, e, 0, "Name:Shifter\nTypes:Creature Shapeshifter\nPT:0/1\nOracle:x\n")
	voice := onBoard(t, e, 1, "Name:Voice\nTypes:Creature Angel\nPT:2/2\n"+
		"K:ETBReplacement:Other:PickHue\nSVar:PickHue:DB$ ChooseColor | Defined$ You\n"+
		"S:Mode$ Continuous | AffectedDefined$ Self | AddKeyword$ Protection:Card.ChosenColor:chosenColor\nOracle:x\n")
	red := onBoard(t, e, 1, "Name:Red Source\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")
	// The shifter's earlier incarnation answered a choose-a-color prompt.
	e.emit(events.Event{Kind: events.Choose, Obj: shifter, Counter: "color", Text: "Red"})
	if e.protectedFrom(shifter, red) {
		t.Fatal("precondition: the plain shifter already has protection from red")
	}
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: shifter, IDs: []state.ObjID{voice}})
	if f := e.G.Obj(shifter).Face(); f == nil || f.Name != "Voice" {
		t.Fatalf("precondition: the shifter did not become a copy of Voice: %+v", f)
	}
	if got := e.G.Obj(shifter).ChosenColor; got != "Red" {
		t.Fatalf("ChosenColor after the copy = %q, want the stale %q", got, "Red")
	}
	if !e.protectedFrom(shifter, red) {
		t.Fatal("the copied protection no longer reads the stale choice: CR 607.5a may be fixed, update the 607.5a row")
	}
}

// TestCRDeviation700_2e pins who chooses the mode of an opponent-chooses charm.
// CR 700.2e: when a spell says another player chooses its mode, that player
// chooses it at the time the caster normally would.
// The engine does not read Chooser$ on api:Charm, so the caster is asked.
func TestCRDeviation700_2e(t *testing.T) {
	t.Parallel()
	lore := "Name:Opponent Lore\nManaCost:B\nTypes:Sorcery\n" +
		"A:SP$ Charm | Chooser$ Opponent | Choices$ DrawOne,GainOne\n" +
		"SVar:DrawOne:DB$ Draw | NumCards$ 1 | Defined$ You | SpellDescription$ You draw a card.\n" +
		"SVar:GainOne:DB$ GainLife | LifeAmount$ 1 | Defined$ You | SpellDescription$ You gain 1 life.\n" +
		"Oracle:x\n"
	e, _, id := newFixtureDeck(t, 7002, lore)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand || o.Controller != 0 {
		t.Fatalf("precondition: Opponent Lore = %+v, want it in seat 0's hand", o)
	}
	addMana(t, e, 0, "B")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes || len(d.Options) != 2 {
		t.Fatalf("pending = %+v, want the two-mode announcement", d)
	}
	if d.Player != 0 {
		t.Fatalf("mode chooser = seat %d: CR 700.2e may be fixed, update the 700.2e row", d.Player)
	}
}

// TestCRDeviation701_31a pins the per-seat planar model.
// CR 701.31a-d: only the planar controller planeswalks, and there is one
// face-up plane in the game.
// The engine gives every seat its own face-up plane and lets any seat walk
// its own planar deck.
func TestCRDeviation701_31a(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	planes := []*cards.Card{searchCorpusCard(t, reg, "Aretopolis"), searchCorpusCard(t, reg, "Pools of Becoming")}
	e := New(Config{Seed: 1901, Names: []string{"A", "B"},
		Decks:       [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)},
		PlanarDecks: [][]*cards.Card{planes, planes}})
	for p := state.PlayerID(0); p < 2; p++ {
		if e.currentPlane(p) == nil {
			t.Fatalf("seat %d has no face-up plane; the engine now models one shared plane (CR 701.31): update the 701.31a-d row", p)
		}
	}
	other := 1 - e.G.Active
	before := e.currentPlane(other).ID
	e.Planeswalk(other)
	if got := e.currentPlane(other); got == nil || got.ID == before {
		t.Fatalf("non-active seat %d could not planeswalk its own deck (plane %v); update the 701.31a-d row", other, got)
	}
}

// TestCRDeviation701_62b pins manifest dread over an empty library.
// CR 701.62b: a "whenever a player manifests dread" ability triggers even if
// some or all of the manifest-dread actions were impossible.
// The engine emits no ManifestDreadAction for a player with an empty
// library, so nothing can trigger.
func TestCRDeviation701_62b(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	src := onBoard(t, e, 0, "Name:Dreader\nTypes:Creature\nPT:1/1\nSVar:MD:DB$ ManifestDread\nOracle:x\n")
	sa := cards.ResolveSVar(e.G.Obj(src).Face().SVars, "MD")
	for len(e.G.Zone(state.ZLibrary, 0)) > 1 {
		e.emit(events.Event{Kind: events.MoveZone, Obj: e.G.Zone(state.ZLibrary, 0)[0], From: state.ZLibrary, To: state.ZExile})
	}
	count := func() int {
		n := 0
		for _, ev := range e.L.Events {
			if ev.Kind == events.ManifestDreadAction && ev.Player == 0 {
				n++
			}
		}
		return n
	}
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0}, sa)
	if count() != 1 || len(e.G.Zone(state.ZLibrary, 0)) != 0 {
		t.Fatalf("one-card library: %d ManifestDreadAction, library %d; want 1 and 0", count(), len(e.G.Zone(state.ZLibrary, 0)))
	}
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0}, sa)
	if count() != 1 {
		t.Fatalf("empty library emitted a ManifestDreadAction (CR 701.62b now followed): update the 701.62b row")
	}
}

// TestCRDeviation701_66a pins who controls a returned earthbent land.
// CR 701.66a: the land returns to the battlefield under the earthbender's
// control.
// The engine returns it under its owner's control.
func TestCRDeviation701_66a(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := searchEngine(t, reg, "Badgermole Cub")
	land := searchMoveByNameSeat(t, e, 1, "Mountain", state.ZBattlefield)
	e.emit(events.Event{Kind: events.ControlChange, Obj: land, Player: 0})
	if o := e.G.Obj(land); o.Owner != 1 || o.Controller != 0 {
		t.Fatalf("stolen land owner/controller = %d/%d, want 1/0", o.Owner, o.Controller)
	}
	searchMoveByName(t, e, "Badgermole Cub", state.ZBattlefield)
	earthbendSettleAnswering(t, e, land, 40)
	if !e.IsCreature(land) {
		t.Fatal("the stolen Mountain was not earthbent")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: land, From: state.ZBattlefield, To: state.ZGraveyard})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	o := e.G.Obj(land)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("earthbent land in %v, want returned to the battlefield", o.Zone)
	}
	if o.Controller != 1 {
		t.Fatalf("returned land controller = %d; it now returns under the earthbender (CR 701.66a): update the 701.66a row", o.Controller)
	}
}

// TestCRDeviation702_2b pins the lifetime of the deathtouch mark.
// CR 702.2b: only deathtouch damage dealt since the last state-based check
// destroys the creature.
// The engine keeps the Deathtouched mark past the check until cleanup, so an
// indestructible creature keeps it.
func TestCRDeviation702_2b(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	src := onBoard(t, e, 0, "Name:Stinger\nTypes:Creature\nPT:1/1\nK:Deathtouch\nSVar:Sting:DB$ DealDamage | ValidTgts$ Creature | NumDmg$ 1\nOracle:x\n")
	wall := onBoard(t, e, 1, "Name:Wall\nTypes:Creature\nPT:0/4\nK:Indestructible\nOracle:x\n")
	sa := cards.ResolveSVar(e.G.Obj(src).Face().SVars, "Sting")
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: wall}}}, sa)
	if o := e.G.Obj(wall); o.Damage != 1 || o.Counter("Deathtouched") != 1 {
		t.Fatalf("deathtouch hit: damage %d mark %d, want 1 and 1", o.Damage, o.Counter("Deathtouched"))
	}
	e.checkStateBased()
	if e.G.Obj(wall).Zone != state.ZBattlefield {
		t.Fatal("indestructible creature left the battlefield")
	}
	if e.G.Obj(wall).Counter("Deathtouched") != 1 {
		t.Fatal("the deathtouch mark no longer outlives the SBA check (CR 702.2b now followed): update the 702.2b row")
	}
}

// TestCRDeviation702_19c pins trample over planeswalkers.
// CR 702.19c,e: excess over the attacked planeswalker's loyalty may go to its
// controller, and with that planeswalker removed from combat the damage goes
// to the defending player.
// The engine reads the keyword as plain trample: all damage stays on the
// planeswalker, and none is dealt once it is gone.
func TestCRDeviation702_19c(t *testing.T) {
	t.Parallel()
	for _, removed := range []bool{false, true} {
		e := combatEngine(t)
		attacker := onBoardReady(t, e, 0, "Name:Stomper\nTypes:Creature\nPT:4/4\nK:Trample:Planeswalker\nOracle:x\n")
		pw := onBoard(t, e, 1, combatPlaneswalker)
		e.emit(events.Event{Kind: events.CounterChange, Obj: pw, Counter: "LOYALTY", Amount: 2})
		if !e.HasKeyword(attacker, "Trample") || e.G.Obj(pw).Counter("LOYALTY") != 2 {
			t.Fatalf("precondition: trample head %v loyalty %d", e.HasKeyword(attacker, "Trample"), e.G.Obj(pw).Counter("LOYALTY"))
		}
		declareAtPlaneswalker(t, e, attacker, pw)
		if removed {
			e.emit(events.Event{Kind: events.MoveZone, Obj: pw, From: state.ZBattlefield, To: state.ZGraveyard})
		}
		life := e.G.Players[1].Life
		e.dealCombatDamage()
		if got := e.G.Players[1].Life; got != life {
			t.Fatalf("removed=%v: defending player took %d (CR 702.19c,e now followed): update the 702.19c,e row", removed, life-got)
		}
		if !removed && e.G.Obj(pw).Counter("LOYALTY") > 0 {
			t.Fatalf("planeswalker kept %d loyalty, want all 4 damage on it", e.G.Obj(pw).Counter("LOYALTY"))
		}
	}
}

// crpinDSetState places a two-faced permanent whose front face carries
// SVar:DBX (a SetState of mode on itself) and returns it with that SA.
func crpinDSetState(t *testing.T, e *Engine, alt, mode string) (state.ObjID, *cards.SA) {
	t.Helper()
	id := onBoard(t, e, 0, "Name:Front\nTypes:Creature\nPT:2/2\nAlternateMode:"+alt+
		"\nSVar:DBX:DB$ SetState | Defined$ Self | Mode$ "+mode+"\nOracle:x\n\nALTERNATE\n\nName:Back\nTypes:Creature\nPT:3/3\nOracle:y\n")
	sa := cards.ResolveSVar(e.G.Obj(id).Face().SVars, "DBX")
	if sa == nil || sa.API != "SetState" {
		t.Fatalf("%s fixture: DBX did not compile to SetState", alt)
	}
	if e.G.Obj(id).FaceIdx != 0 || len(e.G.Obj(id).Card.Faces) != 2 {
		t.Fatalf("%s fixture: want a two-faced permanent on its front face", alt)
	}
	return id, sa
}

// TestFlipCardSecondFlipStaysFlipped pins CR 710.4 (re-audit: the engine now follows it).
// The rule: flipping is one-way, a flipped permanent can't become unflipped.
// The engine: setStateFlipGate leaves an already-flipped permanent as it is.
func TestFlipCardSecondFlipStaysFlipped(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	id, sa := crpinDSetState(t, e, "Flip", "Flip")
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0}, sa)
	if got := e.G.Obj(id).FaceIdx; got != 1 {
		t.Fatalf("first flip: FaceIdx = %d, want 1", got)
	}
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0}, sa)
	if got := e.G.Obj(id).FaceIdx; got != 1 {
		t.Fatalf("second flip: FaceIdx = %d, want 1 (flipping is one-way)", got)
	}
}

// TestCRDeviation712_4c pins CR 712.4c.
// The rule: a meld card can't be transformed; the instruction is ignored.
// The engine: effSetState's transform walk has no meld gate, so a meld card
// on the battlefield transforms onto its result face.
func TestCRDeviation712_4c(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	id, sa := crpinDSetState(t, e, "Meld", "Transform")
	if !state.IsMeldCard(e.G.Obj(id).Card) {
		t.Fatal("fixture is not a meld card")
	}
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0}, sa)
	if got := e.G.Obj(id).FaceIdx; got != 1 {
		t.Fatalf("FaceIdx = %d; the deviation (meld card transforms to face 1) is gone: re-status CR 712.4c", got)
	}
}

// crpinESetState resolves src's SVar Act with the given permanents as targets.
func crpinESetState(t *testing.T, e *Engine, src string, ids ...state.ObjID) {
	t.Helper()
	s := onBoard(t, e, 0, src)
	face := e.G.Obj(s).Face()
	var tgts []state.Target
	for _, id := range ids {
		tgts = append(tgts, state.Target{Obj: id})
	}
	c := effects.NewCtxPtr(s, 0, effects.CtxInit{SVars: face.SVars, Targets: tgts})
	c.TargetsOffered = true
	effects.Resolve(e, c, cards.ResolveSVar(face.SVars, "Act"))
}

// TestCRDeviation712_9 pins CR 712.9-712.10.
// Rule: only a double-faced permanent can transform, and never into an instant or sorcery face.
// Engine: effSetState transforms any two-face card, so a flip card and a DFC with a sorcery back both flip.
func TestCRDeviation712_9(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	flip := onBoard(t, e, 0, "Name:Flip Front\nTypes:Creature\nPT:1/1\nAlternateMode:Flip\nOracle:x\n\nALTERNATE\n\nName:Flip Back\nTypes:Creature\nPT:3/3\nOracle:x\n")
	sorc := onBoard(t, e, 0, "Name:Sorc Front\nTypes:Creature\nPT:1/1\nAlternateMode:DoubleFaced\nOracle:x\n\nALTERNATE\n\nName:Sorc Back\nTypes:Sorcery\nOracle:x\n")
	if e.G.Obj(flip).Card.IsTransformingDFC() || e.G.Obj(flip).FaceIdx != 0 || e.G.Obj(sorc).FaceIdx != 0 {
		t.Fatal("precondition: the flip card is not double-faced and both start on face 0")
	}
	crpinESetState(t, e, "Name:Transformer\nTypes:Artifact\nSVar:Act:DB$ SetState | Defined$ Targeted | Mode$ Transform\nOracle:x\n", flip, sorc)
	if got := faceName(t, e, flip); got != "Flip Back" {
		t.Fatalf("flip card face = %q, want Flip Back (engine transforms a non-DFC today)", got)
	}
	if got := faceName(t, e, sorc); got != "Sorc Back" {
		t.Fatalf("sorcery-back DFC face = %q, want Sorc Back (engine transforms into a sorcery face today)", got)
	}
}

// TestCRDeviation712_16 pins CR 712.16.
// Rule: a double-faced permanent can't be turned face down; the effect does nothing.
// Engine: effSetState's TurnFaceDown has no DFC gate, so the DFC turns face down.
func TestCRDeviation712_16(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	dfc := onBoard(t, e, 0, plainDFCSrc)
	if !e.G.Obj(dfc).Card.IsTransformingDFC() || e.G.Obj(dfc).FaceDown {
		t.Fatal("precondition: a face-up double-faced permanent")
	}
	crpinESetState(t, e, "Name:Hider\nTypes:Artifact\nSVar:Act:DB$ SetState | Defined$ Targeted | Mode$ TurnFaceDown\nOracle:x\n", dfc)
	if !e.G.Obj(dfc).FaceDown {
		t.Fatal("DFC stayed face up; the engine turns a DFC face down today")
	}
}

// TestCRDeviation712_21c pins CR 712.21c.
// Rule: an effect that finds the new object a melded permanent becomes finds both cards and acts on each.
// Engine: the effect follows only the result card's object, so an exile-and-return brings back one card.
func TestCRDeviation712_21c(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const gisela, bruna = "Gisela, the Broken Blade", "Bruna, the Fading Light"
	e, _ := meldGame(t, reg, []string{gisela, bruna}, nil)
	g, b := meldPut(t, e, gisela, 0), meldPut(t, e, bruna, 0)
	if res, ok := effects.Meld(e, 0, g, b, state.MeldOptions{ResultName: "Brisela, Voice of Nightmares"}); !ok || res != g {
		t.Fatalf("precondition: Meld = %d,%v; want %d", res, ok, g)
	}
	crpinESetState(t, e, "Name:Journey\nTypes:Artifact\n"+
		"SVar:Act:DB$ ChangeZone | Defined$ Targeted | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True | SubAbility$ Back\n"+
		"SVar:Back:DB$ ChangeZone | Defined$ Remembered | Origin$ Exile | Destination$ Battlefield\nOracle:x\n", g)
	if z := e.G.Obj(g).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: the result card is in %s, want returned to the battlefield", z)
	}
	if z := e.G.Obj(b).Zone; z != state.ZExile {
		t.Fatalf("partner card is in %s, want exile (the engine returns only the result card today)", z)
	}
}

// TestCRDeviation721_2c pins CR 721.2c.
// Rule: a station card in any zone but the battlefield has no power or toughness.
// Engine: chars.PT reads the printed PT in every zone, so a graveyard station card has power 5.
func TestCRDeviation721_2c(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Station Ship\nManaCost:4\nTypes:Artifact Spacecraft\nPT:5/3\nK:Station\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: station card in %s, want graveyard", z)
	}
	if p, tt := e.Power(id), e.Toughness(id); p != 5 || tt != 3 {
		t.Fatalf("graveyard station card P/T = %d/%d, want 5/3 (engine keeps printed P/T today)", p, tt)
	}
}

// TestCRDeviation730_2d pins CR 730.2d-730.2e.
// Rule: a merged permanent is a token only if its topmost component is a token.
// Engine: foldMutate keeps the target's token status, so a nontoken mutated on top of a token stays a token.
func TestCRDeviation730_2d(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	phoenix := mustCorpusCard(t, reg, "Everquill Phoenix")
	e, _ := tokenReplGame(t, 301, phoenix)
	phoenixID := moveSeededCard(t, e, 0, phoenix, state.ZHand)
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	if !e.G.Obj(bear).IsToken || e.G.Obj(phoenixID).IsToken {
		t.Fatal("precondition: the bear is a token and the Phoenix is not")
	}
	addMana(t, e, 0, "RRRR")
	mutateCastOnto(t, e, mutatedCastOption(t, e, phoenixID), bear, true)
	pile := e.G.Obj(bear)
	if pile.Face() == nil || pile.Face().Name != "Everquill Phoenix" {
		t.Fatalf("precondition: top card = %v, want Everquill Phoenix", pile.Face())
	}
	if !pile.IsToken {
		t.Fatal("pile is not a token; the engine keeps the target's token status today")
	}
}
