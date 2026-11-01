package azmcts

import (
	"context"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// kindFixture is one real decision of a fuller kind: deck a in seat 0
// against deck b, the game seed, the deciding seat, and the shape.
type kindFixture struct {
	name     string
	a, b     string
	seed     uint64
	player   state.PlayerID
	want     decision.Kind
	optKind  string // the first option's Kind ("" any)
	min, max int
	n        int
	kind     string // the searched kind's name
}

// kindFixtures are bot-vs-bot games of the repo decks reaching each fuller
// kind's shapes (found by scanning the default bot's games).
var kindFixtures = []kindFixture{
	{"modes 2 of 3", "mono-red-prowess", "foundations-calling-all-angels", testSeed + 1, 0, decision.KModes, "", 2, 2, 3, "modes"},
	{"modes 1 of 3", "foundations-keen-engineering", "death-n-taxes", testSeed + 1, 0, decision.KModes, "", 1, 1, 3, "modes"},
	{"may trigger", "foundations-calling-all-angels", "foundations-keen-engineering", testSeed, 0, decision.KTriggerOptional, "", 1, 1, 2, "optional"},
	{"choose X of 2", "foundations-keen-engineering", "mono-red-prowess", testSeed + 6, 0, decision.KChoose, "x", 1, 1, 2, "choose"},
	{"choose X of 5", "foundations-keen-engineering", "foundations-tramplesaurus-rex", testSeed + 6, 1, decision.KChoose, "x", 1, 1, 5, "choose"},
	{"sacrifice 2 of 3", "foundations-keen-engineering", "foundations-tramplesaurus-rex", testSeed + 1, 0, decision.KChoose, "sacrifice", 2, 2, 3, "choose"},
	{"exile up to 2 of 3", "foundations-tramplesaurus-rex", "foundations-wretched-ranks", testSeed, 1, decision.KChoose, "exile", 0, 2, 3, "choose"},
	{"up to 2 targets of 3", "mono-red-prowess", "foundations-reign-of-dragons", testSeed + 1, 1, decision.KTarget, "", 1, 2, 3, "multitarget"},
	{"optional target of 2", "foundations-calling-all-angels", "mono-blue-tempo", testSeed + 1, 0, decision.KTarget, "", 0, 1, 2, "multitarget"},
	{"2 targets of 5", "foundations-wretched-ranks", "mono-red-prowess", testSeed + 32, 0, decision.KTarget, "", 2, 2, 5, "multitarget"},
}

func (f kindFixture) matches(d *decision.Decision) bool {
	if d.Player != f.player || d.Kind != f.want || d.Min != f.min || d.Max != f.max || len(d.Options) != f.n {
		return false
	}
	return f.optKind == "" || d.Options[0].Kind == f.optKind
}

// position plays the default bot for both seats from genesis until the
// fixture's decision, and returns the engine there with the bot's answer,
// unsubmitted.
func (f kindFixture) position(t testing.TB) (*rules.Engine, *decision.Decision, decision.Intent) {
	t.Helper()
	e, d, in, ok := botDecision(testConfig(t, f.a, f.b, f.seed), 4000, f.matches)
	if !ok {
		t.Fatalf("%s: no such decision in %s vs %s seed %d", f.name, f.a, f.b, f.seed)
	}
	return e, d, in
}

// botDecision plays the default bot for both seats from genesis until a
// decision pred accepts.
func botDecision(cfg rules.Config, maxSteps int, pred func(*decision.Decision) bool) (*rules.Engine, *decision.Decision, decision.Intent, bool) {
	e := rules.New(cfg)
	e.Advance()
	rngs := searchprobe.BotRandoms(cfg.Seed, 2)
	board := botpolicy.NewBoard(2)
	for steps := 0; steps < maxSteps && !e.G.Over; steps++ {
		d := e.Pending()
		if d == nil {
			return nil, nil, decision.Intent{}, false
		}
		in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
		if pred(d) {
			return e, d, in, true
		}
		if err := e.Submit(in); err != nil {
			return nil, nil, decision.Intent{}, false
		}
	}
	return nil, nil, decision.Intent{}, false
}

// onlyKind is the Kinds searching kind alone.
func onlyKind(kind string) Kinds {
	k, err := ParseKinds(kind)
	if err != nil {
		panic(err)
	}
	return k
}

// legalSets is every set of d's option positions with Min..Max members
// (0..Max under AllowNone) d.Validate accepts, as sorted Index lists, in
// size order then lexicographic.
func legalSets(d *decision.Decision) [][]int {
	n := len(d.Options)
	var out [][]int
	lo := d.Min
	if d.AllowNone {
		lo = 0
	}
	for k := lo; k <= min(d.Max, n); k++ {
		var rec func(start int, cur []int)
		rec = func(start int, cur []int) {
			if len(cur) == k {
				choices := make([]int, k)
				for i, p := range cur {
					choices[i] = d.Options[p].Index
				}
				if d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}) == nil {
					out = append(out, choices)
				}
				return
			}
			for p := start; p < n; p++ {
				rec(p+1, append(cur, p))
			}
		}
		rec(0, nil)
	}
	return out
}

