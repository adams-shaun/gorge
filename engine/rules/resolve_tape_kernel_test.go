package rules

// The resolve_kernel_test.go scenarios on the tape kernel, each with its
// kernel-counter and game-outcome assertions.

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/resolve"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runScenarioKernel runs scenario on a fresh kernel fixture, checks a log-only
// replay, and returns the engine and the kernel counter deltas.
func runScenarioKernel(t *testing.T, seats int, seed uint64, scenario func(t *testing.T, e *Engine), srcs ...string) (*Engine, resolve.Stats) {
	t.Helper()
	before := resolve.ReadStats()
	e, cfg := kernelFixture(t, seats, seed, srcs...)
	scenario(t, e)
	st := resolve.ReadStats().Sub(before)
	replayCheck(t, e, cfg)
	return e, st
}

// A single stop-ask: the ask is posed with the tape exhausted, the run
// unwinds, and the answer re-runs the resolution once, served from the tape;
// the rider after it runs.
func TestKernelChooseColorStopAsk(t *testing.T) {
	var life int32
	e, st := runScenarioKernel(t, 2, 9101, func(t *testing.T, e *Engine) {
		life = e.G.Players[0].Life
		tapeCastAndResolve(t, e, "Tape Wash", "U")
	}, tapeChooseColorSrc)
	if st.Served != 1 || st.Posed != 1 || st.Reruns != 1 {
		t.Fatalf("the tape path did not serve the ChooseColor ask: %+v", st)
	}
	if got := e.G.Players[0].Life - life; got != 2 {
		t.Fatalf("life change %d, want 2", got)
	}
}

// A loop with per-iteration asks: k voters cost one first-run pose, then k
// re-runs serving 1+2+...+k answers; the vote's outcome gains life.
func TestKernelVoteLoop(t *testing.T) {
	for _, seats := range []int{2, 4, 8} {
		t.Run(fmt.Sprintf("seats%d", seats), func(t *testing.T) {
			var life int32
			e, st := runScenarioKernel(t, seats, 9200+uint64(seats), func(t *testing.T, e *Engine) {
				life = e.G.Players[0].Life
				tapeCastAndResolve(t, e, "Tape Council", "B")
			}, tapeVoteSrc())
			k := int64(seats)
			if st.Posed != 1 || st.Reruns != k || st.Served != k*(k+1)/2 {
				t.Fatalf("vote over %d seats: %+v", seats, st)
			}
			if got := e.G.Players[0].Life - life; got != 1 && got != 3 {
				t.Fatalf("life change %d, want one ballot's 1 or 3", got)
			}
		})
	}
}

// The card ballot and the player ballot are served from the tape the same
// way: one answer per voter, re-run per voter.
func TestKernelCardAndPlayerBallots(t *testing.T) {
	for _, seats := range []int{2, 4} {
		t.Run(fmt.Sprintf("card%d", seats), func(t *testing.T) {
			var bear, angel state.ObjID
			e, st := runScenarioKernel(t, seats, 9950+uint64(seats), func(t *testing.T, e *Engine) {
				bear = moveByName(t, e, 0, "ParentLink Bear", state.ZBattlefield)
				angel = moveByName(t, e, 0, "ParentLink Angel", state.ZBattlefield)
				tapeCastAndResolve(t, e, "Tape Judgment", "W")
			}, tapeCardVoteSrc, ptResumeBearSrc, ptResumeAngelSrc)
			if st.Served != int64(seats*(seats+1)/2) {
				t.Fatalf("card ballot: %+v", st)
			}
			if e.G.Obj(bear).Zone != state.ZExile && e.G.Obj(angel).Zone != state.ZExile {
				t.Fatal("the card ballot exiled neither candidate")
			}
		})
		t.Run(fmt.Sprintf("player%d", seats), func(t *testing.T) {
			_, st := runScenarioKernel(t, seats, 9960+uint64(seats), func(t *testing.T, e *Engine) {
				tapeCastAndResolve(t, e, "Tape Verdict", "R")
			}, tapePlayerVoteSrc)
			if st.Served != int64(seats*(seats+1)/2) {
				t.Fatalf("player ballot: %+v", st)
			}
		})
	}
}

