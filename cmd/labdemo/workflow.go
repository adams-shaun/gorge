package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/draftlab/extern"
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// The labdemo step salts, one PCG stream family per workflow step. They
// sit next to draftlab's batch salts (0x1AB6A3 games, 0x1AB6A4 gauntlet)
// in an unused 24-bit word, so no two step streams share a PCG key. Each
// step's item i seeds itself as rand.NewPCG(seed, salt<<32|i); a step's
// whole-run seed is rand.NewPCG(seed, salt<<32|^uint64(0)) — the reserved
// last item, never handed to an item loop.
const (
	saltRandomPools = 0x1ABD01 // step 1: pool and random-deck streams, item matchup<<1|side
	saltRandomBatch = 0x1ABD02 // step 1: the Games batch seed
	saltSealedPools = 0x1ABD03 // step 3: pool streams, item pool index
	saltPods        = 0x1ABD04 // step 4: event seeds, item pod index
	saltGauntlet    = 0x1ABD05 // step 5: gauntlet seeds, item hero index
)

// stepSeed derives a step's whole-run seed: rand.NewPCG(seed, salt<<32|
// ^uint64(0)).
func stepSeed(seed, salt uint64) uint64 {
	return rand.New(rand.NewPCG(seed, salt<<32|^uint64(0))).Uint64()
}

// stepItem derives one step item's seed: rand.NewPCG(seed, salt<<32|i).
func stepItem(seed, salt, i uint64) uint64 {
	return rand.New(rand.NewPCG(seed, salt<<32|i)).Uint64()
}

// runResult is one workflow run's outputs, in step order.
type runResult struct {
	// randomGames is step 1's played games.
	randomGames []draftlab.GameResult
	// sealed is step 3's built decks (DraftID "" — not drafted).
	sealed []draftlab.Deck
	// pods is step 4's finished events, in pod order.
	pods []draftlab.EventResult
	// gauntlets is step 5's finished gauntlets, in hero order.
	gauntlets []draftlab.GauntletResult
	// gamesManifest is step 1's provenance (Lab.Games produces results
	// only; the workflow builds the manifest).
	gamesManifest draftlab.Manifest
}

