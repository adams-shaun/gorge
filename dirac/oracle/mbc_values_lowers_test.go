package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCValues3Lowers plays the MBC values-3 spellings kgen lowers, in
// lockstep with gorge over the mbcvalues fixture pool
// (dirac/cardset/pools/fixtures/mbcvalues):
//
//   - Orcish Settlers: "destroy X target lands" on an activated {X}{X}{R}
//     ability, a TgfBoundsV X..X slot outside a spell (lower_xcountactivated.go;
//     the kernel's X ask offers only an X with that many lands, as gorge's
//     targetsAnnounceableAt).
//   - Gnoll War Band: PlayerCountOpponents$HasPropertywasDealtDamageThisTurn,
//     ExOppsLostLife N 3 over the kernel's per-player damage ledger
//     (dirac/value_oppsdamaged.go), Shock dealing the noncombat damage.
//   - Legacy's Allure: Creature.powerLEX with X its treasure counters,
//     PredPowerGEV PredN 2 on the battlefield, and the TREASURE counter kind.
//
// Preconditions, from gorge's log: Settlers is activated with two or more
// targets, Gnoll War Band is cast in a turn an opponent was dealt damage
// before any attack (noncombat damage only), and Allure's ability takes
// control of a creature. Each mutation subtest breaks one lowering and must
// diverge. Killing Wave's count:Valid spelling stays refused with its memory
// blockers (the report).
func TestMBCValues3Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, tl := runValues3Lockstep(t, 48, true, nil)
		t.Logf("%d games, %d divergences, %d stalls; %+v", run.games, run.divs, run.stalls, tl)
		if tl.settlersMulti < 3 {
			t.Errorf("Orcish Settlers activated with two or more targets %d times, want at least 3", tl.settlersMulti)
		}
		if tl.gnollNC < 2 {
			t.Errorf("Gnoll War Band cast after noncombat damage to an opponent %d times, want at least 2", tl.gnollNC)
		}
		if tl.allureSteal < 3 {
			t.Errorf("Legacy's Allure took control of a creature %d times, want at least 3", tl.allureSteal)
		}
	})
	t.Run("mutation-settlers", func(t *testing.T) {
		dirac.DebugTgtBoundsMaxPlus1 = true
		defer func() { dirac.DebugTgtBoundsMaxPlus1 = false }()
		run, _ := runValues3Lockstep(t, 48, false, nil)
		run.requireDivergence(t, "an X..X activated target slot one too wide")
	})
	t.Run("mutation-gnoll", func(t *testing.T) {
		dirac.DebugOppsDamagedNoncombatOff = true
		defer func() { dirac.DebugOppsDamagedNoncombatOff = false }()
		run, _ := runValues3Lockstep(t, 48, false, nil)
		run.requireDivergence(t, "noncombat damage to a player left out of the dealt-damage ledger")
	})
	t.Run("mutation-allure", func(t *testing.T) {
		run, _ := runValues3Lockstep(t, 48, false, func(tbl *kop.Table) {
			n := 0
			for f := range tbl.Filters {
				if fl := &tbl.Filters[f]; fl.Pred == kop.PredPowerGEV && fl.PredN == 2 {
					fl.PredN = 0 // power >= X instead of <= X
					n++
				}
			}
			if n == 0 {
				t.Fatal("no PredPowerGEV PredN 2 filter to mutate")
			}
		})
		run.requireDivergence(t, "Legacy's Allure's power <= X read as >= X")
	})
}

var values3Watch = map[string]bool{"Orcish Settlers": true, "Gnoll War Band": true, "Legacy's Allure": true, "Shock": true, "Seething Song": true}

var values3Decks = fixtureDecks{
	{{Name: "Orcish Settlers", Count: 7}, {Name: "Gnoll War Band", Count: 5}, {Name: "Shock", Count: 6}, {Name: "Seething Song", Count: 6}, {Name: "Mountain", Count: 16}},
	{{Name: "Legacy's Allure", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 10}},
}

// values3Tally counts the preconditions over gorge's logs.
type values3Tally struct {
	settlers, settlersMulti int // Settlers activations; with two or more targets
	gnoll, gnollNC          int // Gnoll casts; in a turn an opponent was dealt damage before any attack
	allure, allureSteal     int // Allure activations; control changes they made
}

