package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// findMode returns the index of the option carrying mode in opts, or -1.
func findMode(opts []decision.Option, mode string) int {
	for i, o := range opts {
		if o.Mode == mode {
			return i
		}
	}
	return -1
}

// plotDriveToStep passes priority (answering nothing else) until turn/active
// /step is reached, the way driveToStep does but tolerating the moments where
// the engine has not yet materialised its next priority decision.
func plotDriveToStep(t *testing.T, e *Engine, turn int32, active state.PlayerID, step state.Step) {
	t.Helper()
	for i := 0; i < 4000; i++ {
		if e.G.Turn == turn && e.G.Active == active && e.G.Step == step {
			return
		}
		if e.G.Over {
			t.Fatalf("game ended before reaching turn %d seat %d step %s", turn, active, step)
		}
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("non-priority decision %+v while driving to turn %d seat %d step %s",
				d, turn, active, step)
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		if pass < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pass}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	t.Fatalf("did not reach turn %d seat %d step %s within the pass budget", turn, active, step)
}

// plotEngine puts Lock and Load in seat 0's hand at main phase with the pool
// the plot cost needs, and asserts the plot offer is on the table (the
// precondition every later assertion depends on).
func plotEngine(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := handEngine(t, corpusAlternativeCard(t, "Lock and Load"))
	lal := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC] = 3
	e.G.Players[0].Pool[state.MU] = 1
	if findMode(e.legalActions(0), "plot") < 0 {
		t.Fatalf("Lock and Load was never offered a plot action: %+v", e.legalActions(0))
	}
	return e, lal
}

func TestPlotLockAndLoadExilesWithTimeCountersAndCastsFree(t *testing.T) {
	e, lal := plotEngine(t)
	castMode(t, e, lal, "plot")
	o := e.G.Obj(lal)
	// CR 701.34a: the action paid {3}{U} and exiled the card with TIME
	// counters equal to its MANA VALUE (Lock and Load is a {2}{U} sorcery,
	// so 3) carrying the action's own provenance flag.
	if o.Zone != state.ZExile || o.Counter("TIME") != 3 || o.CastFlags&state.FlagPlot == 0 {
		t.Fatalf("plot did not exile with 3 TIME counters and FlagPlot: zone=%s time=%d flags=%d",
			o.Zone, o.Counter("TIME"), o.CastFlags)
	}
	if p := e.G.Players[0].Pool; p[state.MC] != 0 || p[state.MU] != 0 {
		t.Fatalf("plot cost was not paid: pool %+v", p)
	}
	if e.Pending() != nil || e.cast != nil {
		t.Fatalf("the plot action is not a cast; nothing should be pending (cast=%v)", e.cast != nil)
	}
	// CR 701.34: one time counter leaves at each of the owner's upkeeps, and
	// the cast is offered only once the last is gone -- an upkeep (or any
	// non-sorcery moment) before that offers nothing.
	e.beginTurn(0)
	if got := e.G.Obj(lal).Counter("TIME"); got != 2 {
		t.Fatalf("first plotted upkeep TIME=%d, want 2", got)
	}
	if findMode(e.legalActions(0), "plot_cast") >= 0 {
		t.Fatal("plotted card offered a free cast while TIME counters remained")
	}
	e.beginTurn(0)
	if got := e.G.Obj(lal).Counter("TIME"); got != 1 {
		t.Fatalf("second plotted upkeep TIME=%d, want 1", got)
	}
	if findMode(e.legalActions(0), "plot_cast") >= 0 {
		t.Fatal("plotted card offered a free cast at TIME 1")
	}
	e.beginTurn(0)
	if got := e.G.Obj(lal).Counter("TIME"); got != 0 {
		t.Fatalf("third plotted upkeep TIME=%d, want 0", got)
	}
	// CR 701.34d's cast is a standing sorcery-timing permission, not
	// Suspend's upkeep cast-if-able ask: the last counter's upkeep queued no
	// decision at all.
	if d := e.Pending(); d != nil {
		t.Fatalf("the last plotted counter posed an upkeep ask: %+v", d)
	}
	plotDriveToStep(t, e, e.G.Turn, 0, state.StepMain1)
	idx := findMode(e.legalActions(0), "plot_cast")
	if idx < 0 {
		t.Fatalf("plotted card with no TIME counters was never offered its free cast: %+v", e.legalActions(0))
	}
	// The permission holds even after a priority round passes it: it is
	// re-offered every priority round (it is not a one-shot ask).
	pass := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "pass" {
			pass = o.Index
		}
	}
	if err := e.Submit(decision.Intent{Seq: e.Pending().Seq, Player: 0, Choices: []int{pass}}); err != nil {
		t.Fatalf("submit pass: %v", err)
	}
	e.Advance()
	if findMode(e.legalActions(0), "plot_cast") < 0 {
		t.Fatal("the plotted cast permission did not survive a passed priority round")
	}
	// The cast is free: the pool is still empty, and the cast still begins.
	castMode(t, e, lal, "plot_cast")
	if o := e.G.Obj(lal); o.Zone != state.ZStack {
		t.Fatalf("accepted plot_cast left the card in %s, want stack", o.Zone)
	}
	if p := e.G.Players[0].Pool; p[state.MC] != 0 || p[state.MU] != 0 {
		t.Fatalf("the plotted cast paid mana: pool %+v", p)
	}
	before := len(e.G.Zone(state.ZHand, 0))
	e.resolveTop()
	if o := e.G.Obj(lal); o.Zone != state.ZGraveyard {
		t.Fatalf("resolved plotted Lock and Load in %s, want graveyard", o.Zone)
	}
	// The spell's own SA resolved: it drew exactly one more card than the
	// pre-resolution hand held (the draw-step draw was already in hand before
	// resolveTop; the X draw is 0 -- no other instant/sorcery cast this turn,
	// the bare !CastSaSource exclusion counting the plot cast itself out).
	if got := len(e.G.Zone(state.ZHand, 0)); got != before+1 {
		t.Fatalf("after the resolved plotted cast the hand holds %d cards, want %d", got, before+1)
	}
}