// run is main's testable body: the whole example workflow over the flags.
// Every step seeds itself from o.seed, so two runs with the same flags are
// byte-identical, artifacts included. The -drafter/-builder/-player/
// -opponent-players agents resolve through extern.Registry, so any of them
// may be an ext: spec (dialed once per endpoint and shared across roles);
// with default flags every resolved agent is a built-in.
func run(o options, out io.Writer) (retErr error) {
	switch {
	case o.randomGames < 0:
		return fmt.Errorf("-random-games %d is negative", o.randomGames)
	case o.sealed < 0:
		return fmt.Errorf("-sealed %d is negative", o.sealed)
	case o.pods < 0:
		return fmt.Errorf("-pods %d is negative", o.pods)
	case o.podSeats != 4 && o.podSeats != 8:
		return fmt.Errorf("-pod-seats %d, want 4 or 8", o.podSeats)
	case o.opponentCount < 0:
		return fmt.Errorf("-opponent-count %d is negative", o.opponentCount)
	case o.gauntletHeroes < 0:
		return fmt.Errorf("-gauntlet-heroes %d is negative", o.gauntletHeroes)
	case o.games < 1:
		return fmt.Errorf("-games %d, want at least 1", o.games)
	case o.workers < 0:
		return fmt.Errorf("-workers %d is negative", o.workers)
	case o.extMaxBatch < 0:
		return fmt.Errorf("-ext-max-batch %d is negative", o.extMaxBatch)
	case o.extFlush != "" && o.extFlush != string(extern.FlushBarrier) && o.extFlush != string(extern.FlushReady):
		return fmt.Errorf("-ext-flush %q, want %q or %q", o.extFlush, extern.FlushBarrier, extern.FlushReady)
	case o.extFail != "" && o.extFail != string(extern.FailError) && o.extFail != string(extern.FailFallback):
		return fmt.Errorf("-ext-fail %q, want %q or %q", o.extFail, extern.FailError, extern.FailFallback)
	}
	if err := validateSpecs(o); err != nil {
		return err
	}
	l, err := draftlab.Open(o.dir, draftlab.Options{Workers: o.workers, RecordTape: o.verifyTapes})
	if err != nil {
		return err
	}
	reg := extern.NewRegistry(context.Background(), l, extOptions(o))
	defer func() {
		if err := reg.Close(); err != nil && retErr == nil {
			retErr = fmt.Errorf("closing external agents: %w", err)
		}
	}()
	player, err := reg.Player(o.player)
	if err != nil {
		return fmt.Errorf("freezing %s: %w", o.player, err)
	}
	coll := collation(o)
	sheets, err := l.CardNames(o.set, coll)
	if err != nil {
		return err
	}
	// gameNames is the game_data name universe: the set's sheet universe
	// united with the five basics built decks play, sorted unique (FDN's
	// sheets already carry the basics; the union keeps the universe right
	// for sets whose sheets do not).
	gameNames := append(append([]string(nil), sheets...), draftlab.BasicNames[:]...)
	sort.Strings(gameNames)
	gameNames = slices.Compact(gameNames)

	res := runResult{}
	// Step 1: random-deck games. Both seats play the -player policy (the
	// default bot@1 IS "the bot player"); the freeze step below pins it
	// for the rest of the workflow.
	res.randomGames, err = stepRandomGames(l, o, coll, player)
	if err != nil {
		return err
	}
	res.gamesManifest = l.NewManifest("games", stepSeed(o.seed, saltRandomBatch),
		[]draftlab.RoleRef{{Role: draftlab.RolePlayer, Ref: player.Ref}}, res.randomGames)

	// Step 2: freeze the player (the ref the rest of the workflow plays).
	fmt.Fprintf(out, "frozen player %s\n", player.Ref)

	// Step 3: sealed builds (the -builder agent, only when builds run).
	var builder draftlab.NamedBuilder
	if o.sealed > 0 || o.pods > 0 {
		builder, err = reg.Builder(builderSpec(o))
		if err != nil {
			return fmt.Errorf("builder %s: %w", builderSpec(o), err)
		}
	}

	// Step 4: draft pods (the -drafter and -builder agents, only when pods
	// run).
	var drafter draftlab.NamedDrafter
	if o.pods > 0 {
		drafter, err = reg.Drafter(drafterSpec(o))
		if err != nil {
			return fmt.Errorf("drafter %s: %w", drafterSpec(o), err)
		}
	}
	res.sealed, err = stepSealedBuilds(l, o, coll, builder)
	if err != nil {
		return err
	}
	res.pods, err = stepPods(l, o, coll, player, drafter, builder)
	if err != nil {
		return err
	}
	if o.verifyTapes {
		for k, pod := range res.pods {
			if err := l.VerifyEventTapes(pod); err != nil {
				return fmt.Errorf("pod %d: %w", k, err)
			}
		}
		fmt.Fprintf(out, "tapes verified: %d pod(s)\n", len(res.pods))
	}

	// Step 5: gauntlets (opponent players resolved through the registry
	// too, so they may be ext: specs).
	res.gauntlets, err = stepGauntlet(l, o, reg, player, res.sealed, res.pods)
	if err != nil {
		return err
	}

	if o.out != "" {
		if err := writeArtifacts(l, o, sheets, gameNames, res); err != nil {
			return err
		}
	}
	printSummary(o, out, res)
	printExternStats(o, reg)
	return nil
}

// drafterSpec is the run's drafter spec: the empty flag means the
// built-in default rating-colour@1.
func drafterSpec(o options) string {
	if o.drafter == "" {
		return "rating-colour@1"
	}
	return o.drafter
}