// values3Policy plays randomly but takes a cast of a watched card the moment
// one is offered (seven times in eight), an Orcish Settlers activation
// always and an Allure activation a quarter of the times it is offered, so
// Allure gathers counters first. Seething Song is cast only while its player
// controls a ready Settlers: gorge offers an {X} activation only when the
// floating pool pays it (the kernel's cfgGorgeUnpriced gate, legal.go), and
// the Song's five red mana is that pool.
func values3Policy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x76616c33)) // "val3"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority {
			for _, a := range c.D.Options {
				k := a.Kind()
				n := refCardName(c.G, a.Src())
				switch {
				case k == ActActivate && n == "Orcish Settlers":
					return a
				case k == ActActivate && n == "Legacy's Allure" && prng.IntN(4) == 0:
					return a
				case k == ActCast && n == "Seething Song":
					if values3SettlersReady(c.G, state.PlayerID(c.D.Player)) {
						return a
					}
				case k == ActCast && values3Watch[n] && prng.IntN(8) != 0:
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// values3SettlersReady reports whether p, the active player, controls an
// untapped Orcish Settlers that is not summoning sick (its {T} is payable).
func values3SettlersReady(g *GorgeSide, p state.PlayerID) bool {
	gg := g.E.G
	if gg.Active != p {
		return false
	}
	for _, id := range gg.Zone(state.ZBattlefield, p) {
		if o := gg.Obj(id); o != nil && !o.Tapped && !o.SummonSick && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Orcish Settlers" {
			return true
		}
	}
	return false
}

// requireValues3Lowered fails unless the table carries each lowering.
func requireValues3Lowered(t *testing.T, tbl *kop.Table) {
	t.Helper()
	var settlers, gnoll, allure bool
	for i := range tbl.Exprs {
		gnoll = gnoll || tbl.Exprs[i].Kind == kop.ExOppsLostLife && tbl.Exprs[i].N == 3
	}
	for _, cd := range tbl.Cards {
		for _, a := range cd.Abilities {
			for ti := 0; ti < int(a.NTargets); ti++ {
				ts := &a.Targets[ti]
				if cd.Name == "Orcish Settlers" && a.Kind == kop.AbActivated && ts.TFlags&kop.TgfBoundsV != 0 && !ts.UpTo {
					settlers = true
				}
				if cd.Name == "Legacy's Allure" && ts.Filter != 0 {
					fl := &tbl.Filters[ts.Filter]
					allure = allure || fl.Pred == kop.PredPowerGEV && fl.PredN == 2
				}
			}
		}
	}
	if !settlers || !gnoll || !allure {
		t.Fatalf("lowered: Orcish Settlers X..X slot %v, Gnoll War Band ExOppsLostLife N 3 %v, Legacy's Allure power <= X %v", settlers, gnoll, allure)
	}
}

// runValues3Lockstep plays seeds 1..seeds of the mbcvalues matchup, gorge
// leading, alternating the option surface. mutate rewrites the table first.
func runValues3Lockstep(t *testing.T, seeds int, report bool, mutate func(*kop.Table)) (fixtureRun, values3Tally) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbcvalues")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range values3Decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(values3Decks[0], values3Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	requireValues3Lowered(t, tbl)
	if mutate != nil {
		tbl.Filters = append([]kop.Filter(nil), tbl.Filters...) // the set's own table is shared
		mutate(tbl)
	}
	run := fixtureRun{casts: map[string]int{}}
	var tl values3Tally
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"values3-a", "values3-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, values3Policy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		run.games++
		values3Count(g, &tl)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			run.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if run.first == "" {
				run.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || run.divs >= 3 {
				break
			}
		}
	}
	return run, tl
}

// values3Count adds g's log to tl.
func values3Count(g *GorgeSide, tl *values3Tally) {
	name := func(id state.ObjID) string {
		o := g.E.G.Obj(id)
		if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
			return ""
		}
		return o.Card.Faces[0].Name
	}
	var damaged [8]bool // this turn, before any attack
	attacked := false
	open := "" // the watched activation whose targets are being announced
	tgts := 0  // its TargetsChosen events so far
	close := func() {
		if open == "Orcish Settlers" && tgts >= 2 {
			tl.settlersMulti++
		}
		open, tgts = "", 0
	}
	allureOpen := false // an Allure ability is on the stack
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.TurnChange:
			close()
			damaged, attacked = [8]bool{}, false
		case events.DeclareAttackers:
			attacked = true
		case events.Damage:
			if e.Obj == 0 && e.Amount > 0 && !attacked && int(e.Player) < len(damaged) {
				damaged[e.Player] = true
			}
		case events.AbilityPush:
			close()
			switch name(e.Obj) {
			case "Orcish Settlers":
				tl.settlers++
				open = "Orcish Settlers"
			case "Legacy's Allure":
				tl.allure++
				allureOpen = true
			}
		case events.TargetsChosen:
			if open != "" {
				tgts++
			}
		case events.PutOnStack:
			close()
			if name(e.Obj) == "Gnoll War Band" {
				tl.gnoll++
				for p, d := range damaged {
					if d && state.PlayerID(p) != e.Player {
						tl.gnollNC++
						break
					}
				}
			}
		case events.ControlChange:
			if allureOpen {
				tl.allureSteal++
				allureOpen = false
			}
		case events.Resolve:
			close()
		}
	}
	close()
}

