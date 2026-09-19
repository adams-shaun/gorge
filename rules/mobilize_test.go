package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// mobilizeCombat seats the named Mobilize card under seat 0, puts it on the
// battlefield, declares it attacking seat 1 and drains the resulting stack --
// the same shape myriadCombat drives for Myriad. extras are card sources
// shuffled into seat 0's deck at genesis (so every object exists in replay);
// tests move them where they need them with moveToGraveyardFromLibrary /
// moveToBattlefieldFromLibrary. The engine ends here with the Mobilize
// tokens minted and still mid-combat (nothing has cleared their attacking
// marks), which is the state every test below asserts from.
func mobilizeCombat(t *testing.T, name string, seats int, extras ...string) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	src, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("%s missing from corpus", name)
	}
	if d := src.Link(); len(d) != 0 {
		t.Fatalf("link %s: %v", name, d)
	}
	names := make([]string, seats)
	decks := make([][]*cards.Card, seats)
	for i := range names {
		names[i] = string(rune('a' + i))
		decks[i] = mountainDeck(t, 40)
	}
	decks[0] = append(decks[0], src)
	for _, extra := range extras {
		decks[0] = append(decks[0], card(t, extra))
	}
	cfg := seatZeroStart(Config{Seed: 196, Names: names, Decks: decks, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	want := name
	var did state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == want {
			did = id
		}
	}
	if did == 0 {
		t.Fatalf("%s was not dealt", name)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: did, From: state.ZLibrary, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{did}})
	e.priorityRound()
	passUntilStackEmpty(t, e, 30)
	return e, cfg, did
}

// findInLibrary returns the object id whose face is named name in seat 0's
// library (an extra shuffled in at genesis), or fails.
func findInLibrary(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("%s not in seat 0's library", name)
	return 0
}

// moveToGraveyardFromLibrary puts the named genesis-dealt card in seat 0's
// graveyard, through the event the replay folds.
func moveToGraveyardFromLibrary(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	id := findInLibrary(t, e, name)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZGraveyard})
	return id
}

// moveToBattlefieldFromLibrary puts the named genesis-dealt card on seat 0's
// battlefield, through the event the replay folds.
func moveToBattlefieldFromLibrary(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	id := findInLibrary(t, e, name)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	return id
}

// mobilizeTokens returns seat 0's Mobilize Warrior tokens on the battlefield.
func mobilizeTokens(t *testing.T, e *Engine) []*state.Object {
	t.Helper()
	var out []*state.Object
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Warrior Token" {
			out = append(out, o)
		}
	}
	return out
}

// hasTypeWord reports whether the face's Types slice names the word verbatim
// (a Types entry is one whole word -- "Creature", "Warrior" -- never a
// substring).
func hasTypeWord(types []string, word string) bool {
	for _, ty := range types {
		if ty == word {
			return true
		}
	}
	return false
}

// TestMobilizeUsesRealCorpusCard drives Zurgo Stormrender's real K:Mobilize:1
// line end to end: the attack creates exactly one 1/1 red Warrior token that
// entered tapped and attacking the defender. Before the fix the printed
// keyword never expanded and Zurgo attacked bare.
func TestMobilizeUsesRealCorpusCard(t *testing.T) {
	e, cfg, _ := mobilizeCombat(t, "Zurgo Stormrender", 3)
	toks := mobilizeTokens(t, e)
	if len(toks) != 1 {
		t.Fatalf("Mobilize minted %d Warrior tokens, want 1", len(toks))
	}
	tok := toks[0]
	if !tok.Tapped || !tok.IsAttacking {
		t.Fatalf("token tapped=%v attacking=%v, want tapped and attacking", tok.Tapped, tok.IsAttacking)
	}
	if tok.Attacking != 1 {
		t.Fatalf("token attacks seat %d, want the defender seat 1", tok.Attacking)
	}
	f := tok.Face()
	if f.Power() != 1 || f.Toughness() != 1 || !strings.Contains(f.Colors, "red") || !hasTypeWord(f.Types, "Warrior") {
		t.Fatalf("token face %s colours=%q types=%q PT=%d/%d, want a 1/1 red Warrior", f.Name, f.Colors, f.Types, f.Power(), f.Toughness())
	}
	if tok.Controller != 0 {
		t.Fatalf("token controller %d, want seat 0", tok.Controller)
	}
	replayCheck(t, e, cfg)
}

