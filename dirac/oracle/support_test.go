package oracle

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	moduleRootFromOracle = "../.."

	// refusedPairsMax bounds the search once a repro plays clean: pairs with the card dealt in. The search is bounded
	// by pairs only, never by time: 11-22 batch #3458 failed Smiting Helix "inconclusive: 136 pairs of tail-45 in
	// 1m30s" on a loaded runner, while the same tree found pair 1228 diverging in 33 s on an idle one. A pair budget
	// gives the same verdict on every box; load only makes it slower.
	refusedPairsMax = 200
	refusedGameCap  = 10 * time.Second // a game past this is a hang, which proves nothing
	// refusedRecentSources is kfuzz's bad-game line: the sources the refusal loop may blame.
	refusedRecentSources = 8
	// refusedGateSample is how many changed entries one KERNEL_REFUSED_BASE run checks,
	// chosen by the hash of the name: a change refusing a thousand cards at once still
	// fits a gate, and the whole list runs before develop reaches main.
	refusedGateSample = 50
)

// staleRemedy is what a stale entry asks of the seat: either cause, and what each needs.
const staleRemedy = "A stale entry means the defect is fixed and the bot handles it (scripts/regen-main.sh warns, records the clean game and unrefuses the card with a proof row); a ticket fixing the card on purpose may still unrefuse it in its own PR (add the passing test and a row in dirac/support/unrefused.tsv, then go run ./dirac/cmd/kgen -unrefuse <card>); if only the pinned game moved, re-record it with pool custom (see checkRefusedRepro)"

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
	return cardset.IsSynthName(name)
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

// searchedPair is one pair a search played.
type searchedPair struct {
	res        *Result  // nil: the game could not be built or did not finish
	m          *Matchup // the game's matchup (nil with res)
	implicates bool     // the result implicates the card (gameImplicates)
}

// cleanGame is the last clean game a search played with the card dealt in: what
// NewRepro needs to record it as the unrefusal's proof.
type cleanGame struct {
	res  *Result
	m    *Matchup
	seed uint64
	rc   RunConfig
}

// refusedSearch is what searching a refused entry's games ended on: the pairs played
// with the card dealt in, the pair whose game still implicates it (-1: none did, a
// stale refusal), and the last clean game (the unrefusal's proof).
type refusedSearch struct {
	card, pool string
	played     int
	found      int
	cleans     []cleanGame // the last clean games played, candidates for the proof
	clean      *cleanGame  // the candidate that replays clean on its own cards' table (nil: none did)
}

// cleanCandidates bounds refusedSearch.cleans: the games kept to try as the unrefusal's proof.
const cleanCandidates = 6

// proofRepro is the game recorded as an unrefusal's proof: NewRepro's recording, pooled
// "one-off" as the unrefusal proofs are (TestReproGlobMatchesTable wants a pool), and marked
// Stall when the game ended at a declared kernel stall.
func (c *cleanGame) proofRepro(card string) *Repro {
	r := NewRepro(c.m, c.seed, c.rc, c.res)
	r.Pool = "one-off"
	r.Stall = strings.HasPrefix(c.res.Stall, KernelStallPrefix)
	r.Why = card + " plays clean: recorded by regen-main as its unrefusal's proof"
	return r
}

// provenClean is the first candidate (latest first) whose recording replays clean on a raw
// table of its own cards, the replay TestUnrefusalNeedsProof runs: a game that was clean on
// the search's pool table can diverge there. nil when none does.
func provenClean(t *testing.T, card string, cands []cleanGame) *cleanGame {
	t.Helper()
	for i := len(cands) - 1; i >= 0; i-- {
		res, _, err := replayOnOwnCards(t, cands[i].proofRepro(card))
		if err == nil && res.Clean() {
			return &cands[i]
		}
	}
	return nil
}

// runConfig is the config every search game runs under.
func (r *refusedRun) runConfig() RunConfig {
	return RunConfig{Config: Config{GorgeAutopaySurface: r.surface == "autopay"}, MaxDecisions: 20000, MaxTurns: 100}
}

