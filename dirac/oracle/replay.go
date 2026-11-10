package oracle

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// ChanceDraw is one forced random outcome: the engine's IntN(Bound) returns
// Value (rules.ChanceDraw's shape, so a kernel need not import gorge).
type ChanceDraw struct {
	Bound int `json:"bound"`
	Value int `json:"value"`
}

// ChanceSource is the optional kernel extra Replay's forced outcomes need:
// every IntN the kernel drew, in order.
type ChanceSource interface {
	ChanceTranscript() []ChanceDraw
}

// ChanceTranscript implements ChanceSource for a GorgeSide built with
// recording on (RecordChance); an ordinary engine records nothing.
func (g *GorgeSide) ChanceTranscript() []ChanceDraw {
	var out []ChanceDraw
	for _, d := range g.E.ChanceTranscript() {
		out = append(out, ChanceDraw{Bound: d.Bound, Value: d.Value})
	}
	return out
}

// RulesChance translates recorded draws to gorge's chance shape: the forced
// outcomes NewGorgeSide consumes and the prefix gamecfg takes.
func RulesChance(tr []ChanceDraw) []rules.ChanceDraw {
	var out []rules.ChanceDraw
	for _, d := range tr {
		out = append(out, rules.ChanceDraw{Bound: d.Bound, Value: d.Value})
	}
	return out
}

// KernelGame is a game a kernel played on its own, recorded for
// re-execution in gorge: the matchup, the seed, the canonical actions, the
// kernel's projection hash at every decision, and optionally its random
// outcomes (forced on gorge, so a kernel game from a redealt or reseeded
// world replays too).
type KernelGame struct {
	Format      string      `json:"format"` // "koracle-game-2"
	Decks       [2]string   `json:"decks"`
	Cards       [2][]string `json:"cards"`
	Seed        uint64      `json:"seed"`
	Mulligans   int         `json:"mulligans"`
	PassElision bool        `json:"pass_elision"`
	// Commander and Commanders are the matchup's (Repro's same fields).
	Commander  bool  `json:"commander,omitempty"`
	Commanders []int `json:"commanders,omitempty"`
	// AutopaySurface: the game was played on gorge's autopay surface
	// (Config.GorgeAutopaySurface), so gorge replays its plain casts as
	// planned casts, never by lowering a potential play
	AutopaySurface bool         `json:"autopay_surface,omitempty"`
	Actions        []Canon      `json:"actions"`
	Hashes         []uint64     `json:"hashes,omitempty"` // Hashes[i]: projection at decision i (before Actions[i]); one more at the end
	Chance         []ChanceDraw `json:"chance,omitempty"`
	Over           bool         `json:"over"`
	Winner         int8         `json:"winner"`
}

const (
	gameFormat = "koracle-game-2"
	// gameFormat1 is the format before the target byte was repacked
	// (repro.go, format1Ref).
	gameFormat1 = "koracle-game-1"
)

// compareHash is the hash a replay compares: the projection as the
// lockstep compares it (the priority holder is not compared -- outside a
// priority decision the engines' stale holders are not game state -- and a
// finished game compares by its result only).
func compareHash(p *Projection) uint64 {
	q := *p
	q.Priority = 0
	// Chars are read through a kernel table's vocabulary, and a replay has no table. The
	// lockstep compares them.
	for i := range q.Cards {
		q.Cards[i].Chars = CharsProj{}
	}
	if q.Over {
		q = Projection{Over: true, Winner: q.Winner, Life: q.Life}
	}
	return ProjectionHash(&q)
}

// cmdProjBytes is the encoded size of Projection's trailing CmdCasts and
// CmdDamage.
const cmdProjBytes = 2 + 2*2

// ProjectionHash is FNV-1a 64 over p's fixed-size little-endian encoding.
func ProjectionHash(p *Projection) uint64 {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, p)
	b := buf.Bytes()
	if p.CmdCasts == [2]uint8{} && p.CmdDamage == [2][2]uint8{} {
		// a game with no commander bookkeeping hashes as it did before the
		// fields existed (they are the encoding's last bytes)
		b = b[:len(b)-cmdProjBytes]
	}
	h := fnv.New64a()
	h.Write(b)
	return h.Sum64()
}

// RecordKernelGame plays m on seed with k (a kernel through KernelSide) alone under pol and records it.
// The policy's PickCtx has no GorgeSide (G is nil). maxDecisions caps it.
func RecordKernelGame(m *Matchup, seed uint64, k Side, pol Policy, cfg Config, maxDecisions int) *KernelGame {
	g := &KernelGame{Format: gameFormat, Decks: m.Names, Seed: seed, Mulligans: m.Mulligans, PassElision: cfg.PassElision, Winner: -1}
	g.Commander, g.Commanders = m.commanderFields()
	for s := 0; s < 2; s++ {
		for _, c := range m.Decks[s] {
			g.Cards[s] = append(g.Cards[s], cardName(c))
		}
	}
	if maxDecisions <= 0 {
		maxDecisions = 20000
	}
	var p Projection
	for n := 0; ; n++ {
		d := k.Step()
		k.Project(&p)
		g.Hashes = append(g.Hashes, compareHash(&p))
		if d.Kind == DNone || n >= maxDecisions {
			break
		}
		a := pol.Pick(&PickCtx{D: &d, Seq: n})
		g.Actions = append(g.Actions, a)
		k.Apply(a)
	}
	g.Over, g.Winner = k.Terminal()
	if cs, ok := k.(ChanceSource); ok {
		g.Chance = cs.ChanceTranscript()
	}
	return g
}

