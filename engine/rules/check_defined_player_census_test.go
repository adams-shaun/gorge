package rules

// CheckDefinedPlayer$ ("<base>.<property>") is the player-state intervening-if
// (CR 603.4) on a trigger and the applicability gate on a replacement. Before
// lb-crime, checkDefinedPlayerHolds read only isMonarch/hasInitiative and an
// unread predicate failed closed ONLY for actionTriggerModes: Servant of the
// Stinger's DamageDone "if you've committed a crime this turn" fired every
// time, and Jared Carthalion's monarch replacement never applied. This file
// pins the class: every corpus spelling is either read (and proven below) or
// listed in knownUnsupportedDefinedPlayer, which only shrinks, and every
// listed one fails closed.

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// readDefinedPlayerSpecs are the corpus CheckDefinedPlayer$ spellings the
// engine evaluates; each has a behavioural test in this file or (monarch,
// initiative) in the monarch/initiative trigger tests.
var readDefinedPlayerSpecs = map[string]bool{
	"You.isMonarch":                             true,
	"Opponent.isMonarch":                        true,
	"You.hasInitiative":                         true,
	"You.committedCrimeThisTurn":                true, // Servant of the Stinger
	"You.withMostLife":                          true, // Preacher of the Schism
	"Player.withMostTypeCreatureOnly":           true, // Wild Mammoth
	"Player.withMostTypeWizardOnly":             true, // Thoughtbound Primoc
	"Player.IsRemembered+counters_EQ0_Contract": true, // Miss Highwater's effect
	"You.damageDoneSingleSource GE5":            true, // Dragon Cultist
}

// knownUnsupportedDefinedPlayer is the ratchet: spellings no home reads yet.
// Each must fail closed. Delete an entry only with a test proving it is read.
var knownUnsupportedDefinedPlayer = map[string]bool{}

// definedPlayerCensusFloor guards a vacuous census: 25 carrier lines over 9
// spellings measured at the 2026-10-12 pin.
const definedPlayerCensusFloor = 20

var checkDefinedPlayerRe = regexp.MustCompile(`CheckDefinedPlayer\$ ([^|]*)`)

// corpusDefinedPlayerSpecs collects every CheckDefinedPlayer$ value on a
// trigger, a replacement or an SVar-held trigger body (Dragon Cultist's
// granted trigger, Miss Highwater's effect trigger), with a carrier count.
func corpusDefinedPlayerSpecs(t *testing.T) (map[string][]string, int) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	specs := map[string][]string{}
	lines := 0
	add := func(spec, name string) {
		spec = strings.TrimSpace(spec)
		specs[spec] = append(specs[spec], name)
		lines++
	}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			if f == nil {
				continue
			}
			for _, tr := range f.Triggers {
				if v, ok := tr.Param(cards.PKCheckDefinedPlayer); ok {
					add(v, f.Name)
				}
			}
			for _, r := range f.Repls {
				if v, ok := r.Param(cards.PKCheckDefinedPlayer); ok {
					add(v, f.Name)
				}
			}
			keys := make([]string, 0, len(f.SVars))
			for k := range f.SVars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				for _, m := range checkDefinedPlayerRe.FindAllStringSubmatch(f.SVars[k], -1) {
					add(m[1], f.Name)
				}
			}
		}
	}
	return specs, lines
}

func TestCheckDefinedPlayerCorpusCensus(t *testing.T) {
	t.Parallel()
	specs, lines := corpusDefinedPlayerSpecs(t)
	if lines < definedPlayerCensusFloor {
		t.Fatalf("census found %d CheckDefinedPlayer$ carriers, want >= %d (corpus missing?)", lines, definedPlayerCensusFloor)
	}
	keys := make([]string, 0, len(specs))
	for k := range specs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, spec := range keys {
		if !readDefinedPlayerSpecs[spec] && !knownUnsupportedDefinedPlayer[spec] {
			t.Errorf("CheckDefinedPlayer$ %q (%v) is neither read nor listed in knownUnsupportedDefinedPlayer", spec, specs[spec])
		}
	}
	for spec := range knownUnsupportedDefinedPlayer {
		if len(specs[spec]) == 0 {
			t.Errorf("knownUnsupportedDefinedPlayer entry %q has no corpus carrier any more: delete it", spec)
		}
	}
}

