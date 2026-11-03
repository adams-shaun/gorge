package oracle

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// WARNING: pins drift. A synthetic pair is two decks dealt from the lowered card list, so lowering one
// more card moves what every pair holds: a pin by index goes silently empty, or red
// on a game that no longer has the card the test is named for. A pin therefore names
// what its pairs must reach, through synthPairsReaching, and fails when one does not;
// a regression whose deck must not move at all is a recorded repro instead
// (stdReproLockstep).

// A pin is a condition, not a number. synthPairsReaching plays the pinned pairs
// first, which is the whole cost while the pins hold. For a pinned pair that
// never satisfies one of the conditions it searches the pool's other pairs, in
// index order and within synthRepinPairs plays and synthRepinCap of wall-clock in all,
// for pairs that reach every condition and play clean, plays them with the same
// check, and logs each replacement; it fails only when none is found. With
// SYNTH_REPIN=1 it also rewrites the pair list literal at the call site, so a
// pin is refreshed cheaply after a lowering moves the deal.
var (
	synthRepinPairs = 600              // pair plays one test may spend searching for its stale pins
	synthRepinCap   = 20 * time.Second // wall-clock one test may spend searching for its stale pins
)

// synthPairsReaching is synthPairsOn that also fails any pair whose game never
// satisfies one of the reached predicates at a decision, unless a replacement
// pair reaches them all. Each predicate is judged on its own, so the conjunction
// is "every one held at some point in the game".
func synthPairsReaching(t *testing.T, name string, cfg Config, pairs []int, reached ...func(ref *GorgeSide) bool) {
	t.Helper()
	synthPairsReachingAt(t, 2, name, RunConfig{Config: cfg}, 1, pairs, nil, reached...)
}

// synthPairsReachingRun is synthPairsReaching on the run rc, with the mulligans each
// player may take and check run on every finished pair.
func synthPairsReachingRun(t *testing.T, name string, rc RunConfig, mulligans int, pairs []int, check func(pair int, res *Result), reached ...func(ref *GorgeSide) bool) {
	t.Helper()
	synthPairsReachingAt(t, 2, name, rc, mulligans, pairs, check, reached...)
}

func synthPairsReachingAt(t *testing.T, skip int, name string, rc RunConfig, mulligans int, pairs []int, check func(pair int, res *Result), reached ...func(ref *GorgeSide) bool) {
	t.Helper()
	hit := make([]bool, len(reached))
	inner := rc.Check
	rc.Check = func(ref *GorgeSide, ker Side) *Divergence {
		for i, r := range reached {
			if !hit[i] && r(ref) {
				hit[i] = true
			}
		}
		if inner != nil {
			return inner(ref, ker)
		}
		return nil
	}
	play := newSynthPlayer(t, name, rc, mulligans)
	// playHits plays pair and reports which conditions its game reached.
	playHits := func(pair int) (Result, []bool) {
		clear(hit)
		res := play(pair)
		return res, slices.Clone(hit)
	}
	next := slices.Clone(pairs)
	var stale []int // indices into pairs
	var missed [][]int
	for k, pair := range pairs {
		res, got := playHits(pair)
		if res.Div != nil {
			t.Errorf("pair %d diverged at %d: %s %s %s %v %v", pair, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
		}
		if res.Stall != "" {
			t.Errorf("pair %d stalled: %s", pair, res.Stall)
		}
		var miss []int
		for i, h := range got {
			if !h {
				miss = append(miss, i)
			}
		}
		if len(miss) == 0 {
			if check != nil {
				check(pair, &res)
			}
			continue
		}
		stale = append(stale, k)
		missed = append(missed, miss)
	}
	if len(stale) == 0 {
		return
	}
	taken := map[int]bool{}
	for _, p := range pairs {
		taken[p] = true
	}
	replaced := false
	// One scan serves every stale pin: each good pair it finds takes the next stale slot.
	slot := 0
	var founds []Result
	var picked []int
	maxPairs, limit := synthRepinPairs, synthRepinCap
	if os.Getenv("SYNTH_REPIN") == "1" {
		maxPairs, limit = maxPairs*10, limit*10 // a refresh may take its time
	}
	repinSearch(taken, len(stale), maxPairs, limit, func(pair int) bool {
		res, got := playHits(pair)
		if res.Div != nil || res.Stall != "" || slices.Contains(got, false) {
			return false
		}
		picked = append(picked, pair)
		founds = append(founds, res)
		return true
	})
	for n, k := range stale {
		if slot >= len(picked) {
			for _, i := range missed[n] {
				t.Errorf("pair %d of %s never reaches condition %d of %d and no other pair within %d plays or %s does: the pin tests nothing", pairs[k], name, i+1, len(reached), synthRepinPairs, synthRepinCap)
			}
			continue
		}
		pair := picked[slot]
		next[k] = pair
		replaced = true
		for _, i := range missed[n] {
			t.Logf("condition %d of %s now reached by pair %d (re-pin)", i+1, name, pair)
		}
		if check != nil {
			check(pair, &founds[slot])
		}
		slot++
	}
	if replaced && os.Getenv("SYNTH_REPIN") == "1" {
		rewritePairLiteral(t, skip, pairs, next)
	}
}

// repinSearch plays pairs 0, 1, 2, ... in order, skipping those in taken, until
// try has reported want of them good, at most maxPairs plays and limit of
// wall-clock. It returns the pairs found.
func repinSearch(taken map[int]bool, want, maxPairs int, limit time.Duration, try func(pair int) bool) []int {
	deadline := time.Now().Add(limit)
	played := 0
	var found []int
	for pair := 0; played < maxPairs && pair < maxPairs*20; pair++ {
		if taken[pair] {
			continue
		}
		if time.Now().After(deadline) {
			break
		}
		played++
		if try(pair) {
			found = append(found, pair)
			if len(found) == want {
				break
			}
		}
	}
	return found
}

// rewritePairLiteral replaces the []int{...} literal holding old with next on the
// source line of the nearest caller, from skip frames above its own caller, that
// holds it; a helper between the test and the pin is looked through.
func rewritePairLiteral(t *testing.T, skip int, old, next []int) {
	t.Helper()
	want := "[]int{" + joinInts(old) + "}"
	for f := skip + 1; f < skip+8; f++ {
		_, file, line, ok := runtime.Caller(f)
		if !ok {
			break
		}
		file = filepath.Base(file) // the test runs in the package directory
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		lines := strings.Split(string(b), "\n")
		for i := line - 1; i < min(line+3, len(lines)); i++ {
			if i >= 0 && strings.Contains(lines[i], want) {
				lines[i] = strings.Replace(lines[i], want, "[]int{"+joinInts(next)+"}", 1)
				if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
					t.Logf("SYNTH_REPIN: %v", err)
					return
				}
				t.Logf("SYNTH_REPIN: %s:%d pairs now %v", file, i+1, next)
				return
			}
		}
	}
	t.Logf("SYNTH_REPIN: no call site holds %s; set the pairs to %v by hand", want, next)
}

