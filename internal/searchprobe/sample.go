package searchprobe

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"strings"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

// PublicGame has intentionally no original seed, intents, log, or engine.
// Definitions are immutable corpus data for the declared public deck lists.
type PublicGame struct {
	Names        []string
	Decks        [][]*cards.Card
	Tokens       map[string]*cards.Card
	StartingLife int32
}
type History struct {
	Actor   state.PlayerID
	Frames  []Frame
	Answers map[int][]Action
}
type SampleOptions struct {
	Seed                         uint64
	Attempts, Worlds, MaxSubmits int
}
type World struct {
	Config   rules.Config
	Engine   *rules.Engine
	Observer *Collector
}
type SampleResult struct {
	Worlds                                                                   []World `json:"-"`
	Attempts, Accepted, PrefixRejected, BudgetExhausted, Submits, Duplicates int
	ESS                                                                      float64
	FirstRejection                                                           string
}

func Sample(setup PublicGame, h History, opts SampleOptions) (out SampleResult, err error) {
	defer func() {
		var f *Failure
		if err != nil && !errors.As(err, &f) {
			err = fail("invariant", "%v", err)
		}
	}()
	var result SampleResult
	if len(setup.Names) != len(setup.Decks) || len(setup.Names) < 2 || int(h.Actor) >= len(setup.Names) || len(h.Frames) == 0 || opts.Attempts < 1 || opts.Worlds < 1 || opts.MaxSubmits < 1 {
		return result, fmt.Errorf("invalid sampling configuration/history")
	}
	for _, deck := range setup.Decks {
		if len(deck) < 7 {
			return result, fail("unsupported", "undersized genesis deck")
		}
		for _, card := range deck {
			if card == nil || len(card.Faces) == 0 {
				return result, fmt.Errorf("invalid public card definition")
			}
		}
	}
	encoded, err := json.Marshal(h)
	if err != nil {
		return result, err
	}
	digest := sha256.Sum256(encoded)
	seed := binary.LittleEndian.Uint64(digest[:8]) ^ opts.Seed
	var proposals []World
	var logs []float64
	for attempt := 0; attempt < opts.Attempts; attempt++ {
		result.Attempts++
		r := rand.New(rand.NewPCG(seed, uint64(attempt)+0x9e3779b97f4a7c15))
		cfg := rules.Config{Seed: r.Uint64(), Names: setup.Names, Decks: setup.Decks, Tokens: setup.Tokens, StartingLife: setup.StartingLife}
		tape, logWeight, err := genesisProposal(setup, h, r)
		if err != nil {
			return result, err
		}
		e, err := rules.NewHypothetical(cfg, tape)
		if err != nil {
			return result, err
		}
		if err := e.AdvanceHypothetical(); err != nil {
			return result, err
		}
		observer := NewCollector(h.Actor)
		bots := make([]*rand.Rand, len(setup.Names))
		for i := range bots {
			bots[i] = rand.New(rand.NewPCG(r.Uint64(), r.Uint64()))
		}
		pos, submits := 0, 0
		accepted := true
		for i, want := range h.Frames {
			got, err := observer.Capture(e, e.L.Events[pos:])
			if err != nil {
				return result, err
			}
			if !reflect.DeepEqual(got, want) {
				result.PrefixRejected++
				if result.FirstRejection == "" {
					result.FirstRejection = frameDifference(i, got, want)
				}
				accepted = false
				break
			}
			if i == len(h.Frames)-1 {
				break
			}
			if submits >= opts.MaxSubmits {
				result.BudgetExhausted++
				accepted = false
				break
			}
			d := e.Pending()
			if d == nil {
				result.PrefixRejected++
				if result.FirstRejection == "" {
					result.FirstRejection = fmt.Sprintf("frame %d missing pending decision", i)
				}
				accepted = false
				break
			}
			var in decision.Intent
			if d.Player == h.Actor {
				actions, ok := h.Answers[i]
				if !ok {
					return result, fmt.Errorf("missing actor answer at frame %d", i)
				}
				in, err = observer.Match(d, actions)
				if err != nil {
					result.PrefixRejected++
					if result.FirstRejection == "" {
						result.FirstRejection = fmt.Sprintf("frame %d action mismatch: %v", i, err)
					}
					accepted = false
					break
				}
			} else {
				in = botpolicy.Decide(botpolicy.BoardFromGame(e.G, e, d.Player), d, bots[d.Player])
			}
			pos = len(e.L.Events)
			if err := e.SubmitHypothetical(in); err != nil {
				return result, fmt.Errorf("reconstruction submit: %w", err)
			}
			submits++
			result.Submits++
		}
		if accepted {
			result.Accepted++
			proposals = append(proposals, World{Config: cfg, Engine: e, Observer: observer})
			logs = append(logs, logWeight)
		}
	}
	if len(proposals) == 0 {
		return result, nil
	}
	weights, ess, err := normalizeWeights(logs)
	if err != nil {
		return result, err
	}
	result.ESS = ess
	if len(proposals) < opts.Worlds || ess+1e-10 < float64(opts.Worlds) {
		return result, nil
	}
	r := rand.New(rand.NewPCG(seed, 0xa0761d6478bd642f))
	indices, err := weightedIndices(weights, opts.Worlds, r)
	if err != nil {
		return result, err
	}
	seen := make(map[int]bool)
	for _, i := range indices {
		if seen[i] {
			result.Duplicates++
		}
		seen[i] = true
		w := proposals[i]
		w.Engine = w.Engine.Clone()
		w.Observer = w.Observer.clone()
		result.Worlds = append(result.Worlds, w)
	}
	return result, nil
}