// builderSpec is the run's builder spec: the empty flag means the
// built-in default best23@1.
func builderSpec(o options) string {
	if o.builder == "" {
		return "best23@1"
	}
	return o.builder
}

// validateSpecs rejects a malformed ext: spec before any corpus work: the
// flags that may name an external agent each parse the same way the
// registry will.
func validateSpecs(o options) error {
	specs := append([]string{o.player, drafterSpec(o), builderSpec(o)}, playerList(o.opponentPlayers)...)
	for _, spec := range specs {
		if !extern.IsSpec(spec) {
			continue
		}
		if _, _, err := extern.ParseSpec(spec); err != nil {
			return err
		}
	}
	return nil
}

// extOptions translates the -ext-* flags into extern session options. Log
// (the sessions' death and end lines) goes to errOut when it is set; the
// child stderr keeps extern's own default.
func extOptions(o options) extern.Options {
	eo := extern.Options{
		Flush:          extern.FlushMode(o.extFlush),
		MaxBatch:       o.extMaxBatch,
		OnFailure:      extern.Failure(o.extFail),
		StrictRefusals: o.extStrict,
	}
	if o.errOut != nil {
		w := o.errOut
		eo.Log = func(line string) { fmt.Fprintln(w, line) }
	}
	return eo
}

// printExternStats writes one line per dialed external agent, in dial
// order, to errOut (nil discards).
func printExternStats(o options, reg *extern.Registry) {
	if o.errOut == nil {
		return
	}
	for _, a := range reg.Agents() {
		s := a.Stats()
		fmt.Fprintf(o.errOut,
			"extern %s: picks %d builds %d decides %d batches %d max-batch %d forced %d refused %d fallbacks %d\n",
			a.Ref(), s.Picks, s.Builds, s.Decides, s.Batches, s.MaxBatch, s.Forced, s.Refused, s.Fallbacks)
	}
}

// stepRandomGames plays -random-games Bo1 games between two random decks
// (a sealed pool each, draftsim's random build over it), both piloted by
// the frozen player.
func stepRandomGames(l *draftlab.Lab, o options, coll setdata.Collation, player draftlab.NamedPlayer) ([]draftlab.GameResult, error) {
	if o.randomGames == 0 {
		return nil, nil
	}
	matchups := make([]draftlab.Matchup, o.randomGames)
	for i := range matchups {
		for side := 0; side < 2; side++ {
			s := stepItem(o.seed, saltRandomPools, uint64(i)<<1|uint64(side))
			deck, err := randomSealedDeck(l, o, coll, s)
			if err != nil {
				return nil, fmt.Errorf("random deck %d/%d: %w", i, side, err)
			}
			if side == 0 {
				matchups[i].A = draftlab.Entrant{Deck: deck, Player: player}
			} else {
				matchups[i].B = draftlab.Entrant{Deck: deck, Player: player}
			}
		}
	}
	return l.Games(matchups, stepSeed(o.seed, saltRandomBatch))
}

// randomSealedDeck opens a six-pack sealed pool from s and builds
// draftsim's random deck over it.
func randomSealedDeck(l *draftlab.Lab, o options, coll setdata.Collation, s uint64) (draftlab.Deck, error) {
	pool, err := l.SealedPool(o.set, coll, s, event.DefaultSealedPacks)
	if err != nil {
		return draftlab.Deck{}, err
	}
	return l.RandomDeck(pool, s)
}

