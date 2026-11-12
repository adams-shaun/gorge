package oracle

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	moduleRootFromOracle = "../.."

	refusedPairsMax = 200              // pairs with the card dealt in, once its repro plays clean
	refusedEntryCap = 90 * time.Second // per entry: 200 pairs fit on a box at load 60
	refusedGameCap  = 10 * time.Second // a game past this is a hang, which proves nothing
	// refusedRecentSources is kfuzz's bad-game line: the sources the refusal loop may blame.
	refusedRecentSources = 8
	// refusedGateSample is how many changed entries one KERNEL_REFUSED_BASE run checks,
	// chosen by the hash of the name: a change refusing a thousand cards at once still
	// fits a gate, and the whole list runs before develop reaches main.
	refusedGateSample = 50
)

// staleRemedy is what a stale entry asks of the seat: either cause, and what each needs.
const staleRemedy = "A stale entry means either the defect is fixed (add the passing test and a row in dirac/support/unrefused.tsv, then go run ./dirac/cmd/kgen -unrefuse <card>) or the pinned game moved (re-record the game with pool custom; see checkRefusedRepro)"

// refusedRun is one entry's pool: its raw set and the decks its pairs draw from.
type refusedRun struct {
	card    string
	surface string
	mulls   int
	policy  func(uint64) Policy
	set     cardset.Set
	c       *Corpus
	pool    []PoolDeck
	seed    uint64
}

func refusedPolicy(t *testing.T, name string) func(uint64) Policy {
	t.Helper()
	switch name {
	case "random":
		return RandomPolicy
	case "heuristic":
		return HeuristicPolicy
	case "explore":
		return ExplorePolicy
	case "first":
		return func(uint64) Policy { return FirstPolicy() }
	}
	t.Fatalf("policy %q", name)
	return nil
}

// isSynthPool: the synthetic pools are dealt, a deck pool is read.
func isSynthPool(name string) bool {
	return strings.HasPrefix(name, "std-") || strings.HasPrefix(name, "tail-")
}

// openRefusedRun opens the entry's pool. A tail pool is cut by hash over what lowers,
// so the card can have moved: then the pool is whichever synthetic one holds it.
func openRefusedRun(t *testing.T, e *cardset.RefusedEntry) *refusedRun {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	poolName := e.Repro.Pool
	set, err := loadRefusedSet(poolName)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Lookup(e.Card); !ok {
		names, err := cardset.SynthNames()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, n := range names {
			s, err := cardset.SynthByName(n)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := s.Lookup(e.Card); ok {
				poolName, set, found = n, s, true
				break
			}
		}
		if !found {
			t.Fatalf("%s is in no synthetic pool", e.Card)
		}
	}
	r := &refusedRun{card: e.Card, surface: e.Repro.Surface, mulls: e.Repro.Mulligans, policy: refusedPolicy(t, e.Repro.Policy), set: set, seed: e.Repro.Seed}
	if isSynthPool(poolName) {
		if r.c, err = OpenCorpus(cardsDir, ""); err != nil {
			t.Fatal(err)
		}
		var files map[string]deck.File
		if r.pool, files, err = SynthPool(r.c.Reg, set, r.seed, SynthDecks); err != nil {
			t.Fatal(err)
		}
		r.c.AddDecks(files)
	} else {
		r.c, r.pool = testPoolCorpus(t, poolName)
	}
	e.Repro.Pool = poolName
	return r
}

func loadRefusedSet(pool string) (cardset.Set, error) {
	if isSynthPool(pool) {
		return cardset.SynthByName(pool)
	}
	return cardset.RawByName(pool)
}

// holdsCard reports whether either deck of p has the entry's card.
func (r *refusedRun) holdsCard(p Pairing) bool {
	for _, name := range []string{p.A, p.B} {
		f, err := r.c.deckFile(name)
		if err == nil && slices.ContainsFunc(f.Cards, func(e deck.Entry) bool { return e.Name == r.card }) {
			return true
		}
	}
	return false
}

// diverges plays pair p and reports whether its divergence implicates the card. A
// pair that cannot be built or does not finish in time shows nothing.
func (r *refusedRun) diverges(p Pairing) bool {
	done := make(chan bool, 1)
	go func() {
		defer func() {
			if recover() != nil {
				done <- false
			}
		}()
		m, tbl, err := SetMatchup(r.set, r.c, p.A, p.B, r.mulls)
		if err != nil {
			done <- false
			return
		}
		rc := RunConfig{Config: Config{GorgeAutopaySurface: r.surface == "autopay"}, MaxDecisions: 20000, MaxTurns: 100}
		k, err := SetSide(r.set, tbl, rc.Config)(m, p.Seed)
		if err != nil {
			done <- false
			return
		}
		res := Lockstep(m, p.Seed, k, r.policy(p.Seed), rc)
		CloseSide(k)
		done <- gameImplicates(&res, m, r.card)
	}()
	select {
	case d := <-done:
		return d
	case <-time.After(refusedGameCap):
		return false
	}
}

// gameImplicates reports whether a bad game points at card, as the refusal loop reads
// it: a divergence's implicated cards, or the latest sources of an unmodelled stall or
// of a divergence that names none or another card as the victim.
func gameImplicates(res *Result, m *Matchup, card string) bool {
	if res.Clean() {
		return false
	}
	if res.Div != nil && slices.Contains(res.Div.Implicates(m), card) {
		return true
	}
	return slices.Contains(res.RecentSources(m, refusedRecentSources), card)
}

