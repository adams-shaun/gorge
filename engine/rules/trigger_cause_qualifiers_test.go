package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runCauseScenario plays an oracle scenario and fails on any expectation or
// step failure, printing the transcript.
func runCauseScenario(t *testing.T, raw string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed:\n%s", strings.Join(res.Fails, "\n"))
	}
}

const imodaneSetup = `"setup":{"p0":{"battlefield":["Imodane, the Pyrohammer"],"hand":["%s"]},"p1":{"battlefield":[%s]}}`

// Pyroclasm targets nothing and damages every creature: the spell is not one
// "that targets only a single creature", so Imodane stays quiet (p1 would
// drop to 14 with the cause qualifier unread: Imodane and both Elves fire).
func TestImodaneSweeperDoesNotTrigger(t *testing.T) {
	t.Parallel()
	runCauseScenario(t, `{"name":"imodane-sweeper","cr":["603.2"],"why":"ValidCause Spell.numTargets EQ1 excludes a sweeper",
 "setup":{"p0":{"battlefield":["Imodane, the Pyrohammer"],"hand":["Pyroclasm"]},"p1":{"battlefield":["Llanowar Elves","Fyndhorn Elves"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Pyroclasm","mana":"CR"},{"op":"resolve"}],
 "expect":[{"card":"p1:Llanowar Elves","zone":"graveyard"},{"card":"p1:Fyndhorn Elves","zone":"graveyard"},{"stack_size":0},{"life":{"p1":20}}]}`)
}

// Each half of the cause clause excludes a different spell. Arc Trail targets
// two creatures: both are "targeted by" it, but it is not a spell "that
// targets only a single creature" (ValidCause$ Spell.numTargets EQ1). Flame
// Wave targets one player and damages that player's creatures: a single
// target, but the damaged creatures are not what it targets
// (TargetRelativeToCause$ Creature.targetedBy).
func TestImodaneEachCauseClauseExcludes(t *testing.T) {
	t.Parallel()
	runCauseScenario(t, `{"name":"imodane-two-targets","cr":["603.2"],"why":"a two-target spell is not a single-target spell",
 "setup":{"p0":{"battlefield":["Imodane, the Pyrohammer"],"hand":["Arc Trail"]},"p1":{"battlefield":["Llanowar Elves","Fyndhorn Elves"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Arc Trail","mana":"CR","targets":["p1:Llanowar Elves","p1:Fyndhorn Elves"]},{"op":"resolve"}],
 "expect":[{"card":"p1:Llanowar Elves","zone":"graveyard"},{"card":"p1:Fyndhorn Elves","zone":"graveyard"},{"stack_size":0},{"life":{"p1":20}}]}`)
	runCauseScenario(t, `{"name":"imodane-splash","cr":["603.2"],"why":"a creature the spell does not target is not damaged by the spell relative to its target",
 "setup":{"p0":{"battlefield":["Imodane, the Pyrohammer"],"hand":["Flame Wave"]},"p1":{"battlefield":["Llanowar Elves","Fyndhorn Elves"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Flame Wave","mana":"CCCRRRR","targets":["p1"]},{"op":"resolve"}],
 "expect":[{"card":"p1:Llanowar Elves","zone":"graveyard"},{"card":"p1:Fyndhorn Elves","zone":"graveyard"},{"stack_size":0},{"life":{"p1":16}}]}`)
}

// A Lightning Bolt on one creature is the single-target spell the card names:
// Imodane deals 3 to the opponent as well. Bolt to the face is not damage to
// a creature.
func TestImodaneSingleTargetBoltTriggers(t *testing.T) {
	t.Parallel()
	runCauseScenario(t, `{"name":"imodane-bolt-creature-resolved","cr":["603.2"],"why":"the trigger deals the bolt's 3 to the opponent",
 "setup":{"p0":{"battlefield":["Imodane, the Pyrohammer"],"hand":["Lightning Bolt"]},"p1":{"battlefield":["Grizzly Bears"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Lightning Bolt","mana":"R","targets":["p1:Grizzly Bears"]},{"op":"resolve"},{"op":"resolve"}],
 "expect":[{"life":{"p1":17}}]}`)
	runCauseScenario(t, `{"name":"imodane-bolt-face","cr":["603.2"],"why":"damage to a player is not damage to a creature",
 "setup":{"p0":{"battlefield":["Imodane, the Pyrohammer"],"hand":["Lightning Bolt"]},"p1":{"battlefield":["Grizzly Bears"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Lightning Bolt","mana":"R","targets":["p1"]},{"op":"resolve"}],
 "expect":[{"stack_size":0},{"life":{"p1":17}}]}`)
}

