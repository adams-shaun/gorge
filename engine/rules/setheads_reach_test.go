package rules

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Reach: which printed card features a game actually plays. A feature is a
// string "kw:Flying", "api:Draw", "trig:ChangesZone", "static:Continuous" or
// "repl:DamageDone"; a card carries a feature when any of its faces prints it
// (an ability's SubAbility chain, a trigger's effect and a replacement's
// ReplaceWith body included). A card is exercised when one of its objects is
// cast, played as a land, enters the battlefield, or puts an activated,
// triggered or keyword-triggered ability on the stack; the game then exercises
// every feature the card carries. That overstates a card whose other abilities
// never fired, in the same way for every game, which is what a comparison
// between game sets needs. The Standard weight of a feature is the number of
// distinct cards in testdata/sets/*.txt that carry it.

// cardFeatures returns c's features, sorted and unique.
func cardFeatures(c *cards.Card) []string {
	set := map[string]struct{}{}
	chain := func(a *cards.SA) {
		for ; a != nil; a = a.Sub {
			set["api:"+a.API] = struct{}{}
		}
	}
	for _, f := range c.Faces {
		if f == nil {
			continue
		}
		for _, k := range f.Keywords {
			name, _, _ := strings.Cut(k, ":")
			set["kw:"+name] = struct{}{}
		}
		for _, a := range f.Abilities {
			chain(a)
		}
		for _, t := range f.Triggers {
			set["trig:"+t.Mode] = struct{}{}
			chain(t.Effect)
		}
		for _, s := range f.Statics {
			set["static:"+s.Mode] = struct{}{}
		}
		for _, r := range f.Repls {
			set["repl:"+r.Event] = struct{}{}
			chain(r.With)
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// standardPool is every card named by testdata/sets/*.txt that the registry
// loads, in file then line order, with the sets that list each.
type standardPool struct {
	cards  []*cards.Card
	setsOf map[*cards.Card][]string
	// weight is feature -> number of cards in cards carrying it.
	weight map[string]int
	// setSize is set code -> number of cards listed.
	setSize map[string]int
	codes   []string
}

func loadStandardPool(t testing.TB, reg *cards.Registry) *standardPool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "sets", "*.txt"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no set lists under testdata/sets: %v", err)
	}
	sort.Strings(files)
	p := &standardPool{setsOf: map[*cards.Card][]string{}, weight: map[string]int{}, setSize: map[string]int{}}
	for _, fn := range files {
		code := strings.TrimSuffix(filepath.Base(fn), ".txt")
		raw, err := os.ReadFile(fn)
		if err != nil {
			t.Fatal(err)
		}
		p.codes = append(p.codes, code)
		for _, name := range strings.Split(string(raw), "\n") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			c, ok := reg.Lookup(name)
			if !ok || isBasicLand(c) {
				continue
			}
			p.setSize[code]++
			if _, seen := p.setsOf[c]; !seen {
				p.cards = append(p.cards, c)
			}
			p.setsOf[c] = append(p.setsOf[c], code)
		}
	}
	for _, c := range p.cards {
		for _, f := range cardFeatures(c) {
			p.weight[f]++
		}
	}
	return p
}

func isBasicLand(c *cards.Card) bool {
	return len(c.Faces) > 0 && c.Faces[0].IsLand() && strings.Contains(strings.Join(c.Faces[0].Types, " "), "Basic")
}

// exercisedCards returns the cards e's game exercised, in order of first
// exercise.
func exercisedCards(e *Engine) []*cards.Card {
	var out []*cards.Card
	seen := map[*cards.Card]struct{}{}
	add := func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil || o.Card == nil || isBasicLand(o.Card) {
			return
		}
		if _, ok := seen[o.Card]; !ok {
			seen[o.Card] = struct{}{}
			out = append(out, o.Card)
		}
	}
	for _, ev := range e.L.Events {
		switch ev.Kind {
		case events.PutOnStack, events.LandPlayed, events.TriggerPush, events.AbilityPush, events.KeywordTriggerPush:
			add(ev.Obj)
		case events.MoveZone:
			if ev.To == state.ZBattlefield {
				add(ev.Obj)
			}
		}
	}
	return out
}

