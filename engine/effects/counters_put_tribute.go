// PutCounter's Tribute$ shape (CR 702.104, kw_tribute.go's expansion): the
// two asks Forge's CountersPutEffect.java:493 poses when a tribute put
// resolves -- the entering creature's controller picks ONE opponent, then
// the chosen opponent elects to pay the tribute (let the creature enter with
// the counters) or decline. An acceptance folds the events.AlterAttribute
// "Tributed" designation onto the permanent and places the counters with the
// paying opponent as the counter adder; a decline (or an unanswerable ask,
// the R-9 stand-in) places nothing, so the "if tribute wasn't paid" ETB
// trigger the same expansion minted fires.

package effects

import (
	"strconv"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func putCounterTribute(h Host, c *Ctx, sa *cards.SA, pc *PutCounterParams, n int32) {
	g := h.Game()
	kind := pc.Kind
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil {
			continue
		}
		name := "the creature"
		if o.Face() != nil {
			name = o.Face().Name
		}
		// Ask 1: the entering creature's controller picks one opponent
		// (Forge's chooseSingleEntityForEffect over activator.getOpponents()).
		opponents := tributeOpponents(h, c)
		if len(opponents) == 0 {
			// Nobody to offer the tribute to: unpaid by construction.
			continue
		}
		pick := opponents[0]
		d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose,
			Min: 1, Max: 1, Source: c.Source, ResumeKind: "tribute_payer",
			ResumeSA: sa, Prompt: "Choose an opponent to offer the tribute"}
		for _, p := range opponents {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "player", Label: g.Players[p].Name, Player: p})
		}
		// R-9 no-host stand-in: the first opponent in seat order -- the exact
		// pick botpolicy's clamp fallback answers, so a bot-answered ask
		// emits the same events this silent path does.
		if ans, ok := AskTape(h, d); ok && len(ans) > 0 {
			pick = ans[0].Player
		}
		if int(pick) >= len(g.Players) {
			continue
		}
		// Ask 2: the chosen opponent elects pay/decline (Forge's
		// confirmAction(PlayerActionConfirmMode.Tribute)).
		d2 := &decision.Decision{Player: pick, Kind: decision.KChoose,
			Min: 1, Max: 1, Source: c.Source, ResumeKind: "tribute",
			ResumeSA: sa,
			Prompt:   "Let " + name + " enter with " + strconv.Itoa(int(n)) + " +1/+1 counters? (pay tribute)"}
		d2.Options = []decision.Option{
			{Index: 0, Kind: "yes", Label: "Yes — pay the tribute", Player: pick},
			{Index: 1, Kind: "no", Label: "No", Player: pick},
		}
		if ans, ok := AskTape(h, d2); ok {
			if len(ans) == 0 || ans[0].Kind != "yes" {
				// A decline places nothing and folds nothing: the tribute is
				// unpaid, and the expansion's ETB trigger fires.
				continue
			}
		} else {
			// No answer: the deterministic decline stand-in (R-9) — the same
			// class the put_optional election falls back to.
			continue
		}
		// Acceptance: the designation first (Forge's setTributed(true)
		// precedes the counter add), then the counters, attributed to the
		// paying opponent the way a Placer$ grant is.
		h.Emit(events.Event{Kind: events.AlterAttribute, Obj: o.ID, Player: pick,
			Text: "Tributed", Amount: 1})
		prev := h.SetCounterAdder(pick)
		h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID,
			Counter: kind, Amount: n})
		h.SetCounterAdder(prev)
	}
}

// tributeOpponents is the entering creature's controller's living opponents
// (team-aware, in seat order).
func tributeOpponents(h Host, c *Ctx) []state.PlayerID {
	g := h.Game()
	var out []state.PlayerID
	for _, p := range g.AliveFrom(c.Controller) {
		if p != c.Controller && MatchesPlayerSpecWithSVars(h, c, "Opponent", p, c.Controller) {
			out = append(out, p)
		}
	}
	return out
}
