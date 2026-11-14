package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestMBCApiParams3Lowers plays the MBC api-params-3 spellings kgen lowers,
// in lockstep with gorge over probe cards (each the printed line of an MBC
// card without the other-family blockers the full card still carries):
//
//   - DamageAmount$ EQ1 on a plain Mode$ DamageDone line with a recipient
//     read (Sahir, Visitor in Darkness's "a source you control deals exactly
//     1 damage to you"). The deck's pain lands deal 1 damage to their
//     controller (the line fires) and Ancient Tomb deals 2 (it must not).
//   - UnlessCost$ Sac<1/Creature> with UnlessSwitched$ True and UnlessPayer$
//     You on a loyalty AB$ Mana (Thomil, the Destroyer's 0 ability: "you may
//     sacrifice a creature. If you do, add {B}{B}{B}"): the mana is added
//     only when the payer sacrifices.
//
// Manifest Activator$ Player (Sash and Waistcoat, Unmen) stays refused: the
// kernel has no "any player may activate" offer, see the report.
//
// Every subtest requires its mechanism to be reached, so a game that never
// gets there is not a pass; without the lowering the probe is refused and the
// set cannot be built. The DamageAmount$ subtest also runs a mutation that
// drops the gate and must diverge.

const damageAmountProbe = "Name:Damage Gate Probe\nManaCost:1 B B\nTypes:Creature Human Wizard\nPT:3/3\n" +
	"T:Mode$ DamageDone | ValidTarget$ You | ValidSource$ Card.YouCtrl,Emblem.YouCtrl | DamageAmount$ EQ1 | TriggerZones$ Battlefield | Execute$ ProbeCounter | TriggerDescription$ Probe: a source you control deals exactly 1 damage to you.\n" +
	"SVar:ProbeCounter:DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1\n" +
	"Oracle:Probe.\n"

const unlessManaProbe = "Name:Unless Mana Probe\nManaCost:3 B B\nTypes:Legendary Planeswalker Thomil\nLoyalty:4\n" +
	"A:AB$ Mana | Planeswalker$ True | Cost$ SubCounter<0/LOYALTY> | Amount$ 3 | Produced$ B | UnlessPayer$ You | UnlessSwitched$ True | UnlessCost$ Sac<1/Creature> | SpellDescription$ Probe: sacrifice a creature, and if you do, three black mana.\n" +
	"Oracle:Probe.\n"

// apiParams3Play plays g's lockstep games and calls count on every new gorge
// event, returning the divergences (one string each).
func apiParams3Play(t *testing.T, reg *cards.Registry, corpus string, g refsGame, count func(ref *GorgeSide, ev *events.Event)) (divs []string) {
	t.Helper()
	set := refsSet(t, corpus, g.decks)
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(g.decks[0]), entries(g.decks[1]))
	if err != nil {
		t.Fatalf("%s: %v", g.carrier, err)
	}
	var cardDecks [2][]*cards.Card
	for s := range g.decks {
		for _, n := range g.decks[s] {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			cardDecks[s] = append(cardDecks[s], c)
		}
	}
	for seed := uint64(1); seed <= g.seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"apiparams3-a", "apiparams3-b"}, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				count(ref, &evs[seen])
			}
			return nil
		}
		res := Lockstep(m, seed, k, castActivatePolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			divs = append(divs, fmt.Sprintf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail))
		}
	}
	return divs
}

func TestMBCApiParams3Lowers(t *testing.T) {
	corpus := probeCorpus(t, []string{"Caves of Koilos", "Ancient Tomb", "Walking Corpse", "Swamp", "Hill Giant", "Mountain"}, damageAmountProbe, unlessManaProbe)
	reg, err := cards.OpenCorpus(corpus)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("DamageAmount", func(t *testing.T) {
		g := refsGame{
			carrier: "Damage Gate Probe", seeds: 24,
			decks: [2][]string{
				refsDeck(map[string]int{"Damage Gate Probe": 8, "Walking Corpse": 6, "Caves of Koilos": 10, "Ancient Tomb": 3}, "Swamp"),
				refsDeck(map[string]int{"Hill Giant": 6, "Mountain": 14}, "Mountain"),
			},
		}
		var fired, oneDmg, twoDmg int
		var last int // the amount of the last Damage event; its DamageProvenance names the source
		count := func(ref *GorgeSide, ev *events.Event) {
			o := ref.E.G.Obj(ev.Obj)
			switch {
			case ev.Kind == events.TriggerPush && o != nil && o.Face() != nil && o.Face().Name == "Damage Gate Probe":
				fired++
			case ev.Kind == events.Damage:
				last = int(ev.Amount)
			case ev.Kind == events.DamageProvenance && o != nil && o.Face() != nil:
				switch name := o.Face().Name; {
				case name == "Caves of Koilos" && last == 1:
					oneDmg++
				case name == "Ancient Tomb" && last == 2:
					twoDmg++
				}
			}
		}
		for _, d := range apiParams3Play(t, reg, corpus, g, count) {
			t.Error(d)
		}
		t.Logf("trigger fired %d times; %d pain-land pings of 1, %d Ancient Tomb pings of 2", fired, oneDmg, twoDmg)
		if fired < 3 || twoDmg < 3 {
			t.Fatalf("the games never reach the gate: fired %d (want >= 3), Ancient Tomb pings %d (want >= 3)", fired, twoDmg)
		}

		// the mutation: the gate dropped, so Ancient Tomb's 2 damage fires
		// the line gorge withholds
		defer func(old bool) { dirac.DebugDdoNoGate = old }(dirac.DebugDdoNoGate)
		dirac.DebugDdoNoGate = true
		if divs := apiParams3Play(t, reg, corpus, g, func(*GorgeSide, *events.Event) {}); len(divs) == 0 {
			t.Errorf("no divergence with the DamageAmount$ gate dropped: the games never show a non-matching amount")
		}
	})

	t.Run("UnlessCost", func(t *testing.T) {
		g := refsGame{
			carrier: "Unless Mana Probe", seeds: 24,
			decks: [2][]string{
				refsDeck(map[string]int{"Unless Mana Probe": 8, "Walking Corpse": 10, "Swamp": 14}, "Swamp"),
				refsDeck(map[string]int{"Hill Giant": 6, "Mountain": 14}, "Mountain"),
			},
		}
		var activated, paid int
		count := func(ref *GorgeSide, ev *events.Event) {
			o := ref.E.G.Obj(ev.Obj)
			switch {
			case ev.Kind == events.AbilityPush && o != nil && o.Face() != nil && o.Face().Name == "Unless Mana Probe":
				activated++
			case ev.Kind == events.ManaAdd && ev.Amount == 3:
				paid++
			}
		}
		for _, d := range apiParams3Play(t, reg, corpus, g, count) {
			t.Error(d)
		}
		t.Logf("loyalty ability activated %d times, %d resolutions added {B}{B}{B}", activated, paid)
		if activated < 3 || paid < 1 {
			t.Fatalf("the games never reach the unless payment: activated %d (want >= 3), paid %d (want >= 1)", activated, paid)
		}
	})
}
