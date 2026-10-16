package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/draftlab"
	"github.com/adams-shaun/gorge/limited/draftfeat"
	"github.com/adams-shaun/gorge/limited/pickbot"
	"github.com/adams-shaun/gorge/limited/setdata"
)

// rolloutSalt tags the round-robin game seeds: game g of pair (i, j) in
// the pod seeded podSeed is rand.NewPCG(podSeed, rolloutSalt<<32|i<<12|j<<8|g).
const rolloutSalt = 0xD4A7E7

type rolloutOpts struct {
	corpus, set, collation string
	seed                   uint64
	pods, k, iter          int
	league                 []leagueSpec
	leagueFixed            bool
	weights                []float64 // -league-weights, one per -league entry (nil = uniform)
	learner                int       // index of -learner in league (appended last), -1 = none
	learners               int       // learner seats per pod
	picksJSONL             bool
	builder                seatAgent
	table                  string
	pilot                  string
	az                     gateOpts // the az pilot fields registerAZ reads
	allowAZFaults          bool
	workers, maxObjs       int
	out                    string
}

func parseRollout(args []string) (rolloutOpts, error) {
	var o rolloutOpts
	var league, builder, weights, learner string
	fs := flag.NewFlagSet("draftrl rollout", flag.ContinueOnError)
	fs.StringVar(&o.corpus, "corpus", ".cards", "card corpus directory")
	fs.StringVar(&o.set, "set", "FDN", "set code")
	fs.StringVar(&o.collation, "collation", string(setdata.DefaultCollation), "booster collation")
	fs.Uint64Var(&o.seed, "seed", 1_000_000, "first pod seed; pod n uses seed+n (training seeds start at 1,000,000, spec §3.0)")
	fs.IntVar(&o.pods, "pods", 1, "4-seat pods this iteration")
	fs.IntVar(&o.k, "k", 4, "games per pair of seats in each pod's round robin (sides alternate)")
	fs.IntVar(&o.iter, "iter", 0, "iteration number, recorded in the manifest")
	fs.StringVar(&league, "league", "17l", "league drafter specs, comma-separated: 17l | ovr:PATH#FIELD | KIND:ARG (a registered DrafterKind, e.g. net:WEIGHTS.dpkw[@TABLE.json][~T]: argmax, or ~T samples softmax(logits/T) from the seat rng); each pod seat draws one uniformly from -seed")
	fs.BoolVar(&o.leagueFixed, "league-fixed", false, "seat s takes league entry s mod len instead of a random draw")
	fs.StringVar(&weights, "league-weights", "", "comma-separated draw weights, one per -league entry (empty = uniform)")
	fs.StringVar(&learner, "learner", "", "S2 learner drafter spec (e.g. net:cur.dpkw~1): -learners seats of every pod take it at random positions, the rest draw from -league; recorded as the last drafter")
	fs.IntVar(&o.learners, "learners", 2, "learner seats per pod when -learner is set (1-4)")
	fs.BoolVar(&o.picksJSONL, "picks-jsonl", true, "also write picks.jsonl (the JSON twin of picks.bin; large)")
	fs.StringVar(&builder, "builder", "17l", "best23 ratings for every seat: 17l | ovr:PATH#FIELD (the value table)")
	fs.StringVar(&o.table, "table", "", "draftfeat SetTable JSON for the pick features; empty = facts + z-scored builder ratings")
	fs.StringVar(&o.pilot, "pilot", "bot", "round-robin pilot on both seats: a bot policy name, or az (closed-list az search)")
	fs.StringVar(&o.az.azNet, "az-net", "", "az: the .dzgw network; empty = no network")
	fs.IntVar(&o.az.azSims, "az-sims", 25, "az: simulations per searched decision")
	fs.IntVar(&o.az.azBelief, "az-belief", 500, "az: random decks (from training-seed sealed pools) in the closed-list belief, when -az-belief-dir is empty")
	fs.StringVar(&o.az.azBeliefDir, "az-belief-dir", "", "az: deck directory for searchprobe.LoadBeliefDir")
	fs.StringVar(&o.az.azBeliefPool, "az-belief-pool", "", "az: pool.tsv naming the train split of -az-belief-dir")
	fs.IntVar(&o.az.azPanics, "az-panics", 3, "az: print the first N distinct recovered search-panic stacks to stderr; 0 = off")
	fs.BoolVar(&o.allowAZFaults, "allow-az-faults", false, "az: do not fail the run when panics or root-refused are above 0")
	fs.IntVar(&o.workers, "workers", 4, "concurrent games")
	fs.IntVar(&o.maxObjs, "max-objs", 2000, "engine board-size cap (0 = off)")
	fs.StringVar(&o.out, "out", "", "output directory (required)")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	var err error
	if o.league, err = parseLeague(league); err != nil {
		return o, fmt.Errorf("-league: %w", err)
	}
	if o.weights, err = parseWeights(weights, len(o.league)); err != nil {
		return o, fmt.Errorf("-league-weights: %w", err)
	}
	o.learner = -1
	if learner != "" {
		if o.learners < 1 || o.learners > 4 {
			return o, fmt.Errorf("-learners must be 1-4")
		}
		if o.leagueFixed {
			return o, fmt.Errorf("-learner and -league-fixed are exclusive")
		}
		ls, err := parseLeague(learner)
		if err != nil || len(ls) != 1 {
			return o, fmt.Errorf("-learner: want one spec (%v)", err)
		}
		for _, l := range o.league {
			if l.text == ls[0].text {
				return o, fmt.Errorf("-learner %s is also a -league entry: records name the learner by its spec, so it must be unique", l.text)
			}
		}
		o.learner = len(o.league)
		o.league = append(o.league, ls[0])
		if o.weights != nil {
			o.weights = append(o.weights, 0)
		}
	}
	b, err := parseSeatAgents(builder, false)
	if err != nil {
		return o, fmt.Errorf("-builder: %w", err)
	}
	if len(b) != 1 {
		return o, fmt.Errorf("-builder: want one spec")
	}
	o.builder = b[0]
	switch {
	case o.pods < 1:
		return o, fmt.Errorf("-pods must be at least 1")
	case o.k < 1:
		return o, fmt.Errorf("-k must be at least 1")
	case o.workers < 1:
		return o, fmt.Errorf("-workers must be at least 1")
	case o.out == "":
		return o, fmt.Errorf("-out is required")
	}
	o.az.corpus, o.az.set, o.az.collation = o.corpus, o.set, o.collation
	return o, nil
}

