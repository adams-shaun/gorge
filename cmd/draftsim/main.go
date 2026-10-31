// Command draftsim runs headless draft pods (draft-events spec §4.8, E0's
// exit gate): 4 or 8 seats draft (or open sealed packs) with
// draft/limited/pickbot, every seat builds its deck, and the pod's series are
// played as Bo1 games through the rules engine the way cmd/mtgsim drives
// one — round robin by default, Swiss with -structure swiss, or build-only
// with -structure none — with -verify replaying each event from its own
// log. It is the only E0 package that touches the engine; everything under
// draft/limited/ and draft/event/ stays pure.
//
//	draftsim -set FDN -seed 1 -events 5 -verify
//
// The event seed drives everything: the packs (booster.OpenPod), the picks
// (one seeded rand per seat) and the game seeds (derived per
// round/series/game), so the same seed and set rebuild the same event log
// byte for byte. With the default flags the log and output are byte
// identical to the pre-shape-flag runs.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

func main() {
	o := options{}
	fs := flag.NewFlagSet("draftsim", flag.ExitOnError)
	fs.StringVar(&o.set, "set", "FDN", "set code to draft (an embedded draft/limited/setdata spec)")
	fs.StringVar(&o.collation, "collation", string(setdata.DefaultCollation), "collation the packs are cut from: "+string(setdata.Play)+", "+string(setdata.Arena))
	fs.IntVar(&o.seats, "seats", 4, "pod size: 4 or 8")
	fs.StringVar(&o.mode, "mode", string(event.ModeDraft), "supply mode: draft (3 packs per seat, picked) or sealed (6 packs per seat, unopened)")
	fs.StringVar(&o.structure, "structure", "", "series structure: roundrobin, swiss or none (build only); empty = roundrobin at 4 seats, swiss at 8")
	fs.IntVar(&o.rounds, "rounds", 0, "event round count; 0 = the structure's default (n-1 round robin, ceil(log2 n) Swiss)")
	fs.Uint64Var(&o.seed, "seed", 1, "event seed; the first event uses it and each later event adds one")
	fs.IntVar(&o.events, "events", 1, "number of independent events to run")
	fs.StringVar(&o.players, "players", "lethal-pressure,bot,forge-heuristic,lethal-pressure",
		"comma-separated bot policy per seat: one name seats it everywhere, otherwise exactly -seats names (a policy that reads decklists, bots.Info.Env, is refused)")
	fs.StringVar(&o.drafter, "drafter", "rating-colour", "pickbot drafter name")
	fs.StringVar(&o.builder, "builder", "best23", "pickbot builder name")
	fs.StringVar(&o.ratingsFile, "ratings-file", "", "training only: JSON {card: {field: value}} replacing the embedded 17Lands ratings for drafter and builder, rescaled to the 17Lands gih_wr mean/std")
	fs.StringVar(&o.ratingsField, "ratings-field", "gih", "which field of -ratings-file to rate by")
	fs.BoolVar(&o.verify, "verify", false, "replay each event from its own log and every game from its chain, exiting non-zero on the first mismatch")
	fs.StringVar(&o.draftlog, "draftlog", "", "path to append the events' picks to as 17Lands draft_data CSV")
	fs.StringVar(&o.gauntlet, "gauntlet", "", "deck-quality gauntlet after each event: play the built (and random baseline) decks against a spellbench catalog; only fdn-limited, and requires -set FDN")
	fs.IntVar(&o.gauntletGames, "gauntlet-games", 2, "games per gauntlet pairing, alternating which side sits in player 0's seat (one on the play's seat, one on the draw's)")
	fs.StringVar(&o.gauntletPlayer, "gauntlet-player", "bot", "bot policy for BOTH gauntlet seats (an Env policy is refused, like the draft seats)")
	fs.StringVar(&o.baseline, "baseline", "random", "gauntlet baseline deck per seat: random (23 random non-land pool cards + AutoLands) or none")
	fs.BoolVar(&o.verbose, "v", false, "print each seat's policy and deck summary")
	dir := fs.String("dir", ".cards", "corpus directory (holds ir.gob.gz / cardsfolder)")
	fs.Parse(os.Args[1:])
	// Collation is logged on the event only when the flag was set
	// explicitly, so the default-flag logs stay byte-identical to the
	// pre-collation ones (podShape.collation).
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
	rounds    int    // the event's round count (0 for structure none)
	// collation is the EventCreated's Collation value, "" unless -collation
	// was set explicitly (so default-flag logs stay byte-identical).
	collation string
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
	mode := event.ModeDraft
	switch o.mode {
	case "":
	case event.ModeDraft, event.ModeSealed:
		mode = o.mode
	default:
		return podShape{}, fmt.Errorf("unknown -mode %q (want draft or sealed)", o.mode)
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
	return podShape{seats: n, mode: mode, structure: structure, packs: packs, rounds: rounds, collation: collation}, nil
}
