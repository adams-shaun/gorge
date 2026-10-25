package rules

// CR 701.53a, 122.6, 614.1c: an Incubator token is created WITH its +1/+1
// counters, so an "enters with a +1/+1 counter" trigger sees them. The
// pre-fix Incubate path minted the token first and emitted a post-entry
// CounterChange, so a ChangesZone trigger matched at the mint saw zero
// counters and never fired (the token's later counter was not a new entry).
// The Token API path already held its entry check behind a zone batch and
// therefore passed; these two tests pin the difference so neither path can
// regress.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

const entryCounterWatcher = "Name:Entry Counter Watcher\nTypes:Creature Human\nPT:1/1\nT:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Permanent.YouCtrl+counters_GE1_P1P1 | Execute$ TrigPump | TriggerDescription$ Whenever a permanent you control enters with a +1/+1 counter on it, put a +1/+1 counter on CARDNAME.\nSVar:TrigPump:DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1\nOracle:x\n"

func entryCounterWatcherCounters(t *testing.T, e *Engine) int32 {
	t.Helper()
	id := findByName(e, "Entry Counter Watcher", 0)
	if id == 0 {
		t.Fatal("watcher not found")
	}
	return e.G.Obj(id).Counter("P1P1")
}

// TestEntryCounterIncubateEntryTriggerSeesCounters casts Eyes of Gitaxias
// (Incubate 3) and asserts the watcher's entry trigger saw the Incubator's
// three +1/+1 counters. Without the entry fold the watcher stays at zero.
func TestEntryCounterIncubateEntryTriggerSeesCounters(t *testing.T) {
	reg := searchTestRegistry(t)
	watcher := card(t, entryCounterWatcher)
	eyes := lookup(t, reg, "Eyes of Gitaxias")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{watcher, eyes}, []*cards.Card{})
	moveByName(t, e, 0, "Entry Counter Watcher", state.ZBattlefield)
	if got := entryCounterWatcherCounters(t, e); got != 0 {
		t.Fatalf("precondition watcher counters = %d, want 0", got)
	}
	moveByName(t, e, 0, "Eyes of Gitaxias", state.ZHand)
	e.priorityRound()
	addMana(t, e, 0, "CCU")
	castNamed(t, e, "Eyes of Gitaxias")
	passUntilStackEmpty(t, e, 60)
	if got := entryCounterWatcherCounters(t, e); got != 1 {
		t.Fatalf("Incubate: watcher counters = %d, want 1 (entry trigger saw the Incubator's +1/+1 counters)", got)
	}
	replayCheck(t, e, cfg)
}

// TestEntryCounterTokenSpellEntryTriggerSeesCounters is the Token API
// control: a token spell with WithCountersType$ already entered with its
// counters through the held entry check, and must keep doing so after the
// counters move into the mint event itself.
func TestEntryCounterTokenSpellEntryTriggerSeesCounters(t *testing.T) {
	reg := searchTestRegistry(t)
	watcher := card(t, entryCounterWatcher)
	maker := card(t, "Name:Counter Maker\nManaCost:U\nTypes:Sorcery\nA:SP$ Token | TokenScript$ g_1_1_elf | WithCountersType$ P1P1 | WithCountersAmount$ 1 | SpellDescription$ x\nOracle:x\n")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{watcher, maker}, []*cards.Card{})
	moveByName(t, e, 0, "Entry Counter Watcher", state.ZBattlefield)
	moveByName(t, e, 0, "Counter Maker", state.ZHand)
	e.priorityRound()
	addMana(t, e, 0, "U")
	castNamed(t, e, "Counter Maker")
	passUntilStackEmpty(t, e, 60)
	if got := entryCounterWatcherCounters(t, e); got != 1 {
		t.Fatalf("Token spell: watcher counters = %d, want 1 (entry trigger saw the token's +1/+1 counters)", got)
	}
	replayCheck(t, e, cfg)
}