// reach accumulates the features and cards a set of games exercised.
type reach struct {
	features map[string]struct{}
	cards    map[*cards.Card]struct{}
}

func newReach() *reach {
	return &reach{features: map[string]struct{}{}, cards: map[*cards.Card]struct{}{}}
}

// add folds cs in and returns the features it newly covers, sorted.
func (r *reach) add(cs []*cards.Card) []string {
	var fresh []string
	for _, c := range cs {
		r.cards[c] = struct{}{}
		for _, f := range cardFeatures(c) {
			if _, ok := r.features[f]; !ok {
				r.features[f] = struct{}{}
				fresh = append(fresh, f)
			}
		}
	}
	sort.Strings(fresh)
	return fresh
}

func (r *reach) clone() *reach {
	o := newReach()
	for f := range r.features {
		o.features[f] = struct{}{}
	}
	for c := range r.cards {
		o.cards[c] = struct{}{}
	}
	return o
}

func acceptanceReach(t testing.TB, reg *cards.Registry) *reach {
	t.Helper()
	r := newReach()
	for _, seats := range AcceptanceSeatCounts() {
		names, decks, err := testutil.AcceptanceDecks(reg, seats)
		if err != nil {
			t.Fatal(err)
		}
		e, _, err := PlayAcceptance(AcceptanceConfig(reg, names, decks), acceptanceTestBot, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.add(exercisedCards(e))
	}
	return r
}

func setGamesReach(t testing.TB, reg *cards.Registry, games []setGame) *reach {
	t.Helper()
	r := newReach()
	for _, g := range games {
		r.add(exercisedCards(playSetGame(t, reg, g)))
	}
	return r
}

var reachKinds = []string{"kw", "api", "trig", "static", "repl"}

// reportReach logs, per feature kind, how many of the Standard pool's
// distinct features r covers, the share of Standard card-feature incidences
// that covers, and the sets it touched.
func reportReach(t testing.TB, label string, p *standardPool, r *reach, topGaps int) {
	t.Helper()
	for _, kind := range reachKinds {
		total, covered, wTotal, wCovered := 0, 0, 0, 0
		var gaps []string
		for f, w := range p.weight {
			if !strings.HasPrefix(f, kind+":") {
				continue
			}
			total++
			wTotal += w
			if _, ok := r.features[f]; ok {
				covered++
				wCovered += w
			} else {
				gaps = append(gaps, f)
			}
		}
		sort.Slice(gaps, func(i, j int) bool {
			if p.weight[gaps[i]] != p.weight[gaps[j]] {
				return p.weight[gaps[i]] > p.weight[gaps[j]]
			}
			return gaps[i] < gaps[j]
		})
		if len(gaps) > topGaps {
			gaps = gaps[:topGaps]
		}
		for i, g := range gaps {
			gaps[i] = fmt.Sprintf("%s(%d)", strings.TrimPrefix(g, kind+":"), p.weight[g])
		}
		t.Logf("%s %-6s %4d/%4d distinct, weighted %5.1f%%; top gaps %s", label, kind, covered, total,
			100*float64(wCovered)/float64(max(wTotal, 1)), strings.Join(gaps, " "))
	}
	perSet := map[string]int{}
	inStd := 0
	for c := range r.cards {
		if codes, ok := p.setsOf[c]; ok {
			inStd++
			for _, code := range codes {
				perSet[code]++
			}
		}
	}
	touched, ten := 0, 0
	var line []string
	for _, code := range p.codes {
		if perSet[code] > 0 {
			touched++
		}
		if perSet[code] >= 10 {
			ten++
		}
		line = append(line, fmt.Sprintf("%s:%d", code, perSet[code]))
	}
	t.Logf("%s sets %d/%d with >=1 card, %d with >=10; %d distinct cards played, %d of them in the Standard pool (%d)",
		label, touched, len(p.codes), ten, len(r.cards), inStd, len(p.cards))
	t.Logf("%s per set: %s", label, strings.Join(line, " "))
}

// TestSetHeadsReach logs the reach of the acceptance games, the set-spread
// games and both together. It asserts nothing; GORGE_REACH=1 runs it.
func TestSetHeadsReach(t *testing.T) {
	if os.Getenv("GORGE_REACH") != "1" {
		t.Skip("set GORGE_REACH=1 to log reach")
	}
	reg := testutil.CorpusRegistry(t)
	p := loadStandardPool(t, reg)
	before := acceptanceReach(t, reg)
	reportReach(t, "acceptance", p, before, 25)
	games, err := parseSetGames(filepath.Join(setHeadsDir, "games.txt"))
	if err != nil {
		t.Skip("no games.txt yet")
	}
	only := setGamesReach(t, reg, games)
	reportReach(t, "setheads", p, only, 10)
	both := before.clone()
	for c := range only.cards {
		both.add([]*cards.Card{c})
	}
	reportReach(t, "combined", p, both, 25)
}

// ---- generator ----

var pairMasks = func() []uint8 {
	cols := []uint8{cards.ColourWhite, cards.ColourBlue, cards.ColourBlack, cards.ColourRed, cards.ColourGreen}
	var out []uint8
	for i := range cols {
		for j := i + 1; j < len(cols); j++ {
			out = append(out, cols[i]|cols[j])
		}
	}
	return out
}()

var basicFor = map[uint8]string{
	cards.ColourWhite: "Plains", cards.ColourBlue: "Island", cards.ColourBlack: "Swamp",
	cards.ColourRed: "Mountain", cards.ColourGreen: "Forest",
}

const (
	genSpells    = 23
	genLands     = 17
	genCreatures = 10
	genMaxCMC    = 5
)

// deckBuilder picks a 40-card singleton-leaning deck greedily: each slot goes
// to the card carrying the most Standard weight in features the deck and the
// family so far do not yet hold.
type deckBuilder struct {
	pool    *standardPool
	covered map[string]struct{}
}

func (b *deckBuilder) score(c *cards.Card, held map[string]struct{}) float64 {
	s := 0.0
	for _, f := range cardFeatures(c) {
		if _, ok := held[f]; ok {
			continue
		}
		w := math.Sqrt(float64(b.pool.weight[f]))
		if _, ok := b.covered[f]; ok {
			w *= 0.05
		}
		s += w
	}
	return s
}

func (b *deckBuilder) build(label string, set []*cards.Card, mask uint8, reg *cards.Registry) setDeck {
	var spells, lands []*cards.Card
	for _, c := range set {
		f := c.Faces[0]
		if c.ColourIdentity()&^mask != 0 {
			continue
		}
		switch {
		case f.IsLand():
			lands = append(lands, c)
		case f.Cmc() <= genMaxCMC:
			spells = append(spells, c)
		}
	}
	held := map[string]struct{}{}
	chosen := map[*cards.Card]int{}
	pick := func(from []*cards.Card, creaturesOnly bool) *cards.Card {
		var best *cards.Card
		bestScore := -1.0
		for _, c := range from {
			if creaturesOnly && !c.Faces[0].IsCreature() {
				continue
			}
			if chosen[c] >= 2 || (chosen[c] == 1 && len(from) > 3*genSpells) {
				continue
			}
			s := b.score(c, held) / float64(1+chosen[c]*4) / (1 + 0.25*float64(c.Faces[0].Cmc()))
			if s > bestScore || (s == bestScore && best != nil && c.Faces[0].Name < best.Faces[0].Name) {
				best, bestScore = c, s
			}
		}
		return best
	}
	n, creatures := 0, 0
	take := func(c *cards.Card) {
		chosen[c]++
		n++
		if c.Faces[0].IsCreature() {
			creatures++
		}
		for _, f := range cardFeatures(c) {
			held[f] = struct{}{}
		}
	}
	for creatures < genCreatures && n < genSpells {
		c := pick(spells, true)
		if c == nil {
			break
		}
		take(c)
	}
	for n < genSpells {
		c := pick(spells, false)
		if c == nil {
			break
		}
		take(c)
	}
	nl := 0
	for k := 0; k < 2 && len(lands) > 0; k++ {
		c := pick(lands, false)
		if c == nil {
			break
		}
		take(c)
		nl++
	}
	var cols []uint8
	for _, m := range []uint8{cards.ColourWhite, cards.ColourBlue, cards.ColourBlack, cards.ColourRed, cards.ColourGreen} {
		if mask&m != 0 {
			cols = append(cols, m)
		}
	}
	d := setDeck{Label: label}
	var cs []*cards.Card
	for c := range chosen {
		cs = append(cs, c)
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].Faces[0].Name < cs[j].Faces[0].Name })
	for _, c := range cs {
		d.Cards = append(d.Cards, setDeckEntry{Count: chosen[c], Name: c.Faces[0].Name})
	}
	left := genLands - nl
	for i, m := range cols {
		k := left / len(cols)
		if i < left%len(cols) {
			k++
		}
		d.Cards = append(d.Cards, setDeckEntry{Count: k, Name: basicFor[m]})
	}
	return d
}

