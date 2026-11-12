package oracle

import (
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The monarch lockstep (CR 725): every card the BecomeMonarch lowering lifted,
// in two matchups of the monarch pool, so both players take the crown (an
// ETB, Throne of the High City's activation, Éomer's target player, Oath of
// Eorl's chapter, Feast of Succession, Regal Sliver's granted trigger), draw at
// their end step, steal it back with combat damage, and read it (Count$Monarch,
// CheckDefinedPlayer$ You/Opponent.isMonarch, Aragorn's gated Effect, the
// BecomeMonarch triggers of Custodi Lich and Knights of the Black Rose). The
// expected answer at every decision is gorge's.

var kmonarchMatchups = [2][2][]cardset.Entry{
	{ // the Commander-precon cards
		{ // A: W/B/R makers and readers
			{Name: "Throne of the High City", Count: 2}, {Name: "Court of Grace", Count: 2}, {Name: "Custodi Lich", Count: 2},
			{Name: "Knights of the Black Rose", Count: 2}, {Name: "Queen Marchesa", Count: 2}, {Name: "Marchesa's Decree", Count: 1},
			{Name: "Grave Venerations", Count: 1}, {Name: "Éomer, King of Rohan", Count: 1},
			{Name: "Savannah Lions", Count: 3}, {Name: "Elite Vanguard", Count: 2}, {Name: "Walking Corpse", Count: 2},
			{Name: "Goblin Piker", Count: 2}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Plains", Count: 6}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
		},
		{ // B: U/R/W makers and evasive stealers
			{Name: "Throne of the High City", Count: 2}, {Name: "Court of Ire", Count: 2}, {Name: "Faramir, Steward of Gondor", Count: 2},
			{Name: "Aragorn, King of Gondor", Count: 2}, {Name: "Oath of Eorl", Count: 1}, {Name: "Skyline Despot", Count: 1},
			{Name: "Suntail Hawk", Count: 3}, {Name: "Wind Drake", Count: 2}, {Name: "Hill Giant", Count: 2},
			{Name: "Savannah Lions", Count: 2}, {Name: "Shock", Count: 2}, {Name: "Lightning Bolt", Count: 1},
			{Name: "Plains", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 6},
		},
	},
	{ // the rest of the corpus cards the lowering lifted
		{ // C: W/R
			{Name: "Chief Magistrate of Mercadia", Count: 1}, {Name: "Court of Ardenvale", Count: 2}, {Name: "Court of Embereth", Count: 2},
			{Name: "Crimson Fleet Commodore", Count: 2}, {Name: "Palace Sentinels", Count: 2}, {Name: "Regal Sliver", Count: 2},
			{Name: "Throne Warden", Count: 2}, {Name: "Staunch Throneguard", Count: 1},
			{Name: "Savannah Lions", Count: 3}, {Name: "Goblin Piker", Count: 2}, {Name: "Shock", Count: 2},
			{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 9},
		},
		{ // D: U/B/G
			{Name: "Court of Bounty", Count: 2}, {Name: "Court of Cunning", Count: 2}, {Name: "Court of Garenbrig", Count: 2},
			{Name: "Feast of Succession", Count: 1}, {Name: "Garrulous Sycophant", Count: 2}, {Name: "Keeper of Keys", Count: 2},
			{Name: "Thorn of the Black Rose", Count: 2}, {Name: "Nakia, Wakandan Operative", Count: 1},
			{Name: "Grizzly Bears", Count: 3}, {Name: "Walking Corpse", Count: 2}, {Name: "Wind Drake", Count: 2},
			{Name: "Forest", Count: 7}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 6},
		},
	},
}

// kmonarchProbe records, on gorge's side, what the games played: the cards seen
// on the battlefield or cast, and the abilities seen on the stack by their
// source's name and gorge's ability index (activated: Face.Abilities index;
// triggered: 0x80|Face.Triggers index; the monarch's own source-less
// abilities: "(monarch)").
type kmonarchProbe struct {
	played map[string]int
	abs    map[string]map[uint8]int
}