// seatRec is one seats.jsonl line.
type seatRec struct {
	Pod        int            `json:"pod"`
	PodSeed    uint64         `json:"pod_seed"`
	Seat       int            `json:"seat"`
	Drafter    string         `json:"drafter"`
	DrafterRef string         `json:"drafter_ref"`
	Builder    string         `json:"builder"`
	Deck       string         `json:"deck"`
	DeckHash   string         `json:"deck_hash"`
	Main       []string       `json:"main"` // non-basic and basic cards, one per copy, sorted
	Pool       []string       `json:"pool"` // pick order
	Pair       int            `json:"pair"` // true colour pair (draftfeat.Pairs index), training-only label
	PairName   string         `json:"pair_name"`
	Colours    [5]int         `json:"colour_counts"` // non-land main cards per WUBRG colour
	Results    []pairResult   `json:"results"`
	Total      draftlab.Tally `json:"total"`
	Score      float64        `json:"score"` // (wins + draws/2) / games: the reward R_i
	deck       draftlab.Deck
}

type pairResult struct {
	Opp    int `json:"opp"`
	Wins   int `json:"wins"`
	Losses int `json:"losses"`
	Draws  int `json:"draws"`
}

// gameRec is one games.jsonl line: the round-robin game and each side's
// in-hand set (OpeningHand ∪ Drawn by name, cmd/cardval's handLog fold),
// for the per-pick credit C_i(c).
type gameRec struct {
	Pod         int         `json:"pod"`
	Pair        [2]int      `json:"pair"`
	Game        int         `json:"game"`
	Seed        uint64      `json:"seed"`
	Seats       [2]int      `json:"seats"`       // pod seat of engine player 0 and 1
	WinnerSeat  int         `json:"winner_seat"` // -1 = draw or stall
	Stall       string      `json:"stall,omitempty"`
	Turns       int         `json:"turns"`
	Starter     int         `json:"starter"`
	Mulligans   [2]int      `json:"mulligans"`
	InHand      [2][]string `json:"in_hand"`
	OpeningHand [2][]string `json:"opening_hand"`
	Drawn       [2][]string `json:"drawn"`
	Head        string      `json:"head"`
}

