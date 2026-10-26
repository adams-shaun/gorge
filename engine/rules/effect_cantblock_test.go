// cr-g156: an Effect-delivered `Mode$ CantBlock` (Cosmotronic Wave's
// "creatures your opponents control can't block this turn", and the sibling
// carriers Ahn-Crop Crasher, Ember Gale, Hazardous Blast, Headliner Scarlett,
// Immobilizer Eldrazi, Rumbling Ruin, Sontaran General) must register as a
// continuous restriction and be enforced by combat.BlockRestricted beside the
// printed S: static, the same registered walk the Effect-delivered CantBlockBy
// grants already run.

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	cantBlockAttackerSrc = "Name:Hill Giant\nManaCost:3 R\nTypes:Creature Giant\nPT:2/2\nOracle:x\n"
	cantBlockDefenderSrc = "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:3/3\nOracle:x\n"
	cantBlockThirdSrc    = "Name:Canyon Minotaur\nManaCost:3 R\nTypes:Creature Minotaur\nPT:3/3\nOracle:x\n"
)

// cantBlockPlace builds a seatZeroStart fixture with the named card in seat
// seat0's library and one named creature per other seat, moves them with
// logged MoveZone events (the whole board stays replayable from the log), and
// returns the engine plus the moved ids.
func cantBlockPlace(t *testing.T, seed uint64, names []string, spell *cards.Card, others []*cards.Card) (*Engine, Config, state.ObjID, []state.ObjID) {
	t.Helper()
	decks := [][]*cards.Card{{spell, others[0]}}
	decks[0] = append(decks[0], mountainDeck(t, 38)...)
	for i := 1; i < len(names); i++ {
		decks = append(decks, append([]*cards.Card{others[i]}, mountainDeck(t, 39)...))
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: names, Decks: decks,
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	nameOf := func(p state.PlayerID, want string) state.ObjID {
		for _, zone := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(zone, p) {
				if e.G.Obj(id).Face().Name == want {
					return id
				}
			}
		}
		t.Fatalf("no %q in seat %d's hand or library", want, p)
		return 0
	}
	var othersIDs []state.ObjID
	spellID := nameOf(0, spell.Faces[0].Name)
	for i, c := range others {
		id := nameOf(state.PlayerID(i), c.Faces[0].Name)
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
		othersIDs = append(othersIDs, id)
	}
	// The spell stays in seat 0's library: a hand-move here would inflate the
	// opening hand and turn a later cleanup into a two-card discard ask the
	// driving helpers cannot answer. Each caller moves it to hand at its own
	// Main1 (cantBlockToHand), after the drive.
	return e, cfg, spellID, othersIDs
}

// cantBlockToHand moves the spell from seat 0's library to its hand with a
// logged MoveZone and re-asks priority, the same bridge counterplayeraddedall_test.go's
// Rikku fixtures run.
func cantBlockToHand(t *testing.T, e *Engine, spellID state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.MoveZone, Obj: spellID, From: state.ZLibrary, To: state.ZHand})
	e.pending = nil
	e.Advance()
}

// cantBlockRegistered finds the registered Effect-delivered CantBlock
// restriction (the precondition the BlockRestricted walk reads).
func cantBlockRegistered(t *testing.T, e *Engine) *ContinuousEffect {
	t.Helper()
	for i := range e.continuous {
		if e.continuous[i].Restriction == "CantBlock" {
			return &e.continuous[i]
		}
	}
	return nil
}

// cantBlockCast casts the named spell from seat 0's hand at Main1, funding
// the cost with addMana.
func cantBlockCast(t *testing.T, e *Engine, spellID state.ObjID, mana string) {
	t.Helper()
	addMana(t, e, 0, mana)
	d := e.Pending()
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == spellID {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the spell: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 30)
}

