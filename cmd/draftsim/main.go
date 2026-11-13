// Command draftsim runs headless draft pods: 4 or 8 seats draft (or open sealed packs) with
// draft/limited/pickbot, every seat builds its deck, and the pod's series are
// played as Bo1 games through the rules engine the way cmd/mtgsim drives
// one — round robin by default, Swiss with -structure swiss, or build-only
// with -structure none — with -verify replaying each event from its own
// log. The command touches the engine so that everything under
// draft/limited/ and draft/event/ stays pure.
//
//	draftsim -set FDN -seed 1 -events 5 -verify
//
// The event seed drives everything: the packs (booster.OpenPod), the picks
// (one seeded rand per seat) and the game seeds (derived per
// round/series/game), so the same seed and set rebuild the same event log
// byte for byte.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

func main() {
	o := options{}
	fs := flag.NewFlagSet("draftsim", flag.ExitOnError)
	fs.StringVar(&o.set, "set", "FDN", "set code to draft (an embedded draft/limited/setdata spec)")
	fs.StringVar(&o.collation, "collation", string(setdata.DefaultCollation), "collation the packs are cut from: "+string(setdata.Play)+", "+string(setdata.Arena))
	fs.IntVar(&o.seats, "seats", 4, "pod size: 4 or 8")
	fs.IntVar(&o.seats, "pod", 4, "alias of -seats")
	fs.StringVar(&o.format, "format", "", "construction format: empty (limited, 40-card decks and Bo1 series) or commander (a Commander Legends style draft: 60-card decks, or -deck-size, around drafted commanders, then 4-player free-for-all Commander games at 40 life; -rounds is the game count, default 1; -structure none builds only)")
	fs.IntVar(&o.deckSize, "deck-size", 0, fmt.Sprintf("-format commander only: the deck size, commanders and basics included, %d..%d; 0 = %d (the lands scale with it: 17 at 60, 14 at 50)", build.MinCommanderDeckSize, build.MaxDeckSize, build.CommanderDeckSize))
	fs.StringVar(&o.decksOut, "decks-out", "", "directory to write every seat's built deck to as a gorge deck JSON file, <set>_e<event seed>_s<seat>.json (a Commander-draft deck marks its commanders); works with -structure none")
	fs.IntVar(&o.picks, "picks", 0, "cards each seat takes from a pack before passing it; 0 = the format's default (1, or 2 with -format commander)")
	fs.StringVar(&o.mode, "mode", string(event.ModeDraft), "supply mode: draft (3 packs per seat, picked), sealed (6 packs per seat, unopened) or prerelease (sealed plus one rare/mythic promo per seat)")
	fs.StringVar(&o.structure, "structure", "", "series structure: roundrobin, swiss or none (build only); empty = roundrobin at 4 seats, swiss at 8")
	fs.IntVar(&o.rounds, "rounds", 0, "event round count; 0 = the structure's default (n-1 round robin, ceil(log2 n) Swiss)")
	fs.Uint64Var(&o.seed, "seed", 1, "event seed; the first event uses it and each later event adds one")
	fs.IntVar(&o.events, "events", 1, "number of independent events to run")
	fs.StringVar(&o.players, "players", "lethal-pressure,bot,forge-heuristic,lethal-pressure",
		"comma-separated bot policy per seat: one name seats it everywhere, otherwise exactly -seats names (a policy that reads decklists, bots.Info.Env, is refused)")
	fs.StringVar(&o.drafter, "drafter", "rating-colour", "pickbot drafter: a registry name or knobs:path.json (a pickbot.Tuning knob file, optionally @<hash>)")
	fs.StringVar(&o.builder, "builder", "best23", "pickbot builder: a registry name or knobs:path.json (a pickbot.Tuning knob file, optionally @<hash>)")
	fs.StringVar(&o.ratingsFile, "ratings-file", "", "training only: JSON {card: {field: value}} replacing the embedded 17Lands ratings for drafter and builder, rescaled to the 17Lands gih_wr mean/std")
	fs.StringVar(&o.ratingsField, "ratings-field", "gih", "which field of -ratings-file to rate by")
	fs.BoolVar(&o.verify, "verify", false, "replay each event from its own log and every game from its chain, exiting non-zero on the first mismatch")
	fs.StringVar(&o.draftlog, "draftlog", "", "path to append the events' picks to as 17Lands draft_data CSV")
	fs.StringVar(&o.gauntlet, "gauntlet", "", "deck-quality gauntlet after each event: play the built (and random baseline) decks against a spellbench catalog; only fdn-limited, and requires -set FDN")
	fs.IntVar(&o.gauntletGames, "gauntlet-games", 2, "games per gauntlet pairing, alternating which side sits in player 0's seat (one on the play's seat, one on the draw's)")
	fs.StringVar(&o.gauntletPlayer, "gauntlet-player", "bot", "bot policy for BOTH gauntlet seats (an Env policy is refused, like the draft seats)")
	fs.StringVar(&o.baseline, "baseline", "random", "gauntlet baseline deck per seat: random (23 random non-land pool cards + AutoLands) or none")
	fs.StringVar(&o.valueLog, "valuelog", "", "path to append one JSON line per Commander pod game to: each seat's commanders, deck, in-hand cards and finish, the input of an offline card-value regression")
	fs.Float64Var(&o.pickExplore, "pick-explore", 0, "probability per pick that a seat takes a random pack card instead of its persona's pick (card-value sampling; 0 = off)")
	fs.BoolVar(&o.verbose, "v", false, "print each seat's policy and deck summary")
	dir := fs.String("dir", ".cards", "corpus directory (holds ir.gob.gz / cardsfolder)")
	fs.Parse(os.Args[1:])
	// Collation is logged on the event only when the flag was set
	// explicitly, so a default-flag log omits it (podShape.collation).
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "collation" {
			o.collationSet = true
		}
	})
	o.dir = *dir

	out := os.Stdout
	if _, err := run(o, out); err != nil {
		fmt.Fprintln(os.Stderr, "draftsim:", err)
		os.Exit(1)
	}
}

