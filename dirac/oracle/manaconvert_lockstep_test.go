package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (CR 609.4b): Oath of Nissa lets its controller spend mana as
// though it were mana of any color to cast planeswalker spells (S:Mode$
// ManaConvert | ValidCard$ Planeswalker.YouCtrl | ManaConversion$
// AnyType->AnyColor; gorge rules/mana_convert.go). The decks are blue and black
// planeswalkers over Forests and Mountains: nothing in them makes blue or
// black mana, so a planeswalker is castable only through the conversion, and
// only from mana the player floats (Pyretic Ritual's {R}{R}{R}: gorge's cast
// planner funds the pips as printed, and the conversion widens a cast only
// through the pool). Grizzly Bears, a green creature, is outside the
// conversion: it is castable only from a Forest's mana.
var nissaDeck = []cardset.Entry{
	{Name: "Oath of Nissa", Count: 6}, {Name: "Pyretic Ritual", Count: 8},
	{Name: "Jace Beleren", Count: 6}, {Name: "Liliana of the Veil", Count: 4},
	{Name: "Grizzly Bears", Count: 2}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 10},
}

// nissaTally counts what the runs exercised: a planeswalker offered, and cast,
// with an Oath out, one offered without it, and a green creature offered with
// no green mana.
type nissaTally struct {
	offer, cast, noOath, creature int
}

// nissaView is p's board for the policy: an Oath they control, a green source
// they can still tap or float, a planeswalker in hand, red mana floating.
type nissaView struct {
	oath, green, walker bool
	pool                int
}

func nissaRead(g *GorgeSide, p state.PlayerID) nissaView {
	var v nissaView
	pl := &g.E.G.Players[p]
	for _, n := range pl.Pool {
		v.pool += int(n)
	}
	v.green = pl.Pool[state.MG] > 0
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		v.read(g, p, id, true)
	}
	for _, id := range g.E.G.Zone(state.ZBattlefield, 1) {
		v.read(g, p, id, true)
	}
	for _, id := range g.E.G.Zone(state.ZHand, p) {
		v.read(g, p, id, false)
	}
	return v
}

func (v *nissaView) read(g *GorgeSide, p state.PlayerID, id state.ObjID, onBattlefield bool) {
	ob := g.E.G.Obj(id)
	if ob == nil || ob.Face() == nil || ob.Controller != p {
		return
	}
	switch ob.Face().Name {
	case "Oath of Nissa":
		v.oath = v.oath || onBattlefield
	case "Forest":
		v.green = v.green || onBattlefield && !ob.Tapped
	case "Jace Beleren", "Liliana of the Veil":
		v.walker = v.walker || !onBattlefield
	}
}

func nissaPolicy(seed uint64, tl *nissaTally) Policy {
	inner := eagerPolicy(seed, map[string]int{})
	return PolicyFunc(func(pc *PickCtx) Canon {
		if pc.G == nil {
			return inner.Pick(pc)
		}
		v := nissaRead(pc.G, state.PlayerID(pc.D.Player))
		var walker, oath, ritual Canon
		haveWalker, haveOath, haveRitual := false, false, false
		for _, a := range pc.D.Options {
			if a.Kind() != ActCast {
				continue
			}
			o, ok := pc.G.GorgeOption(a)
			if !ok {
				continue
			}
			ob := pc.G.E.G.Obj(o.Obj)
			if ob == nil || ob.Face() == nil {
				continue
			}
			switch ob.Face().Name {
			case "Jace Beleren", "Liliana of the Veil":
				tl.offer++
				if !v.oath {
					tl.noOath++
				}
				walker, haveWalker = a, true
			case "Grizzly Bears":
				if !v.green {
					tl.creature++
				}
			case "Oath of Nissa":
				oath, haveOath = a, true
			case "Pyretic Ritual":
				if v.oath && v.walker && v.pool == 0 {
					ritual, haveRitual = a, true // float {R}{R}{R} for the conversion to spend
				}
			}
		}
		switch {
		case haveWalker:
			tl.cast++
			return walker
		case haveRitual:
			return ritual
		case haveOath:
			return oath
		}
		return inner.Pick(pc)
	})
}

func TestManaConvertLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	var names [2][]string
	for _, e := range nissaDeck {
		names[0] = append(names[0], e.Name)
	}
	set := cardset.Raw(printedDelveSet(t, names))
	var cs [2][]*cards.Card
	for s := range cs {
		for _, e := range nissaDeck {
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
	tbl, _, _, err := set.Matchup(nissaDeck, nissaDeck)
	if err != nil {
		t.Fatal(err)
	}
	var tl nissaTally
	games, divs := 0, 0
	for seed := uint64(1); seed <= 32; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"nissa-a", "nissa-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, nissaPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
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
	if tl.offer == 0 || tl.cast == 0 {
		t.Errorf("no planeswalker offered (%d) or cast (%d) through the conversion", tl.offer, tl.cast)
	}
	if tl.noOath != 0 {
		t.Errorf("a planeswalker was offered %d times with no Oath out", tl.noOath)
	}
	if tl.creature != 0 {
		t.Errorf("a green creature was offered %d times with no green mana", tl.creature)
	}
	t.Run("orrery", orreryLockstep) // manaconvert_orrery_test.go
}
