package oracle

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The SHIELD counter lockstep (shield, CR 122.1c): the shield-counter
// carriers against burn (Shock, Lightning Bolt, Pyroclasm, Prodigal
// Pyromancer), destruction (Murder, Doom Blade, Wrath of God, Day of
// Judgment) and big attackers and blockers (deathtouch Typhoid Rats,
// double strike Disciplined Duelist). No FDN-era "damage can't be
// prevented" card builds (the kernel has no CantPreventDamage reader), so
// none is in the decks. Family's Favor's own RemoveCounter is the one
// non-replacement removal; it sits in matchup 0 only, so matchup 1's
// non-damage spends are destructions.

var kshieldDecks = [][2][]cardset.Entry{{
	{
		{Name: "Captain America, Super-Soldier", Count: 2}, {Name: "Dapper Shieldmate", Count: 2}, {Name: "Swooping Protector", Count: 2},
		{Name: "Boon of Safety", Count: 3}, {Name: "Protection Magic", Count: 2}, {Name: "Proud Pack-Rhino", Count: 2},
		{Name: "Brotherhood Outcast", Count: 2}, {Name: "Rhox Pummeler", Count: 2}, {Name: "Voice of the Vermin", Count: 2},
		{Name: "Family's Favor", Count: 2}, {Name: "Ajani's Pridemate", Count: 1}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 7}, {Name: "Forest", Count: 7},
	},
	{
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Murder", Count: 2}, {Name: "Doom Blade", Count: 2},
		{Name: "Pyroclasm", Count: 2}, {Name: "Prodigal Pyromancer", Count: 2}, {Name: "Typhoid Rats", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Gray Ogre", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
	},
}, {
	{
		{Name: "Brokers Veteran", Count: 3}, {Name: "Wingshield Agent", Count: 3}, {Name: "Singer of Swift Rivers", Count: 3},
		{Name: "Disciplined Duelist", Count: 2}, {Name: "Rhox Pummeler", Count: 2}, {Name: "Voice of the Vermin", Count: 2},
		{Name: "Craw Wurm", Count: 2}, {Name: "Giant Spider", Count: 2}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 7}, {Name: "Plains", Count: 3},
	},
	{
		{Name: "Wrath of God", Count: 2}, {Name: "Day of Judgment", Count: 2}, {Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Prodigal Pyromancer", Count: 2}, {Name: "Serra Angel", Count: 2}, {Name: "Hill Giant", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Prickleboar", Count: 2}, {Name: "Boon of Safety", Count: 2},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8},
	},
}}

// kshieldCollisionDecks is the seed-272 repro for the shield: the
// recipient's granted prevention (Fleeting Flight on a Crystal Barricade
// creature) and a Twinflame Tyrant doubler match the same lethal combat
// damage, so gorge asks the CR 616.1 order before the loss is settled.
var kshieldCollisionDecks = [][2][]cardset.Entry{{
	{
		{Name: "Captain America, Super-Soldier", Count: 2}, {Name: "Dapper Shieldmate", Count: 2}, {Name: "Swooping Protector", Count: 2},
		{Name: "Boon of Safety", Count: 3}, {Name: "Protection Magic", Count: 2}, {Name: "Proud Pack-Rhino", Count: 2},
		{Name: "Brotherhood Outcast", Count: 2}, {Name: "Rhox Pummeler", Count: 2}, {Name: "Voice of the Vermin", Count: 2},
		{Name: "Family's Favor", Count: 2}, {Name: "Ajani's Pridemate", Count: 1}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Savannah Lions", Count: 1}, {Name: "Crystal Barricade", Count: 1}, {Name: "Fleeting Flight", Count: 1},
		{Name: "Plains", Count: 7}, {Name: "Forest", Count: 7},
	},
	{
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Murder", Count: 2}, {Name: "Doom Blade", Count: 2},
		{Name: "Pyroclasm", Count: 2}, {Name: "Prodigal Pyromancer", Count: 2}, {Name: "Typhoid Rats", Count: 2},
		{Name: "Hill Giant", Count: 1}, {Name: "Gray Ogre", Count: 2}, {Name: "Twinflame Tyrant", Count: 1},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
	},
}}