// causeTurnUpBoard seats the named corpus carrier on seat 0 plus one
// face-down Grizzly Bears per seat, returning the bears.
func causeTurnUpBoard(t *testing.T, carrier string) (e *Engine, carrierID, bears0, bears1 state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e = corpusEngine(t, reg, nil, nil)
	carrierID = onBoardCard(t, e, 0, lookup(t, reg, carrier))
	bears0 = onBoardCard(t, e, 0, lookup(t, reg, "Grizzly Bears"))
	bears1 = onBoardCard(t, e, 1, lookup(t, reg, "Grizzly Bears"))
	for _, id := range []state.ObjID{bears0, bears1} {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: bears %d not on the battlefield: %+v", id, o)
		}
		o.FaceDown = true
	}
	if e.G.Obj(carrierID).Controller != 0 || e.G.Obj(bears1).Controller != 1 {
		t.Fatal("precondition: carrier on seat 0, opposing bears on seat 1")
	}
	return e, carrierID, bears0, bears1
}

// Growing Dread's "Whenever YOU turn a permanent face up": the opponent
// turning its own permanent up gets no counter; seat 0's own turn-up does.
func TestGrowingDreadIgnoresOpponentTurnUp(t *testing.T) {
	t.Parallel()
	e, _, bears0, bears1 := causeTurnUpBoard(t, "Growing Dread")
	turnUp(t, e, bears1)
	if got := counterCount(e.G.Obj(bears1), "P1P1"); got != 0 {
		t.Fatalf("an opponent's turn-up put %d +1/+1 counters on its own creature, want 0", got)
	}
	turnUp(t, e, bears0)
	if got := counterCount(e.G.Obj(bears0), "P1P1"); got != 1 {
		t.Fatalf("seat 0's own turn-up put %d +1/+1 counters on the creature, want 1", got)
	}
}

// Oblivious Bookworm remembers "you turned a permanent face up this turn":
// only seat 0's turn-up counts.
func TestObliviousBookwormOpponentTurnUpDoesNotCount(t *testing.T) {
	t.Parallel()
	e, worm, bears0, bears1 := causeTurnUpBoard(t, "Oblivious Bookworm")
	turnUp(t, e, bears1)
	if got := len(e.G.Obj(worm).Imprinted); got != 0 {
		t.Fatalf("an opponent's turn-up was remembered (%d imprinted), want 0", got)
	}
	turnUp(t, e, bears0)
	if got := e.G.Obj(worm).Imprinted; len(got) != 1 || got[0] != bears0 {
		t.Fatalf("seat 0's turn-up imprinted %v, want [%d]", got, bears0)
	}
}

// Wan Shi Tong: "Whenever an opponent searches their library". p1 tutoring
// its own library fires it. Seat 0 (its controller) searching p1's library
// with Bribery does not: the searcher is not an opponent. Before the
// searcher was recorded the trigger read the library's owner and fired.
func TestWanShiTongOpponentSearchingOtherLibrary(t *testing.T) {
	t.Parallel()
	runCauseScenario(t, `{"name":"wst-own-search","cr":["603.2"],"why":"an opponent searching their own library fires",
 "setup":{"p0":{"battlefield":["Wan Shi Tong, Librarian"]},"p1":{"hand":["Demonic Tutor"]}},
 "steps":[{"op":"pass_to","step":"main1","active":"p1"},{"op":"cast","seat":1,"card":"p1:Demonic Tutor","mana":"CB"},{"op":"pass","seat":1},{"op":"pass","seat":0}],
 "expect":[{"stack_size":1},{"trigger_on_stack":"p0:Wan Shi Tong, Librarian"}]}`)
	runCauseScenario(t, `{"name":"wst-controller-searches-opponent","cr":["603.2"],"why":"the controller searching an opponent's library is not an opponent searching",
 "setup":{"p0":{"battlefield":["Wan Shi Tong, Librarian"],"hand":["Bribery"]},"p1":{"library":["Grizzly Bears"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Bribery","mana":"CCCUU","targets":["p1"]},{"op":"resolve","answers":[{"kind":"choose","pick":["Grizzly Bears"]}]}],
 "expect":[{"card":"p1:Grizzly Bears","zone":"battlefield"},{"stack_size":0}]}`)
	// SearchOwnLibrary$ True: an opponent (seat 1) searching a THIRD player's
	// library is an opponent searching, but not their own library.
	runCauseScenario(t, `{"name":"wst-opponent-searches-third-library","cr":["603.2"],"why":"SearchOwnLibrary requires the searcher to own the library","players":3,
 "setup":{"p0":{"battlefield":["Wan Shi Tong, Librarian"]},"p1":{"hand":["Bribery"]},"p2":{"library":["Grizzly Bears"]}},
 "steps":[{"op":"pass_to","step":"main1","active":"p1"},{"op":"cast","seat":1,"card":"p1:Bribery","mana":"CCCUU","targets":["p2"]},{"op":"resolve","answers":[{"kind":"choose","pick":["Grizzly Bears"]}]}],
 "expect":[{"card":"p2:Grizzly Bears","zone":"battlefield"},{"stack_size":0}]}`)
}