// options is one draftsim invocation, gathered from the flags.
type options struct {
	set       string
	collation string
	// collationSet marks -collation as explicitly set on the command line
	// (main's fs.Visit); a literal-built options leaves it false, so the
	// event log omits the collation unless the run named one on purpose.
	collationSet bool
	seats        int
	mode         string
	structure    string
	rounds       int
	seed         uint64
	events       int
	players      string
	drafter      string
	builder      string
	verify       bool
	draftlog     string
	verbose      bool
	dir          string
	// gauntlet, gauntletGames, gauntletPlayer and baseline are the
	// -gauntlet measurement flags (gauntlet.go); gauntletMaxHuman caps the
	// human deck list for tests and is not a flag.
	gauntlet         string
	gauntletGames    int
	gauntletPlayer   string
	baseline         string
	gauntletMaxHuman int
	ratingsFile      string
	ratingsField     string
	// format is -format: "" or event.FormatCommander; picks is -picks;
	// deckSize is -deck-size (0 = build.CommanderDeckSize).
	format   string
	picks    int
	deckSize int
	// decksOut is -decks-out: the directory each seat's built deck is
	// written to as a deck JSON file, "" for none.
	decksOut string
	// valueLog is -valuelog; pickExplore is -pick-explore (valuelog.go).
	valueLog    string
	pickExplore float64
}

// playerList splits the -players flag into its per-seat policies: one name
// seats it at every seat, otherwise the count must be exactly -seats.
func (o options) playerList() ([]string, error) {
	players := strings.Split(o.players, ",")
	for i := range players {
		players[i] = strings.TrimSpace(players[i])
	}
	n, err := o.seatCount()
	if err != nil {
		return nil, err
	}
	if len(players) == 1 && o.players != "" {
		out := make([]string, n)
		for i := range out {
			out[i] = players[0]
		}
		return out, nil
	}
	if len(players) != n {
		return nil, fmt.Errorf("-players names %d seats, want %d (one policy for every seat, or exactly %d)", len(players), n, n)
	}
	return players, nil
}

// seatCount resolves -seats: 0 is the unset value (options literals built
// before the shape flags, and the flag's own default is 4) and means 4.
func (o options) seatCount() (int, error) {
	n := o.seats
	if n == 0 {
		n = draft.Seats
	}
	if n != 4 && n != 8 {
		return 0, fmt.Errorf("-seats must be 4 or 8, got %d", n)
	}
	return n, nil
}

// podShape is one run's resolved pod shape: the seat count, the supply
// mode, the series structure, the per-seat pack count and the event round
// count, with the flag defaults filled in.
type podShape struct {
	seats     int
	mode      string // event.ModeDraft or event.ModeSealed
	structure string // event.StructureRoundRobin, Swiss or None
	packs     int    // packs per seat
	promos    int    // promo cards per seat (prerelease: 1)
	rounds    int    // the event's round count (0 for structure none)
	// collation is the EventCreated's Collation value, "" unless -collation
	// was set explicitly (so a default-flag log omits them).
	collation string
	// format is "" or event.FormatCommander; picks the cards taken per
	// pass (1 unless a Commander draft or -picks says otherwise).
	format string
	picks  int
	// deckSize is a Commander draft's deck size: 0 for
	// build.CommanderDeckSize (and in every limited shape).
	deckSize int
}

// defaultRounds is the round count a structure implies for n seats: n-1
// for the circle-method round robin, ceil(log2 n) for Swiss, none for
// StructureNone.
func defaultRounds(structure string, n int) int {
	switch structure {
	case event.StructureSwiss:
		for r := 0; ; r++ {
			if 1<<uint(r) >= n {
				return r
			}
		}
	case event.StructureNone:
		return 0
	case event.StructurePod:
		return 1
	default:
		return n - 1
	}
}

