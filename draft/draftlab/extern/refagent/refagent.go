// Package refagent is gorge's reference gorge-agent/1 agent: a small
// uniform-random, first-legal (and deliberately illegal, for tests) agent
// used by tests, benchmarks and as a worked example of the wire. It is not
// a training artifact and makes no effort to play well; the Python twin
// (examples/extagent/) need not agree with it bit for bit.
package refagent

import (
	"bytes"
	"fmt"
	"io"
	"math/rand/v2"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/protocol"
)

// Config configures a served agent.
type Config struct {
	// Agent is the hello's agent name; "" means Policy.
	Agent string
	// ModelVersion is the hello's model_version; "" means "1".
	ModelVersion string
	// Policy is "random" (default), "first" or "illegal".
	Policy string
	// Kinds is the hello's kinds; nil means all three.
	Kinds []string
	// MaxBatch is the hello's max_batch; 0 means no preference.
	MaxBatch int
	// SkipForced, PaymentActions and TargetHints are the hello's matching
	// boolean capability flags.
	SkipForced     bool
	PaymentActions bool
	TargetHints    bool
	// ViewOmit names the view parts the policies never read.
	ViewOmit []string
	// DieAfter, when > 0, ends the session with an error when batch
	// DieAfter+1 arrives (before it is answered); that many batches are
	// answered first, and a bye before then still ends the session
	// cleanly (a test hook).
	DieAfter int
	// ObserveItem is called on the serving goroutine with every request
	// item, before it is answered (a test hook).
	ObserveItem func(it wire.Item)
	// ObserveBatch is called with every batch's item count (a test hook).
	ObserveBatch func(n int)
}

// Serve reads gorge's hello, answers with the Config's own hello, then
// serves batches until a bye or a clean end of stream, returning nil. A
// malformed or wrongly-directed message ends the session with an error
// (and, for a bad hello, an error message to gorge).
func Serve(r io.Reader, w io.Writer, c Config) error {
	wr := wire.NewWriter(w)
	rd := wire.NewReader(r, 0)
	hello, err := rd.Read()
	if err != nil {
		return err
	}
	if err := hello.Validate(wire.FromGorge); err != nil || hello.Type != wire.THello {
		_ = wr.Write(wire.Message{V: wire.Version, Type: wire.TError, Code: wire.CodeUnsupported, Message: "bad hello"})
		if err != nil {
			return err
		}
		return fmt.Errorf("refagent: bad hello: type %q", hello.Type)
	}
	if err := wr.Write(agentHello(c)); err != nil {
		return err
	}
	served := 0
	kinds := servedKinds(c)
	for {
		m, err := rd.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch m.Type {
		case wire.TBatch:
			if c.DieAfter > 0 && served >= c.DieAfter {
				return fmt.Errorf("refagent: died after %d batches", served)
			}
			if err := m.Validate(wire.FromGorge); err != nil {
				return err
			}
			if c.ObserveBatch != nil {
				c.ObserveBatch(len(m.Items))
			}
			answers := make([]wire.Answer, 0, len(m.Items))
			for _, it := range m.Items {
				if c.ObserveItem != nil {
					c.ObserveItem(it)
				}
				answers = append(answers, answerOne(c, kinds, it))
			}
			if err := wr.Write(wire.Message{V: wire.Version, Type: wire.TResult, Batch: m.Batch, Answers: answers}); err != nil {
				return err
			}
			served++
		case wire.TStreamEnd:
			// That seat's game ended; the reference agent keeps no
			// per-stream state to drop.
		case wire.TBye:
			return nil
		default:
			_ = wr.Write(wire.Message{V: wire.Version, Type: wire.TError, Code: wire.CodeBadRequest, Message: "unexpected message"})
			return fmt.Errorf("refagent: unexpected message type %q", m.Type)
		}
	}
}

