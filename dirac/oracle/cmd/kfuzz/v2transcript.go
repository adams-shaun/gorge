// v2transcript.go: kfuzz -v2 transcript. It plays the same seeded v2 games on
// both SpellBench v2 hosts -- the dirac kernel (dirac/cmd/sbv2dirac, kernel
// authority with a gorge shadow) and gorge's own server/spellbench/env -- with
// the same scripted answerers and diffs every engine-to-agent message after
// canonical JSON.
//
// The v2 host is the agent-facing wire, not the canonical lockstep surface.
// With 5a's gorge shadow, equal transcripts prove the kernel's decision sets
// and flow match gorge; they do not yet prove a kernel-native projection
// (that is 5b).
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/dirac/cmd/sbv2dirac/sbv2"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

// The diff buckets.
// v2MaxPrintedDiffs bounds the diff detail a fuzz run prints; the summary
// still counts them all.
const v2MaxPrintedDiffs = 50

const (
	v2BucketObservation = "observation_field"
	v2BucketOrder       = "candidate_order"
	v2BucketKind        = "candidate_kind"
	v2BucketFlow        = "extra_or_missing_decision"
	v2BucketOutcome     = "outcome"
)

// v2RunSecret is the fixed run secret both hosts derive their game ids and
// game secrets from. All-zero is the sbv2 tests' value.
var v2RunSecret = make([]byte, 32)

// v2MessageHook is the test hook: it may rewrite one host's raw engine message
// before it is recorded for the transcript (never before the host sees it).
// Nil in production, so a run is a pure function of its inputs.
var v2MessageHook func(host string, index int, raw []byte) []byte

// v2Msg is one canonical engine-to-agent message. Request id, game id,
// provenance and the adapter's x_gorge diagnostics are removed: they belong to
// the host, not the game. Object ids are replaced by first-appearance symbols
// (the ids are host-HMAC'd).
type v2Msg struct {
	Kind string // decision or terminal
	Raw  []byte // canonical JSON, host fields removed, ids normalised
	Turn int    // seat_decision.observation.turn (0 for terminal)
}

// v2Diff is one bucketed transcript difference.
type v2Diff struct {
	Answerer string
	Game     int
	Seed     uint64
	Index    int
	Bucket   string
	Path     string
	Dirac    string
	Gorge    string
}

func (d v2Diff) String() string {
	return fmt.Sprintf("v2 diff answerer=%s pair=%d seed=%d decision=%d bucket=%s path=%s dirac=%s gorge=%s",
		d.Answerer, d.Game, d.Seed, d.Index, d.Bucket, d.Path, d.Dirac, d.Gorge)
}

// v2Summary is a -v2 transcript run's totals.
type v2Summary struct {
	Games          int
	HostPairs      int
	Skipped        int
	Truncated      int // host pairs where either host ended at the decision cap
	ProtocolErrors int
	Divergences    int
	Diffs          []v2Diff
	ByBucket       map[string]int
	DiracOutcomes  map[string]int
	GorgeOutcomes  map[string]int
	DiracTurns     [2]int // sum, max
	GorgeTurns     [2]int
}

// v2Side is one host's half of a game.
type v2Side struct {
	Msgs                    []v2Msg
	Turns                   int
	Outcome, Classification string
	Winner, Reason          string
	Err                     error
}

// v2Game is both hosts' play of one seeded game.
type v2Game struct {
	Dirac, Gorge v2Side
	Divergences  []*oracle.Divergence
}

// v2Task is one (answerer, pool pair) game.
type v2Task struct {
	answerer string
	pair     int
	seed     uint64
	a, b     catalog.Deck
	secret   []byte // the game secret both hosts get (v2GameSecret)
	maxDec   int
	noChance bool // -v2-no-chance: the gorge host keeps its own secret-stream shuffles
}

// v2Played is one task's result.
type v2Played struct {
	game v2Game
	err  error
}