func (p *kmonarchProbe) observe(gs *GorgeSide) {
	g := gs.E.G
	for s := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(s)) {
			if o := g.Obj(id); o != nil && o.Face() != nil {
				p.played[o.Face().Name]++
			}
		}
	}
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil {
			continue
		}
		if o.Ability == nil || o.Card != nil {
			if o.Face() != nil {
				p.played[o.Face().Name]++ // a spell
			}
			continue
		}
		name, ix := "(monarch)", uint8(255)
		if src := g.Obj(o.Source); o.Source != 0 && src != nil && src.Face() != nil {
			name, ix = src.Face().Name, abilityIndex(src.Face(), o.Ability)
		}
		if p.abs[name] == nil {
			p.abs[name] = map[uint8]int{}
		}
		p.abs[name][ix]++
	}
}

// monarchSeeds is the seed count per matchup, KMONARCH_SEEDS overriding it.
func monarchSeeds(def uint64) uint64 {
	if v := os.Getenv("KMONARCH_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

// TestMonarchLockstep: every decision of every seed matches gorge, in both
// matchups, and every lifted card and the designation's every path were played.
func TestMonarchLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := &dirac.MonarchStats{}
	dirac.DebugMonarchStats = st
	defer func() { dirac.DebugMonarchStats = nil }()
	probe := &kmonarchProbe{played: map[string]int{}, abs: map[string]map[uint8]int{}}
	policy := func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				probe.observe(c.G)
			}
			return base.Pick(c)
		})
	}
	seeds := monarchSeeds(160)
	for i, decks := range kmonarchMatchups {
		if div := lockstepRunPolicy(t, "monarch", decks, seeds, true, nil, nil, policy); div != 0 {
			t.Errorf("matchup %d: %d of %d seeds diverged", i, div, seeds)
		}
	}
	t.Logf("kernel: %+v", *st)
	for _, c := range []struct {
		n    int
		what string
	}{
		{st.Became, "the designation moved"}, {st.Repeat, "the monarch became it again"}, {st.Draws, "end-step draw triggers"},
		{st.Steals, "combat-damage steal triggers"}, {st.StealMove, "steals that moved the designation"},
		{st.Fires, "BecomeMonarch triggers"}, {st.ExYes, "monarch reads that held"}, {st.ExNo, "monarch reads that failed"},
	} {
		if c.n == 0 {
			t.Errorf("%s: none", c.what)
		}
	}
	var names []string
	for n := range probe.abs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		t.Logf("stack abilities of %s: %v", n, probe.abs[n])
	}
	// every lifted card was played (a land: put onto the battlefield)
	lifted := []string{
		"Aragorn, King of Gondor", "Chief Magistrate of Mercadia", "Court of Ardenvale", "Court of Bounty", "Court of Cunning",
		"Court of Embereth", "Court of Garenbrig", "Court of Grace", "Court of Ire", "Crimson Fleet Commodore", "Custodi Lich",
		"Faramir, Steward of Gondor", "Feast of Succession", "Garrulous Sycophant", "Grave Venerations", "Keeper of Keys",
		"Knights of the Black Rose", "Marchesa's Decree", "Nakia, Wakandan Operative", "Oath of Eorl", "Palace Sentinels",
		"Queen Marchesa", "Regal Sliver", "Skyline Despot", "Staunch Throneguard", "Thorn of the Black Rose", "Throne Warden",
		"Throne of the High City", "Éomer, King of Rohan",
	}
	for _, n := range lifted {
		if probe.played[n] == 0 {
			t.Errorf("%s: never played", n)
		}
	}
	// the abilities the lowering lifted, by gorge's index: the designation's
	// own two, Throne's activation and each reader's trigger
	for _, a := range []struct {
		name string
		ix   uint8
	}{
		{"(monarch)", 255}, {"Throne of the High City", 1}, {"Court of Grace", 0x81}, {"Court of Ire", 0x81},
		{"Custodi Lich", 0x81}, {"Knights of the Black Rose", 0x81}, {"Queen Marchesa", 0x81}, {"Aragorn, King of Gondor", 0x81},
		{"Éomer, King of Rohan", 0x80}, {"Faramir, Steward of Gondor", 0x80}, {"Faramir, Steward of Gondor", 0x81},
		{"Grave Venerations", 0x81}, {"Skyline Despot", 0x81},
	} {
		if probe.abs[a.name][a.ix] == 0 {
			t.Errorf("%s ability %#x: never on the stack", a.name, a.ix)
		}
	}
}

