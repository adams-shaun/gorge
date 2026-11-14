package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The extra-block requirement lockstep: a block declaration with a CR 509.1c
// requirement (a lure, a must-be-blocked attacker, a creature that blocks this
// turn if able) while a blocker of two attackers (Foriysian Interceptor and
// Brigade: "can block an additional creature each combat", CanBlockAmount$ 1)
// is ready and two or more attackers could take it. gorge offers the second
// pair inside the same declaration, judged against the quota of one pair a
// blocker (decision.blockRequiredCore) that the second pair can pass; the
// kernel once stalled there (StallExtraBlockReq). Printed and granted lures
// (Prized Unicorn, the Lure Aura), a printed must-be-blocked creature
// (Vinebred Brawler) and a block duty (Culvert Ambusher) are in the deck.
var kxbreqDeck = []cardset.Entry{
	{Name: "Prized Unicorn", Count: 3}, {Name: "Vinebred Brawler", Count: 3}, {Name: "Lure", Count: 2},
	{Name: "Culvert Ambusher", Count: 2}, {Name: "Foriysian Interceptor", Count: 3},
	{Name: "Foriysian Brigade", Count: 3}, {Name: "Grizzly Bears", Count: 6}, {Name: "Forest", Count: 11}, {Name: "Plains", Count: 7},
}

var kxbreqTwo = map[string]bool{"Foriysian Interceptor": true, "Foriysian Brigade": true}

type kxbreqStats struct {
	mu     sync.Mutex
	decls  int // declarations with a requirement offered and a two-attacker blocker with two attackers to take
	second int // block picks that are a blocker's second pair
	reqSec int // of them, in a declaration with a requirement
}

// kxbreqPolicy casts, attacks and blocks whenever it can (so the games reach
// the declarations and the second pairs), else random; it counts, from
// gorge's options, the declarations the second pair is offered in.
func kxbreqPolicy(seed uint64, st *kxbreqStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	var blocked map[state.ObjID]bool
	inDecl, hasReq := false, false
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActCastFace, ActActivate, ActPlayLand, ActAttack, ActBlock:
				buf = append(buf, a)
			}
		}
		a := c.D.Options[0]
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if c.G == nil || c.D.Kind != DBlockers {
			inDecl = false
			return a
		}
		if !inDecl {
			inDecl, blocked = true, map[state.ObjID]bool{}
			hasReq = false
			atks := map[state.ObjID]map[state.ObjID]bool{} // two-attacker blocker -> attackers it is offered
			for _, x := range c.D.Options {
				if x.Kind() != ActBlock {
					continue
				}
				o, ok := c.G.GorgeOption(x)
				if !ok {
					continue
				}
				hasReq = hasReq || o.BlockMust || o.Required || o.AttackMust
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && kxbreqTwo[ob.Face().Name] {
					if atks[o.Obj] == nil {
						atks[o.Obj] = map[state.ObjID]bool{}
					}
					atks[o.Obj][o.Attacker] = true
				}
			}
			two := false
			for _, m := range atks {
				two = two || len(m) >= 2
			}
			if hasReq && two {
				st.mu.Lock()
				st.decls++
				st.mu.Unlock()
			}
		}
		if a.Kind() == ActBlock {
			if o, ok := c.G.GorgeOption(a); ok {
				if blocked[o.Obj] {
					st.mu.Lock()
					st.second++
					if hasReq {
						st.reqSec++
					}
					st.mu.Unlock()
				}
				blocked[o.Obj] = true
			}
		}
		return a
	})
}

func TestExtraBlockRequirementLockstep(t *testing.T) {
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range kxbreqDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	set := printedDelveSet(t, names)
	decks := [2][]cardset.Entry{kxbreqDeck, kxbreqDeck}
	var st kxbreqStats
	const seeds = 300
	clean, _, div := lockstepSeeds(t, reg, set, "kxbreq", decks, gd, 1, seeds, true, nil, nil, func(seed uint64) Policy { return kxbreqPolicy(seed, &st) })
	t.Logf("%d seeds, %d clean, %d divergent; %d declarations with a requirement and a two-attacker blocker, %d second pairs (%d under a requirement)", seeds, clean, div, st.decls, st.second, st.reqSec)
	if st.decls < 100 {
		t.Errorf("only %d declarations with a requirement and a two-attacker blocker ready, want >= 100", st.decls)
	}
	if st.reqSec < 100 {
		t.Errorf("only %d second pairs declared under a requirement, want >= 100", st.reqSec)
	}
}
