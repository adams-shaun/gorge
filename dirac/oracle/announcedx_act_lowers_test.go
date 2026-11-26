package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// A printed activated ability's announced X from a cost part with no printed
// {X}, read by its target's filter (CR 107.3, 601.2b, 602.2b: X is announced
// before the targets), in lockstep against gorge:
//
//	Chthonian Nightmare        PayEnergy<X> Sac<1/Creature> Return<1/CARDNAME>, cmcEQX
//	                           (gorge xAsk's energyX, bounded by the energy; kernel cost_energyx.go)
//	Aryel, Knight of Windgrace B T tapXType<X/Knight>, powerLEX
//	                           (gorge tapPermanentCostAsk's X election; kernel cost_tapxann.go)
//
// Each case is built from the card names alone (printedDelveSet) and must
// reach an activation announcing X > 0 in gorge's log (the ability stack
// object's CastInfo Amount: no other card of the decks announces an X).

type announcedXActCase struct {
	name   string
	prefer []string
	decks  [2][]string
	first  uint64
	last   uint64
}

func chthonianCase(first, last uint64) announcedXActCase {
	return announcedXActCase{
		name:   "announcedx_chthonian",
		prefer: []string{"Chthonian Nightmare", "Grizzly Bears", "Scathe Zombies", "Ornithopter", "Hill Giant"},
		decks: [2][]string{
			append(deckOf(6, "Chthonian Nightmare", append(deckOf(2, "Recurring Nightmare", deckOf(6, "Grizzly Bears", deckOf(4, "Scathe Zombies", deckOf(4, "Ornithopter", deckOf(2, "Hill Giant")...)...)...)...), deckOf(16, "Swamp")...)...), deckOf(4, "Forest")...),
			append(deckOf(16, "Grizzly Bears", deckOf(4, "Hill Giant")...), deckOf(20, "Forest")...),
		},
		first: first, last: last,
	}
}

// energyCase: HELIOS One's {3}, {T}, PayEnergy<X>, sacrifice it (its own
// {1}, {T} energy) and Sphinx of the Revelation's {W}{U}{U}, {T},
// PayEnergy<X> (lifelink energy).
func energyCase(first, last uint64) announcedXActCase {
	return announcedXActCase{
		name:   "announcedx_energy",
		prefer: []string{"Sphinx of the Revelation", "HELIOS One", "Grizzly Bears"},
		decks: [2][]string{
			append(deckOf(8, "HELIOS One", deckOf(6, "Sphinx of the Revelation", deckOf(6, "Grizzly Bears")...)...), append(deckOf(10, "Plains"), deckOf(10, "Island")...)...),
			append(deckOf(12, "Grizzly Bears", append(deckOf(4, "Ornithopter"), deckOf(4, "Hill Giant")...)...), deckOf(20, "Forest")...),
		},
		first: first, last: last,
	}
}

// artifactTapCase: tapXType<X/Artifact> and <X/Creature> on Merchant's
// Dockhand, Secluded Starforge, Glacian, Powerstone Engineer, Belisarius Cawl
// and Resonance Technician (copying a cmcEQX instant).
func artifactTapCase(first, last uint64) announcedXActCase {
	return announcedXActCase{
		name:   "announcedx_artifacttap",
		prefer: []string{"Merchant's Dockhand", "Ornithopter", "Belisarius Cawl", "Resonance Technician", "Glacian, Powerstone Engineer", "Opt", "Secluded Starforge"},
		decks: [2][]string{
			append(deckOf(4, "Merchant's Dockhand", deckOf(4, "Secluded Starforge", deckOf(2, "Glacian, Powerstone Engineer", deckOf(3, "Belisarius Cawl", deckOf(2, "Resonance Technician", deckOf(8, "Ornithopter", deckOf(3, "Opt")...)...)...)...)...)...), append(deckOf(8, "Island"), append(deckOf(3, "Plains"), deckOf(3, "Mountain")...)...)...),
			append(deckOf(12, "Grizzly Bears", append(deckOf(4, "Ornithopter"), deckOf(4, "Hill Giant")...)...), deckOf(20, "Forest")...),
		},
		first: first, last: last,
	}
}