// A root target, a targeting link, an intervening
// TgtChoose discard and a ParentTarget reader; ParentTarget reads the right
// link after the served discard.
func TestKernelParentTargetChain(t *testing.T) {
	var bear, angel state.ObjID
	e, st := runScenarioKernel(t, 2, 9301, func(t *testing.T, e *Engine) {
		bear = moveByName(t, e, 0, "ParentLink Bear", state.ZBattlefield)
		angel = moveByName(t, e, 0, "ParentLink Angel", state.ZBattlefield)
		spell := fixtureInHand(t, e, "Parent Link Between")
		addMana(t, e, 0, "R")
		submitChoices(t, e, castOptionFor(t, e, spell).Index)
		driveParentLink(t, e, bear, angel)
	}, parentTargetBetweenScript(), ptResumeBearSrc, ptResumeAngelSrc)
	if !e.HasKeyword(angel, "Flying") || e.HasKeyword(bear, "Flying") {
		t.Fatal("ParentTarget read the wrong link under the tape kernel")
	}
	if st.Served != 1 {
		t.Fatalf("the discard ask was not served from the tape: %+v", st)
	}
}

// Every player discards a card of their choice (TgtChoose over Defined$
// Player): the per-player asks are served and every hand shrinks by one.
func TestKernelEachPlayerDiscardLoop(t *testing.T) {
	e, st := runScenarioKernel(t, 4, 9701, func(t *testing.T, e *Engine) {
		tapeCastAndResolve(t, e, "Tape Each Discard", "B")
	}, tapeEachDiscardSrc)
	if st.Served == 0 {
		t.Fatalf("no discard ask was served from the tape: %+v", st)
	}
	var discards [4]int
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Text == "discarded" {
			discards[e.G.Obj(ev.Obj).Owner]++
		}
	}
	if discards != [4]int{1, 1, 1, 1} {
		t.Fatalf("discards per seat %v, want one each", discards)
	}
}

// A clone of a posed tape resolution shares S0: answering the clone re-runs
// from the same checkpoint and leaves the original untouched, and both end
// identical.
func TestKernelCloneAtPosedDecision(t *testing.T) {
	e, _ := kernelFixture(t, 4, 9801, tapeVoteSrc())
	addMana(t, e, 0, "B")
	submitChoices(t, e, castOptionFor(t, e, fixtureInHand(t, e, "Tape Council")).Index)
	for e.Pending().Kind == decision.KPriority {
		submitChoices(t, e, tapePassIndex(e.Pending()))
	}
	if !TapePosed(e) {
		t.Fatal("precondition: no tape resolution posed")
	}
	c := e.Clone()
	if !TapePosed(c) {
		t.Fatal("the clone lost the posed checkpoint")
	}
	headBefore, nBefore := e.L.Head(), len(e.L.Events)
	tapeResolveAll(t, c)
	if e.L.Head() != headBefore || len(e.L.Events) != nBefore || !TapePosed(e) {
		t.Fatal("answering the clone changed the original")
	}
	tapeResolveAll(t, e)
	tapeRequireSameLog(t, c, e)
}

// An ask-free resolution skips the checkpoint; one that asks still takes it
// and is served from the tape.
func TestKernelMayAskExemptsAskFreeResolutions(t *testing.T) {
	_, st := runScenarioKernel(t, 2, 9201, func(t *testing.T, e *Engine) {
		tapeCastAndResolve(t, e, "Tape Gain", "W")
	}, tapeNoAskSrc)
	if st.Checkpoints != 0 || st.Exempt != 1 || st.Misses != 0 {
		t.Fatalf("the ask-free spell was not exempted: %+v", st)
	}
	_, st = runScenarioKernel(t, 2, 9202, func(t *testing.T, e *Engine) {
		tapeCastAndResolve(t, e, "Tape Wash", "U")
	}, tapeChooseColorSrc)
	if st.Checkpoints != 1 || st.Served != 1 {
		t.Fatalf("the asking spell lost its checkpoint: %+v", st)
	}
}

