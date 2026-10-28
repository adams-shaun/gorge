package rules

import (
	"sort"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// maxSetupSpeed is the top of a seat's speed (CR 702.179b); events.Apply
// clamps to it, so a larger setup value is a scenario typo, not a request.
const maxSetupSpeed = 4

// validateSetupState rejects a seat's counters/speed that the runner cannot
// place: a counters key naming no battlefield card of that seat, a kind with
// no name, a non-positive amount, or a speed outside 0..4. It runs before the
// game is built so a typo fails loudly rather than silently placing nothing.
func validateSetupState(p int, s oracleSeat) error {
	for _, name := range sortedSetupKeys(s.Counters) {
		onField := false
		for _, n := range s.Battlefield {
			onField = onField || cards.NormalizeName(n) == cards.NormalizeName(name)
		}
		if !onField {
			return harnessf("setup: p%d counters name %q, which is not on its battlefield", p, name)
		}
		for _, kind := range sortedSetupKeys(s.Counters[name]) {
			if kind == "" || s.Counters[name][kind] <= 0 {
				return harnessf("setup: p%d counters %q/%q: want a named kind and a positive amount", p, name, kind)
			}
		}
	}
	if err := validateTappedAfterUntap(p, s); err != nil {
		return err
	}
	if s.Speed < 0 || s.Speed > maxSetupSpeed {
		return harnessf("setup: p%d speed %d (want 0..%d)", p, s.Speed, maxSetupSpeed)
	}
	if err := validateSetupAttached(p, s); err != nil {
		return err
	}
	return validateSetupTokens(p, s)
}

// emitSetupSpeed raises the seat's speed to its setup value with one
// SpeedChange event (a fresh seat starts at 0).
func emitSetupSpeed(emit func(events.Event) events.Event, p int, current int32, s oracleSeat) {
	if d := s.Speed - current; d != 0 {
		emit(events.Event{Kind: events.SpeedChange, Player: state.PlayerID(p), Amount: d})
	}
}

// sortedSetupKeys returns m's keys in sorted order, so no map range reaches
// an event.
func sortedSetupKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// validateTappedAfterUntap rejects a tapped_after_untap entry that is empty,
// carries a "pN:" prefix (it is seat-relative), or names a non-token card the
// seat does not place on its battlefield.
func validateTappedAfterUntap(p int, s oracleSeat) error {
	for _, entry := range s.TappedAfterUntap {
		if entry == "" {
			return harnessf("setup: p%d tapped_after_untap has an empty entry", p)
		}
		if _, _, _, _, _, err := splitRef(entry); err == nil {
			return harnessf("setup: p%d tapped_after_untap %q must be seat-relative (no pN: prefix)", p, entry)
		}
		_, name, token, _, _, err := splitRef(oracleSeatRef(p, entry))
		if err != nil {
			return err
		}
		if token {
			continue
		}
		onField := false
		for _, n := range s.Battlefield {
			onField = onField || cards.NormalizeName(n) == cards.NormalizeName(name)
		}
		if !onField {
			return harnessf("setup: p%d tapped_after_untap %q is not on its battlefield", p, entry)
		}
	}
	return nil
}

// tapAfterUntap taps each seat's tapped_after_untap permanents, in seat then
// list order, once the drive is in the scenario turn's first main phase. A
// permanent already tapped is left alone. Every trigger the taps queued is
// dropped: the tap stands in for an earlier action setup never plays.
func (r *oracleRun) tapAfterUntap(sc oracleScenario, seats int) error {
	e := r.e
	pendingBefore := len(e.pendingTriggers)
	for p := 0; p < seats; p++ {
		for _, entry := range sc.Setup[oracleSeatRef(p, "")].TappedAfterUntap {
			id, err := r.resolve(oracleSeatRef(p, entry))
			if err != nil {
				return err
			}
			o := e.G.Obj(id)
			if o == nil || o.Zone != state.ZBattlefield || o.Controller != state.PlayerID(p) {
				return harnessf("setup: p%d tapped_after_untap %q is not a permanent p%d controls", p, entry, p)
			}
			if o.Tapped {
				continue
			}
			e.emit(events.Event{Kind: events.Tap, Obj: id})
		}
	}
	if len(e.pendingTriggers) > pendingBefore {
		e.pendingTriggers = e.pendingTriggers[:pendingBefore]
	}
	e.orderedTriggers = min(e.orderedTriggers, len(e.pendingTriggers))
	return nil
}
