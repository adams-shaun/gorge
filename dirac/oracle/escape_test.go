package oracle

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Escape lockstep (CR 702.138a): constructed decks over the generated
// escape pool, so every graveyard escape cast gorge offers (the card sits in
// its owner's graveyard, the floating pool pays the printed cost and the cost's
// ExileFromGrave<N/Card.Other> part has the exiles) is answered by the kernel's
// mode-2 cast, the printed exiles are picked identically, and an escape cast's
// entering permanent carries the escaped mark both engines read (the
// K:etbCounter riders and the Elder Giants' ConditionNotPresent$ escaped
// sacrifice rider).

var escapeDecks = [2][]cardset.Entry{
	{ // A: five-colour, the graveyard-side escapes and the BBB/RRR floats
		{Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 3},
		{Name: "Island", Count: 2}, {Name: "Plains", Count: 1},
		{Name: "Crumbling Vestige", Count: 3},
		{Name: "Priest of Gix", Count: 3}, {Name: "Coal Stoker", Count: 2},
		{Name: "Lotus Cobra", Count: 2},
		{Name: "Tormenting Voice", Count: 2}, {Name: "Thrill of Possibility", Count: 1},
		{Name: "Satyr's Cunning", Count: 2}, {Name: "Escape Velocity", Count: 1},
		{Name: "Mogis's Favor", Count: 1}, {Name: "Fruit of Tizerus", Count: 1},
		{Name: "Phoenix of Ash", Count: 1}, {Name: "Underworld Rage-Hound", Count: 1},
		{Name: "Bloodbraid Challenger", Count: 1}, {Name: "Woe Strider", Count: 1},
		{Name: "Charred Graverobber", Count: 1}, {Name: "Sleep of the Dead", Count: 1},
		{Name: "Sentinel's Eyes", Count: 1}, {Name: "Ox of Agonas", Count: 1},
	},
	{ // B: G/U/W heavy, the ETB escaped-counter riders, the Elder Giants and
		// the RG/any-colour floats
		{Name: "Forest", Count: 5}, {Name: "Mountain", Count: 3}, {Name: "Swamp", Count: 3},
		{Name: "Island", Count: 2}, {Name: "Plains", Count: 2},
		{Name: "Crumbling Vestige", Count: 2},
		{Name: "Burning-Tree Emissary", Count: 3}, {Name: "Hidden Herbalists", Count: 2},
		{Name: "Lotus Cobra", Count: 3}, {Name: "Flamecache Gecko", Count: 1},
		{Name: "Mulch", Count: 1}, {Name: "Satyr Wayfinder", Count: 2}, {Name: "Tormenting Voice", Count: 1},
		{Name: "Bloodbraid Challenger", Count: 1}, {Name: "Woe Strider", Count: 1},
		{Name: "Chainweb Aracnir", Count: 1}, {Name: "Loathsome Chimera", Count: 1},
		{Name: "Uro, Titan of Nature's Wrath", Count: 1},
		{Name: "Phlage, Titan of Fire's Fury", Count: 1},
		{Name: "Glimpse of Freedom", Count: 1}, {Name: "Run for Your Life", Count: 1},
		{Name: "Underworld Charger", Count: 1}, {Name: "Sweet Oblivion", Count: 1},
	},
}

// escapeNames are the pool's escape carriers, the cards the lockstep counts
// offers and casts for.
var escapeNames = map[string]bool{
	"Satyr's Cunning": true, "Escape Velocity": true, "Mogis's Favor": true, "Fruit of Tizerus": true,
	"Phoenix of Ash": true, "Underworld Rage-Hound": true, "Bloodbraid Challenger": true, "Woe Strider": true,
	"Charred Graverobber": true, "Sleep of the Dead": true, "Elspeth, Sun's Nemesis": true, "Sentinel's Eyes": true,
	"Unbreakable Remnant": true, "Ox of Agonas": true, "Chainweb Aracnir": true, "Loathsome Chimera": true,
	// Unbreakable Remnant is refused (its SpellCast trigger reads
	// Card.Self+wasCastFromYourGraveyardByYou, a cast-provenance word outside
	// a SpellCast ValidCard$), so it is in no deck.
	"Voracious Typhon": true, "Uro, Titan of Nature's Wrath": true, "Phlage, Titan of Fire's Fury": true,
	"Glimpse of Freedom": true, "Run for Your Life": true, "Underworld Charger": true, "Sweet Oblivion": true,
	"Cling to Dust": true,
}

