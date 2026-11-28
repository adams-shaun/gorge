package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Squandered Resources and Vivi Ornitier on ordinary boards: the two hosted
// Commander mana shapes beside the things a deck actually holds, compared with
// gorge decision for decision.
//
//   - Squandered Resources: a victim that makes only {C} (CR 106.7 counts every
//     type of mana, colourless included), a painland victim, a may-pay trigger
//     cost, an unless cost and a ward cost paid in a payment window.
//   - Vivi Ornitier: beside a granted mana ability, a {1},{T} filter and an
//     off-colour optional cost, so every search that activates it must take its
//     finite {U}/{R} answers, its no-tap cost and its once-each-turn bit.

const wasteProbeScript = "Name:Waste Probe\nTypes:Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C\n" +
	"Oracle:{T}: Add {C}.\n"

const painProbeScript = "Name:Pain Probe\nTypes:Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C\n" +
	"A:AB$ Mana | Cost$ T PayLife<1> | Produced$ Combo W U\n" +
	"Oracle:{T}: Add {C}.\n"

const gullProbeScript = "Name:Gull Probe\nManaCost:U\nTypes:Creature Bird\nPT:1/1\n" +
	"Oracle:1/1\n"

const rockProbeScript = "Name:Rock Probe\nManaCost:1\nTypes:Artifact Creature Golem\nPT:1/1\n" +
	"Oracle:1/1\n"

// payProbeScript is a creature costing cost whose enters trigger asks its
// controller to pay {owed} (a may-pay trigger cost, CR 118.12).
func payProbeScript(cost, owed string) string {
	return "Name:Pay Probe\nManaCost:" + cost + "\nTypes:Creature Bird\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDraw | TriggerDescription$ When CARDNAME enters, you may pay {" + owed + "}. If you do, draw a card.\n" +
		"SVar:TrigDraw:AB$ Draw | Cost$ " + owed + " | Defined$ You | NumCards$ 1\n" +
		"Oracle:When Pay Probe enters, you may pay {" + owed + "}. If you do, draw a card.\n"
}

const tollProbeScript = "Name:Toll Probe\nManaCost:B\nTypes:Creature Bird\nPT:1/1\n" +
	"T:Mode$ ChangesZone | Destination$ Battlefield | Origin$ Any | ValidCard$ Card.Self | Execute$ TrigToll | TriggerDescription$ When CARDNAME enters, sacrifice it unless you pay {1}.\n" +
	"SVar:TrigToll:DB$ Sacrifice | UnlessPayer$ You | UnlessCost$ 1\n" +
	"Oracle:When Toll Probe enters, sacrifice it unless you pay {1}.\n"

const wardProbeScript = "Name:Ward Probe\nManaCost:G\nTypes:Creature Turtle\nPT:1/4\n" +
	"K:Ward:1\n" +
	"Oracle:Ward {1}\n"

const zapProbeScript = "Name:Zap Probe\nManaCost:B\nTypes:Instant\n" +
	"A:SP$ DealDamage | ValidTgts$ Creature.OppCtrl | NumDmg$ 1 | SpellDescription$ CARDNAME deals 1 damage to target creature an opponent controls.\n" +
	"Oracle:Zap Probe deals 1 damage to target creature an opponent controls.\n"

type manaShapeBoard struct {
	name string
	// first is the card seat 0 casts whenever offered
	first    string
	fixtures map[string]string
	deck     []string
	// opp is seat 1's deck (40 Forests when nil); it casts what it can
	opp  []string
	real []string
	// seeds are the lockstep seeds played
	seeds []uint64
	// reach reports whether the finished gorge game used the shape under test
	reach func(g *GorgeSide) bool
	// window also requires a payment window to have offered Squandered Probe
	window bool
}

// seedRange is the seeds lo through hi.
func seedRange(lo, hi uint64) []uint64 {
	var seeds []uint64
	for s := lo; s <= hi; s++ {
		seeds = append(seeds, s)
	}
	return seeds
}