// Market Gnome's "exiled from the battlefield WHILE YOU'RE ACTIVATING A CRAFT
// ABILITY": an opponent's Swords to Plowshares exiles it with no trigger;
// paying Braided Net's craft cost with it gains 1 and draws.
func TestMarketGnomeExiledByRemovalNoTrigger(t *testing.T) {
	t.Parallel()
	runCauseScenario(t, `{"name":"gnome-exiled-by-removal","cr":["603.2"],"why":"an exile that is not a craft cost does not trigger",
 "setup":{"p0":{"battlefield":["Market Gnome"]},"p1":{"hand":["Swords to Plowshares"]}},
 "steps":[{"op":"pass_to","step":"main1","active":"p1"},{"op":"cast","seat":1,"card":"p1:Swords to Plowshares","mana":"W","targets":["p0:Market Gnome"]},{"op":"resolve"}],
 "expect":[{"card":"p0:Market Gnome","zone":"exile"},{"stack_size":0},{"life":{"p0":20}},{"hand_size":{"p0":0}}]}`)
	runCauseScenario(t, `{"name":"gnome-exiled-as-craft-material","cr":["603.2"],"why":"exiled as craft material fires",
 "setup":{"p0":{"battlefield":["Braided Net","Market Gnome"]}},
 "steps":[{"op":"activate","seat":0,"card":"p0:Braided Net","ability":"Craft","mana":"CU"}],
 "expect":[{"card":"p0:Market Gnome","zone":"exile"},{"trigger_on_stack":"p0:Market Gnome"},{"stack_size":2}]}`)
}

// causeTokens counts seat 0's battlefield tokens.
func causeTokens(e *Engine) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken {
			n++
		}
	}
	return n
}

// causeEmissarySrc is Foul Emissary's sacrifice trigger line verbatim without
// the corpus card's enters-the-battlefield dig (whose arrange ask would
// interleave with the emerge cast).
const causeEmissarySrc = "Name:Emissary Probe\nManaCost:2 G\nTypes:Creature Human Horror\nPT:1/1\n" +
	"T:Mode$ Sacrificed | ValidCard$ Card.Self | WhileKeyword$ Spell.withEmerge | Execute$ TrigToken | TriggerZones$ Battlefield | TriggerDescription$ When you sacrifice CARDNAME while casting a spell with emerge, create a 3/2 colorless Eldrazi Horror creature token.\n" +
	"SVar:TrigToken:DB$ Token | TokenScript$ c_3_2_eldrazi_horror | TokenOwner$ You\n"

// causeEmergeSrc is an emerge creature with no cast trigger of its own, so
// the Emissary trigger is the only one the cast queues.
const causeEmergeSrc = "Name:Emerge Probe\nManaCost:8\nTypes:Creature Eldrazi\nPT:5/5\nK:Emerge:5 U U\nOracle:x\n"

// Foul Emissary: "When you sacrifice it WHILE CASTING A SPELL WITH EMERGE,
// create a 3/2 Eldrazi Horror". Sacrificed as the emerge cost it makes the
// token; sacrificed any other way it does not.
func TestFoulEmissaryOnlyOnEmergeSacrifice(t *testing.T) {
	t.Parallel()
	e, _, _ := altCostEngine(t, 4244, nil, []string{causeEmergeSrc, causeEmissarySrc}, nil)
	deep := findCardObj(t, e, 0, "Emerge Probe", state.ZHand)
	emissary := findCardObj(t, e, 0, "Emissary Probe", state.ZBattlefield)
	if o := e.G.Obj(emissary); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Emissary Probe not on seat 0's battlefield: %+v", o)
	}
	if causeTokens(e) != 0 {
		t.Fatal("precondition: no tokens before the sacrifice")
	}
	addMana(t, e, 0, "UUCCCCC")
	castEmergeSacrificing(t, e, deep, emissary)
	if got := causeTokens(e); got != 1 {
		t.Fatalf("sacrificed to an emerge cast: %d tokens, want 1", got)
	}

	// The same sacrifice outside any cast flow makes none.
	e2, _, _ := altCostEngine(t, 4245, nil, []string{causeEmissarySrc}, nil)
	em2 := findCardObj(t, e2, 0, "Emissary Probe", state.ZBattlefield)
	e2.emit(events.Sacrifice(em2))
	answerQuiet(t, e2, 40)
	if o := e2.G.Obj(em2); o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Emissary Probe not sacrificed (zone %s)", o.Zone)
	}
	if got := causeTokens(e2); got != 0 {
		t.Fatalf("sacrificed by an effect: %d tokens, want 0", got)
	}
}