func frameDifference(i int, got, want Frame) string {
	part := "decision"
	if !reflect.DeepEqual(got.Identities, want.Identities) {
		part = "identities"
	} else if !reflect.DeepEqual(got.Events, want.Events) {
		part = "events"
	} else if string(got.Board) != string(want.Board) {
		part = "board"
	}
	return fmt.Sprintf("frame %d %s", i, part)
}

// Guide only named actor draws before that library's first non-draw mutation.
// Other chance sites use their prior; whole-prefix rejection handles every
// later shuffle, return, look and reorder without inventing present-day moves.
func genesisProposal(setup PublicGame, h History, r *rand.Rand) ([]rules.ChanceDraw, float64, error) {
	names := make(map[uint32]string)
	for _, f := range h.Frames {
		for _, id := range f.Identities {
			names[id.ID] = id.Name
		}
	}
	fixed := make(map[int]string)
	drawn, shuffled, stopped, toss := 0, false, false, -1
	for _, f := range h.Frames {
		for _, ev := range f.Events {
			if ev.Kind == events.Note && strings.HasSuffix(ev.Text, " won the toss") && toss < 0 {
				toss = int(ev.Player)
			}
			if ev.Kind == events.Shuffle && ev.Player == h.Actor {
				if shuffled {
					stopped = true
				}
				shuffled = true
			}
			if ev.Kind == events.LibraryOrder && ev.Player == h.Actor {
				stopped = true
			}
			// Zone moves do not reliably carry their owner; conservatively stop
			// guidance for either seat's non-draw library mutation.
			if ev.Kind == events.MoveZone && (ev.From == state.ZLibrary || ev.To == state.ZLibrary) {
				stopped = true
			}
			if ev.Kind == events.Draw && ev.Player == h.Actor && !stopped {
				if name := names[ev.Obj]; name != "" {
					fixed[drawn] = name
				}
				drawn++
			}
		}
	}
	if toss < 0 || toss >= len(setup.Names) {
		return nil, 0, fmt.Errorf("missing public genesis toss")
	}
	tape := []rules.ChanceDraw{{Bound: len(setup.Names), Value: toss}}
	logWeight := -math.Log(float64(len(setup.Names)))
	for p, deck := range setup.Decks {
		ids := make([]state.ObjID, len(deck))
		for i := range ids {
			ids[i] = state.ObjID(i + 1)
		}
		var placements []placement
		used := make(map[int]bool)
		if state.PlayerID(p) == h.Actor {
			for pos := 0; pos < len(deck); pos++ {
				name, ok := fixed[pos]
				if !ok {
					continue
				}
				var matching []int
				for j, card := range deck {
					if !used[j] && card.Faces[0].Name == name {
						matching = append(matching, j)
					}
				}
				if len(matching) == 0 {
					return nil, 0, fail("contradictory", "named genesis draws require unavailable card %s", name)
				}
				// Observations name a card, not a physical copy. Uniformly select
				// matching copies and include their multiplicity in p/q.
				logWeight += math.Log(float64(len(matching)))
				j := matching[r.IntN(len(matching))]
				used[j] = true
				placements = append(placements, placement{Index: pos, Card: ids[j]})
			}
		}
		order, factor, err := samplePermutation(ids, placements, r)
		if err != nil {
			return nil, 0, err
		}
		logWeight += factor
		part, err := permutationTape(ids, order)
		if err != nil {
			return nil, 0, err
		}
		tape = append(tape, part...)
	}
	return tape, logWeight, nil
}
