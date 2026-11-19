// kw:Epic (CR 702.50a) behaviour tests: the EDH keyword class ticket's
// Enduring Ideal / Eternal Dominion pair, played as fixtures that mirror the
// printed text (the real corpus cards' primitive completeness is cmd/forgec's
// TestEDHKeywordsSupported; these tests pin the ENGINE behaviour: the
// rest-of-game can't-cast, the self-re-arming upkeep copy, and the copy's
// target re-election).
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const epicIdealSrc = "Name:Test Ideal\nManaCost:5 W W\nTypes:Sorcery\nK:Epic\n" +
	"A:SP$ ChangeZone | Origin$ Library | Destination$ Battlefield | ChangeType$ Enchantment | ChangeNum$ 1 | SpellDescription$ Search your library for an enchantment card and put it onto the battlefield. Then shuffle your library.\nOracle:x\n"

const epicEnchantSrc = "Name:Test Enchant\nManaCost:1 W\nTypes:Enchantment\nOracle:x\n"

// epicDominionSrc mirrors Eternal Dominion's printed TARGET shape (targets an
// opponent, "you may choose a new target for the copy"); the SP body is a
// plain damage so the test reads the copy's effect off seat life totals
// instead of steering a hidden-library search twice.
const epicDominionSrc = "Name:Test Dominion\nManaCost:5 U U\nTypes:Sorcery\nK:Epic\n" +
	"A:SP$ DealDamage | ValidTgts$ Opponent | NumDmg$ 2 | SpellDescription$ Search target opponent's library for an artifact, creature, enchantment, or land card.\nOracle:x\n"

const epicCounterSrc = "Name:Test Counter\nManaCost:U U\nTypes:Instant\n" +
	"A:SP$ Counter | ValidTgts$ Spell | SpellDescription$ Counter target spell.\nOracle:x\n"

// epicBounceSrc is Bilbo's Gambit's "Return target spell to its owner's hand"
// body verbatim: an effect-driven stack exit (moveZoneEvent), which carries no
// Text just like the resolution move does.
const epicBounceSrc = "Name:Test Bounce\nManaCost:1 U\nTypes:Instant\n" +
	"A:SP$ ChangeZone | ValidTgts$ Card.inZoneStack | TgtZone$ Stack | Origin$ Stack | Destination$ Hand | SpellDescription$ Return target spell to its owner's hand.\nOracle:x\n"

// epicHasCastOption reports whether the current priority offer carries a cast
// option for id.
func epicHasCastOption(e *Engine, id state.ObjID) bool {
	d := e.Pending()
	if d == nil {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			return true
		}
	}
	return false
}

// epicDrain drains the stack, answering the asks an Epic copy's resolution
// poses: the hidden-library search pick (passUntilStackEmpty leaves it to the
// caller) and the shapes it already covers.
func epicDrain(t *testing.T, e *Engine, limit int) {
	t.Helper()
	for i := 0; i < limit && !e.G.Over && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining (stack depth %d)", len(e.G.Stack))
		}
		switch {
		case d.Kind == decision.KChoose && d.ResumeKind == "search":
			if len(d.Options) == 0 {
				t.Fatalf("search pick with no option: %+v", d)
			}
			submitChoices(t, e, d.Options[0].Index)
		case d.Kind == decision.KChoose && d.ResumeKind == "search_mayshuffle":
			submitChoices(t, e, d.Options[0].Index)
		case d.Kind == decision.KChoose && (d.ResumeKind == "search_zones" || d.ResumeKind == "search_confirm"):
			choices := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				choices = append(choices, o.Index)
			}
			submitChoices(t, e, choices...)
		case d.Kind == decision.KChoose && d.ResumeKind == "dig":
			submitChoices(t, e)
		default:
			passUntilStackEmpty(t, e, 1)
		}
	}
	if len(e.G.Stack) > 0 {
		t.Fatalf("stack not empty after %d drain rounds: %d left", limit, len(e.G.Stack))
	}
}