// stepSealedBuilds builds a deck from each of -sealed sealed pools with
// the run's builder (default best23@1). The decks carry no draft_id: they
// were not drafted.
func stepSealedBuilds(l *draftlab.Lab, o options, coll setdata.Collation, builder draftlab.NamedBuilder) ([]draftlab.Deck, error) {
	if o.sealed == 0 {
		return nil, nil
	}
	decks := make([]draftlab.Deck, 0, o.sealed)
	for i := 0; i < o.sealed; i++ {
		pool, err := l.SealedPool(o.set, coll, stepItem(o.seed, saltSealedPools, uint64(i)), event.DefaultSealedPacks)
		if err != nil {
			return nil, fmt.Errorf("sealed pool %d: %w", i, err)
		}
		d, err := l.Build(pool, builder)
		if err != nil {
			return nil, fmt.Errorf("sealed build %d: %w", i, err)
		}
		d.Name = fmt.Sprintf("sealed %d", i+1)
		decks = append(decks, d)
	}
	return decks, nil
}

// stepPods runs -pods draft pods of -pod-seats seats under Swiss: the
// run's drafter and builder (defaults rating-colour@1 and best23@1) at
// every seat, the frozen player piloting, each pod from its own derived
// event seed.
func stepPods(l *draftlab.Lab, o options, coll setdata.Collation, player draftlab.NamedPlayer, dr draftlab.NamedDrafter, bd draftlab.NamedBuilder) ([]draftlab.EventResult, error) {
	if o.pods == 0 {
		return nil, nil
	}
	pods := make([]draftlab.EventResult, 0, o.pods)
	for k := 0; k < o.pods; k++ {
		res, err := l.RunEvent(draftlab.EventSpec{
			Set:       o.set,
			Collation: coll,
			Mode:      event.ModeDraft,
			Seats:     o.podSeats,
			Structure: draftlab.Swiss,
			Drafters:  []draftlab.NamedDrafter{dr},
			Builders:  []draftlab.NamedBuilder{bd},
			Players:   []draftlab.NamedPlayer{player},
			Seed:      stepItem(o.seed, saltPods, uint64(k)),
		})
		if err != nil {
			return nil, fmt.Errorf("pod %d: %w", k, err)
		}
		pods = append(pods, res)
	}
	return pods, nil
}

// stepGauntlet runs a gauntlet per built deck — the sealed builds, then
// the pod decks, capped by -gauntlet-heroes (0 = every built deck) — each
// with the frozen player against the -opponents catalog's head
// -opponent-count decks (clamped to the catalog's size), the
// -opponent-players policies (resolved through the registry, so they may
// be ext: specs) cycled over them, -games per pairing.
func stepGauntlet(l *draftlab.Lab, o options, reg *extern.Registry, player draftlab.NamedPlayer, sealed []draftlab.Deck, pods []draftlab.EventResult) ([]draftlab.GauntletResult, error) {
	heroes := make([]draftlab.Deck, 0, len(sealed)+len(podDecks(pods)))
	heroes = append(heroes, sealed...)
	heroes = append(heroes, podDecks(pods)...)
	if o.gauntletHeroes > 0 && len(heroes) > o.gauntletHeroes {
		heroes = heroes[:o.gauntletHeroes]
	}
	if len(heroes) == 0 || o.opponentCount == 0 {
		return nil, nil
	}
	catalog, err := l.CatalogDecks(o.opponents)
	if err != nil {
		return nil, err
	}
	if len(catalog) == 0 {
		return nil, fmt.Errorf("catalog %q produced no decks", o.opponents)
	}
	if o.opponentCount < len(catalog) {
		catalog = catalog[:o.opponentCount]
	}
	var oppPlayers []draftlab.NamedPlayer
	for _, name := range playerList(o.opponentPlayers) {
		np, err := reg.Player(name)
		if err != nil {
			return nil, fmt.Errorf("opponent player %s: %w", name, err)
		}
		oppPlayers = append(oppPlayers, np)
	}
	opponents := make([]draftlab.Entrant, len(catalog))
	for i, cd := range catalog {
		opponents[i] = draftlab.Entrant{Deck: cd, Player: oppPlayers[i%len(oppPlayers)]}
	}
	out := make([]draftlab.GauntletResult, 0, len(heroes))
	for j, heroDeck := range heroes {
		g, err := l.Gauntlet(draftlab.Entrant{Deck: heroDeck, Player: player}, opponents, o.games,
			stepItem(o.seed, saltGauntlet, uint64(j)))
		if err != nil {
			return nil, fmt.Errorf("gauntlet hero %d (%s): %w", j, heroDeck.Name, err)
		}
		out = append(out, g)
	}
	return out, nil
}