func aryelCase(first, last uint64) announcedXActCase {
	return announcedXActCase{
		name:   "announcedx_aryel",
		prefer: []string{"Aryel, Knight of Windgrace", "Youthful Knight"},
		decks: [2][]string{
			append(deckOf(8, "Aryel, Knight of Windgrace", deckOf(12, "Youthful Knight")...), append(deckOf(10, "Plains"), deckOf(10, "Swamp")...)...),
			append(deckOf(12, "Grizzly Bears", append(deckOf(4, "Ornithopter"), deckOf(4, "Hill Giant")...)...), deckOf(20, "Forest")...),
		},
		first: first, last: last,
	}
}

// announcedXActRun plays the case's seeds in lockstep and returns the games,
// the games that reached an X > 0 activation, and the seeds that diverged.
func announcedXActRun(t *testing.T, c announcedXActCase, mutate func(*kop.Table)) (games, hits, divs int) {
	t.Helper()
	reg := synthReg(t)
	set := printedDelveSet(t, c.decks)
	var decks [2][]*cards.Card
	for s := range decks {
		for _, n := range c.decks[s] {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], card)
		}
	}
	tbl, _, _, err := set.Matchup(mbc5Entries(c.decks[0]), mbc5Entries(c.decks[1]))
	if err != nil {
		t.Fatalf("%s: the pool does not lower: %v", c.name, err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	for seed := c.first; seed <= c.last; seed++ {
		cfg := Config{GorgeAutopaySurface: true}
		m, err := NewMatchup(reg, [2]string{c.name + "-a", c.name + "-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 30}
		reached := false
		rc.Finish = func(g *GorgeSide) {
			reached = mbc5Count(g, func(e *events.Event) bool {
				return e.Kind == events.CastInfo && e.Amount > 0 && e.Counter == ""
			}) > 0
		}
		res := Lockstep(m, seed, k, effectLongPolicy(m, effectLongCase{prefer: c.prefer}, seed), rc)
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			t.Logf("%s seed %d: diverged at decision %d: %s %s %v %s", c.name, seed, d.At, d.What, d.Field, d.Diff, d.Detail)
			continue
		}
		if reached {
			hits++
		}
	}
	return games, hits, divs
}

func announcedXActLowers(t *testing.T, c announcedXActCase) {
	t.Helper()
	games, hits, divs := announcedXActRun(t, c, nil)
	t.Logf("%s: %d games, %d reached an X > 0 activation", c.name, games, hits)
	if divs > 0 {
		t.Errorf("%s: %d of %d seeds diverged", c.name, divs, games)
	}
	if hits == 0 {
		t.Errorf("%s: no seed reached an X > 0 activation", c.name)
	}
}

func TestChthonianNightmareLowers(t *testing.T) {
	t.Parallel()
	announcedXActLowers(t, chthonianCase(1, 40))
}

func TestAnnouncedEnergyXLowers(t *testing.T) {
	t.Parallel()
	announcedXActLowers(t, energyCase(1, 40))
}

func TestAnnouncedTapXArtifactsLowers(t *testing.T) {
	t.Parallel()
	announcedXActLowers(t, artifactTapCase(1, 40))
}

func TestAryelKnightOfWindgraceLowers(t *testing.T) {
	t.Parallel()
	announcedXActLowers(t, aryelCase(1, 40))
}

// TestAnnouncedXActMutation: with the energy part dropped from the table (no
// X ask), the announced energy left unpaid, or the tapped count never bound
// as X, some seed diverges. It is not parallel: it sets the kernel's debug
// switches.
func TestAnnouncedXActMutation(t *testing.T) {
	noEnergyX := func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				abs[j].Cost.EnergyX = false
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
	if _, _, n := announcedXActRun(t, chthonianCase(1, 40), noEnergyX); n == 0 {
		t.Error("no energy part: no seed diverged: the pool never announces X by a PayEnergy<X>")
	}
	dirac.DebugEnergyXUnpaid = true
	_, _, n1 := announcedXActRun(t, chthonianCase(1, 40), nil)
	_, _, n2 := announcedXActRun(t, energyCase(1, 40), nil)
	dirac.DebugEnergyXUnpaid = false
	if n1+n2 == 0 {
		t.Error("energy unpaid: no seed diverged: the pool never pays an announced PayEnergy<X>")
	}
	dirac.DebugTapXAnnUnbound = true
	_, _, n := announcedXActRun(t, aryelCase(1, 40), nil)
	dirac.DebugTapXAnnUnbound = false
	if n == 0 {
		t.Error("tap count unbound: no seed diverged: the pool never announces X by a tapXType<X/Knight> election")
	}
}