// TestEffectCantBlockCosmotronicWave is the probe: seat 0 casts Cosmotronic
// Wave in main 1 (1 damage to each creature your opponents control; they
// can't block this turn). Seat 1's 3/3 survives the damage and must be
// offered NO block; seat 0's own creature (not OppCtrl) stays a legal blocker
// for the reverse pair.
func TestEffectCantBlockCosmotronicWave(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	wave, ok := reg.Lookup("Cosmotronic Wave")
	if !ok {
		t.Fatal("corpus has no Cosmotronic Wave")
	}
	e, cfg, waveID, ids := cantBlockPlace(t, 6156, []string{"a", "b"}, wave,
		[]*cards.Card{card(t, cantBlockAttackerSrc), card(t, cantBlockDefenderSrc)})
	atkID, blkID := ids[0], ids[1]
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	cantBlockToHand(t, e, waveID)
	cantBlockCast(t, e, waveID, "CCCR")

	// Preconditions: the wave dealt its 1 to the opponents' creatures and the
	// Effect registered a this-turn CantBlock restriction.
	if got := e.G.Obj(blkID).Damage; got != 1 {
		t.Fatalf("precondition: seat 1's creature damage after the wave = %d, want 1", got)
	}
	ce := cantBlockRegistered(t, e)
	if ce == nil {
		t.Fatal("no CantBlock restriction registered after the wave resolved")
	}
	if !ce.UntilEOT {
		t.Fatal("precondition: the registered CantBlock is not a this-turn effect")
	}

	// Seat 1's 3/3 is forbidden from blocking; seat 0's own creature is not
	// (the ValidCard$ Creature.OppCtrl spec does not name it).
	if !e.blockRestricted(blkID, atkID) {
		t.Fatal("seat 1's creature can still block after Cosmotronic Wave (CR 509.1b)")
	}
	if e.blockRestricted(atkID, blkID) {
		t.Fatal("seat 0's own creature is wrongly restricted (not Creature.OppCtrl)")
	}

	// End to end: the 2/2 attacks, seat 1 is offered no block, and the damage
	// lands unblocked.
	e.G.Obj(atkID).SummonSick = false
	driveToStepAll(t, e, 3, 0, state.StepDeclareAttackers)
	e.askAttackers()
	submitAttackers(t, e, atkID)
	d := e.Pending()
	if d != nil && d.Kind == decision.KBlockers {
		for _, o := range d.Options {
			if o.Obj == blkID {
				t.Fatalf("seat 1's creature was offered as a blocker: %+v", d.Options)
			}
		}
	}
	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("defender life after the unblocked attack = %d, want 18", got)
	}
	replayCheck(t, e, cfg)
}

