package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC zone-moves-10 ticket's three spellings, played in lockstep games
// drawn from their dr-mbc draft-pool halves (kgen TestTrigToGraveyardLowers
// lowers the trigger, TestTopLibraryOptionalLowers the TopOfLibrary move and
// TestHiddenDefinedPlayerLowers the DefinedPlayer$ pick):
//
//   - Syr Konrad, the Grim (dr-mbc-abaa), the Origin$ set on a ChangesZone
//     trigger: it mills a card with its own ability and a creature card lands
//     in a graveyard from a library, so each opponent loses 1 life.
//   - Vivien's Grizzly (dr-mbc-bbba), the TopOfLibrary move with no Optional$
//     True: its ability looks at the top card — the deck is Forests, so the
//     card never qualifies and the ability puts it on the library's bottom.
//   - Blood Clock (tail-33, the corpus pool that holds it), the
//     DefinedPlayer$ pick: at each player's upkeep that player returns a
//     permanent they control to its owner's hand unless they pay 2 life.
//
// Each game is recorded by TestMBCZoneMoves10Record (PINS_RECORD=1) and
// replayed by TestMBCZoneMoves10Lowers and the pool globs: no divergence,
// every scripted action offered, and the behaviour the lowering bought.

const (
	konradRepro     = "mbc_zone_moves_10_konrad_mills_a_creature"
	grizzlyRepro    = "mbc_zone_moves_10_grizzly_bottoms_the_top_card"
	bloodClockRepro = "mbc_zone_moves_10_blood_clock_upkeep_pick"
)

var konradDecks = [2][]string{
	append(deckOf(4, "Syr Konrad, the Grim"), append(deckOf(16, "Swamp"), deckOf(20, "Typhoid Rats")...)...),
	deckOf(40, "Typhoid Rats"),
}

var grizzlyDecks = [2][]string{
	append(deckOf(4, "Vivien's Grizzly"), deckOf(36, "Forest")...),
	deckOf(40, "Mountain"),
}

var clockDecks = [2][]string{
	append(deckOf(4, "Blood Clock"), deckOf(36, "Swamp")...),
	deckOf(40, "Swamp"),
}

// mbcZoneCase is one spelling's fixture: its pool half, decks, the card its
// seat casts, the card whose ability it activates, and the observable the
// game must reach.
type mbcZoneCase struct {
	name, pool, cast, activate string
	decks                      [2][]string
	maxTurns, maxDecisions     int
	// declineModes answers every pay-or-decline ask with the decline, so
	// Blood Clock's "unless they pay 2 life" return happens.
	declineModes bool
	// hit reports the game's observable: a count the replay requires > 0.
	hit func(evs []events.Event, g *state.Game) int
}

func mbcZoneCases() []mbcZoneCase {
	return []mbcZoneCase{
		{
			name: konradRepro, pool: "dr-mbc-abaa", cast: "Syr Konrad, the Grim", activate: "Syr Konrad, the Grim",
			decks: konradDecks, maxTurns: 30, maxDecisions: 400,
			hit: func(evs []events.Event, g *state.Game) int {
				// a creature card milled from a library while Konrad was on
				// the battlefield; the seat 1 life the trigger dealt
				in, milled := false, 0
				for i := range evs {
					e := &evs[i]
					if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
						if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Syr Konrad, the Grim" {
							in = true
						}
					}
					if in && e.Kind == events.MoveZone && e.From == state.ZLibrary && e.To == state.ZGraveyard {
						milled++
					}
				}
				if g.Players[1].Life >= 20 {
					return 0 // the trigger never dealt its damage
				}
				return milled
			},
		},
		{
			name: grizzlyRepro, pool: "dr-mbc-bbba", cast: "Vivien's Grizzly", activate: "Vivien's Grizzly",
			decks: grizzlyDecks, maxTurns: 30, maxDecisions: 300,
			hit: func(evs []events.Event, g *state.Game) int {
				// the top card put on the library's bottom while the Grizzly
				// was on the battlefield (the deck is Forests: no reveal, no
				// hand branch)
				in, moved := false, 0
				for i := range evs {
					e := &evs[i]
					if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
						if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Vivien's Grizzly" {
							in = true
						}
					}
					if in && e.Kind == events.MoveZone && e.From == state.ZLibrary && e.To == state.ZLibrary {
						moved++
					}
				}
				return moved
			},
		},
		{
			name: bloodClockRepro, pool: "tail-33", cast: "Blood Clock", activate: "", declineModes: true,
			decks: clockDecks, maxTurns: 30, maxDecisions: 700,
			hit: func(evs []events.Event, g *state.Game) int {
				// a permanent returned to its owner's hand at an upkeep while
				// the Clock was on the battlefield (nothing else bounces)
				in, moved := false, 0
				for i := range evs {
					e := &evs[i]
					if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
						if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Blood Clock" {
							in = true
						}
					}
					if in && e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZHand {
						moved++
					}
				}
				return moved
			},
		},
	}
}