// A predicate miss (forced here by exempting everything): the resolution
// takes no checkpoint, so its ask reaches Engine.Ask with no run serving it.
// It is refused with one replay-visible Note and the asking primitive takes
// its deterministic default, reported to the legacy-ask
// observer; the spell still finishes resolving.
func TestKernelMayAskMissDegradesInPlace(t *testing.T) {
	tapeExemptAll = true
	t.Cleanup(func() { tapeExemptAll = false })
	var classes []string
	prev := SetTapeLegacyObserver(func(class string) { classes = append(classes, class) })
	t.Cleanup(func() { SetTapeLegacyObserver(prev) })
	var life int32
	e, st := runScenarioKernel(t, 2, 9203, func(t *testing.T, e *Engine) {
		life = e.G.Players[0].Life
		tapeCastAndResolve(t, e, "Tape Wash", "U")
	}, tapeChooseColorSrc)
	if st.Checkpoints != 0 || st.Exempt != 1 || st.Served != 0 {
		t.Fatalf("the forced exemption did not hold: %+v", st)
	}
	if len(classes) != 1 || !strings.HasPrefix(classes[0], "switch choose/choosecolor") {
		t.Fatalf("legacy-ask classes %q", classes)
	}
	if !hasNote(e, "ask answered with its default: no resolution run serves it (choose/choosecolor)") {
		t.Fatal("the unserved ask left no Note")
	}
	chose := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Counter == "color" {
			chose = true
		}
	}
	if !chose {
		t.Fatal("the default colour was not chosen")
	}
	if got := e.G.Players[0].Life - life; got != 2 {
		t.Fatalf("life change %d, want 2 (the rider still runs)", got)
	}
}

// Spec §7.3: a hypothetical world forked at a posed tape resolution
// (CloneHypothetical plus a redeal of the caster's hidden zones) re-runs
// without a prefix divergence, and what it does after the fork reads the
// REDEALT world -- the card drawn after the vote is the redealt library's
// top -- while the real engine draws the true top.
func TestKernelHypotheticalRedealWorld(t *testing.T) {
	e, _ := kernelFixture(t, 2, 9901, tapeDrawVoteSrc,
		"Name:Tape Filler A\nManaCost:1\nTypes:Artifact\nOracle:x\n",
		"Name:Tape Filler B\nManaCost:2\nTypes:Artifact\nOracle:x\n",
		"Name:Tape Filler C\nManaCost:3\nTypes:Artifact\nOracle:x\n")
	moveByName(t, e, 0, "Tape Filler A", state.ZLibrary)
	moveByName(t, e, 0, "Tape Filler B", state.ZLibrary)
	moveByName(t, e, 0, "Tape Filler C", state.ZLibrary)
	addMana(t, e, 0, "B")
	submitChoices(t, e, castOptionFor(t, e, fixtureInHand(t, e, "Tape Draw Vote")).Index)
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d.Kind == decision.KChoose && d.Player == 1 {
			break
		}
		if d.Kind == decision.KPriority {
			submitChoices(t, e, tapePassIndex(d))
			continue
		}
		submitChoices(t, e, 0)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Player != 1 {
		t.Fatalf("precondition: seat 1's vote not posed: %+v", d)
	}
	if !TapePosed(e) {
		t.Fatal("precondition: no tape resolution posed")
	}
	trueHead := e.L.Head()
	w := e.CloneHypothetical(0xfeed)
	tapeTestRedeal(w, 0, 77)
	wantHand := append([]state.ObjID(nil), w.G.Zone(state.ZHand, 0)...)
	wantTop := w.G.Zone(state.ZLibrary, 0)[0]
	trueTop := e.G.Zone(state.ZLibrary, 0)[0]
	if wantTop == trueTop {
		t.Fatal("precondition: the redeal kept the true top card")
	}
	if err := w.SubmitHypothetical(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{1}}); err != nil {
		t.Fatalf("world submit: %v", err)
	}
	if e.L.Head() != trueHead {
		t.Fatal("the world's re-run touched the real engine")
	}
	gotHand := w.G.Zone(state.ZHand, 0)
	if len(gotHand) != len(wantHand)+1 || gotHand[len(gotHand)-1] != wantTop {
		t.Fatalf("post-fork draw read %v, want the redealt top %d (true top %d)", gotHand, wantTop, trueTop)
	}
	submitChoices(t, e, 1)
	if h := e.G.Zone(state.ZHand, 0); h[len(h)-1] != trueTop {
		t.Fatalf("real engine drew %d, want %d", h[len(h)-1], trueTop)
	}
}