// shape resolves and validates the pod-shape flags (-seats, -mode,
// -structure, -rounds) and the -draftlog/mode pairing before anything
// expensive runs, so a bad value never pays for the corpus load.
func (o options) shape() (podShape, error) {
	n, err := o.seatCount()
	if err != nil {
		return podShape{}, err
	}
	if o.picks < 0 || o.picks > draft.MaxPicksPerPass {
		return podShape{}, fmt.Errorf("-picks must be 1..%d, got %d", draft.MaxPicksPerPass, o.picks)
	}
	switch o.format {
	case "":
		if o.deckSize != 0 {
			return podShape{}, fmt.Errorf("-deck-size needs -format %s", event.FormatCommander)
		}
	case event.FormatCommander:
		return o.commanderShape(n)
	default:
		return podShape{}, fmt.Errorf("unknown -format %q (want commander, or empty for limited)", o.format)
	}
	picks := o.picks
	if picks == 0 {
		picks = 1
	}
	if picks > 1 && o.mode != "" && o.mode != event.ModeDraft {
		return podShape{}, fmt.Errorf("-picks needs -mode draft")
	}
	mode := event.ModeDraft
	promos := 0
	switch o.mode {
	case "":
	case event.ModeDraft, event.ModeSealed:
		mode = o.mode
	case "prerelease":
		mode = event.ModeSealed
		promos = event.PrereleasePromos
	default:
		return podShape{}, fmt.Errorf("unknown -mode %q (want draft, sealed or prerelease)", o.mode)
	}
	structure := event.StructureRoundRobin
	if n == 8 {
		structure = event.StructureSwiss
	}
	switch o.structure {
	case "":
	case event.StructureRoundRobin, event.StructureSwiss, event.StructureNone:
		structure = o.structure
	default:
		return podShape{}, fmt.Errorf("unknown -structure %q (want roundrobin, swiss or none)", o.structure)
	}
	if o.rounds != 0 {
		if structure == event.StructureNone {
			return podShape{}, fmt.Errorf("-rounds with -structure none takes no rounds")
		}
		if o.rounds < 1 || o.rounds >= n {
			return podShape{}, fmt.Errorf("-rounds must be 1..%d for %d seats, got %d", n-1, n, o.rounds)
		}
	}
	if mode == event.ModeSealed && o.draftlog != "" {
		return podShape{}, fmt.Errorf("-draftlog needs -mode draft (a sealed event has no picks)")
	}
	packs := event.DefaultSealedPacks
	if mode == event.ModeDraft {
		packs = event.DraftPacks
	}
	rounds := o.rounds
	if rounds == 0 {
		rounds = defaultRounds(structure, n)
	}
	collation := ""
	if o.collationSet {
		collation = o.collation
	}
	return podShape{seats: n, mode: mode, structure: structure, packs: packs, promos: promos, rounds: rounds, collation: collation, picks: picks}, nil
}

// commanderShape resolves -format commander: a 4-seat booster draft, two
// picks per pass unless -picks says otherwise, -deck-size cards a deck
// (default build.CommanderDeckSize), and the pod structure, one
// free-for-all game per -rounds (default 1), or -structure none (build
// only, no games).
func (o options) commanderShape(n int) (podShape, error) {
	if n != 4 {
		return podShape{}, fmt.Errorf("-format commander seats a 4-player pod, got -seats %d", n)
	}
	if o.mode != "" && o.mode != event.ModeDraft {
		return podShape{}, fmt.Errorf("-format commander needs -mode draft, got %q", o.mode)
	}
	structure := event.StructurePod
	switch o.structure {
	case "", event.StructurePod:
	case event.StructureNone:
		structure = event.StructureNone
	default:
		return podShape{}, fmt.Errorf("-format commander plays -structure %s or %s, got %q", event.StructurePod, event.StructureNone, o.structure)
	}
	deckSize := o.deckSize
	if deckSize != 0 && (deckSize < build.MinCommanderDeckSize || deckSize > build.MaxDeckSize) {
		return podShape{}, fmt.Errorf("-deck-size must be %d..%d, got %d", build.MinCommanderDeckSize, build.MaxDeckSize, deckSize)
	}
	if deckSize == build.CommanderDeckSize {
		// The default size is logged as no size, so the log matches a
		// run without the flag byte for byte.
		deckSize = 0
	}
	if o.gauntlet != "" {
		return podShape{}, fmt.Errorf("-gauntlet measures limited decks; it does not run with -format commander")
	}
	rounds := o.rounds
	if structure == event.StructureNone {
		if rounds != 0 {
			return podShape{}, fmt.Errorf("-rounds with -structure none takes no rounds")
		}
	} else if rounds == 0 {
		rounds = 1
	}
	if structure == event.StructurePod && (rounds < 1 || rounds > event.MaxPodGames) {
		return podShape{}, fmt.Errorf("-rounds must be 1..%d games with -format commander, got %d", event.MaxPodGames, o.rounds)
	}
	picks := o.picks
	if picks == 0 {
		picks = 2
	}
	collation := ""
	if o.collationSet {
		collation = o.collation
	}
	return podShape{seats: n, mode: event.ModeDraft, structure: structure, packs: event.DraftPacks,
		rounds: rounds, collation: collation, format: event.FormatCommander, picks: picks, deckSize: deckSize}, nil
}