// TestMobilizeCountScalesWithTheKeyword: a single-count card passing by
// coincidence is the failure mode the brief names, so a K:Mobilize:2 card
// (Bone-Cairn Butcher) must mint exactly two tokens, both tapped and
// attacking, and K:Mobilize:3 (Dalkovan Packbeasts) exactly three.
func TestMobilizeCountScalesWithTheKeyword(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
	}{{"Bone-Cairn Butcher", 2}, {"Dalkovan Packbeasts", 3}} {
		e, _, _ := mobilizeCombat(t, tc.name, 2)
		toks := mobilizeTokens(t, e)
		if len(toks) != tc.count {
			t.Fatalf("%s minted %d tokens, want %d", tc.name, len(toks), tc.count)
		}
		for _, tok := range toks {
			if !tok.Tapped || !tok.IsAttacking || tok.Attacking != 1 {
				t.Fatalf("%s token %d tapped=%v attacking=%v defender=%d", tc.name, tok.ID, tok.Tapped, tok.IsAttacking, tok.Attacking)
			}
		}
	}
}

// TestMobilizeXResolvesFromTheGraveyard: Avenger of the Fallen's
// K:Mobilize:X rides SVar:X:Count$ValidGraveyard Creature.YouOwn; the
// ordinary Num/SVar grammar must resolve X at trigger resolution (one
// creature card in the graveyard -> one token), with no new grammar.
func TestMobilizeXResolvesFromTheGraveyard(t *testing.T) {
	e, cfg, _ := mobilizeCombat(t, "Avenger of the Fallen", 2,
		"Name:Grave Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	moveToGraveyardFromLibrary(t, e, "Grave Bear")
	// The X count reads at TRIGGER resolution, after the attack is declared:
	// re-drive the attack now that the graveyard holds a creature card.
	var avenger state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Avenger of the Fallen" {
			avenger = id
		}
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{avenger}})
	e.priorityRound()
	passUntilStackEmpty(t, e, 30)
	toks := mobilizeTokens(t, e)
	if len(toks) != 1 {
		t.Fatalf("Mobilize X minted %d tokens with one creature card in the graveyard, want 1", len(toks))
	}
	for _, tok := range toks {
		if !tok.Tapped || !tok.IsAttacking || tok.Attacking != 1 {
			t.Fatalf("token %d tapped=%v attacking=%v defender=%d", tok.ID, tok.Tapped, tok.IsAttacking, tok.Attacking)
		}
	}
	replayCheck(t, e, cfg)
}

// TestMobilizeSacrificesAtTheNextEndStep: the surviving token is sacrificed
// at the beginning of the next end step, and Zurgo's second trigger fires for
// the leaving token with the Branch resolving the FALSE arm -- after
// EndCombatReset the token is no longer attacking, so each opponent loses
// 1 life and nobody draws.
func TestMobilizeSacrificesAtTheNextEndStep(t *testing.T) {
	e, cfg, _ := mobilizeCombat(t, "Zurgo Stormrender", 3)
	toks := mobilizeTokens(t, e)
	if len(toks) != 1 {
		t.Fatalf("Mobilize minted %d tokens, want 1", len(toks))
	}
	// End the combat (the token keeps the battlefield, unmarked), then walk
	// main 2 into the end step, where the delayed sacrifice fires. The drain
	// above leaves a priority ask outstanding and the engine defers
	// finishStepBoundary while a decision is pending, so clear it the way
	// TestLeavingEndCombatRemovesAttackerBeforePostcombatMain does before
	// driving the transition by hand -- otherwise EndCombatReset never runs
	// and the token is still marked attacking when it leaves.
	e.pending = nil
	e.setStep(state.StepEndCombat)
	e.advanceStep() // StepMain2
	e.advanceStep() // StepEnd: the registration fires here
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(toks[0].ID); o != nil && o.Zone == state.ZBattlefield {
		t.Fatal("the Mobilize token survived the end step")
	}
	if e.G.Players[1].Life != 19 || e.G.Players[2].Life != 19 {
		t.Fatalf("opponent life %d/%d, want 19/19 (the FALSE arm: each opponent loses 1)", e.G.Players[1].Life, e.G.Players[2].Life)
	}
	if e.G.Players[0].Life != 20 {
		t.Fatalf("controller life %d, want 20 (the TRUE arm must not run)", e.G.Players[0].Life)
	}
	replayCheck(t, e, cfg)
}

