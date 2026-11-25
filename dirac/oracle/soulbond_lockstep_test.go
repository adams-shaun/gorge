package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Soulbond decks (CR 702.95): the pair triggers (this creature enters;
// another creature enters beside it), the optional partner pick, and the
// statics and triggers that read the pair (Creature.PairedWith,
// Creature.Self+Paired: power, keywords, granted activated abilities and
// granted triggers), with the pair ending as a member dies or blinks
// (Deadeye Navigator).
var soulbondDecks = [3][]cardset.Entry{
	{
		{Name: "Druid's Familiar", Count: 3}, {Name: "Wolfir Silverheart", Count: 2}, {Name: "Trusted Forcemage", Count: 3},
		{Name: "Nearheath Pilgrim", Count: 3}, {Name: "Elgaud Shieldmate", Count: 2}, {Name: "Silverblade Paladin", Count: 3},
		{Name: "Geist Trappers", Count: 2}, {Name: "Spectral Gateguards", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Lightning Mauler", Count: 3}, {Name: "Hanweir Lancer", Count: 3}, {Name: "Stonewright", Count: 3},
		{Name: "Galvanic Alchemist", Count: 2}, {Name: "Stern Mentor", Count: 2}, {Name: "Tandem Lookout", Count: 3},
		{Name: "Deadeye Navigator", Count: 2}, {Name: "Wingcrafter", Count: 2},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Donna Noble", Count: 3}, {Name: "Thundering Mightmare", Count: 3}, {Name: "Pathbreaker Wurm", Count: 2},
		{Name: "Diregraf Escort", Count: 3}, {Name: "Lightning Mauler", Count: 3}, {Name: "Hanweir Lancer", Count: 3},
		{Name: "Druid's Familiar", Count: 3},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

func soulbondNames(d []cardset.Entry) []string {
	var out []string
	for _, e := range d {
		for j := 0; j < e.Count; j++ {
			out = append(out, e.Name)
		}
	}
	return out
}

// TestSoulbondLockstep plays the Soulbond decks against each other on both
// surfaces: no divergence, no kernel stall, and the games pair and unpair.
func TestSoulbondLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	var all [2][]string
	for i, d := range soulbondDecks {
		if n := len(soulbondNames(d)); n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		all[i%2] = append(all[i%2], soulbondNames(d)...)
	}
	set := printedDelveSet(t, all)
	const games = 200
	pairs, unpairs, divs := 0, 0, 0
	for seed := uint64(1); seed <= games; seed++ {
		a, b := soulbondDecks[seed%3], soulbondDecks[(seed+1)%3]
		tbl, _, _, err := set.Matchup(a, b)
		if err != nil {
			t.Fatal(err)
		}
		surf := "full"
		if seed%2 == 0 {
			surf = "autopay"
		}
		r := &Repro{Format: reproFormat, Decks: [2]string{"sb-a", "sb-b"}, Cards: [2][]string{soulbondNames(a), soulbondNames(b)}, Seed: seed, PassElision: true, Surface: surf}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 24
		rc.Finish = func(g *GorgeSide) {
			for _, e := range g.E.L.Events {
				if e.Kind != events.Pair {
					continue
				}
				if len(e.IDs) > 0 {
					pairs++
				} else {
					unpairs++
				}
			}
		}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			divs++
			if divs <= 3 {
				lo := max(res.Div.At-12, 0)
				t.Errorf("seed %d (%s): diverged at %d: %s %s %v %s\nref %v\nker %v\nlast actions %v", seed, surf, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions, actionStrings(res.Actions[lo:res.Div.At]))
			}
			continue
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d (%s): kernel stopped: %s", seed, surf, res.Stall)
		}
	}
	t.Logf("%d games, %d divergent, %d pairing(s), %d SBA unpairing(s)", games, divs, pairs, unpairs)
	if pairs == 0 {
		t.Error("no game paired two creatures")
	}
}
