// Command cardfuzz plays bot-vs-bot games between randomly generated
// mono-colour decks drawn from the whole supported corpus, to surface engine
// panics, livelocks, submit errors and replay divergences that the curated
// repo decks never reach.
//
// Each deck is 60 cards: 20 lands (basics of its colour, up to 3 of them
// swapped for sampled non-basic lands: the ~1/3 mana share), and 40 distinct non-land cards
// sampled from the supported cards whose colour identity is that colour or
// colourless. Sampling is weighted toward cards the accumulated coverage
// state has seen least, so repeated runs walk the whole pool.
//
// Runs go in batches: every deck of a batch is generated sequentially from
// (-seed, game index) and the coverage snapshot taken at the batch's start,
// then the batch plays on -workers goroutines.
//
// A game whose live object count passes -max-objects (a runaway token
// engine) is ended by the harness and recorded as kind "bigboard", distinct
// from an engine "hang" or "livelock". Every failure is appended to
// -failures as one JSON line carrying both full deck lists, the game seed
// and the diagnostic, and `cardfuzz -repro <file> -line N` replays it.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/host"
	gbench "github.com/adams-shaun/gorge/internal/bench"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/state"
)

var colourNames = []string{"W", "U", "B", "R", "G"}
var basicFor = []string{"Plains", "Island", "Swamp", "Mountain", "Forest"}

// excludedTypes are card types that never go in a 60-card main deck.
var excludedTypes = []string{"Conspiracy", "Scheme", "Plane", "Phenomenon", "Vanguard", "Dungeon", "Attraction", "Contraption", "Emblem", "Token"}

type poolCard struct {
	card *cards.Card
	name string
	land bool
	// keys is the card's usable-ability inventory (abilityInventory), so the
	// sampling weight can ask whether every one has been used.
	keys []string
}

// pool holds, per colour index 0..4, the eligible non-land and land cards
// (colourless cards appear under every colour).
type pool struct {
	spells [5][]poolCard
	lands  [5][]poolCard
	all    map[string]bool
	cards  map[string]*cards.Card
	keys   map[string][]string // name -> abilityKeys, computed once
	basics [5]*cards.Card
}

func cardName(c *cards.Card) string { return c.Faces[0].Name }

func eligible(c *cards.Card) bool {
	if len(c.Faces) == 0 || c.Faces[0].Name == "" {
		return false
	}
	f := c.Faces[0]
	for _, t := range f.Types {
		for _, x := range excludedTypes {
			if t == x {
				return false
			}
		}
	}
	if f.IsBasic() && f.IsLand() {
		return false
	}
	if strings.Contains(strings.ToLower(f.Oracle), "playing for ante") {
		return false
	}
	// Alchemy rebalances and other digital variants duplicate a printed card.
	if strings.HasPrefix(f.Name, "A-") {
		return false
	}
	return true
}

func identity(c *cards.Card) uint8 {
	var id uint8
	for _, f := range c.Faces {
		id |= f.ColourIdentity()
	}
	return id
}

func buildPool(reg *cards.Registry) (*pool, error) {
	p := &pool{all: map[string]bool{}, cards: map[string]*cards.Card{}, keys: map[string][]string{}}
	sup := effects.Supported()
	for _, c := range reg.Cards {
		if !eligible(c) || len(reg.Unsupported(c, sup)) > 0 {
			continue
		}
		id := identity(c)
		pc := poolCard{card: c, name: cardName(c), land: c.Faces[0].IsLand(), keys: abilityKeys(c)}
		added := false
		for i := 0; i < 5; i++ {
			if id != 0 && id != 1<<i {
				continue
			}
			if pc.land {
				p.lands[i] = append(p.lands[i], pc)
			} else {
				p.spells[i] = append(p.spells[i], pc)
			}
			added = true
		}
		if added {
			p.all[pc.name] = true
			p.cards[pc.name] = c
			p.keys[pc.name] = pc.keys
		}
	}
	for i, b := range basicFor {
		c, ok := reg.Lookup(b)
		if !ok {
			return nil, fmt.Errorf("basic %s not in corpus", b)
		}
		p.basics[i] = c
	}
	for i := 0; i < 5; i++ {
		sort.Slice(p.spells[i], func(a, b int) bool { return p.spells[i][a].name < p.spells[i][b].name })
		sort.Slice(p.lands[i], func(a, b int) bool { return p.lands[i][a].name < p.lands[i][b].name })
	}
	return p, nil
}