// kshieldCounter counts gorge's SHIELD removals (CounterChange SHIELD -1):
// after the prevention Note ("damage prevented by replacement effect") a
// damage spend, otherwise a destruction (or Family's Favor's removal).
type kshieldCounter struct {
	dmg, other int
	evAt       int
}

func (c *kshieldCounter) check(ref *GorgeSide, _ Side) *Divergence {
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt = 0
	}
	for j := c.evAt; j < len(evs); j++ {
		if e := &evs[j]; e.Kind == events.CounterChange && e.Counter == "SHIELD" && e.Amount < 0 {
			if j > 0 && evs[j-1].Kind == events.Note && evs[j-1].Text == "damage prevented by replacement effect" {
				c.dmg++
			} else {
				c.other++
			}
		}
	}
	c.evAt = len(evs)
	return nil
}

type kshieldEnv struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [][2][]*cards.Card
	tbls  []*kop.Table
}

func newKShieldEnv(t *testing.T) *kshieldEnv {
	t.Helper()
	return newKShieldEnvDecks(t, kshieldDecks)
}

func newKShieldEnvDecks(t *testing.T, deckDefs [][2][]cardset.Entry) *kshieldEnv {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("shield")
	if err != nil {
		t.Fatal(err)
	}
	env := &kshieldEnv{reg: reg, set: set}
	for mi, md := range deckDefs {
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
		env.decks = append(env.decks, decks)
		env.tbls = append(env.tbls, tbl)
	}
	return env
}

