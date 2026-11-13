package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestLazyOneSource: the one-source declaration test lazyPoint uses to
// decide that a combat point's candidate set does not depend on the bot's
// answer: exactly one source over every non-Done option, and a Done option.
func TestLazyOneSource(t *testing.T) {
	attack := func(src, tgt uint8) dirac.Action {
		return dirac.MakeAction(dirac.AkAttack, dirac.TCode(src), dirac.TCode(tgt), 0)
	}
	block := func(src, tgt uint8) dirac.Action {
		return dirac.MakeAction(dirac.AkBlock, dirac.TCode(src), dirac.TCode(tgt), 0)
	}
	done := dirac.MakeAction(dirac.AkDone, 0, 0, 0)
	pass := dirac.MakeAction(dirac.AkPass, 0, 0, 0)
	tests := []struct {
		name string
		opts []dirac.Action
		want int
	}{
		{"one attacker at several defenders", []dirac.Action{attack(1, uint8(dirac.PlayerTarget0)), attack(1, uint8(dirac.PlayerTarget1)), done}, 2},
		{"two attackers", []dirac.Action{attack(1, uint8(dirac.PlayerTarget0)), attack(2, uint8(dirac.PlayerTarget0)), done}, -1},
		{"one blocker at several attackers", []dirac.Action{block(1, 2), block(1, 3), done}, 2},
		{"no done option", []dirac.Action{attack(1, uint8(dirac.PlayerTarget0)), attack(1, uint8(dirac.PlayerTarget1))}, -1},
		{"non-combat option", []dirac.Action{attack(1, uint8(dirac.PlayerTarget0)), pass, done}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d dirac.Decision
			for _, a := range tt.opts {
				d.Options[d.N] = a
				d.N++
			}
			if got := lazyOneSource(&d); got != tt.want {
				t.Fatalf("lazyOneSource = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestLazyPointSearchIdentical: the lazyPoint shortcut is a pure speedup.
// The same roots searched with the bot asked at every point (noLazyBot) and
// with the shortcut must reach the same Result and the same Stats, while
// the shortcut asks the bot fewer times.
func TestLazyPointSearchIdentical(t *testing.T) {
	rs := roots(32)
	eagerOpts, lazyOpts := DefaultOptions(), DefaultOptions()
	eagerOpts.Sims, lazyOpts.Sims = 64, 64
	eagerOpts.OpponentNodes, lazyOpts.OpponentNodes = true, true
	eagerOpts.PriorTopK, lazyOpts.PriorTopK = 6, 6
	eagerOpts.LazyPriors, lazyOpts.LazyPriors = true, true
	eb, lb := &pickCounter{PureBot: Heuristic{}}, &pickCounter{PureBot: Heuristic{}}
	esr := NewSearcher(eagerOpts, &fakeNet{val: -1}, eb)
	lsr := NewSearcher(lazyOpts, &fakeNet{val: -1}, lb)
	esr.noLazyBot = true
	skips := 0
	for i := range rs {
		a := esr.Search(rs[i], uint64(i))
		b := lsr.Search(rs[i], uint64(i))
		if !sameSearch(a, b) {
			t.Fatalf("root %d: lazyPoint changed the result", i)
		}
		if a.Stats != b.Stats {
			t.Fatalf("root %d: stats differ:\neager %+v\nlazy  %+v", i, a.Stats, b.Stats)
		}
		if a.Searched != b.Searched || a.Kind != b.Kind || a.Sims != b.Sims {
			t.Fatalf("root %d: result metadata differs: %+v vs %+v", i, a, b)
		}
		skips += b.Stats.PriorSkips
	}
	if skips == 0 {
		t.Fatalf("no point used the lazy-prior skip: the shortcut never acted")
	}
	if lb.n >= eb.n {
		t.Fatalf("Pick calls lazy %d, eager %d: the shortcut saved none", lb.n, eb.n)
	}
	t.Logf("%d skips, %d Pick calls eager, %d lazy", skips, eb.n, lb.n)
}

// TestLazyPointStatefulBot: the shortcut is only for a PureBot. A bot
// without the marker (oracle.SBT is the production one) must be asked at
// every point exactly as before, while a pure bot on the same roots does
// see fewer calls.
func TestLazyPointStatefulBot(t *testing.T) {
	rs := roots(16)
	newSR := func(bot Bot, noLazy bool) *Searcher {
		o := DefaultOptions()
		o.Sims = 32
		sr := NewSearcher(o, &fakeNet{val: -1}, bot)
		sr.noLazyBot = noLazy
		return sr
	}
	run := func(sr *Searcher) {
		for i := range rs {
			sr.Search(rs[i], uint64(i))
		}
	}

	pureLazy, pureEager := &pickCounter{PureBot: Heuristic{}}, &pickCounter{PureBot: Heuristic{}}
	run(newSR(pureLazy, false))
	run(newSR(pureEager, true))
	if pureLazy.n >= pureEager.n {
		t.Fatalf("pure bot: lazy %d calls, eager %d: the shortcut never fired", pureLazy.n, pureEager.n)
	}

	stateLazy, stateEager := &statefulBot{inner: Heuristic{}}, &statefulBot{inner: Heuristic{}}
	run(newSR(stateLazy, false))
	run(newSR(stateEager, true))
	if stateLazy.n != stateEager.n {
		t.Fatalf("stateful bot: lazy %d calls, eager %d: the shortcut skipped a stateful bot", stateLazy.n, stateEager.n)
	}
}

// statefulBot models a bot whose Pick calls have side effects (oracle.SBT):
// it deliberately does not implement PurePick.
type statefulBot struct {
	inner Bot
	n     int
}

func (b *statefulBot) Pick(s *dirac.State, d *dirac.Decision) int {
	b.n++
	return b.inner.Pick(s, d)
}

// TestLazyPointMatchesBuildPoint: every point lazyPoint accepts builds the
// same candidate set as buildPoint does with the real bot's answer, and
// buildPoint takes its lazy-prior skip there (PriorSkips delta 1). The
// search runs with noLazyBot so the bot is asked at every point; the probe
// bot re-runs lazyPoint at each advance point (same node, same tree) and
// then buildPoint with the answer the bot actually gave. This validates
// lazyPoint against buildPoint; TestLazyPointSearchIdentical is the one
// that proves advance takes the shortcut (its lazy bot sees fewer Pick
// calls), so deleting the hook fails there.
func TestLazyPointMatchesBuildPoint(t *testing.T) {
	opts := DefaultOptions()
	opts.Sims = 50
	opts.LazyPriors = true
	sr := NewSearcher(opts, &fakeNet{val: -1}, nil)
	probe := &lazyProbeBot{t: t, sr: sr, inner: Heuristic{}}
	sr.Bot = probe
	sr.noLazyBot = true
	for seed := uint64(1); seed <= 4; seed++ {
		seatGame(sr, seed)
	}
	if probe.checked == 0 {
		t.Fatalf("no point took the lazyPoint path: the probe never checked anything")
	}
	t.Logf("%d lazy points matched buildPoint", probe.checked)
}

// pickCounter counts the bot calls it forwards. It embeds PureBot so the
// wrapper is exactly as pure as the bot it wraps.
type pickCounter struct {
	PureBot
	n int
}

func (b *pickCounter) Pick(s *dirac.State, d *dirac.Decision) int {
	b.n++
	return b.PureBot.Pick(s, d)
}

// lazyProbeBot checks lazyPoint's acceptance against buildPoint.
type lazyProbeBot struct {
	t       *testing.T
	sr      *Searcher
	inner   Bot
	checked int
}

func (b *lazyProbeBot) PurePick() {
	if _, ok := b.inner.(PureBot); !ok {
		panic("lazyProbeBot wraps a stateful bot")
	}
}

func (b *lazyProbeBot) Pick(s *dirac.State, d *dirac.Decision) int {
	sr := b.sr
	// Only advance's own points (not botDeclaration's continuation picks).
	if sr.hint < 0 || s != &sr.w.s || d != &sr.w.d {
		return b.inner.Pick(s, d)
	}
	st0 := sr.St
	var lp point
	lazy := sr.lazyPoint(s, d, &lp)
	sr.St = st0
	if !lazy {
		return b.inner.Pick(s, d)
	}
	bot := b.inner.Pick(s, d)
	var bp point
	var sc dirac.State
	sc.CopyFrom(s)
	dc := *d
	st0 = sr.St
	built := sr.buildPoint(&sc, &dc, bot, &bp)
	skipped := sr.St.PriorSkips - st0.PriorSkips
	sr.St = st0
	if !built {
		b.t.Fatalf("lazyPoint accepted a point buildPoint rejected")
	}
	if skipped != 1 {
		b.t.Fatalf("lazyPoint accepted a point buildPoint did not skip (PriorSkips delta %d)", skipped)
	}
	sameCandidates(b.t, &lp, &bp)
	b.checked++
	return bot
}

// sameCandidates reports whether a and b hold the same candidate set: the
// same count and opponent flag, and for every key of a the same action
// sequence under the same key in b (order is irrelevant under the skip).
func sameCandidates(t *testing.T, a, b *point) {
	t.Helper()
	if a.n != b.n || a.opp != b.opp {
		t.Fatalf("candidate count/opponent differ: %d/%v vs %d/%v", a.n, a.opp, b.n, b.opp)
	}
	for i := 0; i < a.n; i++ {
		j := -1
		for k := 0; k < b.n; k++ {
			if b.keys[k] == a.keys[i] {
				j = k
				break
			}
		}
		if j < 0 {
			t.Fatalf("candidate key %#x missing from buildPoint", uint64(a.keys[i]))
		}
		as := a.acts[a.off[i]:a.off[i+1]]
		bs := b.acts[b.off[j]:b.off[j+1]]
		if len(as) != len(bs) {
			t.Fatalf("candidate key %#x: %d acts vs %d", uint64(a.keys[i]), len(as), len(bs))
		}
		for x := range as {
			if as[x] != bs[x] {
				t.Fatalf("candidate key %#x: act %d %#x vs %#x", uint64(a.keys[i]), x, as[x], bs[x])
			}
		}
	}
}