// cov is the persistent per-card coverage state.
type cov struct {
	Games    int64            `json:"games"`
	Included map[string]int64 `json:"included"`
	Cast     map[string]int64 `json:"cast"`
	Ability  map[string]int64 `json:"ability"`
	Fails    map[string]int64 `json:"fails"`
	// Used counts, per card, the games in which each of its own abilities
	// (abilityInventory keys) was used. Absent from state files written
	// before it existed; those load with it empty.
	Used map[string]map[string]int64 `json:"used,omitempty"`
}

func loadCov(path string) (*cov, error) {
	c := &cov{Included: map[string]int64{}, Cast: map[string]int64{}, Ability: map[string]int64{}, Fails: map[string]int64{}, Used: map[string]map[string]int64{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, err
	}
	for _, m := range []*map[string]int64{&c.Included, &c.Cast, &c.Ability, &c.Fails} {
		if *m == nil {
			*m = map[string]int64{}
		}
	}
	if c.Used == nil {
		c.Used = map[string]map[string]int64{}
	}
	return c, nil
}

func (c *cov) save(path string) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// missing returns the keys of name's inventory never used yet, in keys'
// (sorted) order.
func (c *cov) missing(name string, keys []string) []string {
	var out []string
	u := c.Used[name]
	for _, k := range keys {
		if u[k] == 0 {
			out = append(out, k)
		}
	}
	return out
}

// full reports whether name was cast/played and every ability in keys used.
func (c *cov) full(name string, keys []string) bool {
	return c.Cast[name] > 0 && len(c.missing(name, keys)) == 0
}

// weight favours cards never cast (x8), cards cast but with some ability
// never used (x4), and seldom-included cards. It reads only the coverage
// snapshot, so it is deterministic for a given state.
func (c *cov) weight(pc poolCard) float64 {
	w := 1.0 / float64(1+c.Included[pc.name])
	switch {
	case c.Cast[pc.name] == 0:
		w *= 8
	case len(c.missing(pc.name, pc.keys)) > 0:
		w *= 4
	}
	return w
}

// sampleDistinct draws k distinct cards from cands by weight (Efraimidis-Spirakis).
func sampleDistinct(r *rand.Rand, cands []poolCard, k int, c *cov) []poolCard {
	if k >= len(cands) {
		return append([]poolCard(nil), cands...)
	}
	type kv struct {
		key float64
		i   int
	}
	keys := make([]kv, len(cands))
	for i, pc := range cands {
		u := r.Float64()
		if u == 0 {
			u = 1e-12
		}
		// key = u^(1/w); compare in log space.
		keys[i] = kv{key: math.Log(u) / c.weight(pc), i: i}
	}
	sort.Slice(keys, func(a, b int) bool {
		if keys[a].key != keys[b].key {
			return keys[a].key > keys[b].key
		}
		return keys[a].i < keys[b].i
	})
	out := make([]poolCard, k)
	for j := 0; j < k; j++ {
		out[j] = cands[keys[j].i]
	}
	return out
}

type genDeck struct {
	Colour string   `json:"colour"`
	Cards  []string `json:"cards"`
}

func generate(r *rand.Rand, p *pool, c *cov) genDeck {
	ci := r.IntN(5)
	nonbasic := sampleDistinct(r, p.lands[ci], 3, c)
	spells := sampleDistinct(r, p.spells[ci], 40, c)
	d := genDeck{Colour: colourNames[ci]}
	for i := 0; i < 20-len(nonbasic); i++ {
		d.Cards = append(d.Cards, basicFor[ci])
	}
	for _, pc := range nonbasic {
		d.Cards = append(d.Cards, pc.name)
	}
	for _, pc := range spells {
		d.Cards = append(d.Cards, pc.name)
	}
	return d
}

func resolveDeck(reg *cards.Registry, d genDeck) ([]*cards.Card, error) {
	out := make([]*cards.Card, 0, len(d.Cards))
	for _, n := range d.Cards {
		c, ok := reg.Lookup(n)
		if !ok {
			return nil, fmt.Errorf("card %q not found", n)
		}
		out = append(out, c)
	}
	return out, nil
}

// failure is one JSONL record.
type failure struct {
	Kind    string    `json:"kind"`
	Seed    uint64    `json:"seed"`
	Decks   []genDeck `json:"decks"`
	Turns   int32     `json:"turns"`
	Intents int       `json:"intents"`
	Diag    string    `json:"diag"`
	Sig     string    `json:"sig"`
}

// signature reduces a diagnostic to a dedupe key: for a panic, the panic
// value plus the first engine frame; otherwise the first line.
func signature(kind, diag string) string {
	lines := strings.Split(diag, "\n")
	first := lines[0]
	if i := strings.Index(first, "): "); i >= 0 && strings.HasPrefix(first, "engine panic") {
		first = first[i+3:]
	}
	if kind == "panic" {
		for i, l := range lines {
			if strings.Contains(l, "github.com/adams-shaun/gorge/") && !strings.Contains(l, "internal/bench") && !strings.Contains(l, "runtime/") && i+1 < len(lines) && !strings.Contains(l, "panic(") {
				fr := strings.TrimSpace(l)
				if j := strings.LastIndex(fr, "("); j > 0 {
					fr = fr[:j]
				}
				return kind + ": " + trunc(first, 120) + " @ " + fr
			}
		}
	}
	if kind == "livelock" {
		// Keep only the cycle's event kinds and quoted texts, digits
		// stripped, so the same cycle on different seeds collapses.
		if i := strings.Index(first, "cycle: ["); i >= 0 {
			first = first[i:]
		}
		return kind + ": " + trunc(stripDigits(first), 200)
	}
	return kind + ": " + trunc(stripDigits(first), 160)
}

func stripDigits(s string) string {
	var b strings.Builder
	prev := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			if !prev {
				b.WriteByte('#')
			}
			prev = true
			continue
		}
		prev = false
		b.WriteRune(r)
	}
	return b.String()
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

