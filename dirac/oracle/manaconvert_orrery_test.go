package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Chromatic Orrery arm of TestManaConvertLockstep (CR 609.4b): "You may
// spend mana as though it were mana of any color" (S:Mode$ ManaConvert |
// ValidPlayer$ You | ManaConversion$ AnyType->AnyColor, no ValidSA$ and no
// ValidCard$, so it covers every payment: gorge manaConversionParts gives it
// to pay.SAActivated as to pay.SASpell). The decks make red and green mana
// only, so a black or blue spell is castable, and a black or blue activation
// payable, only through the conversion and only from mana the player floats
// (Pyretic Ritual's {R}{R}{R}: gorge's planner prices the pips as printed, and
// the conversion widens a payment through the pool alone).
var orreryDeck = []cardset.Entry{
	{Name: "Chromatic Orrery", Count: 4}, {Name: "Pyretic Ritual", Count: 5},
	{Name: "Sol Ring", Count: 2}, {Name: "Mana Vault", Count: 1},
	{Name: "Pit Imp", Count: 4}, {Name: "Frozen Shade", Count: 3}, {Name: "Aether Spellbomb", Count: 3},
	{Name: "Jace Beleren", Count: 2}, {Name: "Liliana of the Veil", Count: 2},
	{Name: "Forest", Count: 4}, {Name: "Mountain", Count: 10},
}

// orreryTally counts what the runs exercised: an off-colour spell cast, and an
// off-colour activation made, with an Orrery out; and either one with none (it
// must stay zero: nothing else makes black or blue mana).
type orreryTally struct {
	cast, act, noOrrery, orrery int
	picks                       []string // this game's chosen casts and activations, for a divergence report
}

// orreryView is p's board for the policy.
type orreryView struct {
	orrery, ritual, offColour bool
	pool                      int
}

func orreryRead(g *GorgeSide, p state.PlayerID) orreryView {
	var v orreryView
	pl := &g.E.G.Players[p]
	for _, n := range pl.Pool {
		v.pool += int(n)
	}
	for _, z := range []state.Zone{state.ZBattlefield, state.ZHand} {
		for _, seat := range []state.PlayerID{0, 1} {
			for _, id := range g.E.G.Zone(z, seat) {
				ob := g.E.G.Obj(id)
				if ob == nil || ob.Face() == nil || ob.Controller != p {
					continue
				}
				switch ob.Face().Name {
				case "Chromatic Orrery":
					v.orrery = v.orrery || z == state.ZBattlefield
				case "Pyretic Ritual":
					v.ritual = v.ritual || z == state.ZHand
				case "Pit Imp", "Frozen Shade", "Aether Spellbomb", "Jace Beleren", "Liliana of the Veil":
					v.offColour = true
				}
			}
		}
	}
	return v
}

// orreryBoard lists p's battlefield, a tapped permanent starred.
func orreryBoard(g *GorgeSide, p state.PlayerID) string {
	var out []string
	for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			n := ob.Face().Name
			if ob.Tapped {
				n += "*"
			}
			out = append(out, n)
		}
	}
	return fmt.Sprint(out)
}

func orreryOffColour(name string) bool {
	switch name {
	case "Pit Imp", "Frozen Shade", "Aether Spellbomb", "Jace Beleren", "Liliana of the Veil":
		return true
	}
	return false
}