// changedRefused is the entries' files that differ from base.
func changedRefused(t *testing.T, base string) map[string]bool {
	t.Helper()
	cmd := exec.Command("git", "diff", "--name-only", base, "--", cardset.RefusedDir)
	cmd.Dir = moduleRootFromOracle
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git diff %s: %v", base, err)
	}
	changed := map[string]bool{}
	for _, f := range strings.Fields(string(out)) {
		changed[strings.TrimSuffix(filepath.Base(f), ".json")] = true
	}
	return changed
}

// TestRefusedEntriesStillDiverge: a card may be refused only while it diverges. A
// clean replay of an entry's repro is not staleness, since games shift as gorge moves:
// the card's pool is searched for another game that implicates it, and only finding
// none fails. DIRAC_UPDATE_SUPPORT=1 writes the newer game into the entry.
// KERNEL_REFUSED_BASE=<rev> checks only the entries that differ from rev, at most
// refusedGateSample of them; KERNEL_REFUSED_SAMPLE=<n> checks n of all of them.
func TestRefusedEntriesStillDiverge(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	var only map[string]bool
	if base := os.Getenv("KERNEL_REFUSED_BASE"); base != "" {
		only = sampleRefused(changedRefused(t, base))
	} else if n, _ := strconv.Atoi(os.Getenv("KERNEL_REFUSED_SAMPLE")); n > 0 {
		all := map[string]bool{}
		for i := range entries {
			all[cardset.SupportSlug(entries[i].Card)] = true
		}
		only = sampleOf(all, n)
	}
	for i := range entries {
		e := &entries[i]
		if only != nil && !only[cardset.SupportSlug(e.Card)] {
			continue
		}
		t.Run(e.Card, func(t *testing.T) {
			t.Parallel()
			checkRefusedEntry(t, e)
		})
	}
}

// sampleRefused is at most refusedGateSample of the changed entries.
func sampleRefused(changed map[string]bool) map[string]bool {
	return sampleOf(changed, refusedGateSample)
}

// sampleOf is the n slugs with the lowest name hash, a choice that cannot favour an entry.
func sampleOf(slugs map[string]bool, n int) map[string]bool {
	keys := slices.Collect(maps.Keys(slugs))
	slices.SortFunc(keys, func(a, b string) int {
		return cmp.Compare(cardset.SynthNameHash(a), cardset.SynthNameHash(b))
	})
	out := map[string]bool{}
	for _, k := range keys[:min(n, len(keys))] {
		out[k] = true
	}
	return out
}

func checkRefusedEntry(t *testing.T, e *cardset.RefusedEntry) {
	if e.Repro.Pool == "custom" {
		checkRefusedRepro(t, e)
		return
	}
	r := openRefusedRun(t, e)
	if err := refusedPlayable(r); err != nil {
		t.Fatal(err)
	}
	if r.diverges(PairingAt(r.pool, e.Repro.Pair, r.seed)) {
		return
	}
	deadline := time.Now().Add(refusedEntryCap)
	played := 0
	for pair := 0; played < refusedPairsMax && pair < refusedPairsMax*100; pair++ {
		if time.Now().After(deadline) {
			t.Fatalf("inconclusive: %d pairs of %s in %s without a divergence implicating %s", played, e.Repro.Pool, refusedEntryCap, e.Card)
		}
		p := PairingAt(r.pool, pair, r.seed)
		if !r.holdsCard(p) {
			continue
		}
		played++
		if !r.diverges(p) {
			continue
		}
		e.Repro.Pair = pair
		t.Logf("%s's repro plays clean; %s pair %d still diverges", e.Card, e.Repro.Pool, pair)
		if os.Getenv("DIRAC_UPDATE_SUPPORT") == "1" {
			b, err := e.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(moduleRootFromOracle, filepath.FromSlash(cardset.RefusedDir), cardset.SupportSlug(e.Card)+".json")
			if err := os.WriteFile(path, b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	t.Errorf("stale: %d pairs of %s with %s dealt in and none implicates it. %s", played, e.Repro.Pool, e.Card, staleRemedy)
}

// checkRefusedRepro is checkRefusedEntry for an entry pinned to a recorded game (pool
// custom): the game is replayed by its script on a raw table of its own decks' cards,
// and only a divergence that implicates the card keeps the refusal. There is no pool
// to search, so a clean replay is staleness.
func checkRefusedRepro(t *testing.T, e *cardset.RefusedEntry) {
	if e.Repro.File == "" {
		t.Fatalf("%s: a custom repro names no file", e.Card)
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", e.Repro.File+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.Raw(printedDelveSet(t, r.Cards))
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if !gameImplicates(&res, m, e.Card) {
		t.Errorf("stale: %s's recorded game %s no longer diverges with it implicated. %s", e.Card, e.Repro.File, staleRemedy)
	}
}

// refusedPlayable: an entry naming a card kgen does not lower has nothing to replay.
func refusedPlayable(r *refusedRun) error {
	i, ok := r.set.Lookup(r.card)
	if !ok {
		return fmt.Errorf("%s is not in %s", r.card, r.set.Name())
	}
	if !r.set.Supported()[i] {
		return errors.New(r.set.Reason(i))
	}
	return nil
}

// activateOnlyPolicy activates whenever it can (an exhaust or loyalty
// ability is a rare option), else plays randomly.
func activateOnlyPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// gorgeOptionName is the name of the object a canonical option names.
func gorgeOptionName(c *PickCtx, a Canon) string {
	if c.G == nil {
		return ""
	}
	o, ok := c.G.GorgeOption(a)
	if !ok {
		return ""
	}
	if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
		return ob.Face().Name
	}
	return ""
}

func battlefieldHasName(g *state.Game, pl state.PlayerID, name string) bool {
	for _, b := range g.Zone(state.ZBattlefield, pl) {
		if chName(g, b) == name {
			return true
		}
	}
	return false
}