// Write saves the game as JSON.
func (g *KernelGame) Write(path string) error {
	b, err := json.Marshal(g)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// ReadKernelGame loads a recorded game.
func ReadKernelGame(path string) (*KernelGame, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g KernelGame
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, err
	}
	switch g.Format {
	case gameFormat:
	case gameFormat1:
		// The recorded projection hashes encode the old Ref bytes and cannot
		// be translated; a replay without them compares actions only.
		g.Actions = format1Actions(g.Actions)
		g.Hashes = nil
		g.Format = gameFormat
	default:
		return nil, fmt.Errorf("game %s: format %q, want %q", path, g.Format, gameFormat)
	}
	return &g, nil
}

// Matchup rebuilds the game's matchup from its card names.
func (g *KernelGame) Matchup(reg *cards.Registry) (*Matchup, error) {
	r := Repro{Decks: g.Decks, Cards: g.Cards, Mulligans: g.Mulligans, Commander: g.Commander, Commanders: g.Commanders}
	return r.Matchup(reg)
}

// ReplayResult is a kernel game re-executed in gorge.
type ReplayResult struct {
	Applied int // actions gorge accepted
	// DivergedAt is the first diverging sequence number (the canonical
	// decision index), -1 when gorge reproduced the whole game.
	DivergedAt int
	// Reason: "not_offered" (gorge does not offer Actions[DivergedAt]),
	// "projection" (gorge's projection hash differs before it), "options"
	// unused here, "terminal", "chance" (a forced outcome's bound differs
	// from what gorge asked) or "gorge" (the gorge side failed).
	Reason string
	Detail string
	// Log is gorge's own record of the re-execution: the decision.Intents it
	// submitted, internal answers included, and every event.
	Log *events.Log
	// Verified is engine/replay's verdict on Log: nil when the log replays
	// event for event (forced outcomes: through rules.NewHypothetical).
	Verified error
}

// Replay re-executes kernel game g in gorge: it maps each canonical action to
// gorge's intents (GorgeSide.Apply), compares gorge's projection hash with
// the recorded one at every decision gorge committed before, and reports the
// first diverging sequence number. g.Chance, when set, forces gorge's random
// outcomes (rules.NewHypothetical). The resulting gorge log is then verified
// by engine/replay.
func Replay(reg *cards.Registry, g *KernelGame) (res ReplayResult) {
	res = ReplayResult{DivergedAt: -1}
	m, err := g.Matchup(reg)
	if err != nil {
		res.DivergedAt, res.Reason, res.Detail = 0, "gorge", err.Error()
		return res
	}
	chance := RulesChance(g.Chance)
	side, err := NewGorgeSide(m, g.Seed, Config{PassElision: g.PassElision, GorgeAutopaySurface: g.AutopaySurface}, chance)
	if err != nil {
		res.DivergedAt, res.Reason, res.Detail = 0, "chance", err.Error()
		return res
	}
	defer func() {
		if r := recover(); r != nil {
			res.DivergedAt, res.Reason, res.Detail = res.Applied, "gorge", fmt.Sprint(r)
		}
		res.Log = side.E.L
		res.Verified = verifyLog(m, g.Seed, chance, side.E.L)
	}()
	var p Projection
	cmp := true
	for i := 0; ; i++ {
		d := side.Step()
		if e := side.Err(); e != nil {
			res.DivergedAt, res.Reason, res.Detail = i, "gorge", e.Error()
			return res
		}
		if (cmp || side.Dirty()) && i < len(g.Hashes) {
			side.Project(&p)
			if h := compareHash(&p); h != g.Hashes[i] {
				res.DivergedAt, res.Reason, res.Detail = i, "projection", fmt.Sprintf("gorge %016x, kernel %016x", h, g.Hashes[i])
				return res
			}
		}
		if i >= len(g.Actions) {
			over, w := side.Terminal()
			if d.Kind != DNone && g.Over || over != g.Over || w != g.Winner {
				res.DivergedAt, res.Reason = i, "terminal"
				res.Detail = fmt.Sprintf("gorge over=%v winner=%d kind=%v, kernel over=%v winner=%d", over, w, d.Kind, g.Over, g.Winner)
			}
			return res
		}
		a := g.Actions[i]
		offered := false
		for _, o := range d.Options {
			if o == a {
				offered = true
			}
		}
		if !offered {
			res.DivergedAt, res.Reason = i, "not_offered"
			res.Detail = fmt.Sprintf("%v not in gorge's %v options %v", a, d.Kind, actionStrings(d.Options))
			return res
		}
		side.Apply(a)
		if e := side.Err(); e != nil {
			res.DivergedAt, res.Reason, res.Detail = i, "gorge", e.Error()
			return res
		}
		res.Applied++
		cmp = false
	}
}