func agentHello(c Config) wire.Message {
	policy := normalizedPolicy(c.Policy)
	name := c.Agent
	if name == "" {
		name = policy
	}
	mv := c.ModelVersion
	if mv == "" {
		mv = "1"
	}
	return wire.Message{
		V:              wire.Version,
		Type:           wire.THello,
		Protocol:       wire.Protocol,
		Agent:          name,
		ModelVersion:   mv,
		Kinds:          servedKinds(c),
		MaxBatch:       c.MaxBatch,
		SkipForced:     c.SkipForced,
		PaymentActions: c.PaymentActions,
		TargetHints:    c.TargetHints,
		ViewOmit:       c.ViewOmit,
	}
}

func normalizedPolicy(p string) string {
	if p == "" {
		return "random"
	}
	return p
}

func servedKinds(c Config) []string {
	if len(c.Kinds) == 0 {
		return []string{wire.KindPick, wire.KindBuild, wire.KindDecide}
	}
	return append([]string(nil), c.Kinds...)
}

func answerOne(c Config, kinds []string, it wire.Item) wire.Answer {
	for _, k := range kinds {
		if k == it.Kind {
			return Answer(c, it)
		}
	}
	return wire.Answer{ID: it.ID, Error: &wire.ItemError{Code: wire.CodeRefused, Message: "kind not served"}}
}

// Answer answers one request item with the Config's policy. It is a pure
// function of (the policy, the item): every policy seeds its rng from the
// item's seed alone, so the same item always yields the same answer.
func Answer(c Config, it wire.Item) wire.Answer {
	policy := normalizedPolicy(c.Policy)
	rng := rand.New(rand.NewPCG(it.Seed, 0))
	switch policy {
	case "first":
		return firstAnswer(it, rng)
	case "illegal":
		return illegalAnswer(it)
	default:
		return randomAnswer(it, rng)
	}
}

func randomAnswer(it wire.Item, rng *rand.Rand) wire.Answer {
	switch it.Kind {
	case wire.KindPick:
		return wire.Answer{ID: it.ID, Pick: it.Pack[rng.IntN(len(it.Pack))].ID}
	case wire.KindBuild:
		return wire.Answer{ID: it.ID, Deck: randomBuild(rng, it.Pool, it.DeckSize)}
	case wire.KindDecide:
		d, err := wire.DecisionOf(it.View)
		if err != nil {
			return wire.Answer{ID: it.ID, Error: &wire.ItemError{Code: wire.CodeRefused, Message: err.Error()}}
		}
		return randomDecide(rng, d, it.ID)
	default:
		return wire.Answer{ID: it.ID, Error: &wire.ItemError{Code: wire.CodeRefused, Message: "kind not served"}}
	}
}

// randomBuild builds a deck from the pool's non-land cards whose colours
// lie in the pool's two most frequent colours (ties in WUBRG order), cut to
// and topped up to 23, the rest made up of basics split between the pair.
func randomBuild(rng *rand.Rand, pool []protocol.DraftCard, deckSize int) *protocol.DeckRequest {
	var nonLand []protocol.DraftCard
	counts := [5]int{}
	for _, c := range pool {
		if strings.Contains(c.TypeLine, "Land") {
			continue
		}
		nonLand = append(nonLand, c)
		for i, l := range wubrg {
			if strings.ContainsRune(c.Colors, l) {
				counts[i]++
			}
		}
	}
	order := []int{0, 1, 2, 3, 4}
	sort.SliceStable(order, func(i, j int) bool { return counts[order[i]] > counts[order[j]] })
	var pair []byte
	for _, i := range order[:2] {
		if counts[i] > 0 {
			pair = append(pair, wubrg[i])
		}
	}
	for _, l := range []byte{'W', 'U'} {
		if len(pair) < 2 && !bytes.ContainsRune(pair, rune(l)) {
			pair = append(pair, l)
		}
	}
	inPair := func(colors string) bool {
		for _, r := range colors {
			if !bytes.ContainsRune(pair, r) {
				return false
			}
		}
		return true
	}
	var matched, rest []protocol.DraftCard
	for _, c := range nonLand {
		if inPair(c.Colors) {
			matched = append(matched, c)
		} else {
			rest = append(rest, c)
		}
	}
	rng.Shuffle(len(matched), func(i, j int) { matched[i], matched[j] = matched[j], matched[i] })
	main := matched
	if len(main) > 23 {
		main = main[:23]
	}
	if need := 23 - len(main); need > 0 && len(rest) > 0 {
		rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
		if len(rest) < need {
			need = len(rest)
		}
		main = append(main, rest[:need]...)
	}
	var basics [5]int
	n := deckSize - len(main)
	first := (n + 1) / 2
	basics[letterIndex(pair[0])] = first
	basics[letterIndex(pair[1])] = n - first
	req := &protocol.DeckRequest{Basics: basics}
	for _, c := range main {
		req.Main = append(req.Main, c.ID)
	}
	return req
}

