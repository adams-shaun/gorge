package oracle

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The max-speed grant lockstep: "Max speed —"
// activated abilities with a mana cost (CR 702.179e), on the battlefield
// (Hour of Victory, Slick Imitator, Avishkar Raceway) and from
// the graveyard (the Surveyors, CR 113.6/602.1). gorge offers each as a
// "granted" option priced from the floating pool alone (statics_cost.go
// manaFeasiblePricedP), so the kernel offers it only when the pool pays it
// (dirac/kw_maxspeed.go); the mutation prices it with lands too.

var speedGrantDecks = [2][]cardset.Entry{
	{
		{Name: "Hour of Victory", Count: 4}, {Name: "Goblin Surveyor", Count: 4},
		{Name: "Mutant Surveyor", Count: 3}, {Name: "Avishkar Raceway", Count: 3},
		{Name: "Iridescent Tiger", Count: 3}, {Name: "Burning-Tree Emissary", Count: 3},
		{Name: "Gleaming Barrier", Count: 2}, {Name: "Shock", Count: 3},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Faithless Looting", Count: 3},
		{Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Leonin Surveyor", Count: 4}, {Name: "Loxodon Surveyor", Count: 3},
		{Name: "Glitch Ghost Surveyor", Count: 3}, {Name: "Slick Imitator", Count: 3},
		{Name: "Iridescent Tiger", Count: 3}, {Name: "Burning-Tree Emissary", Count: 3},
		{Name: "Lightwheel Enhancements", Count: 4},
		{Name: "Careful Study", Count: 1}, {Name: "Shock", Count: 2},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 4},
		{Name: "Forest", Count: 3}, {Name: "Mountain", Count: 3},
	},
}

// speedGrantPay is the ETB mana the decks run: Iridescent Tiger adds {W}{U}{B}{R}{G}
// when cast, Burning-Tree Emissary {R}{G} when it enters. They are the floating
// pool a max-speed grant is priced from (gorge's real offer walk reads nothing
// else), so the policy casts them first at speed 4.
var speedGrantPay = map[string]bool{"Iridescent Tiger": true, "Burning-Tree Emissary": true}

type speedGrantTally struct {
	surveyorGrave int // a Surveyor's graveyard grant activated
	bfGrant       int // a battlefield mana-costed grant activated
	played        map[string]int
	badZone       string // a Surveyor's grant offered outside the graveyard
	lwGrave       int    // Lightwheel Enhancements cast from the graveyard
	auraCopies    int    // Slick Imitator copied an Aura spell
	copySeen      map[state.ObjID]bool
}

// speedGrantAuras are the Aura spells Slick Imitator copies.
var speedGrantAuras = map[string]bool{"Lightwheel Enhancements": true, "Rancor": true}

var speedGrantSurveyors = map[string]bool{
	"Glitch Ghost Surveyor": true, "Goblin Surveyor": true, "Leonin Surveyor": true,
	"Loxodon Surveyor": true, "Mutant Surveyor": true,
}

var speedGrantBattlefield = map[string]bool{
	"Hour of Victory": true, "Slick Imitator": true, "Avishkar Raceway": true,
}

func poolSum(p []int32) (n int32) {
	for _, v := range p {
		n += v
	}
	return n
}

// speedGrantPolicy attacks, casts the mana-adding creatures first at speed 4
// (the floating pool a grant is priced from) and activates a "granted" option
// whenever one is offered.
func speedGrantPolicy(seed uint64, tally *speedGrantTally) Policy {
	rnd := RandomPolicy(seed)
	coin := rand.New(rand.NewPCG(seed, 0x5eed))
	return PolicyFunc(func(c *PickCtx) Canon {
		var granted, payCasts, attack, cast, auraTarget, auraCasts, slickCasts, slickCopy []Canon // payCasts: the casts that add mana
		if c.G != nil {
			for _, id := range c.G.E.G.Stack { // a copy of an Aura spell on the stack
				if ob := c.G.E.G.Obj(id); ob != nil && ob.IsCopy && ob.Face() != nil && speedGrantAuras[ob.Face().Name] && !tally.copySeen[id] {
					tally.copySeen[id] = true
					tally.auraCopies++
				}
			}
		}
		for _, a := range c.D.Options {
			switch k := a.Kind(); {
			case k == ActActivate && a.Mode() >= maxSpeedBase:
				granted = append(granted, a)
				if o, ok := c.G.GorgeOption(a); ok && c.G != nil && len(c.G.E.G.Stack) > 0 {
					top := c.G.E.G.Obj(c.G.E.G.Stack[len(c.G.E.G.Stack)-1])
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Slick Imitator" &&
						top != nil && !top.IsCopy && top.Face() != nil && speedGrantAuras[top.Face().Name] {
						slickCopy = append(slickCopy, a) // an Aura spell on top: copy it
					}
				}
			case k == ActActivate && c.G != nil:
				cast = append(cast, a)
			case k == ActTarget && c.G != nil:
				if o, ok := c.G.GorgeOption(a); ok && o.Obj != 0 {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZStack && !ob.IsCopy && ob.Face() != nil && speedGrantAuras[ob.Face().Name] {
						auraTarget = append(auraTarget, a) // Slick Imitator's copy of an Aura spell
					}
				}
			case k == ActAttack:
				attack = append(attack, a)
			case k == ActCast:
				o, ok := c.G.GorgeOption(a)
				hold := false // at speed 4 an Aura waits for a floating pool, so Slick Imitator can be paid
				if ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && speedGrantAuras[ob.Face().Name] {
						pl := c.G.E.G.Players[c.D.Player]
						hold = pl.Speed >= 4 && poolSum(pl.Pool[:]) < 2
					}
				}
				if !hold {
					cast = append(cast, a)
				}
				if ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && speedGrantPay[ob.Face().Name] {
						payCasts = append(payCasts, a)
					} else if ob != nil && ob.Face() != nil && ob.Face().Name == "Slick Imitator" {
						slickCasts = append(slickCasts, a) // on the battlefield early: the copier
					} else if ob != nil && ob.Face() != nil && speedGrantAuras[ob.Face().Name] {
						auraCasts = append(auraCasts, a) // cast from the floating pool, leaving mana for Slick Imitator
					}
				}
			}
		}
		pick := func(opts []Canon) Canon { return opts[coin.IntN(len(opts))] }
		var a Canon
		switch {
		case len(auraTarget) > 0:
			a = pick(auraTarget)
		case len(slickCopy) > 0:
			a = pick(slickCopy)
		case len(granted) > 0:
			a = pick(granted)
		case len(payCasts) > 0 && c.G.E.G.Players[c.D.Player].Speed >= 4:
			a = pick(payCasts)
		case len(auraCasts) > 0 && c.G.E.G.Players[c.D.Player].Speed >= 4 && poolSum(c.G.E.G.Players[c.D.Player].Pool[:]) >= 2:
			a = pick(auraCasts)
		case len(slickCasts) > 0:
			a = pick(slickCasts)
		case len(attack) > 0 && coin.IntN(4) != 0:
			a = pick(attack)
		case len(cast) > 0 && coin.IntN(3) != 0:
			a = pick(cast)
		default:
			a = rnd.Pick(c)
		}
		if a.Kind() == ActCast && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Lightwheel Enhancements" && ob.Zone == state.ZGraveyard {
					tally.lwGrave++
				}
			}
		}
		if k := a.Kind(); k == ActActivate && a.Mode() >= maxSpeedBase && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					name := ob.Face().Name
					tally.played[name]++
					switch {
					case speedGrantSurveyors[name] && ob.Zone == state.ZGraveyard:
						tally.surveyorGrave++
					case speedGrantSurveyors[name]:
						tally.badZone = name
					case speedGrantBattlefield[name]:
						tally.bfGrant++
					}
				}
			}
		}
		return a
	})
}