// playCapped is PlayAcceptance with a smaller intent cap, so a candidate game
// that never settles is dropped instead of holding the generator.
func playCapped(cfg Config, capIntents int) (*Engine, error) {
	e := NewStartingPlayerChoice(cfg)
	answer := acceptanceTestBot(acceptanceBotSeed)
	e.AskStartingPlayer()
	e.Advance()
	for n := 0; !e.G.Over && e.Pending() != nil; n++ {
		if n >= capIntents {
			return e, fmt.Errorf("no finish within %d intents (turn %d)", capIntents, e.G.Turn)
		}
		if err := e.Submit(answer(e, e.Pending())); err != nil {
			return e, err
		}
	}
	if !e.G.Over {
		return e, fmt.Errorf("game did not finish (turn %d)", e.G.Turn)
	}
	return e, nil
}

func aside(g setGame, why string) {
	if dir := os.Getenv("GORGE_SETHEADS_SCRATCH"); dir != "" {
		os.WriteFile(filepath.Join(dir, "aside-"+g.ID+".txt"), []byte(formatSetGames([]setGame{g})+"\n# "+why+"\n"), 0o644)
	}
}

func formatSetGames(games []setGame) string {
	var sb strings.Builder
	sb.WriteString("# Set-spread golden games; see setheads_test.go. Generated by\n" +
		"# GORGE_GENERATE_SETHEADS=1 go test ./engine/rules/ -run '^TestSetHeadsGenerate$' -- then pinned data: edit by regenerating.\n")
	for _, g := range games {
		fmt.Fprintf(&sb, "\ngame %s %d\n", g.ID, g.Seed)
		for _, d := range g.Decks {
			fmt.Fprintf(&sb, "deck %s\n", d.Label)
			for _, c := range d.Cards {
				fmt.Fprintf(&sb, "%d %s\n", c.Count, c.Name)
			}
		}
	}
	return sb.String()
}