type rolloutManifest struct {
	draftlab.Manifest
	Iter        int            `json:"iter"`
	Pods        int            `json:"pods"`
	K           int            `json:"k"`
	League      []string       `json:"league"`
	LeagueFixed bool           `json:"league_fixed,omitempty"`
	Weights     []float64      `json:"league_weights,omitempty"`
	Learner     string         `json:"learner,omitempty"`  // the -learner spec (also the last league entry)
	LearnerIdx  int            `json:"learner_idx"`        // its drafter index in picks.bin, -1 = none
	Learners    int            `json:"learners,omitempty"` // learner seats per pod
	Builder     string         `json:"builder"`
	Pilot       string         `json:"pilot"`
	TableHash   string         `json:"table_hash"`
	MaxObjs     int            `json:"max_objs"`
	GateDecks   int            `json:"gate_decks_excluded"`
	AZ          *azCount       `json:"az,omitempty"`
	Files       []string       `json:"files"`
	PicksBin    map[string]any `json:"picks_bin"`
}

type rolloutResult struct {
	Picks []pickRec
	Seats []seatRec
	Games []draftlab.GameResult
	AZ    *azCount
}

// waitPause blocks while the heavy broker's pause file exists
// (scripts/heavy.sh: GORGE_PAUSE_FILE), checked at each pod and game
// boundary so the job is never stopped mid-step.
func waitPause() {
	p := os.Getenv("GORGE_PAUSE_FILE")
	if p == "" {
		return
	}
	for {
		if _, err := os.Stat(p); err != nil {
			return
		}
		time.Sleep(5 * time.Second)
	}
}