// v2Run is the whole run: every answerer over `Games` clean games starting at
// `FirstGame`. Games are independent and play on `Workers` goroutines; the
// summary and the printed diffs are tallied in task order, so the output is
// identical to a serial run.
func v2Run(opts v2TranscriptOpts) (v2Summary, error) {
	sum := v2Summary{DiracOutcomes: map[string]int{}, GorgeOutcomes: map[string]int{}, ByBucket: map[string]int{}}
	if len(opts.Answerers) == 0 {
		return sum, fmt.Errorf("no answerers")
	}
	if opts.MaxDecisions <= 0 {
		opts.MaxDecisions = 500
	}
	pool, err := oracle.ReadPool(opts.PoolPath, opts.Split)
	if err != nil {
		return sum, err
	}
	if len(pool) < 2 {
		return sum, fmt.Errorf("pool %s has %d decks", opts.PoolPath, len(pool))
	}
	cache := map[string]catalog.Deck{}
	load := func(name string) (catalog.Deck, error) {
		if d, ok := cache[name]; ok {
			return d, nil
		}
		d, err := sbv2.ReadDeck(filepath.Join(opts.DeckDir, name+".json"))
		if err != nil {
			return catalog.Deck{}, err
		}
		cache[name] = d
		return d, nil
	}
	probe, err := sbv2.New(opts.Corpus.Reg, nil)
	if err != nil {
		return sum, err
	}
	defer probe.Close()
	var tasks []v2Task
	for _, answerer := range opts.Answerers {
		switch answerer {
		case "first", "rand":
		default:
			return sum, fmt.Errorf("answerer %q: want first or rand", answerer)
		}
		played := 0
		for pair := opts.FirstGame; played < opts.Games; pair++ {
			p := oracle.PairingAt(pool, pair, opts.Seed)
			a, err := load(p.A)
			if err != nil {
				return sum, err
			}
			b, err := load(p.B)
			if err != nil {
				return sum, err
			}
			// Only decks whose every card the kernel plays in lockstep are
			// in scope (the dirac pool). The current FDN pool is
			// wholly clean; the guard keeps a mixed pool honest.
			probe.AddDeck(a)
			probe.AddDeck(b)
			if len(probe.Unclean(a)) > 0 || len(probe.Unclean(b)) > 0 {
				sum.Skipped++
				continue
			}
			secret, err := v2GameSecret(pair)
			if err != nil {
				return sum, err
			}
			tasks = append(tasks, v2Task{answerer: answerer, pair: pair, seed: p.Seed, a: a, b: b, secret: secret, maxDec: opts.MaxDecisions, noChance: opts.NoChance})
			played++
		}
	}
	played := make([]v2Played, len(tasks))
	workers := opts.Workers
	if workers < 1 || v2MessageHook != nil { // a test hook is not goroutine-safe
		workers = 1
	}
	if workers > len(tasks) {
		workers = len(tasks)
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				played[i] = v2PlayTask(opts.Corpus.Reg, tasks[i])
			}
		}()
	}
	for i := range tasks {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	var firstErr error
	printedDiffs := 0
	for i, t := range tasks {
		r := played[i]
		if r.err != nil {
			sum.ProtocolErrors++
			fmt.Fprintf(os.Stderr, "v2: answerer=%s pair=%d seed=%d: %v\n", t.answerer, t.pair, t.seed, r.err)
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		sum.Games++
		sum.HostPairs++
		if d := r.game.Divergences; len(d) > 0 {
			sum.Divergences++
			fmt.Fprintf(os.Stderr, "v2: answerer=%s pair=%d seed=%d: %d kernel divergences, first: %s\n", t.answerer, t.pair, t.seed, len(d), d[0])
		}
		sum.DiracOutcomes[v2Outcome(r.game.Dirac)]++
		sum.GorgeOutcomes[v2Outcome(r.game.Gorge)]++
		if r.game.Dirac.Outcome == "truncated" || r.game.Gorge.Outcome == "truncated" {
			sum.Truncated++
		}
		for _, side := range []struct {
			s   v2Side
			sum *[2]int
		}{{r.game.Dirac, &sum.DiracTurns}, {r.game.Gorge, &sum.GorgeTurns}} {
			side.sum[0] += side.s.Turns
			if side.s.Turns > side.sum[1] {
				side.sum[1] = side.s.Turns
			}
		}
		diffs := v2DiffStreams(t.answerer, t.pair, t.seed, r.game.Dirac.Msgs, r.game.Gorge.Msgs)
		sum.Diffs = append(sum.Diffs, diffs...)
		for _, d := range diffs {
			sum.ByBucket[d.Bucket]++
		}
		if !opts.Quiet && (opts.Verbose || len(diffs) > 0) {
			fmt.Printf("v2 game answerer=%s pair=%d seed=%d dirac=%s/%dt gorge=%s/%dt diffs=%d",
				t.answerer, t.pair, t.seed, v2Outcome(r.game.Dirac), r.game.Dirac.Turns,
				v2Outcome(r.game.Gorge), r.game.Gorge.Turns, len(diffs))
			fmt.Println()
		}
		for _, d := range diffs {
			if opts.Quiet || printedDiffs >= v2MaxPrintedDiffs {
				break
			}
			printedDiffs++
			fmt.Println(d.String())
			fmt.Printf("  repro: kfuzz -v2 transcript -games 1 -first-game %d -seed %d -answerers %s\n", t.pair, opts.Seed, t.answerer)
		}
		if firstErr == nil && len(diffs) > 0 {
			firstErr = fmt.Errorf("%d v2 transcript diffs", len(diffs))
		}
	}
	if firstErr != nil {
		return sum, firstErr
	}
	return sum, nil
}

// v2PlayTask plays one task on both hosts.
func v2PlayTask(reg *cards.Registry, t v2Task) v2Played {
	ds, err := sbv2.New(reg, nil)
	if err != nil {
		return v2Played{err: err}
	}
	ds.SetRecordChance(!t.noChance)
	ds.AddDeck(t.a)
	ds.AddDeck(t.b)
	game, err := v2PlayGame(reg, ds, t)
	game.Divergences = ds.Divergences()
	ds.Close()
	return v2Played{game: game, err: err}
}

// v2PlayGame plays one game on both hosts with the same decks, secret and
// answerer seeds. The dirac host plays first, with the kernel recording its
// random outcomes; the gorge host then follows that record draw for draw
// (v2ChanceFollowingGame), so a shuffle, a coin flip, a die or a random
// selection all take the kernel's value. Neither host is replayed: gorge's
// env plans non-shuffle draws from the match secret (gorge
// sbp-env-chance-streams), so "the env planned it" says nothing about whether
// it matches the kernel; the ordinal chance planner is the following. Under
// -v2-no-chance the gorge host keeps gamecfg's secret-stream chance (the
// negative control).
func v2PlayGame(reg *cards.Registry, ds *sbv2.Server, t v2Task) (v2Game, error) {
	var g v2Game
	names := v2Names(t.a, t.b)
	g.Dirac = v2PlayHost("dirac", ds.Handle, t.a, t.b, names, t.pair, t.secret, t.seed, t.answerer, t.maxDec)
	if g.Dirac.Err != nil {
		return g, fmt.Errorf("dirac: %w", g.Dirac.Err)
	}
	var chance []rules.ChanceDraw
	if !t.noChance {
		tr := ds.Chance()
		if len(tr) == 0 {
			return g, fmt.Errorf("dirac: no kernel chance transcript (it overflowed kernel.MaxChanceRuns)")
		}
		chance = oracle.RulesChance(tr)
	}
	play := func(h *v2GorgeHost) v2Side {
		h.AddDeck(t.a)
		h.AddDeck(t.b)
		return v2PlayHost("gorge", h.Handle, t.a, t.b, names, t.pair, t.secret, t.seed, t.answerer, t.maxDec)
	}
	g.Gorge = play(newV2GorgeHost(reg, chance))
	if g.Gorge.Err != nil {
		return g, fmt.Errorf("gorge: %w", g.Gorge.Err)
	}
	return g, nil
}

// v2PlayHost runs one host through its stdio Handle and records every engine
// response. It speaks the same v2 requests minihost sends (hello, reset, step)
// but without minihost's digest and validator: the transcript is the subject
// here, and both hosts get byte-identical requests.
func v2PlayHost(name string, handle func([]byte) []byte, a, b catalog.Deck, names []string, pair int, secret []byte, seed uint64, answerer string, maxDec int) v2Side {
	var side v2Side
	gameID := secrets.GameID(v2RunSecret, uint64(pair))
	handle([]byte(fmt.Sprintf(`{"request_type":"hello","protocol":"%s","request_id":"h-1","protocol_minor":0}`, protocol.Name)))
	resp := handle(v2Reset(a, b, names, gameID, secret, maxDec))
	rng := [2]*rand.Rand{}
	if answerer == "rand" {
		rng[0] = rand.New(rand.NewPCG(seed, 0))
		rng[1] = rand.New(rand.NewPCG(seed, 1))
	}
	for n := 0; ; n++ {
		raw := resp
		if v2MessageHook != nil {
			raw = v2MessageHook(name, len(side.Msgs), raw)
		}
		msg, err := v2Prepare(raw)
		if err != nil {
			side.Err = fmt.Errorf("%s: record engine message: %w", name, err)
			return side
		}
		side.Msgs = append(side.Msgs, msg)
		if msg.Turn > side.Turns {
			side.Turns = msg.Turn
		}
		switch msg.Kind {
		case "terminal":
			var t struct {
				Outcome        string  `json:"outcome"`
				Classification string  `json:"classification"`
				Winner         *string `json:"winner"`
				Reason         string  `json:"reason"`
			}
			if err := json.Unmarshal(resp, &t); err != nil {
				side.Err = fmt.Errorf("%s: terminal: %w", name, err)
				return side
			}
			side.Outcome, side.Classification, side.Reason = t.Outcome, t.Classification, t.Reason
			if t.Winner != nil {
				side.Winner = *t.Winner
			}
			return side
		case "decision":
		default:
			side.Err = fmt.Errorf("%s: engine response %q", name, msg.Kind)
			return side
		}
		var head struct {
			Step         uint64 `json:"step"`
			SeatDecision struct {
				ActingSeat string `json:"acting_seat"`
				Candidates []struct {
					CandidateID uint32          `json:"candidate_id"`
					Semantic    json.RawMessage `json:"semantic"`
				} `json:"candidates"`
			} `json:"seat_decision"`
		}
		if err := json.Unmarshal(resp, &head); err != nil {
			side.Err = fmt.Errorf("%s: decision: %w", name, err)
			return side
		}
		seat := 0
		if head.SeatDecision.ActingSeat == "p1" {
			seat = 1
		}
		if len(head.SeatDecision.Candidates) == 0 {
			side.Err = fmt.Errorf("%s: decision with no candidates", name)
			return side
		}
		pick := 0
		if answerer == "rand" {
			pick = rng[seat].IntN(len(head.SeatDecision.Candidates))
		}
		sel := head.SeatDecision.Candidates[pick]
		step := fmt.Sprintf(`{"request_type":"step","protocol":"%s","request_id":"s-%d-%d","game_id":%q,"expected_step":%d,"selection":{"candidate_id":%d,"semantic_echo":%s}}`,
			protocol.Name, pair, n, gameID, head.Step, sel.CandidateID, sel.Semantic)
		resp = handle([]byte(step))
	}
}

// v2GameSecret is the game secret both hosts get for pool pair `pair`. It is
// the run secret's secrets.GameSecret(pair), unless the dirac host would have
// to bump the game seed to give seat p0 the toss (dirac/cmd/sbv2dirac/sbv2
// startGame): then it is the first variant index (pair + j<<32) whose seed
// already gives p0 the toss. The kernel and gorge then draw from one seed and
// both consume the toss as their first draw, so the gorge host's ordinary
// (non-shuffle) chance stays aligned with the kernel's, not only its shuffles.
func v2GameSecret(pair int) ([]byte, error) {
	for j := uint64(0); j < 256; j++ {
		secret := secrets.GameSecret(v2RunSecret, uint64(pair)+j<<32)
		sb := secrets.NewGame(secret).StreamSeed("shared", "gorge_seed", 0)
		if rules.TossWinner(binary.BigEndian.Uint64(sb[:8]), 2) == 0 {
			return secret, nil
		}
	}
	return nil, fmt.Errorf("pair %d: no game secret gives seat p0 the toss", pair)
}

// v2Reset is the reset minihost sends, minus its agent profile.
func v2Reset(a, b catalog.Deck, names []string, gameID string, secret []byte, maxDec int) []byte {
	reset := map[string]any{"request_type": "reset", "protocol": protocol.Name, "request_id": "r-1", "game_id": gameID,
		"format": "pauper-bo1",
		"seats": []any{map[string]any{"seat": "p0", "deck": map[string]any{"deck_id": a.DeckID(), "catalog_id": a.CatalogID}},
			map[string]any{"seat": "p1", "deck": map[string]any{"deck_id": b.DeckID(), "catalog_id": b.CatalogID}}},
		"rules": map[string]any{"opponent_decklist": "visible", "mulligan": "none", "starting_player": "host_assigned",
			"starting_seat": "p0", "card_name_domain": map[string]any{"domain_id": wire.DomainID(names), "names": names},
			"extensions": []string{}, "probe": false},
		"game_secret": hex.EncodeToString(secret), "max_decisions": maxDec, "max_steps": 4 * maxDec}
	out, _ := wire.Canonical(reset)
	return out
}

// v2Prepare strips the host fields from an engine message, normalises object
// ids and classifies it.
func v2Prepare(raw []byte) (v2Msg, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return v2Msg{}, err
	}
	var kind string
	if k, ok := top["response_type"]; ok {
		json.Unmarshal(k, &kind)
	}
	for _, k := range []string{"request_id", "game_id", "provenance", "x_gorge"} {
		delete(top, k)
	}
	keys := make([]string, 0, len(top))
	for k := range top {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(top[k])
	}
	buf.WriteByte('}')
	out := v2NormalizeIDs(buf.Bytes())
	return v2Msg{Kind: kind, Raw: out, Turn: v2TurnFromBytes(out)}, nil
}

