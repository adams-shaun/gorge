package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Demonstrate lockstep (CR 702.144a): the four Techniques' cast trigger
// ("copy it, then an opponent copies it") over the generated demo pool. The
// frozen FDN and BLB pair pools exercise no self-cast trigger, so this is the
// only coverage of onCast's stack walk, OpDemonstrate's election and the
// spell-copy mints it makes for both players.

// kdemoSpells is the Techniques deck: each target requirement (a nonland
// permanent, a card in your own graveyard, a permanent you control) is met
// by the vanilla creatures both decks carry.
var kdemoSpells = []cardset.Entry{
	{Name: "Excavation Technique", Count: 3}, {Name: "Healing Technique", Count: 3},
	{Name: "Incarnation Technique", Count: 2}, {Name: "Replication Technique", Count: 2},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
	{Name: "Plains", Count: 8}, {Name: "Forest", Count: 7}, {Name: "Swamp", Count: 4}, {Name: "Island", Count: 5},
}

// kdemoVanilla is the no-Techniques deck: bodies to target and to die.
var kdemoVanilla = []cardset.Entry{
	{Name: "Gray Ogre", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Coral Merfolk", Count: 3},
	{Name: "Giant Spider", Count: 3}, {Name: "Walking Corpse", Count: 3},
	{Name: "Plains", Count: 5}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 5},
	{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
}

// kdemoPolicy casts or activates whenever it can (so the Techniques are cast
// and the graveyards fill), answers yes to every yes/no ask (the demonstrate
// election and Incarnation Technique's hand-move confirm), and is otherwise
// random.
func kdemoPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		var yes Canon
		hasYes, hasNo := false, false
		for _, a := range c.D.Options {
			switch {
			case a.Kind() == ActCast || a.Kind() == ActActivate:
				buf = append(buf, a)
			case a.Kind() == ActChoose:
				yes, hasYes = a, true
			case a.Kind() == ActChoose+1:
				hasNo = true
			}
		}
		if hasYes && hasNo && len(c.D.Options) == 2 {
			return yes
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// kdemoTally watches the kernel's spell copies: a stack object flagged
// ProjCopy is a demonstrate copy (no other card in the pool makes one). A
// fresh copy is a yes answer (each yes mints the caster's copy and the
// opponent's, one per controller); a copy that leaves the stack resolved.
type kdemoTally struct {
	live     map[int]uint8 // kernel object index -> copy controller
	minted   [2]int
	resolved [2]int
}

func (t *kdemoTally) check(_ *GorgeSide, k Side) *Divergence {
	r, ok := k.(*realSide)
	if !ok {
		return nil
	}
	kp := &r.kp
	seen := make(map[int]bool)
	for i := 0; i < int(kp.NObj); i++ {
		o := &kp.Objs[i]
		if o.Zone != kop.ZoneStack || o.Flags&dirac.ProjCopy == 0 {
			continue
		}
		seen[i] = true
		ctrl := o.Ctrl & 1
		if _, ok := t.live[i]; !ok {
			t.live[i] = ctrl
			t.minted[ctrl]++
		}
	}
	for i, ctrl := range t.live {
		if !seen[i] {
			t.resolved[ctrl]++
			delete(t.live, i)
		}
	}
	return nil
}

func TestDemoLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("demo")
	if err != nil {
		t.Fatal(err)
	}
	// matchup 0 has the Techniques in seat 0's deck, matchup 1 in seat 1's,
	// so a yes from each controller is proven separately.
	matchups := [][2][]cardset.Entry{{kdemoSpells, kdemoVanilla}, {kdemoVanilla, kdemoSpells}}
	games, yeses, resolved := 0, 0, 0
	for mi, md := range matchups {
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
		for seed := uint64(1); seed <= 24; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kdemo-a", "kdemo-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			tal := &kdemoTally{live: map[int]uint8{}}
			res := Lockstep(m, seed, k, kdemoPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: tal.check})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if res.Stall != "" && strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			yeses += tal.minted[mi] // the demonstrating seat's copies
			resolved += tal.resolved[1-mi]
		}
	}
	t.Logf("%d games, %d yes answers, %d opponent copies resolved", games, yeses, resolved)
	if yeses < 4 {
		t.Fatalf("%d demonstrate yes answers, want at least 4", yeses)
	}
	if resolved < 4 {
		t.Fatalf("%d opponent copies resolved, want at least 4", resolved)
	}
}
