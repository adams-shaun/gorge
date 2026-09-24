// Command hindsight measures whether bot decisions admit reliable, affordable
// hindsight labels. It is an offline experiment; it changes no playing seat.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/hindsight"
	"github.com/adams-shaun/gorge/internal/searchprobe"
	"github.com/adams-shaun/gorge/internal/searchseat"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type config struct {
	losses, wins, workers, scanLimit     int
	candidateLimit, block, maxRollouts   int
	attempts, maxSubmits                 int
	reliabilityGames, omniscientGames    int
	seed, sampleSeed                     uint64
	cardsDir, outPath, reportPath, pairs string
}

type gameSetup struct {
	pair  string
	setup searchprobe.PublicGame
}

type branch struct {
	index        int
	turn         int32
	kind         decision.Kind
	engine       *rules.Engine
	collector    *searchprobe.Collector
	history      searchprobe.History
	candidates   []hindsight.Candidate
	capHit       bool
	boardSummary string
}

type selectedGame struct {
	id, scanIndex int
	seed          uint64
	actor         state.PlayerID
	outcome       string
	setup         gameSetup
	branches      []branch
	unsupported   string
}

type reliabilityRecord struct {
	Run             bool `json:"run"`
	BestAlternative int  `json:"best_alternative,omitempty"`
	Clear           bool `json:"clear,omitempty"`
	Agree           bool `json:"agree,omitempty"`
}

type decisionRecord struct {
	RecordType  string `json:"record_type"`
	GameID      int    `json:"game_id"`
	Seed        uint64 `json:"seed"`
	Pair        string `json:"pair"`
	Outcome     string `json:"outcome"`
	Turn        int32  `json:"turn"`
	TurnBucket  string `json:"turn_bucket"`
	Kind        string `json:"kind"`
	Seat        int    `json:"seat"`
	Decision    int    `json:"decision_index"`
	ReverseRank int    `json:"reverse_rank"`
	NOptions    int    `json:"n_options"`
	Chosen      int    `json:"chosen_option"`
	CapHit      bool   `json:"option_cap_hit"`
	Board       string `json:"board_summary"`

	Evaluation  hindsight.Evaluation  `json:"evaluation"`
	Reliability reliabilityRecord     `json:"reliability"`
	Omniscient  *hindsight.Evaluation `json:"omniscient_leaked,omitempty"`
	// WallSeconds is retained in memory for the cost report but deliberately
	// excluded from JSONL: labels and diagnostics are byte-identical across
	// worker counts, while wall-clock scheduling cannot be.
	WallSeconds float64 `json:"-"`
	Error       string  `json:"error,omitempty"`
}

type gameResult struct {
	game    selectedGame
	records []decisionRecord
	err     error
}

