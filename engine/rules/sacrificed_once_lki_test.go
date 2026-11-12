package rules

// Mode$ SacrificedOnce looks back at the sacrificed permanent as it last
// existed on the battlefield (CR 603.10a). Ygra, Eater of All makes every
// other creature a Food only while it is on the battlefield; a creature
// sacrificed as the Food for a forage cost must still be a Food to
// Camellia, the Seedmiser's ValidCard$ Food.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestSacrificedOnceMatchesLKIFood(t *testing.T) {
	t.Parallel()
	for _, mana := range []bool{false, true} {
		name := "plain forage cost"
		if mana {
			name = "mana-cost forage (Camellia's own ability)"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			reg := testutil.CorpusRegistry(t)
			camellia := lookup(t, reg, "Camellia, the Seedmiser")
			ygra := lookup(t, reg, "Ygra, Eater of All")
			elves := lookup(t, reg, "Llanowar Elves")
			forager := card(t, "Name:Forager\nManaCost:0\nTypes:Creature Elf\nPT:1/1\nA:AB$ Pump | Cost$ Forage | NumAtt$ 1\nOracle:x\n")
			e := corpusEngine(t, reg, []*cards.Card{camellia, ygra, elves, forager}, nil)
			camID := addToBattlefield(t, e, camellia, 0)
			addToBattlefield(t, e, ygra, 0)
			fID := addToBattlefield(t, e, forager, 0)
			elfID := addToBattlefield(t, e, elves, 0)

			// Preconditions: the Elves is a creature that is a Food only
			// through Ygra, and no graveyard card offers an easier forage.
			if o := e.G.Obj(elfID); o == nil || o.Zone != state.ZBattlefield {
				t.Fatal("precondition: Llanowar Elves is not on the battlefield")
			}
			if len(e.G.Zone(state.ZGraveyard, 0)) != 0 {
				t.Fatal("precondition: graveyard is not empty")
			}
			if n := countTokensNamed(t, e, "Squirrel Token"); n != 0 {
				t.Fatalf("precondition: %d Squirrels already present", n)
			}

			src, idx := fID, 0
			if mana {
				addMana(t, e, 0, "CC")
				src, idx = camID, 0
			}
			e.pending = nil
			e.priorityRound()
			opt := abilityOption(t, e, src, idx)
			submitChoices(t, e, opt.Index)
			sawElves := false
			drainSetMechanic(t, e, func(d *decision.Decision) []int {
				switch d.Kind {
				case decision.KChoose:
					for _, o := range d.Options {
						if o.Obj == elfID {
							sawElves = true
							return []int{o.Index}
						}
					}
				case decision.KTriggerOrder:
					all := make([]int, 0, len(d.Options))
					for _, o := range d.Options {
						all = append(all, o.Index)
					}
					return all
				}
				return firstOption(d)
			})
			if !sawElves {
				t.Fatal("precondition: the Ygra-Food Elves was never offered as the forage sacrifice")
			}
			if o := e.G.Obj(elfID); o == nil || o.Zone == state.ZBattlefield {
				t.Fatal("precondition: the Elves was not sacrificed")
			}
			if n := countTokensNamed(t, e, "Squirrel Token"); n != 1 {
				t.Fatalf("Squirrels after sacrificing a Ygra-Food for forage = %d, want 1 (Camellia's SacrificedOnce)", n)
			}
		})
	}
}