func sortedCopy(xs []int) []int {
	out := append([]int{}, xs...)
	sort.Ints(out)
	return out
}

// Each fuller kind enumerates its real decisions as the legal option sets:
// the bot's answer first (its own intent, unchanged), then every other
// legal set in size and lexicographic order, each once, capped at the
// limit; the keys are distinct, and each maps back onto the decision
// through a fresh collector (IntentForKey) as the same set. The kind is
// not searched without its flag (AllKinds' behaviour is unchanged).
func TestFullerKindsEnumerateRealDecisions(t *testing.T) {
	for _, f := range kindFixtures {
		e, d, bot := f.position(t)
		if _, kind, ok := enumerate(searchprobe.NewCollector(d.Player), e, d, bot, AllKinds(), 6); kind != "" || ok {
			t.Fatalf("%s: searched as %q under AllKinds", f.name, kind)
		}
		for _, limit := range []int{3, 6, 64} {
			obs := searchprobe.NewCollector(d.Player)
			cands, kind, ok := enumerate(obs, e, d, bot, onlyKind(f.kind), limit)
			legal := legalSets(d)
			if !ok || kind != f.kind {
				t.Fatalf("%s: kind %q ok %v (%d legal sets, bot %v)", f.name, kind, ok, len(legal), bot.Choices)
			}
			if !reflect.DeepEqual(cands[0].in, bot) {
				t.Fatalf("%s: candidate 0 %+v, want the bot's %+v", f.name, cands[0].in, bot)
			}
			want := [][]int{sortedCopy(bot.Choices)}
			for _, s := range legal {
				if !reflect.DeepEqual(s, want[0]) {
					want = append(want, s)
				}
			}
			want = want[:min(limit, len(want))]
			if len(cands) != len(want) {
				t.Fatalf("%s limit %d: %d candidates, want %d (legal %v)", f.name, limit, len(cands), len(want), legal)
			}
			keys := map[Key]bool{}
			for i, c := range cands {
				if err := d.Validate(c.in); err != nil {
					t.Fatalf("%s: candidate %d invalid: %v", f.name, i, err)
				}
				if got := sortedCopy(c.in.Choices); !reflect.DeepEqual(got, want[i]) {
					t.Fatalf("%s: candidate %d is %v, want %v", f.name, i, got, want[i])
				}
				if keys[c.key] {
					t.Fatalf("%s: duplicate key at %d", f.name, i)
				}
				keys[c.key] = true
				in, err := IntentForKey(searchprobe.NewCollector(d.Player), e, d, c.key)
				if err != nil {
					t.Fatalf("%s: candidate %d's key does not map back: %v", f.name, i, err)
				}
				if !reflect.DeepEqual(sortedCopy(in.Choices), want[i]) {
					t.Fatalf("%s: candidate %d's key maps to %v, want %v", f.name, i, in.Choices, want[i])
				}
			}
			if limit == 6 {
				t.Logf("%s (%s, min %d max %d, %d options): bot %v, %d legal sets, candidates %v",
					f.name, d.Kind, d.Min, d.Max, len(d.Options), bot.Choices, len(legal), choicesOf(cands))
			}
		}
	}
}