const wubrg = "WUBRG"

func letterIndex(l byte) int {
	for i := 0; i < 5; i++ {
		if wubrg[i] == l {
			return i
		}
	}
	return 0
}

func randomDecide(rng *rand.Rand, d *decision.Decision, id uint64) wire.Answer {
	var candidates []decision.Option
	for _, o := range d.Options {
		if o.Kind != "concede" {
			candidates = append(candidates, o)
		}
	}
	hi := d.Max
	if hi < 0 {
		hi = 0
	}
	if len(candidates) < hi {
		hi = len(candidates)
	}
	lo := d.Min
	if lo > hi {
		lo = hi
	}
	k := lo
	if hi > lo {
		k += rng.IntN(hi - lo + 1)
	}
	perm := rng.Perm(len(candidates))
	intent := &decision.Intent{Choices: make([]int, 0, k)}
	for _, p := range perm[:k] {
		intent.Choices = append(intent.Choices, candidates[p].Index)
	}
	return wire.Answer{ID: id, Intent: intent}
}

func firstAnswer(it wire.Item, rng *rand.Rand) wire.Answer {
	switch it.Kind {
	case wire.KindPick:
		return wire.Answer{ID: it.ID, Pick: it.Pack[0].ID}
	case wire.KindBuild:
		return wire.Answer{ID: it.ID, Deck: randomBuild(rng, it.Pool, it.DeckSize)}
	case wire.KindDecide:
		d, err := wire.DecisionOf(it.View)
		if err != nil {
			return wire.Answer{ID: it.ID, Error: &wire.ItemError{Code: wire.CodeRefused, Message: err.Error()}}
		}
		var candidates []decision.Option
		for _, o := range d.Options {
			if o.Kind != "concede" {
				candidates = append(candidates, o)
			}
		}
		if d.Kind == decision.KPriority {
			for _, o := range candidates {
				if o.Kind == "pass" {
					return wire.Answer{ID: it.ID, Intent: &decision.Intent{Choices: []int{o.Index}}}
				}
			}
		}
		n := d.Min
		if n < 0 {
			n = 0
		}
		if len(candidates) < n {
			n = len(candidates)
		}
		choices := make([]int, 0, n)
		for _, o := range candidates[:n] {
			choices = append(choices, o.Index)
		}
		return wire.Answer{ID: it.ID, Intent: &decision.Intent{Choices: choices}}
	default:
		return wire.Answer{ID: it.ID, Error: &wire.ItemError{Code: wire.CodeRefused, Message: "kind not served"}}
	}
}

func illegalAnswer(it wire.Item) wire.Answer {
	switch it.Kind {
	case wire.KindPick:
		return wire.Answer{ID: it.ID, Pick: "no-such-card"}
	case wire.KindBuild:
		return wire.Answer{ID: it.ID, Deck: &protocol.DeckRequest{}}
	case wire.KindDecide:
		d, err := wire.DecisionOf(it.View)
		if err != nil {
			return wire.Answer{ID: it.ID, Error: &wire.ItemError{Code: wire.CodeRefused, Message: err.Error()}}
		}
		maxIdx := 0
		for _, o := range d.Options {
			if o.Index > maxIdx {
				maxIdx = o.Index
			}
		}
		return wire.Answer{ID: it.ID, Intent: &decision.Intent{Choices: []int{maxIdx + 1}}}
	default:
		return wire.Answer{ID: it.ID, Error: &wire.ItemError{Code: wire.CodeRefused, Message: "kind not served"}}
	}
}