type gameResult struct {
	idx      int
	fail     *failure
	included []string
	gc       *gameCov
}

// gameCov is one game's coverage: cards cast/played, cards with any ability
// pushed, and per card the inventory keys used (abilitiesUsed).
type gameCov struct {
	cast, ability map[string]bool
	used          map[string]map[string]bool
}

func botSeat(seed uint64) seat.Seat {
	s, err := host.NewBotPolicySeat(host.BotPolicy, seed)
	if err != nil {
		panic(err)
	}
	return s
}

// played walks the finished game for which deck cards were cast/land-played,
// which had an ability put on the stack, and which of their own abilities
// were used (abilitiesUsed).
func played(e *rules.Engine, decks [][]*cards.Card) *gameCov {
	gc := &gameCov{cast: map[string]bool{}, ability: map[string]bool{}}
	name := func(id state.ObjID) string { return realCardName(e, id) }
	for _, ev := range e.L.Events {
		switch ev.Kind {
		case events.PutOnStack:
			if ev.To == state.ZStack {
				if n := name(ev.Obj); n != "" {
					gc.cast[n] = true
				}
			}
		case events.LandPlayed:
			if n := name(ev.Obj); n != "" {
				gc.cast[n] = true
			}
		case events.AbilityPush, events.TriggerPush:
			if n := name(ev.Obj); n != "" {
				gc.ability[n] = true
			}
		}
	}
	gc.used = abilitiesUsed(e, decks)
	return gc
}

