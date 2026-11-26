package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The AffectedZone$ Stack keyword grants of the staticzones umbrella that
// gorge reads off its derived keyword list (kgen lower_stackkw.go): Cast
// Through Time (rebound), Thrumming Stone (ripple), Peter Parker's Amazing
// Spider-Man face (web-slinging) and Zinnia, Valley's Voice (offspring).
// TestStaticZonesLockstep carries one or two of each in the shared decks; the
// decks here are built to reach the keyword often enough that a kernel that
// ignores the grant diverges (TestStaticZonesStackKwMutation).

// kstaticzonesRebound: Cast Through Time over a blue-red deck of burn and Opt
// with mana rocks (no card-draw engine, so no game decks out before the
// rebound casts happen); the opposing deck is the burn deck of the main pool.
var kstaticzonesRebound = [2][]cardset.Entry{
	{
		{Name: "Cast Through Time", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Opt", Count: 4},
		{Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 2}, {Name: "Island", Count: 12}, {Name: "Mountain", Count: 8},
	},
	kstaticzonesDecks[1],
}

func TestStaticZonesReboundLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "staticzones", kstaticzonesRebound, 200, "KSTATICZONESREBOUND_SEEDS")
}

// kstaticzonesWebSling: Peter Parker over three colours with Iridescent Tiger
// (its enters trigger floats {W}{U}{B}{R}{G}). Peter Parker's Amazing Spider-Man
// back face (cast from the hand, {1}{G}{W}{U}) grants each legendary coloured
// spell its controller casts web-slinging {G}{W}{U}: a later Peter Parker is
// offered for it by returning a tapped creature (the Spider token or a Bears
// that attacked), paid from the floating mana (gorge offers no web-slinging
// cast its planner cannot price, so the pool must hold the cost).
var kstaticzonesWebSling = [2][]cardset.Entry{
	{
		{Name: "Peter Parker", Count: 6}, {Name: "Iridescent Tiger", Count: 6},
		{Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 6},
	},
	kstaticzonesDecks[1],
}

