package dirac

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The filtScript failed-verdict memo (mana_filtmemo.go) and filtPay's pooled
// visited table, measured on the synthetic Standard pools whose play searches
// are the most expensive (std-hob: 40% of the pool's instructions).

type memoGame struct {
	tbl   *Table
	decks [MaxSeats][]CardID
}

// memoDecks: two-colour 40-card decks, 23 spells (<=2 copies) + 17 basics.
func memoDecks(set cardset.Set, n int, seed uint64) [][]cardset.Entry {
	r := rand.New(rand.NewPCG(seed, 99))
	sup := set.Supported()
	basics := [5]string{"Plains", "Island", "Swamp", "Mountain", "Forest"}
	cols := [5]kop.ColorMask{kop.ColW, kop.ColU, kop.ColB, kop.ColR, kop.ColG}
	var spells []int
	for i := 0; i < set.NumCards() && i < len(sup); i++ {
		if !sup[i] || set.Inert(i) {
			continue
		}
		if set.Def(i).Types&kop.TLand != 0 {
			continue
		}
		spells = append(spells, i)
	}
	var out [][]cardset.Entry
	for len(out) < n {
		a, b := r.IntN(5), r.IntN(5)
		for b == a {
			b = r.IntN(5)
		}
		m := cols[a] | cols[b]
		var cand []int
		for _, i := range spells {
			if set.Def(i).Colors&^m == 0 {
				cand = append(cand, i)
			}
		}
		if len(cand) < 12 {
			continue
		}
		cnt := map[string]int{}
		for k := 0; k < 23; k++ {
			cnt[set.Names()[cand[r.IntN(len(cand))]]]++
		}
		keys := make([]string, 0, len(cnt))
		for k := range cnt {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		var es []cardset.Entry
		for _, k := range keys {
			es = append(es, cardset.Entry{Name: k, Count: cnt[k]})
		}
		es = append(es, cardset.Entry{Name: basics[a], Count: 9}, cardset.Entry{Name: basics[b], Count: 8})
		out = append(out, es)
	}
	return out
}

// memoGames builds sixteen two-seat games on synthetic pool name (skipping with
// no corpus, see synthSet).
func memoGames(t *testing.T, name string) []memoGame {
	raw := synthSet(t, name)
	set := cardset.AllowUnverified(raw)
	decks := memoDecks(set, 64, 7)
	var out []memoGame
	for c := 0; len(out) < 16 && c < 4*len(decks)+64; c++ {
		pick := make([][]cardset.Entry, 2)
		for j := range pick {
			pick[j] = decks[(c*2+j*7+j)%len(decks)]
		}
		tbl, ids, err := set.MatchupN(pick)
		if err != nil {
			continue
		}
		g := memoGame{tbl: tbl}
		copy(g.decks[:], ids)
		out = append(out, g)
	}
	t.Cleanup(func() {
		for _, g := range out {
			ReleaseTable(g.tbl)
		}
	})
	if len(out) == 0 {
		t.Skipf("%s: no game builds", name)
	}
	return out
}

// memoTrace is one run of n random-policy transitions: per transition, the
// decision posed (kind, options) folded with the hash of the game it left, and
// the memo's hit count.
type memoTrace struct {
	steps []uint64
	hits  uint32
}

func memoStep(h uint64, d *Decision, s *State) uint64 {
	h = h*1099511628211 ^ uint64(d.Kind)<<32 ^ uint64(d.N)
	for i := 0; i < int(d.N); i++ {
		h = h*1099511628211 ^ uint64(d.Options[i])
	}
	return h*1099511628211 ^ s.gameHash()
}

func memoPlay(t *testing.T, games []memoGame, n int) memoTrace {
	var tr memoTrace
	var s State
	var d Decision
	g, i := 0, 0
	var pol RandomPolicy
	start := func() {
		gm := games[g%len(games)]
		s.InitN(gm.tbl, gm.decks, 2, uint64(g)+1, Config{Mulligans: 2})
		pol = NewRandomPolicy(uint64(g) + 1)
	}
	segment := func() {
		defer func() {
			if r := recover(); r != nil {
				if m, ok := r.(string); ok && strings.HasPrefix(m, "dirac: filtScript memo") {
					t.Fatalf("game %d transition %d: %s", g, i, m)
				}
				tr.steps = append(tr.steps, 0xdead) // a panic the base plays too
				g++
				start()
			}
		}()
		for ; i < n; i++ {
			s.StepInto(&d)
			if d.Kind == DkNone {
				g++
				start()
				s.StepInto(&d)
			}
			s.Apply(pol.Pick(&d))
			tr.steps = append(tr.steps, memoStep(0, &d, &s))
		}
	}
	start()
	for i < n {
		segment()
		if i < n {
			i++
		}
	}
	if s.pool != nil {
		tr.hits = s.pool.fmemo.hits
	}
	return tr
}

// TestFiltScriptMemoEquivalence plays the same games with the memo off, on, and
// on with every hit re-searched (a hit whose search finds a script panics): the
// decisions posed and the game after every transition must be the same hash in
// all three, so the memo changes no verdict and no chosen payment.
func TestFiltScriptMemoEquivalence(t *testing.T) {
	for _, pool := range []struct {
		name string
		n    int
		hits bool // the pool's board makes the memo answer
	}{{"std-hob", 30000, true}, {"std-tdm", 30000, true}, {"std-spm", 30000, false}} {
		t.Run(pool.name, func(t *testing.T) {
			games := memoGames(t, pool.name)
			defer func(a, b bool) { DebugFiltNoMemo, DebugFiltMemoCheck = a, b }(DebugFiltNoMemo, DebugFiltMemoCheck)
			DebugFiltNoMemo = true
			off := memoPlay(t, games, pool.n)
			DebugFiltNoMemo, DebugFiltMemoCheck = false, true
			check := memoPlay(t, games, pool.n)
			DebugFiltMemoCheck = false
			on := memoPlay(t, games, pool.n)
			if off.hits != 0 {
				t.Fatalf("precondition: the memo answered %d times while off", off.hits)
			}
			if pool.hits && (check.hits == 0 || on.hits != check.hits) {
				t.Fatalf("precondition: the memo answered %d times (checked) / %d (plain): the pool no longer exercises it", check.hits, on.hits)
			}
			for name, tr := range map[string]memoTrace{"checked": check, "plain": on} {
				if len(tr.steps) != len(off.steps) {
					t.Fatalf("%s: %d transitions, memo-off %d", name, len(tr.steps), len(off.steps))
				}
				for i := range tr.steps {
					if tr.steps[i] != off.steps[i] {
						t.Fatalf("%s: transition %d differs from the memo-off run", name, i)
					}
				}
			}
			t.Logf("%s: %d transitions identical, memo answered %d searches", pool.name, len(off.steps), on.hits)
		})
	}
}

// TestFiltPayAllocatesNothing replays a std-tdm transition that runs filtPay
// (random play rarely reaches it): once the pool holds its visited table, a
// payment probe allocates nothing.
func TestFiltPayAllocatesNothing(t *testing.T) {
	games := memoGames(t, "std-tdm")
	var s, snap, work State
	var d Decision
	type op struct {
		apply bool
		a     Action
	}
	var found *op
	for g := 0; g < 600 && found == nil; g++ {
		gm := games[g%len(games)]
		s.InitN(gm.tbl, gm.decks, 2, uint64(g)+1, Config{Mulligans: 2})
		pol := NewRandomPolicy(uint64(g) + 1)
		func() {
			defer func() { recover() }()
			for i := 0; i < 20000 && found == nil; i++ {
				c0 := uint32(0)
				if s.pool != nil {
					c0 = s.pool.payCalls
				}
				snap.CopyFrom(&s)
				s.StepInto(&d)
				if d.Kind == DkNone {
					return
				}
				if s.pool != nil && s.pool.payCalls != c0 {
					found = &op{}
					return
				}
				a := pol.Pick(&d)
				snap.CopyFrom(&s)
				c0 = 0
				if s.pool != nil {
					c0 = s.pool.payCalls
				}
				s.Apply(a)
				if s.pool != nil && s.pool.payCalls != c0 {
					found = &op{true, a}
				}
			}
		}()
	}
	if found == nil {
		t.Fatal("no std-tdm transition on seeds 1..600 reaches filtPay")
	}
	run := func() {
		work.CopyFrom(&snap)
		if found.apply {
			work.Apply(found.a)
		} else {
			work.StepInto(&d)
		}
	}
	run() // warm: the pool's table, the fork pool, the cold buffers
	c0 := work.pool.payCalls
	run()
	if work.pool.payCalls == c0 {
		t.Fatal("precondition: the replayed transition no longer reaches filtPay")
	}
	if n := testing.AllocsPerRun(20, run); n != 0 {
		t.Fatalf("a transition running filtPay allocated %v times, want 0", n)
	}
}