// TestMBCValues1Lowers is the lockstep game per spelling of the MBC
// values-1 family: a pool built by kgen from the card's own decks (the build
// fails, and this test with it, while the spelling is refused), a policy that
// drives toward the card, and a tally of the behaviour the spelling lowers.
//
// Dyfed, the Guiding Hand lowers a planeswalker's [-X]: the announced X is
// the target bounds of "Untap X target artifacts", offered only while the
// target ask could answer it (CR 601.2b, 601.2c). Triumph of Ferocity lowers
// the ConditionCheckSVar gate beside the present check: draw at upkeep when
// you control the creature with the greatest power or tied for it. Pir,
// Imaginative Rascal lowers the any-kind plus-one counter replacement, the
// loyalty cost's counters included (Jace Beleren's [+2] puts three).
//
// Sahir, Visitor in Darkness and Lae'zel, Vlaakith's Champion stay refused by
// kgen for blockers outside this family (Sahir's trigger-matcher params,
// Lae'zel's Choose a Background and the You recipient of its putter line), so
// they have no game here.
var mbcValues1Decks = [3][2][]cardset.Entry{
	{ // M1: Dyfed beside artifacts to untap (Ornithopter is a free artifact, the Powerstone tokens are made)
		{
			{Name: "Dyfed, the Guiding Hand", Count: 6}, {Name: "Ornithopter", Count: 8},
			{Name: "Island", Count: 16}, {Name: "Forest", Count: 10},
		},
		{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Raging Goblin", Count: 8},
			{Name: "Doom Blade", Count: 4}, {Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 8},
		},
	},
	{ // M2: Triumph of Ferocity beside creatures of several powers on both sides
		{
			{Name: "Triumph of Ferocity", Count: 6}, {Name: "Hill Giant", Count: 6},
			{Name: "Grizzly Bears", Count: 6}, {Name: "Raging Goblin", Count: 6},
			{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 6},
		},
		{
			{Name: "Grizzly Bears", Count: 6}, {Name: "Raging Goblin", Count: 6}, {Name: "Hill Giant", Count: 6},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
		},
	},
	{ // M3: Pir, Imaginative Rascal beside counter makers of several shapes:
		// the greenhouse's one +1/+1 (plus one is two), Jace Beleren's [+2]
		// loyalty cost (three loyalty counters), the pantry's charge counters
		{
			{Name: "Pir, Imaginative Rascal", Count: 6}, {Name: "Atmospheric Greenhouse", Count: 3},
			{Name: "Stocking the Pantry", Count: 3}, {Name: "Wingspan Mentor", Count: 4},
			{Name: "Jace Beleren", Count: 3}, {Name: "Grizzly Bears", Count: 3},
			{Name: "Island", Count: 8}, {Name: "Forest", Count: 10},
		},
		{
			{Name: "Ajani, Caller of the Pride", Count: 4}, {Name: "Walking Corpse", Count: 6},
			{Name: "Doom Blade", Count: 4}, {Name: "Murder", Count: 3},
			{Name: "Swamp", Count: 12}, {Name: "Plains", Count: 11},
		},
	},
}

// mbcValues1Stats is what the gorge-side event log says the spelling decided.
type mbcValues1Stats struct {
	plays  int // casts of the card / activations of its lowered ability
	unTaps int // Dyfed: artifacts untapped while Dyfed is in play (the [-X])
	minusX int // Dyfed: loyalty counters removed (the [-X] cost, X >= 1)
	draws  int // Triumph: draws while Triumph is in play (the upkeep trigger)
	ctr    int // Pir: counter placements while Pir is in play (the plus one)
	loyal3 int // Pir: three loyalty counters at once (Jace's [+2] cost plus one)
	acts   int // Dyfed: activations of any of its abilities (debug)
	// abilSeen is the ability indices the Dyfed activation options carry (debug).
	abilSeen map[int]int
	noArt    int      // Dyfed: [-X] picks with no artifact on the battlefield (debug)
	dyfCtr   int      // Dyfed: counter events on a Dyfed permanent (debug)
	loyalAny int      // any LOYALTY counter event (debug)
	negAny   int      // any negative counter event (debug)
	seed1    []string // matchup 0 seed 1's counter events (debug)
}