// game plays matchup mi on seed: even seeds on gorge's autopay surface,
// odd on the lowering surface; mixedarmsPolicy (it prefers casting and
// attacking), every fifth seed the plain random policy.
func (env *kshieldEnv) game(mi int, seed uint64, check func(*GorgeSide, Side) *Divergence) (Result, error) {
	m, err := NewMatchup(env.reg, [2]string{"kshield-a", "kshield-b"}, env.decks[mi], 1)
	if err != nil {
		return Result{}, err
	}
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	k, err := SetSide(env.set, env.tbls[mi], cfg)(m, seed)
	if err != nil {
		return Result{}, err
	}
	pol := mixedarmsPolicy(seed)
	if seed%5 == 0 {
		pol = RandomPolicy(seed)
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
	CloseSide(k)
	return res, nil
}

// TestShieldLockstep plays both matchups on seeds 1..64 (KSHIELD_SEEDS=n)
// with zero divergences and asserts at least 30 SHIELD counters spent,
// both by damage and by destruction.
func TestShieldLockstep(t *testing.T) {
	t.Parallel()

	env := newKShieldEnv(t)
	seeds := uint64(64) // ~5 s; 400 seeds (800 games) ran clean at shield
	if v := os.Getenv("KSHIELD_SEEDS"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		seeds = n
	}
	stalls := map[string]int{}
	games := 0
	var per [2]kshieldCounter
	for mi := range kshieldDecks {
		for seed := uint64(1); seed <= seeds; seed++ {
			c := kshieldCounter{}
			res, err := env.game(mi, seed, c.check)
			if err != nil {
				t.Fatal(err)
			}
			games++
			per[mi].dmg += c.dmg
			per[mi].other += c.other
			if res.Stall != "" {
				stalls[res.Stall]++
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	var sk []string
	for s, n := range stalls {
		sk = append(sk, s+"="+strconv.Itoa(n))
	}
	sort.Strings(sk)
	t.Logf("%d games, stalls [%s]; SHIELD spent: matchup 0 damage %d other %d, matchup 1 damage %d destroy %d",
		games, strings.Join(sk, " "), per[0].dmg, per[0].other, per[1].dmg, per[1].other)
	if os.Getenv("KSHIELD_SEEDS") != "" {
		return
	}
	dmg, des := per[0].dmg+per[1].dmg, per[1].other
	if dmg+per[0].other+des < 30 || dmg < 10 || des < 3 {
		t.Errorf("SHIELD spent: %d by damage, %d by destruction (matchup 1), %d other: want >= 30 in all, >= 10 by damage, >= 3 by destruction",
			dmg, des, per[0].other)
	}
}

// TestShieldReplacementCollision plays the collision decks. Seed 187
// reaches the CR 616.1 order ask with two options sharing one source (a
// shield counter and the recipient's granted combat-damage prevention), so
// the gorge-side canon must pack the option's occurrence. Seed 272 is the
// lethal pass: the SBA runs before the ask is posed after the pass dealt
// its events, so the loss is settled exactly as gorge settles it.
func TestShieldReplacementCollision(t *testing.T) {
	t.Parallel()

	env := newKShieldEnvDecks(t, kshieldCollisionDecks)
	asks := 0
	check := func(g *GorgeSide, _ Side) *Divergence {
		d := g.E.Pending()
		if d == nil || d.Kind != decision.KReplacement {
			return nil
		}
		n := 0
		seen := map[state.ObjID]int{}
		for i := range d.Options {
			o := &d.Options[i]
			if o.Kind != "replacement" {
				continue
			}
			n++
			seen[o.Obj]++
		}
		if n < 2 {
			return nil
		}
		for id, c := range seen {
			if c <= 1 {
				continue
			}
			asks++
			var modes []uint8
			for i := range d.Options {
				o := &d.Options[i]
				if o.Kind != "replacement" || o.Obj != id {
					continue
				}
				a, ok := g.optionAction(d, o)
				if !ok {
					t.Errorf("seed 187: the %s replacement option has no canon", o.Kind)
					continue
				}
				modes = append(modes, a.Mode())
			}
			if len(modes) < 2 || modes[0] != 0 || modes[1] != 1 {
				t.Errorf("seed 187: same-source replacement canon modes %v, want [0 1]", modes)
			}
			break
		}
		return nil
	}
	res, err := env.game(0, 187, check)
	if err != nil {
		t.Fatal(err)
	}
	if res.Div != nil {
		lo := max(res.Div.At-12, 0)
		t.Logf("last actions %v", actionStrings(res.Actions[lo:res.Div.At]))
		t.Fatalf("seed 187 diverged at decision %d: %s %s %v %s", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
	}
	if asks == 0 {
		t.Fatal("the same-source shield/prevention replacement-order ask was never reached")
	}
	res, err = env.game(0, 272, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Div != nil {
		lo := max(res.Div.At-12, 0)
		t.Logf("last actions %v", actionStrings(res.Actions[lo:res.Div.At]))
		t.Fatalf("seed 272 diverged at decision %d: %s %s %v %s", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
	}
	if !res.Over || res.Winner != 0 {
		t.Fatalf("seed 272: over=%v winner=%d, want gorge to end the game at the lethal pass", res.Over, res.Winner)
	}
}

// TestShieldMutation: with the shield reader switched off
// (dirac.DebugIgnoreShield) the same games diverge.
func TestShieldMutation(t *testing.T) {
	env := newKShieldEnv(t)
	dirac.DebugIgnoreShield = true
	defer func() { dirac.DebugIgnoreShield = false }()
	for mi := range kshieldDecks {
		for seed := uint64(1); seed <= 8; seed++ {
			res, err := env.game(mi, seed, nil)
			if err != nil {
				t.Fatal(err)
			}
			if res.Div != nil {
				t.Logf("matchup %d seed %d: the mutation diverges at decision %d (%s %s)", mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				return
			}
		}
	}
	t.Fatal("ignoring SHIELD counters diverged in no game: the lockstep does not see the reader")
}