// Choose skips a long list (more options than Options.Limit) and a mana
// payment window, which stay the bot's.
func TestChooseSkipsLongListsAndPaymentWindows(t *testing.T) {
	for _, c := range []struct {
		name, a, b string
		seed       uint64
		pred       func(*decision.Decision) bool
	}{
		{"name of 11", "death-n-taxes", "uw-tempo", testSeed, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && len(d.Options) > 6
		}},
		{"payment window", "foundations-tramplesaurus-rex", "dimir-tempo", testSeed, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && len(d.Options) >= 2 && len(d.Options) <= 6 && d.Options[0].Kind == "activate"
		}},
	} {
		e, d, bot, ok := botDecision(testConfig(t, c.a, c.b, c.seed), 4000, c.pred)
		if !ok {
			t.Fatalf("%s: no such decision", c.name)
		}
		if _, kind, ok := enumerate(searchprobe.NewCollector(d.Player), e, d, bot, FullKinds(), 6); kind != "" || ok {
			t.Fatalf("%s: searched as %q", c.name, kind)
		}
		// Under PriorTopK the enumeration cap is past every list, but the
		// gate is still Options.Limit (Search sets chooseMax).
		k := FullKinds()
		k.chooseMax = 6
		if _, kind, ok := enumerate(searchprobe.NewCollector(d.Player), e, d, bot, k, BenchCandidateLimit); kind != "" || ok {
			t.Fatalf("%s: searched as %q under a large cap", c.name, kind)
		}
	}
}

// A fuller kind's prior is the softmax of each candidate's summed option
// scores (the trainer's candidate logit), BotPick marking the bot's options.
func TestFullerKindPriorSumsOptionScores(t *testing.T) {
	net := hashNet()
	checked := 0
	for _, f := range kindFixtures {
		e, d, bot := f.position(t)
		cands, kind, ok := enumerate(searchprobe.NewCollector(d.Player), e, d, bot, onlyKind(f.kind), 64)
		if !ok {
			t.Fatalf("%s: not searched", f.name)
		}
		got, fell := priors(net, e, d, bot, kind, cands, nil)
		if fell {
			t.Fatalf("%s: the prior fell back", f.name)
		}
		v := view.Project(e.G, e, d.Player, d)
		st := policynet.EncodeStateWith(net.Features, v, d.Player, nil)
		enc := make([]policynet.Option, len(d.Options))
		for i := range d.Options {
			enc[i] = policynet.EncodeOptionWith(net.Features, v, d.Player, d.Kind, d.Options[i], i, len(d.Options))
		}
		heuristic.MarkBotPicks(d, enc, bot)
		scores := net.Score(st, enc)
		logits := make([]float64, len(cands))
		for i, c := range cands {
			for _, k := range c.in.Choices {
				logits[i] += float64(scores[k])
			}
		}
		want, _ := softmax(logits)
		for i := range want {
			if math.Abs(got[i]-want[i]) > 1e-12 {
				t.Fatalf("%s: prior %v, want %v", f.name, got, want)
			}
		}
		checked++
	}
	t.Logf("%d fuller-kind priors checked", checked)
}