// TestHeads with every resolution checkpointed (the restore and drop paths at
// full rate) must still produce the pinned golden heads. Not parallel: it
// flips a process-wide default.
func TestKernelHeadsCheckpointAll(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	tapeCheckpointAll = true
	// t.Cleanup runs after the parallel subtests, so the flag spans them all
	// (a defer would restore it before they start).
	t.Cleanup(func() { tapeCheckpointAll = false })
	before := resolve.ReadStats()
	t.Cleanup(func() {
		st := resolve.ReadStats().Sub(before)
		if st.Checkpoints == 0 {
			t.Fatalf("the head games never took a checkpoint: %+v", st)
		}
	})
	// One parallel subtest per seat count: the games are independent and the
	// flag above spans them all.
	for _, seats := range AcceptanceSeatCounts() {
		seats := seats
		t.Run(fmt.Sprintf("%dseats", seats), func(t *testing.T) {
			t.Parallel()
			if got, want := acceptanceHead(t, reg, seats), pinnedHead(t, seats); got != want {
				t.Errorf("%d seats: checkpoint-all head %s, golden %s", seats, got, want)
			}
		})
	}
}

// §7.3 at scale: the clone-fidelity fuzz, and at every posed tape decision a
// hypothetical world (CloneHypothetical plus a redeal of every other seat's
// hand and library) driven a few random intents. A prefix divergence panics.
//
// The wall budget is per top-level test, so each of the first six games is its
// own top-level test. The per-decision checks draw from a side stream that
// does not depend on the shard count, so shards=1 plays a game's main line
// once and probes every decision; game 5 (the longest) alone is split in two,
// each half replaying its main line and probing half the decisions.
// GORGE_TAPE_WORLD_GAMES (above 6) sets how many games the Sweep plays: games
// 6..N-1.
const kernelWorldDefaultGames = 6

func TestKernelWorldsInFuzzGame0(t *testing.T)       { kernelWorldsInFuzzGame(t, 0, 1, 0) }
func TestKernelWorldsInFuzzGame1(t *testing.T)       { kernelWorldsInFuzzGame(t, 1, 1, 0) }
func TestKernelWorldsInFuzzGame2(t *testing.T)       { kernelWorldsInFuzzGame(t, 2, 1, 0) }
func TestKernelWorldsInFuzzGame3(t *testing.T)       { kernelWorldsInFuzzGame(t, 3, 1, 0) }
func TestKernelWorldsInFuzzGame4(t *testing.T)       { kernelWorldsInFuzzGame(t, 4, 1, 0) }
func TestKernelWorldsInFuzzGame5Shard0(t *testing.T) { kernelWorldsInFuzzGame(t, 5, 2, 0) }
func TestKernelWorldsInFuzzGame5Shard1(t *testing.T) { kernelWorldsInFuzzGame(t, 5, 2, 1) }

// TestKernelWorldsInFuzzGamesSweep plays games 6..GORGE_TAPE_WORLD_GAMES-1
// under the same world probe; it skips unless the variable is above 6.
func TestKernelWorldsInFuzzGamesSweep(t *testing.T) {
	games := cloneFuzzEnvInt("GORGE_TAPE_WORLD_GAMES", kernelWorldDefaultGames)
	if games <= kernelWorldDefaultGames {
		t.Skip("set GORGE_TAPE_WORLD_GAMES above 6 to play games 6..N-1 under the world probe")
	}
	reg := testutil.CorpusRegistry(t)
	var worlds, steps atomic.Int64
	var st cloneFuzzStats
	o := kernelWorldsOpts(&worlds, &steps, 1, 0)
	t.Run("games", func(t *testing.T) {
		for g := kernelWorldDefaultGames; g < games; g++ {
			g := g
			t.Run(strconv.Itoa(g), func(t *testing.T) {
				t.Parallel()
				cfg, label := cloneFuzzConfig(t, reg, g)
				playCloneFuzzGame(t, cfg, label, o, &st)
			})
		}
	})
	if worlds.Load() == 0 {
		t.Fatal("no posed tape decision was reached: the world probe measured nothing")
	}
	t.Logf("%d games, %d intents; %d tape worlds, %d world intents", st.games.Load(), st.intents.Load(), worlds.Load(), steps.Load())
}