// search plays pair p and reports its result and whether the game implicates the card.
// ok is false when the game could not be built or did not finish in refusedGameCap: it
// shows nothing.
func (r *refusedRun) search(p Pairing) (sp searchedPair, ok bool) {
	rc := r.runConfig()
	done := make(chan searchedPair, 1)
	go func() {
		defer func() {
			if recover() != nil {
				done <- searchedPair{}
			}
		}()
		m, tbl, err := SetMatchup(r.set, r.c, p.A, p.B, r.mulls)
		if err != nil {
			done <- searchedPair{}
			return
		}
		k, err := SetSide(r.set, tbl, rc.Config)(m, p.Seed)
		if err != nil {
			done <- searchedPair{}
			return
		}
		res := Lockstep(m, p.Seed, k, r.policy(p.Seed), rc)
		CloseSide(k)
		done <- searchedPair{res: &res, m: m, implicates: gameImplicates(&res, m, r.card)}
	}()
	select {
	case sp = <-done:
		return sp, sp.res != nil
	case <-time.After(refusedGameCap):
		return searchedPair{}, false
	}
}

// diverges is search's boolean: whether the game implicates the card. A pair that
// cannot be built or does not finish in time shows nothing.
func (r *refusedRun) diverges(p Pairing) bool {
	sp, ok := r.search(p)
	return ok && sp.implicates
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

// refusedShards is how many top-level tests the refusal check is dealt into:
// one entry's pool search can run for 42 s (Whim of Volrath's tail-02 hunt,
// 11-27) and the whole list is ~178 s of leaf searches, which no cost deal can
// put inside a lane's share of the package cap as one test. Entry i plays in
// shard i % refusedShards. The precheck's lanes, CI and regen-main name the
// shards (^TestRefusedEntriesStillDiverge[0-9]+$); the dirac gate names the
// parent, TestRefusedEntriesStillDiverge, which checks every entry itself.
const refusedShards = 8

// TestRefusedEntriesStillDiverge: a card may be refused only while it diverges. A
// clean replay of an entry's repro is not staleness, since games shift as gorge moves:
// the card's pool is searched for another game that implicates it, and only finding
// none fails. DIRAC_UPDATE_SUPPORT=1 writes the newer game into the entry.
// KERNEL_REFUSED_BASE=<rev> checks only the entries that differ from rev, at most
// refusedGateSample of them; KERNEL_REFUSED_SAMPLE=<n> checks n of all of them
// (the same sample in every shard, each checking its own entries of it).
//
// The parent checks the entries of every shard this run's -run does not also
// select: all of them under the dirac gate's ^TestRefusedEntriesStillDiverge$,
// none in a whole-package run or under an unanchored "RefusedEntriesStillDiverge"
// (the shards check them), so no entry is replayed twice in one process. A deal
// that lists the package's tests by name drops the parent
// (scripts/ci-oracle-shard.sh, test-targets.sh oracle_chunked): the shards it
// also deals cover it.
func TestRefusedEntriesStillDiverge(t *testing.T) {
	run := ""
	if f := flag.Lookup("test.run"); f != nil {
		run = f.Value.String()
	}
	left := refusedShardsLeft(run)
	if len(left) == 0 {
		t.Log("every shard TestRefusedEntriesStillDiverge0..7 runs in this process: they check the entries")
		return
	}
	refusedEntries(t, func(i int) bool { return left[i%refusedShards] })
}

func TestRefusedEntriesStillDiverge0(t *testing.T) { refusedEntriesShard(t, 0) }
func TestRefusedEntriesStillDiverge1(t *testing.T) { refusedEntriesShard(t, 1) }
func TestRefusedEntriesStillDiverge2(t *testing.T) { refusedEntriesShard(t, 2) }
func TestRefusedEntriesStillDiverge3(t *testing.T) { refusedEntriesShard(t, 3) }
func TestRefusedEntriesStillDiverge4(t *testing.T) { refusedEntriesShard(t, 4) }
func TestRefusedEntriesStillDiverge5(t *testing.T) { refusedEntriesShard(t, 5) }
func TestRefusedEntriesStillDiverge6(t *testing.T) { refusedEntriesShard(t, 6) }
func TestRefusedEntriesStillDiverge7(t *testing.T) { refusedEntriesShard(t, 7) }

// refusedShardsLeft is the shards a -run pattern does not select by their
// top-level names (go's matching: each top-level '|' alternative's first
// '/'-separated element, unanchored; an empty or invalid one selects every
// test, so none is left).
func refusedShardsLeft(run string) map[int]bool {
	re, err := regexp.Compile(topRunElem(run))
	if err != nil {
		return nil
	}
	left := map[int]bool{}
	for i := range refusedShards {
		if !re.MatchString("TestRefusedEntriesStillDiverge" + strconv.Itoa(i)) {
			left[i] = true
		}
	}
	return left
}

// topRunElem is the part of a -run pattern that matches top-level names, split
// as the testing package splits it: on '|' and '/' outside brackets and
// parentheses, a '|' ending an alternative. It is every alternative's first
// '/'-separated element, joined by '|'.
func topRunElem(run string) string {
	var alts []string
	cs, ps, from, cut := 0, 0, 0, -1
	for i := 0; i < len(run); i++ {
		switch run[i] {
		case '[':
			cs++
		case ']':
			if cs > 0 {
				cs--
			}
		case '(':
			if cs == 0 {
				ps++
			}
		case ')':
			if cs == 0 && ps > 0 {
				ps--
			}
		case '\\':
			i++
		case '/':
			if cs == 0 && ps == 0 && cut < 0 {
				cut = i
			}
		case '|':
			if cs == 0 && ps == 0 {
				alts = append(alts, firstElem(run[from:i], cut-from))
				from, cut = i+1, -1
			}
		}
	}
	return strings.Join(append(alts, firstElem(run[from:], cut-from)), "|")
}

// firstElem is alt up to cut, its first top-level '/' (none when cut < 0).
func firstElem(alt string, cut int) string {
	if cut < 0 {
		return alt
	}
	return alt[:cut]
}

// refusedEntriesShard checks the entries with i % refusedShards == shard. The
// shards run in parallel: each writes only its own entries' files, and a
// land-grant entry replays in an isolate child, so no shard flips a global
// another one reads.
func refusedEntriesShard(t *testing.T, shard int) {
	t.Parallel()
	refusedEntries(t, func(i int) bool { return i%refusedShards == shard })
}

// refusedEntries checks the refused entries whose index mine selects.
func refusedEntries(t *testing.T, mine func(i int) bool) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	entries = slices.DeleteFunc(entries, deferredRefusal) // before sampling and the deal: no divergence is claimed
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
		if !mine(i) {
			continue
		}
		if only != nil && !only[cardset.SupportSlug(e.Card)] {
			continue
		}
		t.Run(e.Card, func(t *testing.T) {
			if landGrantRefusedCards[e.Card] {
				if landGrantReplay(t) {
					return // its pair is refused at deal now: replayed alone with that refusal off
				}
			} else {
				t.Parallel()
			}
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
	if sp, ok := r.search(PairingAt(r.pool, e.Repro.Pair, r.seed)); ok && sp.implicates {
		return
	}
	s := refusedSearch{card: e.Card, pool: e.Repro.Pool, found: -1}
	for pair := 0; s.played < refusedPairsMax && pair < refusedPairsMax*100; pair++ {
		p := PairingAt(r.pool, pair, r.seed)
		if !r.holdsCard(p) {
			continue
		}
		s.played++
		sp, ok := r.search(p)
		if !ok {
			continue
		}
		if sp.implicates {
			s.found = pair
			break
		}
		if sp.res.Clean() {
			s.cleans = append(s.cleans, cleanGame{res: sp.res, m: sp.m, seed: p.Seed, rc: r.runConfig()})
			if len(s.cleans) > cleanCandidates {
				s.cleans = s.cleans[1:]
			}
		}
	}
	if s.found < 0 && os.Getenv("DIRAC_PIN_STRICT") != "1" && os.Getenv("DIRAC_UNREFUSE_STALE") != "" {
		s.clean = provenClean(t, s.card, s.cleans)
	}
	staleMsg := fmt.Sprintf("stale: %d pairs of %s with %s dealt in and none implicates it. %s", s.played, s.pool, s.card, staleRemedy)
	warnBase := fmt.Sprintf("WARNING: %s no longer diverges (%d pairs of %s)", s.card, s.played, s.pool)
	failed, warn := staleVerdict(s, staleMsg, warnBase, os.Getenv("DIRAC_PIN_STRICT") == "1", os.Getenv("DIRAC_UNREFUSE_STALE"))
	if failed != "" {
		t.Error(failed)
		return
	}
	if warn != "" {
		t.Log(warn)
	}
	if s.found >= 0 {
		e.Repro.Pair = s.found
		t.Logf("%s's repro plays clean; %s pair %d still diverges", e.Card, e.Repro.Pool, s.found)
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
	}
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
	res, m, err := replayOnOwnCards(t, r)
	if err != nil {
		t.Fatal(err)
	}
	if gameImplicates(&res, m, e.Card) {
		return
	}
	// A stale recorded game has no pool to search: its own replay, clean, is the candidate.
	s := refusedSearch{card: e.Card, pool: "custom", played: 1, found: -1}
	if res.Clean() && os.Getenv("DIRAC_PIN_STRICT") != "1" && os.Getenv("DIRAC_UNREFUSE_STALE") != "" {
		s.clean = provenClean(t, e.Card, []cleanGame{{res: &res, m: m, seed: r.Seed, rc: r.RunConfig()}})
	}
	staleMsg := fmt.Sprintf("stale: %s's recorded game %s no longer diverges with it implicated. %s", e.Card, e.Repro.File, staleRemedy)
	warnBase := fmt.Sprintf("WARNING: %s no longer diverges (its recorded game %s)", e.Card, e.Repro.File)
	failed, warn := staleVerdict(s, staleMsg, warnBase, os.Getenv("DIRAC_PIN_STRICT") == "1", os.Getenv("DIRAC_UNREFUSE_STALE"))
	if failed != "" {
		t.Error(failed)
		return
	}
	if warn != "" {
		t.Log(warn)
	}
}

// staleVerdict is the decision a refused entry's search ends on (checkRefusedEntry's and
// checkRefusedRepro's tail). A pair that still implicates the card holds the register's
// assertion: the check passes. A stale refusal — no searched game implicates the card —
// fails under DIRAC_PIN_STRICT=1 (the nightly, which keeps the assertion); outside it a
// fixed card is progress, not a red batch: it warns, and DIRAC_UNREFUSE_STALE=<dir>
// records clean (the last clean game, the unrefusal's proof) and a stale.tsv line for
// scripts/unrefuse-stale.sh, which unrefuses the card. staleMsg is the failure the pin
// demands, warnBase the warning's head ("WARNING: <card> no longer diverges (<played>
// pairs of <pool>)"). It returns the failure to raise ("" when the check passes) and the
// warning to log ("" under the pin).
func staleVerdict(s refusedSearch, staleMsg, warnBase string, strict bool, staleDir string) (failed, warn string) {
	if s.found >= 0 {
		return "", ""
	}
	if strict {
		return staleMsg, ""
	}
	warn = warnBase + ": scripts/regen-main.sh unrefuses it"
	if staleDir == "" {
		return "", warn
	}
	if s.clean == nil {
		return "", warnBase + ": no game that replays clean on its own cards' table was found to record as its proof, so scripts/regen-main.sh cannot unrefuse it; the nightly (DIRAC_PIN_STRICT=1) stays red until it is unrefused by hand"
	}
	if err := writeStaleUnrefuse(staleDir, s); err != nil {
		warn += " (recording the stale game failed: " + err.Error() + ")"
	}
	return "", warn
}

// writeStaleUnrefuse records a stale search's unrefusal under dir: the clean game as
// <slug>.repro.json, and the stale.tsv line the unrefusal script acts on (card, slug,
// the pairs played, the pool searched).
func writeStaleUnrefuse(dir string, s refusedSearch) error {
	slug := cardset.SupportSlug(s.card)
	r := s.clean.proofRepro(s.card)
	if err := r.Write(filepath.Join(dir, slug+".repro.json")); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "stale.tsv"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\t%s\t%d\t%s\n", s.card, slug, s.played, s.pool)
	return err
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

func kcastDeck(t *testing.T, reg *cards.Registry, md [2][]cardset.Entry) [2][]*cards.Card {
	t.Helper()
	var decks [2][]*cards.Card
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	return decks
}

// refCardName is the canonical card's printed name through the gorge side.
func refCardName(g *GorgeSide, ref Ref) string {
	if g == nil || ref >= RefStack || int(ref) >= len(g.id.obj) {
		return ""
	}
	o := g.E.G.Obj(g.id.obj[ref])
	if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
		return ""
	}
	return o.Card.Faces[0].Name
}

// refCardFullName is a two-faced card's full printed name ("Discovery //
// Dispersal", the spelling a deck list and a watch map name it by); "" when
// the card has no second face.
func refCardFullName(g *GorgeSide, ref Ref) string {
	if g == nil || ref >= RefStack || int(ref) >= len(g.id.obj) {
		return ""
	}
	o := g.E.G.Obj(g.id.obj[ref])
	if o == nil || o.Card == nil || len(o.Card.Faces) < 2 {
		return ""
	}
	return o.Card.Faces[0].Name + " // " + o.Card.Faces[1].Name
}

func objectHasType(o *state.Object, ty string) bool {
	if o == nil || o.Face() == nil {
		return false
	}
	for _, w := range o.Face().Types {
		if w == ty {
			return true
		}
	}
	return false
}

func replayEffectLongCase(t *testing.T, reg *cards.Registry, c effectLongCase) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatalf("the recorded game never reaches %s", c.name)
	}
}