// v2StripIDBytes is v2NormalizeIDs's id mask with one fixed placeholder.
func v2StripIDBytes(b []byte) []byte {
	var out bytes.Buffer
	for i := 0; i < len(b); {
		if b[i] == '"' && i+20 <= len(b) && b[i+1] == 'o' && b[i+2] == '-' {
			s := string(b[i+1 : i+19])
			if b[i+19] == '"' && v2IsObjID(s) {
				out.WriteString(`"o"`)
				i += 20
				continue
			}
		}
		out.WriteByte(b[i])
		i++
	}
	return out.Bytes()
}

// v2NormalizeIDs replaces every identity object id ("o-" plus 16 lower hex)
// with a symbolic id derived from the object's visible descriptor, not from
// message order. First-appearance numbering is order-sensitive: a reference in
// a different sub-object (or a reordered battlefield) renumbers the whole
// message and reports a spurious id diff. Sorting the descriptors by their
// id-free canonical JSON, then by first byte position to break ties between
// identical objects, gives both hosts the same labels for the same states.
func v2NormalizeIDs(b []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return b
	}
	desc := map[string][]byte{}
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			if id, ok := t["object_id"].(string); ok && v2IsObjID(id) {
				kb, _ := json.Marshal(t)
				kb = v2StripIDBytes(kb)
				if len(kb) > len(desc[id]) {
					desc[id] = kb
				}
			}
			for _, vv := range t {
				walk(vv)
			}
		case []any:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(v)
	type ent struct {
		id  string
		key []byte
		at  int
	}
	ents := make([]ent, 0, len(desc))
	for id, k := range desc {
		at := bytes.Index(b, []byte(`"`+id+`"`))
		if at < 0 {
			at = len(b)
		}
		ents = append(ents, ent{id: id, key: k, at: at})
	}
	sort.Slice(ents, func(i, j int) bool {
		if c := bytes.Compare(ents[i].key, ents[j].key); c != 0 {
			return c < 0
		}
		return ents[i].at < ents[j].at
	})
	ids := make(map[string]string, len(ents))
	for i, e := range ents {
		ids[e.id] = fmt.Sprintf("o-%d", i)
	}
	var out bytes.Buffer
	for i := 0; i < len(b); {
		if b[i] == '"' && i+20 <= len(b) && b[i+1] == 'o' && b[i+2] == '-' {
			s := string(b[i+1 : i+19])
			if b[i+19] == '"' && v2IsObjID(s) {
				out.WriteByte('"')
				if sym, ok := ids[s]; ok {
					out.WriteString(sym)
				} else {
					out.WriteString(s)
				}
				out.WriteByte('"')
				i += 20
				continue
			}
		}
		out.WriteByte(b[i])
		i++
	}
	return out.Bytes()
}

