package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Nerd Rage's lockstep (dirac/kgen/lower_grantedstatic.go; kernel
// static_granted.go): the aura's Continuous static grants the enchanted
// creature "You have no maximum hand size" (AddStaticAbility$ NoMaxHandSize),
// so its controller keeps every card at the cleanup step instead of
// discarding down to seven. The deck passes with a hand that grows past seven
// (three lands, one creature for the Aura to enchant, then Nerd Rage and
// passes), so a granted reader that either side gets wrong shows up as a
// cleanup discard on one side alone. The table is built from the decks' own
// cards (printedDelveSet).
func addStaticAbilityDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Nerd Rage", Count: 10},
		{Name: "Grizzly Bears", Count: 10},
		{Name: "Island", Count: 10},
		{Name: "Forest", Count: 10},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// addStaticAbilityPolicy plays each seat's first three lands, then casts Nerd
// Rage whenever it is offered and otherwise keeps one Grizzly Bears on the
// battlefield as the Aura's target; every other decision (targets, attackers,
// blockers, the discard pick) is random. The hand therefore grows past seven
// without the policy ever discarding voluntarily. It counts, from gorge's
// options, the Nerd Rage casts it picks.
func addStaticAbilityPolicy(casts *int) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		lands := [2]int{}
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.D.Kind != DPriority || c.G == nil {
				return rnd.Pick(c)
			}
			var nerd, bear, land, pass []Canon
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace:
					o, ok := c.G.GorgeOption(a)
					if !ok {
						continue
					}
					switch addStaticAbilityFaceName(c.G, o.Obj) {
					case "Nerd Rage":
						nerd = append(nerd, a)
					case "Grizzly Bears":
						bear = append(bear, a)
					}
				case ActPlayLand:
					land = append(land, a)
				case ActPass:
					pass = append(pass, a)
				}
			}
			pick := func(opts []Canon) Canon {
				d := *c.D
				d.Options = opts
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			switch {
			case len(nerd) > 0:
				*casts++
				return pick(nerd)
			case len(land) > 0 && lands[c.D.Player] < 3:
				lands[c.D.Player]++
				return pick(land)
			case len(bear) > 0 && !addStaticAbilityHasCreature(c.G):
				return pick(bear)
			case len(pass) > 0:
				return pick(pass)
			}
			return rnd.Pick(c)
		})
	}
}

// addStaticAbilityFaceName is the name of the object a cast option names.
func addStaticAbilityFaceName(g *GorgeSide, id state.ObjID) string {
	if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
		return ob.Face().Name
	}
	return ""
}

// addStaticAbilityHasCreature reports whether either battlefield holds a
// creature: the Aura's cast needs one as its target.
func addStaticAbilityHasCreature(g *GorgeSide) bool {
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
			ob := g.E.G.Obj(id)
			if ob == nil || ob.Face() == nil {
				continue
			}
			for _, typ := range ob.Face().Types {
				if typ == "Creature" {
					return true
				}
			}
		}
	}
	return false
}

func addStaticAbilityRun(t *testing.T, seeds uint64, report bool, mutate func(*kop.Table)) (casts, div int) {
	t.Helper()
	decks := addStaticAbilityDecks()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var gd [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	_, _, div = lockstepSeeds(t, reg, set, "granthand", decks, gd, 1, seeds, report, mutate, nil, addStaticAbilityPolicy(&casts))
	return casts, div
}

func TestAddStaticAbilityLockstep(t *testing.T) {
	t.Parallel()
	casts, div := addStaticAbilityRun(t, 60, true, nil)
	if div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	if casts == 0 {
		t.Fatal("Nerd Rage was never cast; the lockstep proves nothing")
	}
	t.Logf("Nerd Rage casts: %d", casts)
}

// TestAddStaticAbilityLockstepMutationGrant: with the granted static dropped
// from every card (the kernel's enchanted controller keeps a maximum of seven),
// some seed diverges on the cleanup discard gorge does not ask.
func TestAddStaticAbilityLockstepMutationGrant(t *testing.T) {
	t.Parallel()
	_, div := addStaticAbilityRun(t, 60, false, addStaticAbilityDropGrant)
	if div == 0 {
		t.Fatal("no seed diverged with the granted static dropped")
	}
}

func addStaticAbilityDropGrant(tbl *kop.Table) {
	cs := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cs {
		cs[i].Granted = nil
	}
	tbl.Cards = cs
}
