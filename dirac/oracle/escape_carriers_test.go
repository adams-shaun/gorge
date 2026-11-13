package oracle

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The directed Escape carrier coverage: every lowered Escape carrier has a
// recorded game in which its escape cast resolves, and every one of those
// games replays decision by decision against gorge. The case list is checked
// against the escape pool, so a newly lowered carrier without a directed game
// fails instead of shrinking the coverage silently.

// escapeCarrierCase is one lowered carrier: the non-green colour of its
// escape cost, the cost's coloured pips (cycled for the any-colour mana), the
// +1/+1 counters its escape entry puts on it, and whether its escaped entry
// must survive its own "sacrifice unless it escaped" trigger (the Elder
// Giants).
type escapeCarrierCase struct {
	name     string
	colours  []string // basics of the non-green escape colours
	pips     string   // the escape cost's coloured pips, cycled
	counters int      // P1P1 counters the escaped entry carries
	survives bool     // a ConditionNotPresent$ Card.Self+escaped sacrifice rider
}

// escapeCarrierCases is the whole lowered class (the 23 cards the kgen
// ratchet moves). TestEscapeCarrierSetCovered checks it against the escape
// pool.
var escapeCarrierCases = []escapeCarrierCase{
	{name: "Satyr's Cunning", colours: []string{"Mountain"}, pips: "R"},
	{name: "Escape Velocity", colours: []string{"Mountain"}, pips: "R"},
	{name: "Mogis's Favor", colours: []string{"Swamp"}, pips: "B"},
	{name: "Fruit of Tizerus", colours: []string{"Swamp"}, pips: "B"},
	{name: "Phoenix of Ash", colours: []string{"Mountain"}, pips: "RR", counters: 1},
	{name: "Underworld Rage-Hound", colours: []string{"Mountain"}, pips: "R", counters: 1},
	{name: "Bloodbraid Challenger", colours: []string{"Mountain"}, pips: "RG"},
	{name: "Woe Strider", colours: []string{"Swamp"}, pips: "BB", counters: 2},
	{name: "Charred Graverobber", colours: []string{"Swamp"}, pips: "B", counters: 1},
	{name: "Sleep of the Dead", colours: []string{"Island"}, pips: "U"},
	{name: "Elspeth, Sun's Nemesis", colours: []string{"Plains"}, pips: "WW"},
	{name: "Sentinel's Eyes", colours: []string{"Plains"}, pips: "W"},
	{name: "Ox of Agonas", colours: []string{"Mountain"}, pips: "RR", counters: 1},
	{name: "Chainweb Aracnir", pips: "GG", counters: 3},
	{name: "Loathsome Chimera", pips: "G", counters: 1},
	{name: "Voracious Typhon", pips: "GG", counters: 3},
	{name: "Uro, Titan of Nature's Wrath", colours: []string{"Island"}, pips: "GGUU", survives: true},
	{name: "Phlage, Titan of Fire's Fury", colours: []string{"Mountain", "Plains"}, pips: "RRWW", survives: true},
	{name: "Glimpse of Freedom", colours: []string{"Island"}, pips: "U"},
	{name: "Run for Your Life", colours: []string{"Island", "Mountain"}, pips: "UR"},
	{name: "Underworld Charger", colours: []string{"Swamp"}, pips: "B", counters: 2},
	{name: "Sweet Oblivion", colours: []string{"Island"}, pips: "U"},
	{name: "Cling to Dust", colours: []string{"Swamp"}, pips: "B"},
}

// escapePool loads the escape fixture's set and corpus.
func escapePool(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("escape")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// TestEscapeCarrierSetCovered checks the directed cases against the escape
// pool: every lowered Escape card the pool holds must have a case, so a
// lowering added without a game here fails instead of shrinking silently.
func TestEscapeCarrierSetCovered(t *testing.T) {
	if isolate(t) {
		return
	}
	_, set := escapePool(t)
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	var have []string
	for i := range set.Supported() {
		if !set.Supported()[i] {
			continue
		}
		if cd := &tbl.Cards[set.PoolID(i)]; cd.Escape {
			have = append(have, cd.Name)
		}
	}
	slices.Sort(have)
	want := make([]string, len(escapeCarrierCases))
	for i, c := range escapeCarrierCases {
		want[i] = c.name
	}
	slices.Sort(want)
	if !slices.Equal(have, want) {
		t.Fatalf("escape pool carriers %v, directed cases %v", have, want)
	}
}

// escapeCastOutcome is what a finished game shows for one carrier: whether
// its escape cast happened, the +1/+1 counters it entered with, and whether
// it was sacrificed by its own entry trigger.
type escapeCastOutcome struct {
	cast       bool
	entered    bool
	onBattle   bool
	sacrificed bool
	counters   int
}

func readEscapeCastOutcome(g *GorgeSide, name string) escapeCastOutcome {
	var out escapeCastOutcome
	var id state.ObjID
	for _, e := range g.E.L.Events {
		if e.Kind == events.PutOnStack && e.From == state.ZGraveyard && escapeObjName(g, e.Obj) == name {
			out.cast, id = true, e.Obj
			continue
		}
		if id == 0 || e.Obj != id {
			continue
		}
		switch {
		case e.Kind == events.CounterChange:
			out.counters += int(e.Amount)
		case e.Kind == events.MoveZone && e.From == state.ZStack && e.To == state.ZBattlefield:
			out.entered = true
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield && events.IsSacrifice(e):
			out.sacrificed = true
		}
	}
	if id != 0 {
		for _, slot := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(0)) {
			if slot == id {
				out.onBattle = true
			}
		}
	}
	return out
}