// TestMobilizeTokenKilledInCombatDraws: a Mobilize token that left the
// battlefield WHILE STILL ATTACKING (killed mid-combat) resolves the Branch's
// TRUE arm -- Zurgo's controller draws a card, the opponents lose nothing.
func TestMobilizeTokenKilledInCombatDraws(t *testing.T) {
	e, cfg, _ := mobilizeCombat(t, "Zurgo Stormrender", 3)
	toks := mobilizeTokens(t, e)
	if len(toks) != 1 {
		t.Fatalf("Mobilize minted %d tokens, want 1", len(toks))
	}
	tok := toks[0]
	handBefore := len(e.G.Zone(state.ZHand, 0))
	// Lethal damage while the token is still attacking: the state-based sweep
	// buries it, and the leaves trigger sees an attacking token (CR 603.10's
	// LKI read is captured before the move).
	e.emit(events.Event{Kind: events.Damage, Obj: tok.ID, Amount: 1})
	e.checkStateBased()
	if o := e.G.Obj(tok.ID); o != nil && o.Zone == state.ZBattlefield {
		t.Fatal("lethally damaged token stayed on the battlefield")
	}
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if got := len(e.G.Zone(state.ZHand, 0)); got <= handBefore {
		t.Fatalf("seat 0 hand %d after the attacking token died, want more than the %d before (TRUE arm draws)", got, handBefore)
	}
	if e.G.Players[1].Life != 20 || e.G.Players[2].Life != 20 {
		t.Fatalf("opponent life %d/%d, want 20/20 (the FALSE arm must not run)", e.G.Players[1].Life, e.G.Players[2].Life)
	}
	replayCheck(t, e, cfg)
}

// TestMobilizeSecondTriggerCountsOnlyItsOwnTokens: a NON-token creature
// leaving the battlefield never fires Zurgo's second trigger; a Mobilize
// token leaving while NOT attacking (post-combat) resolves the FALSE arm.
func TestMobilizeSecondTriggerCountsOnlyItsOwnTokens(t *testing.T) {
	e, cfg, _ := mobilizeCombat(t, "Zurgo Stormrender", 3,
		"Name:Bystander Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	toks := mobilizeTokens(t, e)
	if len(toks) != 1 {
		t.Fatalf("Mobilize minted %d tokens, want 1", len(toks))
	}
	// A non-token creature dies: no draw, no life loss.
	bo := moveToBattlefieldFromLibrary(t, e, "Bystander Bear")
	e.emit(events.Event{Kind: events.MoveZone, Obj: bo, From: state.ZBattlefield, To: state.ZGraveyard})
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if e.G.Players[1].Life != 20 || e.G.Players[2].Life != 20 {
		t.Fatalf("a non-token departure moved life to %d/%d, want untouched", e.G.Players[1].Life, e.G.Players[2].Life)
	}
	// Now the real token leaves at the end step (the delayed sacrifice; the
	// EndCombatReset has run, so the leaving token is not attacking).
	e.pending = nil
	e.setStep(state.StepEndCombat)
	e.advanceStep()
	e.advanceStep()
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if e.G.Players[1].Life != 19 || e.G.Players[2].Life != 19 {
		t.Fatalf("opponent life %d/%d after the token left, want 19/19", e.G.Players[1].Life, e.G.Players[2].Life)
	}
	replayCheck(t, e, cfg)
}
