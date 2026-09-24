package main

// -onpolicy-corpus (ticket pn13): record every decision a policynet seat
// SCORED during a -pairs matrix run, with the game's outcome for that seat,
// as the on-policy PPO corpus (policynet.OnPolicyRecord; schema documented in
// internal/policynet/onpolicy.go). The seats play exactly as they do without
// the flag -- the recorder only reads the scores the answer was built from --
// so the bench result is byte-identical with and without it
// (TestOnPolicyCorpusLeavesTheBenchUnchanged).
//
// Records are buffered per game slot and written once, after the matrix, in
// (pair, game, decision) order to a temporary file renamed into place, so the
// file is a pure function of the run's flags, never of the worker count or
// scheduling, and a failed run leaves no partial corpus.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/seat"
)

// onpolicyCorpusPath is the -onpolicy-corpus destination ("" = off), set by
// main after flag.Parse and validated by mainExit.
var onpolicyCorpusPath string

// onpolicyCollector holds one slot per (pair, game).
type onpolicyCollector struct {
	games int
	mu    sync.Mutex
	slots [][]policynet.OnPolicyRecord
}

func newOnPolicyCollector(pairs, games int) *onpolicyCollector {
	return &onpolicyCollector{games: games, slots: make([][]policynet.OnPolicyRecord, pairs*games)}
}

// onpolicyGame is one game's pending records, before the outcome is known.
type onpolicyGame struct {
	pos, gameIndex int
	pair           string
	seed           uint64
	decks          [2]string
	recs           []policynet.OnPolicyRecord
}

// attach installs a recorder on every policynet seat of one game. The game
// is played on one goroutine, so the per-game buffer needs no lock.
func (c *onpolicyCollector) attach(pos, gameIndex int, pair string, seed uint64, decks [2]string, seats []seat.Seat) *onpolicyGame {
	g := &onpolicyGame{pos: pos, gameIndex: gameIndex, pair: pair, seed: seed, decks: decks}
	// The recording checkpoint's feature set's hash (v1: EncoderHash()
	// unchanged), so an mz or entity corpus is labelled as what it is.
	fs := policynet.FeaturesV1
	if policynetModel != nil {
		fs = policynetModel.Features
	}
	hash := fmt.Sprintf("%016x", policynet.EncoderHashFor(fs))
	for si, s := range seats {
		pb, ok := s.(*seat.PolicyNetBot)
		if !ok {
			continue
		}
		seatIdx := si
		pb.SetRecorder(func(d seat.PolicyNetDecision) {
			rec := policynet.OnPolicyRecord{
				RecordType: policynet.OnPolicyRecordType, SchemaVersion: policynet.OnPolicySchemaVersion,
				EncoderHash: hash, Pair: pair, PairIndex: pos, GameIndex: gameIndex, Seed: seed,
				Seat: seatIdx, Deck: decks[seatIdx], Kind: d.Kind, Turn: d.Turn, Sequence: d.Seq,
				Subset: d.Subset, State: policynet.EncodeOnPolicyState(d.State),
				Scores: d.Scores, Chosen: d.Chosen, BotChosen: d.BotChosen,
				ValueOld: d.Value, HasValueOld: d.HasValue, Admission: d.Admission,
			}
			if d.Sampled {
				lp := d.LogPBehaviour
				rec.Temperature, rec.LogPBehaviour = d.Temperature, &lp
			}
			rec.Options = make([]policynet.OnPolicyOption, len(d.Options))
			chosen := make([]bool, len(d.Options))
			for _, p := range d.Chosen {
				chosen[p] = true
			}
			for i := range d.Options {
				rec.Options[i] = policynet.EncodeOnPolicyOption(d.Options[i], d.InSpace[i])
				// The PPOLogProb reads the action space through Target.Labelled.
				d.Options[i].Target.Labelled = d.InSpace[i]
			}
			rec.LogPOld, _ = policynet.PPOLogProb(d.Options, d.Scores, chosen, d.Subset)
			a, b := slices.Clone(d.Chosen), slices.Clone(d.BotChosen)
			slices.Sort(a)
			slices.Sort(b)
			rec.Deviated = !slices.Equal(a, b)
			g.recs = append(g.recs, rec)
		})
	}
	return g
}

// finish stamps the game's outcome onto its records and files them in the
// game's slot. A stalled game's records keep outcome_known false.
func (c *onpolicyCollector) finish(g *onpolicyGame, o gameOutcome) {
	for i := range g.recs {
		r := &g.recs[i]
		switch {
		case o.isStalled():
		case o.winner == "":
			r.Outcome, r.OutcomeKnown = 0.5, true
		case o.winnerSeat == r.Seat:
			r.Outcome, r.OutcomeKnown = 1, true
		default:
			r.Outcome, r.OutcomeKnown = 0, true
		}
	}
	c.mu.Lock()
	c.slots[g.pos*c.games+g.gameIndex] = g.recs
	c.mu.Unlock()
}

// checkOnPolicyDestination is the front-door check: the parent must be a
// directory and the destination must not exist yet.
func checkOnPolicyDestination(path string) error {
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("-onpolicy-corpus parent: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("-onpolicy-corpus parent %q is not a directory", filepath.Dir(path))
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("-onpolicy-corpus destination %q already exists", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking -onpolicy-corpus destination: %w", err)
	}
	return nil
}

// write emits every slot in order to a temporary file and renames it into
// place.
func (c *onpolicyCollector) write(path string) (err error) {
	if err := checkOnPolicyDestination(path); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("creating on-policy corpus temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	w := bufio.NewWriterSize(tmp, 1<<20)
	enc := json.NewEncoder(w)
	for _, slot := range c.slots {
		for i := range slot {
			if err := enc.Encode(&slot[i]); err != nil {
				return fmt.Errorf("writing on-policy corpus: %w", err)
			}
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("writing on-policy corpus: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing on-policy corpus: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming on-policy corpus into place: %w", err)
	}
	return nil
}