// mbcZonePolicy is the scripted-seats' policy: seat 0 casts the case's card,
// then activates its ability, then plays lands; everything else passes.
func mbcZonePolicy(m *Matchup, c mbcZoneCase) Policy {
	return PolicyFunc(func(p *PickCtx) Canon {
		opts := p.D.Options
		kindOf := func(k CanonKind) []Canon {
			var out []Canon
			for _, o := range opts {
				if o.Kind() == k {
					out = append(out, o)
				}
			}
			return out
		}
		if p.D.Kind == DPriority && p.D.Player == 0 {
			for _, o := range kindOf(ActCast) {
				if m.InstanceCard(int(o.Src())) == c.cast {
					return o
				}
			}
			if c.activate != "" {
				for _, o := range kindOf(ActActivate) {
					if m.InstanceCard(int(o.Src())) == c.activate {
						return o
					}
				}
			}
			if l := kindOf(ActPlayLand); len(l) > 0 {
				return l[0]
			}
		}
		if p.D.Kind == DAttackers || p.D.Kind == DBlockers {
			if d := kindOf(ActDone); len(d) > 0 {
				return d[0]
			}
		}
		if c.declineModes {
			// a pay-or-decline ask (the Clock's "unless they pay 2 life"):
			// decline, so the Clock's return happens
			if yes, no := kindOf(ActYes), kindOf(ActNo); len(yes) > 0 && len(no) > 0 {
				return no[0]
			}
			if modes := kindOf(ActMode); len(modes) > 1 {
				return modes[len(modes)-1]
			}
		}
		return opts[0]
	})
}

// mbcZonePlay is one case's lockstep game: the pool half's table, the decks,
// the policy, and the hit count the Finish hook scans.
func mbcZonePlay(t *testing.T, reg *cards.Registry, r *Repro, set cardset.Set, pol Policy) (Result, int) {
	t.Helper()
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
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full" // as the pool globs replay it
	rc.MaxDecisions = len(r.Actions) + 200
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
	}
	var hits int
	for _, c := range mbcZoneCases() {
		if strings.TrimSuffix(r.Decks[0], "_0") == c.name {
			hitsFn := c.hit
			rc.Finish = func(g *GorgeSide) { hits = hitsFn(g.E.L.Events, g.E.G) }
			break
		}
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), hits
}

// TestMBCZoneMoves10Lowers replays the three recorded games: no divergence,
// every scripted action offered, and each game's observable reached.
func TestMBCZoneMoves10Lowers(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range mbcZoneCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			r.Why = c.name
			pol := &ScriptPolicy{Actions: r.Actions}
			res, hits := mbcZonePlay(t, reg, r, cardsettest.Synth(t, c.pool), pol)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if pol.Skipped != 0 || pol.next != len(r.Actions) {
				t.Errorf("%d of %d scripted actions were not offered", pol.Skipped, len(r.Actions))
			}
			if hits == 0 {
				t.Fatalf("%s: the replayed game did not reach the behaviour the lowering bought", c.name)
			}
		})
	}
}

// TestMBCZoneMoves10ReprosLockstep replays the pools' other recordings by
// glob, so the new recordings are replayed without a Go edit.
func TestMBCZoneMoves10ReprosLockstep(t *testing.T) {
	stdReprosGlobPools(t, "dr-mbc-abaa", "dr-mbc-bbba", "tail-33")
}

// TestMBCZoneMoves10Record plays each case's policy over seeds until one
// reaches its observable, and writes the repro. It runs only with
// PINS_RECORD=1; the games it writes are the committed repros.
func TestMBCZoneMoves10Record(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	dbg := os.Getenv("MBC_ZONE_DEBUG") != "" // a debugging aid, not coverage
	reg := synthReg(t)
	for _, c := range mbcZoneCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 200; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: "autopay", Kernel: "synth " + c.pool,
					Pool: c.pool, Why: c.name, MaxTurns: int32(c.maxTurns)}
				set := cardsettest.Synth(t, c.pool)
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
				tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
				if err != nil {
					t.Fatal(err)
				}
				rc := r.RunConfig()
				rc.Config.GorgeAutopaySurface = r.Surface != "full"
				rc.MaxDecisions = c.maxDecisions
				var hits int
				hitsFn := c.hit
				var dbgEvs []events.Event
				var dbgGame *state.Game
				if dbg {
					rc.Finish = func(g *GorgeSide) {
						hits = hitsFn(g.E.L.Events, g.E.G)
						dbgEvs = g.E.L.Events
						dbgGame = g.E.G
					}
				} else {
					rc.Finish = func(g *GorgeSide) { hits = hitsFn(g.E.L.Events, g.E.G) }
				}
				k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
				if err != nil {
					t.Fatal(err)
				}
				res := Lockstep(m, r.Seed, k, mbcZonePolicy(m, c), rc)
				CloseSide(k)
				if c.name == bloodClockRepro && seed <= 3 && dbg && dbgEvs != nil {
					cast, moved := 0, 0
					for i := range dbgEvs {
						e := &dbgEvs[i]
						if o := dbgGame.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Blood Clock" && e.Kind == events.MoveZone && e.To == state.ZBattlefield {
							cast++
						}
						if e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZHand {
							moved++
						}
					}
					t.Logf("seed %d: div=%v stall=%q clockETB=%d BFtoHand=%d events=%d", seed, res.Div != nil, res.Stall, cast, moved, len(dbgEvs))
				}
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if hits == 0 {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions, hits %d", seed, len(res.Actions), hits)
				return
			}
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}