// escapeReproOutcomePlay replays a recorded game and returns its result, the
// case's reached predicate and the carrier's escape outcome.
func escapeReproOutcomePlay(t *testing.T, c escapeCase, r *Repro) (Result, bool, escapeCastOutcome) {
	t.Helper()
	reg := synthReg(t)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	set := printedDelveSet(t, r.Cards)
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	var reached bool
	var out escapeCastOutcome
	rc.Finish = func(g *GorgeSide) {
		reached = c.reached(g)
		out = readEscapeCastOutcome(g, c.carrier)
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc), reached, out
}

// TestEscapeReprosOutcome replays every carrier's recorded game and checks the
// escaped entry's observable result: the printed +1/+1 counters, the Elder
// Giants' "sacrifice unless it escaped" rider held off, and the permanent
// still on the battlefield.
func TestEscapeReprosOutcome(t *testing.T) {
	byName := make(map[string]escapeCarrierCase, len(escapeCarrierCases))
	for _, cc := range escapeCarrierCases {
		byName[cc.name] = cc
	}
	for _, c := range escapeReproCases() {
		cc := byName[c.carrier]
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			res, reached, out := escapeReproOutcomePlay(t, c, r)
			if d := res.Div; d != nil {
				t.Fatalf("divergence at %d: %s %s %v %s", d.At, d.What, d.Field, d.Diff, d.Detail)
			}
			if !reached {
				t.Fatalf("the recorded game never casts %s", c.carrier)
			}
			if !out.cast {
				t.Fatalf("the recorded game never casts %s's escape", c.carrier)
			}
			if (cc.counters > 0 || cc.survives) && !out.entered {
				t.Fatalf("%s's escape cast never entered the battlefield", c.carrier)
			}
			if out.entered {
				if out.counters != cc.counters {
					t.Errorf("%s entered with %d counters, want %d", c.carrier, out.counters, cc.counters)
				}
				if cc.survives && out.sacrificed {
					t.Errorf("%s was sacrificed by its own entered trigger", c.carrier)
				}
				if cc.survives && !out.onBattle {
					t.Errorf("%s is not on the battlefield at the end", c.carrier)
				}
			}
		})
	}
}

// escapeGrantPolicy is the directed policy for the flashback-grant game: take
// the carrier's escape (or granted flashback) when offered, discard a drawn
// carrier, build the Lotus Cobra landfall engine, cast and attack with Sphinx
// of Forgotten Lore, and target the graveyard carrier with its grant.
func escapeGrantPolicy(seed uint64, carrier, attack string, prefer int) Policy {
	base := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G != nil && p.D.Kind == DChoose {
			for _, x := range p.D.Options {
				if o, ok := p.G.GorgeOption(x); ok && (o.Kind == "color" || o.Kind == "mana") && len(o.ManaSymbol) == 1 {
					if o.ManaSymbol == "B" {
						return x
					}
				}
			}
		}
		if p.G != nil {
			var esc, flash Canon
			haveEsc, haveFlash := false, false
			for _, x := range p.D.Options {
				o, ok := p.G.GorgeOption(x)
				if !ok || o.Kind != "cast" || escapeObjName(p.G, o.Obj) != carrier {
					continue
				}
				switch o.Mode {
				case "escape":
					esc, haveEsc = x, true
				case "flashback":
					flash, haveFlash = x, true
				}
			}
			if haveEsc && haveFlash {
				if prefer == 2 {
					return flash
				}
				return esc
			}
			if haveFlash && prefer == 2 {
				return flash
			}
			if haveEsc {
				// wait for the pool to pay the granted flashback too, so the
				// two casts are offered at the same decision
				for _, x := range p.D.Options {
					if x.Kind() == ActPass {
						return x
					}
				}
			}
			for _, x := range p.D.Options {
				if o, ok := p.G.GorgeOption(x); ok && o.Kind == "discard" && escapeObjName(p.G, o.Obj) == carrier {
					return x
				}
			}
		}
		if p.D.Kind == DAttackers {
			for _, x := range p.D.Options {
				if x.Kind() == ActAttack && p.G != nil && escapeObjName(p.G, state.ObjID(x.Src())) == attack {
					return x
				}
			}
			for _, x := range p.D.Options {
				if x.Kind() == ActDone {
					return x
				}
			}
		}
		if p.D.Kind == DTarget && p.G != nil {
			for _, x := range p.D.Options {
				if o, ok := p.G.GorgeOption(x); ok && escapeObjName(p.G, o.Obj) == carrier {
					return x
				}
			}
		}
		if p.D.Kind == DPriority && p.G != nil {
			for _, x := range p.D.Options {
				if o, ok := p.G.GorgeOption(x); ok && o.Kind == "play_land" {
					return x
				}
			}
			for _, want := range []string{attack, "Lotus Cobra"} {
				if want == "" {
					continue
				}
				for _, x := range p.D.Options {
					if o, ok := p.G.GorgeOption(x); ok && o.Kind == "cast" && escapeObjName(p.G, o.Obj) == want {
						return x
					}
				}
			}
			for _, want := range []string{"Rampant Growth", "Nature's Lore", "Three Visits", "Cultivate"} {
				for _, x := range p.D.Options {
					if o, ok := p.G.GorgeOption(x); ok && o.Kind == "cast" && escapeObjName(p.G, o.Obj) == want {
						return x
					}
				}
			}
			for _, want := range []string{"Thrill of Possibility"} {
				for _, x := range p.D.Options {
					if o, ok := p.G.GorgeOption(x); ok && o.Kind == "cast" && escapeObjName(p.G, o.Obj) == want {
						return x
					}
				}
			}
			for _, x := range p.D.Options {
				if x.Kind() == ActPass {
					return x
				}
			}
		}
		return base.Pick(p)
	})
}