func joinInts(v []int) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ", ")
}

// cardOut is the condition that a card of the given name is on the stack, the
// battlefield, a graveyard or in exile at a decision: it was cast or entered.
func cardOut(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		g := ref.E.G
		if inZoneNamed(g, state.ZStack, 0, name) {
			return true
		}
		for p := range g.Players {
			for _, z := range []state.Zone{state.ZBattlefield, state.ZGraveyard, state.ZExile} {
				if inZoneNamed(g, z, state.PlayerID(p), name) {
					return true
				}
			}
		}
		return false
	}
}

// onBattlefield is the condition that a card of the given name is on the battlefield.
func onBattlefield(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		for p := range ref.E.G.Players {
			if inZoneNamed(ref.E.G, state.ZBattlefield, state.PlayerID(p), name) {
				return true
			}
		}
		return false
	}
}

// onStack is the condition that a card or ability of the given card name is on the stack.
func onStack(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool { return inZoneNamed(ref.E.G, state.ZStack, 0, name) }
}

func inZoneNamed(g *state.Game, z state.Zone, p state.PlayerID, name string) bool {
	for _, id := range g.Zone(z, p) {
		if o := g.Obj(id); o != nil {
			if f := o.Face(); f != nil && f.Name == name {
				return true
			}
		}
	}
	return false
}

// cardSeen is the condition that a card of the given name is in a hand, or has been
// cast or entered: it was dealt to a player who could use it.
func cardSeen(name string) func(ref *GorgeSide) bool {
	out := cardOut(name)
	return func(ref *GorgeSide) bool {
		if out(ref) {
			return true
		}
		for p := range ref.E.G.Players {
			if inZoneNamed(ref.E.G, state.ZHand, state.PlayerID(p), name) {
				return true
			}
		}
		return false
	}
}

// anyOf is the condition that one of conds holds.
func anyOf(conds ...func(ref *GorgeSide) bool) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		for _, c := range conds {
			if c(ref) {
				return true
			}
		}
		return false
	}
}

// anyOut is the condition that a card of one of the given names was cast or entered.
func anyOut(names ...string) func(ref *GorgeSide) bool {
	conds := make([]func(ref *GorgeSide) bool, len(names))
	for i, n := range names {
		conds[i] = cardOut(n)
	}
	return anyOf(conds...)
}

// madeRestrictedMana is the condition that mana carrying a spending restriction has
// been added to a pool.
func madeRestrictedMana(ref *GorgeSide) bool {
	for i := range ref.E.L.Events {
		ev := &ref.E.L.Events[i]
		if ev.Kind == events.ManaAdd && ev.Amount > 0 && strings.HasPrefix(ev.Text, "mana-restriction:") {
			return true
		}
	}
	return false
}

// tapTriggerFromManaTap is the condition that a mana tap put a Taps trigger on gorge's
// stack: a trigger_push for a permanent whose tap, followed by the mana it added, sits
// just before it (an attack's tap adds no mana). A non-empty listener also requires the
// trigger to be that card's. It remembers how far into the log it has read, so it serves
// one game.
func tapTriggerFromManaTap(listener string) func(ref *GorgeSide) bool {
	seen := 0
	return func(ref *GorgeSide) bool {
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			if evs[seen].Kind != events.TriggerPush {
				continue
			}
			if listener != "" {
				o := ref.E.G.Obj(evs[seen].Obj)
				if o == nil || o.Face() == nil || o.Face().Name != listener {
					continue
				}
			}
			mana := false
			for j := seen - 1; j >= max(0, seen-8); j-- {
				switch evs[j].Kind {
				case events.ManaAdd:
					mana = true
				case events.Tap:
					if mana && evs[j].Obj == evs[seen].Obj {
						return true
					}
				}
			}
		}
		return false
	}
}