// TestMonarchLockstepMutation proves the pool steals the crown with combat
// damage: with the steal trigger dropped, some seed diverges.
func TestMonarchLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoMonarchSteal = true
	defer func() { dirac.DebugNoMonarchSteal = false }()
	if !lockstepDiverges(t, "monarch", kmonarchMatchups[0], 60) {
		t.Fatal("no seed diverged with the combat-damage steal dropped: the pool never steals the crown")
	}
}

// kmonarchCommander is a Commander duel (CR 903) of the monarch pool, each
// deck's first card its commander: Nakia, Wakandan Operative's "whenever your
// commander enters" and Faramir, Steward of Gondor's legendary entry make
// Aragorn's controller the monarch as the commander enters.
var kmonarchCommander = [2][]cardset.Entry{
	{
		{Name: "Aragorn, King of Gondor", Count: 1}, {Name: "Nakia, Wakandan Operative", Count: 3}, {Name: "Faramir, Steward of Gondor", Count: 2},
		{Name: "Throne of the High City", Count: 2}, {Name: "Court of Grace", Count: 2}, {Name: "Suntail Hawk", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Queen Marchesa", Count: 1}, {Name: "Knights of the Black Rose", Count: 2}, {Name: "Custodi Lich", Count: 2},
		{Name: "Court of Ire", Count: 2}, {Name: "Throne of the High City", Count: 2}, {Name: "Savannah Lions", Count: 3},
		{Name: "Walking Corpse", Count: 3}, {Name: "Goblin Piker", Count: 3}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
	},
}

// TestMonarchCommanderLockstep plays kmonarchCommander in lockstep, both
// seat orders: every decision matches gorge, and Nakia's commander trigger
// went on the stack.
func TestMonarchCommanderLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("monarch")
	if err != nil {
		t.Fatal(err)
	}
	probe := &kmonarchProbe{played: map[string]int{}, abs: map[string]map[uint8]int{}}
	seeds := monarchSeeds(160) / 2
	clean := 0
	for order := 0; order < 2; order++ {
		md := [2][]cardset.Entry{kmonarchCommander[order], kmonarchCommander[1-order]}
		var gd [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					gd[s] = append(gd[s], c)
				}
			}
			if len(gd[s]) != 40 {
				t.Fatalf("order %d deck %d: %d cards", order, s, len(gd[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			m, err := NewMatchup(reg, [2]string{"kmonarch-cmd-a", "kmonarch-cmd-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			m.Commander, m.Commanders = true, [2]int{0, 0}
			cfg := Config{GorgeAutopaySurface: seed%4 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			base := castFirstPolicy(seed)
			pol := PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil {
					probe.observe(c.G)
				}
				return base.Pick(c)
			})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			if err := unmodelledStallError(t.Name(), &res); err != nil {
				t.Errorf("order %d seed %d: %v", order, seed, err)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("order %d seed %d last actions %v", order, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("order %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", order, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				continue
			}
			clean++
		}
	}
	t.Logf("commander duels: %d of %d clean; Nakia %v, Faramir %v", clean, 2*seeds, probe.abs["Nakia, Wakandan Operative"], probe.abs["Faramir, Steward of Gondor"])
	if probe.abs["Nakia, Wakandan Operative"][0x80] == 0 {
		t.Error("Nakia's commander trigger never went on the stack")
	}
}