// checkFullerSearch asserts a clean search of a fuller kind at its root.
func checkFullerSearch(t *testing.T, name string, d *decision.Decision, bot decision.Intent, res Result, kind string, sims int) {
	t.Helper()
	st := res.Stats
	if res.Kind != kind || st.Searched != 1 || st.Simulations != sims || st.Completed != sims || st.AllFailed != 0 {
		t.Fatalf("%s: kind %q, stats %+v, want %s with %d completed simulations", name, res.Kind, st, kind, sims)
	}
	if st.Panics+st.SubmitErrors+st.BadWorlds+st.NoWorld+st.ChanceFailures != 0 {
		t.Fatalf("%s: failures: panics %d, submit %d, bad world %d, no world %d, chance %d",
			name, st.Panics, st.SubmitErrors, st.BadWorlds, st.NoWorld, st.ChanceFailures)
	}
	var want [NumExtraKinds]int
	want[extraIndex(kind)] = 1
	if st.ExtraSearched != want || st.KindSearched != ([NumKinds]int{}) || st.Skipped != 0 {
		t.Fatalf("%s: searched by kind %v / %v, skipped %d", name, st.ExtraSearched, st.KindSearched, st.Skipped)
	}
	sum := 0
	for _, v := range res.Visits {
		sum += v
	}
	if sum != st.Completed || st.Terminal+st.StepCapped+st.Expanded != st.Completed {
		t.Fatalf("%s: visits %v sum %d, terminal %d capped %d expanded %d, completed %d", name, res.Visits, sum, st.Terminal, st.StepCapped, st.Expanded, st.Completed)
	}
	if err := d.Validate(res.Intent); err != nil {
		t.Fatalf("%s: chosen intent invalid: %v", name, err)
	}
	if res.Choice == 0 && !reflect.DeepEqual(res.Intent, bot) {
		t.Fatalf("%s: choice 0 played %+v, want the bot's %+v", name, res.Intent, bot)
	}
}

// Search over a fuller kind's real root, every fuller kind on, with no
// network and with one, alone and with the opponent in the tree mirroring
// the seat (OpponentMirror: paying automatically): every simulation
// completes, the kind is counted, the answer is legal, and the result is
// deterministic. The engine is never written.
func TestSearchFullerKindsEndToEnd(t *testing.T) {
	net := hashNet()
	const sims = 48
	for _, f := range kindFixtures {
		e, d, bot := f.position(t)
		head := e.L.Head()
		for _, m := range []*policynet.Model{nil, net} {
			for _, opp := range []bool{false, true} {
				opts := DefaultOptions()
				opts.Sims, opts.Seed, opts.Kinds = sims, 41, FullKinds()
				opts.OpponentNodes, opts.OpponentMirror, opts.AutoPayment = opp, opp, opp
				res := searchAt(t, e, d, bot, m, opts)
				checkFullerSearch(t, f.name, d, bot, res, f.kind, sims)
				if again := searchAt(t, e, d, bot, m, opts); sameResult(res, again) != "" || again.Stats != res.Stats {
					t.Fatalf("%s: not deterministic: %s", f.name, sameResult(res, again))
				}
				if m != nil && !opp {
					t.Logf("%s: labels %v visits %v prior %.3f choice %d", f.name, res.Labels, res.Visits, res.Prior, res.Choice)
				}
			}
		}
		if e.L.Head() != head {
			t.Fatalf("%s: the search wrote the real engine", f.name)
		}
	}
}