// TestCheckDefinedPlayerUnreadFailsClosed: a spelling no home reads holds for
// nobody, whatever the base, on a board where every read predicate holds for
// seat 0 -- so a fail-OPEN evaluator cannot pass.
func TestCheckDefinedPlayerUnreadFailsClosed(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoard(t, reg, "Servant of the Stinger")
	src := ids["Servant of the Stinger"]
	e.emit(events.Event{Kind: events.MonarchChange, Player: 0})
	e.crimeSeatsThisTurn = 1
	if !e.checkDefinedPlayerHolds("You.isMonarch", src, 0) || !e.checkDefinedPlayerHolds("You.committedCrimeThisTurn", src, 0) {
		t.Fatal("precondition: the read predicates must hold for seat 0 on this board")
	}
	unread := []string{"You.noSuchProperty", "Player.noSuchProperty", "Opponent.noSuchProperty", "NoBase.isMonarch", "You", "You."}
	for spec := range knownUnsupportedDefinedPlayer {
		unread = append(unread, spec)
	}
	slices.Sort(unread)
	for _, spec := range unread {
		for _, you := range []state.PlayerID{0, 1} {
			if e.checkDefinedPlayerHolds(spec, src, you) {
				t.Errorf("unread CheckDefinedPlayer$ %q held for you=%d: must fail closed", spec, you)
			}
		}
	}
}

// definedPlayerTrigger returns the face trigger carrying CheckDefinedPlayer$.
func definedPlayerTrigger(t *testing.T, e *Engine, id state.ObjID) cards.Trigger {
	t.Helper()
	for _, tr := range e.G.Obj(id).Face().Triggers {
		if _, ok := tr.Param(cards.PKCheckDefinedPlayer); ok {
			return tr
		}
	}
	t.Fatalf("%s has no CheckDefinedPlayer$ trigger", e.G.Obj(id).Face().Name)
	return cards.Trigger{}
}

// TestCheckDefinedPlayerServantOfTheStingerCrime pins the carrier: the
// DamageDone trigger's "if you've committed a crime this turn" reads the
// controller's crime ledger, and only the controller's, for this turn only.
func TestCheckDefinedPlayerServantOfTheStingerCrime(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoard(t, reg, "Servant of the Stinger")
	sv := ids["Servant of the Stinger"]
	if e.G.Obj(sv).Zone != state.ZBattlefield || e.controllerOf(sv) != 0 {
		t.Fatal("precondition: Servant of the Stinger must be on seat 0's battlefield")
	}
	tr := definedPlayerTrigger(t, e, sv)
	if tr.Mode != "DamageDone" || actionTriggerModes[tr.Mode] {
		t.Fatalf("precondition: want the non-action DamageDone mode, got %q", tr.Mode)
	}
	if e.CommittedCrimeThisTurn(0) || e.triggerConditionHolds(tr, sv) {
		t.Fatal("no crime this turn: the intervening-if must be false")
	}
	e.crimeSeatsThisTurn = 1 << 1 // only the opponent committed a crime
	if e.triggerConditionHolds(tr, sv) {
		t.Fatal("an opponent's crime must not satisfy \"if you've committed a crime\"")
	}
	e.crimeSeatsThisTurn = 1 << 0
	if !e.triggerConditionHolds(tr, sv) {
		t.Fatal("the controller committed a crime this turn: the intervening-if must hold")
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 1, Amount: 3})
	if e.triggerConditionHolds(tr, sv) {
		t.Fatal("a new turn clears the crime ledger: the intervening-if must be false")
	}
}

// TestCheckDefinedPlayerPreacherWithMostLife: "while you have the most life or
// are tied for most life" on a Mode$ Attacks line.
func TestCheckDefinedPlayerPreacherWithMostLife(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoard(t, reg, "Preacher of the Schism")
	pr := ids["Preacher of the Schism"]
	tr := definedPlayerTrigger(t, e, pr)
	if e.G.Players[0].Life != e.G.Players[1].Life {
		t.Fatal("precondition: equal life totals")
	}
	if !e.triggerConditionHolds(tr, pr) {
		t.Fatal("tied for most life must hold")
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: 5})
	if e.triggerConditionHolds(tr, pr) {
		t.Fatal("an opponent with more life: the condition must be false")
	}
}

