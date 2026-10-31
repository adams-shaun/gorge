package main

import (
	"fmt"
	"io"
	"math"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/spellbench"
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The gauntlet's seed-stream salts, distinct from the event's own streams
// (pickSaltBase for picks, gameSaltBase for the round-robin games), so a
// gauntlet game's seed never collides with an event game's.
const (
	gauntletSaltBase   = 0x6A111F
	randomDeckSaltBase = 0x8AD5E5
)

// The gauntlet's game kinds, packed into the seed salt so each (kind,
// side, human deck, game, placement) cell has its own stream.
const (
	kindBuilt  = iota // a seat's built deck
	kindRandom        // the random baseline deck
	kindSanity        // a human-vs-human mirror pairing
)

// gauntletOpts is the validated -gauntlet configuration: which spellbench
// catalog the human decks come from, which bot policy sits on BOTH seats of
// every measurement game, the baseline deck shape and how many games each
// pairing plays.
type gauntletOpts struct {
	catalog  spellbench.Catalog
	player   string
	baseline string // "random" or "none"
	games    int
	// maxHuman caps the human deck list for tests; 0 means every deck in
	// the catalog. It is an internal option, not a flag.
	maxHuman int
}

// gauntletFromOptions validates the -gauntlet flags before anything
// expensive runs; nil, nil when the gauntlet is off (the empty -gauntlet).
func gauntletFromOptions(o options) (*gauntletOpts, error) {
	if o.gauntlet == "" {
		return nil, nil
	}
	if o.gauntlet != "fdn-limited" {
		return nil, fmt.Errorf("unknown -gauntlet %q (only fdn-limited is implemented)", o.gauntlet)
	}
	if o.set != "FDN" {
		return nil, fmt.Errorf("-gauntlet requires -set FDN, got %s", o.set)
	}
	if o.gauntletGames < 1 {
		return nil, fmt.Errorf("-gauntlet-games must be at least 1, got %d", o.gauntletGames)
	}
	if o.baseline != "random" && o.baseline != "none" {
		return nil, fmt.Errorf("unknown -baseline %q (want random or none)", o.baseline)
	}
	cat, err := spellbench.CatalogByID(o.gauntlet)
	if err != nil {
		return nil, err
	}
	// The gauntlet seats one policy on BOTH sides, and the same operator
	// ruling the draft seats follow applies: a policy that builds its
	// search over every seat's REAL decklist must never seat here either.
	if _, err := bots.Normalize(o.gauntletPlayer); err != nil {
		return nil, fmt.Errorf("-gauntlet-player: %w", err)
	}
	if bots.ReadsDecklists(o.gauntletPlayer) {
		return nil, fmt.Errorf("-gauntlet-player %q reads every seat's real decklist (bots.Info.Env); by design, draftsim refuses Env policies", o.gauntletPlayer)
	}
	return &gauntletOpts{
		catalog:  cat,
		player:   o.gauntletPlayer,
		baseline: o.baseline,
		games:    o.gauntletGames,
		maxHuman: o.gauntletMaxHuman,
	}, nil
}

// humanIDs lists the catalog's deck ids in catalog order (spellbench sorts
// them), capped to maxHuman for tests when it is set. The order is the
// pairing order, so it is a pure function of the catalog.
func (g *gauntletOpts) humanIDs() ([]string, error) {
	ids, err := spellbench.CatalogIDs(g.catalog.Dir)
	if err != nil {
		return nil, fmt.Errorf("gauntlet catalog %s: %w", g.catalog.ID, err)
	}
	if g.maxHuman > 0 && g.maxHuman < len(ids) {
		ids = ids[:g.maxHuman]
	}
	return ids, nil
}

// resolveHumanDecks turns the catalog ids into engine card pointers.
func resolveHumanDecks(r *runner, g *gauntletOpts, ids []string) ([][]*cards.Card, error) {
	out := make([][]*cards.Card, len(ids))
	for i, id := range ids {
		cs, err := spellbench.Deck(r.reg, g.catalog.Dir, id)
		if err != nil {
			return nil, fmt.Errorf("human deck %s: %w", id, err)
		}
		out[i] = cs
	}
	return out, nil
}

// gauntletEvent plays one event's measurement games: every seat's built
// deck against every human deck, and — with the random baseline — the same
// gauntlet over one random deck per seat built from that seat's pool.
// Results accumulate in acc; nothing here touches the event log.
func gauntletEvent(r *runner, g *gauntletOpts, ev *event.Event, decks [][]*cards.Card, seed uint64, verify bool, acc *gauntletAccum, out io.Writer) error {
	ids, err := g.humanIDs()
	if err != nil {
		return err
	}
	human, err := resolveHumanDecks(r, g, ids)
	if err != nil {
		return err
	}
	var randomDecks [][]*cards.Card
	if g.baseline == "random" {
		randomDecks = make([][]*cards.Card, len(decks))
		for s := range decks {
			cs, err := randomDeck(seed, s, ev.Pool(s), r.reg)
			if err != nil {
				return err
			}
			randomDecks[s] = cs
		}
	}
	fmt.Fprintf(out, "  gauntlet: %d human decks (%s), %d games per pairing, player %s\n",
		len(ids), g.catalog.ID, g.games, g.player)
	for s := range decks {
		if err := gauntletSide(r, g, seed, verify, kindBuilt, s, decks[s], human, &acc.built[s]); err != nil {
			return err
		}
		if randomDecks != nil {
			if err := gauntletSide(r, g, seed, verify, kindRandom, s, randomDecks[s], human, &acc.random[s]); err != nil {
				return err
			}
		}
	}
	return nil
}

// gauntletSide plays one measuring deck against every human deck and
// tallies the results from the measuring deck's side. Game n of a pairing
// alternates which side sits in player 0's seat (one on the play's seat,
// one on the draw's); the engine's CR 103.1 toss still decides who is
// actually on the play, so the alternation balances the seat placement.
func gauntletSide(r *runner, g *gauntletOpts, seed uint64, verify bool, kind, s int, mine []*cards.Card, human [][]*cards.Card, t *gauntletTally) error {
	for j, theirs := range human {
		for n := 0; n < g.games; n++ {
			me := n % 2
			deckA, deckB := mine, theirs
			if me == 1 {
				deckA, deckB = theirs, mine
			}
			oc, err := playSeeded(r, gauntletSeed(seed, kind, s, j, n, me),
				[2]string{g.player, g.player},
				[2]string{fmt.Sprintf("gauntlet-s%d", s), fmt.Sprintf("human-%d", j)},
				deckA, deckB, verify)
			if err != nil {
				return fmt.Errorf("seat %d vs human %d game %d: %w", s, j, n, err)
			}
			t.record(oc.winner, me)
		}
	}
	return nil
}

// gauntletSanity plays the human-vs-human mirror pairings once per run:
// each human deck against the next one in catalog order, the same games
// per pairing with the same seat alternation. It is seeded from the run's
// -seed, so it does not depend on -events.
func gauntletSanity(r *runner, g *gauntletOpts, seed uint64, verify bool, acc *gauntletAccum, out io.Writer) error {
	ids, err := g.humanIDs()
	if err != nil {
		return err
	}
	human, err := resolveHumanDecks(r, g, ids)
	if err != nil {
		return err
	}
	for i := range ids {
		j := (i + 1) % len(ids)
		for n := 0; n < g.games; n++ {
			me := n % 2
			deckA, deckB := human[i], human[j]
			if me == 1 {
				deckA, deckB = human[j], human[i]
			}
			oc, err := playSeeded(r, gauntletSeed(seed, kindSanity, i, j, n, me),
				[2]string{g.player, g.player},
				[2]string{ids[i], ids[j]}, deckA, deckB, verify)
			if err != nil {
				return fmt.Errorf("sanity %s vs %s game %d: %w", ids[i], ids[j], n, err)
			}
			acc.sanity.record(oc.winner, me)
		}
	}
	return nil
}

// gauntletSeed derives one gauntlet game's engine seed: a pure function of
// (event seed, kind, side index, human deck index, game number, which side
// sat in player 0's seat), salted away from the event's own streams.
func gauntletSeed(seed uint64, kind, s, j, n, me int) uint64 {
	r := rand.New(rand.NewPCG(seed, gauntletSaltBase<<32|
		uint64(kind)<<28|uint64(s)<<24|uint64(j)<<16|uint64(n)<<8|uint64(me)))
	return r.Uint64()
}

// randomDeck builds one seat's random baseline deck from its pool: 23
// random non-land pool cards (Fisher–Yates over a copy, seeded from the
// event seed and the seat; a pool too thin gives what it has) plus
// build.AutoLands basics up to 40, then resolves the deck file.
func randomDeck(seed uint64, seat int, pool []limited.CardRef, reg *cards.Registry) ([]*cards.Card, error) {
	rng := rand.New(rand.NewPCG(seed, randomDeckSaltBase<<32|uint64(seat)))
	nonLand := make([]limited.CardRef, 0, len(pool))
	for _, ref := range pool {
		c, ok := reg.Lookup(ref.Name)
		if !ok {
			return nil, fmt.Errorf("random deck seat %d: pool card %q is not in the corpus", seat, ref.Name)
		}
		if len(c.Faces) > 0 && c.Faces[0].IsLand() {
			continue
		}
		nonLand = append(nonLand, ref)
	}
	shuffled := append([]limited.CardRef(nil), nonLand...)
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	n := 23
	if len(shuffled) < n {
		n = len(shuffled)
	}
	d := build.Deck{Basics: [5]int{}}
	var pips [5]int
	for _, ref := range shuffled[:n] {
		d.Main = append(d.Main, ref.ID)
		c, _ := reg.Lookup(ref.Name)
		for _, face := range c.Faces {
			for i := 0; i < 5; i++ {
				pips[i] += int(cards.ManaCostChroma(face.ManaCost, 1<<i))
			}
		}
	}
	d.Basics = build.AutoLands(d.Main, func(string) [5]int { return pips })
	f := build.ToDeckFile(pool, d, fmt.Sprintf("p%d random gauntlet deck", seat))
	return f.Resolve(reg)
}

// gauntletTally counts one measured side's results across its gauntlet
// games: wins, losses and draws, plus the games total they must sum to.
type gauntletTally struct {
	wins, losses, draws, games int
}

// record folds one game in: winner is the winning player index (nil for a
// draw or a watchdog stall) and me is the player index the measured side
// sat in.
func (t *gauntletTally) record(winner *int, me int) {
	t.games++
	switch {
	case winner == nil:
		t.draws++
	case *winner == me:
		t.wins++
	default:
		t.losses++
	}
}

// poolOf sums a per-seat tally slice into its pooled tally.
func poolOf(ts []gauntletTally) gauntletTally {
	var out gauntletTally
	for _, t := range ts {
		out.wins += t.wins
		out.losses += t.losses
		out.draws += t.draws
		out.games += t.games
	}
	return out
}

// gauntletAccum accumulates the run's gauntlet results: one tally per
// draft seat for the built decks and the random baseline, and the pooled
// human-vs-human sanity tally.
type gauntletAccum struct {
	built  []gauntletTally
	random []gauntletTally
	sanity gauntletTally
}

// newGauntletAccum sizes the per-seat tallies for the pod.
func newGauntletAccum(seats int) *gauntletAccum {
	return &gauntletAccum{
		built:  make([]gauntletTally, seats),
		random: make([]gauntletTally, seats),
	}
}

// print writes the summary block. The three pooled lines — exactly
// "built: W-L-D  win% P  ci95 [LO, HI]", "random: …" and "sanity: …" — are
// the stable contract the tests parse; the per-seat lines sit above them.
// win% is wins over all games (a draw is not a win) with the Wilson score
// 95% interval on that proportion.
func (a *gauntletAccum) print(out io.Writer, baseline string) {
	built := poolOf(a.built)
	for i := range a.built {
		printTally(out, fmt.Sprintf("built seat %d:", i), a.built[i])
	}
	printTally(out, "built:", built)
	if baseline == "random" {
		random := poolOf(a.random)
		for i := range a.random {
			printTally(out, fmt.Sprintf("random seat %d:", i), a.random[i])
		}
		printTally(out, "random:", random)
	}
	printTally(out, "sanity:", a.sanity)
}

// printTally writes one summary line for a tally.
func printTally(out io.Writer, label string, t gauntletTally) {
	p, lo, hi := wilsonCI(t.wins, t.games)
	fmt.Fprintf(out, "  %-16s %d-%d-%d  win%% %.3f  ci95 [%.3f, %.3f]\n",
		label, t.wins, t.losses, t.draws, p, lo, hi)
}

// wilsonCI returns the win proportion over n games and its Wilson score
// 95% interval; n == 0 returns zeros.
func wilsonCI(wins, n int) (p, lo, hi float64) {
	if n == 0 {
		return 0, 0, 0
	}
	p = float64(wins) / float64(n)
	const z = 1.959963984540054 // the two-sided 95% normal quantile
	den := 1 + z*z/float64(n)
	center := (p + z*z/(2*float64(n))) / den
	rad := z / den * math.Sqrt(p*(1-p)/float64(n)+z*z/(4*float64(n)*float64(n)))
	return p, math.Max(0, center-rad), math.Min(1, center+rad)
}