// rollout runs the rollout subcommand (DR-7, operator ruling Q2/Q3): pods
// drafted by league seats, every seat built with best23 on the value-table
// ratings, scored by the round robin within the pod.
func rollout(args []string, out io.Writer) (*rolloutResult, error) {
	o, err := parseRollout(args)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	az := o.pilot == "az"
	lab, err := draftlab.Open(o.corpus, draftlab.Options{AllowDecklistBots: az, MaxObjs: o.maxObjs, Workers: 1})
	if err != nil {
		return nil, err
	}
	coll := setdata.Collation(o.collation)
	guard, err := newGateDeckGuard(lab)
	if err != nil {
		return nil, err
	}
	rc := &ratingsCache{set: o.set, m: map[string]*pickbot.RatingsOverride{}, log: func(s string) { fmt.Fprintln(out, s) }}
	bov, err := rc.get(o.builder)
	if err != nil {
		return nil, err
	}
	var table *draftfeat.SetTable
	if o.table != "" {
		b, err := os.ReadFile(o.table)
		if err != nil {
			return nil, err
		}
		if table, err = draftfeat.DecodeSetTable(b); err != nil {
			return nil, err
		}
	} else if table, err = defaultSetTable(lab, o.set, coll, bov); err != nil {
		return nil, err
	}

	pilotRef := o.pilot
	var stats *azCount
	pilotLabel := o.pilot
	if az {
		if stats, pilotLabel, err = registerAZ(lab, o.az); err != nil {
			return nil, err
		}
		pilotRef = azPolicy
	}
	ref, err := draftlab.ParseAgent(pilotRef)
	if err != nil {
		return nil, err
	}
	player, err := lab.Player(ref)
	if err != nil {
		return nil, err
	}
	leagueText := make([]string, len(o.league))
	for i, l := range o.league {
		leagueText[i] = l.text
	}
	fmt.Fprintf(out, "rollout: set %s (%s), pods %d (seeds %d-%d), league %s, builder best23/%s, pilot %s, k %d, table %s, %d gate decks excluded\n",
		o.set, coll, o.pods, o.seed, o.seed+uint64(o.pods)-1, strings.Join(leagueText, ","), o.builder, pilotLabel, o.k, table.Hash(), len(guard))

	// Phase 1: draft and build every pod serially (cheap, deterministic).
	res := &rolloutResult{AZ: stats}
	for p := 0; p < o.pods; p++ {
		waitPause()
		podSeed := o.seed + uint64(p)
		packs, err := lab.OpenPod(o.set, coll, podSeed, 4, 3)
		if err != nil {
			return nil, fmt.Errorf("pod seed %d: %w", podSeed, err)
		}
		seats := seatPod(len(o.league), podSeed, o.leagueFixed, o.weights, o.learner, o.learners)
		drafters := make([]draftlab.NamedDrafter, 4)
		var podPicks [4][]pickRec
		for s := range drafters {
			d, err := newLeagueDrafter(lab, o.league[seats[s]], o.set, rc)
			if err != nil {
				return nil, err
			}
			d.Drafter = &recordingDrafter{inner: d.Drafter, table: table, pod: p, seat: s, podSeed: podSeed,
				drafter: o.league[seats[s]].text, drafterIdx: seats[s], out: &podPicks[s]}
			drafters[s] = d
		}
		dr, err := lab.Draft(packs, drafters, podSeed)
		if err != nil {
			return nil, fmt.Errorf("pod seed %d: %w", podSeed, err)
		}
		// Seat-major, then pick order: independent of the loop's interleaving.
		for s := 0; s < 4; s++ {
			res.Picks = append(res.Picks, podPicks[s]...)
		}
		for s, pool := range dr.Pools {
			b, err := lab.Builder(draftlab.AgentRef{Name: "best23"})
			if err != nil {
				return nil, err
			}
			if bov != nil {
				b.Builder.(*pickbot.Best23).Ratings = bov
			}
			deck, err := lab.Build(pool, b)
			if err != nil {
				return nil, fmt.Errorf("pod seed %d seat %d: %w", podSeed, s, err)
			}
			deck.Name = fmt.Sprintf("p%d-s%d", podSeed, s)
			deck.DraftID = dr.DraftIDs[s]
			if err := guard.check(deck); err != nil {
				return nil, err
			}
			var main []string
			for _, e := range deck.File.Cards {
				for c := 0; c < e.Count; c++ {
					main = append(main, e.Name)
				}
			}
			sort.Strings(main)
			poolNames := make([]string, len(pool))
			for i, c := range pool {
				poolNames[i] = c.Name
			}
			pair, counts := pairOf(lab, main)
			res.Seats = append(res.Seats, seatRec{
				Pod: p, PodSeed: podSeed, Seat: s, Drafter: o.league[seats[s]].text, DrafterRef: dr.Drafters[s].String(),
				Builder: "best23/" + o.builder.String(), Deck: deck.Name, DeckHash: deckHash(deck), Main: main, Pool: poolNames,
				Pair: pair, PairName: pairName(pair), Colours: counts, deck: deck,
			})
		}
	}
	draftDone := time.Now()

	// Phase 2: every pod's round robin, -workers games at once; results in
	// job order, so nothing depends on the worker count.
	type job struct {
		pod, i, j, g int
		seed         uint64
	}
	var jobs []job
	for p := 0; p < o.pods; p++ {
		podSeed := o.seed + uint64(p)
		for i := 0; i < 4; i++ {
			for j := i + 1; j < 4; j++ {
				for g := 0; g < o.k; g++ {
					gs := rand.New(rand.NewPCG(podSeed, rolloutSalt<<32|uint64(i)<<12|uint64(j)<<8|uint64(g))).Uint64()
					jobs = append(jobs, job{pod: p, i: i, j: j, g: g, seed: gs})
				}
			}
		}
	}
	seatOf := func(p, s int) *seatRec { return &res.Seats[p*4+s] }
	sides := func(jb job) (a, b int) {
		if jb.g%2 == 1 {
			return jb.j, jb.i
		}
		return jb.i, jb.j
	}
	res.Games = make([]draftlab.GameResult, len(jobs))
	errs := make([]error, len(jobs))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < o.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				jb := jobs[i]
				a, b := sides(jb)
				res.Games[i], errs[i] = lab.PlayGame(
					draftlab.Entrant{Deck: seatOf(jb.pod, a).deck, Player: player},
					draftlab.Entrant{Deck: seatOf(jb.pod, b).deck, Player: player}, jb.seed)
			}
		}()
	}
	for i := range jobs {
		waitPause()
		next <- i
	}
	close(next)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("pod %d pair %d-%d game %d: %w", jobs[i].pod, jobs[i].i, jobs[i].j, jobs[i].g, err)
		}
	}
	end := time.Now()

	// Tally the round robin per seat.
	for i := range res.Seats {
		sr := &res.Seats[i]
		for opp := 0; opp < 4; opp++ {
			if opp != sr.Seat {
				sr.Results = append(sr.Results, pairResult{Opp: opp})
			}
		}
	}
	var gameRecs []gameRec
	for i, jb := range jobs {
		g := &res.Games[i]
		a, b := sides(jb)
		ps := [2]int{a, b}
		gr := gameRec{Pod: jb.pod, Pair: [2]int{jb.i, jb.j}, Game: jb.g, Seed: jb.seed, Seats: ps, WinnerSeat: -1,
			Stall: g.Stall, Turns: g.Turns, Starter: g.Starter, Mulligans: g.Mulligans,
			OpeningHand: g.OpeningHand, Drawn: g.Drawn, Head: g.Head}
		if g.Winner >= 0 {
			gr.WinnerSeat = ps[g.Winner]
		}
		for side := 0; side < 2; side++ {
			gr.InHand[side] = unionSorted(g.OpeningHand[side], g.Drawn[side])
			me, opp := ps[side], ps[1-side]
			sr := seatOf(jb.pod, me)
			for k := range sr.Results {
				if sr.Results[k].Opp != opp {
					continue
				}
				switch {
				case g.Winner < 0:
					sr.Results[k].Draws++
					sr.Total.Draws++
				case g.Winner == side:
					sr.Results[k].Wins++
					sr.Total.Wins++
				default:
					sr.Results[k].Losses++
					sr.Total.Losses++
				}
				sr.Total.Games++
			}
		}
		gameRecs = append(gameRecs, gr)
	}
	for i := range res.Seats {
		sr := &res.Seats[i]
		t := &sr.Total
		t.WinRate, t.Lo95, t.Hi95 = wilsonCI(t.Wins, t.Games)
		if t.Games > 0 {
			sr.Score = (float64(t.Wins) + 0.5*float64(t.Draws)) / float64(t.Games)
		}
	}

	if err := writeRollout(lab, o, table, leagueText, pilotLabel, len(guard), res, gameRecs); err != nil {
		return nil, err
	}

	// Per-drafter pooled scores, then the throughput and the az Diag.
	byDrafter := map[string]*draftlab.Tally{}
	for _, sr := range res.Seats {
		t, ok := byDrafter[sr.Drafter]
		if !ok {
			t = &draftlab.Tally{}
			byDrafter[sr.Drafter] = t
		}
		addTally(t, sr.Total)
	}
	for _, d := range leagueText {
		if t, ok := byDrafter[d]; ok {
			printTally(out, d+":", t)
			delete(byDrafter, d)
		}
	}
	draftSec := draftDone.Sub(start).Seconds()
	gameSec := end.Sub(draftDone).Seconds()
	wall := end.Sub(start).Seconds()
	fmt.Fprintf(out, "rollout: %d picks, %d seats, %d games; draft+build %.1fs, games %.1fs (%.2f games/s), wall %.1fs (%.1f pods/h)\n",
		len(res.Picks), len(res.Seats), len(res.Games), draftSec, gameSec, float64(len(res.Games))/max(gameSec, 1e-9), wall, float64(o.pods)*3600/max(wall, 1e-9))
	if stats != nil {
		fmt.Fprintf(out, "  az: %s\n", stats.summary())
		stats.mu.Lock()
		panics, rr := stats.Panics, stats.RootRefused
		stats.mu.Unlock()
		if (panics > 0 || rr > 0) && !o.allowAZFaults {
			return res, fmt.Errorf("az faults: panics %d root-refused %d (zero tolerance; -allow-az-faults to accept)", panics, rr)
		}
	}
	fmt.Fprintf(out, "rollout: wrote %s\n", o.out)
	return res, nil
}

