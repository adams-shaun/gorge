package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: Realmbreaker's Grasp and Faith's Fetters print one compound
// static line (Mode$ CantAttack,CantBlock,CantBeActivated, with a ValidSA$
// that belongs to the CantBeActivated half). The parse-time split gives the
// CantAttack and CantBlock statics the same Params, so ValidSA$ is on them
// too; gorge reads it inert there (CantAttackParamsReadableForRules), and kgen
// now lowers both halves instead of refusing the shared param. The decks are
// white weenies: the Auras land on creatures that were offered as attackers
// on an earlier turn, and the enchanted creature must drop out of the attack
// offers on both surfaces.
var cantAttackSADeck = []cardset.Entry{
	{Name: "Realmbreaker's Grasp", Count: 5}, {Name: "Faith's Fetters", Count: 5},
	{Name: "Savannah Lions", Count: 8}, {Name: "Elite Vanguard", Count: 6},
	{Name: "Plains", Count: 16},
}

// cantAttackSATally counts what the runs exercised: attack offers made, an
// Aura attached to a creature that had been offered before it, and the
// enchanted creature withheld (or, wrongly, offered) at a later attack decision.
type cantAttackSATally struct {
	offers, enchanted, withheld, offeredEnchanted int
}

func cantAttackSAPolicy(seed uint64, tl *cantAttackSATally) Policy {
	inner := eagerPolicy(seed, map[string]int{})
	rnd := RandomPolicy(seed ^ 0x5a)
	wasOffered := map[state.ObjID]bool{}
	return PolicyFunc(func(pc *PickCtx) Canon {
		if pc.G == nil || pc.D.Kind != DAttackers {
			return inner.Pick(pc)
		}
		g := pc.G.E.G
		p := state.PlayerID(pc.D.Player)
		enchanted := map[state.ObjID]bool{}
		for _, pl := range []state.PlayerID{0, 1} {
			for _, id := range g.Zone(state.ZBattlefield, pl) {
				ob := g.Obj(id)
				if ob == nil || ob.Face() == nil {
					continue
				}
				if n := ob.Face().Name; (n == "Realmbreaker's Grasp" || n == "Faith's Fetters") && ob.AttachedTo != 0 {
					enchanted[ob.AttachedTo] = true
				}
			}
		}
		var buf []Canon
		offered := map[state.ObjID]bool{}
		for _, a := range pc.D.Options {
			if a.Kind() != ActAttack {
				continue
			}
			buf = append(buf, a)
			if o, ok := pc.G.GorgeOption(a); ok {
				offered[o.Obj] = true
			}
		}
		for id := range offered {
			tl.offers++
			if enchanted[id] {
				tl.offeredEnchanted++
			}
			wasOffered[id] = true
		}
		for _, id := range g.Zone(state.ZBattlefield, p) {
			ob := g.Obj(id)
			if ob == nil || ob.Face() == nil || ob.Tapped || ob.SummonSick || !slices.Contains(ob.Face().Types, "Creature") {
				continue
			}
			if enchanted[id] && !offered[id] && wasOffered[id] {
				tl.withheld++
			}
		}
		for id := range enchanted {
			if wasOffered[id] {
				tl.enchanted++
			}
		}
		if len(buf) == 0 {
			return inner.Pick(pc)
		}
		d := *pc.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: pc.G, Seq: pc.Seq})
	})
}

func TestCantAttackValidSALockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	var names [2][]string
	for _, e := range cantAttackSADeck {
		names[0] = append(names[0], e.Name)
	}
	set := cardset.Raw(printedDelveSet(t, names))
	var cs [2][]*cards.Card
	for s := range cs {
		for _, e := range cantAttackSADeck {
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
	tbl, _, _, err := set.Matchup(cantAttackSADeck, cantAttackSADeck)
	if err != nil {
		t.Fatal(err)
	}
	var tl cantAttackSATally
	games, divs := 0, 0
	for seed := uint64(1); seed <= 32; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"cantatk-a", "cantatk-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, cantAttackSAPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
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
	if tl.offers == 0 {
		t.Error("no creature was ever offered as an attacker")
	}
	if tl.enchanted == 0 || tl.withheld == 0 {
		t.Errorf("no previously offered creature was enchanted and withheld (enchanted %d, withheld %d): the compared offers never differ", tl.enchanted, tl.withheld)
	}
	if tl.offeredEnchanted != 0 {
		t.Errorf("an enchanted creature was offered as an attacker %d times", tl.offeredEnchanted)
	}
}