func manaShapeBoards() []manaShapeBoard {
	seeds := seedRange(1, 40)
	squa := func(g *GorgeSide) bool { return hostedCmdrSquanderedSacrificedAny(g) }
	vivi := func(g *GorgeSide) bool { return hostedViviActivated(g) }
	return []manaShapeBoard{
		{name: "squandered_wastes", first: "Squandered Probe",
			fixtures: map[string]string{"Squandered Probe": squanderedProbeScript, "Cub Probe": cubProbeScript, "Waste Probe": wasteProbeScript, "Rock Probe": rockProbeScript},
			deck:     concatDecks(deckOf(12, "Squandered Probe"), deckOf(8, "Waste Probe"), deckOf(8, "Forest"), deckOf(6, "Swamp"), deckOf(4, "Cub Probe"), deckOf(2, "Rock Probe")),
			real:     []string{"Forest", "Swamp"}, seeds: seeds, reach: squa},
		{name: "squandered_painland", first: "Squandered Probe",
			fixtures: map[string]string{"Squandered Probe": squanderedProbeScript, "Pain Probe": painProbeScript, "Rock Probe": rockProbeScript, "Toll Probe": tollProbeScript, "Gull Probe": gullProbeScript},
			deck:     concatDecks(deckOf(12, "Squandered Probe"), deckOf(10, "Pain Probe"), deckOf(4, "Forest"), deckOf(5, "Swamp"), deckOf(3, "Rock Probe"), deckOf(3, "Toll Probe"), deckOf(3, "Gull Probe")),
			real:     []string{"Forest", "Swamp"}, seeds: seeds, reach: squa},
		{name: "squandered_painland_owed", first: "Squandered Probe", window: true,
			fixtures: map[string]string{"Squandered Probe": squanderedProbeScript, "Pain Probe": painProbeScript, "Pay Probe": payProbeScript("B", "U")},
			deck:     concatDecks(deckOf(12, "Squandered Probe"), deckOf(12, "Pain Probe"), deckOf(4, "Forest"), deckOf(6, "Swamp"), deckOf(6, "Pay Probe")),
			real:     []string{"Forest", "Swamp"}, seeds: seeds, reach: squa},
		{name: "squandered_maypay", first: "Squandered Probe", window: true,
			fixtures: map[string]string{"Squandered Probe": squanderedProbeScript, "Pay Probe": payProbeScript("B", "G")},
			deck:     concatDecks(deckOf(12, "Squandered Probe"), deckOf(12, "Forest"), deckOf(10, "Swamp"), deckOf(6, "Pay Probe")),
			real:     []string{"Forest", "Swamp"}, seeds: seeds, reach: squa},
		{name: "squandered_colourless_owed", first: "Squandered Probe", window: true,
			fixtures: map[string]string{"Squandered Probe": squanderedProbeScript, "Waste Probe": wasteProbeScript, "Pay Probe": payProbeScript("B", "C")},
			deck:     concatDecks(deckOf(12, "Squandered Probe"), deckOf(10, "Waste Probe"), deckOf(5, "Forest"), deckOf(7, "Swamp"), deckOf(6, "Pay Probe")),
			real:     []string{"Forest", "Swamp"}, seeds: seeds, reach: squa},
		{name: "squandered_unless", first: "Squandered Probe", window: true,
			fixtures: map[string]string{"Squandered Probe": squanderedProbeScript, "Toll Probe": tollProbeScript},
			deck:     concatDecks(deckOf(12, "Squandered Probe"), deckOf(11, "Forest"), deckOf(11, "Swamp"), deckOf(6, "Toll Probe")),
			real:     []string{"Forest", "Swamp"}, seeds: seeds, reach: squa},
		{name: "squandered_ward", first: "Squandered Probe", window: true,
			fixtures: map[string]string{"Squandered Probe": squanderedProbeScript, "Zap Probe": zapProbeScript, "Ward Probe": wardProbeScript},
			deck:     concatDecks(deckOf(12, "Squandered Probe"), deckOf(10, "Forest"), deckOf(10, "Swamp"), deckOf(8, "Zap Probe")),
			opp:      concatDecks(deckOf(26, "Forest"), deckOf(14, "Ward Probe")),
			real:     []string{"Forest", "Swamp"}, seeds: seeds, reach: squa},
		{name: "vivi_grant", first: "Vivi Probe",
			fixtures: map[string]string{"Vivi Probe": viviProbeScript, "Vivi Spell": viviSpellScript, "Grant Probe": grantProbeScript, "Dove Probe": doveProbeScript, "Duo Probe": duoProbeScript},
			deck:     concatDecks(deckOf(8, "Vivi Probe"), deckOf(7, "Island"), deckOf(7, "Mountain"), deckOf(4, "Forest"), deckOf(4, "Grant Probe"), deckOf(3, "Dove Probe"), deckOf(3, "Duo Probe"), deckOf(4, "Vivi Spell")),
			real:     []string{"Island", "Mountain", "Forest"}, seeds: seeds, reach: vivi},
		{name: "vivi_grant_filter_maypay", first: "Vivi Probe",
			fixtures: map[string]string{"Vivi Probe": viviProbeScript, "Vivi Spell": viviSpellScript, "Grant Probe": grantProbeScript, "Sig Probe": sigProbeScript, "Duo Probe": duoProbeScript, "Pay Probe": payProbeScript("U", "G U")},
			deck:     concatDecks(deckOf(8, "Vivi Probe"), deckOf(7, "Island"), deckOf(7, "Mountain"), deckOf(4, "Forest"), deckOf(4, "Grant Probe"), deckOf(3, "Sig Probe"), deckOf(3, "Pay Probe"), deckOf(4, "Vivi Spell")),
			real:     []string{"Island", "Mountain", "Forest"}, seeds: seedRange(41, 100), reach: vivi},
		{name: "vivi_filter", first: "Vivi Probe",
			fixtures: map[string]string{"Vivi Probe": viviProbeScript, "Vivi Spell": viviSpellScript, "Sig Probe": sigProbeScript, "Dove Probe": doveProbeScript, "Duo Probe": duoProbeScript},
			deck:     concatDecks(deckOf(8, "Vivi Probe"), deckOf(8, "Island"), deckOf(8, "Mountain"), deckOf(2, "Forest"), deckOf(4, "Sig Probe"), deckOf(3, "Dove Probe"), deckOf(3, "Duo Probe"), deckOf(4, "Vivi Spell")),
			real:     []string{"Island", "Mountain", "Forest"}, seeds: seeds, reach: vivi},
		{name: "vivi_maypay", first: "Vivi Probe",
			fixtures: map[string]string{"Vivi Probe": viviProbeScript, "Pay Probe": payProbeScript("U", "G")},
			deck:     concatDecks(deckOf(14, "Vivi Probe"), deckOf(9, "Island"), deckOf(9, "Mountain"), deckOf(8, "Pay Probe")),
			real:     []string{"Island", "Mountain", "Forest"}, seeds: seeds, reach: vivi},
		{name: "vivi_maypay_beyond_power", first: "Vivi Probe",
			fixtures: map[string]string{"Vivi Probe": viviProbeScript, "Pay Probe": payProbeScript("U", "U U U U")},
			deck:     concatDecks(deckOf(14, "Vivi Probe"), deckOf(9, "Island"), deckOf(9, "Mountain"), deckOf(8, "Pay Probe")),
			real:     []string{"Island", "Mountain", "Forest"}, seeds: seeds, reach: vivi},
	}
}

