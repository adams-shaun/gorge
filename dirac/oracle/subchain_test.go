package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The copied spell's SubAbility$ chain lockstep
// (CR 707.10c, gorge): Mirrorpool's "Copy target instant or sorcery
// spell you control. You may choose new targets for the copy" copies Vibrant
// Outburst ("deals 3 damage to any target. Tap up to one target creature."),
// whose tap link is a second declaration. The copy's controller is asked
// both: the any-target first, then the tap target (the original's creature
// first, "up to one" so Done keeps none). Shock is the single-declaration
// control. The expected answer at every decision is gorge's.
var ksubchainDecks = [][2][]cardset.Entry{{
	{
		{Name: "Vibrant Outburst", Count: 6}, {Name: "Mirrorpool", Count: 6}, {Name: "Shock", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Mountain", Count: 8},
		{Name: "Island", Count: 8}, {Name: "Wastes", Count: 5},
	},
	{
		{Name: "Vibrant Outburst", Count: 3}, {Name: "Mirrorpool", Count: 3}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Hill Giant", Count: 8}, {Name: "Mountain", Count: 11}, {Name: "Island", Count: 7},
	},
}}

// ksubchainPolicy casts Vibrant Outburst, Shock and a creature whenever it
// can, copies the spell on the stack with Mirrorpool's copy ability (its
// second ability) three games in four, and otherwise plays like RandomPolicy
// (so the copy's answers are random: keep, change the root, change the tap).
func ksubchainPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority && seed%4 != 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActActivate && a.Mode() == 1 && refFaceName(c, a.Src()) == "Mirrorpool" {
					return a
				}
			}
			for _, want := range []string{"Vibrant Outburst", "Shock", "Grizzly Bears", "Hill Giant"} {
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && refFaceName(c, a.Src()) == want {
						return a
					}
				}
			}
		}
		if c.D.Kind == DTarget && seed%4 != 0 {
			// a copy's activation names the Vibrant Outburst, not a Shock
			for _, a := range c.D.Options {
				if a.Kind() == ActTarget && int(a.Tgt()) < int(RefPlayer) && refFaceName(c, a.Tgt()) == "Vibrant Outburst" {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// ksubchainTally counts, from gorge's side, the copies of Vibrant Outburst
// and the copy_targets asks they pose: a root ask (a player is offered) and a
// chained ask (creatures only).
type ksubchainTally struct {
	games, copies, rootAsks, chainAsks, shockAsks int

	seen map[state.ObjID]bool
	last *decision.Decision
}

func (p *ksubchainTally) reset() { p.seen, p.last = map[state.ObjID]bool{}, nil }

func (p *ksubchainTally) check(ref *GorgeSide, _ Side) *Divergence {
	e := ref.E
	for _, si := range e.G.Stack {
		if o := e.G.Obj(si); o != nil && o.IsCopy && o.Face() != nil && o.Face().Name == "Vibrant Outburst" && !p.seen[si] {
			p.seen[si] = true
			p.copies++
		}
	}
	d := e.Pending()
	if d == nil || d == p.last || d.Kind != decision.KTarget || d.ResumeKind != "copy_targets" {
		return nil
	}
	p.last = d
	src := e.G.Obj(d.Source)
	if src == nil || src.Face() == nil {
		return nil
	}
	if src.Face().Name == "Shock" {
		p.shockAsks++
		return nil
	}
	player := false
	for _, o := range d.Options {
		player = player || o.Kind == "player"
	}
	if player {
		p.rootAsks++
	} else {
		p.chainAsks++
	}
	return nil
}

func TestCopySubchainLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("subchain")
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 256
	var tal ksubchainTally
	div, stalls := 0, 0
	for mi, md := range ksubchainDecks {
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
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"en-a", "en-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			tal.reset()
			res := Lockstep(m, seed, k, ksubchainPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 20, Check: tal.check})
			CloseSide(k)
			tal.games++
			if res.Stall != "" && res.Stall != "turns" {
				stalls++
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, %d divergences, %d stalls: %d Vibrant Outburst copies, %d root asks, %d chained asks, %d Shock asks", tal.games, div, stalls, tal.copies, tal.rootAsks, tal.chainAsks, tal.shockAsks)
	if tal.copies < 20 || tal.chainAsks < 10 {
		t.Errorf("copies %d, chained asks %d; want at least 20 and 10", tal.copies, tal.chainAsks)
	}
}
