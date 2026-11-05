package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Vulcan Mind-Meld lockstep (CR 701.20, 406.3, 609.4): the target
// opponent exiles the top card of the caster's library face down after
// looking at it, the caster exiles the top two cards of the opponent's
// library face down, and the player who exiled each card may play it for as
// long as it remains exiled, spending mana of any type. gorge exiles with
// ExileFaceDown$ alone (a blind marker, nobody the looker) and the following
// Effect's MayLookAt$ / MayPlay$ statics, owned by the exiler, give the look
// and the play. The decks hold both players' cards of colours the other side
// cannot cast, so a cast through the any-type permission is reachable.

var kvulcanDecks = [2][]cardset.Entry{
	{
		{Name: "Vulcan Mind-Meld", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Ornithopter", Count: 4},
		{Name: "Hill Giant", Count: 2}, {Name: "Shock", Count: 2}, {Name: "Island", Count: 9},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 3},
	},
	{
		{Name: "Vulcan Mind-Meld", Count: 4}, {Name: "Hill Giant", Count: 6}, {Name: "Shock", Count: 4},
		{Name: "Burglar Rat", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
		{Name: "Island", Count: 6},
	},
}

// kvulcanScan counts, in gorge's event log, the cards Vulcan Mind-Meld
// exiled face down by the exiler's seat, and the plays of such a card.
func kvulcanScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	down := map[state.ObjID]bool{}
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.MoveZone {
			continue
		}
		if ev.To == state.ZExile && strings.HasPrefix(ev.Counter, "exiled_with_face_down") {
			down[ev.Obj] = true
			// the looker (Amount) is the exiler: the caster for the opponent's
			// cards, the opponent for the caster's (the source is the spell)
			if src := g.E.G.Obj(ev.IDs[0]); ev.Counter == "exiled_with_face_down_maylook" && src != nil && int32(src.Owner) != ev.Amount {
				n["exiledByOpp"]++
			} else {
				n["exiledByCtl"]++
			}
			continue
		}
		if ev.From == state.ZExile {
			if down[ev.Obj] && (ev.To == state.ZStack || ev.To == state.ZBattlefield) {
				n["played"]++
			}
			delete(down, ev.Obj)
		}
	}
}

// runVulcan plays the seeded games; mutate sets DebugVulcanWrongHolder and
// stops at the first divergence.
func runVulcan(t *testing.T, seeds uint64, mutate bool) (games, divs int, branch map[string]int, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("vulcanmeld")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kvulcanDecks {
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
	tbl, _, _, err := set.Matchup(kvulcanDecks[0], kvulcanDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugVulcanWrongHolder = mutate
	defer func() { dirac.DebugVulcanWrongHolder = false }()
	played := map[string]int{}
	branch, stalls = map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kvulcan-a", "kvulcan-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			a := inner.Pick(c)
			if k := a.Kind(); (k == ActCast || k == ActActivate) && c.G != nil {
				if o, ok := c.G.GorgeOption(a); ok {
					// a cast of a card from exile: by its owner or by the other player
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZExile && ob.FaceDown {
						branch["castFromExile"]++
						if uint8(ob.Owner) != c.D.Player {
							branch["castByOther"]++
						}
					}
				}
			}
			return a
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		kvulcanScan(gs, branch)
		if d := res.Div; d != nil {
			divs++
			if mutate {
				return
			}
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

func TestVulcanMindMeldLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, branch, stalls := runVulcan(t, 256, false)
	t.Logf("%d games, %d divergences; gorge exiled face down by the opponent %d, by the caster %d; played from exile %d (cast from exile %d, by the other player %d); kernel stalls %v",
		games, divs, branch["exiledByOpp"], branch["exiledByCtl"], branch["played"], branch["castFromExile"], branch["castByOther"], stalls)
	if divs != 0 || len(stalls) != 0 {
		t.Fatalf("%d divergences, stalls %v", divs, stalls)
	}
	if branch["exiledByOpp"] < 20 || branch["exiledByCtl"] < 40 {
		t.Errorf("face-down exiles by the opponent %d and by the caster %d, want at least 20 and 40", branch["exiledByOpp"], branch["exiledByCtl"])
	}
	if branch["played"] < 10 {
		t.Errorf("plays of a Vulcan Mind-Meld exile %d, want at least 10", branch["played"])
	}
	if branch["castByOther"] < 5 {
		t.Errorf("casts of a card in exile by a player other than its owner %d, want at least 5", branch["castByOther"])
	}
}

// TestVulcanMindMeldMutation gives each permission to the other player
// (DebugVulcanWrongHolder): the cast menus differ, so the lockstep must
// diverge.
func TestVulcanMindMeldMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runVulcan(t, 128, true)
	if divs == 0 {
		t.Fatalf("DebugVulcanWrongHolder mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