// TestEscapeFlashbackGrantLockstep: Sphinx of Forgotten Lore grants an Escape
// instant flashback, so gorge offers both the granted flashback cast and the
// escape cast (CR 702.138a). The kernel marks the escape action with its
// target byte and prices the granted flashback as flashback; both casts are
// offered together and each is played.
func TestEscapeFlashbackGrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set := escapePool(t)
	deck := slices.Concat(
		deckOf(2, "Sphinx of Forgotten Lore"),
		deckOf(2, "Cling to Dust"),
		deckOf(4, "Lotus Cobra"),
		deckOf(3, "Rampant Growth"),
		deckOf(3, "Nature's Lore"),
		deckOf(3, "Three Visits"),
		deckOf(3, "Cultivate"),
		deckOf(4, "Thrill of Possibility"),
		deckOf(6, "Island"),
		deckOf(6, "Forest"),
		deckOf(4, "Mountain"),
	)
	if len(deck) != 40 {
		t.Fatalf("deck: %d cards", len(deck))
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(deck), []cardset.Entry{{Name: "Plains", Count: 40}})
	if err != nil {
		t.Fatal(err)
	}
	decks := [2][]*cards.Card{
		escapeCards(t, reg, deck),
		escapeCards(t, reg, deckOf(40, "Plains")),
	}
	var escOffers, flashOffers, both, escCasts, flashCasts int
	for seed := uint64(1); seed <= 128; seed++ {
		prefer := 1
		if seed%2 == 0 {
			prefer = 2
		}
		inner := escapeGrantPolicy(seed, "Cling to Dust", "Sphinx of Forgotten Lore", prefer)
		pol := PolicyFunc(func(p *PickCtx) Canon {
			if p.G != nil && p.D.Kind == DPriority {
				esc, flash := false, false
				for _, x := range p.D.Options {
					o, ok := p.G.GorgeOption(x)
					if !ok || o.Kind != "cast" || escapeObjName(p.G, o.Obj) != "Cling to Dust" {
						continue
					}
					switch o.Mode {
					case "escape":
						esc = true
					case "flashback":
						flash = true
					}
				}
				if esc {
					escOffers++
				}
				if flash {
					flashOffers++
				}
				if esc && flash {
					both++
				}
			}
			a := inner.Pick(p)
			if p.G != nil && p.D.Kind == DPriority {
				if o, ok := p.G.GorgeOption(a); ok && o.Kind == "cast" && escapeObjName(p.G, o.Obj) == "Cling to Dust" {
					switch o.Mode {
					case "escape":
						escCasts++
					case "flashback":
						flashCasts++
					}
				}
			}
			return a
		})
		m, err := NewMatchup(reg, [2]string{"escape-fb-a", "escape-fb-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{GorgeAutopaySurface: seed%2 == 0})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: Config{GorgeAutopaySurface: seed%2 == 0}, MaxDecisions: 6000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d escape offers, %d flashback offers, %d windows with both; %d escape casts, %d flashback casts",
		escOffers, flashOffers, both, escCasts, flashCasts)
	if both == 0 {
		t.Errorf("no decision offered both the escape and the granted flashback cast")
	}
	if escCasts == 0 || flashCasts == 0 {
		t.Errorf("both casts must be played: %d escape, %d flashback", escCasts, flashCasts)
	}
}

// escapeCards resolves a deck list against the corpus.
func escapeCards(t *testing.T, reg *cards.Registry, names []string) []*cards.Card {
	t.Helper()
	out := make([]*cards.Card, 0, len(names))
	for _, n := range names {
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the corpus", n)
		}
		out = append(out, c)
	}
	return out
}