// escapeCostColours are each carrier's escape cost's coloured pips: the colour
// choices (Crumbling Vestige, Lotus Cobra) steer into them.
var escapeCostColours = map[string]string{
	"Satyr's Cunning": "R", "Escape Velocity": "R", "Mogis's Favor": "B", "Fruit of Tizerus": "B",
	"Phoenix of Ash": "RR", "Underworld Rage-Hound": "R", "Bloodbraid Challenger": "RG", "Woe Strider": "BB",
	"Charred Graverobber": "B", "Sleep of the Dead": "U", "Elspeth, Sun's Nemesis": "W", "Sentinel's Eyes": "W",
	"Unbreakable Remnant": "W", "Ox of Agonas": "RR", "Chainweb Aracnir": "G", "Loathsome Chimera": "G",
	"Voracious Typhon": "G", "Uro, Titan of Nature's Wrath": "GU", "Phlage, Titan of Fire's Fury": "RW",
	"Glimpse of Freedom": "U", "Run for Your Life": "UR", "Underworld Charger": "B", "Sweet Oblivion": "U",
	"Cling to Dust": "B",
}

// escapeCostMana is each carrier's escape cost's total mana (generic plus
// pips): the probe prefers the heaviest offered cast, the floats are the
// scarce input.
var escapeCostMana = map[string]int{
	"Satyr's Cunning": 3, "Escape Velocity": 2, "Mogis's Favor": 3, "Fruit of Tizerus": 4,
	"Phoenix of Ash": 4, "Underworld Rage-Hound": 4, "Bloodbraid Challenger": 5, "Woe Strider": 5,
	"Charred Graverobber": 4, "Sleep of the Dead": 3, "Elspeth, Sun's Nemesis": 6, "Sentinel's Eyes": 1,
	"Unbreakable Remnant": 0, "Ox of Agonas": 2, "Chainweb Aracnir": 5, "Loathsome Chimera": 5,
	"Voracious Typhon": 7, "Uro, Titan of Nature's Wrath": 4, "Phlage, Titan of Fire's Fury": 4,
	"Glimpse of Freedom": 3, "Run for Your Life": 4, "Underworld Charger": 5, "Sweet Oblivion": 4,
	"Cling to Dust": 4,
}

// escapeFloaters are the pool's cards whose entry or attack floats mana; the
// escape option exists only while the floating pool pays the cost (gorge's
// planner prices no escape mode), so the probe casts them first.
var escapeFloaters = map[string]bool{
	"Priest of Gix": true, "Akki Rockspeaker": true, "Coal Stoker": true, "Crumbling Vestige": true,
	"Burning-Tree Emissary": true, "Hidden Herbalists": true, "Lotus Cobra": true, "Flamecache Gecko": true,
}

// escapeOutlets are the pool's spells that move cards into a graveyard.
var escapeOutlets = map[string]bool{
	"Tormenting Voice": true, "Thrill of Possibility": true, "Mulch": true, "Satyr Wayfinder": true,
}

type escapeStats struct {
	offers map[string]int // escape offers per carrier (a decision that lists one)
	casts  map[string]int // escape casts taken per carrier
	games  int
}

// escapeGraveColours scores the offered colour options by the escape carriers
// sitting in the decider's graveyard whose cost needs the colour.
func escapeGraveColours(g *GorgeSide, pl uint8) map[rune]int {
	need := map[rune]int{}
	e := g.E
	for _, id := range e.G.Zone(state.ZGraveyard, state.PlayerID(pl)) {
		ob := e.G.Obj(id)
		if ob == nil || ob.Face() == nil || !escapeNames[ob.Face().Name] {
			continue
		}
		for _, c := range escapeCostColours[ob.Face().Name] {
			need[c]++
		}
	}
	return need
}