// podDecks lists a pod slice's built decks, pod order then seat order.
func podDecks(pods []draftlab.EventResult) []draftlab.Deck {
	var out []draftlab.Deck
	for _, pod := range pods {
		out = append(out, pod.Decks...)
	}
	return out
}

// writeArtifacts writes every artifact file under o.out: draft_data.csv
// (all pods, one header), game_data.csv (all finished games of steps 1, 4
// and 5 under one header, each step's event_type), decks.json (steps 3–4),
// events/pod<k>.log.jsonl (k from 1) and manifests.json (an array, step
// order: the games manifest, each pod's event manifest, each gauntlet's).
// The sealed builds carry no manifest: they play no games.
func writeArtifacts(l *draftlab.Lab, o options, sheets, gameNames []string, res runResult) error {
	if err := os.MkdirAll(o.out, 0o755); err != nil {
		return fmt.Errorf("out directory %s: %w", o.out, err)
	}
	if len(res.pods) > 0 {
		if err := writePodArtifacts(o, sheets, res); err != nil {
			return err
		}
	}
	podEventType := "gorge_pod"
	if o.podSeats == 8 {
		podEventType = "gorge_pod8"
	}
	if err := writeGameData(l, o, gameNames, podEventType, res); err != nil {
		return err
	}
	decks := append(append([]draftlab.Deck(nil), res.sealed...), podDecks(res.pods)...)
	if len(decks) > 0 {
		if err := writeDecksFile(o, decks); err != nil {
			return err
		}
	}
	manifests := make([]draftlab.Manifest, 0, 1+len(res.pods)+len(res.gauntlets))
	manifests = append(manifests, res.gamesManifest)
	for _, pod := range res.pods {
		manifests = append(manifests, pod.Manifest)
	}
	for _, g := range res.gauntlets {
		manifests = append(manifests, g.Manifest)
	}
	if err := writeJSON(filepath.Join(o.out, "manifests.json"), manifests); err != nil {
		return err
	}
	return nil
}

