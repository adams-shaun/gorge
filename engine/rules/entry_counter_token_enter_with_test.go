package rules

// cr-g102-entry-counters-enter-with: a token created with
// WithCountersType$/WithCountersAmount$ must ENTER the battlefield with those
// counters (CR 701.53a, 122.6, 614.1c), not receive them after the entry
// event. The difference is observable: an entry trigger whose condition reads
// the entering permanent's counters must see them. The other tests here pin
// the neighbouring behaviour that must not change -- the AddCounter
// replacement applies exactly once, and the entry-counter notification still
// fires the "whenever counters are put on" trigger exactly once.

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// entryCounterTokenSA resolves one api:Token with the given entry counters,
// through the engine's own effToken path, against the real corpus token
// registry.
func entryCounterTokenSA(t *testing.T, e *Engine, source state.ObjID, script, kind string, amount int32) {
	t.Helper()
	effects.Resolve(e, &effects.Ctx{Source: source, Controller: 0}, &cards.SA{Kind: "DB", API: "Token",
		Params: map[string]string{
			"TokenScript":        script,
			"TokenOwner":         "You",
			"WithCountersType":   kind,
			"WithCountersAmount": strconv.Itoa(int(amount)),
		}})
}

// TestEntryCounterTokenEntryTriggerSeesItsCounters is the observable
// difference: Ichorplate Golem's "Whenever a creature you control enters, if
// it has one or more oil counters on it, put an oil counter on it" must see
// the oil counter the token enters with. The token therefore ends with two
// oil counters: the entry counter plus the trigger's.
func TestEntryCounterTokenEntryTriggerSeesItsCounters(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := tokenWithCountersEngine(t, reg, "Ichorplate Golem")
	golem := searchMoveByName(t, e, "Ichorplate Golem", state.ZBattlefield)
	if o := e.G.Obj(golem); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Ichorplate Golem %d not on the battlefield (%+v)", golem, o)
	}
	entryCounterTokenSA(t, e, golem, "g_1_1_elf", "OIL", 1)
	drainQueuedTriggers(t, e)

	tok := newestTokenOnBattlefield(t, e)
	if got := tok.Counter("OIL"); got != 2 {
		t.Fatalf("token oil counters = %d, want 2 (1 entering with it, 1 from Ichorplate Golem's entry trigger); the entry trigger did not see the counter the token entered with", got)
	}
	replayCheck(t, e, cfg)
}

// TestEntryCounterEntryTriggerSnapshotSeesCounters: Yorvo's "Then if that
// creature's power is greater than NICKNAME's power" reads the entering
// creature's power from the trigger's snapshot. A token that ENTERS with
// enough +1/+1 counters has that power at the entry event; a token that gets
// them afterwards does not. Yorvo (4/4 from its four entry counters) takes
// one counter for the Elf entering, then a second only when the Elf entered
// with five +1/+1 counters (power 6).
func TestEntryCounterEntryTriggerSnapshotSeesCounters(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := tokenWithCountersEngine(t, reg, "Yorvo, Lord of Garenbrig")
	yorvo := searchMoveByName(t, e, "Yorvo, Lord of Garenbrig", state.ZBattlefield)
	if got := e.G.Obj(yorvo).Counter("P1P1"); got != 4 {
		t.Fatalf("precondition: Yorvo entry counters = %d, want 4", got)
	}
	entryCounterTokenSA(t, e, yorvo, "g_1_1_elf", "P1P1", 5)
	drainQueuedTriggers(t, e)

	tok := newestTokenOnBattlefield(t, e)
	if got := tok.Counter("P1P1"); got != 5 {
		t.Fatalf("token +1/+1 counters = %d, want 5", got)
	}
	if got := e.G.Obj(yorvo).Counter("P1P1"); got != 6 {
		t.Fatalf("Yorvo +1/+1 counters = %d, want 6 (one for the entry, one because the entering 6/6's power beat Yorvo's 4/4); the trigger's snapshot did not see the counters the token entered with", got)
	}
	replayCheck(t, e, cfg)
}

// TestEntryCounterTokenHardenedScalesAppliesOnce: the entry grant settles
// through the AddCounter replacements exactly once, so Hardened Scales turns
// the requested one +1/+1 counter into two, not three.
func TestEntryCounterTokenHardenedScalesAppliesOnce(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := tokenWithCountersEngine(t, reg, "Hardened Scales")
	scales := searchMoveByName(t, e, "Hardened Scales", state.ZBattlefield)
	if o := e.G.Obj(scales); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Hardened Scales %d not on the battlefield (%+v)", scales, o)
	}
	entryCounterTokenSA(t, e, scales, "g_1_1_elf", "P1P1", 1)
	drainQueuedTriggers(t, e)

	tok := newestTokenOnBattlefield(t, e)
	if got := tok.Counter("P1P1"); got != 2 {
		t.Fatalf("token +1/+1 counters = %d, want 2 (Hardened Scales applies once)", got)
	}
	replayCheck(t, e, cfg)
}

// TestEntryCounterTokenCounterAddedTriggerFiresOnce: the entry-counter
// notification still runs the "whenever one or more +1/+1 counters are put on
// a creature you control" trigger once (CR 122.6), so Simic Ascendancy gets
// one growth counter.
func TestEntryCounterTokenCounterAddedTriggerFiresOnce(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := tokenWithCountersEngine(t, reg, "Simic Ascendancy")
	simic := searchMoveByName(t, e, "Simic Ascendancy", state.ZBattlefield)
	if o := e.G.Obj(simic); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Simic Ascendancy %d not on the battlefield (%+v)", simic, o)
	}
	entryCounterTokenSA(t, e, simic, "g_1_1_elf", "P1P1", 1)
	drainQueuedTriggers(t, e)

	tok := newestTokenOnBattlefield(t, e)
	if got := tok.Counter("P1P1"); got != 1 {
		t.Fatalf("token +1/+1 counters = %d, want 1", got)
	}
	if got := e.G.Obj(simic).Counter("GROWTH"); got != 1 {
		t.Fatalf("Simic Ascendancy growth counters = %d, want 1 (one counters-put trigger)", got)
	}
	replayCheck(t, e, cfg)
}
