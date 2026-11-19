package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// payCumulativeUpkeepMatches implements Mode$ PayCumulativeUpkeep ("Whenever
// you pay CARDNAME's cumulative upkeep", Hibernation's End, edh-trigF). The
// triggering event is the unlogged payment-settled proposal rules'
// cumulative-upkeep window hands to the trigger walk when the cost is paid
// (engine/rules/cumulative.go's noteCumulativeUpkeepPaid), exactly like the
// pay-life proposal interceptPayLifeProposal serves Mode$ PayLife: Counter is
// the state.CumulativeUpkeepPaidCounter internal marker (so no ordinary
// counter trigger can match the proposal), Obj the permanent whose upkeep was
// paid, Player the payer, Amount the age paid -- the upkeep's AGE count at
// the settle, what TriggerAmount binds and what the body's Count$
// CardCounters.AGE reads back at the same object. The proposal is never
// emitted, so no counter is ever recorded and the replay log never carries
// the marker. The script's Paid$ arm selects what this marker serves: Paid$
// True (Hibernation's End, Balduvian Fallen) is inherent -- a payment-settled
// marker exists only when the cost was paid -- while Paid$ False (Heart of
// Bogardan, Thought Lash) describes the DECLINE ("when a player doesn't
// pay"), which no marker reaches, so such a line never fires here.
func payCumulativeUpkeepMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.CounterChange || ev.Counter != state.CumulativeUpkeepPaidCounter || ev.Amount <= 0 {
		return false
	}
	if t.ParamStr(cards.PKPaid) == "False" {
		return false
	}
	p := ev.Player
	if int(p) < 0 || int(p) >= len(e.Game().Players) {
		return false
	}
	return eventCardAndPlayerMatch(e, t, source, ev.Obj, p)
}

func init() {
	registerTrigMatcher(payCumulativeUpkeepMatches, "PayCumulativeUpkeep")
	effects.RegisterNonAPI("trig:PayCumulativeUpkeep")
}
