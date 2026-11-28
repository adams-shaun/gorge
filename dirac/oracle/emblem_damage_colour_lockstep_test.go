package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Chandra, Awakened Inferno's +2 gives each opponent an emblem with "At the
// beginning of your upkeep, this emblem deals 1 damage to you." An emblem has
// no characteristics (CR 114.3): its damage comes from a colorless source, so
// seat 1's Sphere of Law ("If a red source would deal damage to you, prevent 2
// of that damage") does not prevent it. The kernel must not read the creating
// card's red for the emblem's damage.

func emblemColourDecks() [2][]string {
	return [2][]string{
		append(deckOf(8, "Chandra, Awakened Inferno"), deckOf(16, "Mountain")...),
		append(deckOf(8, "Sphere of Law"), deckOf(16, "Plains")...),
	}
}

// emblemColourMaxPlus caps the +2 activations under the kernel's emblems per
// player (dirac maxEmbTotal, 8), whose overflow is a loud capacity stall.
const emblemColourMaxPlus = 7

// emblemColourPolicy: seat 0 activates Chandra's +2 when offered (at most
// emblemColourMaxPlus times, never again after), else plays a land, else
// casts; everything else is random.
func emblemColourPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	plusN := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DPriority || c.D.Player != 0 {
			return rnd.Pick(c)
		}
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		var plus, cast, land, rest []Canon
		for _, o := range c.D.Options {
			isPlus := o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Chandra, Awakened Inferno" && o.Mode() == 0
			switch {
			case isPlus:
				plus = append(plus, o)
			case o.Kind() == ActCast:
				cast = append(cast, o)
			case o.Kind() == ActPlayLand:
				land = append(land, o)
			}
			if !isPlus {
				rest = append(rest, o)
			}
		}
		switch {
		case len(plus) > 0 && plusN < emblemColourMaxPlus:
			plusN++
			return plus[0]
		case len(land) > 0:
			return pick(land)
		case len(cast) > 0:
			return pick(cast)
		}
		return pick(rest)
	})
}

// TestEmblemDamageIsColorlessLockstep plays the matchup over 12 seeds: no
// divergence, no stall, and some game deals seat 1 the emblem's 1 damage.
func TestEmblemDamageIsColorlessLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	deck := emblemColourDecks()
	set := cardset.Raw(printedDelveSet(t, deck)) // Chandra is refused for an unrelated NStack class
	var decks [2][]*cards.Card
	var entries [2][]cardset.Entry
	for s := range decks {
		for _, n := range deck[s] {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
			entries[s] = append(entries[s], cardset.Entry{Name: n, Count: 1})
		}
	}
	tbl, _, _, err := set.Matchup(entries[0], entries[1])
	if err != nil {
		t.Fatal(err)
	}
	pings := 0
	for seed := uint64(1); seed <= 12; seed++ {
		cfg := Config{GorgeAutopaySurface: true}
		m, err := NewMatchup(reg, [2]string{"emblemcolour-a", "emblemcolour-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, emblemColourPolicy(m, seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		for _, e := range g.E.L.Events {
			if e.Kind == events.Damage && e.Obj == 0 && e.Player == 1 && e.Amount == 1 {
				pings++
			}
		}
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if pings == 0 {
		t.Fatalf("no 1-damage event to seat 1: the emblem's trigger was never reached")
	}
	t.Logf("%d 1-damage events to seat 1", pings)
}