func playOne(reg *cards.Registry, decks []genDeck, seed uint64, maxTurns, maxIntents, maxObjects int, verify bool) (fail *failure, gc *gameCov) {
	mk := func(kind, diag string, o gbench.Outcome) *failure {
		return &failure{Kind: kind, Seed: seed, Decks: decks, Turns: o.Turns, Intents: o.Intents, Diag: diag, Sig: signature(kind, diag)}
	}
	var dk [][]*cards.Card
	for _, d := range decks {
		cs, err := resolveDeck(reg, d)
		if err != nil {
			return mk("setup", err.Error(), gbench.Outcome{}), nil
		}
		dk = append(dk, cs)
	}
	names := make([]string, len(decks))
	seats := make([]seat.Seat, len(decks))
	for i := range decks {
		names[i] = fmt.Sprintf("%s-%d", decks[i].Colour, i)
		seats[i] = botSeat(seed ^ (0x9e3779b97f4a7c15 * uint64(i+1)))
	}
	cfg := rules.Config{Names: names, Decks: dk, Tokens: reg.Tokens, Seed: seed, NameUniverse: reg.Cards}
	var o gbench.Outcome
	var e *rules.Engine
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic outside drive loop: %v", r)
			}
		}()
		o, e, err = gbench.PlayGame(cfg, seats, maxTurns, maxIntents, gbench.Hooks{Guard: boardGuard(maxObjects)})
	}()
	if err != nil {
		return mk("error", err.Error(), o), nil
	}
	gc = played(e, dk)
	withCtx := func(kind, diag string) *failure {
		ctx, involved := tailContext(e, 24)
		f := mk(kind, diag+"\n-- last events --\n"+ctx, o)
		if kind != "panic" && len(involved) > 0 {
			f.Sig += " {" + strings.Join(involved, ", ") + "}"
		}
		return f
	}
	switch {
	case gbench.IsAbort(o.StallOn):
		return withCtx(o.StallOn, o.Livelock), gc
	case o.StallOn == "intents":
		return withCtx("intents", fmt.Sprintf("intent cap %d hit at turn %d", maxIntents, o.Turns)), gc
	case o.StallOn == "bigboard":
		// Not an engine bug: the game grew a board past the harness's
		// budget. Its own kind lets triage separate it from hangs, and the
		// signature names the card with the most battlefield copies (the
		// usual token engine) rather than the log tail.
		return mk("bigboard", o.Livelock, o), gc
	}
	// The inert backstop (rules/priority_guard.go) keeps a game from spinning
	// on a priority option whose handler changed nothing, but the option was
	// still an offer/handler disagreement: report the game.
	for i, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.HasPrefix(ev.Text, rules.InertPriorityNotePrefix) {
			return mk("inert", fmt.Sprintf("%s (player %d, event %d)", ev.Text, ev.Player, i), o), gc
		}
	}
	if verify {
		var rerr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					rerr = fmt.Errorf("replay panic: %v", r)
				}
			}()
			_, rerr = replay.Replay(e.L, cfg)
		}()
		if rerr != nil {
			return mk("replay", rerr.Error(), o), gc
		}
	}
	return nil, gc
}

// boardGuard is the harness-side board-size watchdog: once the live
// (non-ceased) object count exceeds max, the game ends as a "bigboard"
// stall. The arena length bounds the live count from above, so a game that
// never grows past max never pays the scan. max <= 0 disables it.
func boardGuard(max int) func(*rules.Engine) (string, string) {
	if max <= 0 {
		return nil
	}
	return func(e *rules.Engine) (string, string) {
		if len(e.G.Objs) <= max {
			return "", ""
		}
		live := 0
		counts := map[string]int{}
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			if o.Zone == state.ZCeased {
				continue
			}
			live++
			if o.Zone == state.ZBattlefield && o.Card != nil {
				counts[cardName(o.Card)]++
			}
		}
		if live <= max {
			return "", ""
		}
		type nc struct {
			n string
			c int
		}
		var top []nc
		for n, c := range counts {
			top = append(top, nc{n, c})
		}
		sort.Slice(top, func(a, b int) bool { return top[a].c > top[b].c || (top[a].c == top[b].c && top[a].n < top[b].n) })
		var b strings.Builder
		fmt.Fprintf(&b, "live object count %d exceeds -max-objects %d at turn %d", live, max, e.G.Turn)
		if len(top) > 0 {
			fmt.Fprintf(&b, " (most on battlefield: %s)", top[0].n)
		}
		b.WriteString("\n-- battlefield --\n")
		for i, t := range top {
			if i == 10 {
				break
			}
			fmt.Fprintf(&b, "%6d %s\n", t.c, t.n)
		}
		return "bigboard", b.String()
	}
}