func run(args []string, stdout, progress io.Writer) error {
	fs := flag.NewFlagSet("hindsight", flag.ContinueOnError)
	fs.SetOutput(progress)
	cfg := config{}
	fs.IntVar(&cfg.losses, "losses", 200, "lost games to measure")
	fs.IntVar(&cfg.wins, "wins", 100, "won games to measure")
	fs.IntVar(&cfg.workers, "workers", 8, "parallel selected games (max 8 recommended)")
	fs.IntVar(&cfg.scanLimit, "scan-limit", 1200, "maximum baseline games scanned to fill outcome quotas")
	fs.IntVar(&cfg.candidateLimit, "candidates", 8, "maximum answers per decision")
	fs.IntVar(&cfg.block, "block", 16, "adaptive rollout block per option")
	fs.IntVar(&cfg.maxRollouts, "max-rollouts", 128, "maximum rollouts per option")
	fs.IntVar(&cfg.attempts, "attempts", 128, "sampler attempts per rollout block")
	fs.IntVar(&cfg.maxSubmits, "max-submits", 5000, "submit cap per rollout and sample replay")
	fs.IntVar(&cfg.reliabilityGames, "reliability-games", 50, "first selected games whose clear labels are rerun")
	fs.IntVar(&cfg.omniscientGames, "omniscient-games", 20, "first selected games also measured with leaked hidden state")
	fs.Uint64Var(&cfg.seed, "seed", 300_000_000, "first held-out game seed")
	fs.Uint64Var(&cfg.sampleSeed, "sample-seed", 0x706e3230, "independent sampler seed")
	fs.StringVar(&cfg.cardsDir, "cards", ".cards", "compiled card corpus")
	fs.StringVar(&cfg.outPath, "out", "", "new deterministic-order JSONL output")
	fs.StringVar(&cfg.reportPath, "report", "", "new Markdown report")
	fs.StringVar(&cfg.pairs, "pairs", "uw-tempo:mono-white-equipment,uw-tempo:mono-blue-tempo,uw-tempo:mono-black-aggro,uw-tempo:mono-red-prowess,uw-tempo:mono-green-stompy", "comma-separated constructed deck pairs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.losses < 0 || cfg.wins < 0 || cfg.losses+cfg.wins == 0 || cfg.workers < 1 || cfg.workers > 8 || cfg.candidateLimit < 2 || cfg.block < 1 || cfg.maxRollouts < cfg.block || cfg.maxRollouts%cfg.block != 0 || cfg.attempts < 1 || cfg.maxSubmits < 1 || cfg.outPath == "" || cfg.reportPath == "" {
		return fmt.Errorf("require outcome games, workers 1..8, candidates>=2, positive budgets, max-rollouts divisible by block, and -out/-report")
	}
	if _, err := os.Stat(cfg.cardsDir); err != nil {
		return fmt.Errorf("card corpus: %w", err)
	}
	for _, path := range []string{cfg.outPath, cfg.reportPath} {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refusing to overwrite %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	setups, err := loadSetups(cfg)
	if err != nil {
		return err
	}

	started := time.Now()
	cpuStart := processCPU()
	total := cfg.losses + cfg.wins
	jobs := make(chan selectedGame)
	results := make(chan gameResult, cfg.workers)
	var wg sync.WaitGroup
	for w := 0; w < cfg.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for g := range jobs {
				recs, err := processGame(g, cfg)
				// Every branch retains an Engine clone and observation history.
				// The ordered result needs only game metadata; release the large
				// branch graph before it enters the all-games slots.
				g.branches = nil
				results <- gameResult{game: g, records: recs, err: err}
			}
		}()
	}
	slots := make([]gameResult, total)
	var collect sync.WaitGroup
	collect.Add(1)
	go func() {
		defer collect.Done()
		for result := range results {
			slots[result.game.id] = result
			fmt.Fprintf(progress, "measured %d/%d: game %d seed %d (%s), %d decisions\n", countFinished(slots), total, result.game.id, result.game.seed, result.game.outcome, len(result.records))
		}
	}()

	losses, wins := 0, 0
	for scan := 0; scan < cfg.scanLimit && (losses < cfg.losses || wins < cfg.wins); scan++ {
		setup := setups[scan%len(setups)]
		actor := state.PlayerID((scan / len(setups)) % 2)
		g, err := captureGame(setup, cfg.seed+uint64(scan), actor, scan, cfg.candidateLimit)
		if err != nil {
			close(jobs)
			wg.Wait()
			close(results)
			collect.Wait()
			return fmt.Errorf("baseline seed %d: %w", cfg.seed+uint64(scan), err)
		}
		keep := g.outcome == "loss" && losses < cfg.losses || g.outcome == "win" && wins < cfg.wins
		if !keep {
			continue
		}
		g.id = losses + wins
		if g.outcome == "loss" {
			losses++
		} else {
			wins++
		}
		fmt.Fprintf(progress, "selected %d/%d: seed %d actor %d %s (%d decisions)\n", losses+wins, total, g.seed, g.actor, g.outcome, len(g.branches))
		jobs <- g
	}
	close(jobs)
	wg.Wait()
	close(results)
	collect.Wait()
	if losses < cfg.losses || wins < cfg.wins {
		return fmt.Errorf("scan limit filled only %d/%d losses and %d/%d wins", losses, cfg.losses, wins, cfg.wins)
	}
	for _, result := range slots {
		if result.err != nil {
			return fmt.Errorf("game %d seed %d: %w", result.game.id, result.game.seed, result.err)
		}
	}

	out, err := os.OpenFile(cfg.outPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	records := orderedRecords(slots)
	for _, rec := range records {
		if err := enc.Encode(rec); err != nil {
			out.Close()
			return err
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	cpu := processCPU() - cpuStart
	wall := time.Since(started).Seconds()
	report, err := renderReport(cfg, slots, records, cpu, wall)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfg.reportPath, []byte(report), 0o644); err != nil {
		return err
	}
	fmt.Fprint(stdout, report)
	return nil
}

func loadSetups(cfg config) ([]gameSetup, error) {
	reg, err := cards.OpenCorpus(cfg.cardsDir)
	if err != nil {
		return nil, err
	}
	var out []gameSetup
	for _, raw := range strings.Split(cfg.pairs, ",") {
		a, b, ok := strings.Cut(strings.TrimSpace(raw), ":")
		if !ok || a == "" || b == "" {
			return nil, fmt.Errorf("bad pair %q", raw)
		}
		da, err := testutil.LoadRepoDeck(reg, a)
		if err != nil {
			return nil, err
		}
		db, err := testutil.LoadRepoDeck(reg, b)
		if err != nil {
			return nil, err
		}
		out = append(out, gameSetup{pair: a + ":" + b, setup: searchprobe.PublicGame{Names: []string{a, b}, Decks: [][]*cards.Card{da, db}, Tokens: reg.Tokens}})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no deck pairs")
	}
	return out, nil
}

func captureGame(setup gameSetup, seed uint64, actor state.PlayerID, scanIndex, candidateLimit int) (selectedGame, error) {
	g := selectedGame{scanIndex: scanIndex, seed: seed, actor: actor, setup: setup}
	cfg := rules.Config{Seed: seed, Names: setup.setup.Names, Decks: setup.setup.Decks, Tokens: setup.setup.Tokens, StartingLife: setup.setup.StartingLife}
	e := rules.New(cfg)
	e.Advance()
	rngs := searchprobe.BotRandoms(seed, len(setup.setup.Names))
	boards := make([]botpolicy.Board, len(setup.setup.Names))
	for i := range boards {
		boards[i] = botpolicy.NewBoard(len(setup.setup.Names))
	}
	feed := searchseat.NewFeed(actor)
	observing := true
	for steps := 0; !e.G.Over; steps++ {
		if steps >= 20000 || e.G.Turn >= 200 {
			return g, fmt.Errorf("baseline stalled at turn %d intent %d", e.G.Turn, steps)
		}
		d := e.Pending()
		if d == nil {
			return g, fmt.Errorf("missing pending decision")
		}
		if observing {
			_, ok := feed.Observe(e)
			if !ok {
				observing = false
				g.unsupported = feed.StopReason()
			}
		}
		b := botpolicy.BoardFromGameInto(e.G, e, d.Player, &boards[d.Player])
		in := botpolicy.Decide(b, d, rngs[d.Player])
		if observing && d.Player == actor {
			cands, capHit, err := hindsight.Candidates(feed.Collector(), e, d, in, candidateLimit)
			if err != nil {
				return g, err
			}
			g.branches = append(g.branches, branch{index: len(g.branches), turn: e.G.Turn, kind: d.Kind, engine: e.Clone(), collector: feed.Collector().Clone(), history: cloneHistory(feed.History()), candidates: cands, capHit: capHit, boardSummary: summarizeBoard(e, actor)})
			if err := feed.RecordAnswer(d, in); err != nil {
				return g, err
			}
		}
		if err := e.Submit(in); err != nil {
			return g, err
		}
	}
	if e.G.Draw {
		g.outcome = "draw"
	} else if e.G.Winner == actor {
		g.outcome = "win"
	} else {
		g.outcome = "loss"
	}
	return g, nil
}

func processGame(g selectedGame, cfg config) ([]decisionRecord, error) {
	records := make([]decisionRecord, 0, len(g.branches))
	for reverseRank, bi := range hindsight.Backward(len(g.branches)) {
		b := &g.branches[bi]
		rec := decisionRecord{RecordType: "hindsight-decision-v1", GameID: g.id, Seed: g.seed, Pair: g.setup.pair, Outcome: g.outcome, Turn: b.turn, TurnBucket: turnBucket(b.turn), Kind: string(b.kind), Seat: int(g.actor), Decision: b.index, ReverseRank: reverseRank, NOptions: len(b.candidates), CapHit: b.capHit, Board: b.boardSummary}
		if len(b.candidates) < 2 {
			rec.Evaluation.SamplerStatus = "no_alternative"
			records = append(records, rec)
			continue
		}
		start := time.Now()
		baseSeed := hindsight.Seed(cfg.sampleSeed, g.seed, uint64(b.index), 0)
		eval, err := evaluateSampled(g, b, cfg, baseSeed)
		rec.WallSeconds = time.Since(start).Seconds()
		if err != nil {
			rec.Error = err.Error()
			records = append(records, rec)
			continue
		}
		rec.Evaluation = eval
		if eval.Clear && g.id < cfg.reliabilityGames {
			rel, err := evaluateSampled(g, b, cfg, baseSeed^0xa0761d6478bd642f)
			if err != nil {
				return records, fmt.Errorf("reliability decision %d: %w", b.index, err)
			}
			rec.Reliability = reliabilityRecord{Run: true, BestAlternative: rel.BestAlternative, Clear: rel.Clear, Agree: rel.Clear && rel.BestAlternative == eval.BestAlternative}
		}
		if g.id < cfg.omniscientGames {
			leaked, err := evaluateOmniscient(b, cfg, baseSeed^0xe7037ed1a0b428db)
			if err != nil {
				return records, fmt.Errorf("omniscient decision %d: %w", b.index, err)
			}
			rec.Omniscient = &leaked
		}
		records = append(records, rec)
	}
	return records, nil
}

func evaluateSampled(g selectedGame, b *branch, cfg config, seed uint64) (hindsight.Evaluation, error) {
	source := func(block, worlds int) ([]searchprobe.World, searchprobe.SampleResult, error) {
		sr, err := searchprobe.Sample(g.setup.setup, b.history, searchprobe.SampleOptions{
			Seed: hindsight.Seed(seed, g.seed, uint64(b.index), uint64(block)), Attempts: cfg.attempts, Worlds: worlds, MaxSubmits: cfg.maxSubmits,
		})
		return sr.Worlds, sr, err
	}
	return hindsight.Evaluate(b.candidates, source, seed, hindsight.EvalOptions{Block: cfg.block, MaxRollouts: cfg.maxRollouts, MaxSubmits: cfg.maxSubmits})
}

func evaluateOmniscient(b *branch, cfg config, seed uint64) (hindsight.Evaluation, error) {
	source := func(_ int, worlds int) ([]searchprobe.World, searchprobe.SampleResult, error) {
		out := make([]searchprobe.World, worlds)
		for i := range out {
			out[i] = searchprobe.World{Engine: b.engine, Observer: b.collector}
		}
		return out, searchprobe.SampleResult{}, nil
	}
	return hindsight.Evaluate(b.candidates, source, seed, hindsight.EvalOptions{Block: cfg.block, MaxRollouts: cfg.maxRollouts, MaxSubmits: cfg.maxSubmits, Clairvoyant: true})
}

func cloneHistory(h searchprobe.History) searchprobe.History {
	out := searchprobe.History{Actor: h.Actor, Frames: append([]searchprobe.Frame(nil), h.Frames...), Answers: make(map[int][]searchprobe.Action, len(h.Answers))}
	for i, actions := range h.Answers {
		out.Answers[i] = append([]searchprobe.Action(nil), actions...)
	}
	return out
}

func summarizeBoard(e *rules.Engine, actor state.PlayerID) string {
	v := view.Project(e.G, e, actor, e.Pending())
	parts := make([]string, 0, len(v.Players))
	for _, p := range v.Players {
		var battlefield []string
		for _, card := range p.Battlefield {
			battlefield = append(battlefield, card.Name)
		}
		sort.Strings(battlefield)
		parts = append(parts, fmt.Sprintf("seat %d life=%d hand=%d battlefield=[%s]", p.ID, p.Life, p.HandSize, strings.Join(battlefield, ", ")))
	}
	return strings.Join(parts, "; ")
}

func turnBucket(turn int32) string {
	switch {
	case turn <= 3:
		return "1-3"
	case turn <= 6:
		return "4-6"
	case turn <= 12:
		return "7-12"
	default:
		return "13+"
	}
}

func orderedRecords(slots []gameResult) []decisionRecord {
	var records []decisionRecord
	for _, result := range slots {
		records = append(records, result.records...)
	}
	return records
}

func countFinished(slots []gameResult) int {
	n := 0
	for _, slot := range slots {
		if slot.game.outcome != "" {
			n++
		}
	}
	return n
}

func processCPU() float64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	return float64(usage.Utime.Sec+usage.Stime.Sec) + float64(usage.Utime.Usec+usage.Stime.Usec)/1e6
}
