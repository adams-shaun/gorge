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
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
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
