package dirac

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The FDN soak: random-policy games over real deck pairings from the
// FDN deck pool through the fdn set's Matchup. The in-test sample is small;
// KERNEL_FDN_SOAK=N plays N pairings (KERNEL_FDN_DECKS overrides the pool
// directory).

type fdnDeck struct {
	Name  string `json:"name"`
	Cards []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"cards"`
}

// poolDecksDir is a frozen pool's deck directory: $GORGE_POOLS/<set>/decks, else
// .pools/<set>/decks at the module root. It repeats dirac/oracle's PoolDecks,
// which this package may not import.
func poolDecksDir(set string) string {
	root := os.Getenv("GORGE_POOLS")
	if root == "" {
		dir, err := os.Getwd()
		if err != nil {
			return ""
		}
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				root = filepath.Join(dir, ".pools")
				break
			}
			up := filepath.Dir(dir)
			if up == dir {
				return ""
			}
			dir = up
		}
	}
	return filepath.Join(root, set, "decks")
}

func fdnDecks(t testing.TB) [][]cardset.Entry {
	t.Helper()
	dir := os.Getenv("KERNEL_FDN_DECKS")
	if dir == "" {
		dir = poolDecksDir("fdn")
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(paths) == 0 {
		t.Skipf("no FDN decks in %q (set GORGE_POOLS or KERNEL_FDN_DECKS)", dir)
	}
	sort.Strings(paths)
	var out [][]cardset.Entry
	set := cardsettest.Set(t, "fdn")
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var d fdnDeck
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		var es []cardset.Entry
		ok := true
		for _, c := range d.Cards {
			i, found := set.Lookup(c.Name)
			if !found || !set.Supported()[i] {
				ok = false
				break
			}
			es = append(es, cardset.Entry{Name: c.Name, Count: c.Count})
		}
		if ok {
			out = append(out, es)
		}
	}
	return out
}

// benchDecks reads the decks a set's benches play: committed, so a bench's
// instruction count does not depend on what a pool holds. Game g plays files 2g
// and 2g+1, in name order.
func benchDecks(t testing.TB, set string) [][]cardset.Entry {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join("testdata", "bench", set, "*.json"))
	sort.Strings(paths)
	out := make([][]cardset.Entry, len(paths))
	for i, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var d fdnDeck
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		for _, c := range d.Cards {
			out[i] = append(out[i], cardset.Entry{Name: c.Name, Count: c.Count})
		}
	}
	return out
}

// TestBenchDecksAreCommitted: each bench set has its 64 pairings on disk, every
// card supported, whichever pool this box has.
func TestBenchDecksAreCommitted(t *testing.T) {
	for _, name := range []string{"fdn", "blb"} {
		decks := benchDecks(t, name)
		if len(decks) != 128 {
			t.Fatalf("%s: %d bench decks, want 128 (two per game)", name, len(decks))
		}
		set := cardsettest.Set(t, name)
		for i, d := range decks {
			for _, e := range d {
				if j, ok := set.Lookup(e.Name); !ok || !set.Supported()[j] {
					t.Errorf("%s deck %d: %s is not supported, so the bench cannot play it", name, i, e.Name)
				}
			}
		}
	}
}

// fdnGame plays one pairing; it returns the decisions answered and whether
// the game ended, and reports a panic with the pairing named.
func fdnGame(t testing.TB, decks [][]cardset.Entry, g int, maxDec int) (n int, done bool, stalled uint8) {
	a := decks[(g*7919)%len(decks)]
	b := decks[(g*104729+31)%len(decks)]
	set := cardsettest.Set(t, "fdn")
	tbl, da, db, err := set.Matchup(a, b)
	if err != nil {
		t.Fatalf("pairing %d: %v", g, err)
	}
	defer ReleaseTable(tbl)
	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Sprintf("pairing %d (decks %d/%d, seed %d): %v", g, (g*7919)%len(decks), (g*104729+31)%len(decks), g, r))
		}
	}()
	s := NewConfig(tbl, da, db, uint64(g)*0x9e37+1, Config{Mulligans: uint8(g % 3), NoElide: g%2 == 1})
	pol := NewRandomPolicy(uint64(g)*7919 + 3)
	n, done = PlayRandom(s, &pol, maxDec)
	return n, done, s.StallReason()
}

func TestFDNSoak(t *testing.T) {
	decks := fdnDecks(t)
	n := 40
	if v := os.Getenv("KERNEL_FDN_SOAK"); v != "" {
		n, _ = strconv.Atoi(v)
	}
	from := 0
	if v := os.Getenv("KERNEL_FDN_SOAK_FROM"); v != "" {
		from, _ = strconv.Atoi(v)
	}
	unfinished, stalled := 0, 0
	var why [NumStall]int
	total := 0
	for g := from; g < from+n; g++ {
		d, done, st := fdnGame(t, decks, g, 30000)
		total += d
		if st != StallNone {
			stalled++
			tallyStall(&why, st)
			t.Logf("pairing %d: stall %d after %d decisions", g, st, d)
		}
		if !done {
			unfinished++
			t.Errorf("pairing %d: not over after %d decisions", g, d)
		}
	}
	t.Logf("%d pairings over %d supported decks: %d decisions, %d unfinished, %d stalled at capacity (%s)", n, len(decks), total, unfinished, stalled, stallBreakdown(&why))
}

// tallyStall records one finished game's stall reason. The histogram is
// sized from the enum so a newly appended Stall* reason cannot index past
// it (the old [6]int panicked for every reason above StallLoop).
func tallyStall(why *[NumStall]int, st uint8) {
	if st != StallNone {
		why[st]++
	}
}

// stallBreakdown renders the non-zero reasons in enum order (deterministic:
// no map range), "none" when the sample stalled nowhere.
func stallBreakdown(why *[NumStall]int) string {
	var b strings.Builder
	for r := 1; r < NumStall; r++ {
		if why[r] == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s=%d", StallName(uint8(r)), why[r])
	}
	if b.Len() == 0 {
		return "none"
	}
	return b.String()
}

// TestFDNSoakStallTally: every stall reason the enum defines must reach the
// soak's accounting without panicking. The old [6]int panicked at
// StallDelayed (6) and above, including StallRegen (28) today.
func TestFDNSoakStallTally(t *testing.T) {
	var why [NumStall]int
	tallyStall(&why, StallNone)
	last := uint8(0)
	for r := uint8(1); r < NumStall; r++ {
		if StallName(r) == "unknown" {
			continue
		}
		last = r
		tallyStall(&why, r)
	}
	for r := uint8(1); r < NumStall; r++ {
		if StallName(r) != "unknown" && why[r] != 1 {
			t.Fatalf("reason %d (%s): count %d, want 1", r, StallName(r), why[r])
		}
	}
	got := stallBreakdown(&why)
	if last := StallName(last); !strings.Contains(got, last+"=1") {
		t.Fatalf("breakdown %q does not report the highest reason %q", got, last)
	}
	if strings.Contains(got, "=0") {
		t.Fatalf("breakdown %q reports a zero count", got)
	}
	var empty [NumStall]int
	if got := stallBreakdown(&empty); got != "none" {
		t.Fatalf("empty breakdown %q, want none", got)
	}
}

var _ = kop.OpCount

// TestFDNPairTrace replays pairing KERNEL_FDN_PAIR and prints its last
// actions (a debugging aid; skipped otherwise).
func TestFDNPairTrace(t *testing.T) {
	v := os.Getenv("KERNEL_FDN_PAIR")
	if v == "" {
		t.Skip("set KERNEL_FDN_PAIR")
	}
	g, _ := strconv.Atoi(v)
	decks := fdnDecks(t)
	a := decks[(g*7919)%len(decks)]
	b := decks[(g*104729+31)%len(decks)]
	set := cardsettest.Set(t, "fdn")
	tbl, da, db, _ := set.Matchup(a, b)
	s := NewConfig(tbl, da, db, uint64(g)*0x9e37+1, Config{Mulligans: uint8(g % 3), NoElide: g%2 == 1})
	pol := NewRandomPolicy(uint64(g)*7919 + 3)
	var d Decision
	var hist []string
	name := func(o uint8) string {
		if int(o) < MaxObj && int(s.objs[o].card) < len(tbl.Cards) {
			return tbl.Cards[s.objs[o].card].Name
		}
		return fmt.Sprint(o)
	}
	defer func() {
		if r := recover(); r != nil {
			for _, h := range hist[max(0, len(hist)-25):] {
				t.Log(h)
			}
			k := s.k()
			for m := s.occ; m.any(); m = m.rest() {
				sl := m.first()
				pw, th := s.pt(k, sl)
				mp := s.manaPCGen(k, sl)
				t.Logf("slot %d %s ctrl %d types %x pt %d/%d kw %x col %x tapped %v mana %v(%d col %x)", sl, tbl.Cards[s.cardAt(sl)].Name, s.perms[sl].ctrl, s.typesGen(k, sl), pw, th, s.kwGen(k, sl), s.colorsGen(k, sl), s.perms[sl].flags&pfTapped != 0, s.canTapForMana(k, sl), mp.manaAb, mp.manaCol)
			}
			panic(r)
		}
	}()
	for i := 0; i < 30000; i++ {
		s.StepInto(&d)
		if d.Kind == DkNone {
			break
		}
		x := pol.Pick(&d)
		hist = append(hist, fmt.Sprintf("t%d step%d p%d dk%d sub%d: kind %d src %s tgt %d mode %d sp %d pend %d", s.turn, s.step, d.Player, d.Kind, d.Sub, x.Kind(), name(uint8(x.Source())), x.Target(), x.Mode(), s.sp, s.npend))
		s.Apply(x)
	}
	if d.Kind != DkNone {
		for _, h := range hist[max(0, len(hist)-40):] {
			t.Log(h)
		}
		t.Errorf("not over: turn %d", s.turn)
	}
}

// fdnBenchTables builds the first n soak pairings' matchup tables (and
// prepares them) outside the timer.
type fdnBenchGame struct {
	tbl    *kop.Table
	da, db []CardID
}

// fdnBenchTables builds through the set, so the benches run on the binary tables.
func fdnBenchTables(b testing.TB, n int) []fdnBenchGame {
	decks := benchDecks(b, "fdn")
	set := cardsettest.Set(b, "fdn")
	out := make([]fdnBenchGame, n)
	for g := range out {
		tbl, da, db, err := set.Matchup(decks[2*g], decks[2*g+1])
		if err != nil {
			b.Fatal(err)
		}
		out[g] = fdnBenchGame{tbl, da, db}
		var s State
		s.Init(tbl, da, db, 0, Config{})
	}
	b.Cleanup(func() {
		for _, g := range out {
			ReleaseTable(g.tbl)
		}
	})
	return out
}

func benchTransitionFDN(b *testing.B, noElide bool) {
	games := fdnBenchTables(b, 64)
	var s State
	var d Decision
	pol := NewRandomPolicy(1)
	g := 0
	cfg := Config{Mulligans: 2, NoElide: noElide}
	s.Init(games[0].tbl, games[0].da, games[0].db, 1, cfg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.StepInto(&d)
		if d.Kind == DkNone {
			g++
			gm := &games[g%len(games)]
			s.Init(gm.tbl, gm.da, gm.db, uint64(g)+1, cfg)
			s.StepInto(&d)
		}
		s.Apply(pol.Pick(&d))
	}
}

// BenchmarkTransitionFDN is BenchmarkTransition over 64 real FDN deck
// pairings, single-option decisions elided.
func BenchmarkTransitionFDN(b *testing.B) { benchTransitionFDN(b, false) }

// BenchmarkMatchupFDN is one kop.Table built from two real FDN decks, the cold
// path that turns a pool's tables into a matchup's own. Reported, not gated.
func BenchmarkMatchupFDN(b *testing.B) {
	decks := benchDecks(b, "fdn")
	set := cardsettest.Set(b, "fdn")
	b.ReportAllocs()
	b.ResetTimer()
	for g := 0; g < b.N; g++ {
		if _, _, _, err := set.Matchup(decks[2*(g%64)], decks[2*(g%64)+1]); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTransitionFDNNoElide poses every decision.
func BenchmarkTransitionFDNNoElide(b *testing.B) { benchTransitionFDN(b, true) }

// BenchmarkTransitionFDNParallel runs the same transition loop on every P,
// each goroutine with its own State: it exercises the paths a parallel
// search takes, where a process-wide counter write would contend on one
// cache line.
func BenchmarkTransitionFDNParallel(b *testing.B) {
	games := fdnBenchTables(b, 64)
	cfg := Config{Mulligans: 2}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var s State
		var d Decision
		pol := NewRandomPolicy(1)
		g := 0
		s.Init(games[0].tbl, games[0].da, games[0].db, 1, cfg)
		for pb.Next() {
			s.StepInto(&d)
			if d.Kind == DkNone {
				g++
				gm := &games[g%len(games)]
				s.Init(gm.tbl, gm.da, gm.db, uint64(g)+1, cfg)
				s.StepInto(&d)
			}
			s.Apply(pol.Pick(&d))
		}
	})
}