// TestSetHeadsGenerate chooses the set-spread games by greedy cover and
// rewrites testdata/setheads/games.txt: per set, three candidate two-seat
// games (decks from the set's colour pairs with the most eligible cards, built
// to hold the Standard-weighted features the family does not yet play), each
// played; the candidate that newly covers the most features is kept, if it
// covers at least GORGE_SETHEADS_MIN_GAIN (default 3). Only supported cards
// are drawn on, so a card becoming supported later never changes a pinned deck.
func TestSetHeadsGenerate(t *testing.T) {
	if os.Getenv("GORGE_GENERATE_SETHEADS") != "1" {
		t.Skip("set GORGE_GENERATE_SETHEADS=1 to regenerate games.txt")
	}
	reg := testutil.CorpusRegistry(t)
	p := loadStandardPool(t, reg)
	unsupported := knownUnsupported(t)
	covered := acceptanceReach(t, reg)
	t.Logf("acceptance covers %d features", len(covered.features))
	byCode := map[string][]*cards.Card{}
	for _, c := range p.cards {
		if _, bad := unsupported[c.Faces[0].Name]; bad {
			continue
		}
		for _, code := range p.setsOf[c] {
			byCode[code] = append(byCode[code], c)
		}
	}
	minGain := 3
	if v := os.Getenv("GORGE_SETHEADS_MIN_GAIN"); v != "" {
		fmt.Sscan(v, &minGain)
	}
	budget := 40 * time.Second
	var chosen []setGame
	var totalTime time.Duration
	for round := 0; round < 4 && totalTime < budget; round++ {
		kept := 0
		for si, code := range p.codes {
			if totalTime >= budget {
				break
			}
			set := byCode[code]
			type ranked struct {
				mask uint8
				n    int
			}
			var rk []ranked
			for _, m := range pairMasks {
				n := 0
				for _, c := range set {
					if c.ColourIdentity()&^m == 0 && !c.Faces[0].IsLand() {
						n++
					}
				}
				rk = append(rk, ranked{m, n})
			}
			sort.SliceStable(rk, func(i, j int) bool { return rk[i].n > rk[j].n })
			b := &deckBuilder{pool: p, covered: covered.features}
			type cand struct {
				g    setGame
				e    *Engine
				took time.Duration
				err  error
			}
			cands := make([]cand, 3)
			for k := range cands {
				a, bb := rk[(k+round)%5].mask, rk[(k+round+1)%5].mask
				id := fmt.Sprintf("%s-%d", strings.ToLower(code), round*3+k+1)
				// The second deck avoids what the first holds, so the two cover
				// different ground.
				da := b.build(id+"-a", set, a, reg)
				held := map[string]struct{}{}
				for f := range covered.features {
					held[f] = struct{}{}
				}
				for _, e := range da.Cards {
					if c, ok := reg.Lookup(e.Name); ok {
						for _, f := range cardFeatures(c) {
							held[f] = struct{}{}
						}
					}
				}
				b2 := &deckBuilder{pool: p, covered: held}
				cands[k].g = setGame{ID: id, Seed: uint64(1000 + round*300 + si*10 + k),
					Decks: []setDeck{da, b2.build(id+"-b", set, bb, reg)}}
			}
			var wg sync.WaitGroup
			for k := range cands {
				wg.Add(1)
				done := make(chan struct{})
				go func() {
					defer close(done)
					defer func() {
						// A candidate that trips an engine consistency check is
						// dropped, and its game written aside for a repro.
						if r := recover(); r != nil {
							cands[k].err = fmt.Errorf("panic: %.300v", r)
							aside(cands[k].g, fmt.Sprint(r))
						}
					}()
					cfg, err := resolveSetGame(reg, cands[k].g)
					if err != nil {
						cands[k].err = err
						return
					}
					start := time.Now()
					e, err := playCapped(cfg, 12000)
					cands[k].took, cands[k].e, cands[k].err = time.Since(start), e, err
				}()
				// A candidate that does not return in 30 seconds is stuck inside
				// the engine; it is abandoned (the goroutine is left to the
				// process's exit) and its game written aside.
				go func() {
					defer wg.Done()
					select {
					case <-done:
					case <-time.After(30 * time.Second):
						cands[k].err = fmt.Errorf("hung for 30 seconds")
						aside(cands[k].g, "hung for 30 seconds")
					}
				}()
			}
			wg.Wait()
			best, bestRate, bestGain := -1, 0.0, 0
			for k, c := range cands {
				if c.err != nil {
					t.Logf("%s: candidate %s dropped: %v", code, c.g.ID, c.err)
					continue
				}
				trial := covered.clone()
				fresh := trial.add(exercisedCards(c.e))
				w := 0.0
				for _, f := range fresh {
					w += math.Sqrt(float64(p.weight[f]))
				}
				// Cost is the event count, not the wall clock, so the choice is the same
				// on a loaded box: about 40k events play per second.
				cost := float64(len(c.e.L.Events)) / 40000
				rate := w / (0.3 + cost)
				t.Logf("%s: candidate %s gains %d features (weight %.1f), %d events, %v", code, c.g.ID, len(fresh), w, len(c.e.L.Events), c.took.Round(time.Millisecond))
				if len(fresh) >= minGain && rate > bestRate {
					best, bestRate, bestGain = k, rate, len(fresh)
				}
			}
			if best < 0 {
				continue
			}
			t.Logf("%s: keep %s, +%d features", code, cands[best].g.ID, bestGain)
			covered.add(exercisedCards(cands[best].e))
			totalTime += time.Duration(float64(len(cands[best].e.L.Events)) / 40000 * float64(time.Second))
			chosen = append(chosen, cands[best].g)
			// Written as it goes: a run that dies keeps what it chose.
			if err := os.WriteFile(filepath.Join(setHeadsDir, "games.txt"), []byte(formatSetGames(chosen)), 0o644); err != nil {
				t.Fatal(err)
			}
			kept++
		}
		if kept == 0 {
			break
		}
	}
	t.Logf("kept %d games, %v of estimated play time (events/40k per second), %d features covered", len(chosen), totalTime.Round(time.Millisecond), len(covered.features))
	if err := os.MkdirAll(filepath.Join(setHeadsDir, "heads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(setHeadsDir, "games.txt"), []byte(formatSetGames(chosen)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// decisionKinds plays cfg and tallies the kinds of the decisions posed to seat
// 0, over the whole game and over its first 40 (the length of the native wire
// golden).
func decisionKinds(t testing.TB, cfg Config) (all, first40 map[string]int, intents int) {
	t.Helper()
	all, first40 = map[string]int{}, map[string]int{}
	e := NewStartingPlayerChoice(cfg)
	answer := acceptanceTestBot(acceptanceBotSeed)
	e.AskStartingPlayer()
	e.Advance()
	seat0 := 0
	for ; !e.G.Over && e.Pending() != nil && intents < 100000; intents++ {
		d := e.Pending()
		if d.Player == 0 {
			all[string(d.Kind)]++
			if seat0 < 40 {
				first40[string(d.Kind)]++
			}
			seat0++
		}
		if err := e.Submit(answer(e, d)); err != nil {
			t.Fatal(err)
		}
	}
	return all, first40, intents
}

func kindsLine(m map[string]int) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for i, k := range ks {
		ks[i] = fmt.Sprintf("%s:%d", k, m[k])
	}
	return strings.Join(ks, " ")
}

// TestSetHeadsWireKinds logs the decision kinds seat 0 meets in the wire
// golden's scenario and in each set game (GORGE_REACH=1).
func TestSetHeadsWireKinds(t *testing.T) {
	if os.Getenv("GORGE_REACH") != "1" {
		t.Skip("set GORGE_REACH=1 to log reach")
	}
	reg := testutil.CorpusRegistry(t)
	names := []string{"death-n-taxes", "mono-red-prowess"}
	var decks [][]*cards.Card
	for _, n := range names {
		decks = append(decks, testutil.RepoDeck(t, reg, n))
	}
	cfg := AcceptanceConfig(reg, names, decks)
	cfg.Seed = 0x5bc0
	all, f40, n := decisionKinds(t, cfg)
	t.Logf("wire golden scenario: %d intents; first40 %s; all %s", n, kindsLine(f40), kindsLine(all))
	games, err := parseSetGames(filepath.Join(setHeadsDir, "games.txt"))
	if err != nil {
		t.Skip("no games.txt yet")
	}
	for _, g := range games {
		cfg, err := resolveSetGame(reg, g)
		if err != nil {
			t.Fatal(err)
		}
		all, f40, n := decisionKinds(t, cfg)
		t.Logf("game %s: %d intents; first40 %s; all %s", g.ID, n, kindsLine(f40), kindsLine(all))
	}
}