// verifyLog re-runs gorge's log of a re-execution through engine/replay:
// Replay itself for a seeded game; for forced outcomes (which
// engine/replay's Config-only randomness cannot express) the same intents
// on rules.NewHypothetical, compared by replay.FirstDivergence.
func verifyLog(m *Matchup, seed uint64, chance []rules.ChanceDraw, l *events.Log) error {
	cfg := m.Config(seed)
	if len(chance) == 0 {
		_, err := replay.Replay(l, cfg)
		return err
	}
	e, err := rules.NewHypothetical(cfg, chance)
	if err != nil {
		return err
	}
	if err := e.AdvanceHypothetical(); err != nil {
		return err
	}
	for i, in := range l.Intents {
		if err := e.SubmitHypothetical(decision.CloneIntent(in)); err != nil {
			return fmt.Errorf("replay: intent %d rejected: %w", i, err)
		}
	}
	if d := replay.FirstDivergence(l.Events, e.L.Events); d != nil {
		return d
	}
	return nil
}

// RecordChance builds a GorgeSide whose engine records its random outcomes
// (rules.NewHypothetical with no forced prefix: the same events as New), so
// RecordKernelGame captures a Chance transcript.
func RecordChance(cfg Config) SideFactory {
	return func(m *Matchup, seed uint64) (Side, error) {
		g := &GorgeSide{M: m, Cfg: cfg}
		g.id.setLayout(m)
		e, err := rules.NewHypothetical(m.Config(seed), nil)
		if err != nil {
			return nil, err
		}
		g.E, g.hyp = e, true
		if err := e.AdvanceHypothetical(); err != nil {
			return nil, err
		}
		g.id.sync(e.G)
		return KernelSide(stubKernel{g}), nil
	}
}

// ChanceShufflePlanner is the gorge-follows-the-kernel half of a shared chance
// stream: a rules.ShufflePlanner (gamecfg.Rules.ShufflePlanner) that deals
// every library shuffle of a gorge game from the random outcomes a kernel
// recorded (ChanceSource), so gorge reaches the order the kernel's own
// Fisher-Yates produced from the same cards in the same positions.
//
// A shuffle of n cards is the kernel's run of draws with bounds n, n-1, ..., 2
// (kernel.shuffleLib), so the transcript splits into maximal descending runs
// and every run ending at bound 2 is a shuffle; the first draw (the toss,
// which gamecfg forces itself) and the draws between shuffles (a random
// discard) are not. The planner hands the runs out in order, one per distinct
// (player, ordinal): gorge asks the same shuffle again for every probe the
// mapping layer runs on a clone, and a repeat gets the same run, so the real
// submit and its probes agree. A request whose size is not the next run's
// returns nil -- gorge then draws it from its own generator and counts it as
// unplanned randomness, which fails the real game loudly and leaves a probe's
// legality check alone -- and moves nothing.
//
// gorge consumes the forced values through its own PCG, so its generator
// advances exactly as the kernel's did and later ordinary draws stay aligned
// (engine/rules forcePermutation). Limit: a run is recognised by its bounds
// alone, so a stray draw with bound n+1 right before an n-card shuffle would
// merge into it; the size check then refuses it rather than guess.
func ChanceShufflePlanner(tr []ChanceDraw) rules.ShufflePlanner {
	type run struct{ at, n int } // draws tr[at:at+n], bounds n+1, n, ..., 2
	var runs []run
	for i := 1; i < len(tr); { // draw 0 is the toss
		j := i + 1
		for j < len(tr) && tr[j].Bound == tr[j-1].Bound-1 {
			j++
		}
		if tr[j-1].Bound == 2 {
			runs = append(runs, run{i, j - i})
		}
		i = j
	}
	type key struct{ player, ordinal int }
	dealt := map[key]run{}
	next := 0
	return func(ctx rules.ShuffleContext) ([]state.ObjID, error) {
		n := len(ctx.Library)
		ids := make([]state.ObjID, n)
		for i, c := range ctx.Library {
			ids[i] = c.ID
		}
		if n < 2 {
			return ids, nil // a shuffle of 0 or 1 cards draws nothing
		}
		k := key{int(ctx.Player), ctx.Ordinal}
		r, ok := dealt[k]
		if !ok {
			if next >= len(runs) || runs[next].n != n-1 || tr[runs[next].at].Bound != n {
				return nil, nil
			}
			r = runs[next]
			dealt[k] = r
			next++
		}
		if r.n != n-1 {
			return nil, nil
		}
		for i := n - 1; i > 0; i-- {
			j := tr[r.at+n-1-i].Value
			if j < 0 || j > i {
				return nil, fmt.Errorf("chance planner: recorded draw %d has value %d outside [0,%d]", r.at+n-1-i, j, i)
			}
			ids[i], ids[j] = ids[j], ids[i]
		}
		return ids, nil
	}
}