// TestHostedCmdrSquanderedBoardsLockstep: every Squandered Resources board
// plays divergence-free, uses the sacrifice on at least one seed and, in the
// payment-window boards, is offered by a window.
func TestHostedCmdrSquanderedBoardsLockstep(t *testing.T) {
	manaShapeBoardsLockstep(t, "squandered_")
}

// TestHostedCmdrViviBoardsLockstep: every Vivi Ornitier board plays
// divergence-free and activates Vivi on at least one seed.
func TestHostedCmdrViviBoardsLockstep(t *testing.T) {
	manaShapeBoardsLockstep(t, "vivi_")
}

func manaShapeBoardsLockstep(t *testing.T, prefix string) {
	ran := 0
	for _, c := range manaShapeBoards() {
		if !strings.HasPrefix(c.name, prefix) {
			continue
		}
		ran++
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			manaShapeBoardLockstep(t, c)
		})
	}
	if ran == 0 {
		t.Fatalf("no board named %s*", prefix)
	}
}

func manaShapeBoardLockstep(t *testing.T, c manaShapeBoard) {
	opp := c.opp
	if opp == nil {
		opp = deckOf(40, "Forest")
	}
	decks := [2][]string{c.deck, opp}
	if len(decks[0]) != 40 || len(decks[1]) != 40 {
		t.Fatalf("deck sizes = %d/%d, want 40/40", len(decks[0]), len(decks[1]))
	}
	dir := filters11Corpus(t, c.fixtures, c.real...)
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
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
	reached, divergent, windowed := false, 0, 0
	var report []string
	for _, seed := range c.seeds {
		m, err := NewMatchup(reg, [2]string{"hms-board-a", "hms-board-b"}, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: Config{}, MaxDecisions: 6000, MaxTurns: 40}
		rc.Finish = func(g *GorgeSide) {
			if c.reach(g) {
				reached = true
			}
		}
		res := Lockstep(m, seed, k, manaShapeBoardPolicy(m, seed, c.first, &windowed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			divergent++
			if divergent <= 3 {
				var last []string
				for _, a := range res.Actions[max(len(res.Actions)-8, 0):] {
					last = append(last, fmt.Sprintf("%v:%q", a, m.InstanceCard(int(a.Src()))))
				}
				report = append(report, fmt.Sprintf("seed %d: divergence at %d: %s %s %v detail=%q\nlast %v\ngorge %v\ndirac %v",
					seed, d.At, d.What, d.Field, d.Diff, d.Detail, last, d.RefOptions, d.KerOptions))
			}
		}
		if h := res.Stall; strings.Contains(h, "hang") || strings.Contains(h, "unmodel") {
			t.Fatalf("seed %d: stall %q", seed, h)
		}
	}
	if divergent != 0 {
		t.Fatalf("%d of %d seeds diverged\n%s", divergent, len(c.seeds), strings.Join(report, "\n"))
	}
	if c.window && windowed == 0 {
		t.Fatalf("no payment window offered Squandered Probe on any of %d seeds", len(c.seeds))
	}
	if !reached {
		t.Fatalf("the shape under test was not used on any of %d seeds, so the lockstep is vacuous", len(c.seeds))
	}
}

// manaShapeBoardPolicy casts first whenever offered, anything else on even
// seeds, pays every optional trigger cost, uses first's own activations when a
// payment window lists them, and otherwise plays a land or answers randomly.
// Seat 1 casts what it can, so a ward creature reaches the battlefield.
func manaShapeBoardPolicy(m *Matchup, seed uint64, first string, windowed *int) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G == nil {
			return rnd.Pick(p)
		}
		if p.D.Kind != DPriority {
			if p.D.Player != 0 {
				return rnd.Pick(p)
			}
			for _, a := range p.D.Options {
				if strings.Contains(a.String(), "trigger_cost_pay") {
					return a
				}
			}
			for _, a := range p.D.Options {
				if m.InstanceCard(int(a.Src())) == "Squandered Probe" {
					*windowed++
					return a
				}
			}
			return rnd.Pick(p)
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActCast && p.D.Player == 0 && m.InstanceCard(int(a.Src())) == first {
				return a
			}
		}
		if seed%2 == 0 || p.D.Player == 1 {
			for _, a := range p.D.Options {
				if a.Kind() == ActCast {
					return a
				}
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActPlayLand {
				return a
			}
		}
		return rnd.Pick(p)
	})
}

// hostedCmdrSquanderedSacrificedAny is hostedCmdrSquanderedSacrificed over the
// boards' land names.
func hostedCmdrSquanderedSacrificedAny(g *GorgeSide) bool {
	if g == nil || g.E == nil {
		return false
	}
	squandered := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil {
			continue
		}
		if e.Kind == events.MoveZone && e.To == state.ZBattlefield && o.Face().Name == "Squandered Probe" {
			squandered = true
		}
		if squandered && events.IsSacrifice(*e) {
			switch o.Face().Name {
			case "Forest", "Swamp", "Waste Probe", "Pain Probe":
				return true
			}
		}
	}
	return false
}
