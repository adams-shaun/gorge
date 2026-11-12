package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The X/X Token lockstep (CR 111.4, 613.4b; gorge
// effects/token.go TokenPower$/TokenToughness$): the carriers that lower
// (Tumbleweed Rising, Abzan Monument, Oviya Pashiri) mint a token whose P/T is a permanent
// layer-7b set resolved once before the mint. X is read from the creatures
// the player controls, so a minted token is often 0/0 and dies to SBA in
// both engines. Giant Growth is aimed at minted tokens, so a later pump is
// stacked on the set. Oviya's X counts the creatures its controller has, a
// count the minted Construct itself would match: it must be read before the
// mint (the kernel's Construct would otherwise be one too big).
var ktokenxptDecks = [][2][]cardset.Entry{{
	{
		{Name: "Tumbleweed Rising", Count: 3}, {Name: "Abzan Monument", Count: 3}, {Name: "Oviya Pashiri, Sage Lifecrafter", Count: 3},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Giant Growth", Count: 4}, {Name: "Murder", Count: 4},
		{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 6}, {Name: "Plains", Count: 4},
	},
	{
		{Name: "Tumbleweed Rising", Count: 3}, {Name: "Oviya Pashiri, Sage Lifecrafter", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Hill Giant", Count: 5}, {Name: "Giant Growth", Count: 4}, {Name: "Shock", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 7},
	},
}}

// ktokenxptPolicy casts the carriers, Giant Growth, Shock and Murder
// whenever it can (three games in four), aims a target at a minted token when
// one is offered, and otherwise plays like RandomPolicy.
func ktokenxptPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		switch c.D.Kind {
		case DPriority:
			if seed%4 == 0 {
				break
			}
			for _, want := range []string{"Tumbleweed Rising", "Abzan Monument", "Oviya Pashiri, Sage Lifecrafter", "Giant Growth", "Shock", "Murder"} {
				// the last match: Oviya's Construct ability follows her Servo one
				var pick Canon
				found := false
				for _, a := range c.D.Options {
					if k := a.Kind(); (k == ActCast || k == ActActivate) && refFaceName(c, a.Src()) == want {
						pick, found = a, true
					}
				}
				if found {
					return pick
				}
			}
		case DTarget:
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() != ActTarget || int(a.Tgt()) >= int(RefPlayer) {
					continue
				}
				if n := refFaceName(c, a.Tgt()); n == "Elemental Token" || n == "Spirit Token" || n == "Construct Token" {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 && seed%3 != 0 {
				return buf[0]
			}
		}
		return rnd.Pick(c)
	})
}

// ktokenxptTally counts, from gorge's side, the X/X tokens minted (a token
// first seen off the battlefield was 0/0 and died to SBA), the Giant Growths
// that targeted a token, and any token whose power and toughness differ.
type ktokenxptTally struct {
	minted, zero, pumps, games, constructs int
	bad                                    []string

	objsAt int
	pumped map[state.ObjID]bool
}

func (p *ktokenxptTally) reset() { p.objsAt, p.pumped = 0, map[state.ObjID]bool{} }

func ktokenxptIsTok(ob *state.Object) bool {
	if !ob.IsToken || ob.Face() == nil {
		return false
	}
	n := ob.Face().Name
	return n == "Elemental Token" || n == "Spirit Token" || n == "Construct Token"
}

func (p *ktokenxptTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	if len(g.Objs) < p.objsAt {
		p.objsAt = 0
	}
	for i := p.objsAt; i < len(g.Objs); i++ {
		ob := &g.Objs[i]
		if !ktokenxptIsTok(ob) {
			continue
		}
		p.minted++
		if ob.Face().Name == "Construct Token" {
			p.constructs++
		}
		if ob.Zone != state.ZBattlefield {
			p.zero++
		}
	}
	p.objsAt = len(g.Objs)
	for _, si := range g.Stack {
		it := g.Obj(si)
		if it == nil || it.Face() == nil || it.Face().Name != "Giant Growth" || p.pumped[si] {
			continue
		}
		for _, t := range it.Targets {
			if o := g.Obj(t.Obj); !t.IsPlayer && o != nil && ktokenxptIsTok(o) {
				p.pumped[si] = true
				p.pumps++
			}
		}
	}
	for i := range g.Objs {
		ob := &g.Objs[i]
		if ktokenxptIsTok(ob) && ob.Zone == state.ZBattlefield {
			id := state.ObjID(i + 1)
			if pw, tg := ref.E.Power(id), ref.E.Toughness(id); pw != tg && len(p.bad) < 5 {
				p.bad = append(p.bad, "token "+ob.Face().Name+" not X/X")
			}
		}
	}
	return nil
}

func TestTokenxptLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tokenxpt")
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 256
	var tal ktokenxptTally
	div, stalls := 0, 0
	for mi, md := range ktokenxptDecks {
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
			res := Lockstep(m, seed, k, ktokenxptPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 20, Check: tal.check})
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
	for _, b := range tal.bad {
		t.Error(b)
	}
	t.Logf("%d games, %d divergences, %d stalls: %d X/X tokens minted, %d with X = 0, %d pumps on a minted token, %d Oviya Constructs (X counts creatures)", tal.games, div, stalls, tal.minted, tal.zero, tal.pumps, tal.constructs)
	if tal.minted < 40 || tal.zero < 5 || tal.pumps < 10 || tal.constructs < 10 {
		t.Errorf("minted %d, X=0 %d, pumps %d, Constructs %d; want at least 40, 5, 10, 10", tal.minted, tal.zero, tal.pumps, tal.constructs)
	}
}
