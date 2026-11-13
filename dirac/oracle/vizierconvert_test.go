package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (CR 609.4b): Vizier of the Menagerie lets its controller spend
// mana as though it were mana of any type to cast creature spells (S:Mode$
// ManaConvert | ManaConversion$ AnyType->AnyType; gorge rules/mana_convert.go).
// The decks are green spells over few Forests and many Mountains, so with
// Vizier out a green creature is often castable only through the conversion,
// from hand or from the top of the library (Vizier's own MayPlay), while Grow
// from the Ashes, a green sorcery, never is.
var vizierDeck = []cardset.Entry{
	{Name: "Vizier of the Menagerie", Count: 8}, {Name: "Wary Thespian", Count: 6},
	{Name: "Tajuru Pathwarden", Count: 4}, {Name: "Treetop Snarespinner", Count: 4},
	{Name: "Grow from the Ashes", Count: 4}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 10},
}

// vizierTally counts what the runs exercised: a green creature offered (and
// cast) only through the conversion, from hand and from the library top, and a
// green sorcery offered with no green mana to pay it.
type vizierTally struct {
	offerHand, offerLib, castHand, castLib, sorcery int
}

// vizierGreenless: p controls a Vizier, no untapped Forest and no green mana
// floats, so a green pip can be paid only by converted mana.
func vizierGreenless(g *GorgeSide, p state.PlayerID) bool {
	if g.E.G.Players[p].Pool[state.MG] > 0 {
		return false
	}
	vizier := false
	for _, id := range append(g.E.G.Zone(state.ZBattlefield, 0), g.E.G.Zone(state.ZBattlefield, 1)...) {
		ob := g.E.G.Obj(id)
		if ob == nil || ob.Controller != p || ob.Face() == nil {
			continue
		}
		switch ob.Face().Name {
		case "Vizier of the Menagerie":
			vizier = true
		case "Forest":
			if !ob.Tapped {
				return false
			}
		}
	}
	return vizier
}

func vizierPolicy(seed uint64, tl *vizierTally) Policy {
	inner := eagerPolicy(seed, map[string]int{})
	return PolicyFunc(func(pc *PickCtx) Canon {
		if pc.G == nil {
			return inner.Pick(pc)
		}
		var pick Canon
		picked, fromLib := false, false
		for _, a := range pc.D.Options {
			if a.Kind() != ActCast {
				continue
			}
			o, ok := pc.G.GorgeOption(a)
			if !ok {
				continue
			}
			ob := pc.G.E.G.Obj(o.Obj)
			if ob == nil || ob.Face() == nil || !vizierGreenless(pc.G, state.PlayerID(pc.D.Player)) {
				continue
			}
			switch ob.Face().Name {
			case "Grow from the Ashes":
				tl.sorcery++
			case "Wary Thespian", "Tajuru Pathwarden", "Treetop Snarespinner", "Vizier of the Menagerie":
				if ob.Zone == state.ZLibrary {
					tl.offerLib++
				} else {
					tl.offerHand++
				}
				if !picked {
					pick, picked, fromLib = a, true, ob.Zone == state.ZLibrary
				}
			}
		}
		if picked {
			if fromLib {
				tl.castLib++
			} else {
				tl.castHand++
			}
			return pick
		}
		return inner.Pick(pc)
	})
}

func TestVizierManaConvertLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fdn")
	var cs [2][]*cards.Card
	for s := range cs {
		for _, e := range vizierDeck {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], card)
			}
		}
		if len(cs[s]) != 40 {
			t.Fatalf("deck %d has %d cards", s, len(cs[s]))
		}
	}
	tbl, _, _, err := set.Matchup(vizierDeck, vizierDeck)
	if err != nil {
		t.Fatal(err)
	}
	var tl vizierTally
	games, divs := 0, 0
	for seed := uint64(1); seed <= 32; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"vizier-a", "vizier-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, vizierPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if res.Div != nil {
			divs++
			if divs <= 3 {
				lo := max(res.Div.At-10, 0)
				t.Errorf("seed %d (autopay %v): diverged at %d: %s %s %v %s\nref %v\nker %v\nrecent %v\nactions %v", seed, cfg.GorgeAutopaySurface, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions, res.RecentSources(m, 8), actionStrings(res.Actions[lo:res.Div.At]))
			}
		}
	}
	t.Logf("%d games, %d divergent, %+v", games, divs, tl)
	if tl.castHand == 0 || tl.castLib == 0 {
		t.Errorf("no green creature cast through the conversion from hand (%d) or the library top (%d)", tl.castHand, tl.castLib)
	}
	if tl.sorcery != 0 {
		t.Errorf("a green sorcery was offered %d times with no green mana", tl.sorcery)
	}
}
