package main

// SCRATCH diagnostic for the cmdr4 object-cap seeds -- not part of the fix,
// deleted before commit. Plays a seed with the same seating and drive loop as
// mtgsim's driveMatch, optionally disarming the object cap, and reports the
// live-object tally (state.Game.LiveObjs) peak and the game's outcome.

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestScratchPeak(t *testing.T) {
	names := []string{"valgavoth-endless-punishment", "hearthhull-worldseed-landfall", "b3-hearthhull-the-worldseed", "b3-cloud-ex-soldier"}
	policies := []string{"forge-heuristic", "lethal-pressure", "bot", "bot"}
	mulls := 0
	seed := uint64(78391)
	if os.Getenv("GORGE_DIAG_78411") == "1" {
		names = []string{"b3-flubs-the-fool", "b3-gavi-nest-warden", "b3-ulalek-fused-atrocity", "b1-ruxa-patient-professor"}
		policies = []string{"lethal-pressure", "lethal-pressure", "forge-heuristic-hold", "bot"}
		mulls = 1
		seed = 78411
	}
	reg := testutil.CorpusRegistry(t)
	decks := make([][]*cards.Card, len(names))
	var cmds [][]int
	o := simOptions{}
	for i, name := range names {
		f, err := o.deckFile(name)
		if err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, f.CommanderIndices())
		cs, err := f.Resolve(reg)
		if err != nil {
			t.Fatal(err)
		}
		decks[i] = cs
	}
	cfg := gameConfig(seed, names, decks, reg, rules.FormatCommander, cmds, mulls)
	cfg.LoopGuard = nil // the scratch run disarms the cap to measure the peak
	e := rules.NewStartingPlayerChoice(cfg)
	e.AskStartingPlayer()
	mseats, err := matchSeats(e, reg, seed, names, policies, "commander", mulls, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	route := newSeatRoute(cfg, mseats)
	log, err := os.Create(fmt.Sprintf("/home/sadams/projects/gorge/.worktrees/hunt-cmdr4-livelock-livelock-detected-object-cap-ob/.ds4/scratch/peak-%d.log", seed))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()

	peak, peakN := 0, 0
	n := 0
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(log, "PANIC: %v (tally=%d arena=%d)\n", r, e.G.LiveObjs(), len(e.G.Objs))
		}
		fmt.Fprintf(log, "final: n=%d turn=%d over=%v winner=%d draw=%v tally=%d arena=%d peakTally=%d at n=%d\n",
			n, e.G.Turn, e.G.Over, e.G.Winner, e.G.Draw, e.G.LiveObjs(), len(e.G.Objs), peak, peakN)
	}()
	ctx := t.Context()
	for !e.G.Over && e.Pending() != nil && n < maxIntents {
		if got := e.G.LiveObjs(); got > peak {
			peak, peakN = got, n
		}
		if n%2000 == 0 {
			fmt.Fprintf(log, "progress n=%d seq=%d turn=%d tally=%d arena=%d stack=%d\n",
				n, len(e.L.Events), e.G.Turn, e.G.LiveObjs(), len(e.G.Objs), len(e.G.Stack))
		}
		d := e.Pending()
		route.observe(e)
		if consumer, ok := mseats[int(d.Player)].(engineseat.PaymentPlanConsumer); ok && consumer.WantsPaymentActions() {
			e.EnsurePaymentActions()
		}
		if consumer, ok := mseats[int(d.Player)].(engineseat.TargetHintConsumer); ok && consumer.WantsTargetHints() {
			e.EnsureTargetHints()
		}
		in, err := route.decide(ctx, e, mseats[int(d.Player)], d)
		if err != nil {
			t.Fatalf("bot error at %d: %v", n, err)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("intent %d rejected: %v", n, err)
		}
		n++
	}
}