// TestCheckDefinedPlayerWithMostTypeOnly: Wild Mammoth / Thoughtbound Primoc's
// "if a player controls more <type> than each other player" needs a UNIQUE
// leader; a shared top count names nobody.
func TestCheckDefinedPlayerWithMostTypeOnly(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoardWith(t, reg, mustCorpusCard(t, reg, "Prodigal Sorcerer"), "Wild Mammoth", "Thoughtbound Primoc", "Prodigal Sorcerer")
	wm, tp := ids["Wild Mammoth"], ids["Thoughtbound Primoc"]
	wmT, tpT := definedPlayerTrigger(t, e, wm), definedPlayerTrigger(t, e, tp)
	if !e.triggerConditionHolds(wmT, wm) || !e.triggerConditionHolds(tpT, tp) {
		t.Fatal("seat 0 alone controls creatures and a Wizard (Prodigal Sorcerer): both must hold")
	}
	var other state.ObjID
	for i := range e.G.Objs {
		if o := &e.G.Objs[i]; o.Owner == 1 && o.Face() != nil && o.Face().Name == "Prodigal Sorcerer" {
			other = o.ID
		}
	}
	if other == 0 {
		t.Fatal("precondition: seat 1 must own a Prodigal Sorcerer")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: other, From: e.G.Obj(other).Zone, To: state.ZBattlefield})
	if e.G.Obj(other).Zone != state.ZBattlefield {
		t.Fatal("precondition: seat 1's Sorcerer must be on the battlefield")
	}
	if e.triggerConditionHolds(tpT, tp) {
		t.Fatal("one Wizard each is a tie: withMostTypeWizardOnly must be false")
	}
	if !e.triggerConditionHolds(wmT, wm) {
		t.Fatal("seat 0 still controls more creatures: withMostTypeCreatureOnly must hold")
	}
}

// TestCheckDefinedPlayerMissHighwaterRememberedCounters: the effect's state
// trigger reads the source's remembered player and that player's counters.
func TestCheckDefinedPlayerMissHighwaterRememberedCounters(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoard(t, reg, "Miss Highwater")
	src := ids["Miss Highwater"]
	const spec = "Player.IsRemembered+counters_EQ0_Contract"
	if e.checkDefinedPlayerHolds(spec, src, 0) {
		t.Fatal("nobody remembered: must be false")
	}
	e.G.Obj(src).Remembered = []state.Target{{Player: 1, IsPlayer: true}}
	if !e.checkDefinedPlayerHolds(spec, src, 0) {
		t.Fatal("remembered seat 1 without a contract counter: must hold")
	}
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 1, Counter: "Contract", Amount: 1})
	if e.G.Players[1].Counter("Contract") != 1 {
		t.Fatal("precondition: seat 1 must have a contract counter")
	}
	if e.checkDefinedPlayerHolds(spec, src, 0) {
		t.Fatal("remembered seat 1 now has a contract counter: must be false")
	}
}

// TestCheckDefinedPlayerJaredMonarchReplacement: the replacement gate shares
// the trigger evaluator, so "while you're the monarch" applies exactly while
// the controller is the monarch (it used to never apply).
func TestCheckDefinedPlayerJaredMonarchReplacement(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoard(t, reg, "Jared Carthalion, True Heir")
	jc := ids["Jared Carthalion, True Heir"]
	var r cards.Repl
	found := false
	for _, rr := range e.G.Obj(jc).Face().Repls {
		if _, ok := rr.Param(cards.PKCheckDefinedPlayer); ok {
			r, found = rr, true
		}
	}
	if !found {
		t.Fatal("precondition: Jared must carry the CheckDefinedPlayer$ replacement")
	}
	if e.replacementConditionHolds(r, jc, 0) {
		t.Fatal("no monarch: the replacement must not apply")
	}
	e.emit(events.Event{Kind: events.MonarchChange, Player: 0})
	if !e.replacementConditionHolds(r, jc, 0) {
		t.Fatal("controller is the monarch: the replacement must apply")
	}
	e.emit(events.Event{Kind: events.MonarchChange, Player: 1})
	if e.replacementConditionHolds(r, jc, 0) {
		t.Fatal("the opponent is the monarch: the replacement must not apply")
	}
}