// The fuller kinds are points inside the walk too, the searching seat's and
// the opponent's, and the opponent's priority points under OpponentMirror
// with AutoPayment are the payment vocabulary (its casts candidates,
// "opp:pay:" keys); without AutoPayment, or without OpponentMirror, no
// opponent payment candidate exists. The node cache stays exact with all of
// it on (the stored states carry the opponents' auto-pay bot streams, and a
// stored point recounts ExtraPoints and OppPayPoints).
func TestFullerWalkAndOpponentPayment(t *testing.T) {
	roots := autoPayRoots(t, 3)
	var total Stats
	for i, r := range roots {
		for _, kind := range []string{"clairvoyant", "pimc"} {
			runMirror := func(cache int, autoPay, mirror bool) Result {
				opts := DefaultOptions()
				opts.Sims, opts.Seed, opts.NodeCache = 160, uint64(i)*7+5, cache
				opts.Kinds, opts.OpponentNodes, opts.OpponentMirror, opts.AutoPayment = FullKinds(), true, mirror, autoPay
				bot := r.bot
				if !autoPay {
					bot = manualBot(t, r)
				}
				obs := searchprobe.NewCollector(r.d.Player)
				res, err := Search(context.Background(), Root{Engine: r.e, Decision: r.d, Bot: bot, Observer: obs}, measureSource(kind, r.e, obs), nil, opts)
				if err != nil {
					t.Fatal(err)
				}
				return res
			}
			run := func(cache int, autoPay bool) Result { return runMirror(cache, autoPay, true) }
			off := run(0, true)
			checkOppSearch(t, off, 160)
			total.Add(off.Stats)
			for _, cache := range []int{1, 3, DefaultNodeCache} {
				if d := sameResult(off, run(cache, true)); d != "" {
					t.Fatalf("%s %s cache %d: the cached search differs: %s", r.name, kind, cache, d)
				}
			}
			if manual := run(0, false); manual.Stats.OppPayPoints != 0 {
				t.Fatalf("%s %s: %d opponent payment points without AutoPayment", r.name, kind, manual.Stats.OppPayPoints)
			}
			if plain := runMirror(0, true, false); plain.Stats.OppPayPoints != 0 || plain.Stats.OppPoints == 0 {
				t.Fatalf("%s %s: %d opponent payment points (of %d opponent points) without OpponentMirror", r.name, kind, plain.Stats.OppPayPoints, plain.Stats.OppPoints)
			}
		}
	}
	t.Logf("opponent points %d (expanded %d), opponent payment points %d, fuller in-walk points %v",
		total.OppPoints, total.OppExpanded, total.OppPayPoints, total.ExtraPoints)
	if total.OppPayPoints == 0 {
		t.Fatal("no opponent payment point")
	}
}