// epicEnchantsOnBattlefield counts the fixture enchantment copies in play.
func epicEnchantsOnBattlefield(e *Engine) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Test Enchant" {
			n++
		}
	}
	return n
}

// epicToUpkeep drives to a player's upkeep like driveToStep, but stops at the
// FIRST priority ask of that upkeep (the delayed Epic trigger is already on
// the stack there); the caller drains it with epicDrain.
func epicToUpkeep(t *testing.T, e *Engine, turn int32, active state.PlayerID) {
	t.Helper()
	for i := 0; i < 4000; i++ {
		if e.G.Turn == turn && e.G.Active == active && e.G.Step == state.StepUpkeep {
			return
		}
		if e.G.Over {
			t.Fatalf("game ended before reaching turn %d seat %d upkeep", turn, active)
		}
		if answerIfDiscard(t, e) {
			continue
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("non-priority decision %+v while driving to turn %d seat %d upkeep (at turn %d seat %d step %s)",
				d, turn, active, e.G.Turn, e.G.Active, e.G.Step)
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	t.Fatalf("did not reach turn %d seat %d upkeep within the pass budget", turn, active)
}

// epicPass submits the current priority ask's pass option once.
func epicPass(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority: %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "pass" {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("priority ask with no pass option: %+v", d)
}

// epicDump logs the copy-relevant tail of the event log (debug aid).
func epicDump(t *testing.T, e *Engine) {
	t.Helper()
	for _, ev := range e.L.Events {
		switch ev.Kind {
		case events.MoveZone, events.PutOnStack, events.StackCopy, events.Note,
			events.TriggerPush, events.DelayedRegister, events.DelayedPush:
			t.Logf("DBG kind=%d obj=%d player=%d counter=%q text=%q ids=%v", ev.Kind, ev.Obj, ev.Player, ev.Counter, ev.Text, ev.IDs)
		}
	}
}

// TestEDHKeywordsEpicLocksCastingAndCopiesEachUpkeep: CR 702.50a. Casting the
// Epic spell resolves it normally; afterwards its caster can't cast spells (a
// bear in hand is refused), and at each of their upkeeps a copy of the spell
// goes on the stack and resolves (its SP effect repeats). Exactly ONE copy per
// upkeep: a copy was never cast, so its own resolution re-arms nothing
// (CR 702.50a: the copy is made except for its epic ability).
func TestEDHKeywordsEpicLocksCastingAndCopiesEachUpkeep(t *testing.T) {
	t.Parallel()
	e, cfg, idealID := newFixtureDeck(t, 33, epicIdealSrc, epicEnchantSrc, epicEnchantSrc, epicEnchantSrc, bearSrc)
	bearID := addToHand(t, e, 0, bearSrc)
	addMana(t, e, 0, "WWWWWWWWW") // 9 W: the Epic's 5WW leaves W for the bear's generic
	addMana(t, e, 0, "G")         // funds the bear the precondition asserts castable
	if !epicHasCastOption(e, bearID) {
		t.Fatalf("precondition: the bear must be castable before the Epic spell resolves")
	}
	// Cast the Epic spell; its SP resolves (the enchantment enters), the
	// arming trigger runs with it (the restriction effect and the first
	// upkeep registration). The SP's library search asks which card to take,
	// so the drain is epicDrain's, not castObj's.
	submitChoices(t, e, castOptionFor(t, e, idealID).Index)
	epicDrain(t, e, 40)
	if got := epicEnchantsOnBattlefield(e); got != 1 {
		t.Fatalf("seq %d: want the resolved Epic spell's SP to have fetched 1 enchantment, got %d", len(e.L.Events), got)
	}
	if len(e.G.Delayed) != 1 {
		t.Fatalf("seq %d: want exactly 1 upkeep registration armed, got %d", len(e.L.Events), len(e.G.Delayed))
	}
	if epicHasCastOption(e, bearID) {
		t.Fatalf("seq %d: the Epic caster must not be offered the bear's cast after the spell resolves", len(e.L.Events))
	}
	// Seat 0's next upkeep: exactly one copy goes on the stack and resolves,
	// fetching a second enchantment. Two would mean the copy's own resolution
	// re-armed the engine.
	epicToUpkeep(t, e, e.G.Turn+2, 0)
	epicDrain(t, e, 40)
	if got := epicEnchantsOnBattlefield(e); got != 2 {
		epicDump(t, e)
		t.Fatalf("seq %d: want 1 copy at the first upkeep (2 enchantments total), got %d", len(e.L.Events), got)
	}
	if len(e.G.Delayed) != 1 {
		t.Fatalf("seq %d: the re-arm must leave exactly 1 live registration, got %d", len(e.L.Events), len(e.G.Delayed))
	}
	// Still locked, and the next upkeep copies once again (2 per-upkeep
	// copies would be the exponential the copy gate exists to prevent).
	if epicHasCastOption(e, bearID) {
		t.Fatalf("seq %d: the Epic lock must persist for the rest of the game", len(e.L.Events))
	}
	epicToUpkeep(t, e, e.G.Turn+2, 0)
	epicDrain(t, e, 40)
	if got := epicEnchantsOnBattlefield(e); got != 3 {
		t.Fatalf("seq %d: want exactly 1 copy per upkeep (3 enchantments total), got %d", len(e.L.Events), got)
	}
	replayCheck(t, e, cfg)
}

// TestEDHKeywordsEpicCopyIsNotCastByTheLockedPlayer: the upkeep copy is put
// on the stack, not cast (CR 707.10), so the Epic caster's own can't-cast
// static never consults it and the copies are offered (and resolve) while the
// restriction stands. Pinned by the second copy resolving after the lock is
// already live in TestEDHKeywordsEpicLocksCastingAndCopiesEachUpkeep; this
// test adds the log-level pin: no PutOnStack names the Epic card as a cast.
func TestEDHKeywordsEpicCopyIsNotCastByTheLockedPlayer(t *testing.T) {
	t.Parallel()
	e, cfg, idealID := newFixtureDeck(t, 34, epicIdealSrc, epicEnchantSrc, epicEnchantSrc, bearSrc)
	addMana(t, e, 0, "WWWWWWWWW")
	submitChoices(t, e, castOptionFor(t, e, idealID).Index)
	epicDrain(t, e, 40)
	epicToUpkeep(t, e, e.G.Turn+2, 0)
	epicDrain(t, e, 40)
	n := 0
	for _, ev := range e.L.Events {
		// The Epic card itself was cast once (its own PutOnStack); a SECOND
		// one naming it would be the copy entering the stack as a cast.
		if ev.Kind == events.PutOnStack && ev.Obj == idealID {
			n++
		}
	}
	if n != 1 {
		epicDump(t, e)
		t.Fatalf("seq %d: the Epic card must have been cast exactly once (the copy is put on the stack, not cast), saw %d PutOnStacks", len(e.L.Events), n)
	}
	replayCheck(t, e, cfg)
}

// TestEDHKeywordsEpicCounteredArmsNothing: a countered Epic spell never
// resolved, so neither clause applies -- no restriction, no upkeep copies.
func TestEDHKeywordsEpicCounteredArmsNothing(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 35, nil, []string{epicIdealSrc, epicEnchantSrc, bearSrc}, []string{epicCounterSrc})
	idealID := findCardObj(t, e, 0, "Test Ideal", state.ZHand)
	bearID := findCardObj(t, e, 0, "Bear", state.ZHand)
	counterID := findCardObj(t, e, 1, "Test Counter", state.ZHand)
	addMana(t, e, 0, "WWWWWWWWW")
	addMana(t, e, 0, "G") // funds the bear the precondition asserts castable
	addMana(t, e, 1, "UU")
	if !epicHasCastOption(e, bearID) {
		t.Fatal("precondition: the bear must be castable before the Epic spell resolves")
	}
	// Cast the Epic spell and keep it on the stack; counter it from seat 1.
	// The Ideal has no targets, so the cast goes straight to the stack; seat
	// 0 receives priority first and passes, giving seat 1 the response window.
	submitChoices(t, e, castOptionFor(t, e, idealID).Index)
	epicPass(t, e)
	submitChoices(t, e, castOptionFor(t, e, counterID).Index)
	if d := e.Pending(); d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target ask for the counter: %+v", d)
	} else {
		// Target the Epic spell: it is the only spell on the stack.
		submitChoices(t, e, d.Options[0].Index)
	}
	passUntilStackEmpty(t, e, 20)
	if len(e.G.Delayed) != 0 {
		t.Fatalf("seq %d: a countered Epic must arm no registration, got %d", len(e.L.Events), len(e.G.Delayed))
	}
	if !epicHasCastOption(e, bearID) {
		t.Fatalf("seq %d: a countered Epic must leave its caster able to cast", len(e.L.Events))
	}
	// No copies at the following upkeeps either.
	epicToUpkeep(t, e, e.G.Turn+2, 0)
	epicDrain(t, e, 40)
	if got := epicEnchantsOnBattlefield(e); got != 0 {
		t.Fatalf("seq %d: a countered Epic must copy nothing, saw %d enchantments", len(e.L.Events), got)
	}
	replayCheck(t, e, cfg)
}

// TestEDHKeywordsEpicBouncedArmsNothing: CR 702.50a grants both clauses to a
// spell that RESOLVED. An effect that bounces the Epic spell off the stack
// before it resolves (Bilbo's Gambit's "Return target spell to its owner's
// hand") must arm nothing -- no restriction, no upkeep registrations -- and
// the card stays an ordinary spell: recastable, and its SP unrun.
func TestEDHKeywordsEpicBouncedArmsNothing(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 37, nil, []string{epicIdealSrc, epicEnchantSrc, bearSrc}, []string{epicBounceSrc})
	idealID := findCardObj(t, e, 0, "Test Ideal", state.ZHand)
	bearID := findCardObj(t, e, 0, "Bear", state.ZHand)
	bounceID := findCardObj(t, e, 1, "Test Bounce", state.ZHand)
	addMana(t, e, 0, "WWWWWWWWW")
	addMana(t, e, 0, "G") // funds the bear the precondition asserts castable
	addMana(t, e, 1, "UU")
	if !epicHasCastOption(e, bearID) {
		t.Fatal("precondition: the bear must be castable before the Epic spell resolves")
	}
	// Cast the Epic spell and keep it on the stack; bounce it from seat 1.
	// The Ideal has no targets, so the cast goes straight to the stack; seat 0
	// receives priority first and passes, giving seat 1 the response window.
	submitChoices(t, e, castOptionFor(t, e, idealID).Index)
	epicPass(t, e)
	submitChoices(t, e, castOptionFor(t, e, bounceID).Index)
	if d := e.Pending(); d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target ask for the bounce: %+v", d)
	} else {
		// Target the Epic spell: it is the only spell on the stack.
		submitChoices(t, e, d.Options[0].Index)
	}
	passUntilStackEmpty(t, e, 20)
	if len(e.G.Delayed) != 0 {
		epicDump(t, e)
		t.Fatalf("seq %d: a bounced Epic must arm no registration, got %d", len(e.L.Events), len(e.G.Delayed))
	}
	if !epicHasCastOption(e, bearID) {
		t.Fatalf("seq %d: a bounced Epic must leave its caster able to cast", len(e.L.Events))
	}
	if findCardObj(t, e, 0, "Test Ideal", state.ZHand) == 0 {
		t.Fatalf("seq %d: the bounced Epic spell must be back in its owner's hand", len(e.L.Events))
	}
	// No copies at the following upkeeps either (the SP never resolved, so it
	// fetched no enchantment; a copy at the upkeep would be the arming bug).
	epicToUpkeep(t, e, e.G.Turn+2, 0)
	epicDrain(t, e, 40)
	if got := epicEnchantsOnBattlefield(e); got != 0 {
		epicDump(t, e)
		t.Fatalf("seq %d: a bounced Epic must copy nothing, saw %d enchantments", len(e.L.Events), got)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("seq %d: a bounced Epic must stay un-armed at the next upkeep, got %d", len(e.L.Events), len(e.G.Delayed))
	}
	replayCheck(t, e, cfg)
}