func orreryPolicy(seed uint64, tl *orreryTally) Policy {
	inner := eagerPolicy(seed, map[string]int{})
	return PolicyFunc(func(pc *PickCtx) Canon {
		if pc.G == nil {
			return inner.Pick(pc)
		}
		v := orreryRead(pc.G, state.PlayerID(pc.D.Player))
		var act, cast, ritual, orrery Canon
		haveAct, haveCast, haveRitual, haveOrrery := false, false, false, false
		for _, a := range pc.D.Options {
			if k := a.Kind(); k != ActCast && k != ActActivate {
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
			name := ob.Face().Name
			switch {
			case a.Kind() == ActActivate && orreryOffColour(name) && ob.Zone == state.ZBattlefield:
				// a coloured activation of a Pit Imp, Frozen Shade or Spellbomb
				if v.orrery {
					tl.act++
				} else if name != "Aether Spellbomb" { // (its {1} part needs no conversion)
					tl.noOrrery++
				}
				act, haveAct = a, true
			case a.Kind() == ActCast && orreryOffColour(name) && name != "Aether Spellbomb": // the Spellbomb's cast is {1}
				if v.orrery {
					tl.cast++
				} else {
					tl.noOrrery++
				}
				cast, haveCast = a, true
			case a.Kind() == ActCast && name == "Pyretic Ritual":
				if v.orrery && v.offColour && v.pool == 0 {
					ritual, haveRitual = a, true // float {R}{R}{R} for the conversion to spend
				}
			case a.Kind() == ActCast && name == "Chromatic Orrery":
				orrery, haveOrrery = a, true
			}
		}
		if v.orrery {
			tl.orrery++
		}
		pick := func(a Canon, what string) Canon {
			if o, ok := pc.G.GorgeOption(a); ok {
				if ob := pc.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					what += fmt.Sprint(" ", ob.Face().Name, " #", o.Obj, " pool", pc.G.E.G.Players[pc.D.Player].Pool, " ", orreryBoard(pc.G, state.PlayerID(pc.D.Player)))
				}
			}
			tl.picks = append(tl.picks, what)
			return a
		}
		switch {
		case haveOrrery:
			return pick(orrery, "orrery")
		case haveAct && v.orrery:
			return pick(act, "act")
		case haveCast && v.orrery:
			return pick(cast, "cast")
		case haveRitual:
			return pick(ritual, "ritual")
		}
		a := inner.Pick(pc)
		if k := a.Kind(); k == ActCast || k == ActActivate {
			if o, ok := pc.G.GorgeOption(a); ok {
				if ob := pc.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					tl.picks = append(tl.picks, fmt.Sprint(k, " ", ob.Face().Name, " #", o.Obj))
				}
			}
		}
		return a
	})
}

// orreryLockstep plays seeded games of the Orrery deck against itself, kernel
// beside gorge, and requires no divergence and that the conversion was
// exercised for both a cast and an activation.
func orreryLockstep(t *testing.T) {
	reg := synthReg(t)
	var names [2][]string
	for _, e := range orreryDeck {
		names[0] = append(names[0], e.Name)
	}
	set := cardset.Raw(printedDelveSet(t, names))
	var cs [2][]*cards.Card
	for s := range cs {
		for _, e := range orreryDeck {
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
	tbl, _, _, err := set.Matchup(orreryDeck, orreryDeck)
	if err != nil {
		t.Fatal(err)
	}
	var tl orreryTally
	games, divs := 0, 0
	for seed := uint64(1); seed <= 32; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"orrery-a", "orrery-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tl.picks = tl.picks[:0]
		res := Lockstep(m, seed, k, orreryPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if res.Div != nil {
			divs++
			if divs <= 3 {
				lo := max(res.Div.At-10, 0)
				t.Errorf("seed %d (autopay %v): diverged at %d: %s %s %v %s\nref %v\nker %v\nrecent %v\nactions %v\npicks %v", seed, cfg.GorgeAutopaySurface, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions, res.RecentSources(m, 8), actionStrings(res.Actions[lo:res.Div.At]), tl.picks[max(len(tl.picks)-2, 0):])
			}
		}
	}
	t.Logf("%d games, %d divergent, %+v", games, divs, tl)
	if tl.orrery == 0 {
		t.Errorf("no Orrery ever reached the battlefield")
	}
	if tl.cast == 0 || tl.act == 0 {
		t.Errorf("no off-colour spell cast (%d) or activation offered (%d) through the conversion", tl.cast, tl.act)
	}
	if tl.noOrrery != 0 {
		t.Errorf("an off-colour spell or activation was offered %d times with no Orrery out", tl.noOrrery)
	}
}