// writePodArtifacts writes the pods' artifacts: draft_data.csv (one
// header, every pod's rows appended) and events/pod<k>.log.jsonl (k from
// 1).
func writePodArtifacts(o options, sheets []string, res runResult) error {
	eventsDir := filepath.Join(o.out, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		return fmt.Errorf("out directory %s: %w", eventsDir, err)
	}
	f, err := os.Create(filepath.Join(o.out, "draft_data.csv"))
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for k, pod := range res.pods {
		if err := draftlab.DraftDataCSV(w, pod.Log, sheets, k == 0); err != nil {
			f.Close()
			return fmt.Errorf("draft_data.csv: pod %d: %w", k, err)
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("draft_data.csv: %w", err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	for k, pod := range res.pods {
		path := filepath.Join(eventsDir, fmt.Sprintf("pod%d.log.jsonl", k+1))
		if err := os.WriteFile(path, pod.Log, 0o644); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

// writeGameData writes game_data.csv: every finished game of steps 1, 4
// and 5 under one header, each step's event_type on its own rows —
// "gorge_lab" for the random-deck games, the pod event type for the pod
// games ("gorge_pod" at 4 seats, "gorge_pod8" at 8) and "gorge_gauntlet"
// for the gauntlet games. A step with no finished games writes no rows.
func writeGameData(l *draftlab.Lab, o options, gameNames []string, podEventType string, res runResult) error {
	f, err := os.Create(filepath.Join(o.out, "game_data.csv"))
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	header := false
	emit := func(games []draftlab.GameResult, eventType, label string) error {
		if finishedCount(games) == 0 {
			return nil
		}
		if err := l.GameDataCSV(w, games, o.set, eventType, gameNames, !header); err != nil {
			return fmt.Errorf("game_data.csv: %s: %w", label, err)
		}
		header = true
		return nil
	}
	if err := emit(res.randomGames, "gorge_lab", "random-deck games"); err != nil {
		return err
	}
	for k, pod := range res.pods {
		if err := emit(pod.Games, podEventType, fmt.Sprintf("pod %d", k)); err != nil {
			return err
		}
	}
	for k, g := range res.gauntlets {
		if err := emit(g.Games, "gorge_gauntlet", fmt.Sprintf("gauntlet %d", k)); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("game_data.csv: %w", err)
	}
	return nil
}

// writeDecksFile writes decks.json: steps 3–4's decks, sealed builds then
// pod decks, input order.
func writeDecksFile(o options, decks []draftlab.Deck) error {
	return writeFile(filepath.Join(o.out, "decks.json"), func(w io.Writer) error {
		return draftlab.WriteDecksJSON(w, decks)
	})
}

// writeJSON writes one JSON file with two-space indent and a trailing
// newline, so the artifacts diff cleanly and stay byte-stable.
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	data = append(data, '\n')
	return writeFile(path, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// writeFile creates path and fills it through one write call.
func writeFile(path string, fill func(w io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	if err := fill(w); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// printSummary writes the run's summary: the game counts per step and each
// hero's pooled gauntlet line "hero <name>: W-L-D win% ci95 [LO, HI]"
// (Wilson's 95% interval, draftlab.Tally's).
func printSummary(o options, out io.Writer, res runResult) {
	fmt.Fprintf(out, "labdemo seed %d: set %s, collation %q, player %s\n", o.seed, o.set, o.collation, o.player)
	fmt.Fprintf(out, "step 1 random-deck games: %d played, %d finished\n", len(res.randomGames), finishedCount(res.randomGames))
	fmt.Fprintf(out, "step 3 sealed builds: %d decks\n", len(res.sealed))
	podGames := collectPodGames(res.pods)
	fmt.Fprintf(out, "step 4 pods: %d pod(s) of %d seats, %d games (%d finished)\n",
		len(res.pods), o.podSeats, len(podGames), finishedCount(podGames))
	gauntletGames := collectGauntletGames(res.gauntlets)
	fmt.Fprintf(out, "step 5 gauntlet: %d hero(es), %d games (%d finished)\n",
		len(res.gauntlets), len(gauntletGames), finishedCount(gauntletGames))
	for _, g := range res.gauntlets {
		t := g.Pooled
		fmt.Fprintf(out, "hero %s: %d-%d-%d %.3f ci95 [%.3f, %.3f]\n",
			g.Hero, t.Wins, t.Losses, t.Draws, t.WinRate, t.Lo95, t.Hi95)
	}
	if o.out != "" {
		fmt.Fprintf(out, "artifacts: %s\n", o.out)
	}
}

// finishedCount counts a game slice's games with a winner.
func finishedCount(games []draftlab.GameResult) int {
	n := 0
	for _, g := range games {
		if g.Winner >= 0 {
			n++
		}
	}
	return n
}

// collectPodGames collects a pod slice's games, pod order then play order.
func collectPodGames(pods []draftlab.EventResult) []draftlab.GameResult {
	var out []draftlab.GameResult
	for _, pod := range pods {
		out = append(out, pod.Games...)
	}
	return out
}

// collectGauntletGames collects a gauntlet slice's games, hero order then
// the gauntlet's own order.
func collectGauntletGames(gs []draftlab.GauntletResult) []draftlab.GameResult {
	var out []draftlab.GameResult
	for _, g := range gs {
		out = append(out, g.Games...)
	}
	return out
}
