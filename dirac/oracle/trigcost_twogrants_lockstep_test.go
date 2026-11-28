package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// twoGrantElfScript is a creature that gives every creature of its controller,
// itself included, two "{T}: Add {G}" abilities.
const twoGrantElfScript = "Name:Twin Grant Probe\nManaCost:G\nTypes:Creature Elf\nPT:1/1\n" +
	"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddAbility$ TapA\n" +
	"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddAbility$ TapB\n" +
	"SVar:TapA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Probe mana {G}.\n" +
	"SVar:TapB:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Probe mana {G}.\n" +
	"Oracle:Creatures you control have two mana abilities.\n"

// oneGrantElfScript grants one such ability, so two copies of it give each
// creature the same granted ability twice.
const oneGrantElfScript = "Name:Solo Grant Probe\nManaCost:G\nTypes:Creature Elf\nPT:1/1\n" +
	"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddAbility$ Tap\n" +
	"SVar:Tap:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Probe mana {G}.\n" +
	"Oracle:Creatures you control have \"{T}: Add {G}.\"\n"

// twoGrantPayScript is a {B} creature whose entering trigger may be paid with
// {G}{G} (CR 603.12).
const twoGrantPayScript = "Name:Twin Pay Probe\nManaCost:B\nTypes:Creature Zombie\nPT:1/1\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigProbeDraw | TriggerDescription$ When CARDNAME enters, you may pay {G}{G}. If you do, draw a card.\n" +
	"SVar:TrigProbeDraw:AB$ Draw | Cost$ G G\n" +
	"Oracle:When Twin Pay Probe enters, you may pay {G}{G}. If you do, draw a card.\n"

// TestTrigCostTwoGrantsPotentialLockstep: gorge's potential walk counts every
// mana ability of a source (potential.go potentialManaFor), so one creature
// holding two granted {T}: Add {G} abilities is two mana of its over-bound and
// the {G}{G} trigger cost is offered; the kernel's trigger-cost gate must offer
// the same pay arm (CR 603.12, 605.1a). The two abilities come from two statics
// of one card, or from two copies of a one-static card. A run is vacuous unless
// some seed poses that ask with exactly one untapped source that can fund it.
func TestTrigCostTwoGrantsPotentialLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("two statics", func(t *testing.T) {
		twoGrantLockstep(t, "Twin Grant Probe", twoGrantElfScript)
	})
	t.Run("two copies", func(t *testing.T) {
		twoGrantLockstep(t, "Solo Grant Probe", oneGrantElfScript)
	})
}

// twoGrantLockstep plays seeds 41-100 of a mirror of grant creatures, pay
// creatures and basic lands.
func twoGrantLockstep(t *testing.T, grant, script string) {
	t.Helper()
	dir := filters11Corpus(t, map[string]string{grant: script, "Twin Pay Probe": twoGrantPayScript},
		"Forest", "Swamp")
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	deck := append(deckOf(10, grant), append(deckOf(10, "Twin Pay Probe"),
		append(deckOf(10, "Forest"), deckOf(10, "Swamp")...)...)...)
	if len(deck) != 40 {
		t.Fatalf("deck size = %d, want 40", len(deck))
	}
	decks := [2][]string{deck, deck}
	var cardDecks [2][]*cards.Card
	var entries [2][]cardset.Entry
	for s, names := range decks {
		for _, n := range names {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			cardDecks[s] = append(cardDecks[s], card)
			entries[s] = append(entries[s], cardset.Entry{Name: n, Count: 1})
		}
	}
	set := printedDelveSetIn(t, dir, decks)
	tbl, _, _, err := set.Matchup(entries[0], entries[1])
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	lone := 0
	for seed := uint64(41); seed <= 100; seed++ {
		m, err := NewMatchup(reg, [2]string{"two-grants-a", "two-grants-b"}, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rnd := RandomPolicy(seed)
		pol := PolicyFunc(func(p *PickCtx) Canon {
			if twoGrantLonePayAsk(p.G) {
				lone++
			}
			return rnd.Pick(p)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Stall == GorgeTriggerCapStall {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
	}
	t.Logf("lone-source pay asks: %d", lone)
	if lone == 0 && !t.Failed() {
		t.Fatal("no seed offered the {G}{G} pay over a lone untapped grant source, so the lockstep is vacuous")
	}
}

// twoGrantLonePayAsk reports a pending trigger-cost pay offer whose payer has
// no untapped land and exactly one creature able to tap for its granted mana.
func twoGrantLonePayAsk(g *GorgeSide) bool {
	if g == nil || g.E == nil {
		return false
	}
	d := g.Pending()
	if d == nil {
		return false
	}
	pay := false
	for _, o := range d.Options {
		pay = pay || o.Kind == "trigger_cost_pay"
	}
	if !pay {
		return false
	}
	lands, creatures := 0, 0
	for _, id := range g.E.G.Zone(state.ZBattlefield, d.Player) {
		o := g.E.G.Obj(id)
		if o == nil || o.Face() == nil || o.Tapped {
			continue
		}
		switch o.Face().Name {
		case "Forest", "Swamp":
			lands++
		default:
			if !o.SummonSick {
				creatures++
			}
		}
	}
	return lands == 0 && creatures == 1
}