func kernelWorldsInFuzzGame(t *testing.T, g, shards, shard int) {
	t.Parallel()
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	var worlds, steps atomic.Int64
	var st cloneFuzzStats
	o := kernelWorldsOpts(&worlds, &steps, shards, shard)
	cfg, label := cloneFuzzConfig(t, reg, g)
	playCloneFuzzGame(t, cfg, label, o, &st)
	if worlds.Load() == 0 {
		t.Fatal("no posed tape decision was reached: the world probe measured nothing")
	}
	t.Logf("game %d shard %d/%d: %d intents; %d tape worlds, %d world intents", g, shard, shards, st.intents.Load(), worlds.Load(), steps.Load())
}

// kernelWorldsOpts is the world probe: the counters are atomic and each call's
// engine and rand are its own game's, so no package state is shared.
func kernelWorldsOpts(worlds, steps *atomic.Int64, shards, shard int) cloneFuzzOpts {
	o := cloneFuzzOpts{every: 4, lockstep: 4, diverge: 4, randomPct: 10}
	o.shards, o.shard = shards, shard
	o.worldHook = func(t *testing.T, e *Engine, r *rand.Rand) {
		if !TapePosed(e) {
			return
		}
		d := e.Pending()
		w := e.CloneHypothetical(r.Uint64())
		for p := range w.G.Players {
			if state.PlayerID(p) != d.Player {
				tapeTestRedeal(w, state.PlayerID(p), r.Uint64())
			}
		}
		worlds.Add(1)
		for i := 0; i < 6 && !w.G.Over && w.Pending() != nil; i++ {
			wd := w.Pending()
			for try := 0; try < 8; try++ {
				if err := w.SubmitHypothetical(cloneFuzzRandomIntent(wd, r)); err == nil {
					steps.Add(1)
					break
				}
			}
		}
	}
	return o
}

// The shards partition the per-decision checks: every decision is probed by
// exactly one shard, and the main line is the same in each.
func TestKernelWorldsShardsCoverEveryDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	run := func(shards, shard int) (map[uint64]bool, *cloneFuzzStats) {
		cfg, label := cloneFuzzConfig(t, reg, 0)
		seqs := map[uint64]bool{}
		var st cloneFuzzStats
		o := cloneFuzzOpts{maxIntents: 300, every: 4, lockstep: 4, diverge: 4, randomPct: 10, shards: shards, shard: shard}
		o.worldHook = func(t *testing.T, e *Engine, r *rand.Rand) { seqs[uint64(e.Pending().Seq)] = true }
		playCloneFuzzGame(t, cfg, label, o, &st)
		return seqs, &st
	}
	ref, refSt := run(1, 0)
	if len(ref) <= 3 {
		t.Fatalf("reference run probed only %d decisions", len(ref))
	}
	union := map[uint64]bool{}
	var clones int64
	for k := 0; k < 3; k++ {
		s, st := run(3, k)
		for q := range s {
			if union[q] {
				t.Errorf("decision seq %d probed by more than one shard", q)
			}
			union[q] = true
		}
		if st.intents.Load() != refSt.intents.Load() {
			t.Errorf("shard %d played %d intents, reference %d: the main line depends on the shard", k, st.intents.Load(), refSt.intents.Load())
		}
		clones += st.clones.Load()
	}
	if len(union) != len(ref) {
		t.Errorf("shards probed %d decisions, reference %d", len(union), len(ref))
	}
	for q := range ref {
		if !union[q] {
			t.Errorf("decision seq %d missed by every shard", q)
		}
	}
	if clones != refSt.clones.Load() {
		t.Errorf("shard clones sum to %d, reference %d", clones, refSt.clones.Load())
	}
}