// escapeProbe wraps a seeded random policy biased to float mana, move escape
// carriers into the graveyard and cast them for their escape cost.
type escapeProbe struct {
	st   *escapeStats
	base Policy
	r    *rand.Rand
}

func (p *escapeProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	pl := c.D.Player
	var esc, floaters, outlets, disc, colours []Canon
	var symbols []rune
	for _, x := range c.D.Options {
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		name := ""
		if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
			name = ob.Face().Name
		}
		switch {
		case o.Kind == "cast" && o.Mode == "escape":
			esc = append(esc, x)
			p.st.offers[name]++
		case o.Kind == "cast" && escapeFloaters[name]:
			floaters = append(floaters, x)
		case o.Kind == "cast" && escapeOutlets[name]:
			outlets = append(outlets, x)
		case o.Kind == "discard" && escapeNames[name]:
			disc = append(disc, x)
		case o.Kind == "color" && len(o.ManaSymbol) == 1 && strings.ContainsRune("WUBRG", rune(o.ManaSymbol[0])):
			colours = append(colours, x)
			symbols = append(symbols, rune(o.ManaSymbol[0]))
		}
	}
	basePick := a
	// a heavy carrier ready in the grave holds floats: no outlets or discards,
	// only float building and the escape itself (a pass otherwise)
	heavyReady := false
	for _, id := range c.G.E.G.Zone(state.ZGraveyard, state.PlayerID(pl)) {
		ob := c.G.E.G.Obj(id)
		if ob == nil || ob.Face() == nil || !escapeNames[ob.Face().Name] {
			continue
		}
		if escapeCostMana[ob.Face().Name] >= 4 {
			heavyReady = true
		}
	}
	switch {
	case len(esc) > 0 && p.r.IntN(8) != 0:
		best, bestW := 0, -1
		for i, x := range esc {
			w := 0
			if o, ok := c.G.GorgeOption(x); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					w = escapeCostMana[ob.Face().Name]
				}
			}
			if w > bestW {
				best, bestW = i, w
			}
		}
		a = esc[best]
		if o, ok := c.G.GorgeOption(a); ok {
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
				p.st.casts[ob.Face().Name]++
			}
		}
	case len(colours) > 0:
		// steer the any-colour floats into the graveyard carriers' costs
		need := escapeGraveColours(c.G, pl)
		best, bestScore := 0, -1
		for i, s := range symbols {
			if score := need[s]; score > bestScore {
				best, bestScore = i, score
			}
		}
		a = colours[best]
	case len(floaters) > 0:
		a = floaters[p.r.IntN(len(floaters))] // float mana for the cast
	case len(disc) > 0 && !heavyReady:
		a = disc[p.r.IntN(len(disc))] // move an escape carrier into the graveyard
	case len(outlets) > 0 && !heavyReady:
		a = outlets[p.r.IntN(len(outlets))]
	}
	if a == basePick && heavyReady {
		// the floats must survive until the escape is payable: a land drop
		// (Crumbling Vestige, a Cobra trigger) or a pass
		for _, x := range c.D.Options {
			if o, ok := c.G.GorgeOption(x); ok && (o.Kind == "play_land" || o.Kind == "pass") {
				return x
			}
		}
	}
	return a
}