// TestEDHKeywordsEternalDominionCopyRechoosesTarget: CR 702.50a's "You may
// choose a new target for the copy". The original hits seat 1; the first
// upkeep copy is offered a target election and the test re-points it at seat
// 2, whose library the copy's SP then hits.
func TestEDHKeywordsEternalDominionCopyRechoosesTarget(t *testing.T) {
	t.Parallel()
	deck := func(s uint64) Config {
		return seatZeroStart(Config{Seed: s, Names: []string{"a", "b", "c"},
			Decks: [][]*cards.Card{
				append([]*cards.Card{card(t, epicDominionSrc)}, mountainDeck(t, 39)...),
				mountainDeck(t, 40),
				mountainDeck(t, 40),
			},
			Tokens: map[string]*cards.Card{},
		})
	}
	cfg := deck(36)
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	var dominionID state.ObjID
	for _, zone := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, cand := range e.G.Zone(zone, 0) {
			if o := e.G.Obj(cand); o != nil && o.Face() != nil && o.Face().Name == "Test Dominion" {
				dominionID = cand
			}
		}
	}
	if dominionID == 0 {
		t.Fatal("fixture Test Dominion not in seat 0's zones")
	}
	if e.G.Obj(dominionID).Zone != state.ZHand {
		e.emit(events.Event{Kind: events.MoveZone, Obj: dominionID, From: state.ZLibrary, To: state.ZHand})
		e.pending = nil
		e.Advance()
	}
	addMana(t, e, 0, "UUUUUUU")
	l1, l2 := e.G.Players[1].Life, e.G.Players[2].Life
	castObj(t, e, dominionID) // target ask answered with option 0 = seat 1
	if got := e.G.Players[1].Life; got != l1-2 {
		t.Fatalf("seq %d: the original Dominion must hit seat 1 (want %d life, got %d)", len(e.L.Events), l1-2, got)
	}
	if got := e.G.Players[2].Life; got != l2 {
		t.Fatalf("seq %d: the original Dominion must not hit seat 2 (want %d life, got %d)", len(e.L.Events), l2, got)
	}
	// Seat 0's next upkeep: the copy poses its CR 707.10c target election;
	// answer it with seat 2 (NOT the inherited option 0). The trigger and the
	// copy sit on the stack first, so pass priority until the election is up.
	epicToUpkeep(t, e, e.G.Turn+int32(len(cfg.Names)), 0) // 3 seats: seat 0 cycles back in 3 player-turns
	d := e.Pending()
	for i := 0; i < 30 && !(d != nil && d.Kind == decision.KTarget && d.ResumeKind == "copy_targets"); i++ {
		epicPass(t, e)
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind != "copy_targets" {
		t.Fatalf("seq %d: the upkeep copy must pose a target election, got %+v", len(e.L.Events), d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Player == 2 {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("seq %d: the copy election must offer seat 2: %+v", len(e.L.Events), d.Options)
	}
	submitChoices(t, e, idx)
	epicDrain(t, e, 40)
	if got := e.G.Players[2].Life; got != l2-2 {
		t.Fatalf("seq %d: the re-targeted copy must hit seat 2 (want %d life, got %d)", len(e.L.Events), l2-2, got)
	}
	if got := e.G.Players[1].Life; got != l1-2 {
		t.Fatalf("seq %d: the re-targeted copy must not also hit seat 1 (want %d life, got %d)", len(e.L.Events), l1-2, got)
	}
	replayCheck(t, e, cfg)
}