// tailContext renders the last n log events with object names, and returns
// the sorted distinct non-basic card names they reference (the likely
// culprits of a cycle).
func tailContext(e *rules.Engine, n int) (string, []string) {
	evs := e.L.Events
	if len(evs) > n {
		evs = evs[len(evs)-n:]
	}
	objName := func(id state.ObjID) string {
		o := e.G.Obj(id)
		if o == nil || o.Card == nil {
			return ""
		}
		nm := cardName(o.Card)
		if o.Source != 0 && o.Source != id {
			if src := e.G.Obj(o.Source); src != nil && src.Card != nil && cardName(src.Card) != nm {
				nm += "<-" + cardName(src.Card)
			}
		}
		return nm
	}
	seen := map[string]bool{}
	var b strings.Builder
	for _, ev := range evs {
		nm := ""
		if ev.Obj != 0 {
			nm = objName(ev.Obj)
			for _, part := range strings.Split(nm, "<-") {
				if part != "" && !isBasicName(part) {
					seen[part] = true
				}
			}
		}
		fmt.Fprintf(&b, "%d %s p=%d obj=%d(%s) %s->%s amt=%d %q\n", ev.Seq, ev.Kind, ev.Player, ev.Obj, nm, ev.From, ev.To, ev.Amount, ev.Text)
	}
	var names []string
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)
	return b.String(), names
}

func isBasicName(n string) bool {
	for _, b := range basicFor {
		if b == n {
			return true
		}
	}
	return false
}

var hang *time.Duration

// playWatched runs playOne on its own goroutine under a wall-clock budget.
// The budget is harness-only (the engine never sees the clock): a game that
// overruns is recorded as a "hang" carrying its goroutine's stack, and the
// goroutine is abandoned since Go cannot kill it.
func playWatched(reg *cards.Registry, decks []genDeck, seed uint64, maxTurns, maxIntents, maxObjects int, verify bool, budget time.Duration) (*failure, *gameCov, bool) {
	type res struct {
		f  *failure
		gc *gameCov
	}
	done := make(chan res, 1)
	gid := make(chan string, 1)
	go func() {
		buf := make([]byte, 64)
		buf = buf[:runtime.Stack(buf, false)]
		fields := strings.Fields(string(buf))
		if len(fields) > 1 {
			gid <- fields[1]
		} else {
			gid <- ""
		}
		f, gc := playOne(reg, decks, seed, maxTurns, maxIntents, maxObjects, verify)
		done <- res{f, gc}
	}()
	id := <-gid
	t := time.NewTimer(budget)
	defer t.Stop()
	select {
	case r := <-done:
		return r.f, r.gc, false
	case <-t.C:
	}
	buf := make([]byte, 64<<20)
	buf = buf[:runtime.Stack(buf, true)]
	stack := ""
	for _, g := range strings.Split(string(buf), "\n\n") {
		if strings.HasPrefix(g, "goroutine "+id+" ") {
			stack = g
			break
		}
	}
	sig := "hang"
	for _, l := range strings.Split(stack, "\n") {
		if strings.HasPrefix(l, "github.com/adams-shaun/gorge/") {
			fr := l
			if j := strings.LastIndex(fr, "("); j > 0 {
				fr = fr[:j]
			}
			sig = "hang: @ " + fr
			break
		}
	}
	return &failure{Kind: "hang", Seed: seed, Decks: decks, Diag: fmt.Sprintf("game exceeded %s wall clock\n%s", budget, stack), Sig: sig}, nil, true
}