// v2IsObjID is the identity package's ObjectID shape: "o-" plus 16 lowercase
// hex digits (secrets.Game.ObjectID).
func v2IsObjID(s string) bool {
	if len(s) != 18 || s[:2] != "o-" {
		return false
	}
	for i := 2; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// v2TurnFromBytes reads observation.turn without parsing the whole message.
func v2TurnFromBytes(b []byte) int {
	i := bytes.Index(b, []byte(`"turn":`))
	if i < 0 {
		return 0
	}
	i += len(`"turn":`)
	n := 0
	for ; i < len(b) && b[i] >= '0' && b[i] <= '9'; i++ {
		n = n*10 + int(b[i]-'0')
	}
	return n
}

// v2Outcome is a game's outcome/winner for the summary.
func v2Outcome(s v2Side) string {
	switch {
	case s.Outcome == "":
		return "none"
	case s.Winner != "":
		return s.Outcome + "/" + s.Winner
	default:
		return s.Outcome
	}
}

// v2Decisions keeps the agent-visible engine messages, in order.
func v2Decisions(msgs []v2Msg) []v2Msg {
	out := make([]v2Msg, 0, len(msgs))
	for _, m := range msgs {
		if m.Kind == "decision" || m.Kind == "terminal" {
			out = append(out, m)
		}
	}
	return out
}

// v2DiffStreams compares two hosts' engine-to-agent streams, whole.
func v2DiffStreams(answerer string, game int, seed uint64, dirac, gorge []v2Msg) []v2Diff {
	d, g := v2Decisions(dirac), v2Decisions(gorge)
	var out []v2Diff
	add := func(index int, bucket, path, dv, gv string) {
		out = append(out, v2Diff{Answerer: answerer, Game: game, Seed: seed, Index: index, Bucket: bucket, Path: path, Dirac: dv, Gorge: gv})
	}
	end := len(d)
	if len(g) > end {
		end = len(g)
	}
	for i := 0; i < end; i++ {
		switch {
		case i >= len(d):
			add(i, v2BucketFlow, "decision", "end", g[i].Kind)
			continue
		case i >= len(g):
			add(i, v2BucketFlow, "decision", d[i].Kind, "end")
			continue
		case d[i].Kind != g[i].Kind:
			add(i, v2BucketFlow, "response_type", d[i].Kind, g[i].Kind)
			continue
		}
		if bytes.Equal(d[i].Raw, g[i].Raw) {
			continue
		}
		dm, err := v2Parse(d[i].Raw)
		if err != nil {
			add(i, v2BucketFlow, "parse", err.Error(), "")
			continue
		}
		gm, err := v2Parse(g[i].Raw)
		if err != nil {
			add(i, v2BucketFlow, "parse", "", err.Error())
			continue
		}
		switch d[i].Kind {
		case "decision":
			v2DiffDecision(i, dm, gm, add)
		case "terminal":
			v2DiffTerminal(i, dm, gm, add)
		}
	}
	return out
}

func v2Parse(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func v2DiffDecision(index int, dm, gm map[string]any, add func(int, string, string, string, string)) {
	if !v2Equal(dm["step"], gm["step"]) {
		add(index, v2BucketFlow, "step", v2Val(dm["step"]), v2Val(gm["step"]))
	}
	dsd, _ := dm["seat_decision"].(map[string]any)
	gsd, _ := gm["seat_decision"].(map[string]any)
	for _, k := range []string{"acting_seat", "seat_step"} {
		if !v2Equal(dsd[k], gsd[k]) {
			add(index, v2BucketFlow, "seat_decision."+k, v2Val(dsd[k]), v2Val(gsd[k]))
		}
	}
	v2DiffLeaves(index, "seat_decision.group", dsd["group"], gsd["group"], v2BucketFlow, add)
	v2DiffLeaves(index, "seat_decision.context", dsd["context"], gsd["context"], v2BucketFlow, add)
	v2DiffLeaves(index, "seat_decision.observation", dsd["observation"], gsd["observation"], v2BucketObservation, add)
	v2DiffCandidates(index, dsd["candidates"], gsd["candidates"], add)
	if !v2Equal(dsd["extensions"], gsd["extensions"]) {
		v2DiffLeaves(index, "seat_decision.extensions", dsd["extensions"], gsd["extensions"], v2BucketFlow, add)
	}
}

func v2DiffTerminal(index int, d, g map[string]any, add func(int, string, string, string, string)) {
	// step_count and decision_count are host bookkeeping, not game outcomes:
	// the terminal's outcome/winner/reason is what must agree.
	for _, k := range []string{"outcome", "classification", "winner", "reason"} {
		if !v2Equal(d[k], g[k]) {
			add(index, v2BucketOutcome, k, v2Val(d[k]), v2Val(g[k]))
		}
	}
}

func v2DiffCandidates(index int, dc, gc any, add func(int, string, string, string, string)) {
	a, aok := dc.([]any)
	b, bok := gc.([]any)
	if !aok || !bok {
		add(index, v2BucketOrder, "seat_decision.candidates", v2Val(dc), v2Val(gc))
		return
	}
	if len(a) != len(b) {
		add(index, v2BucketOrder, "seat_decision.candidates", fmt.Sprint(len(a)), fmt.Sprint(len(b)))
		return
	}
	for i := range a {
		am, _ := a[i].(map[string]any)
		bm, _ := b[i].(map[string]any)
		if !v2Equal(am["candidate_id"], bm["candidate_id"]) {
			add(index, v2BucketOrder, fmt.Sprintf("seat_decision.candidates[%d].candidate_id", i), v2Val(am["candidate_id"]), v2Val(bm["candidate_id"]))
			continue
		}
		ak := v2SemanticKind(am)
		bk := v2SemanticKind(bm)
		if ak != bk {
			add(index, v2BucketKind, fmt.Sprintf("seat_decision.candidates[%d].semantic.kind", i), ak, bk)
			continue
		}
		if !v2Equal(am, bm) {
			add(index, v2BucketOrder, fmt.Sprintf("seat_decision.candidates[%d]", i), v2Val(am), v2Val(bm))
		}
	}
}

func v2SemanticKind(c map[string]any) string {
	sem, _ := c["semantic"].(map[string]any)
	k, _ := sem["kind"].(string)
	return k
}

// v2DiffLeaves reports every differing leaf under path with one bucket.
func v2DiffLeaves(index int, path string, a, b any, bucket string, add func(int, string, string, string, string)) {
	if v2Equal(a, b) {
		return
	}
	if am, ok := a.(map[string]any); ok {
		if bm, ok := b.(map[string]any); ok {
			keys := make([]string, 0, len(am)+len(bm))
			seen := map[string]bool{}
			for k := range am {
				seen[k] = true
				keys = append(keys, k)
			}
			for k := range bm {
				if !seen[k] {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			for _, k := range keys {
				v2DiffLeaves(index, path+"."+k, am[k], bm[k], bucket, add)
			}
			return
		}
	}
	if aa, ok := a.([]any); ok {
		if ba, ok := b.([]any); ok && len(aa) == len(ba) {
			for i := range aa {
				v2DiffLeaves(index, fmt.Sprintf("%s[%d]", path, i), aa[i], ba[i], bucket, add)
			}
			return
		}
	}
	add(index, bucket, path, v2Val(a), v2Val(b))
}

func v2Equal(a, b any) bool { return reflect.DeepEqual(a, b) }

func v2Val(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// v2Names is the distinct card-name domain of a game, in first-seen order.
func v2Names(decks ...catalog.Deck) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range decks {
		for _, r := range d.Rows {
			if !seen[r.Name] {
				seen[r.Name] = true
				out = append(out, r.Name)
			}
		}
	}
	return out
}

// runV2Transcript is the -v2 transcript command.
func runV2Transcript(opts v2TranscriptOpts) int {
	sum, err := v2Run(opts)
	if sum.Games == 0 && err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	fmt.Printf("v2 transcript: answerers %s  games %d  host pairs %d  diffs %d  truncated %d  protocol errors %d  divergences %d  skipped %d\n",
		strings.Join(opts.Answerers, ","), opts.Games, sum.HostPairs, len(sum.Diffs), sum.Truncated, sum.ProtocolErrors, sum.Divergences, sum.Skipped)
	fmt.Printf("  dirac outcomes: %s  %s\n", v2OutcomeLine(sum.DiracOutcomes), v2TurnsLine(sum.DiracTurns, sum.HostPairs))
	fmt.Printf("  gorge outcomes: %s  %s\n", v2OutcomeLine(sum.GorgeOutcomes), v2TurnsLine(sum.GorgeTurns, sum.HostPairs))
	// Both hosts get the canonical decks and one game secret, and the gorge
	// host follows the dirac kernel's recorded chance: the game is built with
	// the kernel's transcript as a positional chance planner, so every draw
	// (shuffle, toss, coin flip, die, selection) is the kernel's. The planner
	// is cloned away, so a mapping probe still draws from its own generator.
	if opts.NoChance {
		fmt.Println("  note: -v2-no-chance negative control: the gorge host keeps its secret-stream chance; this run MUST diff")
	} else {
		fmt.Println("  note: the gorge host follows the dirac kernel's recorded chance draw for draw; pass -v2-no-chance for the negative control")
	}
	if len(sum.ByBucket) > 0 {
		fmt.Printf("  diffs by bucket: %s\n", v2OutcomeLine(sum.ByBucket))
	}
	if opts.NoChance && len(sum.Diffs) == 0 {
		fmt.Fprintln(os.Stderr, "kfuzz: -v2-no-chance expected the independent shuffles to diff, but no diff was recorded")
		return 1
	}
	if len(sum.Diffs) > 0 || sum.ProtocolErrors > 0 || sum.Divergences > 0 || err != nil {
		if err != nil && len(sum.Diffs) == 0 {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
		}
		return 1
	}
	return 0
}

func v2OutcomeLine(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "%s=%d", k, m[k])
	}
	if b.Len() == 0 {
		return "none"
	}
	return b.String()
}

func v2TurnsLine(t [2]int, games int) string {
	avg := 0.0
	if games > 0 {
		avg = float64(t[0]) / float64(games)
	}
	return fmt.Sprintf("turns avg=%.1f max=%d", avg, t[1])
}