// manualBot is the default (manual-payment) bot's answer at r's root.
func manualBot(t *testing.T, r measureRoot) decision.Intent {
	t.Helper()
	in, err := heuristic.NewBot(7).Decide(context.Background(), view.Project(r.e.G, r.e, r.d.Player, r.d), *r.d)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// oppWalk walks one clairvoyant world from root r with the opponent in the
// tree (mirroring the seat under mirror, OpponentMirror), always playing
// each point's candidate 0, and returns the points.
func oppWalk(t *testing.T, r measureRoot, net *policynet.Model, k int, autoPay, mirror bool, steps int) ([]*Point, Stats) {
	t.Helper()
	obs := searchprobe.NewCollector(r.d.Player)
	full, kind, _, ok, _ := enumerateCut(obs, r.e, r.d, r.bot, FullKinds(), BenchCandidateLimit, autoPay)
	if !ok {
		t.Fatalf("%s: no candidates", r.name)
	}
	cands, prior, _, _ := rankedPrior(net, k, 0, r.e, r.d, r.bot, kind, full, nil)
	var st Stats
	limit := DefaultOptions().Limit
	cfg := &walkConfig{
		net: net, kinds: FullKinds(), limit: limit, priorTopK: k, maxSteps: 2000,
		envSeed: 0x70707, actor: r.d.Player, autoPayment: autoPay, rootRefs: obs.Introduced(),
		root: &Point{Keys: keysOf(cands), Prior: prior}, rootCands: cands, rootDec: r.d, stats: &st,
		oppNodes: true, oppForks: make([]*searchprobe.Forker, len(r.e.G.Players)),
	}
	if mirror {
		cfg.oppAutoPay, cfg.oppTopK = autoPay, k
	}
	for p := range cfg.oppForks {
		if pl := state.PlayerID(p); pl != r.d.Player {
			cfg.oppForks[p] = searchprobe.NewCollector(pl).Forker()
		}
	}
	src, err := newTestClairvoyant(r.e, obs)
	if err != nil {
		t.Fatal(err)
	}
	w, err := src.World(0)
	if err != nil {
		t.Fatal(err)
	}
	env, err := newEngineEnv(w, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var pts []*Point
	pt := env.Root()
	for i := 0; i < steps && pt != nil; i++ {
		if pt, err = env.Play(pt.Keys[0]); err != nil {
			t.Fatalf("%s: step %d: %v", r.name, i, err)
		}
		if pt != nil {
			pts = append(pts, pt)
		}
	}
	return pts, st
}

// Under PriorTopK with the opponent in the tree mirroring the seat
// (OpponentMirror), the opponent's points are ranked and cut by the network
// prior as the searching seat's are (at most PriorTopK candidates, counted
// in PriorTopKPoints); its payment points carry "opp:pay:" keys under
// AutoPayment. Without OpponentMirror they are neither: capped at Limit in
// enumeration order, unranked, in the manual-payment vocabulary.
func TestOpponentPointsMirrorTheSeatsTopKAndPayment(t *testing.T) {
	const k = 2
	net := hashNet()
	oppRanked, oppPay := 0, 0
	for _, r := range autoPayRoots(t, 3) {
		checkUnmirrored(t, r, net, k)
		pts, st := oppWalk(t, r, net, k, true, true, 60)
		ranked := 0
		for i, p := range pts {
			if !p.ranked || len(p.Keys) > k {
				t.Fatalf("%s point %d (opp %v): ranked %v with %d keys, want <= %d ranked", r.name, i, p.Opp, p.ranked, len(p.Keys), k)
			}
			ranked++
			if !p.Opp {
				continue
			}
			oppRanked++
			for _, key := range p.Keys {
				if !strings.HasPrefix(string(key), oppKeyPrefix) {
					t.Fatalf("%s point %d: opponent key %q", r.name, i, key)
				}
				if strings.HasPrefix(string(key), oppKeyPrefix+payKeyPrefix) {
					oppPay++
				}
			}
		}
		if st.PriorTopKPoints != ranked {
			t.Fatalf("%s: %d PriorTopK points counted, %d ranked", r.name, st.PriorTopKPoints, ranked)
		}
	}
	t.Logf("opponent points ranked %d, opponent payment candidates %d", oppRanked, oppPay)
	if oppRanked == 0 || oppPay == 0 {
		t.Fatalf("opponent points ranked %d, payment candidates %d; want some of each", oppRanked, oppPay)
	}
}

// checkUnmirrored walks r with PriorTopK k and AutoPayment but without
// OpponentMirror: the searching seat's points are ranked, the opponent's
// are not (at most Limit candidates, no payment key, not counted in
// PriorTopKPoints).
func checkUnmirrored(t *testing.T, r measureRoot, net *policynet.Model, k int) {
	t.Helper()
	pts, st := oppWalk(t, r, net, k, true, false, 60)
	own, opp := 0, 0
	for i, p := range pts {
		if !p.Opp {
			if !p.ranked || len(p.Keys) > k {
				t.Fatalf("%s point %d: ranked %v with %d keys, want <= %d ranked", r.name, i, p.ranked, len(p.Keys), k)
			}
			own++
			continue
		}
		opp++
		if p.ranked || len(p.Keys) > DefaultOptions().Limit {
			t.Fatalf("%s opponent point %d: ranked %v with %d keys without OpponentMirror", r.name, i, p.ranked, len(p.Keys))
		}
		for _, key := range p.Keys {
			if strings.HasPrefix(string(key), oppKeyPrefix+payKeyPrefix) {
				t.Fatalf("%s opponent point %d: payment key %q without OpponentMirror", r.name, i, key)
			}
		}
	}
	if st.PriorTopKPoints != own || st.OppPayPoints != 0 {
		t.Fatalf("%s: %d PriorTopK points counted (%d own points), %d opponent payment points", r.name, st.PriorTopKPoints, own, st.OppPayPoints)
	}
	if opp == 0 {
		t.Fatalf("%s: no opponent point without OpponentMirror", r.name)
	}
}

// visit records of the fuller kinds carry the decision's kind, every
// candidate's options (a set candidate several, flagged Subset), and an
// empty option list only for the empty set (a Min-0 choice the bot or the
// enumeration offers).
func TestFullerKindsVisitRecords(t *testing.T) {
	for _, f := range kindFixtures {
		e, d, bot := f.position(t)
		opts := DefaultOptions()
		opts.Sims, opts.Seed, opts.Kinds = 24, 9, FullKinds()
		res := searchAt(t, e, d, bot, nil, opts)
		rec := visitRecord(e, d, res, policynet.FeaturesMZ, WorldClairvoyant, 24)
		if rec.Kind != d.Kind || len(rec.Cands) != len(res.Candidates) {
			t.Fatalf("%s: record kind %q with %d candidates, want %q with %d", f.name, rec.Kind, len(rec.Cands), d.Kind, len(res.Candidates))
		}
		several := false
		for c, in := range res.Candidates {
			if len(rec.Cands[c]) != len(in.Choices) {
				t.Fatalf("%s: candidate %d records %d options, chose %d", f.name, c, len(rec.Cands[c]), len(in.Choices))
			}
			several = several || len(in.Choices) > 1
			for _, o := range rec.Cands[c] {
				if o < 0 || o >= len(rec.Options) {
					t.Fatalf("%s: candidate %d names option %d of %d", f.name, c, o, len(rec.Options))
				}
			}
		}
		if rec.Subset != several {
			t.Fatalf("%s: subset %v, want %v", f.name, rec.Subset, several)
		}
	}
}

// ParseKinds names the fuller kinds; all is FullKinds, whose first four are
// AllKinds; a Kinds with no kind is refused.
func TestParseFullerKinds(t *testing.T) {
	k, err := ParseKinds("all")
	if err != nil || k != FullKinds() {
		t.Fatalf("all: %+v %v", k, err)
	}
	k, err = ParseKinds("priority,modes,optional,choose,multitarget")
	if err != nil || k != (Kinds{Priority: true, Modes: true, Optional: true, Choose: true, MultiTarget: true}) {
		t.Fatalf("list: %+v %v", k, err)
	}
	f := FullKinds()
	f.Modes, f.Optional, f.Choose, f.MultiTarget = false, false, false, false
	if f != AllKinds() {
		t.Fatal("FullKinds is not AllKinds plus the fuller kinds")
	}
	if _, err := ParseKinds("modes,bogus"); err == nil {
		t.Fatal("an unknown kind parsed")
	}
	o := DefaultOptions()
	o.Kinds = Kinds{chooseMax: 6}
	if o.Validate(nil) == nil {
		t.Fatal("a Kinds with no kind validated")
	}
}

// OpponentMirror needs OpponentNodes: the opponent's points it mirrors.
func TestOpponentMirrorNeedsOpponentNodes(t *testing.T) {
	o := DefaultOptions()
	o.OpponentMirror = true
	if o.Validate(nil) == nil {
		t.Fatal("OpponentMirror without OpponentNodes validated")
	}
	o.OpponentNodes = true
	if err := o.Validate(nil); err != nil {
		t.Fatalf("OpponentMirror with OpponentNodes: %v", err)
	}
}

// A set's key is its options in decision order whatever order the bot
// listed them in, named or not: a multi-target answer reversed ([1 0])
// has the same keys as [0 1] -- before nameKeys and after it, which rebuilds
// a key's actions (at every opponent point, and under Options.NameKeys) --
// while the candidate still plays the bot's own intent.
func TestNameKeysOfASetIgnoreTheBotsOrder(t *testing.T) {
	var f kindFixture
	for _, x := range kindFixtures {
		if x.name == "2 targets of 5" {
			f = x
		}
	}
	e, d, _ := f.position(t)
	var pair []int
	for _, s := range legalSets(d) {
		if len(s) == 2 {
			pair = s
			break
		}
	}
	if pair == nil {
		t.Fatalf("%s: no legal pair", f.name)
	}
	fwd := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: pair}
	rev := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pair[1], pair[0]}}
	if err := d.Validate(rev); err != nil {
		t.Fatalf("%s: the reversed pair %v is invalid: %v", f.name, rev.Choices, err)
	}
	keys := func(bot decision.Intent) (raw, named []Key) {
		obs := searchprobe.NewCollector(d.Player)
		cands, kind, ok := enumerate(obs, e, d, bot, onlyKind("multitarget"), 64)
		if !ok || kind != "multitarget" {
			t.Fatalf("%s: bot %v: kind %q ok %v", f.name, bot.Choices, kind, ok)
		}
		if !reflect.DeepEqual(cands[0].in, bot) {
			t.Fatalf("%s: candidate 0 plays %v, want the bot's %v", f.name, cands[0].in.Choices, bot.Choices)
		}
		got := nameKeys(obs, e, d, cands, 0, nil)
		if !reflect.DeepEqual(got[0].in, bot) {
			t.Fatalf("%s: named candidate 0 plays %v, want the bot's %v", f.name, got[0].in.Choices, bot.Choices)
		}
		return candKeys(cands), candKeys(got)
	}
	rawF, namedF := keys(fwd)
	rawR, namedR := keys(rev)
	if !reflect.DeepEqual(rawF, rawR) {
		t.Fatalf("%s: keys differ between %v and %v", f.name, fwd.Choices, rev.Choices)
	}
	if !reflect.DeepEqual(namedF, namedR) {
		t.Fatalf("%s: named keys differ between %v and %v:\n%q\n%q", f.name, fwd.Choices, rev.Choices, namedF[0], namedR[0])
	}
	if namedF[0] == rawF[0] {
		t.Fatalf("%s: nameKeys named nothing in %v's key", f.name, fwd.Choices)
	}
	t.Logf("%s: pair %v, %d candidates", f.name, pair, len(namedF))
}

