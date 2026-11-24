package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// notedCountersPrefix starts the card-notation label an Effect's
// NoteCounterDefined$ writes (Tawnos's Coffin: "Note the number and kind of
// counters that were on that creature"). The note rides the card itself
// through events.CardNoted: a note is card-identity provenance that survives
// the exile (state.Object.Notes), while the counters themselves cease to
// exist when the card leaves the battlefield (CR 122.2). The label is
// "<prefix><seq>:<KIND>=<n>,..." where <seq> is the card's note count when it
// was noted, so a re-note of the same counters is a new entry and the LAST
// entry is always the current one.
const notedCountersPrefix = "NotedCounters#"

// noteEffectCounters notes the counters on each object NoteCounterDefined$
// names. Only `Targeted` (the corpus's one spelling) is modelled; any other
// selector is a loud Note, never a silent empty note.
func noteEffectCounters(h Host, c *Ctx, defined string) {
	if defined == "" {
		return
	}
	if defined != "Targeted" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "unmodelled NoteCounterDefined$ " + defined})
		return
	}
	for _, t := range refTargetUnion(c) {
		o := h.Game().Obj(t.Obj)
		if t.IsPlayer || o == nil {
			continue
		}
		parts := make([]string, 0, len(o.Counters))
		for _, k := range o.Counters {
			if k.N > 0 && !state.InternalCounterMarker(k.Kind) {
				parts = append(parts, k.Kind+"="+strconv.Itoa(int(k.N)))
			}
		}
		h.Emit(events.Event{Kind: events.CardNoted, Obj: o.ID,
			Text: notedCountersPrefix + strconv.Itoa(len(o.Notes)) + ":" + strings.Join(parts, ",")})
	}
}

// appendNotedCounters folds the moved card's last noted counters into its
// battlefield entry (ChangeZone's WithNotedCounters$ True: "return that
// exiled card to the battlefield ... with the noted number and kind of
// counters on it"), so the permanent enters with them (CR 122.6).
func appendNotedCounters(g *state.Game, ev *events.Event, id state.ObjID, to state.Zone) {
	o := g.Obj(id)
	if o == nil || to != state.ZBattlefield || ev.Kind != events.MoveZone {
		return
	}
	for i := len(o.Notes) - 1; i >= 0; i-- {
		rest, ok := strings.CutPrefix(o.Notes[i], notedCountersPrefix)
		if !ok {
			continue
		}
		_, list, _ := strings.Cut(rest, ":")
		for _, part := range strings.Split(list, ",") {
			kind, n, ok := strings.Cut(part, "=")
			amount, err := strconv.Atoi(n)
			if !ok || err != nil {
				continue
			}
			ev.Pairs = append(ev.Pairs, events.EntryCounterPairs(events.EntryCounterGrant{Kind: kind, Amount: int32(amount)})...)
		}
		return
	}
}