func main() {
	dir := flag.String("dir", ".cards", "corpus directory")
	games := flag.Int("games", 1000, "games to play this run")
	batch := flag.Int("batch", 400, "games per batch (coverage snapshot granularity)")
	seed := flag.Uint64("seed", 1, "base seed; game i of the run plays at hash(seed, games-so-far + i)")
	workers := flag.Int("workers", 8, "parallel games")
	statePath := flag.String("state", "cardfuzz-state.json", "persistent coverage state")
	failPath := flag.String("failures", "cardfuzz-failures.jsonl", "append failure records here")
	maxTurns := flag.Int("max-turns", 100, "turn cap (a stall, not a failure)")
	maxIntents := flag.Int("max-intents", 20000, "intent cap (recorded as an 'intents' failure)")
	maxObjects := flag.Int("max-objects", 20000, "live object cap (recorded as a 'bigboard' failure; 0 disables)")
	verify := flag.Bool("verify", true, "replay every finished game and compare")
	repro := flag.String("repro", "", "replay a failure record from this JSONL file (with -line)")
	line := flag.Int("line", 1, "1-based line of -repro to replay")
	report := flag.Bool("report", false, "print coverage summary from -state and exit")
	missing := flag.Bool("missing", false, "with -report: also list cards cast/played but with abilities never used, and which")
	hang = flag.Duration("hang", 90*time.Second, "wall-clock budget per game before it is recorded as a 'hang' (its goroutine is abandoned)")
	maxHangs := flag.Int("max-hangs", 6, "stop the run once this many hung games are leaked (each burns a core)")
	cpuProfile := flag.String("cpuprofile", "", "with -repro: write a CPU profile of the replay here")
	flag.Parse()

	reg, err := cards.OpenCorpus(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cardfuzz:", err)
		os.Exit(1)
	}
	p, err := buildPool(reg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cardfuzz:", err)
		os.Exit(1)
	}
	if *repro != "" {
		if *cpuProfile != "" {
			pf, err := os.Create(*cpuProfile)
			if err != nil {
				fmt.Fprintln(os.Stderr, "cardfuzz:", err)
				os.Exit(1)
			}
			if err := pprof.StartCPUProfile(pf); err != nil {
				fmt.Fprintln(os.Stderr, "cardfuzz:", err)
				os.Exit(1)
			}
			code := runRepro(reg, *repro, *line, *maxTurns, *maxIntents, *maxObjects)
			pprof.StopCPUProfile()
			pf.Close()
			os.Exit(code)
		}
		os.Exit(runRepro(reg, *repro, *line, *maxTurns, *maxIntents, *maxObjects))
	}
	c, err := loadCov(*statePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cardfuzz:", err)
		os.Exit(1)
	}
	if *report {
		printReport(p, c, true)
		if *missing {
			printMissing(p, c)
		}
		return
	}
	ff, err := os.OpenFile(*failPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cardfuzz:", err)
		os.Exit(1)
	}
	defer ff.Close()
	fw := bufio.NewWriter(ff)

	var stop atomic.Bool
	var hangs atomic.Int64
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() { <-sig; stop.Store(true) }()

	start := time.Now()
	runFails := map[string]int{}
	played := 0
	for played < *games && !stop.Load() {
		n := min(*batch, *games-played)
		// Generate the whole batch against one coverage snapshot.
		type job struct {
			idx   int
			seed  uint64
			decks []genDeck
		}
		jobs := make([]job, n)
		for i := 0; i < n; i++ {
			gi := uint64(c.Games) + uint64(i)
			gs := mix(*seed, gi)
			r := rand.New(rand.NewPCG(gs, gs^0xdeadbeefcafef00d))
			jobs[i] = job{idx: i, seed: gs, decks: []genDeck{generate(r, p, c), generate(r, p, c)}}
		}
		results := make([]gameResult, n)
		var wg sync.WaitGroup
		ch := make(chan job)
		for w := 0; w < *workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range ch {
					if stop.Load() {
						results[j.idx] = gameResult{idx: -1}
						continue
					}
					f, gc, hung := playWatched(reg, j.decks, j.seed, *maxTurns, *maxIntents, *maxObjects, *verify, *hang)
					if hung {
						if hangs.Add(1) > int64(*maxHangs) {
							fmt.Fprintln(os.Stderr, "cardfuzz: too many leaked hung games; stopping")
							stop.Store(true)
						}
					}
					gr := gameResult{idx: j.idx, fail: f, gc: gc}
					for _, d := range j.decks {
						gr.included = append(gr.included, d.Cards...)
					}
					results[j.idx] = gr
				}
			}()
		}
		for _, j := range jobs {
			ch <- j
		}
		close(ch)
		wg.Wait()
		for _, gr := range results {
			if gr.idx < 0 {
				continue
			}
			c.Games++
			played++
			seen := map[string]bool{}
			for _, nme := range gr.included {
				if !seen[nme] {
					seen[nme] = true
					c.Included[nme]++
				}
			}
			if gr.gc != nil {
				for nme := range gr.gc.cast {
					c.Cast[nme]++
				}
				for nme := range gr.gc.ability {
					c.Ability[nme]++
				}
				for nme, keys := range gr.gc.used {
					u := c.Used[nme]
					if u == nil {
						u = map[string]int64{}
						c.Used[nme] = u
					}
					for k := range keys {
						u[k]++
					}
				}
			}
			if gr.fail != nil {
				runFails[gr.fail.Sig]++
				for nme := range seen {
					if !p.isBasic(nme) {
						c.Fails[nme]++
					}
				}
				b, _ := json.Marshal(gr.fail)
				fw.Write(b)
				fw.WriteByte('\n')
			}
		}
		fw.Flush()
		if err := c.save(*statePath); err != nil {
			fmt.Fprintln(os.Stderr, "cardfuzz: save:", err)
		}
		nf := 0
		for _, v := range runFails {
			nf += v
		}
		fmt.Fprintf(os.Stderr, "cardfuzz: %d/%d games, %d failures (%d sigs), %.1f games/s\n", played, *games, nf, len(runFails), float64(played)/time.Since(start).Seconds())
		printReport(p, c, false)
	}
	fmt.Println("== failure signatures this run ==")
	keys := make([]string, 0, len(runFails))
	for k := range runFails {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool {
		return runFails[keys[a]] > runFails[keys[b]] || (runFails[keys[a]] == runFails[keys[b]] && keys[a] < keys[b])
	})
	for _, k := range keys {
		fmt.Printf("%6d  %s\n", runFails[k], k)
	}
}

