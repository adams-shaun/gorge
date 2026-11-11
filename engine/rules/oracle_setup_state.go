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

// maxSetupCommanders is CR 903.3c's bound: a commander deck has one commander
// or a partner pair, so genesis seats no more whole.
const maxSetupCommanders = 2

// maxSetupCommanderDamage is CR 704.6c's threshold minus one: 21 combat damage
// from one commander loses the game at the first state-based check, so no
// setup can stand on it.
const maxSetupCommanderDamage = 20

// validateSetupCommanders rejects a seat's Commander fields the runner cannot
// place: any of them in a constructed scenario, a name that is not one of the
// seat's non-command placements (or is placed more than once, or is already a
// command placement), more than two commanders, a cast count below 1, or a
// damage amount outside 1..maxSetupCommanderDamage. The damage KEY's existence
// is checked after binding, when the setup refs exist. It runs before the game
// is built so a typo fails loudly rather than silently placing nothing.
func validateSetupCommanders(p int, s oracleSeat, commander bool) error {
	if !commander {
		if len(s.Commanders) > 0 {
			return harnessf("setup: p%d commanders needs a commander-format scenario", p)
		}
		if len(s.CommanderCasts) > 0 {
			return harnessf("setup: p%d commander_casts needs a commander-format scenario", p)
		}
		if len(s.CommanderDamage) > 0 {
			return harnessf("setup: p%d commander_damage needs a commander-format scenario", p)
		}
		return nil
	}
	seen := make([]string, 0, len(s.Commanders))
	for _, name := range s.Commanders {
		key := cards.NormalizeName(name)
		if key == "" {
			return harnessf("setup: p%d commanders has an empty name", p)
		}
		for _, prior := range seen {
			if prior == key {
				return harnessf("setup: p%d commanders lists %q more than once", p, name)
			}
		}
		seen = append(seen, key)
		for _, cmd := range s.Command {
			if cards.NormalizeName(cmd) == key {
				return harnessf("setup: p%d commanders names %q, which is already a command placement", p, name)
			}
		}
		switch n := setupPlacementCount(s, name); {
		case n == 0:
			return harnessf("setup: p%d commanders names %q, which is not one of its placements", p, name)
		case n > 1:
			return harnessf("setup: p%d commanders names %q, which is placed more than once", p, name)
		}
	}
	if n := len(s.Command) + len(s.Commanders); n > maxSetupCommanders {
		return harnessf("setup: p%d has %d commanders (want at most %d; CR 903.3c)", p, n, maxSetupCommanders)
	}
	for _, name := range sortedSetupKeys(s.CommanderCasts) {
		n := s.CommanderCasts[name]
		if n < 1 {
			return harnessf("setup: p%d commander_casts %q = %d (want at least 1; CR 903.8)", p, name, n)
		}
		if !setupIsCommander(s, name) {
			return harnessf("setup: p%d commander_casts names %q, which is not one of its commanders", p, name)
		}
	}
	for _, key := range sortedSetupKeys(s.CommanderDamage) {
		n := s.CommanderDamage[key]
		if n < 1 || n > maxSetupCommanderDamage {
			return harnessf("setup: p%d commander_damage %q = %d (want 1..%d; CR 903.10a)", p, key, n, maxSetupCommanderDamage)
		}
	}
	return nil
}

// setupPlacementCount counts the seat's non-command placements named name. A
// commanders entry must name exactly one of them: zero is unplaced and two is
// ambiguous.
func setupPlacementCount(s oracleSeat, name string) int {
	n := 0
	for _, list := range [][]string{s.Battlefield, s.Hand, s.Graveyard, s.Library, s.LibraryTop, s.Exile} {
		for _, placed := range list {
			if cards.NormalizeName(placed) == cards.NormalizeName(name) {
				n++
			}
		}
	}
	return n
}

// setupCommanderListed reports whether s.Commanders lists name: the placement
// is a commander even though setup moves it out of the command zone.
func setupCommanderListed(s oracleSeat, name string) bool {
	for _, n := range s.Commanders {
		if cards.NormalizeName(n) == cards.NormalizeName(name) {
			return true
		}
	}
	return false
}

// setupIsCommander reports whether name is one of the seat's commanders: a
// command placement or a Commanders entry.
func setupIsCommander(s oracleSeat, name string) bool {
	for _, n := range s.Command {
		if cards.NormalizeName(n) == cards.NormalizeName(name) {
			return true
		}
	}
	return setupCommanderListed(s, name)
}

// setupCommanderCasts builds Config.CommanderCasts, parallel to commanders (the
// Config.Commanders indices in placement order, which is the order
// legalCommandersFor returns). It returns nil when no seat names a count, so
// every other Config is unchanged.
func setupCommanderCasts(sc oracleScenario, seats int, commanders [][]int, decks [][]*cards.Card) [][]int32 {
	var out [][]int32
	for p := 0; p < seats; p++ {
		s := sc.Setup[oracleSeatRef(p, "")]
		if len(s.CommanderCasts) == 0 {
			continue
		}
		if out == nil {
			out = make([][]int32, seats)
		}
		out[p] = make([]int32, len(commanders[p]))
		for k, idx := range commanders[p] {
			if idx < 0 || idx >= len(decks[p]) || decks[p][idx] == nil || len(decks[p][idx].Faces) == 0 {
				continue
			}
			out[p][k] = setupCommanderCast(s.CommanderCasts, decks[p][idx].Faces[0].Name)
		}
	}
	return out
}

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