// speedGrantGames plays seeds 1..n of the matchup and returns the first
// divergence or stall report ("" when none).
func speedGrantGames(t *testing.T, reg *cards.Registry, set cardset.Set, n uint64, tally *speedGrantTally) (games int, report string) {
	return speedGrantGamesOf(t, reg, set, speedGrantDecks, n, tally)
}

// speedGrantGamesOf is speedGrantGames over the given decks.
func speedGrantGamesOf(t *testing.T, reg *cards.Registry, set cardset.Set, sgDecks [2][]cardset.Entry, n uint64, tally *speedGrantTally) (games int, report string) {
	t.Helper()
	var decks [2][]*cards.Card
	for s, d := range sgDecks {
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
	tbl, _, _, err := set.Matchup(sgDecks[0], sgDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"speedgrant-a", "speedgrant-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, speedGrantPolicy(seed, tally), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			return games, "seed " + itoa(seed) + ": diverged at decision " + itoa(uint64(d.At)) + ": " + d.What + " " + d.Field + " " + fmt.Sprint(d.Diff) + " " + d.Detail + "\nref " + fmt.Sprint(d.RefOptions) + "\nker " + fmt.Sprint(d.KerOptions)
		}
		if tally.badZone != "" {
			return games, "seed " + itoa(seed) + ": gorge offered " + tally.badZone + "'s grant outside the graveyard"
		}
	}
	return games, ""
}

func speedGrantEnv(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("speedgrant")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func TestSpeedgrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set := speedGrantEnv(t)
	dirac.SpeedCounting = true
	defer func() { dirac.SpeedCounting = false }()
	before := dirac.SpeedCounts[dirac.SpeedCntGatedOffer].Load()
	tally := &speedGrantTally{played: map[string]int{}, copySeen: map[state.ObjID]bool{}}
	games, report := speedGrantGames(t, reg, set, 256, tally)
	if report != "" {
		t.Fatal(report)
	}
	offers := dirac.SpeedCounts[dirac.SpeedCntGatedOffer].Load() - before
	t.Logf("%d games, 0 divergences, %d gated offers, %d Surveyor graveyard activations, %d battlefield mana-costed grants, %d Lightwheel graveyard casts, %d Aura spell copies, plays %v",
		games, offers, tally.surveyorGrave, tally.bfGrant, tally.lwGrave, tally.auraCopies, tally.played)
	if offers < 40 {
		t.Errorf("only %d max-speed offers, want >= 40", offers)
	}
	if tally.surveyorGrave < 10 {
		t.Errorf("only %d Surveyor graveyard activations, want >= 10", tally.surveyorGrave)
	}
	if tally.bfGrant < 5 {
		t.Errorf("only %d battlefield mana-costed grant activations, want >= 5", tally.bfGrant)
	}
	if tally.lwGrave < 10 {
		t.Errorf("only %d Lightwheel graveyard casts, want >= 10", tally.lwGrave)
	}
	if tally.auraCopies < 1 {
		t.Errorf("only %d Aura spell copies, want >= 1", tally.auraCopies)
	}
}

// The kernel offering a grant when its lands could pay it (gorge prices the
// real offer from the floating pool alone) must diverge.
func TestSpeedgrantMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set := speedGrantEnv(t)
	dirac.DebugSpeedGrantLands = true
	defer func() { dirac.DebugSpeedGrantLands = false }()
	games, report := speedGrantGames(t, reg, set, 256, &speedGrantTally{played: map[string]int{}, copySeen: map[state.ObjID]bool{}})
	if report == "" {
		t.Fatalf("%d games with lands pricing the grant: no divergence", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, report)
}