func (p *pool) isBasic(n string) bool {
	for _, b := range basicFor {
		if b == n {
			return true
		}
	}
	return false
}

func printReport(p *pool, c *cov, detail bool) {
	total, inc, cast, abil, full := 0, 0, 0, 0, 0
	for n := range p.all {
		total++
		if c.full(n, p.keys[n]) {
			full++
		}
		if c.Included[n] > 0 {
			inc++
		}
		if c.Cast[n] > 0 {
			cast++
		}
		if c.Cast[n] > 0 || c.Ability[n] > 0 {
			abil++
		}
	}
	fmt.Fprintf(os.Stderr, "cardfuzz: pool %d cards | included %d (%.1f%%) | cast/played %d (%.1f%%) | cast-or-ability %d (%.1f%%) | full %d (%.1f%%) | games %d\n",
		total, inc, pct(inc, total), cast, pct(cast, total), abil, pct(abil, total), full, pct(full, total), c.Games)
	if !detail {
		return
	}
	var never []string
	for n := range p.all {
		if c.Cast[n] == 0 && c.Ability[n] == 0 && c.Included[n] > 0 {
			never = append(never, fmt.Sprintf("%d\t%s", c.Included[n], n))
		}
	}
	sort.Strings(never)
	fmt.Printf("# included but never cast/activated: %d\n", len(never))
	for _, l := range never {
		fmt.Println(l)
	}
}

// printMissing lists every pool card cast/played at least once but with some
// ability in its inventory never used: "<times cast>\t<name>\t<key(desc)> ...".
// Mana abilities are detected by proxy (see abilitiesUsed).
func printMissing(p *pool, c *cov) {
	var lines []string
	for n := range p.all {
		if c.Cast[n] == 0 {
			continue
		}
		cd := p.cards[n]
		miss := c.missing(n, p.keys[n])
		if len(miss) == 0 {
			continue
		}
		desc := abilityDescs(cd)
		parts := make([]string, len(miss))
		for i, k := range miss {
			parts[i] = k + "(" + desc[k] + ")"
		}
		lines = append(lines, fmt.Sprintf("%d\t%s\t%s", c.Cast[n], n, strings.Join(parts, " ")))
	}
	sort.Strings(lines)
	fmt.Printf("# cast/played but with abilities never used: %d\n", len(lines))
	for _, l := range lines {
		fmt.Println(l)
	}
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

func mix(a, b uint64) uint64 {
	x := a*0x9e3779b97f4a7c15 ^ b
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}

func runRepro(reg *cards.Registry, path string, line, maxTurns, maxIntents, maxObjects int) int {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for i := 1; sc.Scan(); i++ {
		if i != line {
			continue
		}
		var rec failure
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fl, _ := playOne(reg, rec.Decks, rec.Seed, maxTurns, maxIntents, maxObjects, true)
		if fl == nil {
			fmt.Println("REPRO: game completed cleanly (not reproduced)")
			return 0
		}
		fmt.Printf("REPRO %s seed=%d turns=%d intents=%d\n%s\n", fl.Kind, fl.Seed, fl.Turns, fl.Intents, fl.Diag)
		return 2
	}
	fmt.Fprintln(os.Stderr, "line not found")
	return 1
}