func TestPlotActionOnlyAtSorceryTiming(t *testing.T) {
	fast := card(t, "Name:Fast Plot\nManaCost:1 U\nTypes:Instant\nK:Plot:1 U\nOracle:x\n")
	e := handEngine(t, fast)
	fastID := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC] = 1
	e.G.Players[0].Pool[state.MU] = 1
	// At main phase the plot action is offered; the plot gate is the
	// engine's sorcery-speed bool, NOT the instant face's own timing.
	if findMode(e.legalActions(0), "plot") < 0 {
		t.Fatal("the instant's plot action was withheld at sorcery timing")
	}
	// Upkeep is not sorcery timing even though the card is an instant.
	e.G.Step = state.StepUpkeep
	if findMode(e.legalActions(0), "plot") >= 0 {
		t.Fatal("the plot action was offered outside sorcery timing")
	}
	// Restore main phase so the payment stage below can run.
	e.G.Step = state.StepMain1
	castMode(t, e, fastID, "plot")
	if o := e.G.Obj(fastID); o.Zone != state.ZExile || o.Counter("TIME") != 2 || o.CastFlags&state.FlagPlot == 0 {
		t.Fatalf("plotted instant exiled wrong: zone=%s time=%d flags=%d", o.Zone, o.Counter("TIME"), o.CastFlags)
	}
}

func TestPlotFreeCastFollowsSorceryTimingWhateverTheFace(t *testing.T) {
	// A plotted card's cast follows sorcery timing even for a face whose own
	// type would allow instant-speed casting: drive the plotted synthetic
	// instant to its final counter and confirm the offer only exists at main
	// phase on the owner's turn.
	fast := card(t, "Name:Fast Plot\nManaCost:1 U\nTypes:Instant\nK:Plot:1 U\nOracle:x\n")
	e := handEngine(t, fast)
	fastID := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC] = 1
	e.G.Players[0].Pool[state.MU] = 1
	castMode(t, e, fastID, "plot")
	e.beginTurn(0)
	e.beginTurn(0)
	if got := e.G.Obj(fastID).Counter("TIME"); got != 0 {
		t.Fatalf("plotted instant upkeep TIME=%d, want 0", got)
	}
	// Still at the final counter's upkeep: no offer (upkeep is not sorcery
	// timing, and the face's instant type must not widen the gate).
	if findMode(e.legalActions(0), "plot_cast") >= 0 {
		t.Fatal("the plotted instant's free cast was offered at upkeep")
	}
	plotDriveToStep(t, e, e.G.Turn, 0, state.StepMain1)
	if findMode(e.legalActions(0), "plot_cast") < 0 {
		t.Fatal("the plotted instant's free cast was never offered at sorcery timing")
	}
}

func TestPlotProvenanceGateBlocksArbitraryExile(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Lock and Load"))
	lal := e.G.Zone(state.ZHand, 0)[0]
	// Exile the card by hand with no plot ACTION: no FlagPlot, no TIME
	// counters -- an arbitrary exiled Plot carrier must never be offered the
	// free cast (the same provenance discipline the Suspend flag carries).
	e.emit(events.Event{Kind: events.MoveZone, Obj: lal, From: state.ZHand, To: state.ZExile})
	o := e.G.Obj(lal)
	if o.Zone != state.ZExile || o.Counter("TIME") != 0 || o.CastFlags&state.FlagPlot != 0 {
		t.Fatalf("precondition: arbitrary exile shape wrong: zone=%s time=%d flags=%d",
			o.Zone, o.Counter("TIME"), o.CastFlags)
	}
	if findMode(e.legalActions(0), "plot_cast") >= 0 {
		t.Fatal("an arbitrarily exiled Plot carrier was offered the plotted cast")
	}
	// And an unmarked exile card never loses TIME counters at upkeep, so it
	// can never drift into the plotted state either.
	e.beginTurn(0)
	if got := e.G.Obj(lal).Counter("TIME"); got != 0 {
		t.Fatalf("unmarked exile card lost a TIME counter at upkeep: %d", got)
	}
}

func TestPlotOfferNeedsPayableCost(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Lock and Load"))
	// The pool is empty: CR 701.34a's action pays {3}{U}, so the offer must
	// be withheld, not stranded into an unpayable cast.
	if findMode(e.legalActions(0), "plot") >= 0 {
		t.Fatal("the plot action was offered with no mana to pay it")
	}
}