// resolveSourceName names a resolving stack object: an ability's
// source permanent, else the stack card itself.
func resolveSourceName(g *state.Game, id state.ObjID) string {
	o := g.Obj(id)
	if o == nil {
		return ""
	}
	// An ability object (activated or triggered) has no card of its own:
	// its Source points at the permanent whose ability is resolving. A spell
	// has no Source and names its card directly.
	if o.Source != 0 {
		if n := chName(g, o.Source); n != "" {
			return n
		}
	}
	return chName(g, id)
}

// trackingEagerPolicy is the cast-first random policy of the czparams lockstep
// and captures the gorge side for the event tally.
func trackingEagerPolicy(seed uint64, played map[string]int, gs **GorgeSide) Policy {
	inner := eagerPolicy(seed, played)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			*gs = c.G
		}
		return inner.Pick(c)
	})
}

// combatCastPolicy attacks, blocks and casts whenever it can (so the
// carriers resolve and the pumped creatures meet blockers and burn), else
// random.
func combatCastPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActAttack || k == ActBlock || k == ActCast {
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

func synthReg(t *testing.T) *cards.Registry {
	t.Helper()
	dir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// castActivatePlayedPolicy casts or activates whenever it can (so the games reach the
// carriers), else random; it counts the carrier plays the gorge side poses.
func castActivatePlayedPolicy(seed uint64, played map[string]int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if k := a.Kind(); (k == ActCast || k == ActActivate) && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					played[ob.Face().Name]++
				}
			}
		}
		return a
	})
}

