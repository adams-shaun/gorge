package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Myriad lockstep (CR 702.116a-b): "whenever this creature attacks, for
// each opponent other than defending player, you may create a token copy
// tapped and attacking that player". With two players there is no such
// opponent, so each instance goes on the stack, both players get priority,
// and it resolves asking nothing. Every corpus card the kernel lowers with
// the keyword plays here: the printed K:Myriad carriers (Scurry of Squirrels
// prints it twice; Hammers of Moradin beside its own attack trigger) and the
// three statics that grant it (Blade of Selves to the equipped creature,
// Legion Loyalty and Cybermen Squadron to their controller's creatures), whose
// granted instances gorge queues as __kwMyriadGranted pushes.

var kmyriadDecks = [2][]cardset.Entry{
	{
		{Name: "Wyrm's Crossing Patrol", Count: 2}, {Name: "Hammers of Moradin", Count: 2},
		{Name: "Herald of the Host", Count: 1}, {Name: "Tabaxi Toucaneers", Count: 1},
		{Name: "Legion Loyalty", Count: 1}, {Name: "Genasi Enforcers", Count: 2},
		{Name: "Warchief Giant", Count: 1}, {Name: "Tiamat's Fanatics", Count: 1},
		{Name: "Goldlust Triad", Count: 1}, {Name: "Dalek Squadron", Count: 2},
		{Name: "Banshee of the Dread Choir", Count: 1}, {Name: "Blade of Selves", Count: 2},
		{Name: "Cybermen Squadron", Count: 1},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 7},
	},
	{
		{Name: "Chittering Dispatcher", Count: 2}, {Name: "Polygoyf", Count: 2},
		{Name: "Scurry of Squirrels", Count: 2}, {Name: "Broodbirth Viper", Count: 1},
		{Name: "Wizards of Thay", Count: 1}, {Name: "Conclave Evangelist", Count: 2},
		{Name: "Sumala Rumblers", Count: 2}, {Name: "Scion of Calamity", Count: 2},
		{Name: "Caller of the Pack", Count: 1}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Forest", Count: 15}, {Name: "Island", Count: 7},
	},
}

// kmyriadGrantors are the statics that grant the keyword.
var kmyriadGrantors = []string{"Blade of Selves", "Legion Loyalty", "Cybermen Squadron"}

// kmyriadPolicy attacks and blocks whenever it can (or stops declaring), and
// otherwise casts, activates or plays a land when it can, so the Myriad
// carriers are cast, equipped and sent in.
func kmyriadPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	pick := func(c *PickCtx) Canon {
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		var done Canon
		haveDone := false
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActAttack, ActBlock:
				buf = append(buf, a)
			case ActDone:
				done, haveDone = a, true
			}
		}
		if len(buf) > 0 {
			if haveDone && len(buf) > 1 {
				buf = append(buf, done)
			}
			return pick(c)
		}
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		return pick(c)
	})
}

// kmyriadScan counts, in gorge's game, the Myriad instances that went on the
// stack: by the attacking creature's name for every instance, and by the
// granting permanent's name for the granted ones.
func kmyriadScan(g *GorgeSide, byCard, byGrantor map[string]int) {
	if g == nil {
		return
	}
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return "?"
	}
	for i := range g.E.G.Objs {
		if o := &g.E.G.Objs[i]; o.Ability != nil && o.Ability.API == "Myriad" {
			byCard[name(o.Source)]++
		}
	}
	for _, ev := range g.E.L.Events {
		if ev.Kind == events.GrantTriggerPush && ev.Counter == "__kwMyriadGranted" {
			byGrantor[name(state.ObjID(ev.Amount))]++
		}
	}
}

func runMyriad(t *testing.T, seeds uint64) (games, stalls int, byCard, byGrantor map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("myriad")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kmyriadDecks {
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
	tbl, _, _, err := set.Matchup(kmyriadDecks[0], kmyriadDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	byCard, byGrantor = map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"myriad-a", "myriad-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kmyriadPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 25})
		CloseSide(k)
		games++
		kmyriadScan(g, byCard, byGrantor)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

// kmyriadMin is the fewest Myriad instances each carrier (and each grantor)
// must put on the stack across the run.
const kmyriadMin = 10

func TestMyriadLockstep(t *testing.T) {
	t.Parallel()

	games, stalls, byCard, byGrantor := runMyriad(t, 200)
	counts := func(m map[string]int) string {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var out []string
		for _, k := range keys {
			out = append(out, fmt.Sprintf("%s=%d", k, m[k]))
		}
		return strings.Join(out, " ")
	}
	t.Logf("%d games, %d stalls; Myriad instances by attacker: %s; granted by: %s", games, stalls, counts(byCard), counts(byGrantor))
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	// the precondition: every carrier's Myriad, and every grant, reached the
	// stack in both engines
	for s, d := range kmyriadDecks {
		for _, e := range d {
			switch e.Name {
			case "Plains", "Island", "Swamp", "Mountain", "Forest", "Llanowar Elves", "Blade of Selves", "Legion Loyalty":
				continue
			}
			if byCard[e.Name] < kmyriadMin {
				t.Errorf("deck %d: %s attacked with Myriad %d times, want at least %d", s, e.Name, byCard[e.Name], kmyriadMin)
			}
		}
	}
	for _, n := range kmyriadGrantors {
		if byGrantor[n] < kmyriadMin {
			t.Errorf("%s granted Myriad instances %d, want at least %d", n, byGrantor[n], kmyriadMin)
		}
	}
}
