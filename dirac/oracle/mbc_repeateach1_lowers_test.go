package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCRepeatEach1Lowers plays the MBC repeat-each-1 spellings kgen
// lowers in lockstep with gorge over the mbcrepeateach1 fixture pool
// (dirac/cardset/pools/fixtures/mbcrepeateach1):
//
//   - Rakdos Charm: RepeatEach RepeatCards$ Creature without ChangeZoneTable$,
//     its body "the repeated card deals 1 damage to its controller"
//     (lower_repeatcardsdamage.go; the kernel's DealDamageFrom
//     kop.DmgFromEachCtrl). Precondition: some Rakdos Charm resolution
//     dealt damage from two or more creatures to players. The mutation
//     deals 2 instead of 1.
//   - Azra Bladeseeker: RepeatPlayers$ Player.YourTeam, the controller alone
//     (gorge models no teams), whose body is the join-forces discard-to-draw.
//     Precondition: some Azra trigger's controller discarded and drew. The
//     mutation loops over every player, so the opponent is asked too.
//
// The other repeat-each-1 spellings stay refused: see the report.
func TestMBCRepeatEach1Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, st := runRepeatEach1Lockstep(t, 24, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, %+v", run.games, run.divs, run.stalls, st)
		if st.charmMulti < 2 {
			t.Errorf("Rakdos Charm's third mode dealt damage from two or more creatures in %d resolutions, want at least 2", st.charmMulti)
		}
		if st.azraLooted < 2 {
			t.Errorf("an Azra Bladeseeker trigger's controller discarded and drew in %d resolutions, want at least 2", st.azraLooted)
		}
	})
	t.Run("mutation/RakdosCharm", func(t *testing.T) {
		run, _ := runRepeatEach1Lockstep(t, 24, false, func(tbl *kop.Table) {
			n := 0
			mutateCard(t, tbl, "Rakdos Charm", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpDealDamageFrom && in.B.N == kop.DmgFromEachCtrl {
					in.A, n = kop.Const(2), n+1
				}
			})
			if n == 0 {
				t.Fatal("Rakdos Charm: no DealDamageFrom DmgFromEachCtrl in the table (the body is not lowered)")
			}
		})
		run.requireDivergence(t, "the repeated creature dealing 2 to its controller")
	})
	t.Run("mutation/AzraBladeseeker", func(t *testing.T) {
		run, _ := runRepeatEach1Lockstep(t, 24, false, func(tbl *kop.Table) {
			n := 0
			mutateCard(t, tbl, "Azra Bladeseeker", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpRepeatEachPlayer && in.Obj == kop.RefController {
					in.Obj, n = kop.RefEachPlayer, n+1
				}
			})
			if n == 0 {
				t.Fatal("Azra Bladeseeker: no RepeatEachPlayer over the controller in the table (Player.YourTeam is not lowered)")
			}
		})
		run.requireDivergence(t, "Azra Bladeseeker's loop over every player")
	})
}

var repeatEach1Watch = map[string]bool{"Rakdos Charm": true, "Azra Bladeseeker": true}

var repeatEach1Decks = []fixtureDecks{
	{
		{{Name: "Rakdos Charm", Count: 8}, {Name: "Azra Bladeseeker", Count: 6}, {Name: "Vampire Nighthawk", Count: 4}, {Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 11}},
		{{Name: "Sewer Rats", Count: 8}, {Name: "Vampire Nighthawk", Count: 4}, {Name: "Azra Bladeseeker", Count: 6}, {Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 11}},
	},
	{
		{{Name: "Sewer Rats", Count: 8}, {Name: "Azra Bladeseeker", Count: 6}, {Name: "Rakdos Charm", Count: 4}, {Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 11}},
		{{Name: "Rakdos Charm", Count: 8}, {Name: "Vampire Nighthawk", Count: 6}, {Name: "Mountain", Count: 13}, {Name: "Swamp", Count: 13}},
	},
}

// repeatEach1Stats are the preconditions read off gorge's logs.
type repeatEach1Stats struct {
	charmMulti int // Rakdos Charm resolutions dealing damage to players from two or more creatures
	azraLooted int // Azra Bladeseeker trigger resolutions in which its controller discarded and drew
}

// runRepeatEach1Lockstep plays seeds 1..seeds of the mbcrepeateach1
// matchups, gorge leading, alternating the matchup and the option surface by
// seed (runFixtureLockstep's shape, keeping each game's gorge log for the
// preconditions). mutate rewrites each matchup's table first.
func runRepeatEach1Lockstep(t *testing.T, seeds int, report bool, mutate func(*kop.Table)) (fixtureRun, repeatEach1Stats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbcrepeateach1")
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][2][]*cards.Card, len(repeatEach1Decks))
	tbls := make([]*kop.Table, len(repeatEach1Decks))
	for mi, md := range repeatEach1Decks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for range e.Count {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			mutate(tbl)
		}
		tbls[mi] = tbl
	}
	run := fixtureRun{casts: map[string]int{}}
	var st repeatEach1Stats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		mi := int(seed/2) % len(repeatEach1Decks)
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"repeat-a", "repeat-b"}, decks[mi], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[mi], cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, fixtureWatchPolicy(seed, repeatEach1Watch, run.casts), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		run.games++
		repeatEach1Count(g, &st)
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
	return run, st
}

// repeatEach1Count reads one gorge log: the events between a watched
// object's Resolve and the next Priority are that resolution's.
func repeatEach1Count(g *GorgeSide, st *repeatEach1Stats) {
	cur := ""
	var ctrl state.PlayerID
	dmg, discarded, drew := 0, false, false
	flush := func() {
		switch cur {
		case "Rakdos Charm":
			if dmg >= 2 {
				st.charmMulti++
			}
		case "Azra Bladeseeker":
			if discarded && drew {
				st.azraLooted++
			}
		}
		cur, dmg, discarded, drew = "", 0, false, false
	}
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.Resolve:
			flush()
			o := g.E.G.Obj(e.Obj)
			if o == nil {
				break
			}
			src := o
			if o.Source != 0 {
				// a triggered ability on the stack names its source
				if so := g.E.G.Obj(o.Source); so != nil {
					src = so
				}
			}
			if src.Card != nil && len(src.Card.Faces) > 0 && repeatEach1Watch[src.Card.Faces[0].Name] {
				cur, ctrl = src.Card.Faces[0].Name, o.Controller
			}
		case events.Priority:
			flush()
		case events.Damage:
			if e.Obj == 0 && e.Amount > 0 {
				dmg++
			}
		case events.MoveZone:
			if o := g.E.G.Obj(e.Obj); o != nil && e.From == state.ZHand && e.To == state.ZGraveyard && o.Owner == ctrl {
				discarded = true
			}
		case events.Draw:
			if e.Player == ctrl {
				drew = true
			}
		}
	}
	flush()
}