// optionFaceName returns the printed name of the gorge option a canonical
// action selects, or "" when it names no object.
func optionFaceName(c *PickCtx, a Canon) string {
	if c.G == nil {
		return ""
	}
	o, ok := c.G.GorgeOption(a)
	if !ok {
		return ""
	}
	ob := c.G.E.G.Obj(o.Obj)
	if ob == nil || ob.Face() == nil {
		return ""
	}
	return ob.Face().Name
}

// castActivatePolicy casts or activates whenever it can (so the games reach the
// carriers), else random.
func castActivatePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
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

// battlefieldObjects lists the objects on a player's battlefield.
func battlefieldObjects(g *state.Game, pl state.PlayerID) (out []*state.Object) {
	for _, b := range g.Zone(state.ZBattlefield, pl) {
		if o := g.Obj(b); o != nil {
			out = append(out, o)
		}
	}
	return out
}

func expandEntries(t *testing.T, reg *cards.Registry, es []cardset.Entry) []*cards.Card {
	t.Helper()
	var out []*cards.Card
	for _, e := range es {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			out = append(out, c)
		}
	}
	return out
}

// castActivateKindPolicy casts or activates whenever it can (so the Vehicles are
// crewed, the Walls sacrificed and the Dragons cast), else random.
func castActivateKindPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast || a.Kind() == ActActivate {
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

func sourceCardName(g *state.Game, id state.ObjID) string {
	o := g.Obj(id)
	if o == nil {
		return ""
	}
	if o.Ability != nil && o.Source != 0 {
		if s := g.Obj(o.Source); s != nil && s.Card != nil && len(s.Card.Faces) > 0 {
			return s.Card.Faces[0].Name
		}
	}
	if o.Card != nil && len(o.Card.Faces) > 0 {
		return o.Card.Faces[0].Name
	}
	return ""
}

// discardedIntoPlay is the condition that a card of the given name was put
// onto the battlefield by a discard move rather than cast (CR 614.1).
func discardedIntoPlay(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		evs := ref.E.L.Events
		for i := range evs {
			e := &evs[i]
			if e.Kind != events.MoveZone || e.To != state.ZBattlefield || !events.IsDiscard(*e) {
				continue
			}
			if o := ref.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
				return true
			}
		}
		return false
	}
}

func firstFaceName(g *state.Game, id state.ObjID) string {
	if o := g.Obj(id); o != nil && o.Card != nil && len(o.Card.Faces) > 0 {
		return o.Card.Faces[0].Name
	}
	return ""
}

// kickHoldPolicy (kfix-cast) takes a kicked cast whenever one is offered
// and otherwise never casts a held card unkicked, so Sowing Mycospawn waits
// for its {1}{C} kicker (Mind Stone) instead of being cast at four mana.
func kickHoldPolicy(base Policy, hold map[Ref]bool) Policy {
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && a.Mode()&0xF == 1 {
				return a
			}
			if a.Kind() == ActCast && hold[a.Src()] {
				continue
			}
			buf = append(buf, a)
		}
		if len(buf) == len(c.D.Options) || len(buf) == 0 {
			return base.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func refFaceName(c *PickCtx, r Ref) string {
	if c.G == nil || int(r) >= int(RefPlayer) {
		return ""
	}
	if o := c.G.E.G.Obj(c.G.id.obj[r]); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}
