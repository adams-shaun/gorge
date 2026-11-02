package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// gorgePhyActFixTime is gorge cr-phyrexian-activation-low-life (81fb97473),
// the commit time in UTC: before it gorge offered a Phyrexian activation only
// while the activator's life covered two per pip, even with the colour
// payable (CR 107.4f, CR 119.4); the kernel offers it whenever some colour /
// life mix is payable.
const gorgePhyActFixTime = "20261026091120"

func skipBeforePhyActFix(t *testing.T) {
	t.Helper()
	if !gorgePinAtLeast(t, gorgePhyActFixTime) {
		t.Skip("gorge pin predates cr-phyrexian-activation-low-life (81fb97473): a Phyrexian activation needs life for every pip")
	}
}

// The Phyrexian activation lockstep (CR 107.4f): the Souleaters'
// {C/P} abilities (tap, +1/+0, trample, unblockable), Moltensteel Dragon's
// firebreathing, and the MOM transform cards' {C/P} transforms (Order of the
// Mirror, Khenra Spellspear, Mite Overseer's token). Each pip is announced
// as its colour or two life, the life faces paid with the activation; one
// deck holds the colours, the other few of them, and burn lowers life
// toward the point where the life face is no longer offered. The expected
// answer at every decision is gorge's.
var kphyactDecks = [2][]cardset.Entry{
	{
		{Name: "Immolating Souleater", Count: 4}, {Name: "Insatiable Souleater", Count: 3}, {Name: "Moltensteel Dragon", Count: 3},
		{Name: "Khenra Spellspear", Count: 3}, {Name: "Order of the Mirror", Count: 3}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 4}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Blinding Souleater", Count: 4}, {Name: "Trespassing Souleater", Count: 4}, {Name: "Mite Overseer", Count: 3},
		{Name: "Savannah Lions", Count: 4}, {Name: "Wind Drake", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
}

// kphyactDecksB holds the other activation shapes: an equip cost of two
// pips (Lashwrithe, living weapon), a pip beside a tap and a sacrifice
// (Birthing Pod), two pips beside a sacrifice of two (Zopandrel), and more
// MOM transforms. Hex Parasite (an {X} beside the pip) is refused at emit:
// twobrid with {X}.
var kphyactDecksB = [2][]cardset.Entry{
	{
		{Name: "Nezumi Freewheeler", Count: 4}, {Name: "Lashwrithe", Count: 3}, {Name: "Blightreaper Thallid", Count: 4},
		{Name: "Birthing Pod", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Wind Drake", Count: 2}, {Name: "Hill Giant", Count: 2}, {Name: "Tarkir Duneshaper", Count: 3},
		{Name: "Swamp", Count: 7}, {Name: "Plains", Count: 3}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Pyretic Prankster", Count: 4}, {Name: "Seraph of New Capenna", Count: 3}, {Name: "Zopandrel, Hunger Dominus", Count: 2},
		{Name: "Mite Overseer", Count: 3}, {Name: "Savannah Lions", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 3},
		{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	},
}

func TestPhyactLockstepB(t *testing.T) {
	t.Parallel()
	skipBeforePhyActFix(t)
	lockstepSet(t, "phyact", kphyactDecksB, 1000, "KPHYACT_SEEDS")
}

func TestPhyactLockstep(t *testing.T) {
	t.Parallel()
	skipBeforePhyActFix(t)
	lockstepSet(t, "phyact", kphyactDecks, 1000, "KPHYACT_SEEDS")
}

// TestPhyactLockstepMutation proves the pool pays activation pips with
// life: without the payment, some seed diverges.
func TestPhyactLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	skipBeforePhyActFix(t)
	dirac.DebugNoPhyrexianLife = true
	defer func() { dirac.DebugNoPhyrexianLife = false }()
	if !lockstepDiverges(t, "phyact", kphyactDecks, 40) {
		t.Fatal("no seed diverged without the two-life payment: the pool never pays an activation pip with life")
	}
}

// kphyactLowLifeDecks: the Souleaters' single {C/P} pips beside their
// colour's basics, against a burn deck that drives their controller low.
var kphyactLowLifeDecks = [2][]cardset.Entry{
	{
		{Name: "Immolating Souleater", Count: 4}, {Name: "Insatiable Souleater", Count: 4}, {Name: "Blinding Souleater", Count: 4},
		{Name: "Trespassing Souleater", Count: 4}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
		{Name: "Plains", Count: 6}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

// TestPhyactLowLifeLockstep plays the CR 119.4 edge: a Phyrexian activation
// at 1 life, which only an untapped source of the pip's colour can pay
// (gorge offered nothing there before cr-phyrexian-activation-low-life). The
// policy takes such an activation whenever gorge offers one at 1 life; the
// games must stay in lockstep and at least one must reach it.
func TestPhyactLowLifeLockstep(t *testing.T) {
	t.Parallel()
	skipBeforePhyActFix(t)
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("phyact")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kphyactLowLifeDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kphyactLowLifeDecks[0], kphyactLowLifeDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	souleater := map[string]bool{"Immolating Souleater": true, "Insatiable Souleater": true, "Blinding Souleater": true, "Trespassing Souleater": true}
	seeds, reached := uint64(400), 0
	for seed := uint64(1); seed <= seeds && reached < 5; seed++ {
		m, err := NewMatchup(reg, [2]string{"kphyact-a", "kphyact-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%2 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		base := castFirstPolicy(seed)
		hit := false
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority && c.G.E.G.Players[c.D.Player].Life == 1 {
				for _, a := range c.D.Options {
					if a.Kind() != ActActivate {
						continue
					}
					if opt, ok := c.G.GorgeOption(a); ok {
						if o := c.G.E.G.Obj(opt.Obj); o != nil && o.Zone == state.ZBattlefield && souleater[o.Face().Name] {
							hit = true
							return a
						}
					}
				}
			}
			return base.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nlast %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail,
				actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
		}
		if hit {
			reached++
		}
	}
	if reached == 0 {
		t.Fatalf("no seed reached a Souleater activation at 1 life in %d games", seeds)
	}
	t.Logf("%d games activated a Souleater at 1 life, all in lockstep", reached)
}