// TestEffectCantBlockExpiresAtEndOfTurn pins the lifetime: the wave's body
// carries no Duration$, so the registration is UntilEOT (the same
// absentDurationMeansThisTurn default CantBlockBy keeps) and the same creature
// is a legal blocker again on seat 0's NEXT turn.
func TestEffectCantBlockExpiresAtEndOfTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	wave, ok := reg.Lookup("Cosmotronic Wave")
	if !ok {
		t.Fatal("corpus has no Cosmotronic Wave")
	}
	e, cfg, waveID, ids := cantBlockPlace(t, 6157, []string{"a", "b"}, wave,
		[]*cards.Card{card(t, cantBlockAttackerSrc), card(t, cantBlockDefenderSrc)})
	atkID, blkID := ids[0], ids[1]
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	cantBlockToHand(t, e, waveID)
	cantBlockCast(t, e, waveID, "CCCR")
	if !e.blockRestricted(blkID, atkID) {
		t.Fatal("precondition: the restriction is not live in the turn of the cast")
	}
	putTurn := e.G.Turn
	driveToStepAll(t, e, putTurn+2, 0, state.StepDeclareAttackers)
	if cantBlockRegistered(t, e) != nil {
		t.Fatalf("the CantBlock restriction survived past end of turn %d", putTurn)
	}
	if e.blockRestricted(blkID, atkID) {
		t.Fatal("seat 1's creature is still forbidden from blocking after the turn ended")
	}
	e.G.Obj(atkID).SummonSick = false
	e.askAttackers()
	submitAttackers(t, e, atkID)
	e.G.Step = state.StepDeclareBlockers
	e.askBlockers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("blocker decision after the window expired = %+v, want the creature offered", d)
	}
	found := false
	for _, o := range d.Options {
		if o.Obj == blkID {
			found = true
		}
	}
	if !found {
		t.Fatalf("the creature had no block option after the wave expired: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}

// TestEffectCantBlockRememberedPlayer pins the targeted-player carriers
// (Ember Gale, Headliner Scarlett): `RememberObjects$ Targeted` captures the
// targeted player and the body's ValidCard$ Creature.RememberedPlayerCtrl
// resolves against that set at consultation time — seat 1's creatures can't
// block, seat 2's still can.
func TestEffectCantBlockRememberedPlayer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	gale, ok := reg.Lookup("Ember Gale")
	if !ok {
		t.Fatal("corpus has no Ember Gale")
	}
	e, cfg, galeID, ids := cantBlockPlace(t, 6158, []string{"a", "b", "c"}, gale,
		[]*cards.Card{card(t, cantBlockAttackerSrc), card(t, cantBlockDefenderSrc), card(t, cantBlockThirdSrc)})
	atkID, blk1, blk2 := ids[0], ids[1], ids[2]
	// Three seats: turn 3 is seat 2's turn; seat 0's second turn is turn 4.
	driveToStepAll(t, e, 4, 0, state.StepMain1)
	cantBlockToHand(t, e, galeID)
	addMana(t, e, 0, "CCCR")
	d := e.Pending()
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == galeID {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Ember Gale: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after casting: %+v, want the player-target ask", d)
	}
	tidx := -1
	for _, o := range d.Options {
		if o.Player == 1 {
			tidx = o.Index
		}
	}
	if tidx < 0 {
		t.Fatalf("seat 1 was not offered as a target: %+v", d.Options)
	}
	submitChoices(t, e, tidx)
	passUntilStackEmpty(t, e, 30)

	// Preconditions: the target is captured and the body's damage half dealt
	// nothing (the bears are green — DBDamage only hits white/blue).
	ce := cantBlockRegistered(t, e)
	if ce == nil {
		t.Fatal("no CantBlock restriction registered after Ember Gale resolved")
	}
	hit := false
	for _, p := range ce.RememberedPlayers {
		if p == 1 {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("the registration does not remember seat 1: %v", ce.RememberedPlayers)
	}
	if got := e.G.Obj(blk1).Damage; got != 0 {
		t.Fatalf("precondition: seat 1's green creature took %d damage, want 0", got)
	}

	// Seat 1's creatures can't block; seat 2's can.
	if !e.blockRestricted(blk1, atkID) {
		t.Fatal("seat 1's creature can still block after Ember Gale")
	}
	if e.blockRestricted(blk2, atkID) {
		t.Fatal("seat 2's creature is wrongly restricted (the target was seat 1)")
	}

	// End to end: seat 0 attacks seat 1, seat 1 is offered no block.
	e.G.Obj(atkID).SummonSick = false
	driveToStepAll(t, e, 4, 0, state.StepDeclareAttackers)
	e.askAttackers()
	d = e.Pending()
	aidx := -1
	for _, o := range d.Options {
		if o.Obj == atkID && o.Player == 1 {
			aidx = o.Index
		}
	}
	if aidx < 0 {
		t.Fatalf("no attack option against seat 1: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{aidx}}); err != nil {
		t.Fatalf("submit attackers: %v", err)
	}
	drainCombatPriority(t, e)
	d = e.Pending()
	if d != nil && d.Kind == decision.KBlockers {
		for _, o := range d.Options {
			if o.Obj == blk1 {
				t.Fatalf("seat 1's creature was offered as a blocker: %+v", d.Options)
			}
		}
	}
	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("defender 1's life after the unblocked attack = %d, want 18", got)
	}
	replayCheck(t, e, cfg)
}

// TestEffectCantBlockUnreadParamStaysUnregistered is the whitelist guard: a
// body carrying a scoping term the registration path cannot evaluate
// (IsPresent$) keeps its loud unimplemented Note and registers NOTHING — the
// creatures stay legal blockers, the fail-closed-to-Note direction every
// sibling whitelist arm keeps.
func TestEffectCantBlockUnreadParamStaysUnregistered(t *testing.T) {
	t.Parallel()
	gated := "Name:Gated Wave\nManaCost:R\nTypes:Sorcery\n" +
		"A:SP$ Effect | StaticAbilities$ CB | SpellDescription$ Creatures your opponents control can't block this turn.\n" +
		"SVar:CB:Mode$ CantBlock | ValidCard$ Creature.OppCtrl | IsPresent$ Creature | PresentCompare$ GE1 | Description$ Creatures your opponents control can't block this turn.\nOracle:x\n"
	e, cfg, spellID, ids := cantBlockPlace(t, 6159, []string{"a", "b"},
		card(t, gated), []*cards.Card{card(t, cantBlockAttackerSrc), card(t, cantBlockDefenderSrc)})
	atkID, blkID := ids[0], ids[1]
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	cantBlockToHand(t, e, spellID)
	cantBlockCast(t, e, spellID, "R")

	if !hasNote(e, "continuous effect CantBlock unimplemented") {
		t.Fatal("no unimplemented Note for the gated CantBlock body")
	}
	if ce := cantBlockRegistered(t, e); ce != nil {
		t.Fatalf("the gated body registered a CantBlock restriction anyway: %+v", *ce)
	}
	if e.blockRestricted(blkID, atkID) {
		t.Fatal("the unread-param body restricted blocking anyway")
	}
	e.G.Obj(atkID).SummonSick = false
	driveToStepAll(t, e, 3, 0, state.StepDeclareAttackers)
	e.askAttackers()
	submitAttackers(t, e, atkID)
	e.G.Step = state.StepDeclareBlockers
	e.askBlockers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("blocker decision = %+v, want the creature offered (nothing registered)", d)
	}
	found := false
	for _, o := range d.Options {
		if o.Obj == blkID {
			found = true
		}
	}
	if !found {
		t.Fatalf("the creature lost its block option to an unregistered body: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}

// TestEffectCantBlockChosenNumberStaysUnregistered: a Rumbling Ruin-shaped
// body (SetChosenNumber$ + powerLEX over Count$ChosenNumber) fails closed to
// the Note rather than registering a restriction that reads the source's 0.
func TestEffectCantBlockChosenNumberStaysUnregistered(t *testing.T) {
	t.Parallel()
	gated := "Name:Ruin Wave\nManaCost:R\nTypes:Sorcery\n" +
		"A:SP$ Effect | SetChosenNumber$ 2 | StaticAbilities$ CB | SpellDescription$ Creatures your opponents control can't block this turn.\n" +
		"SVar:CB:Mode$ CantBlock | ValidCard$ Creature.OppCtrl+powerLEX | Description$ Creatures your opponents control can't block this turn.\nSVar:X:Count$ChosenNumber\nOracle:x\n"
	e, cfg, spellID, ids := cantBlockPlace(t, 6159, []string{"a", "b"},
		card(t, gated), []*cards.Card{card(t, cantBlockAttackerSrc), card(t, cantBlockDefenderSrc)})
	atkID, blkID := ids[0], ids[1]
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	cantBlockToHand(t, e, spellID)
	cantBlockCast(t, e, spellID, "R")

	if !hasNote(e, "continuous effect CantBlock unimplemented") {
		t.Fatal("no unimplemented Note for the SetChosenNumber$ CantBlock body")
	}
	if ce := cantBlockRegistered(t, e); ce != nil {
		t.Fatalf("the SetChosenNumber$ body registered a CantBlock restriction anyway: %+v", *ce)
	}
	if e.blockRestricted(blkID, atkID) {
		t.Fatal("the unread-param body restricted blocking anyway")
	}
	e.G.Obj(atkID).SummonSick = false
	driveToStepAll(t, e, 3, 0, state.StepDeclareAttackers)
	e.askAttackers()
	submitAttackers(t, e, atkID)
	e.G.Step = state.StepDeclareBlockers
	e.askBlockers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("blocker decision = %+v, want the creature offered (nothing registered)", d)
	}
	found := false
	for _, o := range d.Options {
		if o.Obj == blkID {
			found = true
		}
	}
	if !found {
		t.Fatalf("the creature lost its block option to an unregistered body: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}