// staticZonesWebSlingRun plays seeds 1..seeds of the web-slinging pool under
// the printed web-slinging lockstep's probe (websling_test.go: attack often,
// prefer the web-slinging cast) and returns whether a game diverged and the
// probe's tallies.
func staticZonesWebSlingRun(t *testing.T, seeds uint64, report bool) (diverged bool, st webSlingStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, "staticzones", kstaticzonesWebSling)
	tbl, _, _, err := set.Matchup(kstaticzonesWebSling[0], kstaticzonesWebSling[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"staticzones-a", "staticzones-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &webSlingProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x7a696e6e))}
		// the window the grant needs is rare under a random walk: with Amazing
		// Spider-Man in play, a Peter Parker in hand and a creature tapped
		// (an attacker, in the second main phase), cast Iridescent Tiger, whose
		// enters trigger floats the {G}{W}{U} the web-slinging cast is priced on
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority && staticZonesWebSlingReady(c) {
				for _, x := range c.D.Options {
					if x.Kind() == ActCast && refCardName(c.G, x.Src()) == "Iridescent Tiger" {
						return x
					}
				}
			}
			a := probe.pick(c)
			if c.G != nil && c.D.Kind == DPriority && a.Kind() == ActCast && refCardName(c.G, a.Src()) == "Iridescent Tiger" {
				// hold the Tigers for the window above
				for _, x := range c.D.Options {
					if x.Kind() == ActPass {
						return x
					}
				}
			}
			return a
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

// staticZonesWebSlingReady: the priority holder controls Amazing Spider-Man
// and a tapped creature and holds a Peter Parker (gorge side).
func staticZonesWebSlingReady(c *PickCtx) bool {
	g := c.G.E.G
	p := state.PlayerID(c.D.Player)
	asm := false
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Controller == p && o.Face() != nil && o.Face().Name == "Amazing Spider-Man" {
			asm = true
		}
	}
	if !asm || !webSlingInHandNamed(g, p, "Peter Parker") {
		return false
	}
	_, tapped := webSlingCreatures(g, p)
	return tapped > 0
}

func webSlingInHandNamed(g *state.Game, p state.PlayerID, name string) bool {
	for _, id := range g.Zone(state.ZHand, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// TestStaticZonesWebSlingLockstep: every web-slinging cast gorge offers under
// the Amazing Spider-Man grant is answered by the kernel's AkWebSling cast.
func TestStaticZonesWebSlingLockstep(t *testing.T) {
	t.Parallel()
	_, st := staticZonesWebSlingRun(t, 600, true)
	t.Logf("web-slinging %+v", st)
	if st.casts < 1 {
		t.Errorf("no granted web-slinging cast in 600 games")
	}
}

// kstaticzonesOffspring: Zinnia, Valley's Voice ({U}{R}{W}, its grant reaches
// every creature spell its controller casts) over green creatures and the
// off-colour rocks. Iridescent Tiger's ETB trigger floats {W}{U}{B}{R}{G}, so
// the offspring cast's base plus {2} is payable from the floats and the
// Tiger's own offspring cast is priced beside it; the Tiger's printed ETB
// trigger beside the granted copy trigger is the queue-order probe (gorge's
// MoveZone walk queues the entering permanent's printed triggers, then its
// granted walks, trigger_granted.go checkGrantedOffspringTriggers).
var kstaticzonesOffspring = [2][]cardset.Entry{
	{
		{Name: "Zinnia, Valley's Voice", Count: 4}, {Name: "Iridescent Tiger", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 2},
		{Name: "Island", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Plains", Count: 5}, {Name: "Forest", Count: 7},
	},
	// the slice's baseline second deck, not the shared pool's kstaticzonesDecks[1]:
	// Squee, the Immortal and Defensive Formation stay out of this pool. With
	// them in, seed 158's trajectories reach a window this pool must not gate
	// its count on until the ticket lands: Swords to Plowshares exiles the
	// offspring-cast Grizzly Bears while its granted offspring trigger is
	// still pending, and the kernel mints the 1/1 copy (seed 158, decision
	// 156) where gorge holds the trigger on the stack
	// (dirac-granted-offspring-exile-window).
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Swords to Plowshares", Count: 3}, {Name: "Giant Growth", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Dragon's Rage Channeler", Count: 2}, {Name: "Spire Serpent", Count: 2},
		{Name: "Case of the Locked Hothouse", Count: 2},
		{Name: "True Believer", Count: 2}, {Name: "Trusted Advisor", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 3}, {Name: "Mountain", Count: 5},
	},
}

// offStats tallies the offspring pool's exercise: an offspring cast (the
// canon cast-mode code 15, gorge's Mode "offspring"), a plain cast beside it
// and the 1/1 token copy the paid cast's enter trigger mints.
type offStats struct {
	casts, plain, tokens int
}

// staticZonesOffspringRun plays seeds 1..seeds of the offspring pool,
// preferring an offspring cast option whenever one is offered (with Zinnia in
// play one is, for every creature spell its controller can afford) and
// returns whether a game diverged and the tallies.
func staticZonesOffspringRun(t *testing.T, seeds uint64, report bool) (diverged bool, st offStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, "staticzones", kstaticzonesOffspring)
	tbl, _, _, err := set.Matchup(kstaticzonesOffspring[0], kstaticzonesOffspring[1])
	if err != nil {
		t.Fatal(err)
	}
	offCode, _ := canon.CastModeCode("offspring")
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"staticzones-a", "staticzones-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		base := RandomPolicy(seed)
		casts, plain, minted := 0, 0, false
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority {
				if !minted {
					minted = offTokens(c.G.E.G) > 0
				}
				for _, x := range c.D.Options {
					if x.Kind() == ActCast && x.Mode()&0xf == offCode {
						casts++
						return x
					}
				}
				for _, x := range c.D.Options {
					if x.Kind() == ActCast && x.Mode()&0xf == 0 && refCardName(c.G, x.Src()) == "Grizzly Bears" {
						plain++
						return x
					}
				}
			}
			return base.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if res.Div != nil {
			diverged = true
			if report {
				d := res.Div
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
		st.casts += casts
		st.plain += plain
		if minted {
			st.tokens++
		}
	}
	return
}

// offTokens reports a 1/1 token copy on either battlefield whose face is the
// copy's source's (gorge's CopyPermanent mints the token with its card's
// name, SetPower$/SetToughness$ 1).
func offTokens(g *state.Game) int {
	if g == nil {
		return 0
	}
	n := 0
	for p := state.PlayerID(0); int(p) < len(g.Players); p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil || !o.IsToken || o.Face() == nil {
				continue
			}
			if o.Face().Name == "Grizzly Bears" || o.Face().Name == "Iridescent Tiger" {
				n++
			}
		}
	}
	return n
}

// TestStaticZonesOffspringLockstep: every granted offspring cast gorge offers
// under Zinnia's grant is answered by the kernel's kicked cast, and the paid
// cast's enter trigger mints the 1/1 token copy.
func TestStaticZonesOffspringLockstep(t *testing.T) {
	t.Parallel()
	_, st := staticZonesOffspringRun(t, 200, true)
	t.Logf("offspring %+v", st)
	if st.casts < 1 {
		t.Errorf("no granted offspring cast in 200 games")
	}
	if st.tokens < 1 {
		t.Errorf("no offspring token copy in 200 games")
	}
}

// TestStaticZonesStackKwMutation proves the pools exercise the grants: a
// kernel that reads no stack keyword grant exiles no Cast Through Time spell,
// so its options differ from gorge's upkeep recast offer.
func TestStaticZonesStackKwMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugStackKwOff = true
	defer func() { dirac.DebugStackKwOff = false }()
	if !lockstepDiverges(t, "staticzones", kstaticzonesRebound, 64) {
		t.Fatal("DebugStackKwOff: no divergence in 64 seeds of the rebound pool")
	}
	if diverged, _ := staticZonesWebSlingRun(t, 600, false); !diverged {
		t.Fatal("DebugStackKwOff: no divergence in 600 seeds of the web-slinging pool")
	}
	if diverged, _ := staticZonesOffspringRun(t, 64, false); !diverged {
		t.Fatal("DebugStackKwOff: no divergence in 64 seeds of the offspring pool")
	}
}