// escapeSetup builds the matchup's corpus decks and table.
func escapeSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("escape")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range escapeDecks {
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
	tbl, _, _, err := set.Matchup(escapeDecks[0], escapeDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// escapeRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies. report logs the first
// divergence as a test error (the mutation run expects one and stays quiet).
func escapeRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st escapeStats) {
	t.Helper()
	st.offers, st.casts = map[string]int{}, map[string]int{}
	reg, set, tbl, decks := escapeSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		st.games++
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"escape-a", "escape-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &escapeProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x65736370))}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestEscapeLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := escapeRun(t, 384, true)
	t.Logf("%d games, escape offers %v, casts %v, %d divergences, %d stalls", st.games, st.offers, st.casts, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if div {
		return // the divergence is already reported
	}
	offers, casts := 0, 0
	for n := range st.offers {
		offers += st.offers[n]
	}
	for n := range st.casts {
		casts += st.casts[n]
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{offers, 30, "escape offers"},
		{casts, 20, "escape casts"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestEscapeMutation: an offer that ignores the printed cost's exile part
// must make the lockstep diverge (a graveyard without the exiles is offered).
func TestEscapeMutation(t *testing.T) {
	dirac.DebugEscapeSkipExiles = true
	defer func() { dirac.DebugEscapeSkipExiles = false }()
	if diverged, _, _ := escapeRun(t, 32, false); !diverged {
		t.Fatal("DebugEscapeSkipExiles: no divergence in 32 seeds")
	}
}

// escapeCase is one directed escape game: the carrier whose escape cast must
// resolve, the any-colour floats' pips to cycle, and the recorder's tallies.
type escapeCase struct {
	effectLongCase
	carrier string // the card whose escape cast must resolve
	colour  rune   // the any-colour floats' first pip
	pips    string // the escape cost's coloured pips, cycled
	// record diagnostics (the record loop reads them)
	graved  int // decisions with the carrier in a graveyard
	offered int // escape casts of the carrier ever offered
}

// escapeReproCases are the directed escape games: one per lowered carrier,
// each a construction over the Lotus Cobra landfall engine, the ramp spells
// that trigger it, and Thrill of Possibility to discard a drawn carrier. The
// recorder (ESCAPE_RECORD=1) searches seeds until the carrier's escape cast
// resolves, and TestEscapeReprosReach replays every one; TestEscapeReprosOutcome
// checks the escaped entry's counters and its ConditionNotPresent$ rider.
func escapeReproCases() []escapeCase {
	out := make([]escapeCase, 0, len(escapeCarrierCases))
	for _, c := range escapeCarrierCases {
		out = append(out, escapeCarrierReproCase(c))
	}
	return out
}

// escapeCarrierReproCase is one carrier's directed game: the carrier, four
// Lotus Cobras, four land-ramp spells (each Cobra adds one mana of the chosen
// colour per land the spell puts onto the battlefield), four discards and
// enough green sources to start the engine.
func escapeCarrierReproCase(c escapeCarrierCase) escapeCase {
	prefer := []string{"Thrill of Possibility", "Lotus Cobra", "Rampant Growth", "Nature's Lore", "Three Visits", "Cultivate"}
	decks := [2][]string{
		slices.Concat(
			deckOf(2, c.name),
			deckOf(4, "Lotus Cobra"),
			deckOf(3, "Rampant Growth"),
			deckOf(3, "Nature's Lore"),
			deckOf(3, "Three Visits"),
			deckOf(3, "Cultivate"),
			deckOf(4, "Thrill of Possibility"),
			deckOf(12, "Forest"),
			deckOf(4, "Mountain"),
			deckOf(2, "Swamp"),
		),
		deckOf(40, "Plains"),
	}
	return escapeCase{
		effectLongCase: effectLongCase{
			name:    escapeReproName(c.name),
			prefer:  prefer,
			decks:   decks,
			surface: "full",
			reached: escapeCastTaken(c.name),
		},
		carrier: c.name,
		colour:  rune(c.pips[0]),
		pips:    c.pips,
	}
}

// escapeReproName is the case's file-name stem: the printed name's letters
// and digits, lowercased, with runs of other characters as one underscore.
func escapeReproName(name string) string {
	var b strings.Builder
	underscore := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			underscore = false
			continue
		}
		if !underscore && b.Len() > 0 {
			b.WriteByte('_')
			underscore = true
		}
	}
	return strings.TrimSuffix(b.String(), "_") + "_escape_cast"
}

// escapeCastTaken reads the gorge log for the carrier's cast out of its
// graveyard: the escape cast itself. A permanent's entry (and its counters or
// sacrifice rider) is checked by TestEscapeReprosOutcome.
func escapeCastTaken(carrier string) func(*GorgeSide) bool {
	return func(g *GorgeSide) bool {
		name := func(id state.ObjID) string {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
				return o.Face().Name
			}
			return ""
		}
		for _, e := range g.E.L.Events {
			if e.Kind == events.PutOnStack && e.From == state.ZGraveyard && name(e.Obj) == carrier {
				return true
			}
		}
		return false
	}
}

// escapeObjName is o's card's printed name.
func escapeObjName(g *GorgeSide, id state.ObjID) string {
	if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
		return ob.Face().Name
	}
	return ""
}

// escapeReproPolicy is effectLongPolicy's play with the carrier's escape cast
// taken whenever offered, its plain casts dropped from the menu (the carrier
// must reach the graveyard, and a battlefield copy does not), and the
// any-colour floats steered into the case's pip.
func escapeReproPolicy(m *Matchup, c *escapeCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c.effectLongCase, seed)
	pip := 0
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority && p.G != nil {
			zn := p.G.E.G.Zone(state.ZGraveyard, state.PlayerID(p.D.Player))
			for _, id := range zn {
				if escapeObjName(p.G, id) == c.carrier {
					c.graved++
					break
				}
			}
		}
		if p.G != nil {
			// move the carrier into its graveyard at the first chance
			for _, x := range p.D.Options {
				if o, ok := p.G.GorgeOption(x); ok && o.Kind == "discard" &&
					escapeObjName(p.G, o.Obj) == c.carrier {
					return x
				}
			}
		}
		if p.D.Kind == DPriority {
			var esc Canon
			escFound := false
			d := *p.D
			d.Options = nil
			for _, x := range p.D.Options {
				o, ok := p.G.GorgeOption(x)
				if ok && o.Kind == "cast" && o.Mode == "escape" &&
					escapeObjName(p.G, o.Obj) == c.carrier {
					esc, escFound = x, true
					continue
				}
				if ok && o.Kind == "cast" && escapeObjName(p.G, o.Obj) == c.carrier {
					continue // the plain cast keeps the carrier out of the grave
				}
				d.Options = append(d.Options, x)
			}
			if escFound {
				c.offered++
				return esc
			}
			a := inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
			// steer the any-colour floats into the case's next pip
			want := c.colour
			if c.pips != "" {
				want = rune(c.pips[pip%len(c.pips)])
				pip++
			}
			if o, ok := p.G.GorgeOption(a); ok && o.Kind == "color" && len(o.ManaSymbol) == 1 {
				for _, x := range d.Options {
					if oo, ok := p.G.GorgeOption(x); ok && oo.Kind == "color" && len(oo.ManaSymbol) == 1 &&
						rune(oo.ManaSymbol[0]) == want {
						return x
					}
				}
			}
			return a
		}
		return inner.Pick(p)
	})
}

// TestEscapeRecord plays the cases' policies over seeds until the carrier's
// escape cast resolves, and writes the repro. It runs only with
// ESCAPE_RECORD=1; the games it writes are the committed repros.
func TestEscapeRecord(t *testing.T) {
	if os.Getenv("ESCAPE_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range escapeReproCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, escapeReproPolicy(m, &c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached {
					t.Logf("seed %d: %d decisions, the carrier graved at %d, escape offered at %d", seed, len(res.Actions), c.graved, c.offered)
					c.graved, c.offered = 0, 0
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s's escape cast", c.carrier)
		})
	}
}

// TestEscapeReprosReach replays the recorded games: no divergence, every
// scripted action offered, the escape cast resolved.
func TestEscapeReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range escapeReproCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c.effectLongCase, r, printedDelveSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v %s\nref %v\nker %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches %s's escape cast", c.carrier)
			}
		})
	}
}