func unionSorted(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, xs := range [][]string{a, b} {
		for _, n := range xs {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	sort.Strings(out)
	return out
}

// writeRollout writes the iteration's records into o.out.
func writeRollout(lab *draftlab.Lab, o rolloutOpts, table *draftfeat.SetTable, league []string, pilot string, gateDecks int, res *rolloutResult, games []gameRec) error {
	if err := os.MkdirAll(o.out, 0o755); err != nil {
		return err
	}
	jsonl := func(name string, n int, at func(i int) any) error {
		return writeFile(filepath.Join(o.out, name), func(w io.Writer) error {
			enc := json.NewEncoder(w)
			for i := 0; i < n; i++ {
				if err := enc.Encode(at(i)); err != nil {
					return err
				}
			}
			return nil
		})
	}
	files := []string{"picks.bin", "seats.jsonl", "games.jsonl", "game_data.csv", "table.json", "manifest.json"}
	if o.picksJSONL {
		if err := jsonl("picks.jsonl", len(res.Picks), func(i int) any { return &res.Picks[i] }); err != nil {
			return err
		}
		files = append([]string{"picks.jsonl"}, files...)
	}
	if err := writeFile(filepath.Join(o.out, "picks.bin"), func(w io.Writer) error {
		return writePicksBin(w, table.Hash(), league, res.Picks)
	}); err != nil {
		return err
	}
	// learner.bin: every net seat's picks, i32 pod, i32 seat, then the
	// record body (netLogitDrafter.LastInputsBin), for the PPO trainer.
	if err := writeFile(filepath.Join(o.out, "learner.bin"), func(w io.Writer) error {
		var hdr [8]byte
		for i := range res.Picks {
			pr := &res.Picks[i]
			if pr.inBin == nil {
				continue
			}
			binary.LittleEndian.PutUint32(hdr[:4], uint32(int32(pr.Pod)))
			binary.LittleEndian.PutUint32(hdr[4:], uint32(int32(pr.Seat)))
			if _, err := w.Write(hdr[:]); err != nil {
				return err
			}
			if _, err := w.Write(pr.inBin); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := jsonl("seats.jsonl", len(res.Seats), func(i int) any { return &res.Seats[i] }); err != nil {
		return err
	}
	if err := jsonl("games.jsonl", len(games), func(i int) any { return &games[i] }); err != nil {
		return err
	}
	names, err := lab.CardNames(o.set, setdata.Collation(o.collation))
	if err != nil {
		return err
	}
	names = withDeckCards(names, res.Seats)
	if err := writeFile(filepath.Join(o.out, "game_data.csv"), func(w io.Writer) error {
		return lab.GameDataCSV(w, res.Games, o.set, "rollout", names, true)
	}); err != nil {
		return err
	}
	tb, err := table.Encode()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(o.out, "table.json"), tb, 0o644); err != nil {
		return err
	}
	agents := []draftlab.RoleRef{{Role: draftlab.RoleBuilder, Ref: draftlab.AgentRef{Name: "best23"}}}
	seenRef := map[string]bool{}
	for _, sr := range res.Seats {
		if !seenRef[sr.DrafterRef] {
			seenRef[sr.DrafterRef] = true
			r, _ := draftlab.ParseAgent(sr.DrafterRef)
			agents = append(agents, draftlab.RoleRef{Role: draftlab.RoleDrafter, Ref: r})
		}
	}
	if len(res.Games) > 0 {
		r, _ := draftlab.ParseAgent(res.Games[0].Players[0])
		agents = append(agents, draftlab.RoleRef{Role: draftlab.RolePlayer, Ref: r})
	}
	m := rolloutManifest{
		Manifest: lab.NewManifest("rollout", o.seed, agents, res.Games),
		Iter:     o.iter, Pods: o.pods, K: o.k, League: league, LeagueFixed: o.leagueFixed,
		Weights: o.weights, LearnerIdx: o.learner,
		Builder: "best23/" + o.builder.String(), Pilot: pilot, TableHash: table.Hash(), MaxObjs: o.maxObjs,
		GateDecks: gateDecks, AZ: res.AZ,
		Files:    files,
		PicksBin: map[string]any{"version": picksBinVersion, "global": globalLayout, "cand": candLayout, "drafters": league},
	}
	m.Set, m.Collation, m.Mode, m.Seats = o.set, o.collation, "draft", 4
	if o.learner >= 0 {
		m.Learner, m.Learners = league[o.learner], o.learners
	}
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(o.out, "manifest.json"), append(b, '\n'), 0o644)
}

// withDeckCards adds any deck card the set's sheets do not list (basics)
// to the CSV universe.
func withDeckCards(names []string, seats []seatRec) []string {
	have := map[string]bool{}
	for _, n := range names {
		have[n] = true
	}
	out := append([]string(nil), names...)
	for _, sr := range seats {
		for _, n := range append(append([]string(nil), sr.Main...), sr.Pool...) {
			if !have[n] {
				have[n] = true
				out = append(out, n)
			}
		}
	}
	sort.Strings(out)
	return out
}