// mbcValues1Tally is the event-log scanner one matchup keeps between seeds.
type mbcValues1Tally struct {
	evAt int
}

func (p *mbcValues1Tally) scan(ref *GorgeSide, mi, seed int, st *mbcValues1Stats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Draw:
			if e.To != state.ZHand {
				continue
			}
			if battlefieldHasName(g, e.Player, "Triumph of Ferocity") {
				st.draws++
			}
		case events.CounterChange:
			// The event log is read at the end of the game, so an object's
			// zone is read as it stands NOW: the permanent a counter event
			// names is identified by its face, not by where it lies.
			o := g.Obj(e.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			if o.Face().Name == "Dyfed, the Guiding Hand" && e.Counter == "LOYALTY" && e.Amount <= -1 {
				st.minusX++
			}
			if e.Amount <= 0 || o.Zone != state.ZBattlefield {
				continue
			}
			if battlefieldHasName(g, o.Controller, "Pir, Imaginative Rascal") {
				st.ctr++
				if e.Counter == "LOYALTY" && e.Amount == 3 {
					st.loyal3++
				}
			}
		}
	}
	p.evAt = len(evs)
}

// mbcValues1Names is the card-name list the test's kernel table is built from.
func mbcValues1Names(t *testing.T) [2][]string {
	t.Helper()
	var names [2][]string
	for _, d := range mbcValues1Decks {
		for s, dd := range d {
			for _, e := range dd {
				for i := 0; i < e.Count; i++ {
					names[s] = append(names[s], e.Name)
				}
			}
		}
	}
	return names
}

// mbcValues1Policy plays toward the matchup's card: it casts the card, lands
// and (Dyfed) the [-X] ability ahead of anything else, and answers everything
// else at random. The plays it takes are tallied.
func mbcValues1Policy(seed uint64, card string, actAbility int, st *mbcValues1Stats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6d62633431)) // "mbc41"
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			best, bestRank := Canon(0), 0
			for _, o := range c.D.Options {
				r := 0
				g, ok := c.G.GorgeOption(o)
				switch {
				case !ok:
				case o.Kind() == ActCast && g.Kind == "cast" && sacDiscountSource(c.G, g.Obj) == card:
					r = 6
				case o.Kind() == ActActivate && g.Kind == "ability" && actAbility >= 0 &&
					sacDiscountSource(c.G, g.Obj) == card && int(g.Ability) == actAbility:
					r = 5
				}
				if r > bestRank {
					bestRank, best = r, o
				}
			}
			if bestRank >= 5 {
				st.plays++
				return best
			}
		}
		switch c.D.Kind {
		case DPriority:
			buf = buf[:0]
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return buf[prng.IntN(len(buf))]
			}
		}
		return rnd.Pick(c)
	})
}

// runMBCValues1 plays each matchup's lockstep games and returns the games,
// divergences and tallies.
func runMBCValues1(t *testing.T, seeds int) (games, divs int, st [3]mbcValues1Stats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set := printedDelveSet(t, mbcValues1Names(t))
	names := mbcValues1Decks
	cardNames := []string{"Dyfed, the Guiding Hand", "Triumph of Ferocity", "Pir, Imaginative Rascal"}
	abilities := [3]int{-1, -1, -1}
	abilities[0] = 1 // Dyfed's [-X]: the second printed ability
	for mi, md := range names {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		var tl mbcValues1Tally
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{}
			if seed%4 == 0 {
				cfg = Config{GorgeAutopaySurface: true}
			}
			m, err := NewMatchup(reg, [2]string{"mbcvals1-a", "mbcvals1-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, mbcValues1Policy(seed, cardNames[mi], abilities[mi], &st[mi]),
				RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			tl.scan(g, mi, int(seed), &st[mi])
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				t.Errorf("matchup %d seed %d: kernel stall %q", mi, seed, res.Stall)
				continue
			}
			if d := res.Div; d != nil {
				divs++
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, st
}

func TestMBCValues1Lowers(t *testing.T) {
	t.Parallel()
	games, divs, st := runMBCValues1(t, 24)
	t.Logf("%d games, %d divergences", games, divs)
	t.Logf("Dyfed %+v Triumph %+v Pir %+v", st[0], st[1], st[2])
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Dyfed cast or its [-X] activated", st[0].plays},
		{"Dyfed [-X] costs paid with X >= 1", st[0].minusX},
		{"Triumph cast", st[1].plays},
		{"draws under Triumph", st[1].draws},
		{"Pir cast", st[2].plays},
		{"counter placements under Pir", st[2].ctr},
		{"three loyalty counters at once under Pir (Jace's [+2])", st[2].loyal3},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}