// An az seat searching every fuller kind, the opponent in its tree mirroring
// it (OpponentMirror: paying automatically), plays whole games on honest
// redealt worlds with no failed simulation, replays exactly, searches fuller
// kinds at its own decisions and records them.
func TestSeatWithFullerKindsPlaysAndReplays(t *testing.T) {
	// testSeed+4: since the az seat consumes payment plans (2026-10-14) the
	// testSeed game no longer meets a fuller kind at its own root; +4 meets
	// three of them.
	cfg := testConfig(t, "foundations-calling-all-angels", "foundations-keen-engineering", testSeed+4)
	sc := DefaultSeatConfig()
	sc.Search.Sims, sc.World = 8, WorldRedeal
	sc.Search.Kinds, sc.Search.OpponentNodes, sc.Search.OpponentMirror, sc.Search.AutoPayment = FullKinds(), true, true, true
	var diags []Diag
	prev := Watch
	Watch = func(d Diag) { diags = append(diags, d) }
	t.Cleanup(func() { Watch = prev })
	heads := make([]string, 2)
	var total Stats
	kinds := map[decision.Kind]int{}
	for i := range heads {
		diags = diags[:0]
		az, err := NewSeat(cfg.Seed^1, nil, sc)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			az.SetRecorder(func(r policynet.VisitRecord) {
				kinds[r.Kind]++
				for c, opts := range r.Cands {
					if len(opts) == 0 && r.Kind != decision.KAttackers && r.Kind != decision.KBlockers && r.Kind != decision.KChoose && r.Kind != decision.KTarget {
						t.Errorf("%s record: candidate %d has no option", r.Kind, c)
					}
				}
			})
		}
		if heads[i], err = playAZ(t, cfg, az, 600); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			for _, d := range diags {
				total.Add(d.Stats)
			}
		}
	}
	if heads[0] != heads[1] {
		t.Fatalf("replay diverged: %s vs %s", heads[0], heads[1])
	}
	t.Logf("%d searched (fuller %v), records by kind %v; completed %d/%d; opponent points %d, opponent payment points %d, fuller in-walk points %v",
		total.Searched, total.ExtraSearched, kinds, total.Completed, total.Simulations, total.OppPoints, total.OppPayPoints, total.ExtraPoints)
	extra := 0
	for _, n := range total.ExtraSearched {
		extra += n
	}
	if total.Searched == 0 || extra == 0 || total.OppPayPoints == 0 {
		t.Fatal("the seat never searched a fuller kind or met an opponent payment point")
	}
	if total.Panics+total.SubmitErrors+total.BadWorlds+total.ChanceFailures != 0 {
		t.Fatalf("failed simulations: panics %d, submit %d, bad world %d, chance %d", total.Panics, total.SubmitErrors, total.BadWorlds, total.ChanceFailures)
	}
}
